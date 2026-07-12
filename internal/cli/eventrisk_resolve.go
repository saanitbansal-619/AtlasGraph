package cli

import (
	"strings"

	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
	"github.com/atlasgraph/atlas/internal/scoring/events"
)

// resolvedEventRisk is the unified event-risk payload used by the API and CLI.
type resolvedEventRisk struct {
	Source         string
	RealEventData  bool
	DateFrom       string
	DateTo         string
	Scores         []events.CountryScore
	Processed      *eventrisk.RiskFile
	RecentEvents   []eventrisk.NormalizedEvent
	CountryFilter  string
}

func resolveEventRisk(processedDir, legacyDir string) (resolvedEventRisk, error) {
	if processedDir != "" {
		if file, ok := eventrisk.TryLoadProcessed(processedDir); ok {
			real := eventrisk.IsRealProcessedEventRisk(file)
			source := file.Source
			if source == "" {
				source = eventrisk.SourceName
			}
			if real {
				source = eventrisk.SourceName
			}
			return resolvedEventRisk{
				Source:        source,
				RealEventData: real,
				DateFrom:      file.DateFrom,
				DateTo:        file.DateTo,
				Scores:        eventrisk.ToLegacyCountryScores(file),
				Processed:     &file,
			}, nil
		}
	}

	file, err := gdelt.Load(legacyDir)
	if err != nil {
		return resolvedEventRisk{}, err
	}
	source := demoEventSourceLabel(file.Source)
	return resolvedEventRisk{
		Source:        source,
		RealEventData: false,
		Scores:        events.ScoreCountries(file, events.DefaultWeights()),
	}, nil
}

func demoEventSourceLabel(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if strings.Contains(lower, "fixture") || strings.Contains(lower, "demo") || strings.Contains(lower, "synthetic") {
		return "demo"
	}
	return "sample"
}

func (r resolvedEventRisk) withCountryFilter(country string) resolvedEventRisk {
	country = strings.TrimSpace(country)
	if country == "" || r.Processed == nil {
		return r
	}
	if row, ok := eventrisk.CountryRiskFor(*r.Processed, country); ok {
		r.Scores = eventrisk.ToLegacyCountryScores(eventrisk.RiskFile{Countries: []eventrisk.CountryRisk{row}})
	}
	r.RecentEvents = eventrisk.RecentEventsForCountry(*r.Processed, country, 10)
	r.CountryFilter = country
	return r
}
