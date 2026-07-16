package eventrisk

import (
	"fmt"
	"strings"
)

// NormalizeCountry maps common aliases to a canonical country name used in GFIP.
// Returns the canonical name, ISO3 code when known, and whether the country is
// recognized. Unknown countries return ok=false.
func NormalizeCountry(raw string) (canonical string, iso3 string, ok bool) {
	key := normalizeKey(raw)
	if key == "" {
		return "", "", false
	}
	if canonical, ok = countryAliases[key]; ok {
		iso3 = isoByCanonical[canonical]
		return canonical, iso3, true
	}
	if canonical, ok = isoByCanonicalName[key]; ok {
		iso3 = isoByCanonical[canonical]
		return canonical, iso3, true
	}
	return "", "", false
}

func normalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// countryAliases maps lower-case aliases to canonical GFIP country names.
var countryAliases = map[string]string{
	"usa":                              "United States",
	"us":                               "United States",
	"u.s.":                             "United States",
	"u.s.a.":                           "United States",
	"united states":                    "United States",
	"united states of america":         "United States",
	"chn":                              "China",
	"twn":                              "Taiwan",
	"jpn":                              "Japan",
	"kor":                              "Korea, Rep.",
	"deu":                              "Germany",
	"ind":                              "India",
	"rus":                              "Russia",
	"ukr":                              "Ukraine",
	"sau":                              "Saudi Arabia",
	"chl":                              "Chile",
	"chile":                            "Chile",
	"cod":                              "Congo, Dem. Rep.",
	"korea, rep.":                      "Korea, Rep.",
	"korea rep":                        "Korea, Rep.",
	"korea rep.":                       "Korea, Rep.",
	"south korea":                      "Korea, Rep.",
	"republic of korea":                "Korea, Rep.",
	"russia":                           "Russia",
	"russian federation":               "Russia",
	"china":                            "China",
	"people's republic of china":        "China",
	"peoples republic of china":        "China",
	"prc":                              "China",
	"iran":                             "Iran",
	"islamic republic of iran":         "Iran",
	"uae":                              "United Arab Emirates",
	"united arab emirates":             "United Arab Emirates",
	"drc":                              "Congo, Dem. Rep.",
	"congo democratic republic":        "Congo, Dem. Rep.",
	"democratic republic of the congo": "Congo, Dem. Rep.",
	"congo, dem. rep.":                 "Congo, Dem. Rep.",
	"congo dem rep":                    "Congo, Dem. Rep.",
	"ukraine":                          "Ukraine",
	"taiwan":                           "Taiwan",
	"turkey":                           "Turkey",
	"türkiye":                          "Turkey",
	"turkiye":                          "Turkey",
	"japan":                            "Japan",
	"germany":                          "Germany",
	"saudi arabia":                     "Saudi Arabia",
	"india":                            "India",
	"united kingdom":                   "United Kingdom",
	"uk":                               "United Kingdom",
	"great britain":                    "United Kingdom",
	"france":                           "France",
	"brazil":                           "Brazil",
	"australia":                        "Australia",
	"canada":                           "Canada",
	"mexico":                           "Mexico",
	"italy":                            "Italy",
	"spain":                            "Spain",
	"netherlands":                      "Netherlands",
	"singapore":                        "Singapore",
	"malaysia":                         "Malaysia",
}

// isoByCanonical maps canonical names to ISO3 codes for fragility matching.
var isoByCanonical = map[string]string{
	"United States":        "USA",
	"Korea, Rep.":          "KOR",
	"Russia":               "RUS",
	"China":                "CHN",
	"Iran":                 "IRN",
	"United Arab Emirates": "ARE",
	"Congo, Dem. Rep.":     "COD",
	"Ukraine":              "UKR",
	"Taiwan":               "TWN",
	"Japan":                "JPN",
	"Germany":              "DEU",
	"Saudi Arabia":         "SAU",
	"India":                "IND",
	"Chile":                "CHL",
	"United Kingdom":       "GBR",
	"France":               "FRA",
	"Brazil":               "BRA",
	"Australia":            "AUS",
	"Canada":               "CAN",
	"Mexico":               "MEX",
	"Italy":                "ITA",
	"Spain":                "ESP",
	"Netherlands":          "NLD",
	"Singapore":            "SGP",
	"Malaysia":             "MYS",
	"Turkey":               "TUR",
}

var isoByCanonicalName map[string]string

func init() {
	isoByCanonicalName = make(map[string]string, len(isoByCanonical))
	for name := range isoByCanonical {
		isoByCanonicalName[normalizeKey(name)] = name
	}
}

// ISO3ForCountry returns the ISO3 code for a canonical country name.
func ISO3ForCountry(canonical string) string {
	return isoByCanonical[canonical]
}

// WarnUnknownCountry formats a skip warning for an unrecognized country label.
func WarnUnknownCountry(raw string) string {
	return fmt.Sprintf("skipping event with unknown country %q", strings.TrimSpace(raw))
}
