package worldbank

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

// maxPerPage asks the API for everything in one page where possible; the
// per-country/indicator/year panels are tiny, so pagination rarely triggers.
const maxPerPage = 1000

// Client talks to the World Bank Indicators API. The zero value is not usable;
// construct one with NewClient. BaseURL is overridable so tests can point at an
// httptest server.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

// NewClient returns a client pointed at the live API. A non-positive timeout
// disables the client-level timeout (callers should still pass a context).
func NewClient(timeout time.Duration) *Client {
	return &Client{
		BaseURL: DefaultBaseURL,
		HTTP:    &http.Client{Timeout: timeout},
	}
}

// Fetch retrieves every (country × indicator) series within [start, end] and
// returns the normalised records. The first error aborts the whole fetch with a
// message naming the country and indicator that failed.
func (c *Client) Fetch(ctx context.Context, countries []string, indicators []Indicator, start, end int) ([]CountryIndicatorRecord, error) {
	if len(countries) == 0 {
		return nil, fmt.Errorf("no countries requested")
	}
	if len(indicators) == 0 {
		return nil, fmt.Errorf("no indicators requested")
	}
	if start > end {
		return nil, fmt.Errorf("start year %d is after end year %d", start, end)
	}

	// The API accepts several countries in one request (semicolon-separated),
	// so we issue one call per indicator rather than one per (country,indicator).
	clean := make([]string, 0, len(countries))
	for _, country := range countries {
		if country = strings.TrimSpace(country); country != "" {
			clean = append(clean, country)
		}
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("no valid country codes")
	}
	countryPath := strings.Join(clean, ";")

	fetchedAt := time.Now().UTC()
	var records []CountryIndicatorRecord
	for _, ind := range indicators {
		points, err := c.fetchSeries(ctx, countryPath, ind, start, end)
		if err != nil {
			return nil, fmt.Errorf("fetching %s: %w", ind.Code, err)
		}
		records = append(records, normalizeRecords(points, ind, fetchedAt)...)
	}
	return records, nil
}

// fetchSeries walks every page of a single indicator series (across all
// requested countries) and returns all observations.
func (c *Client) fetchSeries(ctx context.Context, countryPath string, ind Indicator, start, end int) ([]apiPoint, error) {
	var all []apiPoint
	for page := 1; ; page++ {
		meta, points, err := c.fetchPage(ctx, countryPath, ind, start, end, page)
		if err != nil {
			return nil, err
		}
		all = append(all, points...)
		if meta.Pages <= page {
			break
		}
	}
	return all, nil
}

// fetchPage retrieves a single page and decodes the World Bank's two-element
// response: [metadata, observations]. countryPath is a raw, semicolon-separated
// list of ISO codes (not URL-escaped, as ISO codes are path-safe).
func (c *Client) fetchPage(ctx context.Context, countryPath string, ind Indicator, start, end, page int) (apiMeta, []apiPoint, error) {
	endpoint := fmt.Sprintf("%s/country/%s/indicator/%s",
		strings.TrimRight(c.BaseURL, "/"),
		countryPath, url.PathEscape(ind.Code))

	q := url.Values{}
	q.Set("format", "json")
	q.Set("per_page", fmt.Sprintf("%d", maxPerPage))
	q.Set("date", fmt.Sprintf("%d:%d", start, end))
	q.Set("page", fmt.Sprintf("%d", page))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return apiMeta{}, nil, fmt.Errorf("building request: %w", err)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return apiMeta{}, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20)) // 8 MiB safety cap
	if err != nil {
		return apiMeta{}, nil, fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return apiMeta{}, nil, fmt.Errorf("unexpected status %s: %s", resp.Status, snippet(body))
	}

	// The API always wraps results in a two-element array.
	var raw []json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return apiMeta{}, nil, fmt.Errorf("malformed JSON response: %w", err)
	}
	if len(raw) == 0 {
		return apiMeta{}, nil, fmt.Errorf("empty response array")
	}

	var meta apiMeta
	if err := json.Unmarshal(raw[0], &meta); err != nil {
		return apiMeta{}, nil, fmt.Errorf("malformed response metadata: %w", err)
	}
	if len(meta.Message) > 0 {
		return apiMeta{}, nil, fmt.Errorf("API error: %s", joinMessages(meta.Message))
	}

	// No data array (e.g. nothing for this country/indicator/range): not an error.
	if len(raw) < 2 {
		return meta, nil, nil
	}
	var points []apiPoint
	if err := json.Unmarshal(raw[1], &points); err != nil {
		return apiMeta{}, nil, fmt.Errorf("malformed observation data: %w", err)
	}
	return meta, points, nil
}

func joinMessages(msgs []apiMessage) string {
	parts := make([]string, 0, len(msgs))
	for _, m := range msgs {
		parts = append(parts, m.Value)
	}
	return strings.Join(parts, "; ")
}

func snippet(b []byte) string {
	const max = 200
	s := strings.TrimSpace(string(b))
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}
