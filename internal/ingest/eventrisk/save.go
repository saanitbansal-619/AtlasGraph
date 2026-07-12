package eventrisk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Save writes a RiskFile to dir/OutputFileName.
func Save(dir string, file RiskFile) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating output dir: %w", err)
	}
	path := filepath.Join(dir, OutputFileName)
	b, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encoding event risk file: %w", err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("writing %s: %w", path, err)
	}
	return path, nil
}

// Load reads dir/OutputFileName.
func Load(dir string) (RiskFile, error) {
	path := filepath.Join(dir, OutputFileName)
	b, err := os.ReadFile(path)
	if err != nil {
		return RiskFile{}, fmt.Errorf("reading %s: %w", path, err)
	}
	file, err := ParseRiskFileJSON(b)
	if err != nil {
		return RiskFile{}, fmt.Errorf("decoding %s: %w", path, err)
	}
	return file, nil
}

// RecentEventsForCountry returns up to n most recent events for a country name.
func RecentEventsForCountry(file RiskFile, country string, n int) []NormalizedEvent {
	q := normalizeKey(country)
	if q == "" {
		return nil
	}
	var matched []NormalizedEvent
	for _, e := range file.Events {
		if normalizeKey(e.Country) == q || normalizeKey(NormalizeCountryName(e.Country)) == q {
			matched = append(matched, e)
		}
	}
	sortEventsDesc(matched)
	if n > 0 && len(matched) > n {
		matched = matched[:n]
	}
	return matched
}

// CountryRiskFor returns one country's risk row when present.
func CountryRiskFor(file RiskFile, country string) (CountryRisk, bool) {
	q := strings.ToLower(strings.TrimSpace(NormalizeCountryName(country)))
	if q == "" {
		q = normalizeKey(country)
	}
	for _, c := range file.Countries {
		canon := strings.ToLower(strings.TrimSpace(NormalizeCountryName(c.Country)))
		if canon == q || normalizeKey(c.Country) == q {
			return c, true
		}
	}
	return CountryRisk{}, false
}

// NormalizeCountryName is a helper for matching canonical names.
func NormalizeCountryName(name string) string {
	if canonical, _, ok := NormalizeCountry(name); ok {
		return canonical
	}
	return name
}

func sortEventsDesc(events []NormalizedEvent) {
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if events[j].Date > events[i].Date {
				events[i], events[j] = events[j], events[i]
			}
		}
	}
}
