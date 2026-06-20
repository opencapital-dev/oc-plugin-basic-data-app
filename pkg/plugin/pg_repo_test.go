package plugin

import (
	"context"
	"strings"
	"testing"

	"github.com/opencapital-dev/oc-plugin-sdk/pluginclient"
)

func makeAppWithFakeClient(fc *fakeClient) *App {
	return &App{
		client:   fc,
		pluginID: "test-plugin",
	}
}

func TestUpsertTickerMappingSQL(t *testing.T) {
	fc := &fakeClient{
		pgQueryResult: pluginclient.Result{
			Columns: []pluginclient.Column{
				{Name: "instrument_id"}, {Name: "portfolio_id"}, {Name: "symbol"},
				{Name: "sector"}, {Name: "subindustry"}, {Name: "vendor_meta"},
				{Name: "subscribed"}, {Name: "created_at"}, {Name: "updated_at"}, {Name: "updated_by"},
			},
			Rows: [][]any{
				{"instr-1", "port-1", "AAPL", nil, nil, map[string]interface{}{}, true, int64(1000), int64(1000), nil},
			},
		},
	}
	app := makeAppWithFakeClient(fc)
	ctx := context.Background()
	_, err := app.UpsertTickerMapping(ctx, "instr-1", "port-1", "AAPL", nil, "test")
	if err != nil {
		t.Fatalf("UpsertTickerMapping: %v", err)
	}
	if len(fc.pgExecCalls) != 1 {
		t.Fatalf("expected 1 PGExec call, got %d", len(fc.pgExecCalls))
	}
	sql := fc.pgExecCalls[0].sql
	if !strings.Contains(sql, "yfinance.instrument_ticker_mapping") {
		t.Errorf("SQL missing table name: %s", sql)
	}
	if !strings.Contains(sql, "ON CONFLICT") {
		t.Errorf("SQL missing ON CONFLICT: %s", sql)
	}
	args := fc.pgExecCalls[0].args
	if args[0] != "instr-1" {
		t.Errorf("arg[0] instrument_id = %v, want instr-1", args[0])
	}
	if args[1] != "port-1" {
		t.Errorf("arg[1] portfolio_id = %v, want port-1", args[1])
	}
	if args[2] != "AAPL" {
		t.Errorf("arg[2] symbol = %v, want AAPL", args[2])
	}
}

func TestGetTickerMappingNotFound(t *testing.T) {
	fc := &fakeClient{
		pgQueryResult: pluginclient.Result{Columns: []pluginclient.Column{{Name: "instrument_id"}}, Rows: nil},
	}
	app := makeAppWithFakeClient(fc)
	_, err := app.GetTickerMapping(context.Background(), "x", "y")
	if err != errNotFound {
		t.Fatalf("expected errNotFound, got %v", err)
	}
}

func TestEnsureSchema(t *testing.T) {
	fc := &fakeClient{}
	app := makeAppWithFakeClient(fc)
	app.ensureSchema(context.Background())

	if len(fc.pgExecCalls) < 2 {
		t.Fatalf("expected at least 2 PGExec calls (CREATE SCHEMA + CREATE TABLE ...), got %d", len(fc.pgExecCalls))
	}

	// First call must be CREATE SCHEMA IF NOT EXISTS yfinance.
	firstSQL := fc.pgExecCalls[0].sql
	if !strings.Contains(firstSQL, "CREATE SCHEMA IF NOT EXISTS yfinance") {
		t.Errorf("first PGExec call missing CREATE SCHEMA IF NOT EXISTS yfinance: %s", firstSQL)
	}

	// One of the calls must create the instrument_ticker_mapping table.
	foundTable := false
	for _, c := range fc.pgExecCalls {
		if strings.Contains(c.sql, "instrument_ticker_mapping") {
			foundTable = true
			break
		}
	}
	if !foundTable {
		t.Error("no PGExec call references instrument_ticker_mapping")
	}
}

func TestListSubscribedTickerMappingsSQL(t *testing.T) {
	fc := &fakeClient{
		pgQueryResult: pluginclient.Result{
			Columns: []pluginclient.Column{
				{Name: "instrument_id"}, {Name: "portfolio_id"}, {Name: "symbol"},
				{Name: "sector"}, {Name: "subindustry"}, {Name: "vendor_meta"},
				{Name: "subscribed"}, {Name: "created_at"}, {Name: "updated_at"}, {Name: "updated_by"},
			},
			Rows: [][]any{
				{"instr-1", "port-1", "AAPL", nil, nil, map[string]interface{}{}, true, int64(1000), int64(1000), nil},
			},
		},
	}
	app := makeAppWithFakeClient(fc)
	rows, err := app.ListSubscribedTickerMappings(context.Background())
	if err != nil {
		t.Fatalf("ListSubscribedTickerMappings: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	sql := fc.pgQueryCalls[0].sql
	if !strings.Contains(sql, "WHERE subscribed") {
		t.Errorf("SQL missing WHERE subscribed: %s", sql)
	}
}
