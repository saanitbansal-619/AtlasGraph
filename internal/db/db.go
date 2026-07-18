// Package db provides the optional PostgreSQL analytics and persistence layer.
// The file-backed ingestion and scoring pipeline does not depend on this
// package; callers opt in by supplying DATABASE_URL.
package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a pgx connection pool.
type DB struct {
	pool *pgxpool.Pool
}

// Connect creates a PostgreSQL connection pool. Call Ping to verify that the
// database is reachable before serving requests.
func Connect(databaseURL string) (*DB, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, fmt.Errorf("database URL is required")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Ping verifies that PostgreSQL is reachable.
func (d *DB) Ping(ctx context.Context) error {
	if d == nil || d.pool == nil {
		return fmt.Errorf("database is not connected")
	}
	if err := d.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return nil
}

// Close releases all pooled PostgreSQL connections.
func (d *DB) Close() {
	if d != nil && d.pool != nil {
		d.pool.Close()
	}
}

// ExecMigrationSQL executes one migration SQL document.
func (d *DB) ExecMigrationSQL(ctx context.Context, sql string) error {
	if strings.TrimSpace(sql) == "" {
		return fmt.Errorf("migration SQL is empty")
	}
	if _, err := d.pool.Exec(ctx, sql); err != nil {
		return fmt.Errorf("execute migration SQL: %w", err)
	}
	return nil
}

// Migrate executes all .sql files in a directory in lexical order. The v1
// migration uses IF NOT EXISTS and is safe to run repeatedly.
func (d *DB) Migrate(ctx context.Context, dir string) error {
	paths, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return fmt.Errorf("no SQL migrations found in %q", dir)
	}
	for _, path := range paths {
		sql, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %q: %w", path, err)
		}
		if err := d.ExecMigrationSQL(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply migration %q: %w", filepath.Base(path), err)
		}
	}
	return nil
}
