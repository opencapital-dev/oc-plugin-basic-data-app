package plugin

import (
	"sync"
)

// BackfillJob is one unit of bar-download work the worker pops off the
// queue. Field shape mirrors the Python BackfillJob so the frontend can
// keep its existing types.
type BackfillJob struct {
	JobID         string  `json:"job_id"`
	InstrumentID  string  `json:"instrument_id"`
	PortfolioID   string  `json:"portfolio_id"`
	BarSize       string  `json:"bar_size"`
	StartTsUs     int64   `json:"start_ts"`
	EndTsUs       int64   `json:"end_ts"`
	Origin        string  `json:"origin"`        // discovery | manual
	EnqueuedAtUs  int64   `json:"created_at"`
	StartedAtUs   *int64  `json:"started_at"`
	FinishedAtUs  *int64  `json:"finished_at"`
	Status        string  `json:"status"` // pending | running | done | failed
	Error         *string `json:"error"`
	RowsPublished *int    `json:"rows_published"`
}

// pairKey returns the composite key used in the running map.
func pairKey(instrumentID, portfolioID string) string {
	return instrumentID + "|" + portfolioID
}

// BackfillState is the in-memory queue + lifecycle ledger the worker
// goroutine and the /yf/jobs HTTP handler share. Mirrors
// services/ingestor-yfinance/backfill_queue.py one-to-one in spirit:
// pending FIFO, running snapshot, last terminal results capped.
//
// Restart-tolerant the same way Python is: state evaporates on restart
// and the discovery loop re-derives "what to backfill" from
// instrument_ticker_mapping on its first tick.
type BackfillState struct {
	mu          sync.Mutex
	pending     []*BackfillJob
	running     map[string]*BackfillJob // pairKey(instrument_id, portfolio_id) → job
	lastResults []*BackfillJob          // bounded ring buffer
	maxResults  int
}

func NewBackfillState() *BackfillState {
	return &BackfillState{
		running:    map[string]*BackfillJob{},
		maxResults: 200,
	}
}

func (s *BackfillState) Enqueue(job *BackfillJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job.Status = "pending"
	s.pending = append(s.pending, job)
}

// Pop removes the head of the queue. Returns (nil, false) when empty.
func (s *BackfillState) Pop() (*BackfillJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) == 0 {
		return nil, false
	}
	job := s.pending[0]
	s.pending = s.pending[1:]
	return job, true
}

func (s *BackfillState) MarkRunning(job *BackfillJob, startedAtUs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job.Status = "running"
	job.StartedAtUs = &startedAtUs
	s.running[pairKey(job.InstrumentID, job.PortfolioID)] = job
}

func (s *BackfillState) MarkFinished(job *BackfillJob, status string, rowsPublished int, errMsg string, finishedAtUs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job.Status = status
	job.FinishedAtUs = &finishedAtUs
	if rowsPublished >= 0 {
		v := rowsPublished
		job.RowsPublished = &v
	}
	if errMsg != "" {
		e := errMsg
		job.Error = &e
	}
	delete(s.running, pairKey(job.InstrumentID, job.PortfolioID))
	s.lastResults = append(s.lastResults, job)
	if len(s.lastResults) > s.maxResults {
		s.lastResults = s.lastResults[len(s.lastResults)-s.maxResults:]
	}
}

// Snapshot returns flat slices for the HTTP layer. Caller must not
// mutate the returned pointers (defensive callers re-marshal anyway).
func (s *BackfillState) Snapshot() (pending, running, results []*BackfillJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	pending = append(pending, s.pending...)
	for _, j := range s.running {
		running = append(running, j)
	}
	results = append(results, s.lastResults...)
	return
}

func (s *BackfillState) RunningFor(instrumentID, portfolioID string) *BackfillJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running[pairKey(instrumentID, portfolioID)]
}

func (s *BackfillState) PendingFor(instrumentID, portfolioID string) *BackfillJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.pending {
		if j.InstrumentID == instrumentID && j.PortfolioID == portfolioID {
			return j
		}
	}
	return nil
}

func (s *BackfillState) LatestResultFor(instrumentID, portfolioID string) *BackfillJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.lastResults) - 1; i >= 0; i-- {
		j := s.lastResults[i]
		if j.InstrumentID == instrumentID && j.PortfolioID == portfolioID {
			return j
		}
	}
	return nil
}
