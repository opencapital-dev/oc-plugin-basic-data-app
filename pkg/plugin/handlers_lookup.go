package plugin

import (
	"net/http"
	"strconv"
)

// handleLookup: GET /yf/lookup?q=<query>&limit=<n>. Free-text Yahoo
// ticker search. No identity needed (no plugin state read), but we still
// pre-flight handlerCtx so the auth gate is consistent across routes.
func (a *App) handleLookup(w http.ResponseWriter, r *http.Request) {
	if !methodGuard(w, r, http.MethodGet) {
		return
	}
	if _, ok := a.handlerCtx(w, r); !ok {
		return
	}
	q := r.URL.Query().Get("q")
	if q == "" {
		respondJSON(w, http.StatusOK, []LookupCandidate{})
		return
	}
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	out, err := a.yf.Lookup(r.Context(), q, limit)
	if err != nil {
		respondErr(w, http.StatusBadGateway, "lookup failed: "+err.Error())
		return
	}
	respondJSON(w, http.StatusOK, out)
}
