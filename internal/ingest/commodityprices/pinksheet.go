package commodityprices

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

const (
	pinkSheetMonthlySheet = "Monthly Prices"
	pinkSheetScanRows     = 30
)

var (
	pinkSheetMonthRE   = regexp.MustCompile(`^(\d{4})M(\d{1,2})$`)
	pinkSheetYYYYMMRE  = regexp.MustCompile(`^(\d{4})-(\d{2})$`)
	pinkSheetMonYearRE = regexp.MustCompile(`^([A-Za-z]{3})-(\d{4})$`)
)

// PinkSheetMeta captures ingest diagnostics for a Pink Sheet XLSX pull.
type PinkSheetMeta struct {
	MappedSeries  int
	SkippedSeries []string
	MissingGFIP   []string
	SheetName     string
	Layout        string // "column-oriented" or "row-oriented"
}

// PinkSheetLoadResult is the outcome of parsing a Pink Sheet monthly XLSX.
type PinkSheetLoadResult struct {
	LoadResult
	Meta PinkSheetMeta
}

type pinkSheetColumn struct {
	index     int
	source    string
	commodity MappedCommodity
}

type pinkSheetDateCol struct {
	index int
	month string
	raw   string
}

// LoadPinkSheetFile opens and parses a World Bank Pink Sheet monthly XLSX.
func LoadPinkSheetFile(path string) (PinkSheetLoadResult, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return PinkSheetLoadResult{}, fmt.Errorf("opening %q: %w", path, err)
	}
	defer f.Close()
	res, err := ParsePinkSheetXLSX(f)
	if err != nil {
		return res, err
	}
	_ = path
	return res, nil
}

// ParsePinkSheetXLSX parses the Monthly Prices sheet from an open workbook.
func ParsePinkSheetXLSX(f *excelize.File) (PinkSheetLoadResult, error) {
	sheet := findMonthlyPricesSheet(f)
	if sheet == "" {
		return PinkSheetLoadResult{}, fmt.Errorf("could not find %q sheet in Pink Sheet workbook", pinkSheetMonthlySheet)
	}

	rows, err := f.GetRows(sheet)
	if err != nil {
		return PinkSheetLoadResult{}, fmt.Errorf("reading sheet %q: %w", sheet, err)
	}
	if len(rows) == 0 {
		return PinkSheetLoadResult{}, fmt.Errorf("sheet %q is empty", sheet)
	}

	// Column-oriented: commodity names across a header row, monthly dates down column A.
	if res, ok := parsePinkSheetColumnOriented(sheet, rows); ok && res.ValidRows() > 0 {
		res.Meta.MissingGFIP = StrategicGFIPCommoditiesMissing(mappedCodesFromRecords(res.Records))
		return res, nil
	}

	// Row-oriented: commodity/series names down rows, monthly dates across columns.
	if res, ok := parsePinkSheetRowOriented(sheet, rows); ok && res.ValidRows() > 0 {
		res.Meta.MissingGFIP = StrategicGFIPCommoditiesMissing(mappedCodesFromRecords(res.Records))
		return res, nil
	}

	return PinkSheetLoadResult{}, pinkSheetNoMapError(rows)
}

func parsePinkSheetColumnOriented(sheet string, rows [][]string) (PinkSheetLoadResult, bool) {
	headerRow, columns, skipped := findCommodityHeaderRow(rows)
	if headerRow < 0 || len(columns) == 0 {
		return PinkSheetLoadResult{}, false
	}

	dataStart := findDateColumnDataStart(rows, headerRow)
	if dataStart < 0 {
		return PinkSheetLoadResult{}, false
	}

	var res PinkSheetLoadResult
	res.Meta.SheetName = sheet
	res.Meta.Layout = "column-oriented"
	res.Meta.MappedSeries = len(columns)
	res.Meta.SkippedSeries = skipped

	for rowIdx := dataStart; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx]
		if len(row) == 0 {
			continue
		}
		month, ok := parsePinkSheetMonth(cellAt(row, 0))
		if !ok {
			continue
		}
		res.TotalRows++

		for _, col := range columns {
			if col.index >= len(row) {
				continue
			}
			price, ok := parsePinkSheetPrice(cellAt(row, col.index))
			if !ok {
				continue
			}
			res.Records = append(res.Records, PriceRecord{
				Date:          month,
				CommodityCode: col.commodity.Code,
				CommodityName: col.commodity.Name,
				PriceUSD:      price,
				Unit:          "USD",
				Source:        PinkSheetSourceName,
			})
		}
	}
	// Column-oriented needs mapped columns and parsed records.
	return res, len(columns) > 0 && len(res.Records) > 0
}

