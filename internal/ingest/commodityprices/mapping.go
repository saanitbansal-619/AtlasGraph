package commodityprices

import "strings"

// MappedCommodity is a GFIP-normalised commodity target for a Pink Sheet series.
type MappedCommodity struct {
	Name     string
	Code     string
	Priority int
}

type mappingRule struct {
	match    func(normalized string) bool
	commodity MappedCommodity
}

// strategicGFIPCommodities are graph commodities that Pink Sheet typically does
// not cover. Ingest reports them as missing rather than failing.
var strategicGFIPCommodities = []string{
	"semiconductors",
	"batteries",
	"solar panels",
	"pharmaceuticals",
	"shipping containers",
	"rare earths",
	"lithium",
	"cobalt",
}

// pinkSheetRules map World Bank Pink Sheet series names to GFIP commodities.
// Higher priority wins when multiple series map to the same GFIP code.
var pinkSheetRules = []mappingRule{
	{match: exact("crude oil, average"), commodity: MappedCommodity{Name: "crude oil", Code: "crude_oil", Priority: 100}},
	{match: containsAll("crude oil", "brent"), commodity: MappedCommodity{Name: "crude oil", Code: "crude_oil", Priority: 80}},
	{match: containsAll("crude oil", "wti"), commodity: MappedCommodity{Name: "crude oil", Code: "crude_oil", Priority: 70}},
	{match: exact("natural gas, europe"), commodity: MappedCommodity{Name: "natural gas", Code: "natural_gas", Priority: 100}},
	{match: exact("natural gas, u.s."), commodity: MappedCommodity{Name: "natural gas", Code: "natural_gas", Priority: 95}},
	{match: exact("natural gas, us"), commodity: MappedCommodity{Name: "natural gas", Code: "natural_gas", Priority: 90}},
	{match: containsAll("liquefied natural gas", "japan"), commodity: MappedCommodity{Name: "LNG", Code: "lng", Priority: 100}},
	{match: containsAll("lng", "japan"), commodity: MappedCommodity{Name: "LNG", Code: "lng", Priority: 90}},
	{match: exact("aluminum"), commodity: MappedCommodity{Name: "aluminum", Code: "aluminum", Priority: 100}},
	{match: exact("copper"), commodity: MappedCommodity{Name: "copper", Code: "copper", Priority: 100}},
	{match: exact("nickel"), commodity: MappedCommodity{Name: "nickel", Code: "nickel", Priority: 100}},
	{match: containsAll("wheat", "hrw"), commodity: MappedCommodity{Name: "wheat", Code: "wheat", Priority: 100}},
	{match: exact("maize"), commodity: MappedCommodity{Name: "corn", Code: "corn", Priority: 100}},
	{match: containsAll("rice", "thai"), commodity: MappedCommodity{Name: "rice", Code: "rice", Priority: 100}},
	{match: exact("urea"), commodity: MappedCommodity{Name: "fertilizer", Code: "fertilizer", Priority: 90}},
	{match: exact("dap"), commodity: MappedCommodity{Name: "fertilizer", Code: "fertilizer", Priority: 85}},
	{match: containsAll("potassium", "chloride"), commodity: MappedCommodity{Name: "fertilizer", Code: "fertilizer", Priority: 80}},
	{match: exact("cobalt"), commodity: MappedCommodity{Name: "cobalt", Code: "cobalt", Priority: 100}},
	{match: containsAll("lithium", "carbonate"), commodity: MappedCommodity{Name: "lithium", Code: "lithium", Priority: 100}},
	{match: exact("gold"), commodity: MappedCommodity{Name: "gold", Code: "gold", Priority: 100}},
	{match: exact("silver"), commodity: MappedCommodity{Name: "silver", Code: "silver", Priority: 100}},
	{match: exact("tin"), commodity: MappedCommodity{Name: "tin", Code: "tin", Priority: 100}},
	{match: exact("zinc"), commodity: MappedCommodity{Name: "zinc", Code: "zinc", Priority: 100}},
	{match: exact("iron ore"), commodity: MappedCommodity{Name: "iron ore", Code: "iron_ore", Priority: 100}},
	{match: exact("coal, australia"), commodity: MappedCommodity{Name: "coal", Code: "coal", Priority: 100}},
}

func normalizePinkSheetHeader(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "\u00a0", " ")
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

func exact(want string) func(string) bool {
	want = normalizePinkSheetHeader(want)
	return func(normalized string) bool { return normalized == want }
}

func containsAll(parts ...string) func(string) bool {
	norm := make([]string, len(parts))
	for i, p := range parts {
		norm[i] = normalizePinkSheetHeader(p)
	}
	return func(normalized string) bool {
		for _, p := range norm {
			if !strings.Contains(normalized, p) {
				return false
			}
		}
		return true
	}
}

// MapPinkSheetSeries maps a Pink Sheet column header to a GFIP commodity.
func MapPinkSheetSeries(header string) (MappedCommodity, bool) {
	n := normalizePinkSheetHeader(header)
	if n == "" || n == "commodity" || n == "date" {
		return MappedCommodity{}, false
	}
	var best *MappedCommodity
	for _, rule := range pinkSheetRules {
		if !rule.match(n) {
			continue
		}
		c := rule.commodity
		if best == nil || c.Priority > best.Priority {
			copy := c
			best = &copy
		}
	}
	if best == nil {
		return MappedCommodity{}, false
	}
	return *best, true
}

// StrategicGFIPCommoditiesMissing reports strategic graph commodities that have
// no mapped Pink Sheet series in the ingested columns.
func StrategicGFIPCommoditiesMissing(mappedCodes map[string]struct{}) []string {
	var out []string
	for _, name := range strategicGFIPCommodities {
		code := normalizeCode(name)
		if _, ok := mappedCodes[code]; !ok {
			out = append(out, name)
		}
	}
	return out
}

// IsRealPriceSource reports whether processed price data came from World Bank Pink Sheet.
func IsRealPriceSource(source string) bool {
	s := strings.ToLower(strings.TrimSpace(source))
	return strings.Contains(s, "world bank") || strings.Contains(s, "pink sheet")
}
