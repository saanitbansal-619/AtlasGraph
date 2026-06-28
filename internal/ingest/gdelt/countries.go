package gdelt

import "strings"

// countryNames maps the ISO3 codes AtlasGraph cares about to the display /
// query names GDELT understands. GDELT searches on free text, so we query by
// country name and record both the code and the name on every event.
//
// This is the codebase's first static ISO3 lookup; it is intentionally small
// and explicit (the supported set), mirroring the hand-curated mapping style
// used elsewhere (e.g. tradegraph's commodity→sector table).
var countryNames = map[string]string{
	"TWN": "Taiwan",
	"CHN": "China",
	"JPN": "Japan",
	"KOR": "South Korea",
	"USA": "United States",
	"DEU": "Germany",
	"SAU": "Saudi Arabia",
	"COD": "Democratic Republic of the Congo",
	"IND": "India",
}

// CountryName returns the display name for an ISO3 code and whether the code is
// one AtlasGraph knows about. Matching is case-insensitive.
func CountryName(code string) (string, bool) {
	name, ok := countryNames[strings.ToUpper(strings.TrimSpace(code))]
	return name, ok
}

// resolveCountryName returns the known display name for a code, or the upper-cased
// code itself when the code is unknown, so unrecognised codes still produce a
// usable query rather than being silently dropped.
func resolveCountryName(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if name, ok := countryNames[code]; ok {
		return name
	}
	return code
}
