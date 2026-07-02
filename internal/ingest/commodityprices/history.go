package commodityprices

import (
	"fmt"
	"sort"
	"strings"
)

// HistoryPoint is one monthly nominal price observation.
type HistoryPoint struct {
	Month string  `json:"month"`
	Price float64 `json:"price"`
}

// CommodityHistory is the price history for one commodity.
type CommodityHistory struct {
	Commodity string         `json:"commodity"`
	Source    string         `json:"source"`
	Points    []HistoryPoint `json:"points"`
}

// CommodityHistoryIndex lists commodities with available history.
type CommodityHistoryIndex struct {
	Source      string   `json:"source"`
	Commodities []string `json:"commodities"`
}

// ListHistoryCommodities returns distinct commodity names present in the file.
func ListHistoryCommodities(file PriceFile) CommodityHistoryIndex {
	seen := map[string]string{}
	for _, r := range file.Records {
		name := strings.TrimSpace(r.CommodityName)
		if name == "" {
			name = r.CommodityCode
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; !ok {
			seen[key] = name
		}
	}
	names := make([]string, 0, len(seen))
	for _, name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	source := file.Source
	if source == "" {
		source = SourceName
	}
	return CommodityHistoryIndex{Source: source, Commodities: names}
}

// HistoryForCommodity returns monthly price history for a commodity name or code.
func HistoryForCommodity(file PriceFile, commodity string) (CommodityHistory, error) {
	q := strings.TrimSpace(commodity)
	if q == "" {
		return CommodityHistory{}, fmt.Errorf("commodity is required")
	}
	qLower := strings.ToLower(q)
	qCode := normalizeCode(q)

	var points []HistoryPoint
	display := ""
	for _, r := range file.Records {
		nameLower := strings.ToLower(strings.TrimSpace(r.CommodityName))
		if nameLower != qLower && r.CommodityCode != qCode {
			continue
		}
		if display == "" {
			display = r.CommodityName
		}
		points = append(points, HistoryPoint{Month: r.Date, Price: r.PriceUSD})
	}
	if len(points) == 0 {
		return CommodityHistory{}, fmt.Errorf("no price history for commodity %q", commodity)
	}

	sort.SliceStable(points, func(i, j int) bool { return points[i].Month < points[j].Month })
	source := file.Source
	if source == "" {
		source = SourceName
	}
	return CommodityHistory{
		Commodity: display,
		Source:    source,
		Points:    points,
	}, nil
}
