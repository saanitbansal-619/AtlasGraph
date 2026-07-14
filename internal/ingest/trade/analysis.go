package trade

import (
	"sort"
	"strings"
)

// NamedValue is a code/name pair with an aggregated trade value, used for the
// "top exporters / importers / commodities" leaderboards in a summary.
type NamedValue struct {
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Value float64 `json:"value_usd"`
}

// Summary is a high-level digest of an ingested trade panel.
type Summary struct {
	Records       int          `json:"records"`
	Years         []int        `json:"years"`
	Countries     int          `json:"countries"`
	Commodities   int          `json:"commodities"`
	TotalValueUSD float64      `json:"total_value_usd"`
	TopExporters  []NamedValue `json:"top_exporters"`
	TopImporters  []NamedValue `json:"top_importers"`
	TopCommodities []NamedValue `json:"top_commodities"`
	// AvailableCommodities lists every commodity name in the trade file, sorted
	// alphabetically. Unlike TopCommodities, this is not capped to a preview size.
	AvailableCommodities []string `json:"available_commodities"`
	// AvailableImporters lists every importer country name, sorted alphabetically.
	// Prefer import-reporter names when flow tags are present on dependency rows.
	AvailableImporters []string `json:"available_importers"`
}

// BuildSummary aggregates a TradeFile into a Summary, keeping the top n entries
// in each leaderboard.
func BuildSummary(file TradeFile, n int) Summary {
	s := Summary{Records: len(file.Records)}

	yearSet := map[int]struct{}{}
	countrySet := map[string]struct{}{}
	commoditySet := map[string]struct{}{}
	exporters := map[string]*NamedValue{}
	importers := map[string]*NamedValue{}
	commodities := map[string]*NamedValue{}

	add := func(m map[string]*NamedValue, code, name string, v float64) {
		key := code
		if key == "" {
			key = name
		}
		nv, ok := m[key]
		if !ok {
			nv = &NamedValue{Code: code, Name: name}
			m[key] = nv
		}
		if nv.Name == "" {
			nv.Name = name
		}
		nv.Value += v
	}

	for _, r := range file.Records {
		s.TotalValueUSD += r.TradeValueUSD
		yearSet[r.Year] = struct{}{}
		exporterCode := ResolveCountryCode(r.ExporterCode, r.ExporterName)
		importerCode := ResolveCountryCode(r.ImporterCode, r.ImporterName)
		exporterName := NormalizeCountryName(r.ExporterName)
		if exporterName == "" {
			exporterName = r.ExporterName
		}
		importerName := NormalizeCountryName(r.ImporterName)
		if importerName == "" {
			importerName = r.ImporterName
		}
		if exporterCode != "" {
			countrySet[exporterCode] = struct{}{}
		} else if exporterName != "" {
			countrySet[exporterName] = struct{}{}
		}
		if importerCode != "" {
			countrySet[importerCode] = struct{}{}
		} else if importerName != "" {
			countrySet[importerName] = struct{}{}
		}
		commoditySet[commodityKey(r)] = struct{}{}
		add(exporters, exporterCode, exporterName, r.TradeValueUSD)
		add(importers, importerCode, importerName, r.TradeValueUSD)
		add(commodities, r.CommodityCode, r.CommodityName, r.TradeValueUSD)
	}

	s.Countries = len(countrySet)
	s.Commodities = len(commoditySet)
	for y := range yearSet {
		s.Years = append(s.Years, y)
	}
	sort.Ints(s.Years)

	s.TopExporters = topNamed(exporters, n)
	s.TopImporters = topNamed(importers, n)
	s.TopCommodities = topNamed(commodities, n)
	s.AvailableCommodities = commodityNames(commodities)
	s.AvailableImporters = namedCountryList(importers)
	return s
}

