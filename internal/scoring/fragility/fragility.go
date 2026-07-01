// Package fragility combines existing GFIP signals — macro exposure, event risk,
// trade concentration, commodity stress, shock propagation and graph structure —
// into explainable unified country- and commodity-level fragility scores.
//
// This is a composite risk score, not a forecast: it summarises how structurally
// exposed an entity is today given the signals AtlasGraph already computes.
package fragility

import (
	"sort"
	"strings"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/scoring/commodities"
	"github.com/atlasgraph/atlas/internal/scoring/events"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
	"github.com/atlasgraph/atlas/internal/simulation"
)

const defaultScenarioID = "taiwan_semiconductor_shock"

// CountryWeights controls how country-level components blend. They sum to 1.0.
type CountryWeights struct {
	MacroExposure      float64 `json:"macro_exposure"`
	EventRisk          float64 `json:"event_risk"`
	TradeConcentration float64 `json:"trade_concentration"`
	ShockExposure      float64 `json:"shock_exposure"`
}

// DefaultCountryWeights is the calibrated country blend used by the CLI and API.
func DefaultCountryWeights() CountryWeights {
	return CountryWeights{
		MacroExposure:      0.30,
		EventRisk:          0.25,
		TradeConcentration: 0.25,
		ShockExposure:      0.20,
	}
}

// CommodityWeights controls how commodity-level components blend. They sum to 1.0.
type CommodityWeights struct {
	CommodityStress       float64 `json:"commodity_stress"`
	SupplierConcentration float64 `json:"supplier_concentration"`
	EventExposure         float64 `json:"event_exposure"`
	GraphCentrality       float64 `json:"graph_centrality"`
}

// DefaultCommodityWeights is the calibrated commodity blend used by the CLI and API.
func DefaultCommodityWeights() CommodityWeights {
	return CommodityWeights{
		CommodityStress:       0.35,
		SupplierConcentration: 0.30,
		EventExposure:         0.20,
		GraphCentrality:       0.15,
	}
}

// Component is one contributor to a unified fragility score.
type Component struct {
	Key          string  `json:"key"`
	Name         string  `json:"name"`
	Score        float64 `json:"score"`
	Weight       float64 `json:"weight"`
	Contribution float64 `json:"contribution"`
	Available    bool    `json:"available"`
}

// CountryScore is the explainable unified fragility result for one country.
type CountryScore struct {
	CountryCode       string      `json:"country_code"`
	CountryName       string      `json:"country_name"`
	Score             float64     `json:"score"`
	RiskLevel         string      `json:"risk_level"`
	Components        []Component `json:"components"`
	MissingComponents []string    `json:"missing_components"`
	TopDrivers        []string    `json:"top_drivers"`
}

// CommodityScore is the explainable unified fragility result for one commodity.
type CommodityScore struct {
	CommodityCode     string      `json:"commodity_code"`
	CommodityName     string      `json:"commodity_name"`
	Score             float64     `json:"score"`
	RiskLevel         string      `json:"risk_level"`
	Components        []Component `json:"components"`
	MissingComponents []string    `json:"missing_components"`
	TopDrivers        []string    `json:"top_drivers"`
}

// Result holds both country and commodity unified fragility rankings.
type Result struct {
	Countries  []CountryScore  `json:"countries"`
	Commodities []CommodityScore `json:"commodities"`
}

// Sources are the optional upstream datasets the scorer may draw from. Nil
// pointers are tolerated; missing inputs mark the affected components as
// unavailable and the blend renormalises over what remains.
type Sources struct {
	Graph       *graph.Graph
	Scenarios   []data.Scenario
	Trade       *trade.TradeFile
	Macro       *worldbank.IndicatorFile
	Events      *gdelt.EventFile
	Commodities *commodityprices.PriceFile
	Config      config.Config
}

