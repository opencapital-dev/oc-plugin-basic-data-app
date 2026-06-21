PNL_COLS = ["realized_equity_avg_base", "realized_forex_avg_base",
            "unrealized_equity_avg_base", "unrealized_forex_avg_base"]

@bind(
    positions=("SELECT DISTINCT ON (portfolio, instrument) * FROM e_instrument "
               "WHERE portfolio=$1 AND quantity != $2 "
               "ORDER BY portfolio, instrument, ts DESC", "$portfolio_id", 0),
    sectors=pg("SELECT DISTINCT ON (portfolio, instrument_id) * FROM yfinance.gw_classification "
               "WHERE portfolio=$1 ORDER BY portfolio, instrument_id, ts DESC", "$portfolio_id"),
)
@metric(output="table")
def pnl_by_sector(positions, sectors):
    df = positions.join(sectors.select("instrument_id", "sector"),
                        left_on="instrument", right_on="instrument_id", how="left")
    df = df.with_columns(
        pl.sum_horizontal(pl.col(PNL_COLS)).alias("pnl"),
        pl.col("sector").fill_null("Unclassified"),
    )
    return (df.group_by("sector").agg(pl.col("pnl").sum())
              .sort("pnl", descending=True)
              .rename({"sector": "Sector", "pnl": "Current PnL"}))
