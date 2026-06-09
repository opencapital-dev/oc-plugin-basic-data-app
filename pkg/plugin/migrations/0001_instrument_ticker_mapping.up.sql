-- v6 Phase 3: yfinance plugin-private state in per-(plugin, org) SQLite.
--
-- Pre-v6 this table lived in Postgres (and was preceded by a JSONB subtree
-- on `instruments.attributes`). The control plane now owns instruments;
-- the Yahoo-symbol mapping is plugin-private and per-org, so it moves
-- into SQLite. Column shapes mirror the pre-v6 Postgres schema with
-- SQLite-native types — vendor_meta is JSON text (no JSONB), timestamps
-- are integer microseconds since epoch (no TIMESTAMPTZ).
CREATE TABLE IF NOT EXISTS instrument_ticker_mapping (
    instrument_id  TEXT    PRIMARY KEY,
    symbol         TEXT    NOT NULL,
    vendor_meta    TEXT    NOT NULL DEFAULT '{}',
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL,
    updated_by     TEXT
) WITHOUT ROWID;

-- Symbol lookups in the live-WS loop hit this index.
CREATE INDEX IF NOT EXISTS instrument_ticker_mapping_symbol_idx
    ON instrument_ticker_mapping (symbol);

-- Discovery loop polls WHERE updated_at > :cursor to detect changed
-- mappings without a command channel.
CREATE INDEX IF NOT EXISTS instrument_ticker_mapping_updated_at_idx
    ON instrument_ticker_mapping (updated_at);
