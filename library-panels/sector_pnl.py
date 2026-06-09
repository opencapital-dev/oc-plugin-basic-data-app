PNL_COLS = ["realized_equity_avg_base", "realized_forex_avg_base",
            "unrealized_equity_avg_base", "unrealized_forex_avg_base"]

@bind(
    positions="instrument{portfolio=\"$portfolio_id\", quantity != 0} @latest",
    sectors  ="yfinance-app/classification{portfolio=\"$portfolio_id\"} @latest",
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