func parsePinkSheetRowOriented(sheet string, rows [][]string) (PinkSheetLoadResult, bool) {
	dateRow, dateCols := findDateHeaderRow(rows)
	if dateRow < 0 || len(dateCols) < 3 {
		return PinkSheetLoadResult{}, false
	}

	var res PinkSheetLoadResult
	res.Meta.SheetName = sheet
	res.Meta.Layout = "row-oriented"
	mappedCodes := map[string]struct{}{}
	var skipped []string

	for rowIdx, row := range rows {
		if rowIdx == dateRow || isBlankRow(row) {
			continue
		}
		label := firstNonEmptyCell(row)
		if label == "" {
			continue
		}
		mapped, ok := MapPinkSheetSeries(label)
		if !ok {
			if rowIdx < pinkSheetScanRows {
				skipped = append(skipped, label)
			}
			continue
		}
		mappedCodes[mapped.Code] = struct{}{}
		res.TotalRows++

		for _, dc := range dateCols {
			if dc.index >= len(row) {
				continue
			}
			price, ok := parsePinkSheetPrice(cellAt(row, dc.index))
			if !ok {
				continue
			}
			res.Records = append(res.Records, PriceRecord{
				Date:          dc.month,
				CommodityCode: mapped.Code,
				CommodityName: mapped.Name,
				PriceUSD:      price,
				Unit:          "USD",
				Source:        PinkSheetSourceName,
			})
		}
	}

	if len(res.Records) == 0 {
		return PinkSheetLoadResult{}, false
	}
	res.Meta.MappedSeries = len(mappedCodes)
	res.Meta.SkippedSeries = skipped
	return res, true
}

func findCommodityHeaderRow(rows [][]string) (int, []pinkSheetColumn, []string) {
	bestRow := -1
	var bestCols []pinkSheetColumn
	var bestSkipped []string
	bestCount := 0

	limit := pinkSheetScanRows
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		cols, skipped := buildPinkSheetColumnsFromRow(rows[i])
		if len(cols) > bestCount {
			bestCount = len(cols)
			bestRow = i
			bestCols = cols
			bestSkipped = skipped
		}
	}
	return bestRow, bestCols, bestSkipped
}

func buildPinkSheetColumnsFromRow(row []string) ([]pinkSheetColumn, []string) {
	bestByCode := map[string]pinkSheetColumn{}
	var skipped []string

	for i, raw := range row {
		name := strings.TrimSpace(raw)
		if name == "" || looksLikeUnitCell(name) {
			continue
		}
		if _, isDate := parsePinkSheetMonth(name); isDate {
			continue
		}
		mapped, ok := MapPinkSheetSeries(name)
		if !ok {
			skipped = append(skipped, name)
			continue
		}
		col := pinkSheetColumn{index: i, source: name, commodity: mapped}
		prev, exists := bestByCode[mapped.Code]
		if !exists || mapped.Priority > prev.commodity.Priority {
			bestByCode[mapped.Code] = col
		}
	}

	out := make([]pinkSheetColumn, 0, len(bestByCode))
	for _, col := range bestByCode {
		out = append(out, col)
	}
	return out, skipped
}

func findDateColumnDataStart(rows [][]string, headerRow int) int {
	for i := headerRow + 1; i < len(rows); i++ {
		if looksLikeUnitRow(rows[i]) {
			continue
		}
		if month, ok := parsePinkSheetMonth(cellAt(rows[i], 0)); ok && month != "" {
			return i
		}
	}
	return -1
}

func findDateHeaderRow(rows [][]string) (int, []pinkSheetDateCol) {
	bestRow := -1
	var bestCols []pinkSheetDateCol
	bestCount := 0

	limit := pinkSheetScanRows
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		cols := detectDateColumns(rows[i])
		if len(cols) > bestCount {
			bestCount = len(cols)
			bestRow = i
			bestCols = cols
		}
	}
	return bestRow, bestCols
}

func detectDateColumns(row []string) []pinkSheetDateCol {
	var out []pinkSheetDateCol
	for i, raw := range row {
		month, ok := parsePinkSheetMonth(raw)
		if !ok {
			continue
		}
		out = append(out, pinkSheetDateCol{index: i, month: month, raw: strings.TrimSpace(raw)})
	}
	return out
}

