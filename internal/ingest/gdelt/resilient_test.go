package gdelt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// noSleepClient returns a client pointed at srv whose rate-limit / back-off
// sleeps are no-ops, so 429 retry behaviour can be tested without real waits.
func noSleepClient(srv *httptest.Server) *Client {
	return &Client{
		BaseURL:      srv.URL,
		HTTP:         srv.Client(),
		MaxRecords:   DefaultLimit,
		DelaySeconds: DefaultDelaySeconds,
		MaxRetries:   DefaultMaxRetries,
		RetryWait:    DefaultRetryWaitSec * time.Second,
		Sleep:        func(time.Duration) {},
	}
}

func TestClampDelaySeconds(t *testing.T) {
	cases := map[int]int{
		0: MinDelaySeconds, 1: MinDelaySeconds, 4: MinDelaySeconds,
		5: 5, 6: 6, 10: 10,
	}
	for in, want := range cases {
		if got := ClampDelaySeconds(in); got != want {
			t.Errorf("ClampDelaySeconds(%d) = %d, want %d", in, got, want)
		}
	}
}

// TestFetchPartialRetriesOn429 verifies a country that is rate-limited a couple
// of times eventually succeeds after retrying.
func TestFetchPartialRetriesOn429(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 { // fail the first two attempts
			http.Error(w, "Please limit requests to one every 5 seconds", http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"articles":[{"url":"https://example.com/a","title":"sanctions and conflict","seendate":"20240115T103000Z","domain":"example.com","language":"English","sourcecountry":"United States"}]}`))
	}))
	defer srv.Close()

	c := noSleepClient(srv)
	res, err := c.FetchPartial(context.Background(), []string{"TWN"}, 7)
	if err != nil {
		t.Fatalf("FetchPartial error: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Errorf("expected 3 attempts (2 retries), got %d", got)
	}
	if len(res.Succeeded) != 1 || len(res.Failed) != 0 {
		t.Fatalf("expected 1 success, 0 failures; got %+v", res)
	}
	if len(res.Records) != 1 {
		t.Fatalf("expected 1 record after retry, got %d", len(res.Records))
	}
}

// TestFetchPartialAllFailOn429 verifies a permanently rate-limited host marks
// every country failed (rather than aborting) and reports the 429 reason.
func TestFetchPartialAllFailOn429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Please limit requests to one every 5 seconds", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := noSleepClient(srv)
	res, err := c.FetchPartial(context.Background(), []string{"TWN", "CHN"}, 7)
	if err != nil {
		t.Fatalf("FetchPartial error: %v", err)
	}
	if len(res.Succeeded) != 0 {
		t.Errorf("expected no successes, got %v", res.Succeeded)
	}
	if len(res.Failed) != 2 {
		t.Fatalf("expected 2 failures, got %+v", res.Failed)
	}
	if !strings.Contains(res.Failed[0].Reason, "429") {
		t.Errorf("expected a 429 reason, got %q", res.Failed[0].Reason)
	}
}

// TestFetchPartialPartialSuccess verifies one country can succeed while another
// fails, with records preserved for the successful one.
func TestFetchPartialPartialSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Taiwan succeeds; everything else is rate-limited.
		if strings.Contains(r.URL.Query().Get("query"), "Taiwan") {
			w.Write([]byte(`{"articles":[{"url":"https://example.com/a","title":"semiconductor sanctions","seendate":"20240115T103000Z","domain":"example.com","language":"English","sourcecountry":"United States"}]}`))
			return
		}
		http.Error(w, "Please limit requests to one every 5 seconds", http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := noSleepClient(srv)
	res, err := c.FetchPartial(context.Background(), []string{"TWN", "CHN"}, 7)
	if err != nil {
		t.Fatalf("FetchPartial error: %v", err)
	}
	if len(res.Succeeded) != 1 || res.Succeeded[0] != "TWN" {
		t.Errorf("expected TWN to succeed, got %v", res.Succeeded)
	}
	if len(res.Failed) != 1 || res.Failed[0].Code != "CHN" {
		t.Errorf("expected CHN to fail, got %+v", res.Failed)
	}
	if len(res.Records) != 1 {
		t.Errorf("expected 1 record from the successful country, got %d", len(res.Records))
	}
}

// TestFetchPartialLimitFlowsThrough verifies MaxRecords (the --limit flag) is
// sent as the GDELT maxrecords parameter.
func TestFetchPartialLimitFlowsThrough(t *testing.T) {
	var gotMax string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMax = r.URL.Query().Get("maxrecords")
		w.Write([]byte(`{"articles":[]}`))
	}))
	defer srv.Close()

	c := noSleepClient(srv)
	c.MaxRecords = 25
	if _, err := c.FetchPartial(context.Background(), []string{"TWN"}, 7); err != nil {
		t.Fatalf("FetchPartial error: %v", err)
	}
	if gotMax != "25" {
		t.Errorf("expected maxrecords=25, got %q", gotMax)
	}
}

// TestFetchPartialNonRateLimitErrorNotRetried verifies a non-429 error fails the
// country immediately, without burning retries.
func TestFetchPartialNonRateLimitErrorNotRetried(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := noSleepClient(srv)
	res, err := c.FetchPartial(context.Background(), []string{"TWN"}, 7)
	if err != nil {
		t.Fatalf("FetchPartial error: %v", err)
	}
	if len(res.Failed) != 1 {
		t.Fatalf("expected 1 failure, got %+v", res.Failed)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("expected exactly 1 attempt for a 500 (no retry), got %d", got)
	}
}

func TestFetchPartialValidatesInput(t *testing.T) {
	c := NewClient(time.Second)
	c.Sleep = func(time.Duration) {}
	if _, err := c.FetchPartial(context.Background(), nil, 7); err == nil {
		t.Error("expected error for no countries")
	}
	if _, err := c.FetchPartial(context.Background(), []string{"USA"}, 0); err == nil {
		t.Error("expected error for days < 1")
	}
}
