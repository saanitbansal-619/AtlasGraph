// Package worldbank ingests macroeconomic indicators from the World Bank
// Indicators API (https://api.worldbank.org/v2) and normalises them into a
// flat, typed record set that can be saved to disk and later fed into the
// AtlasGraph scoring engine.
//
// This is the first real external data source for AtlasGraph. It deliberately
// depends only on the Go standard library and keeps fetching, normalisation and
// persistence in small, separately testable pieces.
package worldbank

import "time"

// SourceName identifies the provenance recorded on every normalised record.
const SourceName = "World Bank Indicators API v2"

// OutputFileName is the canonical file the ingest command writes within its
// output directory and the indicators command reads back.
const OutputFileName = "worldbank_indicators.json"

// DefaultBaseURL is the World Bank Indicators API v2 root.
const DefaultBaseURL = "https://api.worldbank.org/v2"

// Default year bounds when the caller does not specify them.
const (
	DefaultStartYear = 2018
	DefaultEndYear   = 2023
)

// Indicator is a World Bank series we know how to fetch, paired with a clean
// display name (the API echoes its own name, but we keep our own for stable,
// presentation-friendly output).
type Indicator struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// DefaultIndicators is the macro panel AtlasGraph ingests for each country.
// These were chosen because they speak directly to trade exposure, industrial
// concentration and price stability — the raw material for fragility scoring.
var DefaultIndicators = []Indicator{
	{Code: "NY.GDP.MKTP.CD", Name: "GDP (current US$)"},
	{Code: "NE.IMP.GNFS.ZS", Name: "Imports of goods and services (% of GDP)"},
	{Code: "NE.EXP.GNFS.ZS", Name: "Exports of goods and services (% of GDP)"},
	{Code: "NV.IND.MANF.ZS", Name: "Manufacturing value added (% of GDP)"},
	{Code: "FP.CPI.TOTL.ZG", Name: "Inflation, consumer prices (annual %)"},
	{Code: "TX.VAL.TECH.CD", Name: "High-technology exports (current US$)"},
}

// IndicatorName returns the display name for a code, falling back to the code
// itself if it is not in the default panel.
func IndicatorName(code string) string {
	for _, ind := range DefaultIndicators {
		if ind.Code == code {
			return ind.Name
		}
	}
	return code
}

// CountryIndicatorRecord is a single normalised observation: one indicator, for
// one country, in one year. Value is a pointer so a genuinely missing
// observation (the World Bank returns JSON null) is preserved rather than being
// silently coerced to zero.
type CountryIndicatorRecord struct {
	CountryCode   string    `json:"country_code"`
	CountryName   string    `json:"country_name"`
	IndicatorCode string    `json:"indicator_code"`
	IndicatorName string    `json:"indicator_name"`
	Year          int       `json:"year"`
	Value         *float64  `json:"value"`
	Source        string    `json:"source"`
	FetchedAt     time.Time `json:"fetched_at"`
}

// IndicatorFile is the on-disk shape written by the ingest command: a little
// provenance metadata plus the flat record set.
type IndicatorFile struct {
	Source    string                   `json:"source"`
	FetchedAt time.Time                `json:"fetched_at"`
	StartYear int                      `json:"start_year"`
	EndYear   int                      `json:"end_year"`
	Countries []string                 `json:"countries"`
	Records   []CountryIndicatorRecord `json:"records"`
}

// --- World Bank API wire types (decoded, never persisted) ------------------

// apiMeta is the first element of every World Bank response array. On error
// responses (e.g. an invalid country code) it instead carries a Message.
// per_page is intentionally omitted: the World Bank API returns it as a quoted
// string in some responses and a number in others, and we never use it.
type apiMeta struct {
	Page    int          `json:"page"`
	Pages   int          `json:"pages"`
	Total   int          `json:"total"`
	Message []apiMessage `json:"message"`
}

type apiMessage struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// apiPoint is one observation in the second element of a World Bank response.
type apiPoint struct {
	Indicator   apiNamed `json:"indicator"`
	Country     apiNamed `json:"country"`
	CountryISO3 string   `json:"countryiso3code"`
	Date        string   `json:"date"`
	Value       *float64 `json:"value"`
}

type apiNamed struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}
