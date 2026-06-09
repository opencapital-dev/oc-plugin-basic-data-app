package plugin

import (
	"encoding/json"
	"errors"
	"net/http"
)

func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid json: "+err.Error())
		return v, false
	}
	return v, true
}

func respondJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func respondErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"detail": msg})
}

func methodGuard(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	for _, m := range allowed {
		if r.Method == m {
			return true
		}
	}
	w.Header().Set("Allow", joinAllowed(allowed))
	respondErr(w, http.StatusMethodNotAllowed, "method not allowed")
	return false
}

func joinAllowed(methods []string) string {
	out := ""
	for i, m := range methods {
		if i > 0 {
			out += ", "
		}
		out += m
	}
	return out
}

var errNotFound = errors.New("not found")
