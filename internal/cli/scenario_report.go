package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/graphfusion"
	"github.com/atlasgraph/atlas/internal/ingest/eventrisk"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/scoring/commodities"
	"github.com/atlasgraph/atlas/internal/scoring/events"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
	"github.com/atlasgraph/atlas/internal/simulation"
)

// scenarioReportRequest is the POST /api/reports/scenario body.
// drop_percent is preferred; drop is accepted for parity with /api/shock.
type scenarioReportRequest struct {
	Source      string   `json:"source"`
	Commodity   string   `json:"commodity"`
	ShockType   string   `json:"shock_type"`
	DropPercent *float64 `json:"drop_percent"`
	Drop        *float64 `json:"drop"`
	Depth       *int     `json:"depth"`
}

// scenarioReport is the structured analyst-style intelligence report.
type scenarioReport struct {
	Title                  string                 `json:"title"`
	ExecutiveSummary       string                 `json:"executive_summary"`
	KeyFindings            []string               `json:"key_findings"`
	DirectExposure         []reportExposureItem   `json:"direct_exposure"`
	SecondOrderExposure    []reportExposureItem   `json:"second_order_exposure"`
	MostExposedCountries   []reportExposureItem   `json:"most_exposed_countries"`
	MostExposedCommodities []reportExposureItem   `json:"most_exposed_commodities"`
	MostExposedSectors     []reportExposureItem   `json:"most_exposed_sectors"`
	TradeEvidence          []reportTradeEvidence  `json:"trade_evidence"`
	EventRiskContext       []reportContextItem    `json:"event_risk_context"`
	MacroContext           []reportContextItem    `json:"macro_context"`
	CommodityFragility     []reportContextItem    `json:"commodity_fragility_context"`
	ModelAssumptions       []string               `json:"model_assumptions"`
	DataSources            []string               `json:"data_sources"`
	Limitations            []string               `json:"limitations"`
}

type reportExposureItem struct {
	Entity           string  `json:"entity"`
	Type             string  `json:"type"`
	Distance         int     `json:"distance"`
	EstimatedImpact  float64 `json:"estimated_impact"`
	FragilityDelta   float64 `json:"fragility_delta"`
	BaseFragility    float64 `json:"base_fragility"`
	ShockFragility   float64 `json:"shock_fragility"`
	Note             string  `json:"note,omitempty"`
	DataProvenance   string  `json:"data_provenance"`
}

type reportTradeEvidence struct {
	Importer          string  `json:"importer"`
	Commodity         string  `json:"commodity"`
	HHI               float64 `json:"hhi"`
	ConcentrationRisk string  `json:"concentration_risk"`
	TopSupplierName   string  `json:"top_supplier_name"`
	TopSupplierCode   string  `json:"top_supplier_code"`
	TopSupplierShare  float64 `json:"top_supplier_share"`
	Summary           string  `json:"summary"`
	DataProvenance    string  `json:"data_provenance"`
}

type reportContextItem struct {
	Entity         string  `json:"entity"`
	Score          float64 `json:"score"`
	RiskLevel      string  `json:"risk_level"`
	Summary        string  `json:"summary"`
	DataProvenance string  `json:"data_provenance"`
}

func (s *apiServer) handleScenarioReport(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body scenarioReportRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error(),
			`expected {"source":"Taiwan","commodity":"semiconductors","shock_type":"export_collapse","drop_percent":30,"depth":3}`)
		return
	}
	if strings.TrimSpace(body.Source) == "" || strings.TrimSpace(body.Commodity) == "" {
		writeAPIError(w, http.StatusBadRequest, "source and commodity are required",
			`example: {"source":"Taiwan","commodity":"semiconductors","shock_type":"export_collapse","drop_percent":30,"depth":3}`)
		return
	}

	cfg := config.Default()
	req := simulation.ShockRequest{
		Source:    body.Source,
		Commodity: body.Commodity,
		ShockType: cfg.DefaultShockType,
		DropPct:   cfg.DefaultDrop,
		Depth:     cfg.DefaultDepth,
	}
	if strings.TrimSpace(body.ShockType) != "" {
		req.ShockType = body.ShockType
	}
	if body.DropPercent != nil {
		req.DropPct = *body.DropPercent
	} else if body.Drop != nil {
		req.DropPct = *body.Drop
	}
	if body.Depth != nil {
		req.Depth = *body.Depth
	}

	fused, ok := s.loadFused(w)
	if !ok {
		return
	}

	simCtx := fused.SimCtx
	res, err := simulation.RunWithContext(fused.Dataset.Graph, cfg, req, &simCtx)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(),
			"check source/commodity names and that source links to the commodity in this graph")
		return
	}

	ctx := s.collectScenarioReportContext(fused.Meta, simCtx)
	report := buildScenarioReport(res, ctx)
	writeJSONStatus(w, http.StatusOK, report)
}