// Score computes unified country and commodity fragility from the provided
// sources. Either side may be empty when its inputs are absent.
func Score(src Sources) Result {
	cw := DefaultCountryWeights()
	kw := DefaultCommodityWeights()

	macroByKey := indexMacro(src.Macro)
	eventByKey := indexEvents(src.Events)
	tradeConcByKey := tradeConcentrationByImporter(src.Trade)
	shockByName, shockOK := shockExposureByCountry(src.Graph, src.Scenarios, src.Config)

	commodityStress := indexCommodityStress(src.Commodities)
	supplierConcByKey := supplierConcentrationByCommodity(src.Trade)
	eventExpByKey := eventExposureByCommodity(src.Trade, eventByKey)
	centralityByName := commodityCentrality(src.Graph)

	countries := scoreCountries(src, cw, macroByKey, eventByKey, tradeConcByKey, shockByName, shockOK)
	commodities := scoreCommodities(kw, commodityStress, supplierConcByKey, eventExpByKey, centralityByName, src.Graph)

	return Result{Countries: countries, Commodities: commodities}
}

func scoreCountries(
	src Sources,
	w CountryWeights,
	macroByKey map[string]macro.CountryScore,
	eventByKey map[string]events.CountryScore,
	tradeConcByKey map[string]float64,
	shockByName map[string]float64,
	shockOK bool,
) []CountryScore {
	seen := map[string]countryRef{}
	add := func(code, name string) {
		code = strings.ToUpper(strings.TrimSpace(code))
		name = strings.TrimSpace(name)
		key := countryKey(code, name)
		if key == "" {
			return
		}
		seen[key] = mergeCountryRef(seen[key], countryRef{code: code, name: name})
	}

	if src.Graph != nil {
		for _, n := range src.Graph.Nodes() {
			if n.Type == models.Country {
				add("", n.Name)
			}
		}
	}
	for k, s := range macroByKey {
		add(k, s.CountryName)
	}
	for k, s := range eventByKey {
		add(k, s.CountryName)
	}
	for k, v := range tradeConcByKey {
		add(k, importerName(src.Trade, k))
		_ = v
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]CountryScore, 0, len(keys))
	for _, key := range keys {
		ref := seen[key]
		out = append(out, scoreCountry(ref, w, macroByKey, eventByKey, tradeConcByKey, shockByName, shockOK))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].CountryName < out[j].CountryName
	})
	return out
}

type countryRef struct {
	code string
	name string
}

func scoreCountry(
	ref countryRef,
	w CountryWeights,
	macroByKey map[string]macro.CountryScore,
	eventByKey map[string]events.CountryScore,
	tradeConcByKey map[string]float64,
	shockByName map[string]float64,
	shockOK bool,
) CountryScore {
	ref = mergeCountryRef(countryRef{}, ref)
	code, name := ref.code, ref.name
	if name == "" {
		name = code
	}
	if code == "" {
		code = guessCode(name, macroByKey, eventByKey)
	}
	if name == "" && code != "" {
		if n, ok := gdelt.CountryName(code); ok {
			name = n
		}
	}

	var comps []Component
	var missing []string

	macroScore, macroAvail := lookupMacro(ref, macroByKey)
	comps = append(comps, makeComponent("macro_exposure_score", "macro exposure", w.MacroExposure, macroScore, macroAvail))
	if !macroAvail {
		missing = append(missing, "macro_exposure_score")
	}

	eventScore, eventAvail := lookupEvent(ref, eventByKey)
	comps = append(comps, makeComponent("event_risk_score", "event risk", w.EventRisk, eventScore, eventAvail))
	if !eventAvail {
		missing = append(missing, "event_risk_score")
	}

	tradeScore, tradeAvail := lookupTradeConcentration(ref, tradeConcByKey)
	comps = append(comps, makeComponent("trade_concentration_score", "trade concentration", w.TradeConcentration, tradeScore, tradeAvail))
	if !tradeAvail {
		missing = append(missing, "trade_concentration_score")
	}

	shockScore, shockAvail := 0.0, false
	if shockOK {
		shockScore = lookupShockExposure(ref, shockByName)
		shockAvail = true
	}
	comps = append(comps, makeComponent("shock_exposure_score", "shock exposure", w.ShockExposure, shockScore, shockAvail))
	if !shockAvail {
		missing = append(missing, "shock_exposure_score")
	}

	final := blend(comps)
	sort.Strings(missing)

	return CountryScore{
		CountryCode:       code,
		CountryName:       name,
		Score:             final,
		RiskLevel:         RiskLevel(final),
		Components:        comps,
		MissingComponents: missing,
		TopDrivers:        topDrivers(comps, 2),
	}
}

