package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/atlasgraph/atlas/internal/config"
	"github.com/atlasgraph/atlas/internal/data"
	analyticsdb "github.com/atlasgraph/atlas/internal/db"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/graphfusion"
	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/operationalimpact"
	"github.com/atlasgraph/atlas/internal/scoring/commodities"
	"github.com/atlasgraph/atlas/internal/scoring/fragility"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
	"github.com/atlasgraph/atlas/internal/simulation"
)

// serverConfig holds the data-source locations the HTTP API reads from. Each is
// loaded lazily, per request, so the server starts even when some are missing —
// only the affected endpoint then returns a helpful error.
type serverConfig struct {
	GraphData          string          // dataset dir (entities/dependencies/scenarios); "" => embedded sample
	TradeData          string          // ingested trade panel dir
	MacroData          string          // ingested World Bank macro dir (raw indicators)
	ProcessedMacroData string          // processed macro scores dir
	EventData          string          // legacy ingested GDELT event dir (demo fallback)
	ProcessedEventData string          // processed event-risk panel dir (event_risk.json)
	CommodityData      string          // ingested commodity price dir
	Database           *analyticsdb.DB // optional PostgreSQL analytics store
}

// corsAllowedOrigins are the dev-frontend origins permitted by CORS, ready for a
// future Vite frontend on :5173.
var corsAllowedOrigins = map[string]bool{
	"http://localhost:5173": true,
	"http://127.0.0.1:5173": true,
}

// apiServer wires the data config to the HTTP handlers.
type apiServer struct {
	cfg serverConfig
	db  *analyticsdb.DB
}

// newAPIServer builds the HTTP handler for the AtlasGraph API. It is separated
// from the listening loop so it can be exercised directly in tests.
func newAPIServer(cfg serverConfig) http.Handler {
	s := &apiServer{cfg: cfg, db: cfg.Database}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/graph/summary", s.handleGraphSummary)
	mux.HandleFunc("/api/graph/entities", s.handleGraphEntities)
	mux.HandleFunc("/api/scenarios", s.handleScenarios)
	mux.HandleFunc("/api/shock/options", s.handleShockOptions)
	mux.HandleFunc("/api/shock/valid-options", s.handleShockValidOptions)
	mux.HandleFunc("/api/shock", s.handleShock)
	mux.HandleFunc("/api/scenarios/compare", s.handleScenariosCompare)
	mux.HandleFunc("/api/trade/summary", s.handleTradeSummary)
	mux.HandleFunc("/api/trade/options", s.handleTradeOptions)
	mux.HandleFunc("/api/trade/dependency", s.handleTradeDependency)
	mux.HandleFunc("/api/trade/concentration", s.handleTradeConcentration)
	mux.HandleFunc("/api/macro/scores", s.handleMacroScores)
	mux.HandleFunc("/api/events/risk", s.handleEventsRisk)
	mux.HandleFunc("/api/commodities/stress", s.handleCommodityStress)
	mux.HandleFunc("/api/commodities/history", s.handleCommodityHistory)
	mux.HandleFunc("/api/fragility/countries", s.handleFragilityCountries)
	mux.HandleFunc("/api/fragility/commodities", s.handleFragilityCommodities)
	mux.HandleFunc("/api/fragility/summary", s.handleFragilitySummary)
	mux.HandleFunc("/api/reports/scenario", s.handleScenarioReport)
	mux.HandleFunc("/api/db/health", s.handleDBHealth)
	mux.HandleFunc("/api/db/summary", s.handleDBSummary)
	mux.HandleFunc("/api/db/trade/top-suppliers", s.handleDBTopSuppliers)
	mux.HandleFunc("/api/db/scenarios/recent", s.handleDBRecentScenarios)
	mux.HandleFunc("/api/pipeline/summary", s.handlePipelineSummary)
	mux.HandleFunc("/api/custom-data/analyze", s.handleCustomDataAnalyze)
	mux.HandleFunc("/", s.handleNotFound)

	return withCORS(mux)
}

// --- middleware & response helpers -----------------------------------------

