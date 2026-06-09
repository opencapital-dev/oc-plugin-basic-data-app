package plugin

import (
	"net/http"
	"strconv"
)

// JobRow is the slim wire shape the frontend consumes for /yf/jobs.
type JobRow struct {
	JobID         string  `json:"job_id"`
	InstrumentID  string  `json:"instrument_id"`
	PortfolioID   string  `json:"portfolio_id"`
	BarSize       string  `json:"bar_size"`
	StartTs       int64   `json:"start_ts"`
	EndTs         int64   `json:"end_ts"`
	Status        string  `json:"status"`
	Error         *string `json:"error"`
	RowsPublished *int    `json:"rows_published"`
	CreatedAt     int64   `json:"created_at"`
	FinishedAt    *int64  `json:"finished_at"`
	Origin        string  `json:"origin"`
}

func (a *App) handleListJobs(w http.ResponseWriter, r *http.Request) {
	if !methodGuard(w, r, http.MethodGet) {
		return
	}
	instrumentID := r.URL.Query().Get("instrument_id")
	status := r.URL.Query().Get("status")
	limit := 500
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 5000 {
		limit = 5000
	}

	pending, running, results := a.jobs.Snapshot()
	rows := make([]JobRow, 0, len(pending)+len(running)+len(results))
	for _, j := range running {
		rows = append(rows, toJobRow(j))
	}
	for _, j := range pending {
		rows = append(rows, toJobRow(j))
	}
	// Most-recent terminal results first.
	for i := len(results) - 1; i >= 0; i-- {
		rows = append(rows, toJobRow(results[i]))
	}
	if instrumentID != "" {
		filtered := rows[:0]
		for _, r := range rows {
			if r.InstrumentID == instrumentID {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}
	if status != "" {
		filtered := rows[:0]
		for _, r := range rows {
			if r.Status == status {
				filtered = append(filtered, r)
			}
		}
		rows = filtered
	}
	if len(rows) > limit {
		rows = rows[:limit]
	}
	respondJSON(w, http.StatusOK, rows)
}

func toJobRow(j *BackfillJob) JobRow {
	return JobRow{
		JobID:         j.JobID,
		InstrumentID:  j.InstrumentID,
		PortfolioID:   j.PortfolioID,
		BarSize:       j.BarSize,
		StartTs:       j.StartTsUs,
		EndTs:         j.EndTsUs,
		Status:        j.Status,
		Error:         j.Error,
		RowsPublished: j.RowsPublished,
		CreatedAt:     j.EnqueuedAtUs,
		FinishedAt:    j.FinishedAtUs,
		Origin:        j.Origin,
	}
}
