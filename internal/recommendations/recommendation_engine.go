package recommendations

import (
	"fmt"
	"sort"
	"strings"
)

// GenerateExecutiveActionPlan evaluates deterministic rules and assembles the action plan.
func GenerateExecutiveActionPlan(in Input) ExecutiveActionPlan {
	candidates := evaluateRules(in)
	recs := make([]Recommendation, 0, len(candidates))
	for _, c := range candidates {
		recs = append(recs, Recommendation{
			ID:                       c.id,
			Title:                    c.title,
			Description:              c.description,
			Priority:                 c.priority,
			Category:                 c.category,
			Reason:                   c.reason,
			ExpectedImpact:           c.impact,
			ImplementationDifficulty: c.difficulty,
			Confidence:               c.confidence,
			SupportingMetrics:        c.metrics,
		})
	}

	sort.SliceStable(recs, func(i, j int) bool {
		ri, rj := priorityRank(recs[i].Priority), priorityRank(recs[j].Priority)
		if ri != rj {
			return ri > rj
		}
		return recs[i].Title < recs[j].Title
	})

	dist := countPriorities(recs)
	summary := buildExecutiveSummary(in, recs)
	impact := buildEstimatedBusinessImpact(in, recs)
	evidence := buildSupportingEvidence(in)

	return ExecutiveActionPlan{
		Summary:                 summary,
		Recommendations:         recs,
		PriorityDistribution:    dist,
		EstimatedBusinessImpact: impact,
		SupportingEvidence:      evidence,
	}
}

func countPriorities(recs []Recommendation) PriorityDistribution {
	var dist PriorityDistribution
	for _, r := range recs {
		switch r.Priority {
		case PriorityCritical:
			dist.Critical++
		case PriorityHigh:
			dist.High++
		case PriorityMedium:
			dist.Medium++
		default:
			dist.Low++
		}
	}
	return dist
}

func buildExecutiveSummary(in Input, recs []Recommendation) string {
	if len(recs) == 0 {
		return fmt.Sprintf(
			"The simulated %.0f%% %s disruption on %s did not trigger high-priority mitigation rules at current concentration and risk thresholds.",
			in.DropPercent, shockLabel(in.ShockType), commodityTitle(in.Commodity),
		)
	}

	parts := []string{
		fmt.Sprintf(
			"The simulated disruption indicates elevated dependency on %s for %s imports",
			in.Source, commodityTitle(in.Commodity),
		),
	}

	if share := shareValue(in); share > 0 {
		parts[0] += fmt.Sprintf(" (%.0f%% supplier share)", sharePercent(in))
	}
	if hhi := hhiValue(in); hhi > 0 {
		parts[0] += fmt.Sprintf(" with HHI %.3f", hhi)
	}
	parts[0] += "."

	top := recs[0]
	second := ""
	if len(recs) > 1 {
		second = recs[1].Title
	}
	switch {
	case second != "":
		parts = append(parts, fmt.Sprintf(
			"%s and %s are expected to significantly improve resilience.",
			top.Title, second,
		))
	default:
		parts = append(parts, fmt.Sprintf(
			"%s is expected to significantly improve resilience.",
			top.Title,
		))
	}
	return strings.Join(parts, " ")
}

func buildEstimatedBusinessImpact(in Input, recs []Recommendation) string {
	if len(recs) == 0 {
		return "Limited immediate mitigation need; continue monitoring scenario signals."
	}
	critical := countPriorities(recs).Critical
	high := countPriorities(recs).High
	switch {
	case critical > 0:
		return fmt.Sprintf(
			"%d critical and %d high-priority actions could reduce estimated fragility exposure by 20–40%% if implemented within 30 days.",
			critical, high,
		)
	case high > 0:
		return fmt.Sprintf(
			"%d high-priority actions could reduce supply disruption impact by 15–30%% over the next procurement cycle.",
			high,
		)
	default:
		return "Targeted monitoring and buffer actions could improve response readiness with moderate operational effort."
	}
}

func buildSupportingEvidence(in Input) []string {
	evidence := []string{
		fmt.Sprintf("Shock: %.0f%% %s on %s at depth %d", in.DropPercent, shockLabel(in.ShockType), in.Source, in.Depth),
		fmt.Sprintf("Event-risk score %.0f; macro exposure %.0f", in.EventRisk, in.MacroExposure),
	}
	if in.HHI != nil {
		evidence = append(evidence, fmt.Sprintf("Trade concentration HHI %.3f (%s)", *in.HHI, in.TradeConcentration))
	}
	if in.SupplierShare != nil {
		evidence = append(evidence, fmt.Sprintf("Top supplier share %.0f%%", sharePercent(in)))
	}
	if in.FragilityScore > 0 {
		evidence = append(evidence, fmt.Sprintf("Modeled fragility impact %.1f", in.FragilityScore))
	}
	if in.AffectedPaths > 0 {
		evidence = append(evidence, fmt.Sprintf("%d affected dependency paths in propagation graph", in.AffectedPaths))
	}
	return evidence
}
