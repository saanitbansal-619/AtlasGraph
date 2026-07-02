package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/atlasgraph/atlas/internal/data"
	"github.com/atlasgraph/atlas/internal/graph"
	"github.com/atlasgraph/atlas/internal/ingest/commodityprices"
	"github.com/atlasgraph/atlas/internal/ingest/gdelt"
	"github.com/atlasgraph/atlas/internal/ingest/trade"
	"github.com/atlasgraph/atlas/internal/ingest/worldbank"
	"github.com/atlasgraph/atlas/internal/models"
	"github.com/atlasgraph/atlas/internal/scoring/commodities"
	"github.com/atlasgraph/atlas/internal/scoring/events"
	"github.com/atlasgraph/atlas/internal/scoring/fragility"
	"github.com/atlasgraph/atlas/internal/scoring/macro"
	"github.com/atlasgraph/atlas/internal/simulation"
	"github.com/atlasgraph/atlas/internal/tradegraph"
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

// --- trade flows -----------------------------------------------------------

func renderTradeIngestReport(out io.Writer, srcFile, outPath string, res trade.LoadResult, s trade.Summary) {
	section(out, "TRADE INGESTION")
	fmt.Fprintf(out, "  Source file       : %s\n", srcFile)
	fmt.Fprintf(out, "  Output            : %s\n", outPath)
	fmt.Fprintf(out, "  Total rows        : %d\n", res.TotalRows)
	fmt.Fprintf(out, "  Valid rows        : %d\n", res.ValidRows())
	fmt.Fprintf(out, "  Skipped rows      : %d\n", len(res.Skipped))
	for _, sk := range res.Skipped {
		fmt.Fprintf(out, "    - line %d: %s\n", sk.Line, sk.Reason)
	}
	fmt.Fprintf(out, "  Countries detected: %d\n", s.Countries)
	fmt.Fprintf(out, "  Commodities       : %d\n", s.Commodities)
	fmt.Fprintf(out, "  Total trade value : %s\n", usdShort(s.TotalValueUSD))
}

func renderComtradeIngestReport(out io.Writer, srcFile, outPath string, res trade.ComtradeLoadResult, s trade.Summary) {
	section(out, "COMTRADE TRADE INGESTION")
	fmt.Fprintf(out, "  Source file       : %s\n", srcFile)
	fmt.Fprintf(out, "  Output            : %s\n", outPath)
	fmt.Fprintf(out, "  Total rows        : %d\n", res.TotalRows)
	fmt.Fprintf(out, "  Valid rows        : %d\n", res.ValidRows())
	fmt.Fprintf(out, "  Skipped rows      : %d\n", len(res.Skipped))
	for _, sk := range res.Skipped {
		fmt.Fprintf(out, "    - line %d: %s\n", sk.Line, sk.Reason)
	}
	fmt.Fprintf(out, "  Flows imported    : %d\n", res.FlowsImported)
	fmt.Fprintf(out, "  Flows exported    : %d\n", res.FlowsExported)
	fmt.Fprintf(out, "  Countries detected: %d\n", s.Countries)
	fmt.Fprintf(out, "  Commodities       : %d\n", s.Commodities)
	fmt.Fprintf(out, "  Total trade value : %s\n", usdShort(s.TotalValueUSD))
}

// --- GDELT event risk ------------------------------------------------------

// renderGDELTLiveReport prints the live-ingestion report: what was requested,
// the per-country success/failure split, how many records were fetched, and the
// leading countries and risk terms.
func renderGDELTLiveReport(out io.Writer, requested []string, days, limit, delay int, res gdelt.FetchResult, outPath string, s gdelt.Summary) {
	section(out, "GDELT EVENT INGESTION")
	fmt.Fprintf(out, "  Countries requested    : %s\n", strings.Join(requested, ", "))
	fmt.Fprintf(out, "  Days                   : %d\n", days)
	fmt.Fprintf(out, "  Limit per country      : %d\n", limit)
	fmt.Fprintf(out, "  Delay seconds          : %d\n", delay)
	fmt.Fprintf(out, "  Countries succeeded    : %s\n", joinOrNone(res.Succeeded))
	fmt.Fprintf(out, "  Countries failed       : %s\n", joinFailed(res.Failed))
	fmt.Fprintf(out, "  Records fetched        : %d\n", s.Records)
	fmt.Fprintf(out, "  Records with risk terms: %d\n", s.WithRiskTerms)
	fmt.Fprintf(out, "  Output                 : %s\n", outPath)

	renderGDELTLeaderboards(out, s)
}

