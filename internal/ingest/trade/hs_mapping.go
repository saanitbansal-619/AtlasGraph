package trade

import "strings"

// hsCommodityMap maps HS chapter/heading codes to GFIP commodity names.
var hsCommodityMap = map[string]string{
	"1001": "wheat",
	"2709": "crude oil",
	"2711": "natural gas",
	"7403": "copper",
	"7502": "nickel",
	"3105": "fertilizer",
	"8542": "semiconductors",
	"8507": "batteries",
}

// MapHSCode maps an HS code to a GFIP commodity name. Returns ok=false when
// unmapped.
func MapHSCode(code string) (string, bool) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", false
	}
	digits := extractDigits(code)
	if len(digits) >= 4 {
		if name, ok := hsCommodityMap[digits[:4]]; ok {
			return name, true
		}
	}
	if name, ok := hsCommodityMap[digits]; ok {
		return name, true
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

// commodityFromHSOrDesc uses HS mapping first, then description fallback.
func commodityFromHSOrDesc(hsCode, desc string) (string, bool) {
	if name, ok := MapHSCode(hsCode); ok {
		return name, true
	}
	desc = strings.ToLower(strings.TrimSpace(desc))
	switch {
	case strings.Contains(desc, "semiconductor"), strings.Contains(desc, "integrated circuit"):
		return "semiconductors", true
	case strings.Contains(desc, "battery"), strings.Contains(desc, "accumulator"):
		return "batteries", true
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
	default:
		return "", false
	}
}
