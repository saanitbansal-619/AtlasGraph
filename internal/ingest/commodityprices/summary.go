package commodityprices

// Summary is a high-level digest of an ingested commodity price panel, used by
// the ingest command's report.
type Summary struct {
	Records     int    `json:"records"`
	Commodities int    `json:"commodities"`
	FirstMonth  string `json:"first_month"`
	LastMonth   string `json:"last_month"`
}

// BuildSummary aggregates a PriceFile into a Summary: the distinct commodity
// count and the inclusive month range present in the data.
func BuildSummary(file PriceFile) Summary {
	s := Summary{Records: len(file.Records)}
	seen := map[string]struct{}{}
	for _, r := range file.Records {
		if _, ok := seen[r.CommodityCode]; !ok {
			seen[r.CommodityCode] = struct{}{}
		}
		if s.FirstMonth == "" || r.Date < s.FirstMonth {
			s.FirstMonth = r.Date
		}
		if s.LastMonth == "" || r.Date > s.LastMonth {
			s.LastMonth = r.Date
		}
	}
	s.Commodities = len(seen)
	return s
}
