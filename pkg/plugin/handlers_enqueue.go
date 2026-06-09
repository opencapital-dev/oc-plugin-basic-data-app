package plugin

import (
	"net/http"
)

type enqueueRequest struct {
	InstrumentID string `json:"instrument_id"`
	PortfolioID  string `json:"portfolio_id"`
	BarSize      string `json:"bar_size"`
	Start        int64  `json:"start"`
	End          int64  `json:"end"`
}

// handleEnqueueJob: POST /yf/jobs/enqueue. Operator-triggered backfill.
// Instrument-existence is implicit — without a mapping the worker fails
// immediately. We surface that as a 404 here to match the old behaviour.
func (a *App) handleEnqueueJob(w http.ResponseWriter, r *http.Request) {
	if !methodGuard(w, r, http.MethodPost) {
		return
	}
	body, ok := decodeJSON[enqueueRequest](w, r)
	if !ok {
		return
	}
	if body.InstrumentID == "" {
		respondErr(w, http.StatusBadRequest, "instrument_id required")
		return
	}
	if body.PortfolioID == "" {
		respondErr(w, http.StatusBadRequest, "portfolio_id required")
		return
	}
	if body.BarSize == "" {
		body.BarSize = "1d"
	}
	if body.Start <= 0 || body.End <= 0 || body.Start >= body.End {
		respondErr(w, http.StatusUnprocessableEntity, "start must be < end")
		return
	}
	if mapBarSize(body.BarSize) == "" {
		respondErr(w, http.StatusUnprocessableEntity, "unsupported bar size: "+body.BarSize)
		return
	}
	ctx, ok := a.handlerCtx(w, r)
	if !ok {
		return
	}
	if _, err := a.GetTickerMapping(ctx, body.InstrumentID, body.PortfolioID); err != nil {
		if err == errNotFound {
			respondErr(w, http.StatusNotFound, "instrument not mapped for this portfolio; POST /yf/symbols/{id} first")
			return
		}
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	job := &BackfillJob{
		JobID:        genID(),
		InstrumentID: body.InstrumentID,
		PortfolioID:  body.PortfolioID,
		BarSize:      body.BarSize,
		StartTsUs:    body.Start,
		EndTsUs:      body.End,
		Origin:       "manual",
		EnqueuedAtUs: nowMicros(),
	}
	a.jobs.Enqueue(job)
	respondJSON(w, http.StatusCreated, toJobRow(job))
}
