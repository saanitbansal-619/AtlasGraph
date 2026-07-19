package cli

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/atlasgraph/atlas/internal/customdata"
)

const maxCustomCSVBytes = 10 << 20

func (s *apiServer) handleCustomDataAnalyze(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxCustomCSVBytes)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		writeAPIError(w, http.StatusBadRequest,
			"invalid multipart upload: "+err.Error(),
			"upload a CSV file in the multipart field named `file` (maximum 10 MiB)")
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeAPIError(w, http.StatusBadRequest,
			"CSV file is required", "use multipart/form-data with a `file` field")
		return
	}
	defer file.Close()

	analysis, err := customdata.Analyze(file)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(),
			"required columns: importer, commodity, supplier, value_usd")
		return
	}
	datasetName := strings.TrimSpace(r.FormValue("dataset_name"))
	if datasetName == "" {
		datasetName = strings.TrimSuffix(filepath.Base(header.Filename), filepath.Ext(header.Filename))
	}
	if datasetName == "" {
		datasetName = "custom-dataset"
	}
	if len(datasetName) > 200 {
		writeAPIError(w, http.StatusBadRequest, "dataset_name must be 200 characters or fewer", "")
		return
	}

	if s.db != nil {
		if err := s.db.SaveCustomAnalysis(r.Context(), datasetName, analysis); err != nil {
			writeAPIError(w, http.StatusInternalServerError,
				fmt.Sprintf("analysis completed but PostgreSQL persistence failed: %v", err),
				"run `atlas db migrate` to apply migrations/002_custom_client_data.sql")
			return
		}
	}
	writeJSONStatus(w, http.StatusOK, analysis)
}