// AvailableImportReporters returns sorted importer names from dependency rows that
// are suitable for importer-side concentration / supplier queries (import flows).
func AvailableImportReporters(df DependencyFile) []string {
	hasFlowTags := DependencyFileHasFlowTags(df)
	seen := map[string]string{}
	for _, d := range df.Dependencies {
		if !UseForImporterConcentration(d, hasFlowTags) {
			continue
		}
		name := NormalizeCountryName(strings.TrimSpace(d.Importer))
		if name == "" {
			name = strings.TrimSpace(d.Importer)
		}
		if name == "" {
			continue
		}
		seen[strings.ToLower(name)] = name
	}
	out := make([]string, 0, len(seen))
	for _, name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// ImportOption is one importer with the commodities it has import-side records for.
type ImportOption struct {
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Commodities []string `json:"commodities"`
}

// TradeOptions lists importer → commodity pairs available for supplier queries.
type TradeOptions struct {
	Importers []ImportOption `json:"importers"`
}

// BuildTradeOptions builds importer/commodity menus from processed trade data.
// Prefer import-flow dependency rows; fall back to TradeFile records.
func BuildTradeOptions(resolved ResolvedTrade) TradeOptions {
	if resolved.DependencyFile != nil && len(resolved.DependencyFile.Dependencies) > 0 {
		return tradeOptionsFromDeps(*resolved.DependencyFile)
	}
	return tradeOptionsFromFile(resolved.File)
}

func tradeOptionsFromDeps(df DependencyFile) TradeOptions {
	hasFlowTags := DependencyFileHasFlowTags(df)
	byKey := map[string]*importBucket{}
	for _, d := range df.Dependencies {
		if !UseForImporterConcentration(d, hasFlowTags) {
			continue
		}
		name := NormalizeCountryName(strings.TrimSpace(d.Importer))
		if name == "" {
			name = strings.TrimSpace(d.Importer)
		}
		com := strings.TrimSpace(d.Commodity)
		if name == "" || com == "" {
			continue
		}
		key := strings.ToLower(name)
		b, ok := byKey[key]
		if !ok {
			b = &importBucket{name: name, code: CountryCodeForName(name), coms: map[string]struct{}{}}
			byKey[key] = b
		}
		b.coms[com] = struct{}{}
	}
	return finalizeImportOptions(byKey)
}

func tradeOptionsFromFile(file TradeFile) TradeOptions {
	byKey := map[string]*importBucket{}
	for _, r := range file.Records {
		name := NormalizeCountryName(strings.TrimSpace(r.ImporterName))
		if name == "" {
			name = strings.TrimSpace(r.ImporterName)
		}
		code := ResolveCountryCode(r.ImporterCode, name)
		com := strings.TrimSpace(r.CommodityName)
		if com == "" {
			com = strings.TrimSpace(r.CommodityCode)
		}
		if name == "" && code == "" {
			continue
		}
		if name == "" {
			name = code
		}
		if com == "" {
			continue
		}
		key := strings.ToLower(name)
		if code != "" {
			key = strings.ToLower(code)
		}
		b, ok := byKey[key]
		if !ok {
			b = &importBucket{name: name, code: code, coms: map[string]struct{}{}}
			byKey[key] = b
		}
		if b.code == "" && code != "" {
			b.code = code
		}
		b.coms[com] = struct{}{}
	}
	return finalizeImportOptions(byKey)
}

type importBucket struct {
	name string
	code string
	coms map[string]struct{}
}

func finalizeImportOptions(byKey map[string]*importBucket) TradeOptions {
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := TradeOptions{Importers: make([]ImportOption, 0, len(keys))}
	for _, k := range keys {
		b := byKey[k]
		coms := make([]string, 0, len(b.coms))
		for c := range b.coms {
			coms = append(coms, c)
		}
		sort.Strings(coms)
		out.Importers = append(out.Importers, ImportOption{
			Name: b.name, Code: b.code, Commodities: coms,
		})
	}
	return out
}

// Supplier is one exporter's contribution to an importer's purchases of a
// commodity, with its share of the total and a per-supplier dependency band.
type Supplier struct {
	ExporterCode string  `json:"exporter_code"`
	ExporterName string  `json:"exporter_name"`
	ValueUSD     float64 `json:"value_usd"`
	Share        float64 `json:"share"` // 0..1
	Dependency   string  `json:"dependency"`
}

// Dependency is the supplier breakdown for one importer + commodity.
type Dependency struct {
	ImporterCode    string     `json:"importer_code"`
	ImporterName    string     `json:"importer_name"`
	Commodity       string     `json:"commodity"`
	TotalImportsUSD float64    `json:"total_imports_usd"`
	Suppliers       []Supplier `json:"suppliers"`
	HasData         bool       `json:"-"`
}

// Concentration is the HHI-based supplier concentration for one importer +
// commodity.
type Concentration struct {
	ImporterCode string   `json:"importer_code"`
	ImporterName string   `json:"importer_name"`
	Commodity    string   `json:"commodity"`
	HHI          float64  `json:"hhi"`
	RiskLevel    string   `json:"concentration_risk"`
	TopSupplier  Supplier `json:"top_supplier"`
	HasData      bool     `json:"-"`
}

// BuildDependency computes how an importer's purchases of a commodity are split
// across supplier countries. importer matches an ISO code or country name;
// commodity matches a commodity name or code (all case-insensitive). Records
// are aggregated across all years present in the file.
func BuildDependency(file TradeFile, importer, commodity string) Dependency {
	importer = strings.TrimSpace(importer)
	commodity = strings.TrimSpace(commodity)

	dep := Dependency{ImporterCode: strings.ToUpper(importer), Commodity: commodity}
	byExporter := map[string]*Supplier{}

	for _, r := range file.Records {
		if !matchImporter(r, importer) || !matchCommodity(r, commodity) {
			continue
		}
		dep.HasData = true
		if matchesCode(r.ImporterCode, importer) {
			dep.ImporterCode = r.ImporterCode
		}
		if dep.ImporterName == "" {
			dep.ImporterName = r.ImporterName
		}
		// Prefer the canonical commodity name from the data.
		dep.Commodity = r.CommodityName
		dep.TotalImportsUSD += r.TradeValueUSD

		key := exporterGroupKey(r.ExporterCode, r.ExporterName)
		sup, ok := byExporter[key]
		if !ok {
			sup = &Supplier{ExporterCode: r.ExporterCode, ExporterName: r.ExporterName}
			byExporter[key] = sup
		}
		if sup.ExporterName == "" {
			sup.ExporterName = r.ExporterName
		}
		sup.ValueUSD += r.TradeValueUSD
	}

	if !dep.HasData {
		return dep
	}

	for _, sup := range byExporter {
		if dep.TotalImportsUSD > 0 {
			sup.Share = sup.ValueUSD / dep.TotalImportsUSD
		}
		sup.Dependency = SupplierDependencyBand(sup.Share)
		dep.Suppliers = append(dep.Suppliers, *sup)
	}
	sort.SliceStable(dep.Suppliers, func(i, j int) bool {
		if dep.Suppliers[i].ValueUSD != dep.Suppliers[j].ValueUSD {
			return dep.Suppliers[i].ValueUSD > dep.Suppliers[j].ValueUSD
		}
		return dep.Suppliers[i].ExporterName < dep.Suppliers[j].ExporterName
	})
	return dep
}

// BuildConcentration derives the Herfindahl-Hirschman Index (sum of squared
// supplier shares) and a qualitative risk band from a Dependency.
func BuildConcentration(file TradeFile, importer, commodity string) Concentration {
	return concentrationFromDependency(BuildDependency(file, importer, commodity))
}

func concentrationFromDependency(dep Dependency) Concentration {
	con := Concentration{
		ImporterCode: dep.ImporterCode,
		ImporterName: dep.ImporterName,
		Commodity:    dep.Commodity,
		HasData:      dep.HasData,
	}
	if !dep.HasData {
		return con
	}
	for _, sup := range dep.Suppliers {
		con.HHI += sup.Share * sup.Share
	}
	con.RiskLevel = ConcentrationRisk(con.HHI)
	if len(dep.Suppliers) > 0 {
		con.TopSupplier = dep.Suppliers[0]
	}
	return con
}

// SupplierDependencyBand classifies a single supplier's share of an importer's
// purchases: Low (<10%), Medium (10–40%), High (>=40%).
func SupplierDependencyBand(share float64) string {
	switch {
	case share < 0.10:
		return "Low"
	case share < 0.40:
		return "Medium"
	default:
		return "High"
	}
}

// ConcentrationRisk maps an HHI to a qualitative band:
// Low (<0.15), Medium (0.15–0.25), High (>0.25).
func ConcentrationRisk(hhi float64) string {
	switch {
	case hhi < 0.15:
		return "Low"
	case hhi <= 0.25:
		return "Medium"
	default:
		return "High"
	}
}

// --- helpers ---------------------------------------------------------------

func commodityKey(r TradeFlowRecord) string {
	if r.CommodityCode != "" {
		return r.CommodityCode
	}
	return strings.ToLower(r.CommodityName)
}

func matchImporter(r TradeFlowRecord, q string) bool {
	q = strings.TrimSpace(q)
	if q == "" {
		return false
	}
	if matchesCode(r.ImporterCode, q) || strings.EqualFold(r.ImporterName, q) {
		return true
	}
	canonQ := NormalizeImporterQuery(q)
	canonR := NormalizeImporterQuery(r.ImporterName)
	if canonQ != "" && strings.EqualFold(canonQ, canonR) {
		return true
	}
	if strings.EqualFold(NormalizeCountryName(q), NormalizeCountryName(r.ImporterName)) {
		return true
	}
	codeR := ResolveCountryCode(r.ImporterCode, r.ImporterName)
	codeQ := strings.ToUpper(q)
	if len(codeQ) == 3 && codeR != "" && codeR == codeQ {
		return true
	}
	if resolved := CountryCodeForName(q); resolved != "" && codeR != "" && resolved == codeR {
		return true
	}
	return false
}

func matchCommodity(r TradeFlowRecord, q string) bool {
	return strings.EqualFold(r.CommodityName, q) || strings.EqualFold(r.CommodityCode, q)
}

func matchesCode(code, q string) bool {
	return strings.EqualFold(code, q)
}

// exporterGroupKey groups supplier rows by ISO code when present, otherwise by
// normalized exporter name. UN Comtrade dependency rows often omit codes.
func exporterGroupKey(code, name string) string {
	if c := strings.ToUpper(strings.TrimSpace(code)); c != "" {
		return "c:" + c
	}
	return "n:" + strings.ToLower(strings.TrimSpace(name))
}

func commodityNames(m map[string]*NamedValue) []string {
	return namedCountryList(m)
}

func namedCountryList(m map[string]*NamedValue) []string {
	out := make([]string, 0, len(m))
	seen := map[string]struct{}{}
	for _, nv := range m {
		name := strings.TrimSpace(nv.Name)
		if name == "" {
			name = strings.TrimSpace(nv.Code)
		}
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func topNamed(m map[string]*NamedValue, n int) []NamedValue {
	out := make([]NamedValue, 0, len(m))
	for _, nv := range m {
		out = append(out, *nv)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		return out[i].Name < out[j].Name
	})
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}
