package cli

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/atlasgraph/atlas/internal/config"
	analyticsdb "github.com/atlasgraph/atlas/internal/db"
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
	Title            string   `json:"title"`
	ExecutiveSummary string   `json:"executive_summary"`
	KeyFindings      []string `json:"key_findings"`

	DirectExposure      []reportExposureItem `json:"direct_exposure"`
	SecondOrderExposure []reportExposureItem `json:"second_order_exposure"`

	// Exposure-list metadata: totals reflect every affected entity in the run,
	// while returned counts reflect the truncated top-N slices above.
	TotalDirectExposureCount         int `json:"total_direct_exposure_count"`
	TotalSecondOrderExposureCount    int `json:"total_second_order_exposure_count"`
	ReturnedDirectExposureCount      int `json:"returned_direct_exposure_count"`
	ReturnedSecondOrderExposureCount int `json:"returned_second_order_exposure_count"`

	MostExposedCountries   []reportExposureItem  `json:"most_exposed_countries"`
	MostExposedCommodities []reportExposureItem  `json:"most_exposed_commodities"`
	MostExposedSectors     []reportExposureItem  `json:"most_exposed_sectors"`
	TradeEvidence          []reportTradeEvidence `json:"trade_evidence"`
	EventRiskContext       []reportContextItem   `json:"event_risk_context"`
	MacroContext           []reportContextItem   `json:"macro_context"`
	CommodityFragility     []reportContextItem   `json:"commodity_fragility_context"`
	ModelAssumptions       []string              `json:"model_assumptions"`
	DataSources            []string              `json:"data_sources"`
	Limitations            []string              `json:"limitations"`
}

// Exposure-list truncation limits keep the report analyst-readable rather than a raw dump.
const (
	maxDirectExposure   = 10
	maxSecondOrder      = 10
	maxTopEntities      = 5
	maxContextCountries = 6
)

type reportExposureItem struct {
	Entity          string  `json:"entity"`
	Type            string  `json:"type"`
	Distance        int     `json:"distance"`
	EstimatedImpact float64 `json:"estimated_impact"`
	FragilityDelta  float64 `json:"fragility_delta"`
	BaseFragility   float64 `json:"base_fragility"`
	ShockFragility  float64 `json:"shock_fragility"`
	Note            string  `json:"note,omitempty"`
	DataProvenance  string  `json:"data_provenance"`
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
	Available      bool    `json:"available"`
	Score          float64 `json:"score,omitempty"`
	RiskLevel      string  `json:"risk_level,omitempty"`
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
	if s.db != nil {
		if err := s.db.InsertScenarioRun(r.Context(), analyticsdb.ScenarioRunInput{
			ScenarioID:           newScenarioRunID(),
			Source:               req.Source,
			Commodity:            req.Commodity,
			ShockType:            req.ShockType,
			DropPercent:          req.DropPct,
			Depth:                req.Depth,
			TopAffectedCountries: report.MostExposedCountries,
			TopAffectedSectors:   report.MostExposedSectors,
			Report:               report,
		}); err != nil {
			writeAPIError(w, http.StatusInternalServerError,
				"scenario report generated but could not be persisted: "+err.Error(),
				"verify the PostgreSQL migration has been applied")
			return
		}
	}
	writeJSONStatus(w, http.StatusOK, report)
}

func newScenarioRunID() string {
	var suffix [6]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return fmt.Sprintf("scenario-%d", time.Now().UTC().UnixNano())
	}
	return fmt.Sprintf("scenario-%d-%s", time.Now().UTC().UnixMilli(), hex.EncodeToString(suffix[:]))
}