// withCORS echoes an allowed dev-frontend origin and answers CORS preflight
// requests, leaving all other requests untouched.
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); corsAllowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// apiError is the JSON error envelope returned by every endpoint on failure.
type apiError struct {
	Error string `json:"error"`
	Hint  string `json:"hint,omitempty"`
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeAPIError(w http.ResponseWriter, status int, msg, hint string) {
	writeJSONStatus(w, status, apiError{Error: msg, Hint: hint})
}

// requireMethod enforces the HTTP method, returning a JSON 405 otherwise.
func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		writeAPIError(w, http.StatusMethodNotAllowed,
			"method "+r.Method+" not allowed", "use "+method+" for this endpoint")
		return false
	}
	return true
}

// --- handlers --------------------------------------------------------------

func (s *apiServer) handleNotFound(w http.ResponseWriter, r *http.Request) {
	writeAPIError(w, http.StatusNotFound, "no such endpoint: "+r.URL.Path,
		"see GET /health for the service status")
}

func (s *apiServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "atlasgraph-api",
		"version": config.Version,
	})
}

func (s *apiServer) handleGraphSummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	fused, ok := s.loadFused(w)
	if !ok {
		return
	}
	writeJSONStatus(w, http.StatusOK, buildGraphSummaryJSON(fused.Dataset.Graph, config.Default().TopN, &fused.Meta))
}

func (s *apiServer) handleScenarios(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	fused, ok := s.loadFused(w)
	if !ok {
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"scenarios": data.SortScenarios(fused.Dataset.Scenarios),
	})
}

func (s *apiServer) handleGraphEntities(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	fused, ok := s.loadFused(w)
	if !ok {
		return
	}
	writeJSONStatus(w, http.StatusOK, buildGraphEntitiesJSON(fused.Dataset.Graph))
}

func (s *apiServer) handleShockOptions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	fused, ok := s.loadFused(w)
	if !ok {
		return
	}
	writeJSONStatus(w, http.StatusOK, buildShockOptionsJSON(fused.Dataset))
}

// shockRequestBody is the POST /api/shock payload. Drop and Depth are pointers
// so omitted values fall back to engine defaults rather than 0.
type shockRequestBody struct {
	Source                 string   `json:"source"`
	Commodity              string   `json:"commodity"`
	Drop                   *float64 `json:"drop"`
	Depth                  *int     `json:"depth"`
	ShockType              string   `json:"shock_type"`
	Explain                bool     `json:"explain"`
	DurationDays           *int     `json:"duration_days,omitempty"`
	RecoverySpeed          string   `json:"recovery_speed,omitempty"`
	SubstituteAvailability string   `json:"substitute_availability,omitempty"`
	InventoryBufferDays    *int     `json:"inventory_buffer_days,omitempty"`
}

func applyOperationalRequest(req *simulation.ShockRequest, duration *int, recovery, substitute string, inventory *int) error {
	req.OperationalEnabled = true
	req.DurationDays = 30
	req.RecoverySpeed = "Moderate"
	req.SubstituteAvailability = "Medium"
	req.InventoryBufferDays = 30
	if duration != nil {
		req.DurationDays = *duration
	}
	if strings.TrimSpace(recovery) != "" {
		req.RecoverySpeed = recovery
	}
	if strings.TrimSpace(substitute) != "" {
		req.SubstituteAvailability = substitute
	}
	if inventory != nil {
		req.InventoryBufferDays = *inventory
	}
	_, err := operationalimpact.Evaluate(operationalimpact.Assumptions{
		DurationDays:           req.DurationDays,
		RecoverySpeed:          req.RecoverySpeed,
		SubstituteAvailability: req.SubstituteAvailability,
		InventoryBufferDays:    req.InventoryBufferDays,
	})
	return err
}