// scenarioReportContext holds optional observed panels used to enrich the report.
type scenarioReportContext struct {
	Trade            *trade.ResolvedTrade
	EventScores      []events.CountryScore
	MacroScores      []macro.CountryScore
	CommodityScores  []commodities.CommodityScore
	FusionMeta       graphfusion.Meta
	SimCtx           simulation.Context
	HasPinkSheet     bool
	HasEventRisk     bool
	HasMacro         bool
	HasTrade         bool
}

func (s *apiServer) collectScenarioReportContext(meta graphfusion.Meta, simCtx simulation.Context) scenarioReportContext {
	ctx := scenarioReportContext{FusionMeta: meta, SimCtx: simCtx}

	if resolved, err := trade.ResolveTrade(s.cfg.TradeData); err == nil && len(resolved.File.Records)+depCount(resolved) > 0 {
		ctx.Trade = &resolved
		ctx.HasTrade = resolved.RealTradeData || len(resolved.File.Records) > 0
	}

	src, _ := s.fusedFragilitySources()
	if src.ProcessedEventRisk != nil && len(src.ProcessedEventRisk.Countries) > 0 {
		ctx.EventScores = eventrisk.ToLegacyCountryScores(*src.ProcessedEventRisk)
		ctx.HasEventRisk = true
	} else if src.Events != nil {
		ctx.EventScores = events.ScoreCountries(*src.Events, events.DefaultWeights())
		ctx.HasEventRisk = len(ctx.EventScores) > 0
	}

	if src.ProcessedMacro != nil && len(src.ProcessedMacro.Scores) > 0 {
		ctx.MacroScores = processedMacroToScores(*src.ProcessedMacro)
		ctx.HasMacro = true
	} else if src.Macro != nil {
		ctx.MacroScores = macro.ScoreCountries(*src.Macro, 0, macro.DefaultWeights())
		ctx.HasMacro = len(ctx.MacroScores) > 0
	}

	if src.Commodities != nil {
		ctx.CommodityScores = commodities.ScoreCommodities(*src.Commodities, commodities.DefaultWeights())
		ctx.HasPinkSheet = len(ctx.CommodityScores) > 0
	}

	return ctx
}

func depCount(r trade.ResolvedTrade) int {
	if r.DependencyFile == nil {
		return 0
	}
	return len(r.DependencyFile.Dependencies)
}

func processedMacroToScores(f macro.ProcessedScoreFile) []macro.CountryScore {
	out := make([]macro.CountryScore, 0, len(f.Scores))
	for _, s := range f.Scores {
		out = append(out, macro.CountryScore{
			CountryCode: s.CountryCode,
			CountryName: s.CountryName,
			Year:        s.Year,
			Score:       s.MacroExposureScore,
			RiskLevel:   macro.RiskLevel(s.MacroExposureScore),
		})
	}
	return out
}

