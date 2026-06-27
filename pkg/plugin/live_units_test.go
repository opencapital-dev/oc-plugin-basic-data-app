package plugin

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"
)

// The Yahoo FastInfo currency is more reliable than the ws field but can still
// return "USD"/empty for minor-unit LSE tickers whose price is actually in
// pence. The poll path must mirror the backfill classifier: prefer the
// authoritative currency captured at backfill and decide minor/major per poll
// against a reference price — never trust the API currency field alone.

func TestLiveUnitPrefersAuthoritativeCurrency(t *testing.T) {
	cases := []struct {
		name        string
		auth        string
		apiCurrency string
		wantCur     string
		wantDiv     float64
	}{
		{"authoritative GBp beats API USD", "GBp", "USD", "GBP", 100.0},
		{"authoritative GBX beats empty API currency", "GBX", "", "GBP", 100.0},
		{"authoritative USD beats stray API GBp", "USD", "GBp", "USD", 1.0},
		{"no authoritative falls back to API GBp", "", "GBp", "GBP", 100.0},
		{"no authoritative, API USD", "", "USD", "USD", 1.0},
		{"both empty default USD", "", "", "USD", 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cur, div := liveUnit(tc.auth, tc.apiCurrency)
			if cur != tc.wantCur || div != tc.wantDiv {
				t.Fatalf("liveUnit(%q,%q) = (%q,%v), want (%q,%v)",
					tc.auth, tc.apiCurrency, cur, div, tc.wantCur, tc.wantDiv)
			}
		})
	}
}

func TestNormalizeTickValue(t *testing.T) {
	const eps = 1e-9
	cases := []struct {
		name  string
		value float64
		ref   float64
		div   float64
		want  float64
	}{
		{"pence tick divided to pounds", 176.0, 176.0, 100.0, 1.76},
		{"intermittent pound tick stays pounds", 1.76, 176.0, 100.0, 1.76},
		{"usd major passthrough", 150.0, 0.0, 1.0, 150.0},
		{"minor with no reference divides unconditionally", 176.0, 0.0, 100.0, 1.76},
		{"zero value untouched", 0.0, 176.0, 100.0, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeTickValue(tc.value, tc.ref, tc.div)
			if math.Abs(got-tc.want) > eps {
				t.Fatalf("normalizeTickValue(%v,%v,%v) = %v, want %v",
					tc.value, tc.ref, tc.div, got, tc.want)
			}
		})
	}
}

func TestCanonicalUnit(t *testing.T) {
	// Missing canonical → empty currency, zero ref.
	if cur, ref := canonicalUnit(TickerMapping{VendorMeta: map[string]any{}}); cur != "" || ref != 0 {
		t.Fatalf("no-canonical = (%q,%v), want (\"\",0)", cur, ref)
	}
	// vendor_meta as parsed JSON (numbers come back as float64).
	m := TickerMapping{VendorMeta: map[string]any{
		"canonical": map[string]any{"symbol": "GKP.L", "currency": "GBp", "ref_price": float64(176.9)},
	}}
	cur, ref := canonicalUnit(m)
	if cur != "GBp" || math.Abs(ref-176.9) > 1e-9 {
		t.Fatalf("canonicalUnit = (%q,%v), want (GBp,176.9)", cur, ref)
	}
}

// decodePayload pulls the JSON payload (arg index 7) out of the recorded INSERT.
func decodePayload(t *testing.T, call fakeCall) map[string]any {
	t.Helper()
	s, ok := call.args[7].(string)
	if !ok {
		t.Fatalf("payload arg not a string: %T", call.args[7])
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	return p
}

// livePollerWithTarget constructs a QuotePoller wired to fc and yf,
// pre-loaded with one symbol → target mapping for unit tests.
func livePollerWithTarget(fc *fakeClient, yf quoteFetcher, symbol string, tgt symbolTarget) *QuotePoller {
	return &QuotePoller{
		yf:       yf,
		client:   fc,
		pluginID: "basic-data-app",
		current:  map[string]struct{}{},
		bySymbol: map[string][]symbolTarget{symbol: {tgt}},
		ticks:    NewLiveTickMap(),
	}
}

// fixedNow is a stable poll time used in unit tests.
var fixedNow = time.Date(2024, 12, 5, 8, 0, 0, 0, time.UTC)

// Regression for the NAV-to-3M bug: a GKP pence price arrives tagged
// currency=USD from the API. With the authoritative GBp currency + reference
// captured at backfill, the price must be divided to major units (GBP).
func TestPollTickNormalizesPenceFromBadApiCurrency(t *testing.T) {
	const sym = "GKP.L"
	fc := &fakeClient{}
	yf := &fakeFetcher{price: 176.9, currency: "USD"} // wrong/unreliable API currency
	pol := livePollerWithTarget(fc, yf, sym, symbolTarget{
		InstrumentID: "GKP", PortfolioID: "p", Currency: "GBp", RefPrice: 176.9,
	})
	pol.pollOnce(context.Background(), fixedNow)
	if len(fc.execCalls) != 1 {
		t.Fatalf("want 1 insert, got %d", len(fc.execCalls))
	}
	p := decodePayload(t, fc.execCalls[0])
	if p["currency"] != "GBP" {
		t.Errorf("currency = %v, want GBP", p["currency"])
	}
	// 176.9 pence / 100 = 1.769 GBP
	if mid := p["bid_price"].(float64); math.Abs(mid-1.769) > 1e-6 {
		t.Errorf("bid_price = %v, want ~1.769 (pence/100)", mid)
	}
	if ask := p["ask_price"].(float64); math.Abs(ask-1.769) > 1e-6 {
		t.Errorf("ask_price = %v, want ~1.769", ask)
	}
}

// The same symbol intermittently sends an already-major pound price (1.76 / GBP).
// It must NOT be divided again into 0.01769.
func TestPollTickIntermittentPoundPriceNotDivided(t *testing.T) {
	const sym = "GKP.L"
	fc := &fakeClient{}
	yf := &fakeFetcher{price: 1.762, currency: "GBP"}
	pol := livePollerWithTarget(fc, yf, sym, symbolTarget{
		InstrumentID: "GKP", PortfolioID: "p", Currency: "GBp", RefPrice: 176.9,
	})
	pol.pollOnce(context.Background(), fixedNow)
	p := decodePayload(t, fc.execCalls[0])
	if mid := p["bid_price"].(float64); math.Abs(mid-1.762) > 1e-6 {
		t.Errorf("bid_price = %v, want 1.762 (unchanged major price)", mid)
	}
	if p["currency"] != "GBP" {
		t.Errorf("currency = %v, want GBP", p["currency"])
	}
}

// A USD instrument with no canonical unit must pass through unchanged.
func TestPollTickUSDPassthrough(t *testing.T) {
	const sym = "AAPL"
	fc := &fakeClient{}
	yf := &fakeFetcher{price: 150.0, currency: "USD"}
	pol := livePollerWithTarget(fc, yf, sym, symbolTarget{InstrumentID: "AAPL", PortfolioID: "p"})
	pol.pollOnce(context.Background(), fixedNow)
	p := decodePayload(t, fc.execCalls[0])
	if mid := p["bid_price"].(float64); math.Abs(mid-150.0) > 1e-6 {
		t.Errorf("bid_price = %v, want 150.0", mid)
	}
	if p["currency"] != "USD" {
		t.Errorf("currency = %v, want USD", p["currency"])
	}
}