func (s *apiServer) handleShock(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body shockRequestBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error(),
			`expected {"source":"Taiwan","commodity":"semiconductors","drop":30,"depth":3,"shock_type":"export_collapse"}`)
		return
	}
	if strings.TrimSpace(body.Source) == "" || strings.TrimSpace(body.Commodity) == "" {
		writeAPIError(w, http.StatusBadRequest, "source and commodity are required",
			`example: {"source":"Taiwan","commodity":"semiconductors","drop":30,"depth":3,"shock_type":"export_collapse"}`)
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
	if body.Drop != nil {
		req.DropPct = *body.Drop
	}
	if body.Depth != nil {
		req.Depth = *body.Depth
	}
	if err := applyOperationalRequest(&req, body.DurationDays, body.RecoverySpeed, body.SubstituteAvailability, body.InventoryBufferDays); err != nil {
		writeAPIError(w, http.StatusBadRequest, err.Error(),
			"provide duration_days >= 0, recovery_speed Fast/Moderate/Slow, substitute_availability High/Medium/Low, and inventory_buffer_days >= 0")
		return
	}

	fused, ok := s.loadFused(w)
	if !ok {
		return
	}

	simCtx := fused.SimCtx
	res, err := simulation.RunWithContext(fused.Dataset.Graph, cfg, req, &simCtx)
	if err != nil {
		// Run's failures are client-driven (unknown entity, bad ranges, …).
		writeAPIError(w, http.StatusBadRequest, err.Error(),
			"check source/commodity names and that source links to the commodity in this graph")
		return
	}
	out := buildJSONResult(res, nil, body.Explain)
	attachDataFusion(&out, fused.Meta, fused.SimCtx)
	// Attach non-fatal, graph-aware warnings (suboptimal but still-valid combos).
	out.Warnings = shockWarnings(fused.Dataset.Graph, res.Profile, req.Source, req.Commodity)
	writeJSONStatus(w, http.StatusOK, out)
}

// compareScenarioBody is one shock in POST /api/scenarios/compare.
type compareScenarioBody struct {
	Label                  string   `json:"label"`
	Source                 string   `json:"source"`
	Commodity              string   `json:"commodity"`
	ShockType              string   `json:"shock_type"`
	Drop                   *float64 `json:"drop"`
	Depth                  *int     `json:"depth"`
	Explain                bool     `json:"explain"`
	DurationDays           *int     `json:"duration_days,omitempty"`
	RecoverySpeed          string   `json:"recovery_speed,omitempty"`
	SubstituteAvailability string   `json:"substitute_availability,omitempty"`
	InventoryBufferDays    *int     `json:"inventory_buffer_days,omitempty"`
}

type compareRequestBody struct {
	Scenarios []compareScenarioBody `json:"scenarios"`
}

func (s *apiServer) handleScenariosCompare(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	var body compareRequestBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error(),
			`expected {"scenarios":[{"label":"...","source":"Taiwan","commodity":"semiconductors","shock_type":"export_collapse","drop":30,"depth":3}]}`)
		return
	}
	if len(body.Scenarios) == 0 {
		writeAPIError(w, http.StatusBadRequest, "at least one scenario is required",
			`example: {"scenarios":[{"label":"Taiwan semiconductor export collapse","source":"Taiwan","commodity":"semiconductors","shock_type":"export_collapse","drop":30,"depth":3}]}`)
		return
	}

	cfg := config.Default()
	inputs := make([]simulation.CompareScenario, 0, len(body.Scenarios))
	for _, sc := range body.Scenarios {
		if strings.TrimSpace(sc.Source) == "" || strings.TrimSpace(sc.Commodity) == "" {
			writeAPIError(w, http.StatusBadRequest, "each scenario requires source and commodity", "")
			return
		}
		req := simulation.ShockRequest{
			Source:    sc.Source,
			Commodity: sc.Commodity,
			ShockType: cfg.DefaultShockType,
			DropPct:   cfg.DefaultDrop,
			Depth:     cfg.DefaultDepth,
		}
		if strings.TrimSpace(sc.ShockType) != "" {
			req.ShockType = sc.ShockType
		}
		if sc.Drop != nil {
			req.DropPct = *sc.Drop
		}
		if sc.Depth != nil {
			req.Depth = *sc.Depth
		}
		if err := applyOperationalRequest(&req, sc.DurationDays, sc.RecoverySpeed, sc.SubstituteAvailability, sc.InventoryBufferDays); err != nil {
			writeAPIError(w, http.StatusBadRequest, err.Error(), "check operational assumptions for each scenario")
			return
		}
		inputs = append(inputs, simulation.CompareScenario{Label: sc.Label, Request: req})
	}

	fused, ok := s.loadFused(w)
	if !ok {
		return
	}

	simCtx := fused.SimCtx
	cmp := simulation.CompareScenariosWithContext(fused.Dataset.Graph, cfg, inputs, &simCtx)
	out := buildCompareJSON(cmp, func(p simulation.ShockProfile, source, commodity string) []string {
		return shockWarnings(fused.Dataset.Graph, p, source, commodity)
	})
	writeJSONStatus(w, http.StatusOK, out)
}

