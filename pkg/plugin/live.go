package plugin

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
	"github.com/opencapital-dev/oc-plugin-sdk/datakey"
	yflive "github.com/wnjoon/go-yfinance/pkg/live"
	yfmodels "github.com/wnjoon/go-yfinance/pkg/models"
)

type symbolTarget struct {
	InstrumentID string
	PortfolioID  string
	// Currency is the authoritative Yahoo metadata currency (e.g. "GBp")
	// resolved at backfill; RefPrice is the minor-unit reference anchor. Both
	// drive minor-unit (pence) normalization of live ticks independent of the
	// unreliable ws currency. Empty/zero until the first backfill resolves them.
	Currency string
	RefPrice float64
}

type LiveSubscriber struct {
	ws       *yflive.WebSocket
	client   rwPGClient
	ticks    *LiveTickMap
	pluginID string
	mu       sync.Mutex
	current  map[string]struct{}
	bySymbol map[string][]symbolTarget
}

func NewLiveSubscriber(client rwPGClient, ticks *LiveTickMap, pluginID string) (*LiveSubscriber, error) {
	ws, err := yflive.New()
	if err != nil {
		return nil, err
	}
	return &LiveSubscriber{
		ws:       ws,
		client:   client,
		ticks:    ticks,
		pluginID: pluginID,
		current:  map[string]struct{}{},
		bySymbol: map[string][]symbolTarget{},
	}, nil
}

// Start connects + begins the listen goroutine.
func (s *LiveSubscriber) Start(_ context.Context) error {
	if err := s.ws.Connect(); err != nil {
		return err
	}
	_ = s.ws.ListenAsync(func(data *yfmodels.PricingData) {
		s.publishTick(data)
	})
	return nil
}

// canonicalSymbol returns the REST-resolved fully-qualified Yahoo symbol stored
// at backfill in vendor_meta.canonical.symbol, falling back to the raw mapping
// symbol until the first backfill resolves it. Subscribing the canonical form
// makes the live ws resolve the same listing REST used.
func canonicalSymbol(m TickerMapping) string {
	if c, ok := m.VendorMeta["canonical"].(map[string]any); ok {
		if s, ok := c["symbol"].(string); ok && s != "" {
			return s
		}
	}
	return m.Symbol
}

// canonicalUnit returns the authoritative currency + minor-unit reference price
// captured at backfill in vendor_meta.canonical. Empty/zero when no backfill has
// resolved them yet, in which case the live path falls back to the ws currency.
func canonicalUnit(m TickerMapping) (string, float64) {
	c, ok := m.VendorMeta["canonical"].(map[string]any)
	if !ok {
		return "", 0
	}
	currency, _ := c["currency"].(string)
	var ref float64
	switch v := c["ref_price"].(type) {
	case float64:
		ref = v
	case json.Number:
		ref, _ = v.Float64()
	}
	return currency, ref
}

// desiredSymbols maps the upper-cased canonical Yahoo symbol → its targets.
// Subscribing the canonical form keeps the ws and REST on the same listing.
func desiredSymbols(mappings []TickerMapping) map[string][]symbolTarget {
	out := map[string][]symbolTarget{}
	for _, m := range mappings {
		sym := canonicalSymbol(m)
		if sym == "" {
			continue
		}
		up := strings.ToUpper(sym)
		currency, ref := canonicalUnit(m)
		out[up] = append(out[up], symbolTarget{
			InstrumentID: m.InstrumentID,
			PortfolioID:  m.PortfolioID,
			Currency:     currency,
			RefPrice:     ref,
		})
	}
	return out
}

