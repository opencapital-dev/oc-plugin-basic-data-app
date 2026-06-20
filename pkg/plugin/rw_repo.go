package plugin

import (
	"context"
	"fmt"
)

type RwFxPairUsedRow struct {
	BaseCcy     string `json:"base_ccy"`
	QuoteCcy    string `json:"quote_ccy"`
	FirstSeenTs int64  `json:"first_seen_ts"`
	LastSeenTs  int64  `json:"last_seen_ts"`
	EventCount  int    `json:"event_count"`
}

func (a *App) ListFxPairsUsed(ctx context.Context) ([]RwFxPairUsedRow, error) {
	res, err := a.client.Query(ctx,
		`SELECT base_ccy, quote_ccy, first_seen_ts, last_seen_ts, event_count FROM fx_pairs_used`)
	if err != nil {
		return nil, fmt.Errorf("list fx_pairs_used: %w", err)
	}
	col := colIndex(res.Columns)
	out := make([]RwFxPairUsedRow, 0, len(res.Rows))
	for _, row := range res.Rows {
		out = append(out, RwFxPairUsedRow{
			BaseCcy:     rwString(row[col["base_ccy"]]),
			QuoteCcy:    rwString(row[col["quote_ccy"]]),
			FirstSeenTs: rwMicros(row[col["first_seen_ts"]]),
			LastSeenTs:  rwMicros(row[col["last_seen_ts"]]),
			EventCount:  rwInt(row[col["event_count"]]),
		})
	}
	return out, nil
}

func (a *App) LastObservedPerInstrument(ctx context.Context) (map[string]int64, error) {
	res, err := a.client.Query(ctx,
		`SELECT source_id, observed_at FROM ohlcv_coverage`)
	if err != nil {
		return nil, fmt.Errorf("last observed: %w", err)
	}
	col := colIndex(res.Columns)
	out := map[string]int64{}
	for _, row := range res.Rows {
		sid := rwString(row[col["source_id"]])
		ts := rwMicros(row[col["observed_at"]])
		if existing, ok := out[sid]; !ok || ts > existing {
			out[sid] = ts
		}
	}
	return out, nil
}

func (a *App) LastDataPerInstrument(ctx context.Context) (map[string]int64, error) {
	res, err := a.client.Query(ctx,
		`SELECT source_id, observed_at FROM data_coverage`)
	if err != nil {
		return nil, fmt.Errorf("last data: %w", err)
	}
	col := colIndex(res.Columns)
	out := map[string]int64{}
	for _, row := range res.Rows {
		sid := rwString(row[col["source_id"]])
		ts := rwMicros(row[col["observed_at"]])
		if existing, ok := out[sid]; !ok || ts > existing {
			out[sid] = ts
		}
	}
	return out, nil
}

func (a *App) MinBusinessTs(ctx context.Context) (*int64, error) {
	res, err := a.client.Query(ctx,
		`SELECT portfolio_id, instrument_id, first_seen_ts FROM instruments_catalog`)
	if err != nil {
		return nil, fmt.Errorf("min business_ts: %w", err)
	}
	col := colIndex(res.Columns)
	var min *int64
	for _, row := range res.Rows {
		us := rwMicros(row[col["first_seen_ts"]])
		if us == 0 {
			continue
		}
		if min == nil || us < *min {
			v := us
			min = &v
		}
	}
	return min, nil
}