// scenarioReportContext holds optional observed panels used to enrich the report.
type scenarioReportContext struct {
	Trade           *trade.ResolvedTrade
	EventScores     []events.CountryScore
	MacroScores     []macro.CountryScore
	CommodityScores []commodities.CommodityScore
	FusionMeta      graphfusion.Meta
	SimCtx          simulation.Context
	HasPinkSheet    bool
	HasEventRisk    bool
	HasMacro        bool
	HasTrade        bool
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
		ctx.MacroScores = src.ProcessedMacro.ToCountryScores()
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
	direct, directTotal := topExposure(res.Direct, graphProv, maxDirectExposure)
	second, secondTotal := topExposure(res.SecondOrder, graphProv, maxSecondOrder)
	countries, _ := topExposure(res.TopCountries, graphProv, maxTopEntities)
	commodityRows, _ := topExposure(res.TopCommodities, graphProv, maxTopEntities)
	sectors, _ := topExposure(res.TopSectors, graphProv, maxTopEntities)

	tradeEv := buildTradeEvidence(res, ctx)
	eventCtx := buildEventRiskContext(res, ctx)
	macroCtx := buildMacroContext(res, ctx)
	commodityCtx := buildCommodityFragilityContext(res, ctx, tradeEv)

	dataSources := buildReportDataSources(ctx)
	summary := buildExecutiveSummary(res, countries, sectors, directTotal, tradeEv)
	findings := buildKeyFindings(res, countries, sectors, tradeEv, eventCtx, macroCtx, dataSources)

	return scenarioReport{
		Title:            title,
		ExecutiveSummary: summary,
		KeyFindings:      findings,

		DirectExposure:      direct,
		SecondOrderExposure: second,

		TotalDirectExposureCount:         directTotal,
		TotalSecondOrderExposureCount:    secondTotal,
		ReturnedDirectExposureCount:      len(direct),
		ReturnedSecondOrderExposureCount: len(second),

		MostExposedCountries:   countries,
		MostExposedCommodities: commodityRows,
		MostExposedSectors:     sectors,
		TradeEvidence:          tradeEv,
		EventRiskContext:       eventCtx,
		MacroContext:           macroCtx,
		CommodityFragility:     commodityCtx,
		ModelAssumptions:       buildModelAssumptions(res, ctx),
		DataSources:            dataSources,
		Limitations:            buildReportLimitations(ctx),
	}
}

// topExposure converts node impacts to report rows sorted by fragility delta
// (highest first), returning the truncated top-N slice and the pre-truncation
// total. Ties break on estimated impact then entity name for determinism.
func topExposure(items []simulation.NodeImpact, provenance string, limit int) ([]reportExposureItem, int) {
	all := make([]reportExposureItem, 0, len(items))
	for _, it := range items {
		all = append(all, reportExposureItem{
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
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].FragilityDelta != all[j].FragilityDelta {
			return all[i].FragilityDelta > all[j].FragilityDelta
		}
		if all[i].EstimatedImpact != all[j].EstimatedImpact {
			return all[i].EstimatedImpact > all[j].EstimatedImpact
		}
		return all[i].Entity < all[j].Entity
	})
	total := len(all)
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	if all == nil {
		all = []reportExposureItem{}
	}
	return all, total
}

func buildExecutiveSummary(res simulation.Result, countries, sectors []reportExposureItem, directTotal int, tradeEv []reportTradeEvidence) string {
	source := res.SourceNode.Name
	commodity := res.CommodityNode.Name
	drop := res.Request.DropPct
	shockLabel := strings.ReplaceAll(res.Profile.Type, "_", " ")
	pathN := len(res.Paths)

	parts := []string{
		fmt.Sprintf(
			"A model-derived %.0f%% %s %s originating from %s propagates through the baseline dependency graph, producing an estimated %d directly exposed %s across %d dependency %s.",
			drop, commodity, shockLabel, source,
			directTotal, plural(directTotal, "entity", "entities"),
			pathN, plural(pathN, "path", "paths"),
		),
	}

	countryNames := topEntityNames(countries, 3)
	sectorNames := topEntityNames(sectors, 2)
	switch {
	case len(countryNames) > 0 && len(sectorNames) > 0:
		parts = append(parts, fmt.Sprintf(
			"Top reported exposure includes %s among countries and %s among sectors.",
			joinNames(countryNames), joinNames(sectorNames),
		))
	case len(countryNames) > 0:
		parts = append(parts, fmt.Sprintf(
			"Top reported exposure includes %s among countries.", joinNames(countryNames),
		))
	case len(sectorNames) > 0:
		parts = append(parts, fmt.Sprintf(
			"Top reported exposure includes %s among sectors.", joinNames(sectorNames),
		))
	}

	if best, ok := strongestTradeEvidence(tradeEv); ok {
		parts = append(parts, fmt.Sprintf(
			"Observed trade concentration for %s (UN Comtrade) supports importer-side supplier dependence, led by %s.",
			commodity, prefer(best.Importer, "a major importer"),
		))
	}
	parts = append(parts, "Figures are relative exposure estimates, not predictions.")
	return sanitizeText(strings.Join(parts, " "))
}

