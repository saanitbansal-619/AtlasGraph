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

// MacroPipelinePerPage is the page size used by macro pipeline fetches.
const MacroPipelinePerPage = 20000

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
	countryPath, err := joinCountryPath(countries)
	if err != nil {
		return nil, err
	}
	if start > end {
		return nil, fmt.Errorf("start year %d is after end year %d", start, end)
	}
	if len(indicators) == 0 {
		return nil, fmt.Errorf("no indicators requested")
	}

	fetchedAt := time.Now().UTC()
	var records []CountryIndicatorRecord
	opts := FetchQueryOpts{StartYear: start, EndYear: end, PerPage: maxPerPage}
	for _, ind := range indicators {
		points, err := c.fetchSeriesWithOpts(ctx, countryPath, ind, opts)
		if err != nil {
			return nil, fmt.Errorf("fetching %s: %w", ind.Code, err)
		}
		records = append(records, normalizeRecords(points, ind, fetchedAt)...)
	}
	return records, nil
}

// FetchWarning records a non-fatal issue while fetching one indicator series.
type FetchWarning struct {
	Indicator string `json:"indicator"`
	Message   string `json:"message"`
}

// FetchQueryOpts controls a resilient World Bank indicators request.
type FetchQueryOpts struct {
	StartYear int // both zero omits the date filter (all years)
	EndYear   int
	PerPage   int
}

// BestEffortResult is the outcome of a partial-tolerant fetch.
type BestEffortResult struct {
	Records  []CountryIndicatorRecord
	Warnings []FetchWarning
}

// FetchBestEffort retrieves indicator series and continues when individual
// indicators fail. It returns an error only when no indicator returned data.
func (c *Client) FetchBestEffort(ctx context.Context, countries []string, indicators []Indicator, opts FetchQueryOpts) (BestEffortResult, error) {
	countryPath, err := joinCountryPath(countries)
	if err != nil {
		return BestEffortResult{}, err
	}
	if len(indicators) == 0 {
		return BestEffortResult{}, fmt.Errorf("no indicators requested")
	}
	if opts.StartYear > 0 && opts.EndYear > 0 && opts.StartYear > opts.EndYear {
		return BestEffortResult{}, fmt.Errorf("start year %d is after end year %d", opts.StartYear, opts.EndYear)
	}
	if opts.PerPage <= 0 {
		opts.PerPage = MacroPipelinePerPage
	}

	fetchedAt := time.Now().UTC()
	out := BestEffortResult{}
	for _, ind := range indicators {
		points, err := c.fetchSeriesWithOpts(ctx, countryPath, ind, opts)
		if err != nil {
			out.Warnings = append(out.Warnings, FetchWarning{
				Indicator: ind.Code,
				Message:   err.Error(),
			})
			continue
		}
		out.Records = append(out.Records, normalizeRecords(points, ind, fetchedAt)...)
	}
	if len(out.Records) == 0 {
		return out, fmt.Errorf("no indicators were successfully fetched")
	}
	return out, nil
}

func joinCountryPath(countries []string) (string, error) {
	if len(countries) == 0 {
		return "", fmt.Errorf("no countries requested")
	}
	clean := make([]string, 0, len(countries))
	for _, country := range countries {
		if country = strings.TrimSpace(country); country != "" {
			clean = append(clean, strings.ToUpper(country))
		}
	}
	if len(clean) == 0 {
		return "", fmt.Errorf("no valid country codes")
	}
	return strings.Join(clean, ";"), nil
}

func (c *Client) fetchSeriesWithOpts(ctx context.Context, countryPath string, ind Indicator, opts FetchQueryOpts) ([]apiPoint, error) {
	var all []apiPoint
	for page := 1; ; page++ {
		meta, points, err := c.fetchPageWithOpts(ctx, countryPath, ind, opts, page)
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

func indicatorEndpoint(baseURL, countryPath string, ind Indicator) string {
	return fmt.Sprintf("%s/country/%s/indicator/%s",
		strings.TrimRight(baseURL, "/"),
		countryPath,
		ind.Code)
}

func queryValuesForFetch(opts FetchQueryOpts, page int) url.Values {
	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = maxPerPage
	}
	q := url.Values{}
	q.Set("format", "json")
	q.Set("per_page", fmt.Sprintf("%d", perPage))
	if opts.StartYear > 0 && opts.EndYear > 0 {
		q.Set("date", fmt.Sprintf("%d:%d", opts.StartYear, opts.EndYear))
	}
	q.Set("page", fmt.Sprintf("%d", page))
	return q
}

func (c *Client) fetchPageWithOpts(ctx context.Context, countryPath string, ind Indicator, opts FetchQueryOpts, page int) (apiMeta, []apiPoint, error) {
	endpoint := indicatorEndpoint(c.BaseURL, countryPath, ind)
	q := queryValuesForFetch(opts, page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return apiMeta{}, nil, fmt.Errorf("building request: %w", err)
	}
	return c.decodeFetchResponse(req)
}

func (c *Client) decodeFetchResponse(req *http.Request) (apiMeta, []apiPoint, error) {
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
