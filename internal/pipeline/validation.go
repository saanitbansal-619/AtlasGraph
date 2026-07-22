package pipeline

import (
	"encoding/json"
	"strings"

	analyticsdb "github.com/atlasgraph/atlas/internal/db"
)

// ValidationCheck is one pipeline validation outcome exposed to the API.
type ValidationCheck struct {
	CheckName   string  `json:"check_name"`
	Status      string  `json:"status"`
	MetricValue float64 `json:"metric_value"`
	Source      string  `json:"source"`
	Details     any     `json:"details,omitempty"`
}

func normalizeDBValidationChecks(raw []analyticsdb.PipelineValidationCheck) []ValidationCheck {
	out := make([]ValidationCheck, 0, len(raw))
	for _, item := range raw {
		out = append(out, normalizeValidationCheck(item.CheckName, item.RawStatus, item.MetricValue, item.Source, item.Details))
	}
	return out
}

func normalizeValidationCheck(name, rawStatus string, metric float64, source string, details json.RawMessage) ValidationCheck {
	status := mapRawValidationStatus(name, rawStatus, metric)
	var parsed any
	if len(details) > 0 {
		_ = json.Unmarshal(details, &parsed)
	}
	if parsed == nil {
		parsed = map[string]any{"count": metric}
	}
	return ValidationCheck{
		CheckName: name, Status: status, MetricValue: metric,
		Source: source, Details: parsed,
	}
}

func mapRawValidationStatus(name, rawStatus string, metric float64) string {
	normalized := strings.ToLower(strings.TrimSpace(rawStatus))
	switch normalized {
	case "passed", "ok", "success":
		return "passed"
	case "failed", "error":
		return "failed"
	case "warning", "warn":
		if isCoreLoadCheck(name) && metric == 0 {
			return "failed"
		}
		if isOptionalCheck(name) {
			return "warning"
		}
		return "warning"
	default:
		if isCoreLoadCheck(name) && metric == 0 {
			return "failed"
		}
		if isOptionalCheck(name) && metric > 0 {
			return "warning"
		}
		if metric == 0 && isCoreLoadCheck(name) {
			return "failed"
		}
		return "passed"
	}
}

func isOptionalCheck(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "missing") ||
		strings.Contains(lower, "unmapped") ||
		strings.Contains(lower, "aggregate") ||
		strings.Contains(lower, "skipped") ||
		strings.Contains(lower, "optional")
}

func isCoreLoadCheck(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "total trade rows") ||
		strings.Contains(lower, "event risk rows") ||
		strings.Contains(lower, "dependency graph") ||
		strings.Contains(lower, "graph edges")
}

func summarizeValidationChecks(checks []ValidationCheck) (passed, warnings, failed, invalidRows int) {
	for _, check := range checks {
		switch check.Status {
		case "passed":
			passed++
		case "warning":
			warnings++
			if isOptionalCheck(check.CheckName) {
				invalidRows += int(check.MetricValue)
			}
		case "failed":
			failed++
		}
	}
	return passed, warnings, failed, invalidRows
}

func makeValidationCheck(name string, value int, zeroIsGood bool, source string) ValidationCheck {
	rawStatus := "ok"
	if (zeroIsGood && value > 0) || (!zeroIsGood && value == 0) {
		rawStatus = "warning"
	}
	details := map[string]any{"count": value}
	if isOptionalCheck(name) && value > 0 {
		details["note"] = "Records skipped during normalization or optional public-source coverage gaps."
	}
	if isCoreLoadCheck(name) && value == 0 {
		details["note"] = "Core dataset produced no loadable rows."
	}
	return normalizeValidationCheck(name, rawStatus, float64(value), source, mustJSON(details))
}

func mustJSON(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}
