// Package tradegraph converts normalised trade-flow records (produced by the
// trade ingestion pipeline) into an AtlasGraph-compatible dataset: entities,
// dependencies and scenario presets. The generated JSON uses exactly the wire
// shapes the data loader accepts, so the standard graph/shock commands work
// against the output unchanged.
//
// The conversion is intentionally transparent and rule-based:
//
//   - exporter country --exports--> commodity, weighted by the exporter's share
//     of that commodity's total export value in the dataset (supplier importance);
//   - commodity --imports--> importer country, weighted by the importer's
//     top-supplier share and carrying the sourcing HHI as concentration
//     (so the supplier-dependency signal is preserved);
//   - importer country --industry_dependency--> sector, for the sectors mapped
//     to the commodity, weighted by a coarse default dependency.
//
// No external data or APIs are involved; everything is derived from the trade
// records already on disk.
package tradegraph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/atlasgraph/atlas/internal/ingest/trade"
)

// Shock types every generated trade edge is willing to carry. Whether a shock
// actually traverses an edge is still governed by the simulation profile; this
// list only widens the per-edge restriction.
var tradeShockTypes = []string{"export_collapse", "supply_cut", "price_spike"}

// --- wire formats (must match internal/data's loader exactly) --------------

// Entity is one named graph entity.
type Entity struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// EntitiesFile is the entities.json wire shape.
type EntitiesFile struct {
	Countries   []Entity `json:"countries"`
	Commodities []Entity `json:"commodities"`
	Sectors     []Entity `json:"sectors"`
	Routes      []Entity `json:"routes"`
	Companies   []Entity `json:"companies"`
}

// Dependency is one directed, weighted dependency edge.
type Dependency struct {
	Source            string   `json:"source"`
	Target            string   `json:"target"`
	Relationship      string   `json:"relationship_type"`
	Weight            float64  `json:"weight"`
	Concentration     *float64 `json:"concentration,omitempty"`
	Commodity         string   `json:"commodity,omitempty"`
	Sector            string   `json:"sector,omitempty"`
	AllowedShockTypes []string `json:"allowed_shock_types,omitempty"`
	Description       string   `json:"description,omitempty"`
}

// DependenciesFile is the dependencies.json wire shape.
type DependenciesFile struct {
	Dependencies []Dependency `json:"dependencies"`
}

// Scenario mirrors data.Scenario's JSON exactly.
type Scenario struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	Source       string  `json:"source"`
	Commodity    string  `json:"commodity"`
	ShockType    string  `json:"shock_type"`
	ShockPercent float64 `json:"shock_percent"`
	Depth        int     `json:"depth"`
	Description  string  `json:"description"`
}

// ScenariosFile is the scenarios.json wire shape.
type ScenariosFile struct {
	Scenarios []Scenario `json:"scenarios"`
}

// --- build result ----------------------------------------------------------

// Result is the generated dataset plus a few headline statistics for reporting.
type Result struct {
	Entities     EntitiesFile
	Dependencies DependenciesFile
	Scenarios    ScenariosFile

	TopDependency      *Dependency        // highest-weight generated edge
	HighestImportConc  *ImportConcentration // most concentrated importer+commodity
}

// ImportConcentration summarises the most concentrated import dependency found.
type ImportConcentration struct {
	Importer      string
	Commodity     string
	HHI           float64
	TopSupplier   string
	TopSupplierSh float64 // 0..1
}

// Counts of each entity/edge category in the result.
func (r Result) CountCountries() int   { return len(r.Entities.Countries) }
func (r Result) CountCommodities() int { return len(r.Entities.Commodities) }
func (r Result) CountSectors() int     { return len(r.Entities.Sectors) }
func (r Result) CountDependencies() int { return len(r.Dependencies.Dependencies) }
func (r Result) CountScenarios() int   { return len(r.Scenarios.Scenarios) }

// Build converts a trade file into a generated AtlasGraph dataset.
func Build(file trade.TradeFile) Result {
	b := newBuilder(file)
	b.collectEntities()
	b.buildExportEdges()
	b.buildImportEdges()
	b.buildIndustryEdges()
	b.buildScenarios()
	return b.finish()
}

// --- internal builder ------------------------------------------------------

type builder struct {
	file trade.TradeFile

	countries   map[string]string // code -> display name
	commodities map[string]struct{}
	sectors     map[string]struct{}

	// exporter share inputs
	commodityTotal   map[string]float64            // commodity -> total export value
	exporterByComm   map[string]map[string]float64 // commodity -> exporterCode -> value
	exporterName     map[string]string             // exporterCode -> name

	deps       []Dependency
	scenarios  []Scenario
	highestConc *ImportConcentration
}

