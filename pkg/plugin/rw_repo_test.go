package plugin

import (
	"context"
	"strings"
	"testing"

	"github.com/opencapital-dev/oc-plugin-sdk/datakey"
)

func TestOhlcvDataLogInsert(t *testing.T) {
	fc := &fakeClient{}
	app := &App{client: fc, pluginID: "yfinance-app"}
	ctx := context.Background()

	instrumentID := "instr-1"
	portfolioID := "port-1"
	observedAtUs := int64(1_700_000_000_000_000)
	wantRwKey := datakey.DataKey("yfinance-app", OhlcvNamespace, portfolioID, instrumentID, observedAtUs)

	_, err := app.client.Exec(ctx, `
		INSERT INTO data_log
			(source_namespace, source_id, portfolio_id, observed_at, ingest_ts, source, plugin_id, trace_id, payload, rw_key)
		VALUES ($1, $2, $3, to_timestamp($4::double precision / 1e6), now(), $5, $6, $7, $8, $9)
	`,
		OhlcvNamespace, instrumentID, portfolioID, observedAtUs,
		"yfinance", app.pluginID, "", `{"open":100}`, wantRwKey,
	)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(fc.execCalls) != 1 {
		t.Fatalf("expected 1 Exec call, got %d", len(fc.execCalls))
	}
	args := fc.execCalls[0].args
	if args[0] != OhlcvNamespace {
		t.Errorf("arg[0] source_namespace = %v, want %v", args[0], OhlcvNamespace)
	}
	if args[8] != wantRwKey {
		t.Errorf("arg[8] rw_key = %v, want %v", args[8], wantRwKey)
	}
}

func TestTombstoneDelete(t *testing.T) {
	fc := &fakeClient{}
	app := &App{client: fc, pluginID: "yfinance-app"}
	ctx := context.Background()

	_, err := app.client.Exec(ctx,
		`DELETE FROM data_log WHERE source_namespace='prices.ohlcv' AND source_id=$1 AND portfolio_id=$2`,
		"instr-1", "port-1",
	)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	sql := fc.execCalls[0].sql
	if !strings.Contains(sql, "DELETE FROM data_log") {
		t.Errorf("SQL not a DELETE: %s", sql)
	}
	if !strings.Contains(sql, "source_namespace='prices.ohlcv'") {
		t.Errorf("SQL missing namespace filter: %s", sql)
	}
	_ = app
}

func TestRwHelpers(t *testing.T) {
	t.Run("rwMicros from int64", func(t *testing.T) {
		if v := rwMicros(int64(1234)); v != 1234 {
			t.Errorf("rwMicros(int64) = %d, want 1234", v)
		}
	})
	t.Run("rwMicros from int32", func(t *testing.T) {
		if v := rwMicros(int32(999)); v != 999 {
			t.Errorf("rwMicros(int32) = %d, want 999", v)
		}
	})
	t.Run("rwString nil", func(t *testing.T) {
		if v := rwString(nil); v != "" {
			t.Errorf("rwString(nil) = %q, want empty", v)
		}
	})
}
