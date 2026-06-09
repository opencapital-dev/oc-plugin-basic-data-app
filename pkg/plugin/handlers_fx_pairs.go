package plugin

import (
	"net/http"
	"strings"
)

// FxPairUsedRow is the /yf/fx-pairs row. Composed from the RW
// fx_pairs_used MV + the plugin's local SQLite mapping (which carries
// the operator-assigned Yahoo symbol for each `FX:XXXYYY` instrument).
type FxPairUsedRow struct {
	BaseCcy                string  `json:"base_ccy"`
	QuoteCcy               string  `json:"quote_ccy"`
	FirstSeenTs            int64   `json:"first_seen_ts"`
	LastSeenTs             int64   `json:"last_seen_ts"`
	EventCount             int     `json:"event_count"`
	InstrumentID           *string `json:"instrument_id"`
	YahooSymbol            *string `json:"yahoo_symbol"`
	BackfillState          string  `json:"backfill_state"`
	LatestRunStatus        *string `json:"latest_run_status"`
	LatestRunRowsPublished *int    `json:"latest_run_rows_published"`
	LatestRunError         *string `json:"latest_run_error"`
	LatestRunFinishedAt    *int64  `json:"latest_run_finished_at"`
	LatestRunBarSize       *string `json:"latest_run_bar_size"`
}

func (a *App) handleListFxPairs(w http.ResponseWriter, r *http.Request) {
	if !methodGuard(w, r, http.MethodGet) {
		return
	}
	ctx, ok := a.handlerCtx(w, r)
	if !ok {
		return
	}
	pairs, err := a.ListFxPairsUsed(ctx)
	if err != nil {
		// RW down → degrade to empty list; the UI still renders the page.
		respondJSON(w, http.StatusOK, []FxPairUsedRow{})
		return
	}
	mappings, err := a.ListTickerMappings(ctx)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Map FX instrument IDs (`FX:XXXYYY`) → first mapping found.
	// Mappings are now pair-keyed; for display purposes we take the first
	// match per instrument (FX symbols don't vary across portfolios).
	byID := map[string]TickerMapping{}
	for _, m := range mappings {
		if _, seen := byID[m.InstrumentID]; !seen {
			byID[m.InstrumentID] = m
		}
	}

	out := make([]FxPairUsedRow, 0, len(pairs))
	for _, p := range pairs {
		base := strings.ToUpper(p.BaseCcy)
		quote := strings.ToUpper(p.QuoteCcy)
		directID := "FX:" + base + quote
		inverseID := "FX:" + quote + base
		var (
			instrumentID *string
			yahoo        *string
			bfState      = "none"
			runStatus    *string
			rowsPub      *int
			errMsg       *string
			finishedAt   *int64
			barSize      *string
		)
		var resolvedID string
		var resolvedPortfolioID string
		if m, ok := byID[directID]; ok {
			resolvedID = directID
			resolvedPortfolioID = m.PortfolioID
			if m.Symbol != "" {
				s := m.Symbol
				yahoo = &s
			}
		} else if m, ok := byID[inverseID]; ok {
			resolvedID = inverseID
			resolvedPortfolioID = m.PortfolioID
			if m.Symbol != "" {
				s := m.Symbol
				yahoo = &s
			}
		}
		if resolvedID != "" {
			instrumentID = &resolvedID
			bfState, runStatus, rowsPub, errMsg, finishedAt, barSize =
				backfillSummaryFor(a.jobs, resolvedID, resolvedPortfolioID)
		}
		out = append(out, FxPairUsedRow{
			BaseCcy:                base,
			QuoteCcy:               quote,
			FirstSeenTs:            p.FirstSeenTs,
			LastSeenTs:             p.LastSeenTs,
			EventCount:             p.EventCount,
			InstrumentID:           instrumentID,
			YahooSymbol:            yahoo,
			BackfillState:          bfState,
			LatestRunStatus:        runStatus,
			LatestRunRowsPublished: rowsPub,
			LatestRunError:         errMsg,
			LatestRunFinishedAt:    finishedAt,
			LatestRunBarSize:       barSize,
		})
	}
	respondJSON(w, http.StatusOK, out)
}
