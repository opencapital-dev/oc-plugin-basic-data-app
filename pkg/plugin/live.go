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
)

// quoteFetcher is the interface QuotePoller uses to retrieve the current price
// for a symbol. *YfClient satisfies this interface via FetchQuote.
type quoteFetcher interface {
	FetchQuote(ctx context.Context, symbol string) (price float64, currency string, err error)
}

// quoteDayKey returns the data_log rw_key for a quote row keyed to the UTC day
// containing atUs (microseconds). Two polls on the same UTC day for the same
// (portfolio, instrument) produce the same key, causing data_log's PRIMARY KEY
// upsert — one row per (instrument, day). Today's row updates each minute;
// past days freeze at their last poll value (≈ close).
func quoteDayKey(pluginID, portfolio, instrument string, atUs int64) string {
	dayStart := time.UnixMicro(atUs).UTC().Truncate(24 * time.Hour)
	return datakey.DataKey(pluginID, QuoteNamespace, portfolio, instrument, dayStart.UnixMicro())
}

type symbolTarget struct {
	InstrumentID string
	PortfolioID  string
	// Currency is the authoritative Yahoo metadata currency (e.g. "GBp")
	// resolved at backfill; RefPrice is the minor-unit reference anchor. Both
	// drive minor-unit (pence) normalization of polled prices independent of
	// the unreliable API-reported currency. Empty/zero until the first
	// backfill resolves them.
	Currency string
	RefPrice float64
}

// QuotePoller replaces the WebSocket LiveSubscriber. On a 60-second timer it
// calls YfClient.FetchQuote for each tracked symbol and upserts ONE data_log
// quote row per (instrument, day) — same-day polls hit the same primary-key row
// (the day-truncated rw_key makes them identical), so today's price moves while
// past days freeze at their last poll value (~close).
type QuotePoller struct {
	yf       quoteFetcher
	client   rwPGClient
	ticks    *LiveTickMap
	pluginID string
	mu       sync.Mutex
	current  map[string]struct{}
	bySymbol map[string][]symbolTarget
	cancel   context.CancelFunc
}

func NewQuotePoller(yf quoteFetcher, client rwPGClient, ticks *LiveTickMap, pluginID string) *QuotePoller {
	return &QuotePoller{
		yf:       yf,
		client:   client,
		ticks:    ticks,
		pluginID: pluginID,
		current:  map[string]struct{}{},
		bySymbol: map[string][]symbolTarget{},
	}
}

// Start launches the background 60-second poll goroutine. The goroutine exits
// when ctx is done or Close is called.
func (p *QuotePoller) Start(ctx context.Context) error {
	pollerCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	go func() {
		tk := time.NewTicker(60 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-pollerCtx.Done():
				return
			case t := <-tk.C:
				p.pollOnce(pollerCtx, t)
			}
		}
	}()
	return nil
}

// Close cancels the poll goroutine started by Start.
func (p *QuotePoller) Close() {
	if p != nil && p.cancel != nil {
		p.cancel()
	}
}

// canonicalSymbol returns the REST-resolved fully-qualified Yahoo symbol stored
// at backfill in vendor_meta.canonical.symbol, falling back to the raw mapping
// symbol until the first backfill resolves it. Polling the canonical form
// matches the same listing REST used during backfill.
func canonicalSymbol(m TickerMapping) string {
	if c, ok := m.VendorMeta["canonical"].(map[string]any); ok {
		if s, ok := c["symbol"].(string); ok && s != "" {
			return s
		}
	}
	return m.Symbol
}

// canonicalUnit returns the authoritative currency + minor-unit reference price
// captured at backfill in vendor_meta.canonical. Empty/zero when no backfill
// has resolved them yet.
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
// Using the canonical form keeps the poll and REST on the same listing.
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

