package worldbank

import "strings"

// SummaryLine is the latest available observation for one indicator.
type SummaryLine struct {
	IndicatorCode string
	IndicatorName string
	Year          int      // year of the latest non-null value (0 if none)
	Value         *float64 // nil when the country has no value for this indicator
}

// Summary is a per-country snapshot: the latest value of each indicator in the
// default panel, plus the most recent year for which any value exists.
type Summary struct {
	CountryCode string
	CountryName string
	LatestYear  int
	Lines       []SummaryLine
	HasData     bool
}

// BuildSummary distils an IndicatorFile into a single country's latest values.
// Indicators are emitted in DefaultIndicators order so output is stable, and
// each line carries the most recent year that actually has a value.
func BuildSummary(file IndicatorFile, iso3 string) Summary {
	iso3 = strings.ToUpper(strings.TrimSpace(iso3))
	sum := Summary{CountryCode: iso3}

	// latest[code] tracks the newest non-null observation seen per indicator.
	type best struct {
		year  int
		value *float64
	}
	latest := map[string]best{}

	for _, r := range file.Records {
		if !strings.EqualFold(r.CountryCode, iso3) {
			continue
		}
		sum.HasData = true
		if r.CountryName != "" {
			sum.CountryName = r.CountryName
		}
		if r.Value == nil {
			continue
		}
		cur, ok := latest[r.IndicatorCode]
		if !ok || r.Year > cur.year {
			latest[r.IndicatorCode] = best{year: r.Year, value: r.Value}
			if r.Year > sum.LatestYear {
				sum.LatestYear = r.Year
			}
		}
	}

	if !sum.HasData {
		return sum
	}

	for _, ind := range DefaultIndicators {
		line := SummaryLine{IndicatorCode: ind.Code, IndicatorName: ind.Name}
		if b, ok := latest[ind.Code]; ok {
			line.Year = b.year
			line.Value = b.value
		}
		sum.Lines = append(sum.Lines, line)
	}
	return sum
}
