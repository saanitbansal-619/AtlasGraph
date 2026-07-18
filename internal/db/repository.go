package db

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type TradeFlow struct {
	Year                                 int
	ImporterName, ImporterCode           string
	ExporterName, ExporterCode           string
	Commodity, HSCode, TradeFlow, Source string
	TradeValueUSD                        float64
}

type EventRiskSignal struct {
	CountryName, CountryCode, EventType string
	EventCount                          int
	RiskScore                           float64
	RiskLevel, Source                   string
}

type MacroScore struct {
	CountryName, CountryCode string
	Score                    float64
	RiskLevel, Source        string
}

type CommodityPrice struct {
	Commodity, DateMonth string
	Price                float64
	Unit, Source         string
}

type DependencyEdge struct {
	SourceNode, TargetNode           string
	SourceType, TargetType           string
	RelationshipType, DataProvenance string
	Weight                           float64
}

type DataQualityCheck struct {
	CheckName   string
	Status      string
	MetricValue float64
	Details     any
	Source      string
}

type LoadBatch struct {
	TradeFlows       []TradeFlow
	EventRiskSignals []EventRiskSignal
	MacroScores      []MacroScore
	CommodityPrices  []CommodityPrice
	DependencyEdges  []DependencyEdge
	QualityChecks    []DataQualityCheck
}

type LoadCounts struct {
	TradeFlows       int `json:"trade_flows"`
	EventRiskSignals int `json:"event_risk_signals"`
	MacroScores      int `json:"macro_scores"`
	CommodityPrices  int `json:"commodity_prices"`
	DependencyEdges  int `json:"dependency_edges"`
	QualityChecks    int `json:"data_quality_checks"`
}

