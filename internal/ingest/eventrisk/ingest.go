package eventrisk

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// IngestFromFile loads a local GDELT-style CSV or JSON file and scores country risk.
func IngestFromFile(path, source string) (RiskFile, []string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	var res IngestResult
	var err error
	switch ext {
	case ".csv":
		res, err = LoadCSV(path)
	case ".json":
		res, err = LoadJSON(path)
	default:
		return RiskFile{}, nil, fmt.Errorf("unsupported file type %q (want .csv or .json)", ext)
	}
	if err != nil {
		return RiskFile{}, res.Warnings, err
	}
	if len(res.Events) == 0 {
		return RiskFile{}, res.Warnings, fmt.Errorf("no valid events found in %s", path)
	}

	sourceName := strings.TrimSpace(source)
	if sourceName == "" {
		sourceName = SourceName
	}
	// Normalize common CLI labels to the canonical public source name.
	if strings.EqualFold(sourceName, "gdelt") {
		sourceName = SourceName
	}

	now := time.Now().UTC()
	countries := ScoreEvents(res.Events, now)
	from, to := dateRange(res.Events)
	rowsProcessed := res.RowsProcessed
	if rowsProcessed == 0 {
		rowsProcessed = len(res.Events)
	}

	file := RiskFile{
		Source:             sourceName,
		IngestedAt:         now,
		SourceFile:         path,
		DateFrom:           from,
		DateTo:             to,
		LatestEventDate:    to,
		EventCount:         len(res.Events),
		RowsProcessed:      rowsProcessed,
		CountriesCovered:   len(countries),
		EventTypeBreakdown: eventTypeBreakdown(res.Events),
		ScoringNote:        DefaultScoringNote,
		Countries:          countries,
		Events:             res.Events,
	}
	return file, res.Warnings, nil
}

func dateRange(events []NormalizedEvent) (from, to string) {
	if len(events) == 0 {
		return "", ""
	}
	from = events[0].Date
	to = events[0].Date
	for _, e := range events[1:] {
		if e.Date < from {
			from = e.Date
		}
		if e.Date > to {
			to = e.Date
		}
	}
	return from, to
}

func eventTypeBreakdown(events []NormalizedEvent) map[string]int {
	out := map[string]int{}
	for _, e := range events {
		t := e.EventType
		if t == "" {
			t = "other"
		}
		out[t]++
	}
	return out
}

// SortedEventTypeKeys returns event types sorted by count descending for reports.
func SortedEventTypeKeys(breakdown map[string]int) []string {
	keys := make([]string, 0, len(breakdown))
	for k := range breakdown {
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if breakdown[keys[i]] != breakdown[keys[j]] {
			return breakdown[keys[i]] > breakdown[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}