func buildKeyFindings(
	res simulation.Result,
	countries, sectors []reportExposureItem,
	tradeEv []reportTradeEvidence,
	eventCtx, macroCtx []reportContextItem,
	dataSources []string,
) []string {
	findings := []string{}

	// a. shock profile
	findings = append(findings, fmt.Sprintf(
		"Shock profile %q attenuates propagation at %.0f%% per hop with recommended depth %d.",
		res.Profile.Type, res.Profile.Attenuation*100, res.Profile.RecommendedDepth,
	))

	// b. top exposed countries
	if len(countries) > 0 {
		findings = append(findings, fmt.Sprintf(
			"Most exposed countries by relative fragility: %s (top delta %+.2f).",
			joinNames(topEntityNames(countries, 3)), countries[0].FragilityDelta,
		))
	}

	// c. top exposed sectors
	if len(sectors) > 0 {
		findings = append(findings, fmt.Sprintf(
			"Most exposed sectors by relative fragility: %s (top delta %+.2f).",
			joinNames(topEntityNames(sectors, 3)), sectors[0].FragilityDelta,
		))
	}

	// d. strongest trade concentration evidence
	if best, ok := strongestTradeEvidence(tradeEv); ok {
		findings = append(findings, fmt.Sprintf(
			"Strongest observed trade concentration: %s imports of %s at HHI %.2f (%s), top supplier %s ~%.1f%% share (UN Comtrade).",
			best.Importer, best.Commodity, best.HHI, best.ConcentrationRisk,
			labelOrBlank(best.TopSupplierName, best.TopSupplierCode), best.TopSupplierShare*100,
		))
	}

	// e. event-risk context
	if ev, ok := firstAvailableContext(eventCtx); ok {
		findings = append(findings, fmt.Sprintf(
			"Event-risk context: %s scores %.1f (%s) from GDELT public event signals.",
			ev.Entity, ev.Score, ev.RiskLevel,
		))
	}

	// f. macro context availability (report the source country's status first)
	if mf := buildMacroFinding(res, macroCtx); mf != "" {
		findings = append(findings, mf)
	}

	// g. data provenance
	if len(dataSources) > 0 {
		findings = append(findings, fmt.Sprintf(
			"Evidence provenance: %s.", joinNames(dataSources),
		))
	}

	// Keep the brief tight: 6-8 findings max.
	if len(findings) > 8 {
		findings = findings[:8]
	}
	for i := range findings {
		findings[i] = sanitizeText(findings[i])
	}
	return findings
}

// buildMacroFinding surfaces the shock source's macro availability first so the
// finding never claims "0.0 (Low)" for a country without World Bank Macro data.
func buildMacroFinding(res simulation.Result, macroCtx []reportContextItem) string {
	source := res.SourceNode.Name
	for _, m := range macroCtx {
		if !sameCountryLabel(m.Entity, source) {
			continue
		}
		if m.Available {
			return fmt.Sprintf(
				"Macro context: %s scores %.1f (%s) from World Bank Macro indicators.",
				m.Entity, m.Score, m.RiskLevel,
			)
		}
		return fmt.Sprintf("Macro context: World Bank Macro data is unavailable for %s.", m.Entity)
	}
	if m, ok := firstAvailableContext(macroCtx); ok {
		return fmt.Sprintf(
			"Macro context: %s scores %.1f (%s) from World Bank Macro indicators.",
			m.Entity, m.Score, m.RiskLevel,
		)
	}
	if names := unavailableContextNames(macroCtx); len(names) > 0 {
		return fmt.Sprintf("Macro context: World Bank Macro data is unavailable for %s.", joinNames(names))
	}
	return ""
}

func sameCountryLabel(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if strings.EqualFold(a, b) {
		return true
	}
	return strings.EqualFold(trade.NormalizeCountryName(a), trade.NormalizeCountryName(b))
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
	for _, name := range limitStrings(focusCountryNames(res), maxContextCountries) {
		if sc, ok := findEventScore(ctx.EventScores, name); ok {
			out = append(out, reportContextItem{
				Entity:    sc.CountryName,
				Available: true,
				Score:     round(sc.Score, 1),
				RiskLevel: sc.RiskLevel,
				Summary: fmt.Sprintf(
					"GDELT-based event-risk score %.1f (%s) provides public-signal context for %s; this is relative exposure, not a forecast.",
					sc.Score, sc.RiskLevel, sc.CountryName,
				),
				DataProvenance: "GDELT",
			})
			continue
		}
		display := displayCountryName(name)
		out = append(out, reportContextItem{
			Entity:    display,
			Available: false,
			Summary: fmt.Sprintf(
				"GDELT event-risk data is unavailable for %s; event-risk context is not included in this scenario score.",
				display,
			),
			DataProvenance: "GDELT",
		})
	}
	return out
}