func newBuilder(file trade.TradeFile) *builder {
	return &builder{
		file:           file,
		countries:      map[string]string{},
		commodities:    map[string]struct{}{},
		sectors:        map[string]struct{}{},
		commodityTotal: map[string]float64{},
		exporterByComm: map[string]map[string]float64{},
		exporterName:   map[string]string{},
	}
}

func (b *builder) collectEntities() {
	for _, r := range b.file.Records {
		b.countries[r.ExporterCode] = displayName(r.ExporterName, r.ExporterCode)
		b.countries[r.ImporterCode] = displayName(r.ImporterName, r.ImporterCode)
		b.commodities[r.CommodityName] = struct{}{}
		for _, sd := range sectorsFor(r.CommodityName) {
			b.sectors[sd.Sector] = struct{}{}
		}

		b.commodityTotal[r.CommodityName] += r.TradeValueUSD
		if b.exporterByComm[r.CommodityName] == nil {
			b.exporterByComm[r.CommodityName] = map[string]float64{}
		}
		b.exporterByComm[r.CommodityName][r.ExporterCode] += r.TradeValueUSD
		b.exporterName[r.ExporterCode] = displayName(r.ExporterName, r.ExporterCode)
	}
}

// buildExportEdges: exporter country --exports--> commodity, weighted by the
// exporter's share of that commodity's total export value.
func (b *builder) buildExportEdges() {
	for commodity, byExporter := range b.exporterByComm {
		total := b.commodityTotal[commodity]
		for code, val := range byExporter {
			if val <= 0 || total <= 0 {
				continue
			}
			share := val / total
			w := roundWeight(share)
			name := b.exporterName[code]
			conc := w
			b.deps = append(b.deps, Dependency{
				Source:            name,
				Target:            commodity,
				Relationship:      "exports",
				Weight:            w,
				Concentration:     &conc,
				Commodity:         commodity,
				AllowedShockTypes: tradeShockTypes,
				Description: fmt.Sprintf("%s supplies %.0f%% of %s trade value in this dataset.",
					name, share*100, commodity),
			})
		}
	}
}

// buildImportEdges: commodity --imports--> importer country, weighted by the
// importer's top-supplier share, carrying the sourcing HHI as concentration.
func (b *builder) buildImportEdges() {
	for _, key := range b.importerCommodityPairs() {
		con := trade.BuildConcentration(b.file, key.importerCode, key.commodity)
		if !con.HasData {
			continue
		}
		w := roundWeight(con.TopSupplier.Share)
		conc := clamp01(round4(con.HHI))
		importerName := displayName(con.ImporterName, key.importerCode)
		b.deps = append(b.deps, Dependency{
			Source:            key.commodity,
			Target:            importerName,
			Relationship:      "imports",
			Weight:            w,
			Concentration:     &conc,
			Commodity:         key.commodity,
			AllowedShockTypes: tradeShockTypes,
			Description: fmt.Sprintf("%s sources %s with HHI %.2f (top supplier %s %.1f%%).",
				importerName, key.commodity, con.HHI, con.TopSupplier.ExporterName, con.TopSupplier.Share*100),
		})

		// Track the most concentrated import dependency for reporting.
		if b.highestConc == nil || con.HHI > b.highestConc.HHI {
			b.highestConc = &ImportConcentration{
				Importer:      importerName,
				Commodity:     key.commodity,
				HHI:           con.HHI,
				TopSupplier:   con.TopSupplier.ExporterName,
				TopSupplierSh: con.TopSupplier.Share,
			}
		}
	}
}

// buildIndustryEdges: importer country --industry_dependency--> sector, for the
// sectors mapped to each commodity the importer buys.
func (b *builder) buildIndustryEdges() {
	for _, key := range b.importerCommodityPairs() {
		sectors := sectorsFor(key.commodity)
		if len(sectors) == 0 {
			continue
		}
		importerName := b.countries[key.importerCode]
		for _, sd := range sectors {
			w := roundWeight(sd.Weight)
			conc := w
			b.deps = append(b.deps, Dependency{
				Source:            importerName,
				Target:            sd.Sector,
				Relationship:      "industry_dependency",
				Weight:            w,
				Concentration:     &conc,
				Commodity:         key.commodity,
				Sector:            sd.Sector,
				AllowedShockTypes: tradeShockTypes,
				Description: fmt.Sprintf("%s industry depends on %s via %s.",
					importerName, key.commodity, sd.Sector),
			})
		}
	}
}

