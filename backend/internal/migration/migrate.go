// Package migration runs SQL migrations at startup using an embedded file system.
// It tracks applied migrations in a schema_migrations table so each file runs exactly once.
package migration

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/pgconn"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

// Run applies all pending migrations in lexicographic order.
// Safe to call every startup — already-applied migrations are skipped.
// If a migration fails with a Postgres "already exists" error (e.g. the schema
// was bootstrapped manually), it is marked applied and execution continues.
func Run(ctx context.Context, db *pgxpool.Pool, log *slog.Logger) error {
	if err := ensureMigrationsTable(ctx, db); err != nil {
		return fmt.Errorf("migration: ensure table: %w", err)
	}

	files, err := fs.Glob(sqlFiles, "sql/*.sql")
	if err != nil {
		return fmt.Errorf("migration: glob sql files: %w", err)
	}
	sort.Strings(files) // guarantee lexicographic order (001_, 002_, …)

	for _, f := range files {
		name := migrationName(f)

		applied, err := isApplied(ctx, db, name)
		if err != nil {
			return fmt.Errorf("migration: check %s: %w", name, err)
		}
		if applied {
			log.Debug("migration already applied", "name", name)
			continue
		}

		sql, err := sqlFiles.ReadFile(f)
		if err != nil {
			return fmt.Errorf("migration: read %s: %w", f, err)
		}

		log.Info("applying migration", "name", name)
		if _, execErr := db.Exec(ctx, string(sql)); execErr != nil {
			if isAlreadyExistsErr(execErr) {
				log.Warn("migration skipped — objects already exist", "name", name)
			} else {
				return fmt.Errorf("migration: apply %s: %w", name, execErr)
			}
		}

		if err := markApplied(ctx, db, name); err != nil {
			return fmt.Errorf("migration: mark %s applied: %w", name, err)
		}

		log.Info("migration applied", "name", name)
	}

	return nil
}

// isAlreadyExistsErr returns true for Postgres "duplicate object" error classes:
//   - 42P07 duplicate_table
//   - 42701 duplicate_column
//   - 42710 duplicate_object  (indexes, constraints, etc.)
func isAlreadyExistsErr(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "42P07", "42701", "42710":
			return true
		}
	}
	return false
}

func ensureMigrationsTable(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name       TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func isApplied(ctx context.Context, db *pgxpool.Pool, name string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)`, name,
	).Scan(&exists)
	return exists, err
}

func markApplied(ctx context.Context, db *pgxpool.Pool, name string) error {
	_, err := db.Exec(ctx,
		`INSERT INTO schema_migrations(name) VALUES($1) ON CONFLICT DO NOTHING`, name,
	)
	return err
}

// migrationName strips the directory prefix and ".sql" suffix, e.g.
// "sql/001_init.sql" → "001_init".
func migrationName(path string) string {
	base := path
	if i := strings.LastIndex(path, "/"); i >= 0 {
		base = path[i+1:]
	}
	return strings.TrimSuffix(base, ".sql")
}
