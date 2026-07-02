package eventrisk

import "time"

// SourceName is the provenance label for ingested public GDELT-style event files.
const SourceName = "GDELT"

// OutputFileName is the canonical scored output written by the ingest command.
const OutputFileName = "event_risk.json"

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
	Country          string   `json:"country"`
	CountryCode      string   `json:"country_code,omitempty"`
	EventRiskScore   float64  `json:"event_risk_score"`
	RiskLevel        string   `json:"risk_level"`
	EventCount       int      `json:"event_count"`
	RecentEventCount int      `json:"recent_event_count"`
	AverageTone      float64  `json:"average_tone"`
	TopEventTypes    []string `json:"top_event_types"`
	Source           string   `json:"source"`
}

// RiskFile is the processed event-risk panel written to disk.
type RiskFile struct {
	Source     string            `json:"source"`
	IngestedAt time.Time         `json:"ingested_at"`
	SourceFile string            `json:"source_file,omitempty"`
	DateFrom   string            `json:"date_from,omitempty"`
	DateTo     string            `json:"date_to,omitempty"`
	EventCount int               `json:"event_count"`
	Countries  []CountryRisk     `json:"countries"`
	Events     []NormalizedEvent `json:"events,omitempty"`
}

// IngestResult captures parse warnings and normalized rows from one file.
type IngestResult struct {
	Events   []NormalizedEvent
	Warnings []string
}
