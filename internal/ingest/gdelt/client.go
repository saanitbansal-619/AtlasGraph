package gdelt

import (
	"context"
	"encoding/json"
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
}

// NewClient returns a client pointed at the live API. A non-positive timeout
// disables the client-level timeout (callers should still pass a context).
func NewClient(timeout time.Duration) *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		HTTP:       &http.Client{Timeout: timeout},
		MaxRecords: DefaultMaxRecords,
	}
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
		return nil, fmt.Errorf("unexpected status %s: %s", resp.Status, snippet(body))
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
