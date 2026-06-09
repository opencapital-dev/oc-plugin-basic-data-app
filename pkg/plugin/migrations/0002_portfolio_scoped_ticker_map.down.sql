-- Revert to single-keyed table (pre-Track-2c schema).
DROP TABLE IF EXISTS instrument_ticker_mapping;
CREATE TABLE instrument_ticker_mapping (
    instrument_id  TEXT    PRIMARY KEY,
    symbol         TEXT    NOT NULL,
    vendor_meta    TEXT    NOT NULL DEFAULT '{}',
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL,
    updated_by     TEXT
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS instrument_ticker_mapping_symbol_idx
    ON instrument_ticker_mapping (symbol);
CREATE INDEX IF NOT EXISTS instrument_ticker_mapping_updated_at_idx
    ON instrument_ticker_mapping (updated_at);
