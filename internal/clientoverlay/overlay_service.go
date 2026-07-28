// Package clientoverlay matches uploaded client supplier data against shock
// scenarios and computes organization-specific exposure metrics.
package clientoverlay

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/atlasgraph/atlas/internal/customdata"
)

// ExposureRow is one matched importer+commodity exposure line.
type ExposureRow struct {
	Importer               string  `json:"importer"`
	Commodity              string  `json:"commodity"`
	Supplier               string  `json:"supplier"`
	SupplierValueUSD       float64 `json:"supplier_value_usd"`
	TotalImportValueUSD    float64 `json:"total_import_value_usd"`
	SupplierShare          float64 `json:"supplier_share"`
	HHI                    float64 `json:"hhi,omitempty"`
	ConcentrationRisk      string  `json:"concentration_risk,omitempty"`
	EstimatedExposedTrade  float64 `json:"estimated_exposed_trade"`
	EstimatedRemainingTrade float64 `json:"estimated_remaining_trade"`
}

// OverlayResult aggregates matched client exposures for a shock scenario.
type OverlayResult struct {
	Exposures                []ExposureRow `json:"exposures"`
	MatchedImporters         int           `json:"matched_importers"`
	TotalEstimatedExposedUSD float64       `json:"total_estimated_exposed_trade_usd"`
	HighestSupplierShare     float64       `json:"highest_supplier_share"`
	HighestHHI               float64       `json:"highest_hhi"`
	AverageConcentrationRisk string        `json:"average_concentration_risk"`
	TopImporter              string        `json:"top_importer,omitempty"`
	Commodity                string        `json:"commodity"`
	Source                   string        `json:"source"`
	ShockDropPercent         float64       `json:"shock_drop_percent"`
	Assessment               string        `json:"assessment"`
}

// Request identifies a shock scenario against client analysis data.
type Request struct {
	Source      string
	Commodity   string
	DropPercent float64
}

var commodityAliases = map[string]string{
	"lng":                   "natural_gas",
	"liquefied_natural_gas":   "natural_gas",
	"natural_gas":             "natural_gas",
	"oil":                     "crude_oil",
	"petroleum":               "crude_oil",
	"crude_oil":               "crude_oil",
	"rare_earth":              "rare_earths",
	"rare_earth_metals":       "rare_earths",
	"rare_earth_compounds":    "rare_earths",
	"rare_earths":             "rare_earths",
	"lithium_carbonate":       "lithium",
	"lithium_carbonates":      "lithium",
}

func normToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func commodityKey(value string) string {
	key := normToken(value)
	key = strings.ReplaceAll(key, " ", "_")
	for strings.Contains(key, "__") {
		key = strings.ReplaceAll(key, "__", "_")
	}
	key = strings.Trim(key, "_")
	if alias, ok := commodityAliases[key]; ok {
		return alias
	}
	return key
}

func commoditiesMatch(a, b string) bool {
	return commodityKey(a) == commodityKey(b)
}

func suppliersMatch(shockSource, supplier string) bool {
	return normToken(shockSource) == normToken(supplier)
}

type concentrationKey struct {
	importer  string
	commodity string
}

func concentrationIndex(results []customdata.ConcentrationResult) map[concentrationKey]customdata.ConcentrationResult {
	out := make(map[concentrationKey]customdata.ConcentrationResult, len(results))
	for _, row := range results {
		out[concentrationKey{
			importer:  normToken(row.Importer),
			commodity: commodityKey(row.Commodity),
		}] = row
	}
	return out
}

type groupAcc struct {
	importer         string
	commodity        string
	supplier         string
	supplierValueUSD float64
}

