-- Track 2c: ticker map keyed per (instrument_id, portfolio_id) — instrument_id
-- is a broker-local ticker that can mean different securities across portfolios.
-- sector/subindustry are plugin-private enrichment (control_db.instruments,
-- which used to hold them, was dropped). Dev data disposable -> recreate.
DROP TABLE IF EXISTS instrument_ticker_mapping;
CREATE TABLE instrument_ticker_mapping (
    instrument_id  TEXT    NOT NULL,
    portfolio_id   TEXT    NOT NULL,
    symbol         TEXT    NOT NULL,
    sector         TEXT,
    subindustry    TEXT,
    vendor_meta    TEXT    NOT NULL DEFAULT '{}',
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL,
    updated_by     TEXT,
    PRIMARY KEY (instrument_id, portfolio_id)
) WITHOUT ROWID;
CREATE INDEX instrument_ticker_mapping_symbol_idx     ON instrument_ticker_mapping (symbol);
CREATE INDEX instrument_ticker_mapping_updated_at_idx ON instrument_ticker_mapping (updated_at);
CREATE INDEX instrument_ticker_mapping_portfolio_idx  ON instrument_ticker_mapping (portfolio_id);
