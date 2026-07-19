package cli

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/atlasgraph/atlas/internal/customdata"
)

func customUploadRequest(t *testing.T, csv string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "client-suppliers.csv")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte(csv)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("dataset_name", "test-client"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/custom-data/analyze", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestAPICustomDataAnalyzeMultipartCSV(t *testing.T) {
	csv := "importer,commodity,supplier,value_usd\nUSA,semiconductors,Taiwan,75\nUSA,semiconductors,Korea,25\n"
	rec := httptest.NewRecorder()
	newAPIServer(serverConfig{}).ServeHTTP(rec, customUploadRequest(t, csv))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d\n%s", rec.Code, rec.Body.String())
	}
	var analysis customdata.Analysis
	if err := json.Unmarshal(rec.Body.Bytes(), &analysis); err != nil {
		t.Fatal(err)
	}
	if len(analysis.ConcentrationResults) != 1 || analysis.ConcentrationResults[0].HHI != 0.625 {
		t.Fatalf("analysis = %+v", analysis)
	}
}

func TestAPICustomDataAnalyzeRejectsMissingColumns(t *testing.T) {
	rec := httptest.NewRecorder()
	newAPIServer(serverConfig{}).ServeHTTP(rec,
		customUploadRequest(t, "importer,commodity,value_usd\nUSA,oil,10\n"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400\n%s", rec.Code, rec.Body.String())
	}
}
