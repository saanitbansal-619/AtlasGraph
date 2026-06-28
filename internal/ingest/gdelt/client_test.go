package gdelt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// artListResponse is a realistic GDELT DOC 2.0 ArtList payload. ArtList omits
// per-article tone, so tone is absent here (it must normalise to 0).
const artListResponse = `{
  "articles": [
    {
      "url": "https://news.example.com/a1",
      "url_mobile": "",
      "title": "Taiwan faces new export controls amid semiconductor tensions",
      "seendate": "20240115T103000Z",
      "socialimage": "https://img.example.com/1.jpg",
      "domain": "news.example.com",
      "language": "English",
      "sourcecountry": "United States"
    },
    {
      "url": "https://news.example.org/a2",
      "title": "Markets steady as shipping resumes",
      "seendate": "20240114T080000Z",
      "domain": "news.example.org",
      "language": "English",
      "sourcecountry": "Japan"
    }
  ]
}`

func newServer(h http.HandlerFunc) (*Client, func()) {
	srv := httptest.NewServer(h)
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), MaxRecords: DefaultMaxRecords}
	return c, srv.Close
}

func TestFetchParsesArticlesAndMapsCountry(t *testing.T) {
	var gotQuery string
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		if r.URL.Query().Get("mode") != "artlist" || r.URL.Query().Get("format") != "json" {
			t.Errorf("unexpected mode/format params: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("timespan") != "7d" {
			t.Errorf("expected timespan 7d, got %q", r.URL.Query().Get("timespan"))
		}
		w.Write([]byte(artListResponse))
	})
	defer closeFn()

	recs, err := c.Fetch(context.Background(), []string{"TWN"}, 7)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if !strings.Contains(gotQuery, "Taiwan") {
		t.Errorf("query should mention the country name, got %q", gotQuery)
	}
	if !strings.Contains(gotQuery, "semiconductor") || !strings.Contains(gotQuery, `"export controls"`) {
		t.Errorf("query should include risk terms, got %q", gotQuery)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}

	r := recs[0]
	if r.CountryCode != "TWN" || r.CountryName != "Taiwan" {
		t.Errorf("bad country mapping: %+v", r)
	}
	if r.Title == "" || r.URL != "https://news.example.com/a1" || r.Domain != "news.example.com" {
		t.Errorf("bad article fields: %+v", r)
	}
	if r.SourceCountry != "United States" || r.Language != "English" {
		t.Errorf("bad source/language: %+v", r)
	}
	if r.Source != SourceName || r.FetchedAt.IsZero() {
		t.Errorf("missing provenance: %+v", r)
	}
	want := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !r.PublishedAt.Equal(want) {
		t.Errorf("published_at = %v, want %v", r.PublishedAt, want)
	}
	// Title matches "export controls" and "semiconductor".
	if !contains(r.RiskTermsMatched, "export controls") || !contains(r.RiskTermsMatched, "semiconductor") {
		t.Errorf("expected matched risk terms, got %v", r.RiskTermsMatched)
	}
	// ArtList omits tone => must default to 0, not break parsing.
	if r.Tone != 0 {
		t.Errorf("expected tone 0 when absent, got %v", r.Tone)
	}
	// Themes must be non-nil for a stable schema.
	if r.Themes == nil {
		t.Errorf("themes should be non-nil")
	}
}

func TestFetchToneParsedWhenPresent(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"articles":[{"url":"u","title":"conflict escalates","seendate":"20240115T103000Z","domain":"d","language":"English","sourcecountry":"China","tone":-6.5}]}`))
	})
	defer closeFn()

	recs, err := c.Fetch(context.Background(), []string{"CHN"}, 7)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(recs) != 1 || recs[0].Tone != -6.5 {
		t.Fatalf("expected tone -6.5, got %+v", recs)
	}
}

func TestFetchUnknownCountryFallsBackToCode(t *testing.T) {
	var gotQuery string
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		w.Write([]byte(`{"articles":[{"url":"u","title":"strike","seendate":"20240115T103000Z","domain":"d","language":"English","sourcecountry":"X"}]}`))
	})
	defer closeFn()

	recs, err := c.Fetch(context.Background(), []string{"xyz"}, 7)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if !strings.Contains(gotQuery, "XYZ") {
		t.Errorf("unknown code should be used as the query term, got %q", gotQuery)
	}
	if recs[0].CountryCode != "XYZ" || recs[0].CountryName != "XYZ" {
		t.Errorf("unknown code should map to itself, got %+v", recs[0])
	}
}

func TestFetchMultipleCountries(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"articles":[{"url":"u","title":"sanctions","seendate":"20240115T103000Z","domain":"d","language":"English","sourcecountry":"S"}]}`))
	})
	defer closeFn()

	recs, err := c.Fetch(context.Background(), []string{"TWN", "CHN"}, 7)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records (one per country), got %d", len(recs))
	}
	if recs[0].CountryCode != "TWN" || recs[1].CountryCode != "CHN" {
		t.Errorf("records not tagged per country: %+v", recs)
	}
}

func TestFetchNon200(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	defer closeFn()

	if _, err := c.Fetch(context.Background(), []string{"USA"}, 7); err == nil {
		t.Fatal("expected an error for a 500 response")
	}
}

func TestFetchEmptyBodyIsNotError(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		// no body
	})
	defer closeFn()

	recs, err := c.Fetch(context.Background(), []string{"USA"}, 7)
	if err != nil {
		t.Fatalf("empty body should not error, got %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected 0 records, got %d", len(recs))
	}
}

func TestFetchMalformedJSON(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{ this is not valid json`))
	})
	defer closeFn()

	_, err := c.Fetch(context.Background(), []string{"USA"}, 7)
	if err == nil || !strings.Contains(err.Error(), "malformed JSON") {
		t.Fatalf("expected malformed JSON error, got %v", err)
	}
}

func TestFetchNonJSONNotice(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Your query was too short or too long."))
	})
	defer closeFn()

	_, err := c.Fetch(context.Background(), []string{"USA"}, 7)
	if err == nil || !strings.Contains(err.Error(), "non-JSON") {
		t.Fatalf("expected non-JSON notice error, got %v", err)
	}
}

func TestFetchContextTimeout(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte(artListResponse))
	})
	defer closeFn()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err := c.Fetch(ctx, []string{"USA"}, 7); err == nil {
		t.Fatal("expected a timeout error")
	}
}

func TestFetchValidatesInput(t *testing.T) {
	c := NewClient(time.Second)
	if _, err := c.Fetch(context.Background(), nil, 7); err == nil {
		t.Error("expected error for no countries")
	}
	if _, err := c.Fetch(context.Background(), []string{"USA"}, 0); err == nil {
		t.Error("expected error for days < 1")
	}
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