// buildScenarioReport assembles a deterministic analyst report from a shock result
// and optional observed-context panels.
func buildScenarioReport(res simulation.Result, ctx scenarioReportContext) scenarioReport {
	source := res.SourceNode.Name
	commodity := res.CommodityNode.Name
	drop := res.Request.DropPct
	shockLabel := strings.ReplaceAll(res.Profile.Type, "_", " ")

	title := fmt.Sprintf("Scenario Intelligence Report: %s %s %s (%.0f%%)",
		source, commodity, shockLabel, drop)

	graphProv := graphfusion.SourceStrategic
	direct := reportImpacts(res.Direct, graphProv)
	second := reportImpacts(res.SecondOrder, graphProv)
	countries := reportImpacts(res.TopCountries, graphProv)
	commodities := reportImpacts(res.TopCommodities, graphProv)
	sectors := reportImpacts(res.TopSectors, graphProv)

	tradeEv := buildTradeEvidence(res, ctx)
	eventCtx := buildEventRiskContext(res, ctx)
	macroCtx := buildMacroContext(res, ctx)
	commodityCtx := buildCommodityFragilityContext(res, ctx)

	findings := buildKeyFindings(res, tradeEv, eventCtx, macroCtx, commodityCtx)
	summary := buildExecutiveSummary(res, findings, tradeEv)

	return scenarioReport{
		Title:                  title,
		ExecutiveSummary:       summary,
		KeyFindings:            findings,
		DirectExposure:         direct,
		SecondOrderExposure:    second,
		MostExposedCountries:   countries,
		MostExposedCommodities: commodities,
		MostExposedSectors:     sectors,
		TradeEvidence:          tradeEv,
		EventRiskContext:       eventCtx,
		MacroContext:           macroCtx,
		CommodityFragility:     commodityCtx,
		ModelAssumptions:       buildModelAssumptions(res, ctx),
		DataSources:            buildReportDataSources(ctx),
		Limitations:            buildReportLimitations(ctx),
	}
}

func reportImpacts(items []simulation.NodeImpact, provenance string) []reportExposureItem {
	out := make([]reportExposureItem, 0, len(items))
	for _, it := range items {
		out = append(out, reportExposureItem{
			Entity:          it.Node.Name,
			Type:            string(it.Node.Type),
			Distance:        it.Distance,
			EstimatedImpact: round(it.Impact, 4),
			FragilityDelta:  round(it.Delta, 4),
			BaseFragility:   round(it.BaseFragility, 2),
			ShockFragility:  round(it.ShockFragility, 2),
			Note:            "model-derived estimated exposure from dependency propagation",
			DataProvenance:  provenance,
		})
	}
	if out == nil {
		out = []reportExposureItem{}
	}
	return out
}

func buildExecutiveSummary(res simulation.Result, findings []string, tradeEv []reportTradeEvidence) string {
	source := res.SourceNode.Name
	commodity := res.CommodityNode.Name
	drop := res.Request.DropPct
	shockLabel := strings.ReplaceAll(res.Profile.Type, "_", " ")

	countryN := len(res.TopCountries)
	sectorN := len(res.TopSectors)
	pathN := len(res.Paths)

	var topCountry string
	if countryN > 0 {
		topCountry = res.TopCountries[0].Node.Name
	}

	parts := []string{
		fmt.Sprintf(
			"A model-derived %.0f%% %s %s originating from %s produces estimated exposure across %d countries, %d sectors, and %d dependency paths under the baseline dependency graph.",
			drop, commodity, shockLabel, source, countryN, sectorN, pathN,
		),
	}
	if topCountry != "" {
		parts = append(parts, fmt.Sprintf(
			"The highest relative fragility increase among countries is estimated for %s.",
			topCountry,
		))
	}
	if len(tradeEv) > 0 {
		parts = append(parts, fmt.Sprintf(
			"Observed trade concentration for %s provides supporting evidence for importer-side supplier dependence (UN Comtrade).",
			commodity,
		))
	}
	if len(findings) > 0 {
		parts = append(parts, "Key findings below summarize propagation, trade, event-risk, and macro context without claiming predictive certainty.")
	}
	return strings.Join(parts, " ")
}

