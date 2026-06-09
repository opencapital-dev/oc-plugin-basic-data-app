package plugin

import (
	"errors"
	"net/http"
	"strings"
)

// classificationRequest is the PATCH body. portfolio_id identifies the pair.
// sector/industry are tri-state: omitted (nil) = leave unchanged; present
// (incl. empty string) = set to that value with source=user. Pointers
// distinguish "absent" from "empty".
type classificationRequest struct {
	PortfolioID string  `json:"portfolio_id"`
	Sector      *string `json:"sector"`
	Industry    *string `json:"industry"`
}

// classificationResponse mirrors the request's field naming (industry, not the
// subindustry column name) so the PATCH contract is symmetric.
type classificationResponse struct {
	InstrumentID string  `json:"instrument_id"`
	PortfolioID  string  `json:"portfolio_id"`
	Sector       *string `json:"sector,omitempty"`
	Industry     *string `json:"industry,omitempty"`
}

// handleClassification: PATCH /yf/classification/{instrument_id} writes a
// manual sector/industry override (source=user) for the (instrument,
// portfolio) pair. It never calls Yahoo — a manual edit must not re-fetch.
func (a *App) handleClassification(w http.ResponseWriter, r *http.Request) {
	if !methodGuard(w, r, http.MethodPatch) {
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/yf/classification/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		respondErr(w, http.StatusBadRequest, "missing instrument_id")
		return
	}
	ctx, ok := a.handlerCtx(w, r)
	if !ok {
		return
	}
	body, ok := decodeJSON[classificationRequest](w, r)
	if !ok {
		return
	}
	if body.PortfolioID == "" {
		respondErr(w, http.StatusUnprocessableEntity, "portfolio_id required")
		return
	}
	if body.Sector == nil && body.Industry == nil {
		respondErr(w, http.StatusUnprocessableEntity, "sector or industry required")
		return
	}
	m, err := a.SetClassification(ctx, id, body.PortfolioID, body.Sector, body.Industry, sourceUser)
	if err != nil {
		if errors.Is(err, errNotFound) {
			respondErr(w, http.StatusNotFound, "mapping not found")
			return
		}
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	respondJSON(w, http.StatusOK, classificationResponse{
		InstrumentID: m.InstrumentID,
		PortfolioID:  m.PortfolioID,
		Sector:       m.Sector,
		Industry:     m.Subindustry,
	})
}
