package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
	"github.com/atlasgraph/atlas/internal/simulation"
)

const ruleWidth = 64

// renderScenarioBanner prints preset metadata ahead of a scenario's report.
func renderScenarioBanner(out io.Writer, scen data.Scenario) {
	section(out, "SCENARIO PRESET")
	fmt.Fprintf(out, "  ID         : %s\n", scen.ID)
	fmt.Fprintf(out, "  Name       : %s\n", scen.Name)
	fmt.Fprintf(out, "  Shock type : %s\n", scen.ShockType)
	if scen.Description != "" {
		fmt.Fprintf(out, "  Summary    : %s\n", scen.Description)
	}
}

// renderResult prints a clean, sectioned report of a shock simulation. When
// explain is set, it also prints the propagation logic the rules applied.
func renderResult(out io.Writer, g *graph.Graph, res simulation.Result, explain bool) {
	section(out, "SCENARIO")
	fmt.Fprintf(out, "  Source           : %s (%s)\n", res.SourceNode.Name, res.SourceNode.Type)
	fmt.Fprintf(out, "  Commodity        : %s\n", res.CommodityNode.Name)
	fmt.Fprintf(out, "  Shock type       : %s (%s)\n", res.Profile.Type, res.Profile.Name)
	fmt.Fprintf(out, "  Flow drop        : %.0f%%\n", res.Request.DropPct)
	fmt.Fprintf(out, "  Propagation depth: %d hops\n", res.Request.Depth)
	fmt.Fprintf(out, "  Initial impact   : %s  (flow drop x supplier share)\n", pct(res.InitialImpact))

	if explain {
		renderPropagationLogic(out, res)
	}

	section(out, "DIRECT EXPOSURE")
	if len(res.Direct) == 0 {
		fmt.Fprintln(out, "  (none within the requested depth)")
	} else {
		renderImpactTable(out, res.Direct)
	}

	section(out, "SECOND-ORDER EXPOSURE")
	if len(res.SecondOrder) == 0 {
		fmt.Fprintln(out, "  (none within the requested depth)")
	} else {
		renderImpactTable(out, res.SecondOrder)
	}

	section(out, "AFFECTED DEPENDENCY PATHS")
	if len(res.Paths) == 0 {
		fmt.Fprintln(out, "  (no downstream paths within the requested depth)")
	} else {
		for _, p := range res.Paths {
			fmt.Fprintf(out, "  %s   [impact %s, path weight %.2f]\n",
				joinLabeledPath(p), pct(p.EndImpact), p.PathWeight)
		}
	}

	section(out, "CHANGED FRAGILITY SCORES")
	renderFragilityTable(out, res.AllAffected)

	section(out, "HIGHEST-RISK ENTITIES")
	renderTop(out, "Countries", res.TopCountries)
	renderTop(out, "Commodities", res.TopCommodities)
	renderTop(out, "Sectors", res.TopSectors)

	section(out, "GRAPH IMPACT SUMMARY")
	renderSummary(out, g, res)
}

// renderPropagationLogic explains how the shock's profile and the rules shaped
// propagation: which relationships were allowed, whether cross-commodity jumps
// were permitted, and which unrelated commodity branches were blocked.
func renderPropagationLogic(out io.Writer, res simulation.Result) {
	section(out, "PROPAGATION LOGIC")
	fmt.Fprintf(out, "  Shock type                 : %s\n", res.Profile.Type)
	fmt.Fprintf(out, "  Allowed relationships      : %s\n", strings.Join(res.Profile.AllowedRelationshipStrings(), ", "))
	fmt.Fprintf(out, "  Per-hop attenuation        : %.2f\n", res.Profile.Attenuation)
	fmt.Fprintf(out, "  Cross-commodity propagation: %s\n", enabledDisabled(res.Profile.CrossCommodity))
	if blocked := res.BlockedCommodities(); len(blocked) > 0 {
		fmt.Fprintf(out, "  Blocked unrelated branches : %s\n", strings.Join(blocked, ", "))
	} else {
		fmt.Fprintln(out, "  Blocked unrelated branches : (none)")
	}
	if len(res.BlockedEdges) > 0 {
		fmt.Fprintln(out, "  Blocked edges:")
		for _, b := range res.BlockedEdges {
			fmt.Fprintf(out, "    %s --%s--> %s   [%s]\n", b.From.Name, b.Relationship, b.To.Name, b.Reason)
		}
	}
}