// renderGDELTFixtureReport prints the offline fixture-ingestion report. It is
// clearly labelled FIXTURE MODE so synthetic demo data is never confused with a
// live API pull.
func renderGDELTFixtureReport(out io.Writer, fixturePath, outPath string, countries []string, s gdelt.Summary) {
	section(out, "GDELT EVENT INGESTION — FIXTURE MODE")
	fmt.Fprintf(out, "  Source fixture         : %s\n", fixturePath)
	fmt.Fprintf(out, "  Output                 : %s\n", outPath)
	fmt.Fprintf(out, "  Records loaded         : %d\n", s.Records)
	fmt.Fprintf(out, "  Countries              : %s\n", joinOrNone(countries))
	fmt.Fprintf(out, "  Records with risk terms: %d\n", s.WithRiskTerms)

	renderGDELTLeaderboards(out, s)
	fmt.Fprint(out, "\n  Note: synthetic, reproducible demo data — not real GDELT output.\n")
}

func renderGDELTLeaderboards(out io.Writer, s gdelt.Summary) {
	fmt.Fprintln(out, "\n  Top countries by event count:")
	if len(s.TopCountries) == 0 {
		fmt.Fprintln(out, "    (none)")
	} else {
		for i, nc := range s.TopCountries {
			fmt.Fprintf(out, "    %d. %-32s %d\n", i+1, nc.Name, nc.Count)
		}
	}

	fmt.Fprintln(out, "\n  Top matched risk terms:")
	if len(s.TopRiskTerms) == 0 {
		fmt.Fprintln(out, "    (none)")
	} else {
		for i, nc := range s.TopRiskTerms {
			fmt.Fprintf(out, "    %d. %-32s %d\n", i+1, nc.Name, nc.Count)
		}
	}
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "(none)"
	}
	return strings.Join(items, ", ")
}

func joinFailed(failed []gdelt.FailedCountry) string {
	if len(failed) == 0 {
		return "(none)"
	}
	codes := make([]string, len(failed))
	for i, f := range failed {
		codes[i] = f.Code
	}
	return strings.Join(codes, ", ")
}

func renderEventRiskScores(out io.Writer, scores []events.CountryScore) {
	section(out, "EVENT RISK SCORES")
	tw := newTable(out)
	fmt.Fprintln(tw, "  COUNTRY\tEVENTS\tAVG TONE\tSCORE\tRISK\tTOP TERMS")
	for _, s := range scores {
		fmt.Fprintf(tw, "  %s\t%d\t%.1f\t%.1f\t%s\t%s\n",
			s.CountryName, s.Events, s.AvgTone, s.Score, s.RiskLevel, strings.Join(s.TopTerms, ", "))
	}
	flush(tw)
	fmt.Fprint(out, "\n  Risk bands: Low 0-30 | Medium 30-60 | High 60-80 | Critical 80-100\n")
	fmt.Fprint(out, "  Note: a public event-risk signal from global news, not ground truth.\n")
}

func renderTradeSummary(out io.Writer, s trade.Summary) {
	section(out, "TRADE FLOW SUMMARY")
	fmt.Fprintf(out, "  Records          : %d\n", s.Records)
	fmt.Fprintf(out, "  Years            : %s\n", yearsLabel(s.Years))
	fmt.Fprintf(out, "  Countries        : %d\n", s.Countries)
	fmt.Fprintf(out, "  Commodities      : %d\n", s.Commodities)
	fmt.Fprintf(out, "  Total trade value: %s\n", usdShort(s.TotalValueUSD))

	renderTradeLeaderboard(out, "Top exporters", s.TopExporters)
	renderTradeLeaderboard(out, "Top importers", s.TopImporters)
	renderTradeLeaderboard(out, "Top commodities", s.TopCommodities)
}

func renderTradeLeaderboard(out io.Writer, label string, items []trade.NamedValue) {
	fmt.Fprintf(out, "\n  %s:\n", label)
	if len(items) == 0 {
		fmt.Fprintln(out, "    (none)")
		return
	}
	for i, nv := range items {
		name := nv.Name
		if name == "" {
			name = nv.Code
		}
		fmt.Fprintf(out, "    %d. %-22s %s\n", i+1, name, usdShort(nv.Value))
	}
}

