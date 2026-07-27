package recommendations

import (
	"fmt"
	"strings"
)

type ruleCandidate struct {
	id          string
	title       string
	description string
	category    Category
	reason      string
	impact      string
	priority    Priority
	difficulty  Difficulty
	confidence  Confidence
	metrics     SupportingMetrics
}

func priorityRank(p Priority) int {
	switch p {
	case PriorityCritical:
		return 4
	case PriorityHigh:
		return 3
	case PriorityMedium:
		return 2
	default:
		return 1
	}
}

func boostPriority(p Priority) Priority {
	switch p {
	case PriorityLow:
		return PriorityMedium
	case PriorityMedium:
		return PriorityHigh
	case PriorityHigh:
		return PriorityCritical
	default:
		return PriorityCritical
	}
}

func applyDropBoost(p Priority, dropPercent float64) Priority {
	if dropPercent > 40 {
		return boostPriority(p)
	}
	return p
}

func hhiValue(in Input) float64 {
	if in.HHI != nil {
		return *in.HHI
	}
	return 0
}

func shareValue(in Input) float64 {
	if in.SupplierShare != nil {
		return *in.SupplierShare
	}
	return 0
}

func sharePercent(in Input) float64 {
	return shareValue(in) * 100
}

func shockLabel(shockType string) string {
	return strings.ReplaceAll(shockType, "_", " ")
}

func commodityTitle(commodity string) string {
	if commodity == "" {
		return "supply"
	}
	return commodity
}

func baseMetrics(in Input) SupportingMetrics {
	m := SupportingMetrics{
		"drop_percent":    in.DropPercent,
		"event_risk":      in.EventRisk,
		"macro_exposure":  in.MacroExposure,
		"fragility_score": in.FragilityScore,
		"depth":           float64(in.Depth),
	}
	if in.HHI != nil {
		m["hhi"] = *in.HHI
	}
	if in.SupplierShare != nil {
		m["supplier_share"] = sharePercent(in)
	}
	return m
}