func scoreCommodities(
	w CommodityWeights,
	stressByKey map[string]commodities.CommodityScore,
	supplierConcByKey map[string]float64,
	eventExpByKey map[string]float64,
	centralityByName map[string]float64,
	g *graph.Graph,
) []CommodityScore {
	seen := map[string]commodityRef{}
	add := func(code, name string) {
		code = strings.TrimSpace(code)
		name = strings.TrimSpace(name)
		key := commodityKey(code, name)
		if key == "" {
			return
		}
		seen[key] = mergeCommodityRef(seen[key], commodityRef{code: code, name: name})
	}

	if g != nil {
		for _, n := range g.Nodes() {
			if n.Type == models.Commodity {
				add("", n.Name)
			}
		}
	}
	for k, s := range stressByKey {
		add(k, s.CommodityName)
	}
	for k := range supplierConcByKey {
		add(k, commodityNameFromKey(k, stressByKey))
	}
	for k := range eventExpByKey {
		add(k, commodityNameFromKey(k, stressByKey))
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]CommodityScore, 0, len(keys))
	for _, key := range keys {
		out = append(out, scoreCommodity(seen[key], w, stressByKey, supplierConcByKey, eventExpByKey, centralityByName))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].CommodityName < out[j].CommodityName
	})
	return out
}

type commodityRef struct {
	code string
	name string
}

func scoreCommodity(
	ref commodityRef,
	w CommodityWeights,
	stressByKey map[string]commodities.CommodityScore,
	supplierConcByKey map[string]float64,
	eventExpByKey map[string]float64,
	centralityByName map[string]float64,
) CommodityScore {
	ref = mergeCommodityRef(commodityRef{}, ref)
	code, name := ref.code, ref.name
	if name == "" {
		name = strings.ReplaceAll(code, "_", " ")
	}
	if code == "" {
		code = guessCommodityCode(name, stressByKey)
	}
	if code == "" && name != "" {
		code = commodityKey("", name)
	}

	var comps []Component
	var missing []string

	stressScore, stressAvail := lookupCommodityStress(ref, stressByKey)
	comps = append(comps, makeComponent("commodity_stress_score", "commodity stress", w.CommodityStress, stressScore, stressAvail))
	if !stressAvail {
		missing = append(missing, "commodity_stress_score")
	}

	supplierScore, supplierAvail := lookupSupplierConcentration(ref, supplierConcByKey)
	comps = append(comps, makeComponent("supplier_concentration_score", "supplier concentration", w.SupplierConcentration, supplierScore, supplierAvail))
	if !supplierAvail {
		missing = append(missing, "supplier_concentration_score")
	}

	eventScore, eventAvail := lookupEventExposure(ref, eventExpByKey)
	comps = append(comps, makeComponent("event_exposure_score", "event exposure", w.EventExposure, eventScore, eventAvail))
	if !eventAvail {
		missing = append(missing, "event_exposure_score")
	}

	centScore, centAvail := lookupCentrality(ref, centralityByName)
	comps = append(comps, makeComponent("graph_centrality_score", "graph centrality", w.GraphCentrality, centScore, centAvail))
	if !centAvail {
		missing = append(missing, "graph_centrality_score")
	}

	final := blend(comps)
	sort.Strings(missing)

	return CommodityScore{
		CommodityCode:     code,
		CommodityName:     name,
		Score:             final,
		RiskLevel:         RiskLevel(final),
		Components:        comps,
		MissingComponents: missing,
		TopDrivers:        topDrivers(comps, 2),
	}
}

func makeComponent(key, name string, weight, score float64, available bool) Component {
	c := Component{Key: key, Name: name, Weight: weight, Available: available}
	if available {
		c.Score = clampScore(score)
		c.Contribution = weight * c.Score
	}
	return c
}