func (s *apiServer) handleTradeSummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	resolved, ok := s.loadTrade(w)
	if !ok {
		return
	}
	writeJSONStatus(w, http.StatusOK, buildTradeSummaryJSON(resolved, trade.BuildSummary(resolved.File, 5)))
}

func (s *apiServer) handleTradeOptions(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	resolved, ok := s.loadTrade(w)
	if !ok {
		return
	}
	opts := trade.BuildTradeOptions(resolved)
	writeJSONStatus(w, http.StatusOK, buildTradeOptionsJSON(resolved, opts))
}

func (s *apiServer) handleTradeDependency(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	importer := strings.TrimSpace(r.URL.Query().Get("importer"))
	commodity := strings.TrimSpace(r.URL.Query().Get("commodity"))
	if importer == "" || commodity == "" {
		writeAPIError(w, http.StatusBadRequest, "importer and commodity query parameters are required",
			"example: /api/trade/dependency?importer=USA&commodity=semiconductors")
		return
	}
	resolved, ok := s.loadTrade(w)
	if !ok {
		return
	}
	dep := trade.BuildDependencyResolved(resolved, importer, commodity)
	if !dep.HasData {
		writeAPIError(w, http.StatusNotFound,
			"no trade flows for importer "+importer+" and commodity "+commodity, "")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildTradeDependencyJSON(resolved, dep))
}

func (s *apiServer) handleTradeConcentration(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	importer := strings.TrimSpace(r.URL.Query().Get("importer"))
	commodity := strings.TrimSpace(r.URL.Query().Get("commodity"))
	if importer == "" || commodity == "" {
		writeAPIError(w, http.StatusBadRequest, "importer and commodity query parameters are required",
			"example: /api/trade/concentration?importer=USA&commodity=semiconductors")
		return
	}
	resolved, ok := s.loadTrade(w)
	if !ok {
		return
	}
	con := trade.BuildConcentrationResolved(resolved, importer, commodity)
	if !con.HasData {
		writeAPIError(w, http.StatusNotFound,
			"no trade flows for importer "+importer+" and commodity "+commodity, "")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildTradeConcentrationJSON(resolved, con))
}

func (s *apiServer) handleMacroScores(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	if processed, ok := macro.TryLoadProcessed(s.cfg.ProcessedMacroData); ok {
		writeJSONStatus(w, http.StatusOK, processed)
		return
	}
	file, err := worldbank.Load(s.cfg.MacroData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas ingest macro --out data/processed/macro` or `atlas ingest worldbank --countries ...`")
		return
	}
	scores := macro.ScoreCountries(file, 0, macro.DefaultWeights())
	if len(scores) == 0 {
		writeAPIError(w, http.StatusInternalServerError,
			"no country data found in "+s.cfg.MacroData, "run `atlas ingest macro` or `atlas ingest worldbank` first")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildMacroJSON(scores, 0))
}

func (s *apiServer) handleEventsRisk(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	resolved, err := resolveEventRisk(s.cfg.ProcessedEventData, s.cfg.EventData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas ingest events --file <csv>` or `atlas ingest gdelt --fixture ...` and pass --processed-event-data / --event-data")
		return
	}
	country := strings.TrimSpace(r.URL.Query().Get("country"))
	resolved = resolved.withCountryFilter(country)
	if len(resolved.Scores) == 0 {
		writeAPIError(w, http.StatusInternalServerError,
			"no event data found", "run `atlas ingest events --file <csv>` or `atlas ingest gdelt --fixture ...` first")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildResolvedEventRiskJSON(resolved))
}

