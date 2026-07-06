package trade

import "strings"

// ComtradeRealSourceName is the provenance label for processed UN Comtrade data.
const ComtradeRealSourceName = "UN Comtrade"

// NormalizeCountryName maps common trade country aliases to canonical names.
func NormalizeCountryName(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return ""
	}
	if canon, ok := countryAliases[key]; ok {
		return canon
	}
	return strings.TrimSpace(raw)
}

var countryAliases = map[string]string{
	"usa":                      "United States",
	"us":                       "United States",
	"u.s.":                     "United States",
	"u.s.a.":                   "United States",
	"united states":            "United States",
	"united states of america": "United States",
	"korea, rep.":              "Korea, Rep.",
	"korea rep":                "Korea, Rep.",
	"korea rep.":               "Korea, Rep.",
	"south korea":              "Korea, Rep.",
	"republic of korea":        "Korea, Rep.",
	"china":                    "China",
	"germany":                  "Germany",
	"japan":                    "Japan",
	"canada":                   "Canada",
	"mexico":                   "Mexico",
	"türkiye":                  "Turkey",
	"turkiye":                  "Turkey",
	"turkey":                   "Turkey",
	"united kingdom":           "United Kingdom",
	"uk":                       "United Kingdom",
	"great britain":            "Great Britain",
}

// importerAliases maps query aliases to canonical importer names used in data.
var importerAliases = map[string]string{
	"usa":                      "United States",
	"us":                       "United States",
	"united states of america": "United States",
}

// NormalizeImporterQuery normalizes an importer query for dependency lookups.
func NormalizeImporterQuery(q string) string {
	key := strings.ToLower(strings.TrimSpace(q))
	if key == "" {
		return ""
	}
	if canon, ok := importerAliases[key]; ok {
		return canon
	}
	return NormalizeCountryName(q)
}

// isAggregatePartner reports whether a partner label is a Comtrade aggregate row.
func isAggregatePartner(partner string) bool {
	key := strings.ToLower(strings.TrimSpace(partner))
	switch key {
	case "world", "total", "all", "areas, nes", "areas nes", "areas n.e.s.":
		return true
	default:
		return false
	}
}

// looksLikeCountry returns true when a partner label is plausibly a country name.
func looksLikeCountry(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || isAggregatePartner(name) {
		return false
	}
	for _, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
	}
	return false
}