// blend renormalises over available components so missing inputs do not
// artificially deflate the unified score.
func blend(comps []Component) float64 {
	num, den := 0.0, 0.0
	for _, c := range comps {
		if c.Available {
			num += c.Weight * c.Score
			den += c.Weight
		}
	}
	if den == 0 {
		return 0
	}
	return num / den
}

// RiskLevel maps a 0..100 unified fragility score to a qualitative band.
func RiskLevel(score float64) string {
	switch {
	case score < 30:
		return "Low"
	case score < 60:
		return "Medium"
	case score < 80:
		return "High"
	default:
		return "Critical"
	}
}

func topDrivers(comps []Component, n int) []string {
	avail := make([]Component, 0, len(comps))
	for _, c := range comps {
		if c.Available && c.Contribution > 0 {
			avail = append(avail, c)
		}
	}
	sort.SliceStable(avail, func(i, j int) bool {
		if avail[i].Contribution != avail[j].Contribution {
			return avail[i].Contribution > avail[j].Contribution
		}
		return avail[i].Name < avail[j].Name
	})
	if len(avail) > n {
		avail = avail[:n]
	}
	out := make([]string, len(avail))
	for i, c := range avail {
		out[i] = c.Name
	}
	return out
}

func clampScore(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func indexMacro(file *worldbank.IndicatorFile) map[string]macro.CountryScore {
	out := map[string]macro.CountryScore{}
	if file == nil {
		return out
	}
	for _, s := range macro.ScoreCountries(*file, 0, macro.DefaultWeights()) {
		out[strings.ToUpper(s.CountryCode)] = s
	}
	return out
}

func indexEvents(file *gdelt.EventFile) map[string]events.CountryScore {
	out := map[string]events.CountryScore{}
	if file == nil {
		return out
	}
	for _, s := range events.ScoreCountries(*file, events.DefaultWeights()) {
		out[strings.ToUpper(s.CountryCode)] = s
	}
	return out
}

func indexCommodityStress(file *commodityprices.PriceFile) map[string]commodities.CommodityScore {
	out := map[string]commodities.CommodityScore{}
	if file == nil {
		return out
	}
	for _, s := range commodities.ScoreCommodities(*file, commodities.DefaultWeights()) {
		out[commodityKey(s.CommodityCode, s.CommodityName)] = s
	}
	return out
}

func shockExposureByCountry(g *graph.Graph, scenarios []data.Scenario, cfg config.Config) (map[string]float64, bool) {
	if g == nil || len(scenarios) == 0 {
		return nil, false
	}
	sc, ok := pickDefaultScenario(scenarios)
	if !ok {
		return nil, false
	}
	res, err := simulation.Run(g, cfg, simulation.ShockRequest{
		Source:    sc.Source,
		Commodity: sc.Commodity,
		ShockType: sc.ShockType,
		DropPct:   sc.ShockPercent,
		Depth:     sc.Depth,
	})
	if err != nil {
		return nil, false
	}
	out := map[string]float64{}
	for _, ni := range res.AllAffected {
		if ni.Node.Type != models.Country {
			continue
		}
		score := ni.Impact * 100
		if score > out[ni.Node.Name] {
			out[ni.Node.Name] = score
		}
	}
	return out, true
}

func pickDefaultScenario(scenarios []data.Scenario) (data.Scenario, bool) {
	for _, s := range scenarios {
		if s.ID == defaultScenarioID {
			return s, true
		}
	}
	if len(scenarios) == 0 {
		return data.Scenario{}, false
	}
	return scenarios[0], true
}

func importerName(file *trade.TradeFile, code string) string {
	if file == nil {
		return code
	}
	for _, r := range file.Records {
		if strings.EqualFold(r.ImporterCode, code) && r.ImporterName != "" {
			return r.ImporterName
		}
	}
	return code
}

func commodityNameFromKey(key string, stress map[string]commodities.CommodityScore) string {
	if s, ok := stress[key]; ok && s.CommodityName != "" {
		return s.CommodityName
	}
	return key
}
