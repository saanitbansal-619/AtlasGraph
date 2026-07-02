package eventrisk

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type jsonEvent struct {
	Country   string  `json:"country"`
	Date      string  `json:"date"`
	EventType string  `json:"event_type"`
	Severity  float64 `json:"severity"`
	Tone      float64 `json:"tone"`
	Source    string  `json:"source"`
	Summary   string  `json:"summary"`
}

type jsonEnvelope struct {
	Records []jsonEvent `json:"records"`
	Events  []jsonEvent `json:"events"`
}

// LoadJSON parses a GDELT-style event JSON file (bare array or wrapped object).
func LoadJSON(path string) (IngestResult, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return IngestResult{}, fmt.Errorf("reading %q: %w", path, err)
	}
	return ParseJSON(b)
}

// ParseJSON decodes event records from JSON bytes.
func ParseJSON(b []byte) (IngestResult, error) {
	var rows []jsonEvent
	if err := json.Unmarshal(b, &rows); err == nil && len(rows) > 0 {
		return normalizeJSONRows(rows)
	}

	var env jsonEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return IngestResult{}, fmt.Errorf("decoding JSON events: %w", err)
	}
	rows = env.Records
	if len(rows) == 0 {
		rows = env.Events
	}
	return normalizeJSONRows(rows)
}

func normalizeJSONRows(rows []jsonEvent) (IngestResult, error) {
	var out IngestResult
	for _, row := range rows {
		ev, warn, ok := normalizeJSONEvent(row)
		if warn != "" {
			out.Warnings = append(out.Warnings, warn)
		}
		if ok {
			out.Events = append(out.Events, ev)
		}
	}
	return out, nil
}

func normalizeJSONEvent(row jsonEvent) (NormalizedEvent, string, bool) {
	country := strings.TrimSpace(row.Country)
	if country == "" {
		return NormalizedEvent{}, "", false
	}
	canonical, _, known := NormalizeCountry(country)
	if !known {
		return NormalizedEvent{}, WarnUnknownCountry(country), false
	}
	date := normalizeDate(strings.TrimSpace(row.Date))
	if date == "" {
		return NormalizedEvent{}, fmt.Sprintf("skipping event for %s with missing/invalid date", canonical), false
	}
	eventType := normalizeEventType(row.EventType)
	severity := normalizeSeverity(row.Severity)
	source := strings.TrimSpace(row.Source)
	if source == "" {
		source = SourceName
	}
	return NormalizedEvent{
		Country:   canonical,
		Date:      date,
		EventType: eventType,
		Severity:  severity,
		Tone:      row.Tone,
		Source:    source,
		Summary:   strings.TrimSpace(row.Summary),
	}, "", true
}
