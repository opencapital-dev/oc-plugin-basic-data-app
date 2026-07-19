package plugin

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/opencapital-dev/oc-plugin-sdk/datakey"
)

// TestQuoteDayKey verifies that quoteDayKey truncates atUs to the UTC day
// boundary: two timestamps on the same UTC day produce the same key; timestamps
// on different days produce distinct keys.
func TestQuoteDayKey(t *testing.T) {
	const pluginID = "basic-data-app"
	const portfolioID = "port-1"
	const instrumentID = "instr-1"

	day1 := time.Date(2024, 12, 5, 0, 0, 0, 0, time.UTC)
	us1 := day1.Add(8 * time.Hour).UnixMicro()  // 08:00 Dec 5
	us2 := day1.Add(16 * time.Hour).UnixMicro() // 16:00 Dec 5, same day
	us3 := day1.Add(24 * time.Hour).UnixMicro() // 00:00 Dec 6, next day

	key1 := quoteDayKey(pluginID, portfolioID, instrumentID, us1)
	key2 := quoteDayKey(pluginID, portfolioID, instrumentID, us2)
	key3 := quoteDayKey(pluginID, portfolioID, instrumentID, us3)

	if key1 != key2 {
		t.Errorf("same-day times must produce the same key:\n  key1=%q\n  key2=%q", key1, key2)
	}
	if key1 == key3 {
		t.Errorf("different-day times must produce different keys:\n  key1=%q\n  key3=%q", key1, key3)
	}
	wantKey := datakey.DataKey(pluginID, QuoteNamespace, portfolioID, instrumentID, day1.UnixMicro())
	if key1 != wantKey {
		t.Errorf("key1=%q, want day-midnight key %q", key1, wantKey)
	}
}

// fakeFetcher is a test double for quoteFetcher.
type fakeFetcher struct {
	price    float64
	currency string
	err      error
}

func (f *fakeFetcher) FetchQuote(_ context.Context, _ string) (float64, string, error) {
	return f.price, f.currency, f.err
}

// TestLiveQuoteDailyKey asserts that two polls on the same UTC day produce the
// same rw_key (so they upsert the same data_log row), while polls on different
// UTC days produce distinct keys. The stored observed_at must remain the actual
// poll time, not the day boundary.
func TestLiveQuoteDailyKey(t *testing.T) {
	const pluginID = "basic-data-app"
	const portfolioID = "port-1"
	const instrumentID = "instr-1"
	const symbol = "AAPL"

	day1 := time.Date(2024, 12, 5, 0, 0, 0, 0, time.UTC)
	tick1 := day1.Add(8 * time.Hour)  // 08:00 Dec 5
	tick2 := day1.Add(16 * time.Hour) // 16:00 Dec 5, same day
	tick3 := day1.Add(24 * time.Hour) // 00:00 Dec 6, next day

	fc := &fakeClient{}
	yf := &fakeFetcher{price: 150.0, currency: "USD"}
	pol := &QuotePoller{
		yf:       yf,
		client:   fc,
		pluginID: pluginID,
		current:  map[string]struct{}{},
		bySymbol: map[string][]symbolTarget{
			symbol: {{InstrumentID: instrumentID, PortfolioID: portfolioID}},
		},
		ticks: NewLiveTickMap(),
	}

	pol.pollOnce(context.Background(), tick1)
	pol.pollOnce(context.Background(), tick2)
	yf.price = 152.0
	pol.pollOnce(context.Background(), tick3)

	if len(fc.execCalls) != 3 {
		t.Fatalf("expected 3 Exec calls, got %d", len(fc.execCalls))
	}

	key1, _ := fc.execCalls[0].args[8].(string)
	key2, _ := fc.execCalls[1].args[8].(string)
	key3, _ := fc.execCalls[2].args[8].(string)

	// Same UTC day → same rw_key (upsert).
	if key1 != key2 {
		t.Errorf("same-day polls must share rw_key:\n  tick1: %q\n  tick2: %q", key1, key2)
	}
	// Different UTC day → distinct rw_key.
	if key1 == key3 {
		t.Errorf("different-day polls must have distinct rw_keys:\n  day1: %q\n  day2: %q", key1, key3)
	}

	// Key must equal quoteDayKey with day-truncated micros.
	dayUs := day1.UnixMicro() // Dec 5 midnight UTC in micros
	wantKey := datakey.DataKey(pluginID, QuoteNamespace, portfolioID, instrumentID, dayUs)
	if key1 != wantKey {
		t.Errorf("rw_key = %q, want day-truncated key %q", key1, wantKey)
	}

	// observed_at arg (index 3) must still be the actual poll micros, not the day boundary.
	if got := fc.execCalls[0].args[3]; got != tick1.UnixMicro() {
		t.Errorf("observed_at = %v, want %v (actual poll micros)", got, tick1.UnixMicro())
	}
}