func (s *LiveSubscriber) SetSymbols(ctx context.Context, mappings []TickerMapping) {
	bySymbol := desiredSymbols(mappings)
	desired := make(map[string]struct{}, len(bySymbol))
	for up := range bySymbol {
		desired[up] = struct{}{}
	}

	s.mu.Lock()
	toAdd := []string{}
	toRemove := []string{}
	for sym := range desired {
		if _, ok := s.current[sym]; !ok {
			toAdd = append(toAdd, sym)
		}
	}
	for sym := range s.current {
		if _, ok := desired[sym]; !ok {
			toRemove = append(toRemove, sym)
		}
	}
	s.current = desired
	s.bySymbol = bySymbol
	s.mu.Unlock()

	sort.Strings(toAdd)
	sort.Strings(toRemove)

	if len(toAdd) > 0 {
		if err := s.ws.Subscribe(toAdd); err != nil {
			log.DefaultLogger.Warn("live ws subscribe failed", "count", len(toAdd), "err", err)
		}
	}
	if len(toRemove) > 0 {
		if err := s.ws.Unsubscribe(toRemove); err != nil {
			log.DefaultLogger.Warn("live ws unsubscribe failed", "count", len(toRemove), "err", err)
		}
	}
}

func (s *LiveSubscriber) Close() {
	if s == nil || s.ws == nil {
		return
	}
	_ = s.ws.Close()
}

// quoteObservedMicros converts a Yahoo WebSocket quote time (epoch
// milliseconds) to epoch microseconds. A zero time (Yahoo omitted it)
// falls back to now.
func quoteObservedMicros(timeMs int64, now time.Time) int64 {
	if timeMs == 0 {
		return now.UnixMicro()
	}
	return timeMs * 1_000
}

func (s *LiveSubscriber) publishTick(data *yfmodels.PricingData) {
	if data == nil || data.ID == "" {
		return
	}
	up := strings.ToUpper(data.ID)
	s.mu.Lock()
	targets := s.bySymbol[up]
	s.mu.Unlock()
	if len(targets) == 0 {
		return
	}

	observedAtUs := quoteObservedMicros(data.Time, time.Now())

	rawMid := float64(data.Price)
	rawBid := float64(data.Bid)
	rawAsk := float64(data.Ask)

	ctx := context.Background()

	for _, tgt := range targets {
		// Resolve the unit per target from the authoritative currency captured
		// at backfill (falling back to the ws currency), then classify THIS tick
		// minor/major against the reference — the ws sends pence and pounds
		// intermittently on the same minor-unit listing.
		majorCurrency, divisor := liveUnit(tgt.Currency, data.Currency)
		mid := normalizeTickValue(rawMid, tgt.RefPrice, divisor)
		bid := normalizeTickValue(rawBid, tgt.RefPrice, divisor)
		ask := normalizeTickValue(rawAsk, tgt.RefPrice, divisor)
		if bid <= 0 {
			bid = mid
		}
		if ask <= 0 {
			ask = mid
		}

		s.ticks.Set(tgt.InstrumentID+"|"+tgt.PortfolioID, observedAtUs)

		payloadJSON, perr := json.Marshal(map[string]any{
			"bid_price": bid,
			"ask_price": ask,
			"bid_size":  data.BidSize,
			"ask_size":  data.AskSize,
			"currency":  majorCurrency,
			"venue":     data.Exchange,
		})
		if perr != nil {
			continue
		}
		rwKey := datakey.DataKey(s.pluginID, QuoteNamespace, tgt.PortfolioID, tgt.InstrumentID, observedAtUs)
		_, err := s.client.Exec(ctx, `
			INSERT INTO data_log
				(source_namespace, source_id, portfolio_id, observed_at, ingest_ts, source, plugin_id, trace_id, payload, rw_key)
			VALUES ($1, $2, $3, to_timestamp($4::double precision / 1e6), now(), $5, $6, $7, $8, $9)
		`,
			QuoteNamespace, tgt.InstrumentID, tgt.PortfolioID, observedAtUs,
			"yahoo_ws", s.pluginID, "", string(payloadJSON), rwKey,
		)
		if err != nil {
			log.DefaultLogger.Warn("live tick publish failed",
				"instrument_id", tgt.InstrumentID,
				"portfolio_id", tgt.PortfolioID,
				"err", err)
		}
	}
}