func buildKeyFindings(
	res simulation.Result,
	tradeEv []reportTradeEvidence,
	eventCtx, macroCtx, commodityCtx []reportContextItem,
) []string {
	var findings []string
	source := res.SourceNode.Name
	commodity := res.CommodityNode.Name

	findings = append(findings, fmt.Sprintf(
		"Shock profile %q attenuates propagation at %.0f%% per hop with recommended depth %d (baseline dependency graph).",
		res.Profile.Type, res.Profile.Attenuation*100, res.Profile.RecommendedDepth,
	))

	if len(res.Direct) > 0 {
		findings = append(findings, fmt.Sprintf(
			"%d entities show direct estimated exposure to %s (distance 2 from the shocked commodity).",
			len(res.Direct), commodity,
		))
	}
	if len(res.SecondOrder) > 0 {
		findings = append(findings, fmt.Sprintf(
			"%d entities show second-order estimated exposure via downstream dependency links.",
			len(res.SecondOrder),
		))
	}
	if len(res.TopCountries) > 0 {
		findings = append(findings, fmt.Sprintf(
			"Most exposed country by relative fragility delta: %s (Δ %.2f).",
			res.TopCountries[0].Node.Name, res.TopCountries[0].Delta,
		))
	}
	if len(res.TopSectors) > 0 {
		findings = append(findings, fmt.Sprintf(
			"Most exposed sector by relative fragility delta: %s (Δ %.2f).",
			res.TopSectors[0].Node.Name, res.TopSectors[0].Delta,
		))
	}
	for _, te := range tradeEv {
		if te.TopSupplierName != "" {
			findings = append(findings, fmt.Sprintf(
				"Observed trade concentration: %s imports of %s show HHI %.2f (%s risk), with top supplier %s at %.1f%% share (UN Comtrade).",
				te.Importer, te.Commodity, te.HHI, te.ConcentrationRisk, te.TopSupplierName, te.TopSupplierShare*100,
			))
			break
		}
	}
	if len(eventCtx) > 0 {
		findings = append(findings, fmt.Sprintf(
			"Event-risk context for %s: score %.1f (%s) from GDELT public event signals.",
			eventCtx[0].Entity, eventCtx[0].Score, eventCtx[0].RiskLevel,
		))
	}
	if len(macroCtx) > 0 {
		findings = append(findings, fmt.Sprintf(
			"Macro-risk context for %s: relative fragility score %.1f (%s) from World Bank Macro indicators.",
			macroCtx[0].Entity, macroCtx[0].Score, macroCtx[0].RiskLevel,
		))
	}
	if len(commodityCtx) > 0 {
		findings = append(findings, fmt.Sprintf(
			"Commodity price-stress context for %s: score %.1f (%s) from World Bank Pink Sheet when available.",
			commodityCtx[0].Entity, commodityCtx[0].Score, commodityCtx[0].RiskLevel,
		))
	}
	_ = source
	if findings == nil {
		findings = []string{}
	}
	return findings
}

func buildTradeEvidence(res simulation.Result, ctx scenarioReportContext) []reportTradeEvidence {
	out := []reportTradeEvidence{}
	if ctx.Trade == nil {
		return out
	}
	commodity := res.CommodityNode.Name
	importers := make([]string, 0, len(res.TopCountries)+1)
	seen := map[string]struct{}{}
	for _, c := range res.TopCountries {
		if c.Node.Type != models.Country {
			continue
		}
		name := c.Node.Name
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		importers = append(importers, name)
	}
	// Also try a few large demand-side names if tops are empty.
	if len(importers) == 0 {
		for _, name := range []string{"United States", "China", "Germany", "Japan", "India"} {
			importers = append(importers, name)
		}
	}

	const maxEvidence = 5
	for _, importer := range importers {
		if len(out) >= maxEvidence {
			break
		}
		con := trade.BuildConcentrationResolved(*ctx.Trade, importer, commodity)
		if !con.HasData {
			continue
		}
		share := con.TopSupplier.Share
		summary := fmt.Sprintf(
			"Observed trade concentration for %s imports of %s: HHI %.2f (%s). Top supplier %s holds an estimated %.1f%% share.",
			con.ImporterName, con.Commodity, con.HHI, con.RiskLevel,
			labelOrBlank(con.TopSupplier.ExporterName, con.TopSupplier.ExporterCode),
			share*100,
		)
		out = append(out, reportTradeEvidence{
			Importer:          prefer(con.ImporterName, importer),
			Commodity:         prefer(con.Commodity, commodity),
			HHI:               round(con.HHI, 4),
			ConcentrationRisk: con.RiskLevel,
			TopSupplierName:   con.TopSupplier.ExporterName,
			TopSupplierCode:   con.TopSupplier.ExporterCode,
			TopSupplierShare:  round(share, 4),
			Summary:           summary,
			DataProvenance:    "UN Comtrade",
		})
	}
	return out
}