func renderTradeDependency(out io.Writer, d trade.Dependency) {
	section(out, "SUPPLIER DEPENDENCY")
	fmt.Fprintf(out, "  Importer     : %s\n", labelOrCode(d.ImporterName, d.ImporterCode))
	fmt.Fprintf(out, "  Commodity    : %s\n", d.Commodity)
	fmt.Fprintf(out, "  Total imports: %s\n\n", usdShort(d.TotalImportsUSD))

	tw := newTable(out)
	fmt.Fprintln(tw, "  SUPPLIER\tVALUE\tSHARE\tDEPENDENCY")
	for _, sup := range d.Suppliers {
		fmt.Fprintf(tw, "  %s\t%s\t%.1f%%\t%s\n",
			labelOrCode(sup.ExporterName, sup.ExporterCode), usdShort(sup.ValueUSD), sup.Share*100, sup.Dependency)
	}
	flush(tw)
	fmt.Fprint(out, "\n  Dependency bands: Low <10% | Medium 10-40% | High >=40% (single-supplier share)\n")
}

func renderTradeConcentration(out io.Writer, c trade.Concentration) {
	section(out, "SUPPLIER CONCENTRATION")
	fmt.Fprintf(out, "  Importer          : %s\n", labelOrCode(c.ImporterName, c.ImporterCode))
	fmt.Fprintf(out, "  Commodity         : %s\n", c.Commodity)
	fmt.Fprintf(out, "  HHI               : %.2f\n", c.HHI)
	fmt.Fprintf(out, "  Concentration risk: %s\n", c.RiskLevel)
	fmt.Fprintf(out, "  Top supplier      : %s, %.1f%%\n",
		labelOrCode(c.TopSupplier.ExporterName, c.TopSupplier.ExporterCode), c.TopSupplier.Share*100)
	fmt.Fprint(out, "\n  Risk bands: HHI < 0.15 Low | 0.15-0.25 Medium | > 0.25 High\n")
}

// usdShort renders a dollar amount compactly, e.g. 85000000000 -> "US$ 85.0B".
func usdShort(v float64) string {
	abs := v
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 1e12:
		return fmt.Sprintf("US$ %.1fT", v/1e12)
	case abs >= 1e9:
		return fmt.Sprintf("US$ %.1fB", v/1e9)
	case abs >= 1e6:
		return fmt.Sprintf("US$ %.1fM", v/1e6)
	case abs >= 1e3:
		return fmt.Sprintf("US$ %.1fK", v/1e3)
	default:
		return fmt.Sprintf("US$ %.0f", v)
	}
}

// yearsLabel renders a sorted year set as a single year, a contiguous range, or
// a comma-separated list.
func yearsLabel(years []int) string {
	if len(years) == 0 {
		return "-"
	}
	if len(years) == 1 {
		return fmt.Sprintf("%d", years[0])
	}
	lo, hi := years[0], years[len(years)-1]
	if hi-lo+1 == len(years) {
		return fmt.Sprintf("%d-%d", lo, hi)
	}
	parts := make([]string, len(years))
	for i, y := range years {
		parts[i] = fmt.Sprintf("%d", y)
	}
	return strings.Join(parts, ", ")
}

func labelOrCode(name, code string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return code
}

// --- commodity prices ------------------------------------------------------