func enabledDisabled(v bool) string {
	if v {
		return "enabled"
	}
	return "disabled"
}

func renderImpactTable(out io.Writer, items []simulation.NodeImpact) {
	tw := newTable(out)
	fmt.Fprintln(tw, "  ENTITY\tTYPE\tIMPACT\tFRAGILITY (BASE -> SHOCK)")
	for _, ni := range items {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%.1f -> %.1f  (+%.1f)\n",
			ni.Node.Name, ni.Node.Type, pct(ni.Impact),
			ni.BaseFragility, ni.ShockFragility, ni.Delta)
	}
	flush(tw)
}

func renderFragilityTable(out io.Writer, items []simulation.NodeImpact) {
	if len(items) == 0 {
		fmt.Fprintln(out, "  (no fragility changes)")
		return
	}
	tw := newTable(out)
	fmt.Fprintln(tw, "  ENTITY\tTYPE\tDIST\tBASE\tSHOCK\tDELTA")
	for _, ni := range items {
		fmt.Fprintf(tw, "  %s\t%s\t%d\t%.1f\t%.1f\t+%.1f\n",
			ni.Node.Name, ni.Node.Type, ni.Distance,
			ni.BaseFragility, ni.ShockFragility, ni.Delta)
	}
	flush(tw)
}

func renderTop(out io.Writer, label string, items []simulation.NodeImpact) {
	fmt.Fprintf(out, "  %s:\n", label)
	if len(items) == 0 {
		fmt.Fprintln(out, "    (none)")
		return
	}
	for i, ni := range items {
		fmt.Fprintf(out, "    %d. %-22s fragility %.1f  (+%.1f)\n",
			i+1, ni.Node.Name, ni.ShockFragility, ni.Delta)
	}
}

func renderSummary(out io.Writer, g *graph.Graph, res simulation.Result) {
	var countries, commodities, sectors int
	var sumDelta, maxDelta float64
	var maxNode string
	for _, ni := range res.AllAffected {
		switch ni.Node.Type {
		case models.Country:
			countries++
		case models.Commodity:
			commodities++
		case models.Sector:
			sectors++
		}
		sumDelta += ni.Delta
		if ni.Delta > maxDelta {
			maxDelta = ni.Delta
			maxNode = ni.Node.Name
		}
	}
	avg := 0.0
	if len(res.AllAffected) > 0 {
		avg = sumDelta / float64(len(res.AllAffected))
	}
	fmt.Fprintf(out, "  Nodes in graph        : %d\n", g.NodeCount())
	fmt.Fprintf(out, "  Affected nodes        : %d  (countries %d, commodities %d, sectors %d)\n",
		len(res.AllAffected), countries, commodities, sectors)
	fmt.Fprintf(out, "  Affected paths        : %d\n", len(res.Paths))
	fmt.Fprintf(out, "  Avg fragility delta   : +%.1f\n", avg)
	if maxNode != "" {
		fmt.Fprintf(out, "  Largest single impact : %s (+%.1f fragility)\n", maxNode, maxDelta)
	}
}

// --- world bank indicators -------------------------------------------------