func buildEventRiskContext(res simulation.Result, ctx scenarioReportContext) []reportContextItem {
	out := []reportContextItem{}
	if !ctx.HasEventRisk || len(ctx.EventScores) == 0 {
		return out
	}
	names := focusCountryNames(res)
	for _, name := range names {
		if sc, ok := findEventScore(ctx.EventScores, name); ok {
			out = append(out, reportContextItem{
				Entity: sc.CountryName,
				Score:  round(sc.Score, 1),
				RiskLevel: sc.RiskLevel,
				Summary: fmt.Sprintf(
					"GDELT-based event-risk score %.1f (%s) provides public-signal context for %s; this is relative exposure, not a forecast.",
					sc.Score, sc.RiskLevel, sc.CountryName,
				),
				DataProvenance: "GDELT",
			})
		}
	}
	return out
}

func buildMacroContext(res simulation.Result, ctx scenarioReportContext) []reportContextItem {
	out := []reportContextItem{}
	if !ctx.HasMacro || len(ctx.MacroScores) == 0 {
		return out
	}
	for _, name := range focusCountryNames(res) {
		if sc, ok := findMacroScore(ctx.MacroScores, name); ok {
			out = append(out, reportContextItem{
				Entity:    sc.CountryName,
				Score:     round(sc.Score, 1),
				RiskLevel: sc.RiskLevel,
				Summary: fmt.Sprintf(
					"World Bank Macro relative fragility score %.1f (%s) for %s indicates structural exposure context around the scenario.",
					sc.Score, sc.RiskLevel, sc.CountryName,
				),
				DataProvenance: "World Bank Macro",
			})
		}
	}
	return out
}

func buildCommodityFragilityContext(res simulation.Result, ctx scenarioReportContext) []reportContextItem {
	out := []reportContextItem{}
	if !ctx.HasPinkSheet || len(ctx.CommodityScores) == 0 {
		return out
	}
	names := []string{res.CommodityNode.Name}
	for _, c := range res.TopCommodities {
		names = append(names, c.Node.Name)
	}
	seen := map[string]struct{}{}
	for _, name := range names {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if sc, ok := findCommodityScore(ctx.CommodityScores, name); ok {
			out = append(out, reportContextItem{
				Entity:    sc.CommodityName,
				Score:     round(sc.Score, 1),
				RiskLevel: sc.RiskLevel,
				Summary: fmt.Sprintf(
					"World Bank Pink Sheet price-stress score %.1f (%s) for %s provides commodity fragility context when price history is available.",
					sc.Score, sc.RiskLevel, sc.CommodityName,
				),
				DataProvenance: "World Bank Pink Sheet",
			})
		}
		if len(out) >= 4 {
			break
		}
	}
	return out
}

func focusCountryNames(res simulation.Result) []string {
	seen := map[string]struct{}{}
	var names []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	add(res.SourceNode.Name)
	for _, c := range res.TopCountries {
		add(c.Node.Name)
	}
	for _, d := range res.Direct {
		if d.Node.Type == models.Country {
			add(d.Node.Name)
		}
	}
	return names
}

func findEventScore(scores []events.CountryScore, name string) (events.CountryScore, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	canon := strings.ToLower(trade.NormalizeCountryName(name))
	for _, s := range scores {
		if strings.EqualFold(s.CountryName, name) ||
			strings.ToLower(s.CountryName) == want ||
			strings.ToLower(trade.NormalizeCountryName(s.CountryName)) == canon ||
			strings.EqualFold(s.CountryCode, name) {
			return s, true
		}
	}
	return events.CountryScore{}, false
}

