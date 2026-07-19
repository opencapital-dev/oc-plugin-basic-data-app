# yfinance → oc-plugin-sdk v0.2.0 migration plan

Hard cutover, single-user local, no back-compat. yfinance ingests prices (backfill OHLCV + live quotes) and keeps ticker-mapping/classification state. v0.2.0 replaced gateway publish + read-gateway/DSL + SQLite + JWT with direct pgwire `Client.Exec`/`Query` (RisingWave) + `Client.PGExec`/`PGQuery` (Postgres) + `datakey`. Identity is `{PluginID}` only; loopback trust.

## State store: SQLite → Postgres (schema `yfinance` in control_db)

The plugin owns a Postgres schema `yfinance` (self-created on startup; the Spec-B host reconciler will later own schema/role provisioning — `CREATE … IF NOT EXISTS` stays forward-compatible). On startup run via `PGExec`:
```sql
CREATE SCHEMA IF NOT EXISTS yfinance;
CREATE TABLE IF NOT EXISTS yfinance.instrument_ticker_mapping (
    instrument_id VARCHAR NOT NULL,
    portfolio_id  VARCHAR NOT NULL,
    symbol        VARCHAR NOT NULL,
    sector        VARCHAR,
    subindustry   VARCHAR,
    vendor_meta   JSONB NOT NULL DEFAULT '{}'::jsonb,
    subscribed    BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    BIGINT NOT NULL,
    updated_at    BIGINT NOT NULL,
    updated_by    VARCHAR,
    PRIMARY KEY (instrument_id, portfolio_id)
);
CREATE INDEX IF NOT EXISTS itm_symbol_idx ON yfinance.instrument_ticker_mapping(symbol);
CREATE INDEX IF NOT EXISTS itm_updated_idx ON yfinance.instrument_ticker_mapping(updated_at);
CREATE OR REPLACE VIEW yfinance.gw_classification AS
  SELECT portfolio_id AS portfolio, instrument_id, updated_at AS ts, sector, subindustry AS industry
  FROM yfinance.instrument_ticker_mapping;
```
(Note: the v0.1 SQLite filtered `subscribed != false` in code; promote it to a real `subscribed BOOLEAN` column to keep `ListSubscribed` a clean WHERE.)

Rewrite the 6 `sqlite_repo.go` methods to `PGQuery`/`PGExec` against `yfinance.instrument_ticker_mapping` (same semantics):
- `UpsertTickerMapping` → `INSERT … ON CONFLICT (instrument_id, portfolio_id) DO UPDATE`
- `SetClassification` → `UPDATE … SET sector,subindustry,vendor_meta,updated_at,updated_by`
- `GetTickerMapping` → `SELECT … WHERE instrument_id=$1 AND portfolio_id=$2`
- `ListSubscribedTickerMappings` → `SELECT … WHERE subscribed ORDER BY portfolio_id, instrument_id`
- `ListTickerMappings` → `SELECT … ORDER BY portfolio_id, instrument_id`
Delete `sqlite_repo.go`'s `openSQLite`, `sqlite_migrations.go`, `migrations/`.

## Writes → RisingWave `data_log` (Exec)

Both writes INSERT into `data_log` (cols: `source_namespace, source_id, portfolio_id, observed_at, ingest_ts, source, plugin_id, trace_id, payload, rw_key`), `rw_key = datakey.DataKey(a.pluginID, namespace, portfolioID, sourceID, observedAtMicros)`, `plugin_id = a.pluginID`, `ingest_ts = now()`, `observed_at = to_timestamp($/1e6)`:
- **Backfill OHLCV** (`backfill_worker.go:182`): namespace `prices.ohlcv`, source `"yfinance"`, payload {open,high,low,close,volume,trade_count,bar_cadence,currency,venue}.
- **Live quote** (`live.go:217`): namespace `prices.quote`, source `"yahoo_ws"`, payload {bid_price,ask_price,bid_size,ask_size,currency,venue}.

**Tombstone** (backfill, before re-backfilling an instrument): replace the `ReadGatewayQuery(ohlcv_coverage{instrument}@window)` + per-row `PublishTombstones` with ONE delete:
```sql
DELETE FROM data_log WHERE source_namespace='prices.ohlcv' AND source_id=$1 AND portfolio_id=$2
```
This drops the `ohlcv_coverage{instrument}@window` read entirely.

## Reads → RisingWave (Query), org_id removed

Replace the 5 `ReadGatewayQuery` selectors with `Client.Query(sql, args...)`. Verify exact view names + columns against `/Users/ignacioballester/trading-code/opencapital/dataplane/risingwave/schemas/08-ingestor-discovery/*.sql` (org_id dropped by opencapital A2; scope by portfolio_id):
- `fx_pairs_used` → `SELECT base_ccy, quote_ccy, first_seen_ts, last_seen_ts, event_count FROM fx_pairs_used`
- `ohlcv_coverage` (all) → `SELECT source_id, observed_at FROM ohlcv_coverage`
- `data_coverage` (all) → `SELECT source_id, observed_at FROM data_coverage`
- `instruments_used` (discovery + rw_repo) → `SELECT portfolio_id, instrument_id, first_seen_ts FROM instruments_used` (+ kind/currency/base_currency if present in the view — check `03-instruments_catalog.sql`; use `instruments_catalog` if those cols live there)
- (the `ohlcv_coverage{instrument}@window` tombstone read is deleted — see above)

`Client.Query` returns `Result{Columns []Column, Rows [][]any}` — values are pgx-native Go types (int64/float64/string/time), NOT JSON. Replace the `asString/asMicros/asInt` JSON-coercion helpers (`readgateway_helpers.go`) with direct type assertions on `Rows[i][j]`; keep a small `colIndex(Columns)` helper. Delete `readgateway_helpers.go`'s JSON-specific bits + `rw_repo.go`'s ReadGatewayRequest usage.

## Identity/auth — delete

Remove all 7 `client.WithRequest(...)` calls (app.go, backfill_worker.go, discovery.go, live.go, handlers). Loopback trust: workers/handlers use the shared `a.client` directly. `live.go` `publishCtx` no longer stores a bearer — pass a plain context or the client. Drop `IdentityFrom`/`SessionJWT`/`OrgID`. Add `a.pluginID` (from `client.Config().PluginID`) for rw_key/plugin_id.

## go.mod + settings

`github.com/ignacioballester/oc-plugin-sdk v0.1.0` → `github.com/opencapital-dev/oc-plugin-sdk v0.2.0` + `replace … => ../oc-plugin-sdk`; change import paths; `go mod tidy`. SDK `NewFromSettings` parses RW + PG coords from jsonData; yfinance `AppOptions` (poll/qps/burst/liveEnable/backfillEnable) stay.

## Verify
`go build ./... && go vet ./... && go test ./...` green. Straggler grep empty: `PublishData|PublishTombstones|ReadGatewayQuery|WithRequest|OpenDB|IdentityFrom|SessionJWT|OrgID|ignacioballester|sqlite`. Tests: cover the PG repo SQL + the data_log write SQL/rw_key + the read SELECT mapping (fake the SDK Client's Exec/Query/PGExec/PGQuery via a small interface seam).