func renderCountryIndicators(out io.Writer, s worldbank.Summary) {
	section(out, "COUNTRY INDICATORS")
	name := s.CountryName
	if name == "" {
		name = "(unknown)"
	}
	fmt.Fprintf(out, "  Country               : %s (%s)\n", name, s.CountryCode)
	if s.LatestYear > 0 {
		fmt.Fprintf(out, "  Latest year with data : %d\n\n", s.LatestYear)
	} else {
		fmt.Fprint(out, "  Latest year with data : (none)\n\n")
	}

	tw := newTable(out)
	fmt.Fprintln(tw, "  INDICATOR\tYEAR\tVALUE")
	for _, line := range s.Lines {
		year := "-"
		if line.Year > 0 {
			year = fmt.Sprintf("%d", line.Year)
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", line.IndicatorName, year, formatIndicatorValue(line.IndicatorCode, line.Value))
	}
	flush(tw)
}

// formatIndicatorValue renders a value appropriately for its indicator:
// currency series (codes ending in "CD") get grouped digits with a US$ marker,
// everything else is treated as a percentage.
func formatIndicatorValue(code string, v *float64) string {
	if v == nil {
		return "n/a"
	}
	if strings.HasSuffix(code, "CD") {
		return "US$ " + groupThousands(*v)
	}
	return fmt.Sprintf("%.2f%%", *v)
}

// groupThousands formats a float's integer part with comma separators, e.g.
// 27360935000000 -> "27,360,935,000,000".
func groupThousands(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	digits := fmt.Sprintf("%.0f", v)
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	n := len(digits)
	for i, d := range digits {
		if i > 0 && (n-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	return b.String()
}

// --- macro exposure scores -------------------------------------------------

func renderMacroScores(out io.Writer, scores []macro.CountryScore, yearLens int, verbose bool) {
	section(out, "MACRO EXPOSURE SCORES")
	if yearLens > 0 {
		fmt.Fprintf(out, "  Year lens: %d (latest available <= %d per indicator)\n\n", yearLens, yearLens)
	} else {
		fmt.Fprint(out, "  Year lens: latest available per indicator\n\n")
	}

	tw := newTable(out)
	fmt.Fprintln(tw, "  COUNTRY\tYEAR\tSCORE\tRISK\tTOP DRIVERS")
	for _, s := range scores {
		fmt.Fprintf(tw, "  %s\t%s\t%.1f\t%s\t%s\n",
			s.CountryName, yearLabel(s.Year), s.Score, s.RiskLevel, strings.Join(s.TopDrivers, ", "))
	}
	flush(tw)

	fmt.Fprint(out, "\n  Risk bands: Low 0-30 | Medium 30-60 | High 60-80 | Critical 80-100\n")

	if verbose {
		for _, s := range scores {
			renderMacroDetail(out, s)
		}
	}
}

func renderMacroDetail(out io.Writer, s macro.CountryScore) {
	section(out, fmt.Sprintf("%s (%s) — %.1f %s", s.CountryName, s.CountryCode, s.Score, s.RiskLevel))
	tw := newTable(out)
	fmt.Fprintln(tw, "  COMPONENT\tSCORE\tWEIGHT\tCONTRIBUTION\tYEAR")
	for _, c := range s.Components {
		if !c.Available {
			fmt.Fprintf(tw, "  %s\t(no data)\t%.2f\t-\t-\n", c.Name, c.Weight)
			continue
		}
		fmt.Fprintf(tw, "  %s\t%.1f\t%.2f\t%.2f\t%s\n",
			c.Name, c.Score, c.Weight, c.Contribution, yearLabel(c.YearUsed))
	}
	flush(tw)
}

func yearLabel(y int) string {
	if y <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", y)
}

// renderMacroFormula documents exactly how the Macro Exposure Score is built:
// its weighted formula, what each component measures, the risk bands, and an
// explicit statement of what the score is and is not.
func renderMacroFormula(out io.Writer, w macro.Weights) {
	section(out, "MACRO EXPOSURE SCORE — FORMULA")
	fmt.Fprintf(out, "  Score name: Macro Exposure Score\n\n")

	fmt.Fprintln(out, "  Formula weights:")
	fmt.Fprintf(out, "      %.2f * trade_exposure_score\n", w.Trade)
	fmt.Fprintf(out, "    + %.2f * manufacturing_dependency_score\n", w.Manufacturing)
	fmt.Fprintf(out, "    + %.2f * inflation_stress_score\n", w.Inflation)
	fmt.Fprintf(out, "    + %.2f * high_tech_concentration_score\n", w.HighTech)
	fmt.Fprintf(out, "    + %.2f * economic_buffer_risk_score\n\n", w.BufferRisk)

	fmt.Fprintln(out, "  Component definitions:")
	fmt.Fprintln(out, "    trade_exposure_score           = imports % GDP + exports % GDP exposure")
	fmt.Fprintln(out, "    manufacturing_dependency_score = manufacturing value added % GDP exposure")
	fmt.Fprintln(out, "    inflation_stress_score         = inflation pressure")
	fmt.Fprintln(out, "    high_tech_concentration_score  = high-tech exports relative to GDP")
	fmt.Fprintln(out, "    economic_buffer_risk_score     = inverse GDP-size buffer risk")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "  Risk bands:")
	fmt.Fprintln(out, "    Low      : 0-30")
	fmt.Fprintln(out, "    Medium   : 30-60")
	fmt.Fprintln(out, "    High     : 60-80")
	fmt.Fprintln(out, "    Critical : 80-100")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "  Note:")
	fmt.Fprintln(out, "    This is an exposure-oriented macro score, not a prediction of recession,")
	fmt.Fprintln(out, "    crisis, default, or stock-market performance. Full AtlasGraph fragility")
	fmt.Fprintln(out, "    scoring will later combine macro exposure with graph dependency, trade")
	fmt.Fprintln(out, "    concentration, event risk, and commodity volatility.")
}

// --- small formatting helpers ---------------------------------------------

func section(out io.Writer, title string) {
	fmt.Fprintf(out, "\n%s\n%s\n", title, strings.Repeat("-", ruleWidth))
}

func pct(v float64) string { return fmt.Sprintf("%.0f%%", v*100) }

func joinPath(nodes []models.Node) string {
	names := make([]string, len(nodes))
	for i, n := range nodes {
		names[i] = n.Name
	}
	return strings.Join(names, " -> ")
}

// joinLabeledPath renders a path with the relationship (and commodity, when
// present) on each hop, e.g.
//
//	Taiwan --exports/semiconductors--> semiconductors --imports/semiconductors--> United States
func joinLabeledPath(p simulation.Path) string {
	if len(p.Nodes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(p.Nodes[0].Name)
	for i, e := range p.Edges {
		label := e.Relationship
		if e.Commodity != "" {
			label += "/" + e.Commodity
		}
		fmt.Fprintf(&b, " --%s--> %s", label, p.Nodes[i+1].Name)
	}
	return b.String()
}

func newTable(out io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
}

func flush(tw *tabwriter.Writer) { _ = tw.Flush() }

// --- scenario list ---------------------------------------------------------

func renderScenarioList(out io.Writer, scenarios []data.Scenario) {
	section(out, "SCENARIO PRESETS")
	if len(scenarios) == 0 {
		fmt.Fprintln(out, "  (no scenarios defined)")
		return
	}
	tw := newTable(out)
	fmt.Fprintln(tw, "  ID\tNAME\tSOURCE\tCOMMODITY\tSHOCK\tDROP\tDEPTH")
	for _, s := range scenarios {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\t%.0f%%\t%d\n",
			s.ID, s.Name, s.Source, s.Commodity, s.ShockType, s.ShockPercent, s.Depth)
	}
	flush(tw)
	fmt.Fprintf(out, "\nRun one with: atlas scenario run <id>\n")
}

// --- graph summary ---------------------------------------------------------

func renderGraphSummary(out io.Writer, g *graph.Graph, top int) {
	section(out, "GRAPH SUMMARY")
	tw := newTable(out)
	fmt.Fprintf(tw, "  Total entities\t%d\n", g.NodeCount())
	fmt.Fprintf(tw, "  Countries\t%d\n", g.CountByType(models.Country))
	fmt.Fprintf(tw, "  Commodities\t%d\n", g.CountByType(models.Commodity))
	fmt.Fprintf(tw, "  Sectors\t%d\n", g.CountByType(models.Sector))
	fmt.Fprintf(tw, "  Routes\t%d\n", g.CountByType(models.Route))
	fmt.Fprintf(tw, "  Companies\t%d\n", g.CountByType(models.Company))
	fmt.Fprintf(tw, "  Dependencies\t%d\n", g.EdgeCount())
	flush(tw)

	section(out, "HIGHEST-DEGREE NODES")
	type deg struct {
		node     models.Node
		in, out  int
		combined int
	}
	var ds []deg
	for _, n := range g.Nodes() {
		ds = append(ds, deg{node: n, in: g.InDegree(n.ID), out: g.OutDegree(n.ID), combined: g.Degree(n.ID)})
	}
	sort.SliceStable(ds, func(i, j int) bool {
		if ds[i].combined != ds[j].combined {
			return ds[i].combined > ds[j].combined
		}
		return ds[i].node.Name < ds[j].node.Name
	})
	if top > 0 && len(ds) > top {
		ds = ds[:top]
	}
	tw = newTable(out)
	fmt.Fprintln(tw, "  ENTITY\tTYPE\tDEGREE\tIN\tOUT")
	for _, d := range ds {
		fmt.Fprintf(tw, "  %s\t%s\t%d\t%d\t%d\n", d.node.Name, d.node.Type, d.combined, d.in, d.out)
	}
	flush(tw)
}

// --- graph paths -----------------------------------------------------------

func renderGraphPaths(out io.Writer, g *graph.Graph, from, to models.Node, depth int, paths [][]models.NodeID) {
	section(out, "DEPENDENCY PATHS")
	fmt.Fprintf(out, "  From : %s (%s)\n", from.Name, from.Type)
	fmt.Fprintf(out, "  To   : %s (%s)\n", to.Name, to.Type)
	fmt.Fprintf(out, "  Depth: up to %d hops\n\n", depth)
	if len(paths) == 0 {
		fmt.Fprintf(out, "  No dependency paths found within %d hops.\n", depth)
		return
	}
	for _, p := range paths {
		nodes := make([]models.Node, len(p))
		weight := 1.0
		for i, id := range p {
			n, _ := g.Node(id)
			nodes[i] = n
			if i > 0 {
				if e, ok := g.EdgeBetween(p[i-1], id); ok {
					weight *= e.Weight
				}
			}
		}
		fmt.Fprintf(out, "  %s   [%d hops, path weight %.2f]\n", joinPath(nodes), len(p)-1, weight)
	}
	fmt.Fprintf(out, "\n  %d path(s) found.\n", len(paths))
}

// --- risk leaderboard ------------------------------------------------------

func renderRiskLeaderboard(out io.Writer, countries, commodities, sectors []rankedEntity) {
	section(out, "RISK LEADERBOARD (baseline fragility)")
	renderRankedBoard(out, "Countries", countries)
	renderRankedBoard(out, "Commodities", commodities)
	renderRankedBoard(out, "Sectors", sectors)
}

func renderRankedBoard(out io.Writer, label string, items []rankedEntity) {
	fmt.Fprintf(out, "  %s:\n", label)
	if len(items) == 0 {
		fmt.Fprintln(out, "    (none)")
		return
	}
	for i, r := range items {
		fmt.Fprintf(out, "    %d. %-24s fragility %.1f\n", i+1, r.Node.Name, r.Fragility)
	}
}