// Compute matches client analysis rows to a shock and returns exposure metrics.
func Compute(analysis customdata.Analysis, req Request) OverlayResult {
	result := OverlayResult{
		Commodity:        strings.TrimSpace(req.Commodity),
		Source:           strings.TrimSpace(req.Source),
		ShockDropPercent: req.DropPercent,
		Exposures:        []ExposureRow{},
	}
	if strings.TrimSpace(req.Source) == "" || strings.TrimSpace(req.Commodity) == "" {
		return result
	}

	concentrations := concentrationIndex(analysis.ConcentrationResults)
	groups := map[concentrationKey]groupAcc{}

	for _, row := range analysis.ValidRows {
		if !suppliersMatch(req.Source, row.Supplier) {
			continue
		}
		if !commoditiesMatch(req.Commodity, row.Commodity) {
			continue
		}
		key := concentrationKey{
			importer:  normToken(row.Importer),
			commodity: commodityKey(row.Commodity),
		}
		acc, ok := groups[key]
		if !ok {
			acc = groupAcc{
				importer:  row.Importer,
				commodity: row.Commodity,
				supplier:  row.Supplier,
			}
		}
		acc.supplierValueUSD += row.ValueUSD
		groups[key] = acc
	}

	dropFactor := req.DropPercent / 100
	if dropFactor < 0 {
		dropFactor = 0
	}

	exposures := make([]ExposureRow, 0, len(groups))
	for key, group := range groups {
		concentration, hasConcentration := concentrations[key]
		total := group.supplierValueUSD
		if hasConcentration && concentration.TotalValueUSD > 0 {
			total = concentration.TotalValueUSD
		}
		share := 0.0
		if total > 0 {
			share = group.supplierValueUSD / total
		}
		exposed := group.supplierValueUSD * dropFactor
		remaining := group.supplierValueUSD - exposed
		if remaining < 0 {
			remaining = 0
		}

		row := ExposureRow{
			Importer:                group.importer,
			Commodity:               group.commodity,
			Supplier:                group.supplier,
			SupplierValueUSD:        group.supplierValueUSD,
			TotalImportValueUSD:     total,
			SupplierShare:           share,
			EstimatedExposedTrade:   exposed,
			EstimatedRemainingTrade: remaining,
		}
		if hasConcentration {
			row.HHI = concentration.HHI
			row.ConcentrationRisk = concentration.ConcentrationRisk
		}
		exposures = append(exposures, row)
	}

	sort.SliceStable(exposures, func(i, j int) bool {
		if exposures[i].EstimatedExposedTrade != exposures[j].EstimatedExposedTrade {
			return exposures[i].EstimatedExposedTrade > exposures[j].EstimatedExposedTrade
		}
		if exposures[i].SupplierShare != exposures[j].SupplierShare {
			return exposures[i].SupplierShare > exposures[j].SupplierShare
		}
		return exposures[i].Importer < exposures[j].Importer
	})

	result.Exposures = exposures
	result.MatchedImporters = len(exposures)
	if len(exposures) == 0 {
		return result
	}

	top := exposures[0]
	result.TopImporter = top.Importer
	result.HighestSupplierShare = top.SupplierShare
	result.HighestHHI = maxHHI(exposures)

	var totalExposed float64
	riskScore := 0.0
	riskCount := 0
	for _, row := range exposures {
		totalExposed += row.EstimatedExposedTrade
		if row.ConcentrationRisk != "" {
			riskScore += riskToScore(row.ConcentrationRisk)
			riskCount++
		}
		if row.SupplierShare > result.HighestSupplierShare {
			result.HighestSupplierShare = row.SupplierShare
		}
		if row.HHI > result.HighestHHI {
			result.HighestHHI = row.HHI
		}
	}
	result.TotalEstimatedExposedUSD = totalExposed
	result.AverageConcentrationRisk = averageRiskLabel(riskScore, riskCount)
	result.Assessment = BuildAssessment(result)
	return result
}

func maxHHI(rows []ExposureRow) float64 {
	max := 0.0
	for _, row := range rows {
		if row.HHI > max {
			max = row.HHI
		}
	}
	return max
}

func riskToScore(risk string) float64 {
	switch strings.TrimSpace(risk) {
	case "High":
		return 3
	case "Medium":
		return 2
	case "Low":
		return 1
	default:
		return 0
	}
}

func averageRiskLabel(score float64, count int) string {
	if count == 0 {
		return ""
	}
	avg := score / float64(count)
	switch {
	case avg >= 2.5:
		return "High"
	case avg >= 1.5:
		return "Medium"
	default:
		return "Low"
	}
}

// BuildAssessment generates the Client Exposure Assessment narrative.
func BuildAssessment(result OverlayResult) string {
	if result.MatchedImporters == 0 || len(result.Exposures) == 0 {
		return ""
	}
	top := result.Exposures[0]
	sharePct := top.SupplierShare * 100
	exposed := formatCompactUSD(top.EstimatedExposedTrade)
	commodity := strings.TrimSpace(result.Commodity)
	if commodity == "" {
		commodity = "the shocked commodity"
	}
	return fmt.Sprintf(
		"The uploaded client portfolio indicates direct dependence on %s for %s imports. %s has approximately %.0f%% supplier dependence on %s, resulting in an estimated %s of directly exposed trade under the simulated disruption.",
		result.Source,
		commodity,
		result.TopImporter,
		sharePct,
		result.Source,
		exposed,
	)
}
func FormatCompactUSD(value float64) string {
	return formatCompactUSD(value)
}

func formatCompactUSD(value float64) string {
	abs := math.Abs(value)
	switch {
	case abs >= 1e12:
		return fmt.Sprintf("$%.1fT", value/1e12)
	case abs >= 1e9:
		return fmt.Sprintf("$%.1fB", value/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("$%.1fM", value/1e6)
	case abs >= 1e3:
		return fmt.Sprintf("$%.1fK", value/1e3)
	default:
		return fmt.Sprintf("$%.0f", value)
	}
}
