package plugin

import "sync"

// LiveTickMap is a thread-safe snapshot of the most recent tick microseconds
// per instrument_id, mirroring the Python `live_ingestor.snapshot_last_tick_at_us()`.
// The live goroutine writes; the /yf/instruments handler reads.
type LiveTickMap struct {
	mu sync.RWMutex
	m  map[string]int64
}

func NewLiveTickMap() *LiveTickMap {
	return &LiveTickMap{m: map[string]int64{}}
}

func (t *LiveTickMap) Set(instrumentID string, micros int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.m[instrumentID] = micros
}

func (t *LiveTickMap) Get(instrumentID string) (int64, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	v, ok := t.m[instrumentID]
	return v, ok
}

func (t *LiveTickMap) Snapshot() map[string]int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make(map[string]int64, len(t.m))
	for k, v := range t.m {
		out[k] = v
	}
	return out
}
