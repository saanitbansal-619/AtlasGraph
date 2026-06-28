package gdelt

import "sort"

// NamedCount is a label paired with an event/term frequency, used for the
// "top countries" and "top risk terms" leaderboards in the ingest report.
type NamedCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Summary is a high-level digest of an ingested GDELT event panel.
type Summary struct {
	Records       int          `json:"records"`
	WithRiskTerms int          `json:"records_with_risk_terms"`
	TopCountries  []NamedCount `json:"top_countries"`
	TopRiskTerms  []NamedCount `json:"top_risk_terms"`
}

// BuildSummary aggregates an EventFile into a Summary, keeping the top n entries
// in each leaderboard (n <= 0 keeps all).
func BuildSummary(file EventFile, n int) Summary {
	s := Summary{Records: len(file.Records)}

	countryCounts := map[string]int{}
	countryName := map[string]string{}
	termCounts := map[string]int{}

	for _, r := range file.Records {
		key := r.CountryCode
		countryCounts[key]++
		if countryName[key] == "" {
			if r.CountryName != "" {
				countryName[key] = r.CountryName
			} else {
				countryName[key] = r.CountryCode
			}
		}
		if len(r.RiskTermsMatched) > 0 {
			s.WithRiskTerms++
		}
		for _, t := range r.RiskTermsMatched {
			termCounts[t]++
		}
	}

	for code, count := range countryCounts {
		s.TopCountries = append(s.TopCountries, NamedCount{Name: countryName[code], Count: count})
	}
	for term, count := range termCounts {
		s.TopRiskTerms = append(s.TopRiskTerms, NamedCount{Name: term, Count: count})
	}

	sortNamedCounts(s.TopCountries)
	sortNamedCounts(s.TopRiskTerms)
	s.TopCountries = topN(s.TopCountries, n)
	s.TopRiskTerms = topN(s.TopRiskTerms, n)
	return s
}

func sortNamedCounts(items []NamedCount) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count != items[j].Count {
			return items[i].Count > items[j].Count
		}
		return items[i].Name < items[j].Name
	})
}

func topN(items []NamedCount, n int) []NamedCount {
	if n > 0 && len(items) > n {
		return items[:n]
	}
	return items
}
