package db

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInitialMigrationExistsAndDefinesAnalyticsTables(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "001_init_postgres.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(raw))
	for _, table := range []string{
		"trade_flows", "event_risk_signals", "macro_scores",
		"commodity_prices", "dependency_edges", "scenario_runs",
		"data_quality_checks",
	} {
		if !strings.Contains(sql, "create table if not exists "+table) {
			t.Errorf("migration does not create %s", table)
		}
	}
	for _, indexColumns := range []string{
		"(importer_code, commodity)", "(exporter_code, commodity)", "(year)",
		"(created_at desc)", "(country_code)",
	} {
		if !strings.Contains(sql, indexColumns) {
			t.Errorf("migration does not contain index columns %s", indexColumns)
		}
	}
}

func TestCustomClientDataMigrationExists(t *testing.T) {
	path := filepath.Join("..", "..", "migrations", "002_custom_client_data.sql")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := strings.ToLower(string(raw))
	for _, table := range []string{"custom_trade_flows", "custom_concentration_results"} {
		if !strings.Contains(sql, "create table if not exists "+table) {
			t.Errorf("migration does not create %s", table)
		}
	}
}

func TestPostgresPingWhenConfigured(t *testing.T) {
	url := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if url == "" {
		t.Skip("DATABASE_URL is not set; skipping PostgreSQL integration test")
	}
	store, err := Connect(url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer store.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
}
