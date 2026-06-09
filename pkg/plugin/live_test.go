package plugin

import (
	"testing"
	"time"
)

func TestQuoteObservedMicros(t *testing.T) {
	cases := []struct {
		name   string
		timeMs int64
		now    time.Time
		check  func(t *testing.T, result int64, timeMs int64, now time.Time)
	}{
		{
			name:   "2024-12-05 ms value converts to microseconds",
			timeMs: 1_733_400_000_000,
			now:    time.Time{},
			check: func(t *testing.T, result int64, timeMs int64, _ time.Time) {
				if result != timeMs*1_000 {
					t.Fatalf("result = %d, want %d (input*1000)", result, timeMs*1_000)
				}
				gotYear := time.UnixMicro(result).UTC().Year()
				wantYear := time.UnixMilli(timeMs).UTC().Year()
				if gotYear != wantYear {
					t.Fatalf("year = %d, want %d", gotYear, wantYear)
				}
			},
		},
		{
			name:   "zero timeMs falls back to now",
			timeMs: 0,
			now:    time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC),
			check: func(t *testing.T, result int64, _ int64, now time.Time) {
				if result != now.UnixMicro() {
					t.Fatalf("result = %d, want %d (now.UnixMicro)", result, now.UnixMicro())
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := quoteObservedMicros(tc.timeMs, tc.now)
			tc.check(t, result, tc.timeMs, tc.now)
		})
	}
}