// SetSymbols updates the set of tracked symbols. The previous WebSocket
// subscribe/unsubscribe calls are no longer needed; this now only updates
// the in-memory symbol → target bookkeeping.
func (p *QuotePoller) SetSymbols(_ context.Context, mappings []TickerMapping) {
	bySymbol := desiredSymbols(mappings)
	desired := make(map[string]struct{}, len(bySymbol))
	for up := range bySymbol {
		desired[up] = struct{}{}
	}

	p.mu.Lock()
	var toAdd, toRemove []string
	for sym := range desired {
		if _, ok := p.current[sym]; !ok {
			toAdd = append(toAdd, sym)
		}
	}
	for sym := range p.current {
		if _, ok := desired[sym]; !ok {
			toRemove = append(toRemove, sym)
		}
	}
	p.current = desired
	p.bySymbol = bySymbol
	p.mu.Unlock()

	sort.Strings(toAdd)
	sort.Strings(toRemove)
	if len(toAdd) > 0 {
		log.DefaultLogger.Debug("quote poller: tracking new symbols", "symbols", toAdd)
	}
	if len(toRemove) > 0 {
		log.DefaultLogger.Debug("quote poller: dropped symbols", "symbols", toRemove)
	}
}

// pollOnce is the per-tick work unit: for every tracked symbol it fetches the
// current price via FetchQuote and upserts one data_log row per (instrument,
// day). now is the authoritative timestamp — in production it comes from the
// ticker channel; in tests a fixed time is injected for determinism.
func (p *QuotePoller) pollOnce(ctx context.Context, now time.Time) {
	p.mu.Lock()
	snapshot := make(map[string][]symbolTarget, len(p.bySymbol))
	for k, v := range p.bySymbol {
		snapshot[k] = v
	}
	p.mu.Unlock()

	observedAtUs := now.UnixMicro()

	for sym, targets := range snapshot {
		price, currency, err := p.yf.FetchQuote(ctx, sym)
		if err != nil {
			log.DefaultLogger.Warn("quote poller: FetchQuote failed", "symbol", sym, "err", err)
			continue
		}

		for _, tgt := range targets {
			// Resolve the unit per target from the authoritative currency
			// captured at backfill (falling back to the API-reported currency).
			// Yahoo's FastInfo currency is more reliable than the ws field but
			// still returns "GBp" / "GBX" for LSE tickers — normalizeMinorUnits
			// handles those; liveUnit prefers the authoritative backfill value.
			majorCurrency, divisor := liveUnit(tgt.Currency, currency)
			mid := normalizeTickValue(price, tgt.RefPrice, divisor)

			p.ticks.Set(tgt.InstrumentID+"|"+tgt.PortfolioID, observedAtUs)

			payloadJSON, perr := json.Marshal(map[string]any{
				"bid_price": mid,
				"ask_price": mid,
				"bid_size":  0,
				"ask_size":  0,
				"currency":  majorCurrency,
				"venue":     "",
			})
			if perr != nil {
				continue
			}

			// rw_key is day-truncated so same-day polls upsert the same row.
			// observed_at stays the actual poll time so the point advances
			// intra-day (and RW sees it as updated).
			rwKey := quoteDayKey(p.pluginID, tgt.PortfolioID, tgt.InstrumentID, observedAtUs)
			_, execErr := p.client.Exec(ctx, `
				INSERT INTO data_log
					(source_namespace, source_id, portfolio_id, observed_at, ingest_ts, source, plugin_id, trace_id, payload, rw_key)
				VALUES ($1, $2, $3, to_timestamp($4::double precision / 1e6), now(), $5, $6, $7, $8, $9)
			`,
				QuoteNamespace, tgt.InstrumentID, tgt.PortfolioID, observedAtUs,
				"yahoo_poll", p.pluginID, "", string(payloadJSON), rwKey,
			)
			if execErr != nil {
				log.DefaultLogger.Warn("quote poller: publish failed",
					"instrument_id", tgt.InstrumentID,
					"portfolio_id", tgt.PortfolioID,
					"err", execErr)
			}
		}
	}
}
