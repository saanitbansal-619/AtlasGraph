package gdelt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Fetcher retrieves normalised event records for a set of ISO3 country codes
// over a look-back window. *Client is the live implementation; tests can supply
// their own (or point a *Client at an httptest server) so no real GDELT call is
// ever made under test.
type Fetcher interface {
	Fetch(ctx context.Context, countries []string, days int) ([]GDELTEventRecord, error)
}

// Client talks to the GDELT DOC 2.0 API. The zero value is not usable; construct
// one with NewClient. BaseURL is overridable so tests (and an optional CLI flag)
// can point it at an httptest server.
type Client struct {
	BaseURL    string
	HTTP       *http.Client
	MaxRecords int

	// DelaySeconds spaces consecutive per-country requests apart to respect
	// GDELT's rate limit. MaxRetries / RetryWait control 429 back-off.
	DelaySeconds int
	MaxRetries   int
	RetryWait    time.Duration

	// Sleep is the pause function used for inter-request spacing and 429
	// back-off. It defaults to time.Sleep; tests override it with a no-op so
	// rate-limit behaviour can be exercised without real waits.
	Sleep func(time.Duration)
}

// NewClient returns a client pointed at the live API. A non-positive timeout
// disables the client-level timeout (callers should still pass a context).
func NewClient(timeout time.Duration) *Client {
	return &Client{
		BaseURL:      DefaultBaseURL,
		HTTP:         &http.Client{Timeout: timeout},
		MaxRecords:   DefaultMaxRecords,
		DelaySeconds: DefaultDelaySeconds,
		MaxRetries:   DefaultMaxRetries,
		RetryWait:    DefaultRetryWaitSec * time.Second,
		Sleep:        time.Sleep,
	}
}

// FailedCountry records a country whose fetch could not be completed, with the
// reason, so the caller can report partial failures without aborting.
type FailedCountry struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
}

// FetchResult is the outcome of a resilient, per-country fetch: the records that
// were retrieved plus which countries succeeded and which failed.
type FetchResult struct {
	Records   []GDELTEventRecord
	Succeeded []string
	Failed    []FailedCountry
}

// ClampDelaySeconds enforces GDELT's documented minimum spacing of one request
// every 5 seconds: any value below MinDelaySeconds is raised to MinDelaySeconds.
func ClampDelaySeconds(d int) int {
	if d < MinDelaySeconds {
		return MinDelaySeconds
	}
	return d
}

// statusError carries the HTTP status of a non-200 GDELT response so the retry
// logic can single out 429 (Too Many Requests) for back-off.
type statusError struct {
	Code    int
	Status  string
	Snippet string
}

func (e *statusError) Error() string {
	return fmt.Sprintf("unexpected status %s: %s", e.Status, e.Snippet)
}

func isRateLimited(err error) bool {
	var se *statusError
	return errors.As(err, &se) && se.Code == http.StatusTooManyRequests
}

// sleepFor pauses using the client's Sleep hook (default time.Sleep), ignoring
// non-positive durations.
func (c *Client) sleepFor(d time.Duration) {
	if d <= 0 {
		return
	}
	if c.Sleep != nil {
		c.Sleep(d)
		return
	}
	time.Sleep(d)
}

// FetchPartial fetches each country independently and never aborts the whole run
// on a single country's failure. It spaces requests apart (DelaySeconds), backs
// off and retries on HTTP 429 (up to MaxRetries), and records any country that
// still fails so the caller can save partial results and report the failures.
func (c *Client) FetchPartial(ctx context.Context, countries []string, days int) (FetchResult, error) {
	if len(countries) == 0 {
		return FetchResult{}, fmt.Errorf("no countries requested")
	}
	if days < 1 {
		return FetchResult{}, fmt.Errorf("days must be >= 1, got %d", days)
	}

	delay := time.Duration(ClampDelaySeconds(c.DelaySeconds)) * time.Second
	fetchedAt := time.Now().UTC()

	var res FetchResult
	first := true
	for _, code := range countries {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		if !first {
			c.sleepFor(delay) // space requests to respect the rate limit
		}
		first = false

		name := resolveCountryName(code)
		articles, err := c.fetchCountryWithRetry(ctx, name, days)
		if err != nil {
			res.Failed = append(res.Failed, FailedCountry{Code: code, Reason: err.Error()})
			continue
		}
		res.Records = append(res.Records, normalizeArticles(articles, code, name, fetchedAt)...)
		res.Succeeded = append(res.Succeeded, code)
	}
	return res, nil
}

