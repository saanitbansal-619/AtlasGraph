package gdelt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// gdeltTimeLayout is the timestamp format GDELT uses for seendate, e.g.
// "20240115T103000Z".
const gdeltTimeLayout = "20060102T150405Z"

// normalizeArticles converts raw GDELT articles for one country into normalised
// records, attaching the queried country's code/name, parsing the timestamp,
// and recording which risk terms appear in each title.
func normalizeArticles(articles []docArticle, code, name string, fetchedAt time.Time) []GDELTEventRecord {
	out := make([]GDELTEventRecord, 0, len(articles))
	for _, a := range articles {
		published, _ := time.Parse(gdeltTimeLayout, strings.TrimSpace(a.SeenDate))

		tone := 0.0
		if a.Tone != nil {
			tone = *a.Tone
		}
		themes := a.Themes
		if themes == nil {
			themes = []string{}
		}

		out = append(out, GDELTEventRecord{
			CountryCode:      code,
			CountryName:      name,
			Title:            strings.TrimSpace(a.Title),
			URL:              strings.TrimSpace(a.URL),
			SourceCountry:    strings.TrimSpace(a.SourceCountry),
			Domain:           strings.TrimSpace(a.Domain),
			PublishedAt:      published.UTC(),
			Tone:             tone,
			Language:         strings.TrimSpace(a.Language),
			Themes:           themes,
			RiskTermsMatched: MatchRiskTerms(a.Title),
			Source:           SourceName,
			FetchedAt:        fetchedAt,
		})
	}
	return out
}

// MatchRiskTerms returns the risk terms (in canonical RiskTerms order) that
// appear in the given text, matched case-insensitively. The result is never nil
// so the JSON schema stays stable.
func MatchRiskTerms(text string) []string {
	lower := strings.ToLower(text)
	matched := make([]string, 0)
	for _, term := range RiskTerms {
		if strings.Contains(lower, strings.ToLower(term)) {
			matched = append(matched, term)
		}
	}
	return matched
}

// Save writes an EventFile to dir/OutputFileName, creating dir if needed, and
// returns the full path written.
func Save(dir string, file EventFile) (string, error) {
	if strings.TrimSpace(dir) == "" {
		return "", fmt.Errorf("output directory is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory %q: %w", dir, err)
	}
	path := filepath.Join(dir, OutputFileName)
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding records: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("writing %q: %w", path, err)
	}
	return path, nil
}

// Load reads the EventFile from dir/OutputFileName.
func Load(dir string) (EventFile, error) {
	path := filepath.Join(dir, OutputFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return EventFile{}, fmt.Errorf("reading %q: %w", path, err)
	}
	var file EventFile
	if err := json.Unmarshal(b, &file); err != nil {
		return EventFile{}, fmt.Errorf("parsing %q: %w", path, err)
	}
	return file, nil
}

// SortRecords orders records deterministically for stable output files and
// diffs: by country code, then most-recent first, then title.
func SortRecords(records []GDELTEventRecord) {
	sort.SliceStable(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if a.CountryCode != b.CountryCode {
			return a.CountryCode < b.CountryCode
		}
		if !a.PublishedAt.Equal(b.PublishedAt) {
			return a.PublishedAt.After(b.PublishedAt)
		}
		return a.Title < b.Title
	})
}
