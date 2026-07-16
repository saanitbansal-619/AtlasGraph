package macroingest

import (
	"strings"

	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
)

// APIExcludedCountries are ISO3 codes omitted from World Bank API requests.
// They remain in the requested panel and are scored with missing indicators.
var APIExcludedCountries = map[string]string{
	"TWN": "Taiwan",
}

// APICountryCodes returns API-safe country codes and skipped codes with names.
func APICountryCodes(codes []string) (apiCodes []string, skipped map[string]string) {
	skipped = map[string]string{}
	for _, code := range codes {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}
		if name, excluded := APIExcludedCountries[code]; excluded {
			skipped[code] = name
			continue
		}
		apiCodes = append(apiCodes, code)
	}
	return apiCodes, skipped
}

// CountryDisplayName returns a human label for a country code when known.
func CountryDisplayName(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if name, ok := APIExcludedCountries[code]; ok {
		return name
	}
	return ""
}

// PipelineIndicatorCodes lists the indicator series expected in macro scores.
func PipelineIndicatorCodes() []string {
	out := make([]string, 0, len(PipelineIndicators))
	for _, ind := range PipelineIndicators {
		out = append(out, ind.Code)
	}
	return out
}

// PlaceholderRecords creates empty country records for API-skipped countries.
func PlaceholderRecords(skipped map[string]string) []worldbank.CountryIndicatorRecord {
	if len(skipped) == 0 {
		return nil
	}
	var out []worldbank.CountryIndicatorRecord
	for code, name := range skipped {
		out = append(out, worldbank.CountryIndicatorRecord{
			CountryCode: code,
			CountryName: name,
		})
	}
	return out
}