func looksLikeUnitRow(row []string) bool {
	for _, c := range row {
		if looksLikeUnitCell(c) {
			return true
		}
	}
	return false
}

func looksLikeUnitCell(raw string) bool {
	s := strings.TrimSpace(raw)
	return strings.HasPrefix(s, "($") || strings.HasPrefix(s, "(") && strings.Contains(s, "/")
}

func parsePinkSheetMonth(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}

	upper := strings.ToUpper(s)
	if m := pinkSheetMonthRE.FindStringSubmatch(upper); len(m) == 3 {
		month, err := strconv.Atoi(m[2])
		if err != nil || month < 1 || month > 12 {
			return "", false
		}
		return fmt.Sprintf("%s-%02d", m[1], month), true
	}

	if m := pinkSheetYYYYMMRE.FindStringSubmatch(s); len(m) == 3 {
		month, err := strconv.Atoi(m[2])
		if err != nil || month < 1 || month > 12 {
			return "", false
		}
		return fmt.Sprintf("%s-%02d", m[1], month), true
	}

	if m := pinkSheetMonYearRE.FindStringSubmatch(s); len(m) == 3 {
		mon := strings.ToLower(m[1])
		if len(mon) > 0 {
			mon = strings.ToUpper(mon[:1]) + mon[1:]
		}
		t, err := time.Parse("Jan-2006", mon+"-"+m[2])
		if err != nil {
			return "", false
		}
		return t.Format("2006-01"), true
	}

	if t, ok := normalizeDate(s); ok {
		return t, true
	}

	// Excel serial date as string or number-like cell.
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 20000 && f < 60000 {
		// Excel epoch 1899-12-30
		t := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(f))
		return t.Format("2006-01"), true
	}

	return "", false
}

func parsePinkSheetPrice(raw string) (float64, bool) {
	s := strings.TrimSpace(raw)
	if s == "" || s == "…" || s == "..." || strings.EqualFold(s, "n/a") || strings.EqualFold(s, "na") {
		return 0, false
	}
	price, reason := parsePrice(s)
	return price, reason == ""
}

func firstNonEmptyCell(row []string) string {
	for _, c := range row {
		if s := strings.TrimSpace(c); s != "" {
			return s
		}
	}
	return ""
}

func cellAt(row []string, i int) string {
	if i < 0 || i >= len(row) {
		return ""
	}
	return row[i]
}

func mappedCodesFromRecords(recs []PriceRecord) map[string]struct{} {
	out := map[string]struct{}{}
	for _, r := range recs {
		out[r.CommodityCode] = struct{}{}
	}
	return out
}

func pinkSheetNoMapError(rows [][]string) error {
	labels := collectRowLabels(rows, 20)
	dates := collectDetectedDates(rows, 10)
	var b strings.Builder
	b.WriteString("no mapped commodity series found in Pink Sheet workbook\n")
	b.WriteString("  first row labels:\n")
	for _, l := range labels {
		b.WriteString("    - ")
		b.WriteString(l)
		b.WriteByte('\n')
	}
	b.WriteString("  detected date-like columns:\n")
	for _, d := range dates {
		b.WriteString("    - ")
		b.WriteString(d)
		b.WriteByte('\n')
	}
	return fmt.Errorf(b.String())
}

func collectRowLabels(rows [][]string, limit int) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, row := range rows {
		label := firstNonEmptyCell(row)
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		out = append(out, label)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func collectDetectedDates(rows [][]string, limit int) []string {
	var out []string
	seen := map[string]struct{}{}
	scan := pinkSheetScanRows
	if len(rows) < scan {
		scan = len(rows)
	}
	for i := 0; i < scan; i++ {
		for _, raw := range rows[i] {
			month, ok := parsePinkSheetMonth(raw)
			if !ok {
				continue
			}
			key := fmt.Sprintf("%s (raw %q, row %d)", month, strings.TrimSpace(raw), i+1)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, key)
			if len(out) >= limit {
				return out
			}
		}
	}
	return out
}

