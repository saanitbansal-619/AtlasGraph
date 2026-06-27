package worldbank

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// gdpResponse is a realistic two-element World Bank payload. Note per_page is a
// quoted string (as the live API sometimes returns) and one value is null.
func gdpResponse(iso3 string) string {
	return fmt.Sprintf(`[
	  {"page":1,"pages":1,"per_page":"1000","total":2,"sourceid":"2","lastupdated":"2024-07-01"},
	  [
	    {"indicator":{"id":"NY.GDP.MKTP.CD","value":"GDP (current US$)"},"country":{"id":"US","value":"United States"},"countryiso3code":"%s","date":"2023","value":27360935000000,"decimal":0},
	    {"indicator":{"id":"NY.GDP.MKTP.CD","value":"GDP (current US$)"},"country":{"id":"US","value":"United States"},"countryiso3code":"%s","date":"2022","value":null,"decimal":0}
	  ]
	]`, iso3, iso3)
}

func newServer(h http.HandlerFunc) (*Client, func()) {
	srv := httptest.NewServer(h)
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client()}
	return c, srv.Close
}

func gdpIndicator() Indicator {
	return Indicator{Code: "NY.GDP.MKTP.CD", Name: "GDP (current US$)"}
}

func TestFetchSuccess(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/country/USA/indicator/NY.GDP.MKTP.CD") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		fmt.Fprint(w, gdpResponse("USA"))
	})
	defer closeFn()

	recs, err := c.Fetch(context.Background(), []string{"USA"}, []Indicator{gdpIndicator()}, 2022, 2023)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 records, got %d", len(recs))
	}
	r := recs[0]
	if r.CountryCode != "USA" || r.CountryName != "United States" {
		t.Errorf("bad country fields: %+v", r)
	}
	if r.IndicatorCode != "NY.GDP.MKTP.CD" || r.Year != 2023 {
		t.Errorf("bad indicator/year: %+v", r)
	}
	if r.Value == nil || *r.Value != 27360935000000 {
		t.Errorf("expected 2023 value, got %v", r.Value)
	}
	if r.Source != SourceName || r.FetchedAt.IsZero() {
		t.Errorf("missing provenance: %+v", r)
	}
	// The 2022 observation is genuinely missing and must stay nil.
	if recs[1].Value != nil {
		t.Errorf("expected nil value for missing 2022 observation, got %v", *recs[1].Value)
	}
}

func TestFetchPagination(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "1":
			fmt.Fprint(w, `[{"page":1,"pages":2,"per_page":"1","total":2},[
				{"indicator":{"id":"NY.GDP.MKTP.CD","value":"GDP"},"country":{"id":"US","value":"United States"},"countryiso3code":"USA","date":"2023","value":100}]]`)
		case "2":
			fmt.Fprint(w, `[{"page":2,"pages":2,"per_page":"1","total":2},[
				{"indicator":{"id":"NY.GDP.MKTP.CD","value":"GDP"},"country":{"id":"US","value":"United States"},"countryiso3code":"USA","date":"2022","value":90}]]`)
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	})
	defer closeFn()

	recs, err := c.Fetch(context.Background(), []string{"USA"}, []Indicator{gdpIndicator()}, 2022, 2023)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("expected 2 paginated records, got %d", len(recs))
	}
}

func TestFetchNon200(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	defer closeFn()

	if _, err := c.Fetch(context.Background(), []string{"USA"}, []Indicator{gdpIndicator()}, 2022, 2023); err == nil {
		t.Fatal("expected an error for a 500 response")
	}
}

func TestFetchMalformedJSON(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{ this is not the array you are looking for`)
	})
	defer closeFn()

	_, err := c.Fetch(context.Background(), []string{"USA"}, []Indicator{gdpIndicator()}, 2022, 2023)
	if err == nil || !strings.Contains(err.Error(), "malformed JSON") {
		t.Fatalf("expected malformed JSON error, got %v", err)
	}
}

func TestFetchAPIMessageError(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"message":[{"id":"120","key":"Invalid value","value":"The provided parameter value is not valid"}]}]`)
	})
	defer closeFn()

	_, err := c.Fetch(context.Background(), []string{"ZZZ"}, []Indicator{gdpIndicator()}, 2022, 2023)
	if err == nil || !strings.Contains(err.Error(), "not valid") {
		t.Fatalf("expected API message error, got %v", err)
	}
}

func TestFetchEmptyResult(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"page":1,"pages":0,"per_page":"1000","total":0},null]`)
	})
	defer closeFn()

	recs, err := c.Fetch(context.Background(), []string{"USA"}, []Indicator{gdpIndicator()}, 2022, 2023)
	if err != nil {
		t.Fatalf("empty result should not error, got %v", err)
	}
	if len(recs) != 0 {
		t.Fatalf("expected 0 records, got %d", len(recs))
	}
}

func TestFetchContextTimeout(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		fmt.Fprint(w, gdpResponse("USA"))
	})
	defer closeFn()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, err := c.Fetch(ctx, []string{"USA"}, []Indicator{gdpIndicator()}, 2022, 2023); err == nil {
		t.Fatal("expected a timeout error")
	}
}
