package plugin

import (
	"time"

	"github.com/opencapital-dev/oc-plugin-sdk/pluginclient"
)

// colIndex maps a Result's column names to their positions.
func colIndex(cols []pluginclient.Column) map[string]int {
	idx := make(map[string]int, len(cols))
	for i, c := range cols {
		idx[c.Name] = i
	}
	return idx
}

// rwString extracts a string cell from a pgx-native result row.
func rwString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// rwInt extracts an integer cell (int64 or int32 from pgx).
func rwInt(v any) int {
	switch n := v.(type) {
	case int64:
		return int(n)
	case int32:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}

// rwMicros extracts a bigint epoch-microseconds cell.
// RisingWave returns bigint as int64; timestamps return as time.Time.
func rwMicros(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int32:
		return int64(n)
	case float64:
		return int64(n)
	case time.Time:
		return n.UnixMicro()
	}
	return 0
}