func findMacroScore(scores []macro.CountryScore, name string) (macro.CountryScore, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	canon := strings.ToLower(trade.NormalizeCountryName(name))
	for _, s := range scores {
		if strings.EqualFold(s.CountryName, name) ||
			strings.ToLower(s.CountryName) == want ||
			strings.ToLower(trade.NormalizeCountryName(s.CountryName)) == canon ||
			strings.EqualFold(s.CountryCode, name) {
			return s, true
		}
	}
	return macro.CountryScore{}, false
}

func findCommodityScore(scores []commodities.CommodityScore, name string) (commodities.CommodityScore, bool) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, s := range scores {
		if strings.EqualFold(s.CommodityName, name) ||
			strings.ToLower(s.CommodityName) == want ||
			strings.EqualFold(s.CommodityCode, name) ||
			strings.Contains(strings.ToLower(s.CommodityName), want) ||
			strings.Contains(want, strings.ToLower(s.CommodityName)) {
			return s, true
		}
	}
	return commodities.CommodityScore{}, false
}

func buildModelAssumptions(res simulation.Result, ctx scenarioReportContext) []string {
	assumptions := []string{
		fmt.Sprintf("Shock type %q propagates only along allowed relationship types: %s.",
			res.Profile.Type, strings.Join(res.Profile.AllowedRelationshipStrings(), ", ")),
		fmt.Sprintf("Initial disruption is modeled as a %.0f%% flow drop with attenuation %.2f per hop and depth %d.",
			res.Request.DropPct, res.Profile.Attenuation, res.Request.Depth),
		"Impact magnitudes are model-derived estimated exposure, not observed outcomes.",
		"Country and sector rankings use relative fragility deltas on the fused baseline dependency graph.",
	}
	if ctx.SimCtx.EventRiskMultiplierApplied {
		assumptions = append(assumptions, "Event-risk multipliers from GDELT were applied during propagation where matched.")
	}
	if ctx.SimCtx.RealPriceStressUsed {
		assumptions = append(assumptions, "Commodity price-stress signals from World Bank Pink Sheet informed vulnerability weighting where available.")
	}
	if ctx.FusionMeta.RealTradeEdgesUsed {
		assumptions = append(assumptions, "Real UN Comtrade dependency edges were fused into the baseline graph where available.")
	}
	return assumptions
}

func buildReportDataSources(ctx scenarioReportContext) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(graphfusion.SourceStrategic)
	if ctx.HasTrade || ctx.FusionMeta.RealTradeEdgesUsed {
		add("UN Comtrade")
	}
	if ctx.HasEventRisk || ctx.SimCtx.RealEventRiskUsed {
		add("GDELT")
	}
	if ctx.HasMacro {
		add("World Bank Macro")
	}
	if ctx.HasPinkSheet || ctx.SimCtx.RealPriceStressUsed {
		add("World Bank Pink Sheet")
	}
	for _, s := range ctx.FusionMeta.DataSources {
		add(s)
	}
	sort.Strings(out)
	return out
}

func buildReportLimitations(ctx scenarioReportContext) []string {
	limits := []string{
		"This report does not predict future events; it estimates relative exposure under stated model assumptions.",
		"Graph coverage and processed panel completeness bound the blast radius that can be observed.",
		"Trade concentration reflects historical UN Comtrade flows and may lag current market conditions.",
		"Event-risk and macro scores are public-signal / structural indicators, not causal attributions.",
	}
	if !ctx.HasTrade {
		limits = append(limits, "UN Comtrade trade evidence was unavailable for this run; trade concentration sections may be empty.")
	}
	if !ctx.HasEventRisk {
		limits = append(limits, "GDELT event-risk context was unavailable for this run.")
	}
	if !ctx.HasMacro {
		limits = append(limits, "World Bank Macro context was unavailable for this run.")
	}
	if !ctx.HasPinkSheet {
		limits = append(limits, "World Bank Pink Sheet price context was unavailable for this run.")
	}
	return limits
}

func prefer(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func labelOrBlank(name, code string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return code
}