func renderCommodityIngestReport(out io.Writer, srcFile, outPath string, res commodityprices.LoadResult, s commodityprices.Summary, sourceName string, meta commodityprices.PinkSheetMeta) {
	section(out, "COMMODITY PRICE INGESTION")
	fmt.Fprintf(out, "  Source file  : %s\n", srcFile)
	fmt.Fprintf(out, "  Data source  : %s\n", sourceName)
	fmt.Fprintf(out, "  Output       : %s\n", outPath)
	fmt.Fprintf(out, "  Rows         : %d\n", res.TotalRows)
	fmt.Fprintf(out, "  Valid rows   : %d\n", res.ValidRows())
	fmt.Fprintf(out, "  Skipped rows : %d\n", len(res.Skipped))
	for _, sk := range res.Skipped {
		fmt.Fprintf(out, "    - line %d: %s\n", sk.Line, sk.Reason)
	}
	fmt.Fprintf(out, "  Commodities  : %d\n", s.Commodities)
	fmt.Fprintf(out, "  Date range   : %s\n", monthRange(s.FirstMonth, s.LastMonth))
	fmt.Fprintf(out, "  Latest month : %s\n", monthOrDash(s.LastMonth))
	if meta.SheetName != "" {
		fmt.Fprintf(out, "  Pink Sheet   : %s (%d mapped series)\n", meta.SheetName, meta.MappedSeries)
	}
	if len(meta.MissingGFIP) > 0 {
		fmt.Fprintf(out, "  Missing GFIP : %s\n", strings.Join(meta.MissingGFIP, ", "))
	}
	if commodityprices.IsRealPriceSource(sourceName) {
		fmt.Fprint(out, "\n  Note: real public monthly historical prices from World Bank Pink Sheet — not live streaming data.\n")
	} else {
		fmt.Fprint(out, "\n  Note: the bundled sample is synthetic, reproducible demo data — not real prices.\n")
	}
}

func renderCommodityStressScores(out io.Writer, scores []commodities.CommodityScore) {
	section(out, "COMMODITY STRESS SCORES")
	tw := newTable(out)
	fmt.Fprintln(tw, "  COMMODITY\tLATEST PRICE\t3M CHANGE\t12M CHANGE\tVOLATILITY\tSCORE\tRISK")
	for _, s := range scores {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%.1f%%\t%.1f\t%s\n",
			s.CommodityName,
			priceLabel(s.LatestPrice, s.Unit),
			changeLabel(s.Change3M, s.Change3MAvailable),
			changeLabel(s.Change12M, s.Change12MAvailable),
			s.Volatility,
			s.Score,
			s.RiskLevel,
		)
	}
	flush(tw)
	fmt.Fprint(out, "\n  Risk bands: Low 0-30 | Medium 30-60 | High 60-80 | Critical 80-100\n")
	fmt.Fprint(out, "  Note: this is a commodity price stress score, not a prediction of future prices.\n")
}

// renderCommodityFormula documents exactly how the Commodity Stress Score is
// built: its weighted formula, what each component measures, the risk bands, and
// an explicit statement of what the score is and is not.
func renderCommodityFormula(out io.Writer, w commodities.Weights) {
	section(out, "COMMODITY STRESS SCORE — FORMULA")
	fmt.Fprintf(out, "  Score name: Commodity Stress Score\n\n")

	fmt.Fprintln(out, "  Formula weights:")
	fmt.Fprintf(out, "      %.2f * recent_change_score\n", w.RecentChange)
	fmt.Fprintf(out, "    + %.2f * volatility_score\n", w.Volatility)
	fmt.Fprintf(out, "    + %.2f * momentum_score\n\n", w.Momentum)

	fmt.Fprintln(out, "  Component definitions:")
	fmt.Fprintln(out, "    recent_change_score = magnitude of the % price change over the last 3 months")
	fmt.Fprintln(out, "    volatility_score    = standard deviation of monthly returns over the last 12 months")
	fmt.Fprintln(out, "    momentum_score      = magnitude of the % price change over the last 12 months")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "  Risk bands:")
	fmt.Fprintln(out, "    Low      : 0-30")
	fmt.Fprintln(out, "    Medium   : 30-60")
	fmt.Fprintln(out, "    High     : 60-80")
	fmt.Fprintln(out, "    Critical : 80-100")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "  Note:")
	fmt.Fprintln(out, "    This is a commodity price stress score, not a prediction of future prices.")
	fmt.Fprintln(out, "    It summarises recent price movement and volatility from historical monthly")
	fmt.Fprintln(out, "    data only. Full AtlasGraph fragility scoring will later combine commodity")
	fmt.Fprintln(out, "    stress with graph dependency, trade concentration, macro exposure and event risk.")
}

// changeLabel renders a signed percentage change, or "n/a" when unavailable.
func changeLabel(v float64, ok bool) string {
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%+.1f%%", v)
}

// priceLabel renders a price with thousands grouping and an optional unit.
func priceLabel(price float64, unit string) string {
	s := groupThousands(price)
	// keep two decimals for sub-thousand prices where cents matter
	if price < 1000 {
		s = fmt.Sprintf("%.2f", price)
	}
	if unit != "" {
		return s + " " + unit
	}
	return s
}

