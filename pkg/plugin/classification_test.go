package plugin

import "testing"

func TestSetClassificationSource(t *testing.T) {
	t.Run("sets sector source on nil meta", func(t *testing.T) {
		got := setClassificationSource(nil, "sector", "yfinance")
		if got["sector_source"] != "yfinance" {
			t.Fatalf("sector_source = %v, want yfinance", got["sector_source"])
		}
		if _, ok := got["industry_source"]; ok {
			t.Fatalf("industry_source should be untouched")
		}
	})

	t.Run("overwrites only the named field, preserves others", func(t *testing.T) {
		in := map[string]any{"sector_source": "yfinance", "subscribed": true}
		got := setClassificationSource(in, "industry", "user")
		if got["industry_source"] != "user" {
			t.Fatalf("industry_source = %v, want user", got["industry_source"])
		}
		if got["sector_source"] != "yfinance" {
			t.Fatalf("sector_source clobbered: %v", got["sector_source"])
		}
		if got["subscribed"] != true {
			t.Fatalf("subscribed clobbered: %v", got["subscribed"])
		}
	})

	t.Run("user overrides a prior yfinance source", func(t *testing.T) {
		in := map[string]any{"sector_source": "yfinance"}
		got := setClassificationSource(in, "sector", "user")
		if got["sector_source"] != "user" {
			t.Fatalf("sector_source = %v, want user", got["sector_source"])
		}
	})
}