func findMonthlyPricesSheet(f *excelize.File) string {
	for _, name := range f.GetSheetList() {
		if strings.EqualFold(strings.TrimSpace(name), pinkSheetMonthlySheet) {
			return name
		}
	}
	if sheets := f.GetSheetList(); len(sheets) > 0 {
		for _, name := range sheets {
			if !strings.EqualFold(name, "AFOSHEET") && !strings.EqualFold(name, "Description") {
				return name
			}
		}
	}
	return ""
}

// DetectSource guesses the ingest source from a file path.
func DetectSource(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".xlsx"), strings.HasSuffix(lower, ".xls"):
		return "worldbank-pinksheet"
	default:
		return "csv"
	}
}

// IngestFromFile loads CSV or Pink Sheet XLSX based on source flag or extension.
func IngestFromFile(path, source string) (LoadResult, string, PinkSheetMeta, error) {
	if strings.TrimSpace(source) == "" {
		source = DetectSource(path)
	}
	switch source {
	case "worldbank-pinksheet", "pinksheet", "pink-sheet":
		res, err := LoadPinkSheetFile(path)
		return res.LoadResult, PinkSheetSourceName, res.Meta, err
	case "csv", "":
		res, err := LoadFile(path)
		return res, SourceName, PinkSheetMeta{}, err
	default:
		return LoadResult{}, "", PinkSheetMeta{}, fmt.Errorf("unknown commodity price source %q (want csv or worldbank-pinksheet)", source)
	}
}

// WritePinkSheetFixture creates a minimal column-oriented Pink Sheet-style XLSX for tests.
func WritePinkSheetFixture(path string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheet := pinkSheetMonthlySheet
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return err
	}

	// Match real Pink Sheet layout: metadata rows, commodity header row 5, data from row 7.
	headers := []string{"", "Crude oil, average", "Natural gas, Europe", "Copper", "Wheat, US HRW", "Maize"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 5)
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return err
		}
	}
	unitRow := []string{"", "($/bbl)", "($/mmbtu)", "($/mt)", "($/mt)", "($/mt)"}
	for i, u := range unitRow {
		cell, _ := excelize.CoordinatesToCellName(i+1, 6)
		_ = f.SetCellValue(sheet, cell, u)
	}

	rows := [][2]string{
		{"2023M01", "80.4"},
		{"2023M02", "82.1"},
		{"2023M03", "78.6"},
		{"2023M04", "84.0"},
	}
	for r, row := range rows {
		dateCell, _ := excelize.CoordinatesToCellName(1, 7+r)
		if err := f.SetCellValue(sheet, dateCell, row[0]); err != nil {
			return err
		}
		priceCell, _ := excelize.CoordinatesToCellName(2, 7+r)
		if err := f.SetCellValue(sheet, priceCell, row[1]); err != nil {
			return err
		}
		gasCell, _ := excelize.CoordinatesToCellName(3, 7+r)
		_ = f.SetCellValue(sheet, gasCell, 4.5+float64(r)*0.1)
		copperCell, _ := excelize.CoordinatesToCellName(4, 7+r)
		_ = f.SetCellValue(sheet, copperCell, 8500+float64(r)*10)
		wheatCell, _ := excelize.CoordinatesToCellName(5, 7+r)
		_ = f.SetCellValue(sheet, wheatCell, 300+float64(r))
		maizeCell, _ := excelize.CoordinatesToCellName(6, 7+r)
		_ = f.SetCellValue(sheet, maizeCell, 250+float64(r))
	}

	return f.SaveAs(path)
}

// WritePinkSheetRowFixture creates a row-oriented Pink Sheet-style XLSX for tests.
func WritePinkSheetRowFixture(path string) error {
	f := excelize.NewFile()
	defer f.Close()
	sheet := pinkSheetMonthlySheet
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return err
	}
	dates := []string{"", "2023M01", "2023M02", "2023M03", "2023M04"}
	for i, d := range dates {
		cell, _ := excelize.CoordinatesToCellName(i+1, 3)
		_ = f.SetCellValue(sheet, cell, d)
	}
	series := [][]string{
		{"Crude oil, average", "", "80.4", "82.1", "78.6", "84.0"},
		{"Copper", "", "8500", "8510", "8520", "8530"},
	}
	for r, row := range series {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, 4+r)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
	return f.SaveAs(path)
}

// RealPinkSheetPath is the conventional local path for the downloaded workbook.
func RealPinkSheetPath() string {
	return filepath.Join("data", "raw", "worldbank_pinksheet", "CMO-Historical-Data-Monthly.xlsx")
}
