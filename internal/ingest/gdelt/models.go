// Package gdelt ingests real-world event/news risk signals from the GDELT
// DOC 2.0 API (https://api.gdeltproject.org/api/v2/doc/doc) and normalises them
// into a flat, typed record set that can be saved to disk and later scored for
// country-level event risk.
//
// This is AtlasGraph's third external signal, alongside World Bank macro
// indicators and Comtrade-style trade flows. It deliberately depends only on
// the Go standard library and keeps fetching, normalisation and persistence in
// small, separately testable pieces. The HTTP client is isolated behind the
// Fetcher interface so tests can drive it from saved JSON fixtures (via
// httptest) without ever calling the live GDELT service.
package gdelt

import "time"

// SourceName identifies the provenance recorded on every normalised record.
const SourceName = "GDELT DOC 2.0 API"

// OutputFileName is the canonical file the ingest command writes within its
// output directory and the events command reads back.
const OutputFileName = "gdelt_events.json"

// DefaultBaseURL is the GDELT DOC 2.0 article-search endpoint.
const DefaultBaseURL = "https://api.gdeltproject.org/api/v2/doc/doc"

// Defaults applied when the caller does not specify them.
const (
	DefaultDays       = 7  // look-back window in days
	DefaultMaxRecords = 75 // articles requested per country (GDELT caps at 250)
)

// RiskTerms are the geopolitical / supply-chain keywords AtlasGraph searches
// for alongside each country name. A document is considered risk-relevant when
// its title matches one or more of these terms.
var RiskTerms = []string{
	"sanctions",
	"conflict",
	"military",
	"protest",
	"strike",
	"supply chain",
	"export controls",
	"trade restrictions",
	"shipping disruption",
	"semiconductor",
	"energy",
	"commodity",
}

// GDELTEventRecord is a single normalised news/event document for a queried
// country. Fields the GDELT response does not provide (e.g. tone, themes in
// ArtList mode) are left at their zero value so the schema stays stable.
type GDELTEventRecord struct {
	CountryCode      string    `json:"country_code"`
	CountryName      string    `json:"country_name"`
	Title            string    `json:"title"`
	URL              string    `json:"url"`
	SourceCountry    string    `json:"source_country"`
	Domain           string    `json:"domain"`
	PublishedAt      time.Time `json:"published_at"`
	Tone             float64   `json:"tone"`
	Language         string    `json:"language"`
	Themes           []string  `json:"themes"`
	RiskTermsMatched []string  `json:"risk_terms_matched"`
	Source           string    `json:"source"`
	FetchedAt        time.Time `json:"fetched_at"`
}

// EventFile is the on-disk shape written by the ingest command: a little
// provenance metadata plus the flat record set.
type EventFile struct {
	Source    string             `json:"source"`
	FetchedAt time.Time          `json:"fetched_at"`
	Days      int                `json:"days"`
	Countries []string           `json:"countries"`
	Records   []GDELTEventRecord `json:"records"`
}

// --- GDELT API wire types (decoded, never persisted) -----------------------

// docResponse is the GDELT DOC 2.0 ArtList JSON envelope.
type docResponse struct {
	Articles []docArticle `json:"articles"`
}

// docArticle is one article in an ArtList response. Tone/themes are optional:
// the ArtList mode omits them, but other GDELT exports include them, so we keep
// them so richer fixtures (and future modes) carry through.
type docArticle struct {
	URL           string   `json:"url"`
	Title         string   `json:"title"`
	SeenDate      string   `json:"seendate"`
	Domain        string   `json:"domain"`
	Language      string   `json:"language"`
	SourceCountry string   `json:"sourcecountry"`
	Tone          *float64 `json:"tone,omitempty"`
	Themes        []string `json:"themes,omitempty"`
}