func buildMacroContext(res simulation.Result, ctx scenarioReportContext) []reportContextItem {
	out := []reportContextItem{}
	if !ctx.HasMacro || len(ctx.MacroScores) == 0 {
		return out
	}
	for _, name := range limitStrings(focusCountryNames(res), maxContextCountries) {
		// A record only counts as available when the source macro dataset holds
		// real indicator data. A bare 0-score row with no available components
		// (e.g. Taiwan/TWN, which is absent from the World Bank Macro API) must
		// not surface as a misleading "0.0 (Low)" macro context.
		if sc, ok := findMacroScore(ctx.MacroScores, name); ok && macroScoreHasData(sc) {
			out = append(out, reportContextItem{
				Entity:    sc.CountryName,
				Available: true,
				Score:     round(sc.Score, 1),
				RiskLevel: sc.RiskLevel,
				Summary: fmt.Sprintf(
					"World Bank Macro relative fragility score %.1f (%s) for %s indicates structural exposure context around the scenario.",
					sc.Score, sc.RiskLevel, sc.CountryName,
				),
				DataProvenance: "World Bank Macro",
			})
			continue
		}
		display := displayCountryName(name)
		out = append(out, reportContextItem{
			Entity:    display,
			Available: false,
			Summary: fmt.Sprintf(
				"World Bank Macro data is unavailable for %s; macro context is not included in this scenario score.",
				display,
			),
			DataProvenance: "World Bank Macro",
		})
	}
	return out
}

// macroScoreHasData reports whether a macro score reflects real indicator data.
// A country with a 0 score and no available components (or all indicators
// missing) is treated as having no macro data rather than a genuine low score.
func macroScoreHasData(sc macro.CountryScore) bool {
	if sc.Score > 0 {
		return true
	}
	for _, c := range sc.Components {
		if c.Available {
			return true
		}
	}
	return false
}

// buildCommodityFragilityContext always emits a fragility item for the shocked
// commodity, blending price-stress (when available) with supplier concentration,
// event exposure and graph centrality so the section is never empty.
func buildCommodityFragilityContext(res simulation.Result, ctx scenarioReportContext, tradeEv []reportTradeEvidence) []reportContextItem {
	out := []reportContextItem{}
	commodity := res.CommodityNode.Name
	if strings.TrimSpace(commodity) == "" {
		return out
	}

	var signals []string
	if te, ok := strongestTradeEvidenceForCommodity(tradeEv, commodity); ok {
		signals = append(signals, fmt.Sprintf(
			"observed supplier concentration HHI %.2f (%s), top supplier %s ~%.1f%% share (UN Comtrade)",
			te.HHI, te.ConcentrationRisk, labelOrBlank(te.TopSupplierName, te.TopSupplierCode), te.TopSupplierShare*100,
		))
	}
	if ctx.HasEventRisk {
		if ev, ok := findEventScore(ctx.EventScores, res.SourceNode.Name); ok {
			signals = append(signals, fmt.Sprintf(
				"source event-risk %.1f (%s) for %s (GDELT)", ev.Score, ev.RiskLevel, ev.CountryName,
			))
		}
	}
	signals = append(signals, fmt.Sprintf(
		"graph centrality reflected in %d directly and %d second-order exposed entities (baseline dependency graph)",
		len(res.Direct), len(res.SecondOrder),
	))

	item := reportContextItem{Entity: commodity, DataProvenance: "World Bank Pink Sheet"}
	if sc, ok := commodityPriceScore(ctx, commodity); ok {
		item.Available = true
		item.Score = round(sc.Score, 1)
		item.RiskLevel = sc.RiskLevel
		item.Summary = sanitizeText(fmt.Sprintf(
			"World Bank Pink Sheet price-stress score %.1f (%s) for %s; supporting signals: %s.",
			sc.Score, sc.RiskLevel, commodity, strings.Join(signals, "; "),
		))
	} else {
		item.Available = false
		item.Summary = sanitizeText(fmt.Sprintf(
			"World Bank Pink Sheet price-stress data is unavailable for %s; commodity fragility is derived from supporting signals: %s.",
			commodity, strings.Join(signals, "; "),
		))
	}
	out = append(out, item)

	// Add any other top commodities that do have price data, for context.
	seen := map[string]struct{}{strings.ToLower(strings.TrimSpace(commodity)): {}}
	for _, c := range res.TopCommodities {
		if len(out) >= 4 {
			break
		}
		name := c.Node.Name
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if sc, ok := commodityPriceScore(ctx, name); ok {
			out = append(out, reportContextItem{
				Entity:    sc.CommodityName,
				Available: true,
				Score:     round(sc.Score, 1),
				RiskLevel: sc.RiskLevel,
				Summary: sanitizeText(fmt.Sprintf(
					"World Bank Pink Sheet price-stress score %.1f (%s) for %s provides additional commodity fragility context.",
					sc.Score, sc.RiskLevel, sc.CommodityName,
				)),
				DataProvenance: "World Bank Pink Sheet",
			})
		}
	}
	return out
}

