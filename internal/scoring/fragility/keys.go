package fragility

import (
	"strings"
	"unicode"

	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
)

// countryNameAliases maps normalized country name tokens to a preferred ISO3
// code. This bridges graph display names, trade labels and macro/event codes.
var countryNameAliases = map[string]string{
	"south_korea":                      "KOR",
	"korea_rep":                        "KOR",
	"korea_republic":                   "KOR",
	"republic_of_korea":                "KOR",
	"korea_south":                      "KOR",
	"united_states":                    "USA",
	"united_states_of_america":         "USA",
	"democratic_republic_of_the_congo": "COD",
	"dem_rep_of_the_congo":             "COD",
	"democratic_republic_of_congo":     "COD",
	"congo_democratic_republic":        "COD",
	"congo_dem_rep":                    "COD",
	"dr_congo":                         "COD",
	"drc":                              "COD",
	"congo_drc":                        "COD",
	"saudi_arabia":                     "SAU",
	"kingdom_of_saudi_arabia":          "SAU",
}

// countryCodeAliases maps alternate ISO-style codes to a canonical ISO3 code.
var countryCodeAliases = map[string]string{
	"DRC": "COD",
}

// countryKey returns the canonical deduplication key for a country. ISO3 codes
// win when known; otherwise a normalized name slug is used.
func countryKey(code, name string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	name = strings.TrimSpace(name)

	if canon := canonicalCountryCode(code); canon != "" {
		return canon
	}
	if canon := canonicalCountryCodeFromName(name); canon != "" {
		return canon
	}
	if code != "" {
		return code
	}
	if name != "" {
		return "name:" + normalizeNameSlug(name)
	}
	return ""
}

// commodityKey returns the canonical deduplication key for a commodity.
// Spaces and underscores are treated consistently ("crude oil" == "crude_oil").
func commodityKey(code, name string) string {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)

	if code != "" {
		return normalizeCommodityKey(code)
	}
	if name != "" {
		return normalizeCommodityKey(name)
	}
	return ""
}

func normalizeCommodityKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, " ", "_")
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	return strings.Trim(s, "_")
}

func normalizeNameSlug(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, ch := range []string{",", ".", "'", "(", ")", ";", ":"} {
		name = strings.ReplaceAll(name, ch, " ")
	}
	name = strings.Join(strings.Fields(name), " ")
	return normalizeCommodityKey(name)
}

func canonicalCountryCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return ""
	}
	if alias, ok := countryCodeAliases[code]; ok {
		return alias
	}
	if _, ok := gdelt.CountryName(code); ok {
		return code
	}
	if len(code) == 3 && code != "" {
		return code
	}
	return ""
}

func canonicalCountryCodeFromName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	slug := normalizeNameSlug(name)
	if code, ok := countryNameAliases[slug]; ok {
		return code
	}
	for _, code := range knownCountryCodes() {
		if n, ok := gdelt.CountryName(code); ok && namesMatch(name, n) {
			return code
		}
	}
	if len(name) == 3 && strings.ToUpper(name) == name {
		if canon := canonicalCountryCode(name); canon != "" {
			return canon
		}
	}
	return ""
}

func mergeCountryRef(existing, incoming countryRef) countryRef {
	merged := countryRef{
		code: firstNonEmpty(existing.code, incoming.code),
		name: preferCountryDisplayName(existing.name, incoming.name),
	}
	canon := countryKey(merged.code, merged.name)
	if strings.HasPrefix(canon, "name:") {
		merged.code = ""
		if merged.name == "" {
			merged.name = strings.ReplaceAll(strings.TrimPrefix(canon, "name:"), "_", " ")
		}
		return merged
	}
	merged.code = canon
	if display, ok := gdelt.CountryName(canon); ok {
		merged.name = preferCountryDisplayName(merged.name, display)
	}
	return merged
}

func mergeCommodityRef(existing, incoming commodityRef) commodityRef {
	merged := commodityRef{
		code: firstNonEmpty(existing.code, incoming.code),
		name: preferCommodityDisplayName(existing.name, incoming.name),
	}
	key := commodityKey(merged.code, merged.name)
	if merged.code == "" && merged.name != "" {
		merged.code = key
	}
	merged.name = preferCommodityDisplayName(merged.name, strings.ReplaceAll(key, "_", " "))
	return merged
}

func preferCountryDisplayName(names ...string) string {
	best := ""
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if best == "" {
			best = n
			continue
		}
		// Prefer the longer, more descriptive label (e.g. "South Korea" over "KOR").
		if displayScore(n) > displayScore(best) {
			best = n
		}
	}
	return best
}

func preferCommodityDisplayName(names ...string) string {
	best := ""
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if best == "" {
			best = n
			continue
		}
		// Prefer spaced display names over underscore codes when both refer to
		// the same canonical commodity ("crude oil" over "crude_oil").
		if strings.Contains(n, " ") && !strings.Contains(best, " ") {
			best = n
			continue
		}
		if displayScore(n) > displayScore(best) {
			best = n
		}
	}
	return best
}

func displayScore(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	score := len(s)
	if strings.Contains(s, " ") {
		score += 10
	}
	for _, r := range s {
		if unicode.IsLetter(r) && unicode.IsUpper(r) {
			score++
		}
	}
	return score
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// countryRefsMatch reports whether two country refs denote the same canonical
// country.
func countryRefsMatch(a, b countryRef) bool {
	return countryKey(a.code, a.name) == countryKey(b.code, b.name)
}

// commodityRefsMatch reports whether two commodity refs denote the same
// canonical commodity.
func commodityRefsMatch(a, b commodityRef) bool {
	return commodityKey(a.code, a.name) == commodityKey(b.code, b.name)
}