// ReplaceAnalytics atomically truncates v1 analytics tables and bulk loads the
// supplied normalized rows. scenario_runs is also reset in v1 as requested by
// the load command's replace-all semantics.
func (d *DB) ReplaceAnalytics(ctx context.Context, batch LoadBatch) (LoadCounts, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return LoadCounts{}, fmt.Errorf("begin analytics load: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `TRUNCATE TABLE
		trade_flows, event_risk_signals, macro_scores, commodity_prices,
		dependency_edges, scenario_runs, data_quality_checks RESTART IDENTITY`); err != nil {
		return LoadCounts{}, fmt.Errorf("truncate analytics tables: %w", err)
	}

	copyRows := func(table string, columns []string, rows [][]any) error {
		if len(rows) == 0 {
			return nil
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{table}, columns, pgx.CopyFromRows(rows)); err != nil {
			return fmt.Errorf("load %s: %w", table, err)
		}
		return nil
	}

	tradeRows := make([][]any, 0, len(batch.TradeFlows))
	for _, r := range batch.TradeFlows {
		tradeRows = append(tradeRows, []any{r.Year, r.ImporterName, r.ImporterCode, r.ExporterName, r.ExporterCode, r.Commodity, r.HSCode, r.TradeValueUSD, r.TradeFlow, r.Source})
	}
	if err := copyRows("trade_flows", []string{"year", "importer_name", "importer_code", "exporter_name", "exporter_code", "commodity", "hs_code", "trade_value_usd", "trade_flow", "source"}, tradeRows); err != nil {
		return LoadCounts{}, err
	}

	eventRows := make([][]any, 0, len(batch.EventRiskSignals))
	for _, r := range batch.EventRiskSignals {
		eventRows = append(eventRows, []any{r.CountryName, r.CountryCode, r.EventType, r.EventCount, r.RiskScore, r.RiskLevel, r.Source})
	}
	if err := copyRows("event_risk_signals", []string{"country_name", "country_code", "event_type", "event_count", "risk_score", "risk_level", "source"}, eventRows); err != nil {
		return LoadCounts{}, err
	}

	macroRows := make([][]any, 0, len(batch.MacroScores))
	for _, r := range batch.MacroScores {
		macroRows = append(macroRows, []any{r.CountryName, r.CountryCode, r.Score, r.RiskLevel, r.Source})
	}
	if err := copyRows("macro_scores", []string{"country_name", "country_code", "macro_score", "risk_level", "source"}, macroRows); err != nil {
		return LoadCounts{}, err
	}

	priceRows := make([][]any, 0, len(batch.CommodityPrices))
	for _, r := range batch.CommodityPrices {
		priceRows = append(priceRows, []any{r.Commodity, r.DateMonth, r.Price, r.Unit, r.Source})
	}
	if err := copyRows("commodity_prices", []string{"commodity", "date_month", "price", "unit", "source"}, priceRows); err != nil {
		return LoadCounts{}, err
	}

	edgeRows := make([][]any, 0, len(batch.DependencyEdges))
	for _, r := range batch.DependencyEdges {
		edgeRows = append(edgeRows, []any{r.SourceNode, r.TargetNode, r.SourceType, r.TargetType, r.RelationshipType, r.Weight, r.DataProvenance})
	}
	if err := copyRows("dependency_edges", []string{"source_node", "target_node", "source_type", "target_type", "relationship_type", "weight", "data_provenance"}, edgeRows); err != nil {
		return LoadCounts{}, err
	}

	checkRows := make([][]any, 0, len(batch.QualityChecks))
	for _, r := range batch.QualityChecks {
		details, err := json.Marshal(r.Details)
		if err != nil {
			return LoadCounts{}, fmt.Errorf("encode quality check %q details: %w", r.CheckName, err)
		}
		checkRows = append(checkRows, []any{r.CheckName, r.Status, r.MetricValue, string(details), r.Source})
	}
	if err := copyRows("data_quality_checks", []string{"check_name", "status", "metric_value", "details", "source"}, checkRows); err != nil {
		return LoadCounts{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return LoadCounts{}, fmt.Errorf("commit analytics load: %w", err)
	}
	return LoadCounts{
		TradeFlows:       len(batch.TradeFlows),
		EventRiskSignals: len(batch.EventRiskSignals),
		MacroScores:      len(batch.MacroScores),
		CommodityPrices:  len(batch.CommodityPrices),
		DependencyEdges:  len(batch.DependencyEdges),
		QualityChecks:    len(batch.QualityChecks),
	}, nil
}

type Summary struct {
	TradeFlows        int64 `json:"trade_flows"`
	EventRiskSignals  int64 `json:"event_risk_signals"`
	MacroScores       int64 `json:"macro_scores"`
	CommodityPrices   int64 `json:"commodity_prices"`
	DependencyEdges   int64 `json:"dependency_edges"`
	ScenarioRuns      int64 `json:"scenario_runs"`
	DataQualityChecks int64 `json:"data_quality_checks"`
}

func (d *DB) Summary(ctx context.Context) (Summary, error) {
	var s Summary
	err := d.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM trade_flows),
		(SELECT count(*) FROM event_risk_signals),
		(SELECT count(*) FROM macro_scores),
		(SELECT count(*) FROM commodity_prices),
		(SELECT count(*) FROM dependency_edges),
		(SELECT count(*) FROM scenario_runs),
		(SELECT count(*) FROM data_quality_checks)`).Scan(
		&s.TradeFlows, &s.EventRiskSignals, &s.MacroScores, &s.CommodityPrices,
		&s.DependencyEdges, &s.ScenarioRuns, &s.DataQualityChecks,
	)
	if err != nil {
		return Summary{}, fmt.Errorf("query analytics summary: %w", err)
	}
	return s, nil
}

type TopSupplier struct {
	ExporterName  string  `json:"exporter_name"`
	ExporterCode  string  `json:"exporter_code"`
	TradeValueUSD float64 `json:"trade_value_usd"`
	Share         float64 `json:"share"`
}

