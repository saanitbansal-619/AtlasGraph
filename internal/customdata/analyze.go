// Package customdata parses client-supplied supplier dependency CSV files and
// computes deterministic supplier concentration analytics.
package customdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
)

type Row struct {
	Importer  string  `json:"importer"`
	Commodity string  `json:"commodity"`
	Supplier  string  `json:"supplier"`
	ValueUSD  float64 `json:"value_usd"`
}

type ValidationError struct {
	Row     int    `json:"row"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type DatasetSummary struct {
	RowsProcessed int     `json:"rows_processed"`
	ValidRows     int     `json:"valid_rows"`
	InvalidRows   int     `json:"invalid_rows"`
	Importers     int     `json:"importers"`
	Commodities   int     `json:"commodities"`
	Suppliers     int     `json:"suppliers"`
	TotalValueUSD float64 `json:"total_value_usd"`
}

type ConcentrationResult struct {
	Importer          string  `json:"importer"`
	Commodity         string  `json:"commodity"`
	TotalValueUSD     float64 `json:"total_value_usd"`
	SupplierCount     int     `json:"supplier_count"`
	TopSupplier       string  `json:"top_supplier"`
	TopSupplierShare  float64 `json:"top_supplier_share"`
	HHI               float64 `json:"hhi"`
	ConcentrationRisk string  `json:"concentration_risk"`
}

type Analysis struct {
	DatasetSummary       DatasetSummary        `json:"dataset_summary"`
	ConcentrationResults []ConcentrationResult `json:"concentration_results"`
	ValidationErrors     []ValidationError     `json:"validation_errors"`
	// ValidRows are the normalized supplier dependency rows used for overlay matching.
	ValidRows []Row `json:"normalized_rows"`
}

var aliases = map[string]string{
	"importer":        "importer",
	"importer_name":   "importer",
	"commodity":       "commodity",
	"supplier":        "supplier",
	"supplier_name":   "supplier",
	"exporter":        "supplier",
	"value_usd":       "value_usd",
	"trade_value_usd": "value_usd",
	"value":           "value_usd",
}

func Analyze(r io.Reader) (Analysis, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err == io.EOF {
		return Analysis{}, fmt.Errorf("CSV is empty")
	}
	if err != nil {
		return Analysis{}, fmt.Errorf("read CSV header: %w", err)
	}
	columns, err := resolveColumns(header)
	if err != nil {
		return Analysis{}, err
	}

	analysis := Analysis{
		ConcentrationResults: []ConcentrationResult{},
		ValidationErrors:     []ValidationError{},
		ValidRows:            []Row{},
	}
	line := 1
	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		line++
		analysis.DatasetSummary.RowsProcessed++
		if readErr != nil {
			analysis.ValidationErrors = append(analysis.ValidationErrors, ValidationError{
				Row: line, Field: "row", Message: readErr.Error(),
			})
			continue
		}
		row, errors := parseRow(record, columns, line)
		if len(errors) > 0 {
			analysis.ValidationErrors = append(analysis.ValidationErrors, errors...)
			continue
		}
		analysis.ValidRows = append(analysis.ValidRows, row)
	}
	if analysis.DatasetSummary.RowsProcessed == 0 {
		return Analysis{}, fmt.Errorf("CSV contains a header but no data rows")
	}
	analysis.DatasetSummary.ValidRows = len(analysis.ValidRows)
	analysis.DatasetSummary.InvalidRows = analysis.DatasetSummary.RowsProcessed - len(analysis.ValidRows)
	completeAnalysis(&analysis)
	return analysis, nil
}

func resolveColumns(header []string) (map[string]int, error) {
	columns := map[string]int{}
	for index, raw := range header {
		name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff")))
		canonical, ok := aliases[name]
		if !ok {
			continue
		}
		if _, exists := columns[canonical]; exists {
			return nil, fmt.Errorf("CSV contains multiple columns for %q", canonical)
		}
		columns[canonical] = index
	}
	missing := []string{}
	for _, required := range []string{"importer", "commodity", "supplier", "value_usd"} {
		if _, ok := columns[required]; !ok {
			missing = append(missing, required)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("CSV missing required columns: %s", strings.Join(missing, ", "))
	}
	return columns, nil
}

func parseRow(record []string, columns map[string]int, line int) (Row, []ValidationError) {
	value := func(field string) string {
		index := columns[field]
		if index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}
	row := Row{
		Importer:  value("importer"),
		Commodity: strings.ToLower(value("commodity")),
		Supplier:  value("supplier"),
	}
	errors := []ValidationError{}
	for field, text := range map[string]string{
		"importer": row.Importer, "commodity": row.Commodity, "supplier": row.Supplier,
	} {
		if text == "" {
			errors = append(errors, ValidationError{Row: line, Field: field, Message: "is required"})
		}
	}
	rawValue := value("value_usd")
	if rawValue == "" {
		errors = append(errors, ValidationError{Row: line, Field: "value_usd", Message: "is required"})
	} else {
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(rawValue, ",", ""), 64)
		if err != nil || parsed <= 0 || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			errors = append(errors, ValidationError{Row: line, Field: "value_usd", Message: "must be a positive number"})
		} else {
			row.ValueUSD = parsed
		}
	}
	sort.SliceStable(errors, func(i, j int) bool { return errors[i].Field < errors[j].Field })
	return row, errors
}

type group struct {
	importer  string
	commodity string
	suppliers map[string]*supplierTotal
	total     float64
}

type supplierTotal struct {
	name  string
	value float64
}

func completeAnalysis(analysis *Analysis) {
	importers := map[string]struct{}{}
	commodities := map[string]struct{}{}
	suppliers := map[string]struct{}{}
	groups := map[string]*group{}
	for _, row := range analysis.ValidRows {
		importerKey := strings.ToLower(row.Importer)
		supplierKey := strings.ToLower(row.Supplier)
		key := importerKey + "\x00" + row.Commodity
		g := groups[key]
		if g == nil {
			g = &group{importer: row.Importer, commodity: row.Commodity, suppliers: map[string]*supplierTotal{}}
			groups[key] = g
		}
		s := g.suppliers[supplierKey]
		if s == nil {
			s = &supplierTotal{name: row.Supplier}
			g.suppliers[supplierKey] = s
		}
		s.value += row.ValueUSD
		g.total += row.ValueUSD
		analysis.DatasetSummary.TotalValueUSD += row.ValueUSD
		importers[importerKey] = struct{}{}
		commodities[row.Commodity] = struct{}{}
		suppliers[supplierKey] = struct{}{}
	}
	analysis.DatasetSummary.Importers = len(importers)
	analysis.DatasetSummary.Commodities = len(commodities)
	analysis.DatasetSummary.Suppliers = len(suppliers)

	for _, g := range groups {
		result := ConcentrationResult{
			Importer: g.importer, Commodity: g.commodity,
			TotalValueUSD: g.total, SupplierCount: len(g.suppliers),
		}
		for _, supplier := range g.suppliers {
			share := supplier.value / g.total
			result.HHI += share * share
			if supplier.value > result.TopSupplierShare*g.total ||
				(supplier.value == result.TopSupplierShare*g.total && supplier.name < result.TopSupplier) {
				result.TopSupplier = supplier.name
				result.TopSupplierShare = share
			}
		}
		result.ConcentrationRisk = riskLevel(result.HHI)
		analysis.ConcentrationResults = append(analysis.ConcentrationResults, result)
	}
	sort.SliceStable(analysis.ConcentrationResults, func(i, j int) bool {
		a, b := analysis.ConcentrationResults[i], analysis.ConcentrationResults[j]
		if a.HHI != b.HHI {
			return a.HHI > b.HHI
		}
		if a.Importer != b.Importer {
			return a.Importer < b.Importer
		}
		return a.Commodity < b.Commodity
	})
}

func riskLevel(hhi float64) string {
	switch {
	case hhi >= 0.25:
		return "High"
	case hhi >= 0.15:
		return "Medium"
	default:
		return "Low"
	}
}
