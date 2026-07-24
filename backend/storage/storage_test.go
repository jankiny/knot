package storage

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"knot-backend/appdata"

	_ "modernc.org/sqlite"
)

func TestOpenAppliesVersionedMigrationAndReopens(t *testing.T) {
	directory := repositoryTestDirectory(t, "storage_reopen")

	db, err := Open(context.Background(), directory)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	var version int
	if err := db.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf("expected schema version %d, got %d", CurrentSchemaVersion, version)
	}

	var tableName string
	if err := db.QueryRow(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table' AND name = 'indexed_documents'
	`).Scan(&tableName); err != nil {
		t.Fatalf("indexed_documents table missing: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	reopened, err := Open(context.Background(), directory)
	if err != nil {
		t.Fatalf("reopen database: %v", err)
	}
	defer reopened.Close()

	var migrationCount int
	if err := reopened.QueryRow("SELECT COUNT(*) FROM schema_migrations").Scan(&migrationCount); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if migrationCount != CurrentSchemaVersion {
		t.Fatalf("expected %d migration record, got %d", CurrentSchemaVersion, migrationCount)
	}
}

func TestMigrationFailureRollsBackSchemaChanges(t *testing.T) {
	directory := repositoryTestDirectory(t, "storage_rollback")
	databasePath := filepath.Join(directory.Path, DatabaseFileName)

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()

	err = applyMigrations(
		context.Background(),
		db,
		[]migration{{version: 1, filename: "broken.sql"}},
		func(string) ([]byte, error) {
			return []byte(`
				CREATE TABLE migration_partial (id TEXT PRIMARY KEY);
				THIS IS NOT VALID SQL;
			`), nil
		},
	)
	if err == nil {
		t.Fatal("expected migration failure")
	}

	var tableCount int
	if queryErr := db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name IN ('migration_partial', 'schema_migrations')
	`).Scan(&tableCount); queryErr != nil {
		t.Fatalf("inspect rolled back schema: %v", queryErr)
	}
	if tableCount != 0 {
		t.Fatalf("expected all migration changes to roll back, found %d tables", tableCount)
	}
}

func TestMigrateUpgradesVersionOneDatabaseToCurrentSchema(t *testing.T) {
	directory := repositoryTestDirectory(t, "storage_upgrade_v1")
	databasePath := filepath.Join(directory.Path, DatabaseFileName)

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()

	if err := applyMigrations(
		context.Background(),
		db,
		migrations[:1],
		migrationFiles.ReadFile,
	); err != nil {
		t.Fatalf("create version one database: %v", err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("upgrade database: %v", err)
	}

	var version int
	if err := db.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&version); err != nil {
		t.Fatalf("read upgraded schema version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf(
			"expected schema version %d, got %d",
			CurrentSchemaVersion,
			version,
		)
	}

	var indexCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'index'
			AND name IN (
				'idx_indexed_documents_source_status',
				'idx_indexed_documents_type_date',
				'idx_indexed_documents_project'
			)
	`).Scan(&indexCount); err != nil {
		t.Fatalf("inspect indexed document indexes: %v", err)
	}
	if indexCount != 3 {
		t.Fatalf("expected 3 indexed document indexes, got %d", indexCount)
	}

	var contextTableCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table'
			AND name IN (
				'context_manifests',
				'context_manifest_documents'
			)
	`).Scan(&contextTableCount); err != nil {
		t.Fatalf("inspect context manifest tables: %v", err)
	}
	if contextTableCount != 2 {
		t.Fatalf("expected 2 context manifest tables, got %d", contextTableCount)
	}
}

func TestMigrateUpgradesVersionTwoDatabaseToContextManifests(t *testing.T) {
	directory := repositoryTestDirectory(t, "storage_upgrade_v2")
	databasePath := filepath.Join(directory.Path, DatabaseFileName)

	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()

	if err := applyMigrations(
		context.Background(),
		db,
		migrations[:2],
		migrationFiles.ReadFile,
	); err != nil {
		t.Fatalf("create version two database: %v", err)
	}
	if err := Migrate(context.Background(), db); err != nil {
		t.Fatalf("upgrade database: %v", err)
	}

	var version int
	if err := db.QueryRow(
		"SELECT COALESCE(MAX(version), 0) FROM schema_migrations",
	).Scan(&version); err != nil {
		t.Fatalf("read upgraded schema version: %v", err)
	}
	if version != CurrentSchemaVersion {
		t.Fatalf(
			"expected schema version %d, got %d",
			CurrentSchemaVersion,
			version,
		)
	}

	var manifestIndexCount int
	if err := db.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'index'
			AND name IN (
				'idx_context_manifests_expires_at',
				'idx_context_manifest_documents_document'
			)
	`).Scan(&manifestIndexCount); err != nil {
		t.Fatalf("inspect context manifest indexes: %v", err)
	}
	if manifestIndexCount != 2 {
		t.Fatalf(
			"expected 2 context manifest indexes, got %d",
			manifestIndexCount,
		)
	}
}

func repositoryTestDirectory(t *testing.T, name string) appdata.Directory {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	safeName := strings.NewReplacer("/", "_", "\\", "_", " ", "_").Replace(name)
	path, err := filepath.Abs(filepath.Join(
		workingDirectory,
		"..",
		"..",
		"data",
		"_test_stage1",
		safeName,
	))
	if err != nil {
		t.Fatalf("resolve test data directory: %v", err)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("reset test data directory: %v", err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatalf("create test data directory: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(path)
	})
	return appdata.Directory{Path: path, Source: appdata.SourceEnvironment}
}