func mergeMetrics(base SupportingMetrics, extra SupportingMetrics) SupportingMetrics {
	out := SupportingMetrics{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func evaluateRules(in Input) []ruleCandidate {
	var out []ruleCandidate
	share := shareValue(in)
	hhi := hhiValue(in)

	// IF supplier_share > 60% → Supplier Diversification (High, High confidence)
	if share > 0.60 {
		out = append(out, ruleCandidate{
			id:          "supplier-diversification",
			title:       fmt.Sprintf("Diversify %s Suppliers", titleCase(commodityTitle(in.Commodity))),
			description: "Reduce single-source dependency by qualifying additional suppliers across regions.",
			category:    CategorySupplierDiversification,
			reason: fmt.Sprintf(
				"%s accounts for %.0f%% of analyzed %s imports.",
				in.Source, sharePercent(in), commodityTitle(in.Commodity),
			),
			impact:     "Reduce supplier concentration and improve resilience against follow-on disruptions.",
			priority:   PriorityHigh,
			difficulty: DifficultyMedium,
			confidence: ConfidenceHigh,
			metrics:    mergeMetrics(baseMetrics(in), SupportingMetrics{"supplier_share": sharePercent(in)}),
		})
	}

	// IF HHI > 0.45 → Alternative Sourcing (High)
	if hhi > 0.45 {
		out = append(out, ruleCandidate{
			id:          "alternative-sourcing",
			title:       fmt.Sprintf("Activate Alternative %s Sources", titleCase(commodityTitle(in.Commodity))),
			description: "Shift volume to pre-qualified alternate suppliers to lower concentration risk.",
			category:    CategoryAlternativeSourcing,
			reason: fmt.Sprintf(
				"Import concentration HHI of %.3f exceeds the high-risk threshold for %s.",
				hhi, commodityTitle(in.Commodity),
			),
			impact:     "Can restore 20–50% of disrupted volume within one procurement cycle when alternates exist.",
			priority:   PriorityHigh,
			difficulty: DifficultyHigh,
			confidence: ConfidenceHigh,
			metrics:    mergeMetrics(baseMetrics(in), SupportingMetrics{"hhi": hhi}),
		})
	}

	// IF Event Risk > 50 → Supplier Monitoring (Medium)
	if in.EventRisk > 50 {
		out = append(out, ruleCandidate{
			id:          "supplier-monitoring",
			title:       fmt.Sprintf("Increase Monitoring of %s Supply Chain", in.Source),
			description: "Expand supplier and transit monitoring to detect escalation early.",
			category:    CategorySupplierMonitoring,
			reason: fmt.Sprintf(
				"Event-risk score of %.0f for %s exceeds the monitoring threshold during this scenario.",
				in.EventRisk, in.Source,
			),
			impact:     "Improves early detection of escalation paths and shortens response lead time.",
			priority:   PriorityMedium,
			difficulty: DifficultyLow,
			confidence: ConfidenceHigh,
			metrics:    mergeMetrics(baseMetrics(in), SupportingMetrics{"event_risk": in.EventRisk}),
		})
	}

	// IF Macro Exposure > 40 → Country Risk Monitoring (Medium)
	if in.MacroExposure > 40 {
		out = append(out, ruleCandidate{
			id:          "country-risk-monitoring",
			title:       fmt.Sprintf("Monitor Country Risk for %s", in.Source),
			description: "Track macroeconomic and structural indicators affecting supplier-country stability.",
			category:    CategoryCountryRiskMonitoring,
			reason: fmt.Sprintf(
				"Macro exposure score of %.0f for %s indicates elevated structural country risk.",
				in.MacroExposure, in.Source,
			),
			impact:     "Supports proactive contingency planning before macro stress translates into supply delays.",
			priority:   PriorityMedium,
			difficulty: DifficultyLow,
			confidence: ConfidenceHigh,
			metrics:    mergeMetrics(baseMetrics(in), SupportingMetrics{"macro_exposure": in.MacroExposure}),
		})
	}

	// IF Shock Type == Export Collapse → Inventory Increase
	if in.ShockType == "export_collapse" || in.ShockType == "supply_cut" {
		priority := PriorityMedium
		confidence := ConfidenceMedium
		if in.ShockType == "export_collapse" {
			priority = PriorityHigh
			confidence = ConfidenceHigh
		}
		reason := fmt.Sprintf(
			"A %.0f%% %s shock on %s warrants additional inventory buffer while supply normalizes.",
			in.DropPercent, shockLabel(in.ShockType), commodityTitle(in.Commodity),
		)
		if in.InventoryBufferDays != nil && *in.InventoryBufferDays < 21 {
			reason = fmt.Sprintf(
				"Scenario assumes only %d days of inventory buffer during a %s shock with %.0f%% supply reduction.",
				*in.InventoryBufferDays, shockLabel(in.ShockType), in.DropPercent,
			)
		}
		metrics := baseMetrics(in)
		if in.InventoryBufferDays != nil {
			metrics["inventory_buffer_days"] = float64(*in.InventoryBufferDays)
		}
		out = append(out, ruleCandidate{
			id:          "inventory-increase",
			title:       fmt.Sprintf("Increase %s Inventory Buffer", titleCase(commodityTitle(in.Commodity))),
			description: "Build safety stock above routine operating inventory to absorb supply interruption.",
			category:    CategoryInventoryIncrease,
			reason:      reason,
			impact:      "Extends operational runway by 2–6 weeks and dampens immediate production disruptions.",
			priority:    priority,
			difficulty:  DifficultyMedium,
			confidence:  confidence,
			metrics:     metrics,
		})
	}

	// IF Shock Type == Route Disruption → Logistics Rerouting
	if in.ShockType == "route_disruption" {
		out = append(out, ruleCandidate{
			id:          "logistics-rerouting",
			title:       fmt.Sprintf("Reroute %s Logistics Corridors", titleCase(commodityTitle(in.Commodity))),
			description: "Activate alternate shipping lanes and inland transport to bypass disrupted routes.",
			category:    CategoryLogisticsRerouting,
			reason: fmt.Sprintf(
				"Route disruption on %s blocks established %s corridors; alternate lanes should be evaluated immediately.",
				in.Source, commodityTitle(in.Commodity),
			),
			impact:     "Maintains delivery continuity on secondary corridors and reduces dwell-time risk.",
			priority:   PriorityHigh,
			difficulty: DifficultyMedium,
			confidence: ConfidenceHigh,
			metrics:    baseMetrics(in),
		})
	}

	// Strategic stockpiling for strategic commodities under severe stress
	if isStrategicCommodity(in.Commodity) && (in.DropPercent >= 30 || in.MacroExposure >= 55 || in.ShockType == "price_spike") {
		out = append(out, ruleCandidate{
			id:          "strategic-stockpiling",
			title:       fmt.Sprintf("Build Strategic %s Reserve", titleCase(commodityTitle(in.Commodity))),
			description: "Establish a dedicated strategic reserve beyond routine operating inventory.",
			category:    CategoryStrategicStockpiling,
			reason: fmt.Sprintf(
				"A %.0f%% shock on strategic %s with macro exposure %.0f supports dedicated stockpiling.",
				in.DropPercent, commodityTitle(in.Commodity), in.MacroExposure,
			),
			impact:     "Buffers against prolonged supply tightness and reduces spot-market exposure during recovery.",
			priority:   PriorityMedium,
			difficulty: DifficultyHigh,
			confidence: ConfidenceMedium,
			metrics:    baseMetrics(in),
		})
	}

	// Contract hedging for price shocks and energy commodities
	if in.ShockType == "price_spike" || (isEnergyCommodity(in.Commodity) && in.MacroExposure >= 45) {
		out = append(out, ruleCandidate{
			id:          "contract-hedging",
			title:       fmt.Sprintf("Hedge %s Price Exposure", titleCase(commodityTitle(in.Commodity))),
			description: "Use forward contracts or indexed pricing to limit spot-market volatility during disruption.",
			category:    CategoryContractHedging,
			reason: fmt.Sprintf(
				"Price-spike or macro pressure (%.0f) on %s increases cost volatility under the simulated shock.",
				in.MacroExposure, commodityTitle(in.Commodity),
			),
			impact:     "Stabilizes procurement cost and protects margin during price volatility.",
			priority:   PriorityMedium,
			difficulty: DifficultyMedium,
			confidence: ConfidenceMedium,
			metrics:    baseMetrics(in),
		})
	}

	// Demand reduction under severe disruption and high fragility
	if in.DropPercent >= 35 && in.FragilityScore >= 10 {
		out = append(out, ruleCandidate{
			id:          "demand-reduction",
			title:       "Temporarily Reduce Non-Critical Demand",
			description: "Prioritize essential consumption and defer discretionary demand until supply stabilizes.",
			category:    CategoryDemandReduction,
			reason: fmt.Sprintf(
				"A %.0f%% shock with fragility impact %.1f indicates supply cannot fully meet baseline demand.",
				in.DropPercent, in.FragilityScore,
			),
			impact:     "Preserves limited supply for critical operations and reduces systemic amplification.",
			priority:   PriorityMedium,
			difficulty: DifficultyLow,
			confidence: ConfidenceMedium,
			metrics:    baseMetrics(in),
		})
	}

	// Dual sourcing when both share and HHI are elevated
	if share > 0.50 && hhi > 0.30 && (isSemiconductorOrMineral(in.Commodity)) {
		out = append(out, ruleCandidate{
			id:          "dual-sourcing",
			title:       fmt.Sprintf("Establish Dual Sourcing for %s", titleCase(commodityTitle(in.Commodity))),
			description: "Split procurement across two qualified suppliers to limit single-point failure.",
			category:    CategoryDualSourcing,
			reason: fmt.Sprintf(
				"Supplier share %.0f%% and HHI %.3f on %s indicate dual sourcing is required for resilience.",
				sharePercent(in), hhi, commodityTitle(in.Commodity),
			),
			impact:     "Provides operational continuity if one supplier is disrupted.",
			priority:   PriorityHigh,
			difficulty: DifficultyHigh,
			confidence: ConfidenceHigh,
			metrics: mergeMetrics(baseMetrics(in), SupportingMetrics{
				"supplier_share": sharePercent(in),
				"hhi":            hhi,
			}),
		})
	}

	// Moderate concentration: diversify at medium priority when HHI 0.15–0.45
	if hhi >= 0.15 && hhi <= 0.45 && share > 0.35 && share <= 0.60 {
		out = append(out, ruleCandidate{
			id:          "supplier-diversification-moderate",
			title:       fmt.Sprintf("Broaden %s Supplier Base", titleCase(commodityTitle(in.Commodity))),
			description: "Qualify additional suppliers to reduce moderate concentration risk.",
			category:    CategorySupplierDiversification,
			reason: fmt.Sprintf(
				"Moderate concentration (HHI %.3f, share %.0f%%) on %s from %s warrants diversification.",
				hhi, sharePercent(in), commodityTitle(in.Commodity), in.Source,
			),
			impact:     "Lowers concentration risk before it reaches critical thresholds.",
			priority:   PriorityMedium,
			difficulty: DifficultyMedium,
			confidence: ConfidenceMedium,
			metrics: mergeMetrics(baseMetrics(in), SupportingMetrics{
				"hhi":            hhi,
				"supplier_share": sharePercent(in),
			}),
		})
	}

	for i := range out {
		out[i].priority = applyDropBoost(out[i].priority, in.DropPercent)
	}
	return out
}

func titleCase(s string) string {
	if s == "" {
		return s
	}
	parts := strings.Fields(strings.ToLower(s))
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func isStrategicCommodity(commodity string) bool {
	switch strings.ToLower(strings.TrimSpace(commodity)) {
	case "semiconductors", "crude oil", "natural gas", "lng", "lithium", "cobalt",
		"nickel", "copper", "rare earths", "wheat", "corn", "rice", "fertilizer",
		"uranium", "steel", "aluminum", "batteries", "solar panels", "pharmaceuticals":
		return true
	default:
		return false
	}
}

func isEnergyCommodity(commodity string) bool {
	switch strings.ToLower(strings.TrimSpace(commodity)) {
	case "crude oil", "natural gas", "lng", "uranium":
		return true
	default:
		return false
	}
}

func isSemiconductorOrMineral(commodity string) bool {
	switch strings.ToLower(strings.TrimSpace(commodity)) {
	case "semiconductors", "lithium", "cobalt", "nickel", "copper", "rare earths":
		return true
	default:
		return false
	}
}