func (d *DB) TopSuppliers(ctx context.Context, importer, commodity string, limit int) ([]TopSupplier, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := d.pool.Query(ctx, `WITH suppliers AS (
			SELECT exporter_name, exporter_code, sum(trade_value_usd)::double precision AS value
			FROM trade_flows
			WHERE (upper(importer_code) = upper($1) OR lower(importer_name) = lower($1))
			  AND lower(commodity) = lower($2)
			GROUP BY exporter_name, exporter_code
		), totals AS (
			SELECT coalesce(sum(value), 0) AS total FROM suppliers
		)
		SELECT exporter_name, exporter_code, value,
		       CASE WHEN total > 0 THEN value / total ELSE 0 END AS share
		FROM suppliers CROSS JOIN totals
		ORDER BY value DESC, exporter_name
		LIMIT $3`, importer, commodity, limit)
	if err != nil {
		return nil, fmt.Errorf("query top suppliers: %w", err)
	}
	defer rows.Close()
	out := []TopSupplier{}
	for rows.Next() {
		var item TopSupplier
		if err := rows.Scan(&item.ExporterName, &item.ExporterCode, &item.TradeValueUSD, &item.Share); err != nil {
			return nil, fmt.Errorf("scan top supplier: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read top suppliers: %w", err)
	}
	return out, nil
}

type ScenarioRunInput struct {
	ScenarioID, Source, Commodity, ShockType string
	DropPercent                              float64
	Depth                                    int
	TopAffectedCountries                     any
	TopAffectedSectors                       any
	Report                                   any
}

func (d *DB) InsertScenarioRun(ctx context.Context, in ScenarioRunInput) error {
	countries, err := json.Marshal(in.TopAffectedCountries)
	if err != nil {
		return fmt.Errorf("encode top affected countries: %w", err)
	}
	sectors, err := json.Marshal(in.TopAffectedSectors)
	if err != nil {
		return fmt.Errorf("encode top affected sectors: %w", err)
	}
	report, err := json.Marshal(in.Report)
	if err != nil {
		return fmt.Errorf("encode scenario report: %w", err)
	}
	_, err = d.pool.Exec(ctx, `INSERT INTO scenario_runs
		(scenario_id, source, commodity, shock_type, drop_percent, depth,
		 top_affected_countries, top_affected_sectors, report_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (scenario_id) DO UPDATE SET
			source = EXCLUDED.source,
			commodity = EXCLUDED.commodity,
			shock_type = EXCLUDED.shock_type,
			drop_percent = EXCLUDED.drop_percent,
			depth = EXCLUDED.depth,
			top_affected_countries = EXCLUDED.top_affected_countries,
			top_affected_sectors = EXCLUDED.top_affected_sectors,
			report_json = EXCLUDED.report_json,
			created_at = now()`,
		in.ScenarioID, in.Source, in.Commodity, in.ShockType, in.DropPercent, in.Depth,
		countries, sectors, report)
	if err != nil {
		return fmt.Errorf("insert scenario run: %w", err)
	}
	return nil
}

type ScenarioRun struct {
	ScenarioID           string          `json:"scenario_id"`
	Source               string          `json:"source"`
	Commodity            string          `json:"commodity"`
	ShockType            string          `json:"shock_type"`
	DropPercent          float64         `json:"drop_percent"`
	Depth                int             `json:"depth"`
	TopAffectedCountries json.RawMessage `json:"top_affected_countries"`
	TopAffectedSectors   json.RawMessage `json:"top_affected_sectors"`
	Report               json.RawMessage `json:"report_json"`
	CreatedAt            time.Time       `json:"created_at"`
}

func (d *DB) RecentScenarios(ctx context.Context, limit int) ([]ScenarioRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.pool.Query(ctx, `SELECT scenario_id, source, commodity, shock_type,
		drop_percent::double precision, depth, top_affected_countries,
		top_affected_sectors, report_json, created_at
		FROM scenario_runs ORDER BY created_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent scenarios: %w", err)
	}
	defer rows.Close()
	out := []ScenarioRun{}
	for rows.Next() {
		var item ScenarioRun
		if err := rows.Scan(&item.ScenarioID, &item.Source, &item.Commodity, &item.ShockType,
			&item.DropPercent, &item.Depth, &item.TopAffectedCountries,
			&item.TopAffectedSectors, &item.Report, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan recent scenario: %w", err)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read recent scenarios: %w", err)
	}
	return out, nil
}
