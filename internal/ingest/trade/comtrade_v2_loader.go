package trade

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	v2PeriodCols    = []string{"period", "refyear", "year"}
	v2FlowCols      = []string{"trade flow", "flowdesc", "flow"}
	v2ReporterCols  = []string{"reporter", "reporterdesc", "reporter_iso", "reporteriso"}
	v2PartnerCols   = []string{"partner", "partnerdesc", "partner_iso", "partneriso"}
	v2HSCodeCols    = []string{"commodity code", "cmdcode", "commoditycode", "hs code"}
	v2HSDescCols    = []string{"cmddesc", "commodity", "commodity desc"}
	v2ValueCols     = []string{"trade value (us$)", "primaryvalue", "tradevalue", "value", "trade value"}
	v2WeightCols    = []string{"net weight (kg)", "netweight", "net weight"}
	v2QtyCols       = []string{"qty", "quantity"}
	v2QtyUnitCols   = []string{"qty unit", "qtyunit", "qtyunitabbr", "unit"}
)

// LoadComtradeV2File parses one flexible UN Comtrade CSV export.
func LoadComtradeV2File(path string) (ComtradeV2LoadResult, []aggregatedFlow, error) {
	f, err := os.Open(path)
	if err != nil {
		return ComtradeV2LoadResult{}, nil, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()
	return ParseComtradeV2CSV(f)
}

// ParseComtradeV2CSV reads flexible Comtrade CSV rows into annual aggregated flows.
func ParseComtradeV2CSV(r io.Reader) (ComtradeV2LoadResult, []aggregatedFlow, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err == io.EOF {
		return ComtradeV2LoadResult{}, nil, fmt.Errorf("comtrade CSV is empty")
	}
	if err != nil {
		return ComtradeV2LoadResult{}, nil, fmt.Errorf("reading header: %w", err)
	}

	idx := indexV2Header(header)
	var res ComtradeV2LoadResult
	res.CommoditiesMapped = map[string]struct{}{}
	res.Importers = map[string]struct{}{}
	res.Exporters = map[string]struct{}{}

	byKey := map[string]*aggregatedFlow{}
	line := 1
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		line++
		if err != nil {
			return res, nil, fmt.Errorf("reading line %d: %w", line, err)
		}
		if isBlankRow(row) {
			continue
		}
		res.RawRows++

		flow, reason := parseComtradeV2Row(row, idx, &res)
		if reason != "" {
			res.Skipped = append(res.Skipped, SkippedRow{Line: line, Reason: reason})
			continue
		}
		res.ValidRows++
		key := flowKey(flow)
		agg, ok := byKey[key]
		if !ok {
			copyFlow := flow
			agg = &copyFlow
			byKey[key] = agg
		} else {
			agg.tradeValueUSD += flow.tradeValueUSD
			agg.netWeightKg += flow.netWeightKg
			agg.quantity += flow.quantity
			if agg.quantityUnit == "" {
				agg.quantityUnit = flow.quantityUnit
			}
		}
		if res.YearMin == 0 || flow.year < res.YearMin {
			res.YearMin = flow.year
		}
		if flow.year > res.YearMax {
			res.YearMax = flow.year
		}
	}

	flows := make([]aggregatedFlow, 0, len(byKey))
	for _, f := range byKey {
		flows = append(flows, *f)
	}
	sort.SliceStable(flows, func(i, j int) bool {
		a, b := flows[i], flows[j]
		if a.year != b.year {
			return a.year < b.year
		}
		if a.importer != b.importer {
			return a.importer < b.importer
		}
		if a.commodity != b.commodity {
			return a.commodity < b.commodity
		}
		return a.exporter < b.exporter
	})
	return res, flows, nil
}

// IngestUNComtradeDir ingests every CSV in a directory.
func IngestUNComtradeDir(dir string) (DependencyFile, ComtradeV2LoadResult, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return DependencyFile{}, ComtradeV2LoadResult{}, fmt.Errorf("reading dir %q: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".csv") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return DependencyFile{}, ComtradeV2LoadResult{}, fmt.Errorf("no CSV files found in %s", dir)
	}
	return IngestUNComtradeFiles(paths)
}

