package eventrisk

import (
	"strings"

	"github.com/atlasgraph/atlas/internal/scoring/events"
)

// ToLegacyCountryScores converts processed country risk rows into the legacy
// events.CountryScore shape used by fragility scoring and the existing API.
func ToLegacyCountryScores(file RiskFile) []events.CountryScore {
	out := make([]events.CountryScore, 0, len(file.Countries))
	for _, c := range file.Countries {
		code := c.CountryCode
		if code == "" {
			code = ISO3ForCountry(c.Country)
		}
		if code == "" {
			code = strings.ToUpper(strings.ReplaceAll(c.Country, " ", ""))
		}
		comps := c.Components
		if len(comps) == 0 {
			comps = []events.Component{
				{Key: "recent_events", Name: "recent events", Score: c.EventRiskScore, Weight: weightRecentEvents, Contribution: c.EventRiskScore * weightRecentEvents},
				{Key: "negative_tone", Name: "negative tone", Score: c.EventRiskScore, Weight: weightNegativeTone, Contribution: c.EventRiskScore * weightNegativeTone},
				{Key: "event_severity", Name: "event severity", Score: c.EventRiskScore, Weight: weightSeverity, Contribution: c.EventRiskScore * weightSeverity},
			}
		}
		out = append(out, events.CountryScore{
			CountryCode: code,
			CountryName: c.Country,
			Events:      c.EventCount,
			AvgTone:     c.AverageTone,
			Score:       c.EventRiskScore,
			RiskLevel:   c.RiskLevel,
			TopDrivers:  topDriversFromComponents(comps, 2),
			TopTerms:    c.TopEventTypes,
			Components:  comps,
		})
	}
	return out
}

func topDriversFromComponents(comps []events.Component, n int) []string {
	avail := make([]events.Component, 0, len(comps))
	for _, c := range comps {
		if c.Contribution > 0 {
			avail = append(avail, c)
		}
	}
	if len(avail) == 0 {
		return []string{"recent events", "negative tone", "event severity"}
	}
	// sort by contribution desc
	for i := 0; i < len(avail); i++ {
		for j := i + 1; j < len(avail); j++ {
			if avail[j].Contribution > avail[i].Contribution {
				avail[i], avail[j] = avail[j], avail[i]
			}
		}
	}
	if len(avail) > n {
		avail = avail[:n]
	}
	out := make([]string, 0, len(avail))
	for _, c := range avail {
		out = append(out, c.Name)
	}
	return out
}
