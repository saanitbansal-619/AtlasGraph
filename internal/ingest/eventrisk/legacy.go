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
		out = append(out, events.CountryScore{
			CountryCode: code,
			CountryName: c.Country,
			Events:      c.EventCount,
			AvgTone:     c.AverageTone,
			Score:       c.EventRiskScore,
			RiskLevel:   c.RiskLevel,
			TopDrivers:  []string{"recent events", "negative tone", "event severity"},
			TopTerms:    c.TopEventTypes,
			Components: []events.Component{
				{Key: "recent_events", Name: "recent events", Score: c.EventRiskScore, Weight: 0.4, Contribution: c.EventRiskScore * 0.4},
				{Key: "negative_tone", Name: "negative tone", Score: c.EventRiskScore, Weight: 0.35, Contribution: c.EventRiskScore * 0.35},
				{Key: "event_severity", Name: "event severity", Score: c.EventRiskScore, Weight: 0.25, Contribution: c.EventRiskScore * 0.25},
			},
		})
	}
	return out
}