func (s *apiServer) handleCommodityStress(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	file, err := commodityprices.Load(s.cfg.CommodityData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas ingest commodity-prices --file <csv>` or pass an existing --commodity-data dir")
		return
	}
	scores := commodities.ScoreCommodities(file, commodities.DefaultWeights())
	if len(scores) == 0 {
		writeAPIError(w, http.StatusInternalServerError,
			"no commodity data found in "+s.cfg.CommodityData, "run `atlas ingest commodity-prices --file <csv>` first")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildCommodityStressJSON(scores, file.Source))
}

func (s *apiServer) handleCommodityHistory(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	file, err := commodityprices.Load(s.cfg.CommodityData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas ingest commodity-prices --file <csv|xlsx>` or pass an existing --commodity-data dir")
		return
	}
	if len(file.Records) == 0 {
		writeAPIError(w, http.StatusInternalServerError,
			"no commodity data found in "+s.cfg.CommodityData, "run `atlas ingest commodity-prices --file <csv|xlsx>` first")
		return
	}

	commodity := strings.TrimSpace(r.URL.Query().Get("commodity"))
	if commodity == "" {
		writeJSONStatus(w, http.StatusOK, commodityprices.ListHistoryCommodities(file))
		return
	}

	history, err := commodityprices.HistoryForCommodity(file, commodity)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err.Error(),
			"try GET /api/commodities/history for available commodities")
		return
	}
	writeJSONStatus(w, http.StatusOK, history)
}

func (s *apiServer) handleFragilityCountries(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	src, _ := s.fusedFragilitySources()
	res := fragility.Score(src)
	writeJSONStatus(w, http.StatusOK, buildFragilityJSON(res).Countries)
}

func (s *apiServer) handleFragilityCommodities(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	src, _ := s.fusedFragilitySources()
	res := fragility.Score(src)
	writeJSONStatus(w, http.StatusOK, buildFragilityJSON(res).Commodities)
}

func (s *apiServer) handleFragilitySummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	src, meta := s.fusedFragilitySources()
	if src.ProcessedMacro != nil && len(src.ProcessedMacro.Scores) > 0 {
		meta.DataSources = appendMacroDataSource(meta.DataSources)
	}
	res := fragility.Score(src)
	writeJSONStatus(w, http.StatusOK, buildFragilitySummaryJSON(res, 5, meta))
}

func (s *apiServer) fusedFragilitySources() (fragility.Sources, graphfusion.Meta) {
	return loadFusedFragilitySources(s.cfg.fusionConfig())
}

func (s *apiServer) fragilitySources() fragility.Sources {
	src, _ := s.fusedFragilitySources()
	return src
}

