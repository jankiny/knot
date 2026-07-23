package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"
)

const CurrentSchemaVersion = 2

//go:embed migrations/*.sql
var migrationFiles embed.FS

type migration struct {
	version  int
	filename string
}

var migrations = []migration{
	{version: 1, filename: "migrations/001_source_roots.sql"},
	{version: 2, filename: "migrations/002_indexed_documents.sql"},
}

// Migrate applies every pending schema migration in a single transaction and
// records each applied version. A failure rolls back both schema changes and
// version records.
func Migrate(ctx context.Context, db *sql.DB) error {
	return applyMigrations(ctx, db, migrations, migrationFiles.ReadFile)
}

func applyMigrations(
	ctx context.Context,
	db *sql.DB,
	pending []migration,
	readFile func(string) ([]byte, error),
) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema migration: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}

	var version int
	if err := tx.QueryRowContext(
		ctx,
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > CurrentSchemaVersion {
		return fmt.Errorf(
			"database schema version %d is newer than supported version %d",
			version,
			CurrentSchemaVersion,
		)
	}

	for _, item := range pending {
		if item.version <= version {
			continue
		}
		if item.version != version+1 {
			return fmt.Errorf(
				"schema migration sequence gap: current=%d next=%d",
				version,
				item.version,
			)
		}

		statement, err := readFile(item.filename)
		if err != nil {
			return fmt.Errorf("read migration %d: %w", item.version, err)
		}
		if _, err := tx.ExecContext(ctx, string(statement)); err != nil {
			return fmt.Errorf("apply migration %d: %w", item.version, err)
		}
		if _, err := tx.ExecContext(
			ctx,
			"INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)",
			item.version,
			time.Now().UTC().Format(time.RFC3339Nano),
		); err != nil {
			return fmt.Errorf("record migration %d: %w", item.version, err)
		}
		version = item.version
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema migration: %w", err)
	}
	return nil
}
