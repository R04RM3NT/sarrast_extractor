// Package database is the single, centralized persistence layer for the whole
// application. It owns the SQLite connection, schema creation, migrations,
// repositories and every transaction. The CLI, TUI, scanner and web server all
// go through this package; nobody else talks to SQLite directly.
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// StoragePath is the single fixed location of the SQLite database. It is an
// internal constant and is never configurable by the user.
const StoragePath = "data/series.db"

// ThumbnailsDir is the fixed, internal directory where downloaded series
// thumbnails are cached. It is never configurable by the user.
const ThumbnailsDir = "data/thumbnails"

// ErrNotFound is returned when a queried row does not exist.
var ErrNotFound = errors.New("not found")

// DB wraps the shared *sql.DB handle and adds the schema + migration logic.
type DB struct {
	sql  *sql.DB
	path string
	log  *slog.Logger
}

// New opens (creating it on first use) the centralized SQLite database at
// StoragePath, applies the schema and runs the JSON migration. A single *DB is
// meant to be created once per process and shared by every component.
func New(log *slog.Logger) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(StoragePath), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	if err := os.MkdirAll(ThumbnailsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create thumbnails directory: %w", err)
	}
	return Open(StoragePath, log)
}

// Open opens the SQLite database at path, applies the schema and runs the
// JSON migration. It is the programmatic entry point used by tests and any
// future component that needs a specific in-memory or temporary file; the
// product uses New, which always targets the single fixed StoragePath.
func Open(path string, log *slog.Logger) (*DB, error) {
	if log == nil {
		log = slog.Default()
	}
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := applyPragmas(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply sqlite pragmas: %w", err)
	}

	handle := &DB{sql: db, path: path, log: log}
	if err := handle.init(); err != nil {
		db.Close()
		return nil, err
	}
	return handle, nil
}

// dsn builds a connection string. The file is created on first use. PRAGMAs
// are not passed on the DSN (only the last _pragma would survive) but applied
// in code by applyPragmas on the single shared connection.
func dsn(path string) string {
	return "file:" + filepath.ToSlash(filepath.Clean(path)) + "?_pragma=journal_mode(WAL)"
}

// applyPragmas configures the connection that the shared handle uses. The
// PRAGMAs are scoped to one connection, so they must run on every fresh
// connection the pool hands out; with SetMaxOpenConns(1) there is only ever
// one, and it is run once here before anything else reads or writes.
func applyPragmas(db *sql.DB) error {
	for _, pragma := range []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA journal_mode = WAL",
	} {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("%s: %w", pragma, err)
		}
	}
	return nil
}

// Path returns the absolute database file location (for display and tests).
func (db *DB) Path() string { return db.path }

// SQL exposes the underlying handle for advanced queries. Prefer the
// repository methods above raw SQL.
func (db *DB) SQL() *sql.DB { return db.sql }

// init applies pending migrations and the one-time JSON import the first time
// a fresh database is opened. Existing databases are upgraded in place and the
// JSON import is idempotent.
func (db *DB) init() error {
	if err := db.ping(); err != nil {
		return err
	}
	if err := db.runMigrations(); err != nil {
		return err
	}
	if err := MigrateJSONIfPresent(db); err != nil {
		return fmt.Errorf("json migration: %w", err)
	}
	return nil
}

// ping forces a connection so the driver churns its schema metadata before
// anything else touches it.
func (db *DB) ping() error {
	if err := db.sql.Ping(); err != nil {
		return fmt.Errorf("ping sqlite database: %w", err)
	}
	return nil
}

// Close releases the underlying connection and in-memory prepared statements.
func (db *DB) Close() error { return db.sql.Close() }

// runMigrations applies versioned migrations, tracking progress via PRAGMA
// user_version. New databases are created by a single migration that builds the
// schema; existing databases are upgraded in place and skip it.
func (db *DB) runMigrations() error {
	var version int
	if err := db.sql.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	tx, err := db.sql.Begin()
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback()

	for v := version + 1; v <= migrationVersion; v++ {
		if err := migrations[v](tx); err != nil {
			return fmt.Errorf("apply migration v%d: %w", v, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
			return fmt.Errorf("set schema version: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration transaction: %w", err)
	}
	return nil
}
