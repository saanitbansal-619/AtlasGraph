package eventrisk

import (
	"time"

	"github.com/atlasgraph/atlas/internal/scoring/events"
)

// SourceName is the provenance label for ingested public GDELT-style event files.
const SourceName = "GDELT"

// OutputFileName is the canonical scored output written by the ingest command.
const OutputFileName = "event_risk.json"

// DefaultScoringNote documents how event-risk scores are derived.
const DefaultScoringNote = "Country event-risk scores blend recent event volume, negative tone, and event severity from GDELT-style public event records. Scores are model-derived exposure estimates, not factual predictions."

// NormalizedEvent is one public event/news record after ingest normalization.
type NormalizedEvent struct {
	Country   string  `json:"country"`
	Date      string  `json:"date"`
	EventType string  `json:"event_type"`
	Severity  float64 `json:"severity"`
	Tone      float64 `json:"tone"`
	Source    string  `json:"source"`
	Summary   string  `json:"summary,omitempty"`
}

// CountryRisk is the scored event-risk summary for one country.
type CountryRisk struct {
	Country          string             `json:"country"`
	CountryCode      string             `json:"country_code,omitempty"`
	EventRiskScore   float64            `json:"event_risk_score"`
	RiskLevel        string             `json:"risk_level"`
	EventCount       int                `json:"event_count"`
	RecentEventCount int                `json:"recent_event_count"`
	AverageTone      float64            `json:"average_tone"`
	TopEventTypes    []string           `json:"top_event_types"`
	Source           string             `json:"source"`
	Components       []events.Component `json:"components,omitempty"`
}

// RiskFile is the processed event-risk panel written to disk.
type RiskFile struct {
	Source             string            `json:"source"`
	IngestedAt         time.Time         `json:"ingested_at"`
	SourceFile         string            `json:"source_file,omitempty"`
	DateFrom           string            `json:"date_from,omitempty"`
	DateTo             string            `json:"date_to,omitempty"`
	LatestEventDate    string            `json:"latest_event_date,omitempty"`
	EventCount         int               `json:"event_count"`
	RowsProcessed      int               `json:"rows_processed,omitempty"`
	CountriesCovered   int               `json:"countries_covered,omitempty"`
	EventTypeBreakdown map[string]int    `json:"event_type_breakdown,omitempty"`
	ScoringNote        string            `json:"scoring_note,omitempty"`
	Countries          []CountryRisk     `json:"countries"`
	Events             []NormalizedEvent `json:"events,omitempty"`
}

// IngestResult captures parse warnings and normalized rows from one file.
type IngestResult struct {
	Events        []NormalizedEvent
	Warnings      []string
	RowsProcessed int
}