func commodityPriceScore(ctx scenarioReportContext, name string) (commodities.CommodityScore, bool) {
	if !ctx.HasPinkSheet || len(ctx.CommodityScores) == 0 {
		return commodities.CommodityScore{}, false
	}
	return findCommodityScore(ctx.CommodityScores, name)
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

// sanitizeText strips Unicode-encoding artifacts (notably the mojibake "Î" that
// appears when a Δ byte sequence is mis-decoded) and normalizes any stray Δ to
// plain-ASCII "delta " wording so report text stays clean across clients.
func sanitizeText(s string) string {
	s = strings.ReplaceAll(s, "Î", "+")
	s = strings.ReplaceAll(s, "\u0394 ", "delta ")
	s = strings.ReplaceAll(s, "\u0394", "delta ")
	s = strings.ReplaceAll(s, "delta  ", "delta ")
	return s
}

func plural(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// joinNames renders names with an Oxford comma: "A", "A and B", "A, B, and C".
func joinNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return strings.Join(names[:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}

func topEntityNames(items []reportExposureItem, n int) []string {
	out := make([]string, 0, n)
	for _, it := range items {
		if strings.TrimSpace(it.Entity) == "" {
			continue
		}
		out = append(out, it.Entity)
		if len(out) >= n {
			break
		}
	}
	return out
}

func limitStrings(in []string, n int) []string {
	if n > 0 && len(in) > n {
		return in[:n]
	}
	return in
}

// displayCountryName prefers the canonical trade name for display, falling back
// to the raw name when no alias is known.
func displayCountryName(name string) string {
	if canon := trade.NormalizeCountryName(name); strings.TrimSpace(canon) != "" {
		return canon
	}
	return strings.TrimSpace(name)
}

func firstAvailableContext(items []reportContextItem) (reportContextItem, bool) {
	for _, it := range items {
		if it.Available {
			return it, true
		}
	}
	return reportContextItem{}, false
}

func unavailableContextNames(items []reportContextItem) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, it := range items {
		if it.Available {
			continue
		}
		key := strings.ToLower(it.Entity)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, it.Entity)
	}
	return limitStrings(out, 3)
}

func strongestTradeEvidence(items []reportTradeEvidence) (reportTradeEvidence, bool) {
	best, found := reportTradeEvidence{}, false
	for _, te := range items {
		if te.TopSupplierName == "" && te.TopSupplierCode == "" {
			continue
		}
		if !found || te.HHI > best.HHI {
			best, found = te, true
		}
	}
	return best, found
}

func strongestTradeEvidenceForCommodity(items []reportTradeEvidence, commodity string) (reportTradeEvidence, bool) {
	want := strings.ToLower(strings.TrimSpace(commodity))
	best, found := reportTradeEvidence{}, false
	for _, te := range items {
		if strings.ToLower(strings.TrimSpace(te.Commodity)) != want {
			continue
		}
		if te.TopSupplierName == "" && te.TopSupplierCode == "" {
			continue
		}
		if !found || te.HHI > best.HHI {
			best, found = te, true
		}
	}
	return best, found
}
