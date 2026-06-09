CREATE VIEW gw_classification AS
  SELECT portfolio_id   AS portfolio,
         instrument_id,
         updated_at      AS ts,
         sector,
         subindustry     AS industry
  FROM instrument_ticker_mapping;