// buildScenarios emits generated presets when their trigger flow is present.
func (b *builder) buildScenarios() {
	candidates := []struct {
		id, name, exporter, commodity, shockType, desc string
		percent                                        float64
	}{
		{
			id: "taiwan_semiconductor_shock", name: "Taiwan Semiconductor Supply Shock",
			exporter: "Taiwan", commodity: "semiconductors", shockType: "export_collapse", percent: 30,
			desc: "A 30% drop in Taiwan's semiconductor exports, cascading through importers and chip-dependent sectors.",
		},
		{
			id: "lithium_battery_shock", name: "Lithium Battery Supply Shock",
			exporter: "China", commodity: "lithium batteries", shockType: "supply_cut", percent: 35,
			desc: "A 35% cut in China's lithium battery exports and its impact on EV and automotive supply chains.",
		},
		{
			id: "crude_oil_supply_shock", name: "Crude Oil Supply Shock",
			exporter: "Saudi Arabia", commodity: "crude oil", shockType: "supply_cut", percent: 25,
			desc: "A 25% cut in Saudi crude oil exports and the cascade into logistics and energy-intensive industry.",
		},
	}
	for _, c := range candidates {
		if !b.exporterSupplies(c.exporter, c.commodity) {
			continue
		}
		b.scenarios = append(b.scenarios, Scenario{
			ID: c.id, Name: c.name, Source: b.canonicalExporter(c.exporter),
			Commodity: c.commodity, ShockType: c.shockType, ShockPercent: c.percent,
			Depth: 3, Description: c.desc,
		})
	}
}

func (b *builder) finish() Result {
	r := Result{
		Entities: EntitiesFile{
			Countries:   sortedEntities(b.countries),
			Commodities: sortedNameSet(b.commodities),
			Sectors:     sortedNameSet(b.sectors),
			Routes:      []Entity{},
			Companies:   []Entity{},
		},
		HighestImportConc: b.highestConc,
	}

	sort.SliceStable(b.deps, func(i, j int) bool {
		a, c := b.deps[i], b.deps[j]
		if a.Source != c.Source {
			return a.Source < c.Source
		}
		if a.Relationship != c.Relationship {
			return a.Relationship < c.Relationship
		}
		if a.Target != c.Target {
			return a.Target < c.Target
		}
		return a.Commodity < c.Commodity
	})
	r.Dependencies = DependenciesFile{Dependencies: b.deps}

	sort.SliceStable(b.scenarios, func(i, j int) bool { return b.scenarios[i].ID < b.scenarios[j].ID })
	r.Scenarios = ScenariosFile{Scenarios: b.scenarios}

	// Highest-weight dependency, deterministic on ties.
	for i := range b.deps {
		d := b.deps[i]
		if r.TopDependency == nil || d.Weight > r.TopDependency.Weight {
			cp := d
			r.TopDependency = &cp
		}
	}
	return r
}

// --- helpers ---------------------------------------------------------------

type importerCommodity struct {
	importerCode string
	commodity    string
}

// importerCommodityPairs returns the unique (importer, commodity) pairs present,
// sorted for deterministic output.
func (b *builder) importerCommodityPairs() []importerCommodity {
	seen := map[importerCommodity]struct{}{}
	var out []importerCommodity
	for _, r := range b.file.Records {
		k := importerCommodity{importerCode: r.ImporterCode, commodity: r.CommodityName}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].importerCode != out[j].importerCode {
			return out[i].importerCode < out[j].importerCode
		}
		return out[i].commodity < out[j].commodity
	})
	return out
}

func (b *builder) exporterSupplies(exporter, commodity string) bool {
	for _, r := range b.file.Records {
		if !strings.EqualFold(r.CommodityName, commodity) {
			continue
		}
		if strings.EqualFold(r.ExporterName, exporter) || strings.EqualFold(r.ExporterCode, exporter) {
			return true
		}
	}
	return false
}

// canonicalExporter returns the dataset's display name for an exporter so the
// scenario's source matches a generated country entity exactly.
func (b *builder) canonicalExporter(exporter string) string {
	for _, r := range b.file.Records {
		if strings.EqualFold(r.ExporterName, exporter) || strings.EqualFold(r.ExporterCode, exporter) {
			return displayName(r.ExporterName, r.ExporterCode)
		}
	}
	return exporter
}

func displayName(name, code string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return code
}

func sortedEntities(m map[string]string) []Entity {
	out := make([]Entity, 0, len(m))
	for _, name := range m {
		out = append(out, Entity{Name: name})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedNameSet(m map[string]struct{}) []Entity {
	out := make([]Entity, 0, len(m))
	for name := range m {
		out = append(out, Entity{Name: name})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// roundWeight rounds to 4 decimals and keeps the result within (0,1] so it
// always satisfies the loader's weight validation.
func roundWeight(v float64) float64 {
	v = round4(v)
	if v <= 0 {
		return 0.0001
	}
	if v > 1 {
		return 1
	}
	return v
}

func round4(v float64) float64 {
	return float64(int64(v*1e4+0.5)) / 1e4
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
