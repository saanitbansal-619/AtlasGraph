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
	"usa":                                 "United States",
	"us":                                  "United States",
	"u.s.":                                "United States",
	"u.s.a.":                              "United States",
	"united states":                       "United States",
	"united states of america":            "United States",
	"korea, rep.":                         "Korea, Rep.",
	"korea rep":                           "Korea, Rep.",
	"korea rep.":                          "Korea, Rep.",
	"south korea":                         "Korea, Rep.",
	"republic of korea":                   "Korea, Rep.",
	"rep. of korea":                       "Korea, Rep.",
	"rep of korea":                        "Korea, Rep.",
	"taiwan":                              "Taiwan",
	"taiwan, china":                       "Taiwan",
	"taiwan china":                        "Taiwan",
	"other asia, nes":                     "Taiwan",
	"other asia nes":                      "Taiwan",
	"other asia, n.e.s.":                  "Taiwan",
	"other asia, not elsewhere specified": "Taiwan",
	"other asia not elsewhere specified":  "Taiwan",
	"china":                               "China",
	"germany":                             "Germany",
	"japan":                               "Japan",
	"canada":                              "Canada",
	"mexico":                              "Mexico",
	"türkiye":                             "Turkey",
	"turkiye":                             "Turkey",
	"turkey":                              "Turkey",
	"united kingdom":                      "United Kingdom",
	"uk":                                  "United Kingdom",
	"great britain":                       "Great Britain",
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

// CountryCodeForName returns a best-effort ISO3 code for a display country name.
func CountryCodeForName(name string) string {
	raw := strings.TrimSpace(name)
	if raw == "" {
		return ""
	}
	// Prefer alias → canonical → ISO3 so Comtrade variants resolve consistently.
	canon := NormalizeCountryName(raw)
	for _, candidate := range []string{canon, raw} {
		key := strings.ToLower(strings.TrimSpace(candidate))
		if key == "" {
			continue
		}
		if code, ok := countryISO3[key]; ok {
			return code
		}
	}
	return ""
}

// ResolveCountryCode returns code if non-empty, otherwise derives ISO3 from name.
func ResolveCountryCode(code, name string) string {
	if c := strings.ToUpper(strings.TrimSpace(code)); c != "" {
		return c
	}
	return CountryCodeForName(name)
}

var countryISO3 = map[string]string{
	"united states":                       "USA",
	"china":                               "CHN",
	"taiwan":                              "TWN",
	"taiwan, china":                       "TWN",
	"other asia, nes":                     "TWN",
	"other asia nes":                      "TWN",
	"other asia, n.e.s.":                  "TWN",
	"other asia, not elsewhere specified": "TWN",
	"other asia not elsewhere specified":  "TWN",
	"japan":                               "JPN",
	"korea, rep.":                         "KOR",
	"rep. of korea":                       "KOR",
	"rep of korea":                        "KOR",
	"republic of korea":                   "KOR",
	"south korea":                         "KOR",
	"germany":                             "DEU",
	"canada":                              "CAN",
	"mexico":                              "MEX",
	"united kingdom":                      "GBR",
	"great britain":                       "GBR",
	"france":                              "FRA",
	"india":                               "IND",
	"brazil":                              "BRA",
	"australia":                           "AUS",
	"italy":                               "ITA",
	"spain":                               "ESP",
	"netherlands":                         "NLD",
	"singapore":                           "SGP",
	"malaysia":                            "MYS",
	"turkey":                              "TUR",
	"russia":                              "RUS",
	"russian federation":                  "RUS",
	"saudi arabia":                        "SAU",
	"ukraine":                             "UKR",
	"iran":                                "IRN",
	"united arab emirates":                "ARE",
	"congo, dem. rep.":                    "COD",
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
