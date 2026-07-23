// Package storage owns the Knot SQLite database lifecycle and schema
// migrations. Callers must provide the directory returned by appdata.Resolve.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"knot-backend/appdata"

	_ "modernc.org/sqlite"
)

const DatabaseFileName = "knot.db"

// Open creates or opens the application database and applies all pending
// migrations before returning it.
func Open(ctx context.Context, directory appdata.Directory) (*sql.DB, error) {
	if !filepath.IsAbs(directory.Path) {
		return nil, fmt.Errorf("application data directory must be absolute")
	}
	if err := os.MkdirAll(directory.Path, 0o700); err != nil {
		return nil, fmt.Errorf("create application data directory: %w", err)
	}

	databasePath := filepath.Join(directory.Path, DatabaseFileName)
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open application database: %w", err)
	}

	// SQLite connection pragmas are connection-local. Keeping one connection
	// gives the desktop backend deterministic locking and foreign-key behavior.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to application database: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("configure database busy timeout: %w", err)
	}
	if err := Migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
