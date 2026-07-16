package trade

import (
	"path/filepath"
	"strings"
)

// hsCommodityMap maps HS chapter/heading/subheading codes to GFIP commodity names.
// Longer codes are preferred by MapHSCode (6-digit before 4-digit).
var hsCommodityMap = map[string]string{
	"1001":   "wheat",
	"2709":   "crude oil",
	"2711":   "natural gas",
	"7403":   "copper",
	"7502":   "nickel",
	"3105":   "fertilizer",
	"8542":   "semiconductors",
	"8507":   "batteries",
	"283691": "lithium",
	"280530": "rare earths",
	"284690": "rare earths",
	"260500": "cobalt",
	"810520": "cobalt",
	"810530": "cobalt",
}

// filenameCommodityHints maps filename tokens to GFIP commodity names.
var filenameCommodityHints = map[string]string{
	"lithium_carbonates":    "lithium",
	"lithium_carbonate":     "lithium",
	"lithium":               "lithium",
	"rare_earth_metals":     "rare earths",
	"rare_earth_compounds":  "rare earths",
	"rare_earths":           "rare earths",
	"rare_earth":            "rare earths",
	"cobalt_ores":           "cobalt",
	"cobalt_intermediate":   "cobalt",
	"cobalt_unwrought":      "cobalt",
	"cobalt":                "cobalt",
}

// MapHSCode maps an HS code to a GFIP commodity name. Returns ok=false when
// unmapped. Prefers longer exact matches (6-digit) before 4-digit headings.
func MapHSCode(code string) (string, bool) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", false
	}
	digits := extractDigits(code)
	if digits == "" {
		return "", false
	}
	if name, ok := hsCommodityMap[digits]; ok {
		return name, true
	}
	if len(digits) >= 6 {
		if name, ok := hsCommodityMap[digits[:6]]; ok {
			return name, true
		}
	}
	if len(digits) >= 4 {
		if name, ok := hsCommodityMap[digits[:4]]; ok {
			return name, true
		}
	}
	return "", false
}

func extractDigits(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// MapCommodityFilename maps a source CSV filename to a GFIP commodity when the
// basename contains a known strategic-commodity token.
func MapCommodityFilename(path string) (string, bool) {
	base := strings.ToLower(filepath.Base(path))
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "-", "_")
	base = strings.ReplaceAll(base, " ", "_")

	// Prefer longer / more specific keys first.
	keys := make([]string, 0, len(filenameCommodityHints))
	for k := range filenameCommodityHints {
		keys = append(keys, k)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if len(keys[j]) > len(keys[i]) {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	for _, k := range keys {
		if strings.Contains(base, k) {
			return filenameCommodityHints[k], true
		}
	}
	return "", false
}

// commodityFromHSOrDesc uses HS mapping first, then description fallback,
// then an optional filename hint (for single-commodity Comtrade exports).
func commodityFromHSOrDesc(hsCode, desc, filenameHint string) (string, bool) {
	if name, ok := MapHSCode(hsCode); ok {
		return name, true
	}
	desc = strings.ToLower(strings.TrimSpace(desc))
	switch {
	case strings.Contains(desc, "semiconductor"), strings.Contains(desc, "integrated circuit"):
		return "semiconductors", true
	case strings.Contains(desc, "battery"), strings.Contains(desc, "accumulator"):
		return "batteries", true
	case strings.Contains(desc, "lithium"):
		return "lithium", true
	case strings.Contains(desc, "rare earth"), strings.Contains(desc, "rare-earth"):
		return "rare earths", true
	case strings.Contains(desc, "cobalt"):
		return "cobalt", true
	case strings.Contains(desc, "wheat"):
		return "wheat", true
	case strings.Contains(desc, "crude"), strings.Contains(desc, "petroleum oil"):
		return "crude oil", true
	case strings.Contains(desc, "natural gas"):
		return "natural gas", true
	case strings.Contains(desc, "copper"):
		return "copper", true
	case strings.Contains(desc, "nickel"):
		return "nickel", true
	case strings.Contains(desc, "fertil"):
		return "fertilizer", true
	}
	if filenameHint != "" {
		if name, ok := MapCommodityFilename(filenameHint); ok {
			return name, true
		}
	}
	return "", false
}
