package eventrisk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsRealProcessedEventRisk reports whether a loaded panel is real processed GDELT
// event risk (as opposed to demo/legacy fallback data).
func IsRealProcessedEventRisk(file RiskFile) bool {
	if len(file.Countries) == 0 {
		return false
	}
	src := strings.ToLower(strings.TrimSpace(file.Source))
	if src == "" {
		return true
	}
	if strings.Contains(src, "demo") || strings.Contains(src, "sample") || strings.Contains(src, "fixture") || strings.Contains(src, "synthetic") {
		return false
	}
	return strings.Contains(src, "gdelt") || strings.EqualFold(file.Source, SourceName)
}

// TryLoadProcessed reads dir/event_risk.json when present and valid.
func TryLoadProcessed(dir string) (RiskFile, bool) {
	if strings.TrimSpace(dir) == "" {
		return RiskFile{}, false
	}
	file, err := Load(dir)
	if err != nil || len(file.Countries) == 0 {
		return RiskFile{}, false
	}
	return file, true
}

// IndexCountryScores builds a lookup map keyed by normalized country names and
// common aliases for graph/simulation matching.
func IndexCountryScores(file RiskFile) map[string]float64 {
	out := map[string]float64{}
	for _, c := range file.Countries {
		score := c.EventRiskScore
		canonical := NormalizeCountryName(c.Country)
		for _, name := range []string{canonical, c.Country} {
			key := strings.ToLower(strings.TrimSpace(name))
			if key == "" {
				continue
			}
			if prev, ok := out[key]; !ok || score > prev {
				out[key] = score
			}
		}
		registerAliasScores(out, canonical, score)
	}
	return out
}

func registerAliasScores(out map[string]float64, canonical string, score float64) {
	for alias, target := range countryAliases {
		if target != canonical {
			continue
		}
		if prev, ok := out[alias]; ok && prev >= score {
			continue
		}
		out[alias] = score
	}
}

// LookupScore finds an event-risk score for a graph country display name.
func LookupScore(index map[string]float64, countryName string) (float64, bool) {
	if len(index) == 0 {
		return 0, false
	}
	candidates := []string{
		strings.ToLower(strings.TrimSpace(countryName)),
		strings.ToLower(strings.TrimSpace(NormalizeCountryName(countryName))),
	}
	seen := map[string]struct{}{}
	for _, key := range candidates {
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		if v, ok := index[key]; ok {
			return v, true
		}
	}
	return 0, false
}

// ParseRiskFileJSON decodes event_risk.json flexibly: standard RiskFile,
// {countries:[...]}, {risks:[...]}, or a top-level country array.
func ParseRiskFileJSON(b []byte) (RiskFile, error) {
	trimmed := strings.TrimSpace(string(b))
	if strings.HasPrefix(trimmed, "[") {
		var rows []CountryRisk
		if err := json.Unmarshal(b, &rows); err == nil && len(rows) > 0 && strings.TrimSpace(rows[0].Country) != "" {
			file := RiskFile{Countries: rows, Source: SourceName}
			return normalizeRiskFile(file), nil
		}
	}

	var file RiskFile
	if err := json.Unmarshal(b, &file); err != nil {
		return RiskFile{}, err
	}
	if len(file.Countries) > 0 {
		return normalizeRiskFile(file), nil
	}

	var wrapped struct {
		Source    string        `json:"source"`
		DateFrom  string        `json:"date_from"`
		DateTo    string        `json:"date_to"`
		Countries []CountryRisk `json:"countries"`
		Risks     []CountryRisk `json:"risks"`
		Scores    []CountryRisk `json:"scores"`
	}
	if err := json.Unmarshal(b, &wrapped); err == nil {
		switch {
		case len(wrapped.Countries) > 0:
			file.Countries = wrapped.Countries
		case len(wrapped.Risks) > 0:
			file.Countries = wrapped.Risks
		case len(wrapped.Scores) > 0:
			file.Countries = wrapped.Scores
		}
		if file.Source == "" {
			file.Source = wrapped.Source
		}
		if file.DateFrom == "" {
			file.DateFrom = wrapped.DateFrom
		}
		if file.DateTo == "" {
			file.DateTo = wrapped.DateTo
		}
	}
	if len(file.Countries) > 0 {
		return normalizeRiskFile(file), nil
	}

	return RiskFile{}, fmt.Errorf("no country risk rows found")
}

func normalizeRiskFile(file RiskFile) RiskFile {
	if file.Source == "" {
		file.Source = SourceName
	}
	for i := range file.Countries {
		c := &file.Countries[i]
		if canonical := NormalizeCountryName(c.Country); canonical != "" {
			c.Country = canonical
		}
		if c.CountryCode == "" {
			c.CountryCode = ISO3ForCountry(c.Country)
		}
		if c.Source == "" {
			c.Source = file.Source
		}
	}
	return file
}

// LoadFile reads an explicit event_risk.json path (file or directory).
func LoadFile(path string) (RiskFile, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return RiskFile{}, fmt.Errorf("path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return RiskFile{}, err
	}
	if info.IsDir() {
		return Load(path)
	}
	if filepath.Base(path) != OutputFileName {
		return RiskFile{}, fmt.Errorf("expected %q file, got %q", OutputFileName, filepath.Base(path))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return RiskFile{}, err
	}
	return ParseRiskFileJSON(b)
}