// IngestUNComtradeFiles ingests one or more UN Comtrade CSV exports.
func IngestUNComtradeFiles(paths []string) (DependencyFile, ComtradeV2LoadResult, error) {
	var total ComtradeV2LoadResult
	total.CommoditiesMapped = map[string]struct{}{}
	total.Importers = map[string]struct{}{}
	total.Exporters = map[string]struct{}{}

	var allFlows []aggregatedFlow
	for _, path := range paths {
		res, flows, err := LoadComtradeV2File(path)
		if err != nil {
			return DependencyFile{}, total, err
		}
		total.FilesProcessed++
		total.RawRows += res.RawRows
		total.ValidRows += res.ValidRows
		total.SkippedAggregateRows += res.SkippedAggregateRows
		total.SkippedUnmapped += res.SkippedUnmapped
		total.Skipped = append(total.Skipped, res.Skipped...)
		for k := range res.CommoditiesMapped {
			total.CommoditiesMapped[k] = struct{}{}
		}
		for k := range res.Importers {
			total.Importers[k] = struct{}{}
		}
		for k := range res.Exporters {
			total.Exporters[k] = struct{}{}
		}
		if total.YearMin == 0 || (res.YearMin > 0 && res.YearMin < total.YearMin) {
			total.YearMin = res.YearMin
		}
		if res.YearMax > total.YearMax {
			total.YearMax = res.YearMax
		}
		allFlows = append(allFlows, flows...)
	}
	if len(allFlows) == 0 {
		return DependencyFile{}, total, fmt.Errorf("no valid trade rows found in %d file(s)", len(paths))
	}

	merged := mergeAggregatedFlows(allFlows)
	deps := buildDependenciesWithShares(merged, ComtradeRealSourceName)
	year := total.YearMax
	if year == 0 {
		year = total.YearMin
	}
	file := DependencyFile{
		Source:       ComtradeRealSourceName,
		Year:         year,
		GeneratedAt:  time.Now().UTC(),
		Dependencies: deps,
	}
	return file, total, nil
}

func mergeAggregatedFlows(flows []aggregatedFlow) []aggregatedFlow {
	byKey := map[string]*aggregatedFlow{}
	for _, f := range flows {
		key := flowKey(f)
		agg, ok := byKey[key]
		if !ok {
			copyFlow := f
			agg = &copyFlow
			byKey[key] = agg
		} else {
			agg.tradeValueUSD += f.tradeValueUSD
			agg.netWeightKg += f.netWeightKg
			agg.quantity += f.quantity
		}
	}
	out := make([]aggregatedFlow, 0, len(byKey))
	for _, f := range byKey {
		out = append(out, *f)
	}
	return out
}

