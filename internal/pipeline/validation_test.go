package pipeline

import (
	"testing"
)

func TestMapRawValidationStatusOptionalMissingIsWarning(t *testing.T) {
	status := mapRawValidationStatus("missing macro scores", "warning", 2)
	if status != "warning" {
		t.Fatalf("status = %q, want warning", status)
	}
}

func TestMapRawValidationStatusCoreZeroRowsIsFailed(t *testing.T) {
	status := mapRawValidationStatus("total trade rows loaded", "warning", 0)
	if status != "failed" {
		t.Fatalf("status = %q, want failed", status)
	}
}

func TestSummarizeValidationChecksCountsWarningsSeparately(t *testing.T) {
	checks := []ValidationCheck{
		{CheckName: "total trade rows loaded", Status: "passed", MetricValue: 10},
		{CheckName: "missing macro scores", Status: "warning", MetricValue: 2},
		{CheckName: "missing exporter codes", Status: "warning", MetricValue: 1},
	}
	passed, warnings, failed, invalidRows := summarizeValidationChecks(checks)
	if passed != 1 || warnings != 2 || failed != 0 {
		t.Fatalf("counts = %d/%d/%d, want 1/2/0", passed, warnings, failed)
	}
	if invalidRows != 3 {
		t.Fatalf("invalidRows = %d, want 3", invalidRows)
	}
}

func TestDeriveStatusCompletedWhenOnlyPassedChecks(t *testing.T) {
	summary := PipelineRunSummary{
		TotalRowsProcessed: 100,
		SourcesProcessed: []SourceRow{
			{Name: "UN Comtrade trade rows", RowsProcessed: 50},
			{Name: "Dependency graph edges", RowsProcessed: 50},
		},
		ValidationChecksPassed: 5,
	}
	if got := deriveStatus(summary, false); got != "completed" {
		t.Fatalf("status = %q, want completed", got)
	}
}

func TestDeriveStatusWarningWhenOptionalChecksWarn(t *testing.T) {
	summary := PipelineRunSummary{
		TotalRowsProcessed: 100,
		SourcesProcessed: []SourceRow{
			{Name: "UN Comtrade trade rows", RowsProcessed: 50},
			{Name: "Dependency graph edges", RowsProcessed: 50},
		},
		ValidationChecksWarnings: 2,
		InvalidRows:              3,
	}
	if got := deriveStatus(summary, false); got != "warning" {
		t.Fatalf("status = %q, want warning", got)
	}
}

func TestDeriveStatusFailedWhenDBEnabledAndNoRowsLoaded(t *testing.T) {
	summary := PipelineRunSummary{
		TotalRowsProcessed: 100,
		SourcesProcessed: []SourceRow{
			{Name: "UN Comtrade trade rows", RowsProcessed: 50},
			{Name: "Dependency graph edges", RowsProcessed: 50},
		},
		ValidationChecksPassed: 5,
	}
	if got := deriveStatus(summary, true); got != "failed" {
		t.Fatalf("status = %q, want failed", got)
	}
}