// TestPollTickDataLogInsert drives pollOnce through the fakeClient seam
// and asserts: correct column order in the INSERT, namespace == prices.quote,
// source == "yahoo_poll", and rw_key == day-truncated quoteDayKey(...).
func TestPollTickDataLogInsert(t *testing.T) {
	const pluginID = "basic-data-app"
	const portfolioID = "port-1"
	const instrumentID = "instr-1"
	const symbol = "AAPL"

	fc := &fakeClient{}
	yf := &fakeFetcher{price: 150.0, currency: "USD"}
	pol := &QuotePoller{
		yf:       yf,
		client:   fc,
		pluginID: pluginID,
		current:  map[string]struct{}{},
		bySymbol: map[string][]symbolTarget{
			symbol: {{InstrumentID: instrumentID, PortfolioID: portfolioID}},
		},
		ticks: NewLiveTickMap(),
	}

	now := time.Date(2024, 12, 5, 8, 0, 0, 0, time.UTC)
	observedAtUs := now.UnixMicro()
	dayStart := time.Date(2024, 12, 5, 0, 0, 0, 0, time.UTC)
	wantRwKey := datakey.DataKey(pluginID, QuoteNamespace, portfolioID, instrumentID, dayStart.UnixMicro())

	pol.pollOnce(context.Background(), now)

	if len(fc.execCalls) != 1 {
		t.Fatalf("expected 1 Exec call, got %d", len(fc.execCalls))
	}
	call := fc.execCalls[0]

	// Verify INSERT targets data_log.
	if !strings.Contains(call.sql, "INSERT INTO data_log") {
		t.Errorf("SQL missing INSERT INTO data_log: %s", call.sql)
	}

	args := call.args
	// Column order: source_namespace, source_id, portfolio_id, observedAtUs, source, plugin_id, trace_id, payload, rw_key
	if args[0] != QuoteNamespace {
		t.Errorf("arg[0] source_namespace = %v, want %v", args[0], QuoteNamespace)
	}
	if args[1] != instrumentID {
		t.Errorf("arg[1] source_id = %v, want %v", args[1], instrumentID)
	}
	if args[2] != portfolioID {
		t.Errorf("arg[2] portfolio_id = %v, want %v", args[2], portfolioID)
	}
	if args[3] != observedAtUs {
		t.Errorf("arg[3] observed_at_us = %v, want %v", args[3], observedAtUs)
	}
	if args[4] != "yahoo_poll" {
		t.Errorf("arg[4] source = %v, want yahoo_poll", args[4])
	}
	if args[5] != pluginID {
		t.Errorf("arg[5] plugin_id = %v, want %v", args[5], pluginID)
	}
	if args[8] != wantRwKey {
		t.Errorf("arg[8] rw_key = %v, want %v", args[8], wantRwKey)
	}
}

// TestPollTickZeroPriceSkipped verifies that when FetchQuote returns price == 0,
// pollOnce performs no INSERT — mirroring backfill_worker.go's skip-on-zero guard.
func TestPollTickZeroPriceSkipped(t *testing.T) {
	const sym = "AAPL"
	fc := &fakeClient{}
	yf := &fakeFetcher{price: 0, currency: "USD"}
	pol := livePollerWithTarget(fc, yf, sym, symbolTarget{InstrumentID: "AAPL", PortfolioID: "p"})
	pol.pollOnce(context.Background(), fixedNow)
	if len(fc.execCalls) != 0 {
		t.Errorf("zero-price symbol must not produce an INSERT, got %d", len(fc.execCalls))
	}
}

func TestCanonicalSymbol(t *testing.T) {
	// No canonical → raw symbol.
	if got := canonicalSymbol(TickerMapping{Symbol: "AET", VendorMeta: map[string]any{}}); got != "AET" {
		t.Errorf("no-canonical = %q, want AET", got)
	}
	// Canonical present → canonical wins.
	m := TickerMapping{Symbol: "AET", VendorMeta: map[string]any{
		"canonical": map[string]any{"symbol": "AET.L", "exch": "LSE"},
	}}
	if got := canonicalSymbol(m); got != "AET.L" {
		t.Errorf("canonical = %q, want AET.L", got)
	}
	// Empty canonical symbol → fall back to raw.
	m2 := TickerMapping{Symbol: "AET", VendorMeta: map[string]any{
		"canonical": map[string]any{"symbol": "", "exch": "LSE"},
	}}
	if got := canonicalSymbol(m2); got != "AET" {
		t.Errorf("empty-canonical = %q, want AET", got)
	}
}

func TestSetSymbolsUsesCanonical(t *testing.T) {
	mappings := []TickerMapping{
		{InstrumentID: "AET", PortfolioID: "p", Symbol: "AET",
			VendorMeta: map[string]any{"canonical": map[string]any{"symbol": "AET.L", "exch": "LSE"}}},
	}
	got := desiredSymbols(mappings)
	if _, ok := got["AET.L"]; !ok {
		t.Errorf("expected desired to contain canonical AET.L, got %v", got)
	}
	if _, ok := got["AET"]; ok {
		t.Errorf("must not subscribe the raw ambiguous symbol AET")
	}
}
