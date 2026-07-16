package worldbank

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func tradeIndicator() Indicator {
	return Indicator{Code: "NE.TRD.GNFS.ZS", Name: "Trade (% of GDP)"}
}

func TestFetchBestEffortContinuesOnIndicatorFailure(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/indicator/NY.GDP.MKTP.CD"):
			fmt.Fprint(w, gdpResponse("USA"))
		case strings.Contains(r.URL.Path, "/indicator/NE.TRD.GNFS.ZS"):
			http.Error(w, "bad request", http.StatusBadRequest)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if q := r.URL.Query().Get("per_page"); q != "20000" {
			t.Errorf("per_page = %q, want 20000", q)
		}
		if r.URL.Query().Get("date") != "" {
			t.Errorf("date filter should be omitted, got %q", r.URL.Query().Get("date"))
		}
	})
	defer closeFn()

	res, err := c.FetchBestEffort(context.Background(), []string{"USA"}, []Indicator{gdpIndicator(), tradeIndicator()}, FetchQueryOpts{})
	if err != nil {
		t.Fatalf("FetchBestEffort: %v", err)
	}
	if len(res.Records) == 0 {
		t.Fatal("expected GDP records")
	}
	if len(res.Warnings) != 1 || res.Warnings[0].Indicator != "NE.TRD.GNFS.ZS" {
		t.Fatalf("warnings = %#v", res.Warnings)
	}
}

func TestFetchBestEffortMultiCountryPath(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		want := "/country/USA;CHN/indicator/NY.GDP.MKTP.CD"
		if !strings.HasSuffix(r.URL.Path, want) && r.URL.Path != want {
			if !strings.Contains(r.URL.Path, "USA;CHN") {
				t.Errorf("unexpected path %q", r.URL.Path)
			}
		}
		fmt.Fprint(w, gdpResponse("USA"))
	})
	defer closeFn()

	_, err := c.FetchBestEffort(context.Background(), []string{"USA", "CHN"}, []Indicator{gdpIndicator()}, FetchQueryOpts{})
	if err != nil {
		t.Fatalf("FetchBestEffort: %v", err)
	}
}

func TestFetchBestEffortFailsWhenAllIndicatorsFail(t *testing.T) {
	c, closeFn := newServer(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	})
	defer closeFn()

	_, err := c.FetchBestEffort(context.Background(), []string{"USA"}, []Indicator{gdpIndicator()}, FetchQueryOpts{})
	if err == nil || !strings.Contains(err.Error(), "no indicators were successfully fetched") {
		t.Fatalf("expected total failure, got %v", err)
	}
}

func TestIndicatorEndpointUsesRawCode(t *testing.T) {
	got := indicatorEndpoint("https://api.worldbank.org/v2", "USA;CHN", tradeIndicator())
	want := "https://api.worldbank.org/v2/country/USA;CHN/indicator/NE.TRD.GNFS.ZS"
	if got != want {
		t.Fatalf("endpoint = %q, want %q", got, want)
	}
}
