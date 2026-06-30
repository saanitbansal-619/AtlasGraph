package gdelt

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// fixtureEvent is the on-disk shape of a single event in a local GDELT fixture
// file. It mirrors GDELTEventRecord's public fields so a fixture is easy to
// author by hand, but it is decoded separately so loading can fill defaults and
// re-stamp provenance.
type fixtureEvent struct {
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
}

// fixtureFile accepts either a bare JSON array of events or an object wrapping
// them under "records"/"events", so fixtures can be written in whichever style
// is most convenient.
type fixtureFile struct {
	Records []fixtureEvent `json:"records"`
	Events  []fixtureEvent `json:"events"`
}

// LoadFixture reads a local fixture file and normalises its events into the same
// GDELTEventRecord schema produced by a live fetch. It does not touch the
// network. Records are stamped with FixtureSourceName (unless the fixture sets
// its own source) and a fresh fetched_at, and risk_terms_matched is derived
// from the title when the fixture does not supply it.
func LoadFixture(path string) ([]GDELTEventRecord, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("fixture path is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading fixture %q: %w", path, err)
	}

	events, err := decodeFixture(b)
	if err != nil {
		return nil, fmt.Errorf("parsing fixture %q: %w", path, err)
	}

	fetchedAt := time.Now().UTC()
	out := make([]GDELTEventRecord, 0, len(events))
	for _, e := range events {
		code := strings.ToUpper(strings.TrimSpace(e.CountryCode))
		name := strings.TrimSpace(e.CountryName)
		if name == "" {
			name = resolveCountryName(code)
		}

		themes := e.Themes
		if themes == nil {
			themes = []string{}
		}

		terms := e.RiskTermsMatched
		if len(terms) == 0 {
			terms = MatchRiskTerms(e.Title)
		}
		if terms == nil {
			terms = []string{}
		}

		source := strings.TrimSpace(e.Source)
		if source == "" {
			source = FixtureSourceName
		}

		out = append(out, GDELTEventRecord{
			CountryCode:      code,
			CountryName:      name,
			Title:            strings.TrimSpace(e.Title),
			URL:              strings.TrimSpace(e.URL),
			SourceCountry:    strings.TrimSpace(e.SourceCountry),
			Domain:           strings.TrimSpace(e.Domain),
			PublishedAt:      e.PublishedAt.UTC(),
			Tone:             e.Tone,
			Language:         strings.TrimSpace(e.Language),
			Themes:           themes,
			RiskTermsMatched: terms,
			Source:           source,
			FetchedAt:        fetchedAt,
		})
	}
	return out, nil
}

// decodeFixture parses fixture bytes as either a bare array or an object with a
// records/events field.
func decodeFixture(b []byte) ([]fixtureEvent, error) {
	trimmed := strings.TrimSpace(string(b))
	if trimmed == "" {
		return nil, fmt.Errorf("fixture is empty")
	}
	if strings.HasPrefix(trimmed, "[") {
		var arr []fixtureEvent
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return nil, err
		}
		return arr, nil
	}
	var wrapper fixtureFile
	if err := json.Unmarshal([]byte(trimmed), &wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.Records) > 0 {
		return wrapper.Records, nil
	}
	return wrapper.Events, nil
}
