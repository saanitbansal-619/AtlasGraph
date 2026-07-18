package cli

import (
	"net/http"
	"strings"
)

func (s *apiServer) handleDBHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if s.db == nil {
		writeJSONStatus(w, http.StatusOK, map[string]any{
			"enabled": false,
			"status":  "disabled",
		})
		return
	}
	if err := s.db.Ping(r.Context()); err != nil {
		writeJSONStatus(w, http.StatusServiceUnavailable, map[string]any{
			"enabled": true,
			"status":  "error",
			"error":   err.Error(),
		})
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"enabled": true,
		"status":  "ok",
	})
}

func (s *apiServer) requireDB(w http.ResponseWriter) bool {
	if s.db != nil {
		return true
	}
	writeAPIError(w, http.StatusServiceUnavailable,
		"PostgreSQL analytics is disabled",
		"set DATABASE_URL and restart atlas serve; file-backed endpoints remain available")
	return false
}

func (s *apiServer) handleDBSummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) || !s.requireDB(w) {
		return
	}
	summary, err := s.db.Summary(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas db migrate` and `atlas db load`")
		return
	}
	writeJSONStatus(w, http.StatusOK, summary)
}

func (s *apiServer) handleDBTopSuppliers(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) || !s.requireDB(w) {
		return
	}
	importer := strings.TrimSpace(r.URL.Query().Get("importer"))
	commodity := strings.TrimSpace(r.URL.Query().Get("commodity"))
	if importer == "" || commodity == "" {
		writeAPIError(w, http.StatusBadRequest,
			"importer and commodity query parameters are required",
			"example: /api/db/trade/top-suppliers?importer=USA&commodity=semiconductors")
		return
	}
	suppliers, err := s.db.TopSuppliers(r.Context(), importer, commodity, 10)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas db migrate` and `atlas db load`")
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"importer":  importer,
		"commodity": commodity,
		"suppliers": suppliers,
	})
}

func (s *apiServer) handleDBRecentScenarios(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) || !s.requireDB(w) {
		return
	}
	runs, err := s.db.RecentScenarios(r.Context(), 20)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas db migrate` before starting the server")
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{"scenarios": runs})
}