// loadTrade loads processed trade data, preferring trade_dependencies.json.
func (s *apiServer) loadTrade(w http.ResponseWriter) (trade.ResolvedTrade, bool) {
	resolved, err := trade.ResolveTrade(s.cfg.TradeData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas ingest trade --dir <dir> --source un-comtrade` or `atlas ingest trade --file <csv>` and pass --trade-data")
		return trade.ResolvedTrade{}, false
	}
	return resolved, true
}

// --- graph summary JSON ----------------------------------------------------

type jsonGraphNode struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Degree int    `json:"degree"`
	In     int    `json:"in_degree"`
	Out    int    `json:"out_degree"`
}

type jsonGraphSummary struct {
	Nodes        int             `json:"nodes"`
	Countries    int             `json:"countries"`
	Commodities  int             `json:"commodities"`
	Sectors      int             `json:"sectors"`
	Routes       int             `json:"routes"`
	Companies    int             `json:"companies"`
	Dependencies int             `json:"dependencies"`
	TopNodes     []jsonGraphNode `json:"top_nodes"`

	FusionEnabled       bool     `json:"fusion_enabled"`
	BaseEntities        int      `json:"base_entities"`
	BaseDependencies    int      `json:"base_dependencies"`
	FusedEntities       int      `json:"fused_entities"`
	FusedDependencies   int      `json:"fused_dependencies"`
	RealTradeEdges      int      `json:"real_trade_edges"`
	RealTradeEdgesUsed  bool     `json:"real_trade_edges_used"`
	RealEventRiskUsed   bool     `json:"real_event_risk_used"`
	RealPriceStressUsed bool     `json:"real_price_stress_used"`
	DataSources         []string `json:"data_sources"`
}

// buildGraphSummaryJSON mirrors the text `graph summary` view as JSON: entity
// counts plus the highest-degree nodes.
func buildGraphSummaryJSON(g *graph.Graph, top int, meta *graphfusion.Meta) jsonGraphSummary {
	out := jsonGraphSummary{
		Nodes:        g.NodeCount(),
		Countries:    g.CountByType(models.Country),
		Commodities:  g.CountByType(models.Commodity),
		Sectors:      g.CountByType(models.Sector),
		Routes:       g.CountByType(models.Route),
		Companies:    g.CountByType(models.Company),
		Dependencies: g.EdgeCount(),
		TopNodes:     []jsonGraphNode{},
	}

	nodes := g.Nodes()
	ranked := make([]jsonGraphNode, 0, len(nodes))
	for _, n := range nodes {
		ranked = append(ranked, jsonGraphNode{
			Name:   n.Name,
			Type:   string(n.Type),
			Degree: g.Degree(n.ID),
			In:     g.InDegree(n.ID),
			Out:    g.OutDegree(n.ID),
		})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Degree != ranked[j].Degree {
			return ranked[i].Degree > ranked[j].Degree
		}
		return ranked[i].Name < ranked[j].Name
	})
	if top > 0 && len(ranked) > top {
		ranked = ranked[:top]
	}
	out.TopNodes = ranked
	if meta != nil {
		out.FusionEnabled = meta.FusionEnabled
		out.BaseEntities = meta.BaseEntities
		out.BaseDependencies = meta.BaseDependencies
		out.FusedEntities = meta.FusedEntities
		out.FusedDependencies = meta.FusedDependencies
		out.RealTradeEdges = meta.RealTradeEdges
		out.RealTradeEdgesUsed = meta.RealTradeEdgesUsed
		out.RealEventRiskUsed = meta.RealEventRiskUsed
		out.RealPriceStressUsed = meta.RealPriceStressUsed
		out.DataSources = meta.DataSources
		if out.DataSources == nil {
			out.DataSources = []string{}
		}
	}
	return out
}

// --- graph entities --------------------------------------------------------

type jsonGraphEntities struct {
	Countries   []string `json:"countries"`
	Commodities []string `json:"commodities"`
	Sectors     []string `json:"sectors"`
	Routes      []string `json:"routes"`
	Companies   []string `json:"companies"`
}

// buildGraphEntitiesJSON groups every node's display name by node type, sorted
// for stable output. Empty groups are emitted as empty arrays (never null) so
// the frontend can rely on the shape.
func buildGraphEntitiesJSON(g *graph.Graph) jsonGraphEntities {
	out := jsonGraphEntities{
		Countries:   []string{},
		Commodities: []string{},
		Sectors:     []string{},
		Routes:      []string{},
		Companies:   []string{},
	}
	for _, n := range g.Nodes() { // already sorted by ID
		switch n.Type {
		case models.Country:
			out.Countries = append(out.Countries, n.Name)
		case models.Commodity:
			out.Commodities = append(out.Commodities, n.Name)
		case models.Sector:
			out.Sectors = append(out.Sectors, n.Name)
		case models.Route:
			out.Routes = append(out.Routes, n.Name)
		case models.Company:
			out.Companies = append(out.Companies, n.Name)
		}
	}
	sort.Strings(out.Countries)
	sort.Strings(out.Commodities)
	sort.Strings(out.Sectors)
	sort.Strings(out.Routes)
	sort.Strings(out.Companies)
	return out
}

// --- shock options ---------------------------------------------------------

type jsonShockTypeOption struct {
	Type           string   `json:"type"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	RecommendedFor []string `json:"recommended_for"`
	Requires       []string `json:"requires"`
}

type jsonRecommendedScenario struct {
	Label     string  `json:"label"`
	Source    string  `json:"source"`
	Commodity string  `json:"commodity"`
	ShockType string  `json:"shock_type"`
	Drop      float64 `json:"drop"`
	Depth     int     `json:"depth"`
}

type jsonShockOptions struct {
	Sources              []string                  `json:"sources"`
	Commodities          []string                  `json:"commodities"`
	ShockTypes           []jsonShockTypeOption     `json:"shock_types"`
	RecommendedScenarios []jsonRecommendedScenario `json:"recommended_scenarios"`
}

// shockTypeGuidance is static, human-facing guidance per shock type: what kind
// of relationship the shock travels and which relationship it most relies on.
var shockTypeGuidance = map[string]struct {
	recommendedFor []string
	requires       []string
}{
	string(models.ShockExportCollapse): {
		recommendedFor: []string{"country -> commodity export relationships"},
		requires:       []string{"exports"},
	},
	string(models.ShockSupplyCut): {
		recommendedFor: []string{"country -> commodity supply relationships"},
		requires:       []string{"exports", "supplies"},
	},
	string(models.ShockPriceSpike): {
		recommendedFor: []string{"commodity -> price-exposed sectors"},
		requires:       []string{"price_exposure"},
	},
	string(models.ShockRouteDisruption): {
		recommendedFor: []string{"route -> commodity chokepoint relationships"},
		requires:       []string{"route_exposure"},
	},
}

// candidateRecommendedScenarios are the spec's suggested scenarios. They are
// only surfaced when they actually make sense for the loaded graph.
var candidateRecommendedScenarios = []jsonRecommendedScenario{
	{Label: "Taiwan Semiconductor Export Collapse", Source: "Taiwan", Commodity: "semiconductors", ShockType: "export_collapse", Drop: 30, Depth: 3},
	{Label: "China Lithium Battery Supply Cut", Source: "China", Commodity: "lithium batteries", ShockType: "supply_cut", Drop: 35, Depth: 3},
	{Label: "Saudi Crude Oil Supply Cut", Source: "Saudi Arabia", Commodity: "crude oil", ShockType: "supply_cut", Drop: 25, Depth: 3},
}

func buildShockOptionsJSON(ds *data.Dataset) jsonShockOptions {
	g := ds.Graph
	out := jsonShockOptions{
		Sources:              shockSources(g),
		Commodities:          commodityNames(g),
		ShockTypes:           make([]jsonShockTypeOption, 0, len(simulation.AllProfiles())),
		RecommendedScenarios: []jsonRecommendedScenario{},
	}

	for _, p := range simulation.AllProfiles() {
		opt := jsonShockTypeOption{
			Type:           p.Type,
			Name:           p.Name,
			Description:    p.Description,
			RecommendedFor: []string{},
			Requires:       []string{},
		}
		if guide, ok := shockTypeGuidance[p.Type]; ok {
			if len(guide.recommendedFor) > 0 {
				opt.RecommendedFor = guide.recommendedFor
			}
			if len(guide.requires) > 0 {
				opt.Requires = guide.requires
			}
		}
		out.ShockTypes = append(out.ShockTypes, opt)
	}

	// Spec candidates first (so their friendly labels win), then the dataset's
	// own generated presets — all filtered to combinations valid for this graph.
	seen := map[string]bool{}
	add := func(rs jsonRecommendedScenario) {
		key := strings.ToLower(rs.Source + "|" + rs.Commodity + "|" + rs.ShockType)
		if seen[key] || !scenarioMakesSense(g, rs.Source, rs.Commodity, rs.ShockType) {
			return
		}
		seen[key] = true
		out.RecommendedScenarios = append(out.RecommendedScenarios, rs)
	}
	for _, rs := range candidateRecommendedScenarios {
		add(rs)
	}
	for _, sc := range data.SortScenarios(ds.Scenarios) {
		add(jsonRecommendedScenario{
			Label:     sc.Name,
			Source:    sc.Source,
			Commodity: sc.Commodity,
			ShockType: sc.ShockType,
			Drop:      sc.ShockPercent,
			Depth:     sc.Depth,
		})
	}
	return out
}

// shockSources returns the names of nodes that can originate a shock, i.e.
// non-commodity nodes with at least one outgoing edge into a commodity node.
func shockSources(g *graph.Graph) []string {
	var out []string
	seen := map[string]bool{}
	for _, n := range g.Nodes() {
		if n.Type == models.Commodity || n.Type == models.Sector {
			continue
		}
		for _, e := range g.OutEdges(n.ID) {
			to, ok := g.Node(e.To)
			if ok && to.Type == models.Commodity {
				if !seen[n.Name] {
					seen[n.Name] = true
					out = append(out, n.Name)
				}
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

func commodityNames(g *graph.Graph) []string {
	var out []string
	for _, n := range g.Nodes() {
		if n.Type == models.Commodity {
			out = append(out, n.Name)
		}
	}
	sort.Strings(out)
	return out
}

// scenarioMakesSense reports whether a (source, commodity, shockType) triple is
// runnable on the current graph: both endpoints exist, a direct edge links them,
// and route_disruption is only recommended when the graph actually has routes.
func scenarioMakesSense(g *graph.Graph, source, commodity, shockType string) bool {
	if shockType == string(models.ShockRouteDisruption) && g.CountByType(models.Route) == 0 {
		return false
	}
	src, ok := g.FindByName(source)
	if !ok {
		return false
	}
	com, ok := g.NodeByName(models.Commodity, commodity)
	if !ok {
		return false
	}
	_, ok = g.EdgeBetween(src.ID, com.ID)
	return ok
}

// --- shock warnings --------------------------------------------------------

// shockWarnings returns non-fatal advisories for a shock that ran successfully
// but may be a weak or unusual fit for the current graph. It never blocks a
// request; it only explains why results might be limited.
func shockWarnings(g *graph.Graph, profile simulation.ShockProfile, source, commodity string) []string {
	var w []string

	switch profile.Type {
	case string(models.ShockRouteDisruption):
		if g.CountByType(models.Route) == 0 {
			w = append(w, "route_disruption works best with route nodes, but the current graph has no routes.")
		}
	case string(models.ShockPriceSpike):
		if !graphHasRelationship(g, models.RelPriceExposure) {
			w = append(w, "price_spike works best with price_exposure relationships, but the current graph has none.")
		}
	}

	src, okS := g.FindByName(source)
	com, okC := g.NodeByName(models.Commodity, commodity)
	if okS && okC {
		rels := edgeTypesBetween(g, src.ID, com.ID)
		switch profile.Type {
		case string(models.ShockExportCollapse):
			if !rels[models.RelExports] {
				w = append(w, fmt.Sprintf("No direct exports edge found from %s to %s in this graph.", src.Name, com.Name))
			}
		case string(models.ShockSupplyCut):
			if !rels[models.RelExports] && !rels[models.RelSupplies] {
				w = append(w, fmt.Sprintf("No direct exports/supplies edge found from %s to %s in this graph.", src.Name, com.Name))
			}
		default:
			if len(rels) > 0 && !profileAllowsAny(profile, rels) {
				w = append(w, fmt.Sprintf("The %s shock does not travel along the link from %s to %s; results may be limited.", profile.Name, src.Name, com.Name))
			}
		}
	}
	return w
}

func edgeTypesBetween(g *graph.Graph, from, to models.NodeID) map[models.EdgeType]bool {
	set := map[models.EdgeType]bool{}
	for _, e := range g.OutEdges(from) {
		if e.To == to {
			set[e.Type] = true
		}
	}
	return set
}

func graphHasRelationship(g *graph.Graph, t models.EdgeType) bool {
	for _, n := range g.Nodes() {
		for _, e := range g.OutEdges(n.ID) {
			if e.Type == t {
				return true
			}
		}
	}
	return false
}

func profileAllowsAny(p simulation.ShockProfile, rels map[models.EdgeType]bool) bool {
	for r := range rels {
		if p.Allows(r) {
			return true
		}
	}
	return false
}
