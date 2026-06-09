package plugin

import (
	"errors"
	"net/http"
	"strings"

	"github.com/grafana/grafana-plugin-sdk-go/backend/log"
)

type symbolChangeRequest struct {
	Symbol      string `json:"symbol"`
	PortfolioID string `json:"portfolio_id"`
	UpdatedBy   string `json:"updated_by"`
}

type symbolChangeResponse struct {
	InstrumentID string  `json:"instrument_id"`
	PortfolioID  string  `json:"portfolio_id"`
	Symbol       string  `json:"symbol"`
	Sector       *string `json:"sector,omitempty"`
	Industry     *string `json:"industry,omitempty"`
}

// handleSymbol: GET /yf/symbols/{instrument_id} reads, POST writes. The
// discovery loop picks up the change on its next tick and the worker
// handles the purge + backfill.
//
// GET: requires ?portfolio_id= query param to identify the pair.
// POST: requires body.portfolio_id to identify the pair.
func (a *App) handleSymbol(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/yf/symbols/")
	id = strings.TrimSuffix(id, "/")
	if id == "" {
		respondErr(w, http.StatusBadRequest, "missing instrument_id")
		return
	}
	ctx, ok := a.handlerCtx(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		portfolioID := r.URL.Query().Get("portfolio_id")
		if portfolioID == "" {
			respondErr(w, http.StatusBadRequest, "portfolio_id query param required")
			return
		}
		m, err := a.GetTickerMapping(ctx, id, portfolioID)
		if err != nil {
			if errors.Is(err, errNotFound) {
				respondErr(w, http.StatusNotFound, "mapping not found")
				return
			}
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		respondJSON(w, http.StatusOK, m)
	case http.MethodPost:
		body, ok := decodeJSON[symbolChangeRequest](w, r)
		if !ok {
			return
		}
		if body.Symbol == "" {
			respondErr(w, http.StatusUnprocessableEntity, "symbol required")
			return
		}
		if body.PortfolioID == "" {
			respondErr(w, http.StatusUnprocessableEntity, "portfolio_id required")
			return
		}
		updatedBy := body.UpdatedBy
		if updatedBy == "" {
			updatedBy = "yfinance-plugin"
		}
		m, err := a.UpsertTickerMapping(ctx, id, body.PortfolioID, body.Symbol, nil, updatedBy)
		if err != nil {
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Synchronously derive sector/industry from Yahoo. A different symbol is
		// a different company, so this overwrites any prior (incl. user) values.
		// Non-fatal: a lookup failure leaves the symbol mapped, classification null.
		if sector, industry, ierr := a.yf.Info(ctx, body.Symbol); ierr != nil {
			log.DefaultLogger.Warn("yfinance: classification fetch failed",
				"symbol", body.Symbol, "err", ierr)
		} else {
			// Yahoo returns "" for symbols with no profile (ETFs/FX/crypto).
			// Pass nil (not &"") so SetClassification leaves the column NULL
			// rather than storing an empty string.
			sectorPtr, industryPtr := nilIfEmpty(sector), nilIfEmpty(industry)
			m, err = a.SetClassification(ctx, id, body.PortfolioID, sectorPtr, industryPtr, sourceYfinance)
			if err != nil {
				respondErr(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
		respondJSON(w, http.StatusOK, symbolChangeResponse{
			InstrumentID: m.InstrumentID,
			PortfolioID:  m.PortfolioID,
			Symbol:       m.Symbol,
			Sector:       m.Sector,
			Industry:     m.Subindustry,
		})
	default:
		methodGuard(w, r, http.MethodGet, http.MethodPost)
	}
}
