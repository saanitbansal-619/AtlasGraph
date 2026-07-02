package eventrisk

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

var csvCountryCols = []string{"country", "country_name", "location", "actor1_country", "actor2_country"}
var csvDateCols = []string{"date", "event_date", "published_at", "sqldate", "day"}
var csvTypeCols = []string{"event_type", "type", "category", "eventcode", "event_root_code"}
var csvSeverityCols = []string{"severity", "goldstein", "goldsteinscale", "impact"}
var csvToneCols = []string{"tone", "avg_tone", "avgtone", "mediatone"}
var csvSummaryCols = []string{"summary", "title", "description", "headline", "text"}

// LoadCSV parses a GDELT-style event CSV into normalized events.
func LoadCSV(path string) (IngestResult, error) {
	f, err := os.Open(path)
	if err != nil {
		return IngestResult{}, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()
	return ParseCSV(f)
}

// ParseCSV reads event records from a CSV reader.
func ParseCSV(r io.Reader) (IngestResult, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return IngestResult{}, fmt.Errorf("reading CSV header: %w", err)
	}
	idx := indexHeader(header)

	var out IngestResult
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, fmt.Errorf("reading CSV row: %w", err)
		}
		ev, warn, ok := parseCSVRow(idx, row)
		if warn != "" {
			out.Warnings = append(out.Warnings, warn)
		}
		if ok {
			out.Events = append(out.Events, ev)
		}
	}
	return out, nil
}

type headerIndex map[string]int

func indexHeader(header []string) headerIndex {
	idx := headerIndex{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if key != "" {
			idx[key] = i
		}
	}
	return idx
}

func (idx headerIndex) first(cols []string) (int, bool) {
	for _, c := range cols {
		if i, ok := idx[c]; ok {
			return i, true
		}
	}
	return -1, false
}

func parseCSVRow(idx headerIndex, row []string) (NormalizedEvent, string, bool) {
	country := ""
	if i, ok := idx.first(csvCountryCols); ok && i < len(row) {
		country = strings.TrimSpace(row[i])
	}
	if country == "" {
		return NormalizedEvent{}, "", false
	}

	canonical, _, known := NormalizeCountry(country)
	if !known {
		return NormalizedEvent{}, WarnUnknownCountry(country), false
	}

	date := ""
	if i, ok := idx.first(csvDateCols); ok && i < len(row) {
		date = normalizeDate(strings.TrimSpace(row[i]))
	}
	if date == "" {
		return NormalizedEvent{}, fmt.Sprintf("skipping event for %s with missing/invalid date", canonical), false
	}

	eventType := "other"
	if i, ok := idx.first(csvTypeCols); ok && i < len(row) {
		if v := strings.TrimSpace(row[i]); v != "" {
			eventType = normalizeEventType(v)
		}
	}

	severity := 0.5
	if i, ok := idx.first(csvSeverityCols); ok && i < len(row) {
		if v, ok := parseFloat(row[i]); ok {
			severity = normalizeSeverity(v)
		}
	}

	tone := 0.0
	if i, ok := idx.first(csvToneCols); ok && i < len(row) {
		if v, ok := parseFloat(row[i]); ok {
			tone = v
		}
	}

	summary := ""
	if i, ok := idx.first(csvSummaryCols); ok && i < len(row) {
		summary = strings.TrimSpace(row[i])
	}

	return NormalizedEvent{
		Country:   canonical,
		Date:      date,
		EventType: eventType,
		Severity:  severity,
		Tone:      tone,
		Source:    SourceName,
		Summary:   summary,
	}, "", true
}

func parseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

func normalizeDate(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	layouts := []string{
		"2006-01-02",
		"2006/01/02",
		"20060102",
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"01/02/2006",
		"1/2/2006",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	if len(raw) >= 8 {
		if t, err := time.Parse("20060102", raw[:8]); err == nil {
			return t.UTC().Format("2006-01-02")
		}
	}
	return ""
}
