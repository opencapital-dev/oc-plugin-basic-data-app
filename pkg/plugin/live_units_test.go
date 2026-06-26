package plugin

import (
	"encoding/json"
	"math"
	"testing"

	yfmodels "github.com/wnjoon/go-yfinance/pkg/models"
)

// The Yahoo live ws fills `currency` unreliably for LSE minor-unit listings
// (arrives "USD"/empty while the price itself is in pence), and intermittently
// flips a single symbol between pence (176) and pounds (1.76) tick to tick.
// The live path must mirror the backfill classifier: prefer the authoritative
// currency captured at backfill and decide minor/major per tick against a
// reference price — never trust the ws currency field.

func TestLiveUnitPrefersAuthoritativeCurrency(t *testing.T) {
	cases := []struct {
		name    string
		auth    string
		ws      string
		wantCur string
		wantDiv float64
	}{
		{"authoritative GBp beats ws USD", "GBp", "USD", "GBP", 100.0},
		{"authoritative GBX beats empty ws", "GBX", "", "GBP", 100.0},
		{"authoritative USD beats stray ws GBp", "USD", "GBp", "USD", 1.0},
		{"no authoritative falls back to ws GBp", "", "GBp", "GBP", 100.0},
		{"no authoritative, ws USD", "", "USD", "USD", 1.0},
		{"both empty default USD", "", "", "USD", 1.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cur, div := liveUnit(tc.auth, tc.ws)
			if cur != tc.wantCur || div != tc.wantDiv {
				t.Fatalf("liveUnit(%q,%q) = (%q,%v), want (%q,%v)",
					tc.auth, tc.ws, cur, div, tc.wantCur, tc.wantDiv)
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

func liveSubWithTarget(fc *fakeClient, symbol string, tgt symbolTarget) *LiveSubscriber {
	return &LiveSubscriber{
		client:   fc,
		pluginID: "basic-data-app",
		current:  map[string]struct{}{},
		bySymbol: map[string][]symbolTarget{symbol: {tgt}},
		ticks:    NewLiveTickMap(),
	}
}

// Regression for the NAV-to-3M bug: a GKP pence tick (176) arrives tagged
// currency=USD from the ws. With the authoritative GBp currency + reference
// captured at backfill, the tick must be divided to 1.76 and stamped GBP.
func TestPublishTickNormalizesPenceFromBadWsCurrency(t *testing.T) {
	const sym = "GKP.L"
	fc := &fakeClient{}
	sub := liveSubWithTarget(fc, sym, symbolTarget{
		InstrumentID: "GKP", PortfolioID: "p", Currency: "GBp", RefPrice: 176.9,
	})
	sub.publishTick(&yfmodels.PricingData{
		ID: sym, Time: 1_733_400_000_000,
		Price: 176.9, Bid: 176.8, Ask: 177.0,
		Currency: "USD", // wrong/unreliable ws currency
	})
	if len(fc.execCalls) != 1 {
		t.Fatalf("want 1 insert, got %d", len(fc.execCalls))
	}
	p := decodePayload(t, fc.execCalls[0])
	if p["currency"] != "GBP" {
		t.Errorf("currency = %v, want GBP", p["currency"])
	}
	if bid := p["bid_price"].(float64); math.Abs(bid-1.768) > 1e-6 {
		t.Errorf("bid_price = %v, want ~1.768 (pence/100)", bid)
	}
	if ask := p["ask_price"].(float64); math.Abs(ask-1.77) > 1e-6 {
		t.Errorf("ask_price = %v, want ~1.77", ask)
	}
}

// The same symbol intermittently sends an already-major pound tick (1.76 / GBP).
// It must NOT be divided again into 0.0176.
func TestPublishTickIntermittentPoundTickNotDivided(t *testing.T) {
	const sym = "GKP.L"
	fc := &fakeClient{}
	sub := liveSubWithTarget(fc, sym, symbolTarget{
		InstrumentID: "GKP", PortfolioID: "p", Currency: "GBp", RefPrice: 176.9,
	})
	sub.publishTick(&yfmodels.PricingData{
		ID: sym, Time: 1_733_400_000_000,
		Price: 1.762, Bid: 1.761, Ask: 1.763,
		Currency: "GBP",
	})
	p := decodePayload(t, fc.execCalls[0])
	if bid := p["bid_price"].(float64); math.Abs(bid-1.761) > 1e-6 {
		t.Errorf("bid_price = %v, want 1.761 (unchanged major tick)", bid)
	}
	if p["currency"] != "GBP" {
		t.Errorf("currency = %v, want GBP", p["currency"])
	}
}

// A USD instrument with no canonical unit must pass through unchanged.
func TestPublishTickUSDPassthrough(t *testing.T) {
	const sym = "AAPL"
	fc := &fakeClient{}
	sub := liveSubWithTarget(fc, sym, symbolTarget{InstrumentID: "AAPL", PortfolioID: "p"})
	sub.publishTick(&yfmodels.PricingData{
		ID: sym, Time: 1_733_400_000_000,
		Price: 150.0, Bid: 149.9, Ask: 150.1, Currency: "USD",
	})
	p := decodePayload(t, fc.execCalls[0])
	// float32 ws fields → float64 carries ~1e-5 relative error; tolerate it.
	if bid := p["bid_price"].(float64); math.Abs(bid-149.9) > 1e-3 {
		t.Errorf("bid_price = %v, want 149.9", bid)
	}
	if p["currency"] != "USD" {
		t.Errorf("currency = %v, want USD", p["currency"])
	}
}