func monthOrDash(m string) string {
	if strings.TrimSpace(m) == "" {
		return "-"
	}
	return m
}

func monthRange(first, last string) string {
	if first == "" && last == "" {
		return "-"
	}
	if first == last {
		return monthOrDash(first)
	}
	return fmt.Sprintf("%s to %s", monthOrDash(first), monthOrDash(last))
}

// --- generated trade graph -------------------------------------------------

func renderTradeGraphBuild(out io.Writer, srcData, outDir string, r tradegraph.Result) {
	section(out, "TRADE GRAPH BUILD")
	fmt.Fprintf(out, "  Source trade data  : %s\n", srcData)
	fmt.Fprintf(out, "  Output             : %s\n", outDir)
	fmt.Fprintf(out, "  Countries          : %d\n", r.CountCountries())
	fmt.Fprintf(out, "  Commodities        : %d\n", r.CountCommodities())
	fmt.Fprintf(out, "  Sectors            : %d\n", r.CountSectors())
	fmt.Fprintf(out, "  Dependencies       : %d\n", r.CountDependencies())
	fmt.Fprintf(out, "  Generated scenarios: %d\n", r.CountScenarios())

	if r.TopDependency != nil {
		d := r.TopDependency
		fmt.Fprintf(out, "  Top generated dependency: %s --%s--> %s (weight %.2f)\n",
			d.Source, d.Relationship, d.Target, d.Weight)
	} else {
		fmt.Fprintln(out, "  Top generated dependency: (none)")
	}
	if c := r.HighestImportConc; c != nil {
		fmt.Fprintf(out, "  Highest concentration import dependency: %s <- %s (HHI %.2f, top %s %.1f%%)\n",
			c.Importer, c.Commodity, c.HHI, c.TopSupplier, c.TopSupplierSh*100)
	} else {
		fmt.Fprintln(out, "  Highest concentration import dependency: (none)")
	}

	if len(r.Scenarios.Scenarios) > 0 {
		fmt.Fprintln(out, "\n  Scenarios:")
		for _, s := range r.Scenarios.Scenarios {
			fmt.Fprintf(out, "    - %s (%s, %s %.0f%%)\n", s.ID, s.Source, s.ShockType, s.ShockPercent)
		}
	}
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

// joinLabeledEdgePath renders a graph.EdgePath with the relationship (and
// commodity, when present) on each hop, e.g.
//
//	Taiwan --exports/semiconductors--> semiconductors --imports/semiconductors--> United States
func joinLabeledEdgePath(p graph.EdgePath) string {
	if len(p.Nodes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(p.Nodes[0].Name)
	for i, e := range p.Edges {
		label := string(e.Type)
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

// --- scenario comparison ---------------------------------------------------

func renderCompareResult(out io.Writer, g *graph.Graph, cmp simulation.ComparisonResult) {
	section(out, "SCENARIO COMPARISON")
	s := cmp.Summary
	fmt.Fprintf(out, "  Worst overall              : %s\n", orDash(s.WorstOverallScenario))
	fmt.Fprintf(out, "  Most countries affected    : %s\n", orDash(s.MostCountriesAffected))
	fmt.Fprintf(out, "  Most sectors affected      : %s\n", orDash(s.MostSectorsAffected))
	fmt.Fprintf(out, "  Highest avg fragility Δ    : %s\n", orDash(s.HighestAverageFragilityDelta))
	fmt.Fprintf(out, "  Highest max fragility Δ    : %s\n", orDash(s.HighestMaxFragilityDelta))

	for i, sc := range cmp.Results {
		fmt.Fprintf(out, "\n  #%d  %s\n", i+1, sc.Label)
		fmt.Fprintf(out, "      %s · %s · %s · drop %.0f%% · depth %d\n",
			sc.Source, sc.Commodity, sc.ShockType, sc.Drop, sc.Depth)
		if sc.RunError != "" {
			fmt.Fprintf(out, "      error: %s\n", sc.RunError)
			continue
		}
		fmt.Fprintf(out, "      affected nodes: %d  paths: %d  avg Δ: %.2f  max Δ: %.2f\n",
			sc.AffectedNodesCount, sc.AffectedPathsCount, sc.AvgFragilityDelta, sc.MaxFragilityDelta)
		if w := shockWarnings(g, sc.Profile, sc.Source, sc.Commodity); len(w) > 0 {
			for _, msg := range w {
				fmt.Fprintf(out, "      warning: %s\n", msg)
			}
		}
		if len(sc.TopAffectedCountries) > 0 {
			fmt.Fprintf(out, "      top countries:")
			for _, c := range sc.TopAffectedCountries {
				fmt.Fprintf(out, " %s (%.2f)", c.Entity, c.Delta)
			}
			fmt.Fprintln(out)
		}
		if len(sc.TopAffectedSectors) > 0 {
			fmt.Fprintf(out, "      top sectors:")
			for _, c := range sc.TopAffectedSectors {
				fmt.Fprintf(out, " %s (%.2f)", c.Entity, c.Delta)
			}
			fmt.Fprintln(out)
		}
	}
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
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

// renderGraphPathsFiltered renders shock- and commodity-aware dependency paths.
// Every hop is labelled with its relationship (and commodity), and when explain
// is set it shows the filtering logic and the branches the rules pruned —
// mirroring the shock command's PROPAGATION LOGIC view so the two stay
// consistent.
func renderGraphPathsFiltered(out io.Writer, from, to models.Node, depth int, profile simulation.ShockProfile, commodity string, paths []graph.EdgePath, blocked []simulation.BlockedEdge, blockedPaths int, explain bool) {
	section(out, "DEPENDENCY PATHS")
	fmt.Fprintf(out, "  From       : %s (%s)\n", from.Name, from.Type)
	fmt.Fprintf(out, "  To         : %s (%s)\n", to.Name, to.Type)
	fmt.Fprintf(out, "  Depth      : up to %d hops\n", depth)
	fmt.Fprintf(out, "  Commodity  : %s\n", commodity)
	fmt.Fprintf(out, "  Shock type : %s (%s)\n", profile.Type, profile.Name)

	if explain {
		section(out, "PATH FILTERING")
		fmt.Fprintf(out, "  Shock type                 : %s\n", profile.Type)
		fmt.Fprintf(out, "  Commodity filter           : %s\n", commodity)
		fmt.Fprintf(out, "  Allowed relationships      : %s\n", strings.Join(profile.AllowedRelationshipStrings(), ", "))
		fmt.Fprintf(out, "  Cross-commodity propagation: %s\n", enabledDisabled(profile.CrossCommodity))
		fmt.Fprintf(out, "  Blocked edges              : %d\n", len(blocked))
		fmt.Fprintf(out, "  Blocked paths              : %d\n", blockedPaths)
		if branches := blockedCommodityBranches(blocked); len(branches) > 0 {
			fmt.Fprintf(out, "  Blocked unrelated branches : %s\n", strings.Join(branches, ", "))
		} else {
			fmt.Fprintln(out, "  Blocked unrelated branches : (none)")
		}
		if len(blocked) > 0 {
			fmt.Fprintln(out, "  Blocked edges:")
			for _, b := range blocked {
				fmt.Fprintf(out, "    %s --%s--> %s   [%s]\n", b.From.Name, b.Relationship, b.To.Name, b.Reason)
			}
		}
	}

	section(out, "MATCHING PATHS")
	if len(paths) == 0 {
		fmt.Fprintf(out, "  No %s paths found from %s to %s within %d hops.\n", commodity, from.Name, to.Name, depth)
		return
	}
	for _, p := range paths {
		fmt.Fprintf(out, "  %s   [%d hops, path weight %.2f]\n", joinLabeledEdgePath(p), len(p.Edges), p.Weight())
	}
	fmt.Fprintf(out, "\n  %d path(s) found.\n", len(paths))
}

// blockedCommodityBranches returns the distinct commodities whose branches were
// pruned specifically because they were unrelated to the active commodity
// (cross-commodity blocks), not merely because of a relationship mismatch. It
// mirrors simulation.Result.BlockedCommodities for the path-filtering view.
func blockedCommodityBranches(blocked []simulation.BlockedEdge) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, b := range blocked {
		if b.Commodity == "" || !b.CrossCommodity {
			continue
		}
		if _, ok := seen[b.Commodity]; ok {
			continue
		}
		seen[b.Commodity] = struct{}{}
		out = append(out, b.Commodity)
	}
	sort.Strings(out)
	return out
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

// --- unified fragility -----------------------------------------------------

func renderFragilityScores(out io.Writer, res fragility.Result) {
	section(out, "UNIFIED FRAGILITY SCORES")

	fmt.Fprintln(out, "COUNTRIES")
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "COUNTRY\tSCORE\tRISK\tTOP DRIVERS")
	for _, s := range res.Countries {
		fmt.Fprintf(tw, "%s\t%.1f\t%s\t%s\n", s.CountryName, s.Score, s.RiskLevel, strings.Join(s.TopDrivers, ", "))
	}
	tw.Flush()
	fmt.Fprintln(out)

	fmt.Fprintln(out, "COMMODITIES")
	tw = tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "COMMODITY\tSCORE\tRISK\tTOP DRIVERS")
	for _, s := range res.Commodities {
		fmt.Fprintf(tw, "%s\t%.1f\t%s\t%s\n", s.CommodityName, s.Score, s.RiskLevel, strings.Join(s.TopDrivers, ", "))
	}
	tw.Flush()
	fmt.Fprintln(out)

	fmt.Fprintln(out, "Risk bands:")
	fmt.Fprintln(out, "  Low      : 0-30")
	fmt.Fprintln(out, "  Medium   : 30-60")
	fmt.Fprintln(out, "  High     : 60-80")
	fmt.Fprintln(out, "  Critical : 80-100")
}

func renderFragilityFormula(out io.Writer) {
	cw := fragility.DefaultCountryWeights()
	kw := fragility.DefaultCommodityWeights()

	section(out, "UNIFIED FRAGILITY SCORE — FORMULA")
	fmt.Fprintf(out, "  Score name: Unified Fragility Score\n\n")

	fmt.Fprintln(out, "  Country formula weights:")
	fmt.Fprintf(out, "      %.2f * macro_exposure_score\n", cw.MacroExposure)
	fmt.Fprintf(out, "    + %.2f * event_risk_score\n", cw.EventRisk)
	fmt.Fprintf(out, "    + %.2f * trade_concentration_score\n", cw.TradeConcentration)
	fmt.Fprintf(out, "    + %.2f * shock_exposure_score\n\n", cw.ShockExposure)

	fmt.Fprintln(out, "  Country component sources:")
	fmt.Fprintln(out, "    macro_exposure_score      = existing World Bank macro exposure score")
	fmt.Fprintln(out, "    event_risk_score          = existing GDELT event-risk score")
	fmt.Fprintln(out, "    trade_concentration_score = average supplier HHI across imported commodities")
	fmt.Fprintln(out, "    shock_exposure_score      = default scenario shock impact on the country")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "  Commodity formula weights:")
	fmt.Fprintf(out, "      %.2f * commodity_stress_score\n", kw.CommodityStress)
	fmt.Fprintf(out, "    + %.2f * supplier_concentration_score\n", kw.SupplierConcentration)
	fmt.Fprintf(out, "    + %.2f * event_exposure_score\n", kw.EventExposure)
	fmt.Fprintf(out, "    + %.2f * graph_centrality_score\n\n", kw.GraphCentrality)

	fmt.Fprintln(out, "  Commodity component sources:")
	fmt.Fprintln(out, "    commodity_stress_score       = existing commodity price stress score")
	fmt.Fprintln(out, "    supplier_concentration_score = average importer-side HHI across trade flows")
	fmt.Fprintln(out, "    event_exposure_score         = average event risk of exporter countries")
	fmt.Fprintln(out, "    graph_centrality_score       = commodity node degree relative to the graph")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "  Risk bands:")
	fmt.Fprintln(out, "    Low      : 0-30")
	fmt.Fprintln(out, "    Medium   : 30-60")
	fmt.Fprintln(out, "    High     : 60-80")
	fmt.Fprintln(out, "    Critical : 80-100")
	fmt.Fprintln(out)

	fmt.Fprintln(out, "  Note:")
	fmt.Fprintln(out, "    This is an explainable composite risk score, not a prediction.")
	fmt.Fprintln(out, "    It combines existing GFIP signals — macro exposure, event risk, trade")
	fmt.Fprintln(out, "    concentration, commodity stress, shock propagation and graph structure.")
	fmt.Fprintln(out, "    Missing components are excluded and weights are renormalised over what")
	fmt.Fprintln(out, "    remains available for each entity.")
}