func buildDependenciesWithShares(flows []aggregatedFlow, source string) []TradeDependency {
	totals := map[string]float64{}
	for _, f := range flows {
		totals[groupKey(f.importer, f.commodity, f.year)] += f.tradeValueUSD
	}
	out := make([]TradeDependency, 0, len(flows))
	for _, f := range flows {
		share := 0.0
		if total := totals[groupKey(f.importer, f.commodity, f.year)]; total > 0 {
			share = f.tradeValueUSD / total
		}
		out = append(out, TradeDependency{
			Importer: f.importer, Exporter: f.exporter, Commodity: f.commodity,
			HSCode: f.hsCode, Year: f.year, TradeValueUSD: f.tradeValueUSD,
			NetWeightKg: f.netWeightKg, Quantity: f.quantity, QuantityUnit: f.quantityUnit,
			Share: share, Source: source,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Importer != b.Importer {
			return a.Importer < b.Importer
		}
		if a.Commodity != b.Commodity {
			return a.Commodity < b.Commodity
		}
		if a.Share != b.Share {
			return a.Share > b.Share
		}
		return a.Exporter < b.Exporter
	})
	return out
}

func flowKey(f aggregatedFlow) string {
	return fmt.Sprintf("%s|%s|%s|%s|%d", f.importer, f.exporter, f.commodity, f.hsCode, f.year)
}

func groupKey(importer, commodity string, year int) string {
	return fmt.Sprintf("%s|%s|%d", importer, commodity, year)
}

type v2HeaderIndex map[string]int

func indexV2Header(header []string) v2HeaderIndex {
	idx := v2HeaderIndex{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		if key != "" {
			idx[key] = i
		}
	}
	return idx
}

func (idx v2HeaderIndex) first(cols []string) (int, bool) {
	for _, c := range cols {
		if i, ok := idx[c]; ok {
			return i, true
		}
	}
	return -1, false
}

func (idx v2HeaderIndex) get(row []string, cols []string) string {
	i, ok := idx.first(cols)
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func parseComtradeV2Row(row []string, idx v2HeaderIndex, res *ComtradeV2LoadResult) (aggregatedFlow, string) {
	flowRaw := strings.ToLower(idx.get(row, v2FlowCols))
	isImport := strings.Contains(flowRaw, "import") || flowRaw == "m"
	isExport := strings.Contains(flowRaw, "export") || flowRaw == "x"
	if !isImport && !isExport {
		if flowRaw == "" {
			isImport = true
		} else {
			return aggregatedFlow{}, fmt.Sprintf("unsupported flow %q", flowRaw)
		}
	}

	reporter := NormalizeCountryName(idx.get(row, v2ReporterCols))
	partner := NormalizeCountryName(idx.get(row, v2PartnerCols))
	if partner == "" {
		partner = strings.TrimSpace(idx.get(row, v2PartnerCols))
	}
	if isAggregatePartner(partner) {
		res.SkippedAggregateRows++
		return aggregatedFlow{}, "aggregate partner row"
	}
	if !looksLikeCountry(partner) {
		return aggregatedFlow{}, fmt.Sprintf("invalid partner %q", partner)
	}
	if reporter == "" {
		reporter = strings.TrimSpace(idx.get(row, v2ReporterCols))
	}

	var importer, exporter string
	if isImport {
		importer, exporter = reporter, partner
	} else {
		importer, exporter = partner, reporter
	}
	if importer == "" || exporter == "" {
		return aggregatedFlow{}, "missing importer or exporter"
	}

	year, reason := parsePeriodYear(idx.get(row, v2PeriodCols))
	if reason != "" {
		return aggregatedFlow{}, reason
	}

	hsCode := idx.get(row, v2HSCodeCols)
	desc := idx.get(row, v2HSDescCols)
	commodity, ok := commodityFromHSOrDesc(hsCode, desc)
	if !ok {
		res.SkippedUnmapped++
		return aggregatedFlow{}, fmt.Sprintf("unmapped commodity hs=%q desc=%q", hsCode, desc)
	}

	valueRaw := idx.get(row, v2ValueCols)
	if valueRaw == "" {
		return aggregatedFlow{}, "missing trade value"
	}
	value, vreason := parseAmount(valueRaw, "trade value")
	if vreason != "" {
		return aggregatedFlow{}, vreason
	}

	weight, _ := parseAmount(idx.get(row, v2WeightCols), "net weight")
	qty, _ := parseAmount(idx.get(row, v2QtyCols), "qty")
	qtyUnit := idx.get(row, v2QtyUnitCols)

	res.CommoditiesMapped[commodity] = struct{}{}
	res.Importers[importer] = struct{}{}
	res.Exporters[exporter] = struct{}{}

	return aggregatedFlow{
		importer: importer, exporter: exporter, commodity: commodity, hsCode: hsCode,
		year: year, tradeValueUSD: value, netWeightKg: weight, quantity: qty, quantityUnit: qtyUnit,
	}, ""
}

func parsePeriodYear(raw string) (int, string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, "missing period"
	}
	digits := extractDigits(raw)
	switch len(digits) {
	case 4:
		year, err := strconv.Atoi(digits)
		if err != nil || year <= 0 {
			return 0, fmt.Sprintf("invalid period %q", raw)
		}
		return year, ""
	case 6:
		year, err := strconv.Atoi(digits[:4])
		if err != nil || year <= 0 {
			return 0, fmt.Sprintf("invalid period %q", raw)
		}
		return year, ""
	default:
		year, err := strconv.Atoi(digits)
		if err != nil || year < 1900 || year > 2100 {
			return 0, fmt.Sprintf("invalid period %q", raw)
		}
		return year, ""
	}
}