// fetchCountryWithRetry performs one country's query, retrying only on HTTP 429
// (waiting RetryWait between attempts). Any other error fails immediately.
func (c *Client) fetchCountryWithRetry(ctx context.Context, countryName string, days int) ([]docArticle, error) {
	retries := c.MaxRetries
	if retries < 0 {
		retries = 0
	}
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		articles, err := c.fetchCountry(ctx, countryName, days)
		if err == nil {
			return articles, nil
		}
		lastErr = err
		if isRateLimited(err) && attempt < retries {
			wait := c.RetryWait
			if wait <= 0 {
				wait = DefaultRetryWaitSec * time.Second
			}
			c.sleepFor(wait)
			continue
		}
		return nil, err
	}
	return nil, lastErr
}

// Fetch issues one GDELT query per country (country name + risk terms) over the
// last `days` days and returns the normalised, deduplicated records. The first
// error aborts the whole fetch with a message naming the country that failed.
func (c *Client) Fetch(ctx context.Context, countries []string, days int) ([]GDELTEventRecord, error) {
	if len(countries) == 0 {
		return nil, fmt.Errorf("no countries requested")
	}
	if days < 1 {
		return nil, fmt.Errorf("days must be >= 1, got %d", days)
	}

	fetchedAt := time.Now().UTC()
	var records []GDELTEventRecord
	for _, code := range countries {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		name := resolveCountryName(code)
		articles, err := c.fetchCountry(ctx, name, days)
		if err != nil {
			return nil, fmt.Errorf("fetching %s: %w", code, err)
		}
		records = append(records, normalizeArticles(articles, code, name, fetchedAt)...)
	}
	return records, nil
}

// fetchCountry performs a single DOC 2.0 ArtList query for one country.
func (c *Client) fetchCountry(ctx context.Context, countryName string, days int) ([]docArticle, error) {
	max := c.MaxRecords
	if max <= 0 {
		max = DefaultMaxRecords
	}

	q := url.Values{}
	q.Set("query", buildQuery(countryName))
	q.Set("mode", "artlist")
	q.Set("format", "json")
	q.Set("timespan", fmt.Sprintf("%dd", days))
	q.Set("maxrecords", fmt.Sprintf("%d", max))
	q.Set("sort", "datedesc")

	endpoint := strings.TrimRight(c.BaseURL, "/") + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20)) // 16 MiB safety cap
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &statusError{Code: resp.StatusCode, Status: resp.Status, Snippet: snippet(body)}
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, nil // no matching documents in the window: not an error
	}
	// GDELT sometimes returns a plain-text notice (e.g. a too-short query) with a
	// 200 status; treat anything that is not a JSON object as an API-level error.
	if !strings.HasPrefix(trimmed, "{") {
		return nil, fmt.Errorf("GDELT returned a non-JSON response: %s", snippet(body))
	}

	var parsed docResponse
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil, fmt.Errorf("malformed JSON response: %w", err)
	}
	return parsed.Articles, nil
}

// buildQuery composes a GDELT query string: the country name plus an OR-group of
// the risk terms. Multi-word terms are quoted so GDELT treats them as phrases.
func buildQuery(countryName string) string {
	terms := make([]string, 0, len(RiskTerms))
	for _, t := range RiskTerms {
		terms = append(terms, quoteIfPhrase(t))
	}
	return fmt.Sprintf("%s (%s)", quoteIfPhrase(countryName), strings.Join(terms, " OR "))
}

func quoteIfPhrase(s string) string {
	if strings.Contains(s, " ") {
		return `"` + s + `"`
	}
	return s
}

func snippet(b []byte) string {
	const max = 200
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
