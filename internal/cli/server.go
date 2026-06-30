package cli

import (
	"encoding/json"
	"net/http"
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

// serverConfig holds the data-source locations the HTTP API reads from. Each is
// loaded lazily, per request, so the server starts even when some are missing —
// only the affected endpoint then returns a helpful error.
type serverConfig struct {
	GraphData     string // dataset dir (entities/dependencies/scenarios); "" => embedded sample
	TradeData     string // ingested trade panel dir
	MacroData     string // ingested World Bank macro dir
	EventData     string // ingested GDELT event dir
	CommodityData string // ingested commodity price dir
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
}

// newAPIServer builds the HTTP handler for the AtlasGraph API. It is separated
// from the listening loop so it can be exercised directly in tests.
func newAPIServer(cfg serverConfig) http.Handler {
	s := &apiServer{cfg: cfg}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/graph/summary", s.handleGraphSummary)
	mux.HandleFunc("/api/scenarios", s.handleScenarios)
	mux.HandleFunc("/api/shock", s.handleShock)
	mux.HandleFunc("/api/trade/summary", s.handleTradeSummary)
	mux.HandleFunc("/api/trade/dependency", s.handleTradeDependency)
	mux.HandleFunc("/api/trade/concentration", s.handleTradeConcentration)
	mux.HandleFunc("/api/macro/scores", s.handleMacroScores)
	mux.HandleFunc("/api/events/risk", s.handleEventsRisk)
	mux.HandleFunc("/api/commodities/stress", s.handleCommodityStress)
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
	ds, err := loadDataset(s.cfg.GraphData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"build a graph with `atlas graph build-trade` or pass an existing --data dir")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildGraphSummaryJSON(ds.Graph, config.Default().TopN))
}

func (s *apiServer) handleScenarios(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	ds, err := loadDataset(s.cfg.GraphData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"build a graph with `atlas graph build-trade` or pass an existing --data dir")
		return
	}
	writeJSONStatus(w, http.StatusOK, map[string]any{
		"scenarios": data.SortScenarios(ds.Scenarios),
	})
}

// shockRequestBody is the POST /api/shock payload. Drop and Depth are pointers
// so omitted values fall back to engine defaults rather than 0.
type shockRequestBody struct {
	Source    string   `json:"source"`
	Commodity string   `json:"commodity"`
	Drop      *float64 `json:"drop"`
	Depth     *int     `json:"depth"`
	ShockType string   `json:"shock_type"`
	Explain   bool     `json:"explain"`
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

	ds, err := loadDataset(s.cfg.GraphData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"build a graph with `atlas graph build-trade` or pass an existing --data dir")
		return
	}

	res, err := simulation.Run(ds.Graph, cfg, req)
	if err != nil {
		// Run's failures are client-driven (unknown entity, bad ranges, …).
		writeAPIError(w, http.StatusBadRequest, err.Error(),
			"check source/commodity names and that source links to the commodity in this graph")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildJSONResult(res, nil, body.Explain))
}

func (s *apiServer) handleTradeSummary(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	file, ok := s.loadTrade(w)
	if !ok {
		return
	}
	writeJSONStatus(w, http.StatusOK, trade.BuildSummary(file, 5))
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
	file, ok := s.loadTrade(w)
	if !ok {
		return
	}
	dep := trade.BuildDependency(file, importer, commodity)
	if !dep.HasData {
		writeAPIError(w, http.StatusNotFound,
			"no trade flows for importer "+importer+" and commodity "+commodity, "")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildTradeDependencyJSON(dep))
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
	file, ok := s.loadTrade(w)
	if !ok {
		return
	}
	con := trade.BuildConcentration(file, importer, commodity)
	if !con.HasData {
		writeAPIError(w, http.StatusNotFound,
			"no trade flows for importer "+importer+" and commodity "+commodity, "")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildTradeConcentrationJSON(con))
}

func (s *apiServer) handleMacroScores(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	file, err := worldbank.Load(s.cfg.MacroData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas ingest worldbank --countries ...` or pass an existing --macro-data dir")
		return
	}
	scores := macro.ScoreCountries(file, 0, macro.DefaultWeights())
	if len(scores) == 0 {
		writeAPIError(w, http.StatusInternalServerError,
			"no country data found in "+s.cfg.MacroData, "run `atlas ingest worldbank --countries ...` first")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildMacroJSON(scores, 0))
}

func (s *apiServer) handleEventsRisk(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	file, err := gdelt.Load(s.cfg.EventData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas ingest gdelt ...` (or use --fixture) or pass an existing --event-data dir")
		return
	}
	scores := events.ScoreCountries(file, events.DefaultWeights())
	if len(scores) == 0 {
		writeAPIError(w, http.StatusInternalServerError,
			"no event data found in "+s.cfg.EventData, "run `atlas ingest gdelt ...` first")
		return
	}
	writeJSONStatus(w, http.StatusOK, buildEventRiskJSON(scores))
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
	writeJSONStatus(w, http.StatusOK, buildCommodityStressJSON(scores))
}

// loadTrade loads the configured trade panel, writing a JSON error and
// returning ok=false on failure.
func (s *apiServer) loadTrade(w http.ResponseWriter) (trade.TradeFile, bool) {
	file, err := trade.Load(s.cfg.TradeData)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err.Error(),
			"run `atlas ingest trade --file <csv>` or pass an existing --trade-data dir")
		return trade.TradeFile{}, false
	}
	return file, true
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
}

// buildGraphSummaryJSON mirrors the text `graph summary` view as JSON: entity
// counts plus the highest-degree nodes.
func buildGraphSummaryJSON(g *graph.Graph, top int) jsonGraphSummary {
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
	return out
}
