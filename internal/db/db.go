// Package db opens the SQLite database, applies migrations and holds the
// queries used by the rest of the application.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	"github.com/stepbeta/jobwatch/migrations"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Open opens (creating if needed) the SQLite database at path, enables
// WAL mode and foreign keys on every connection, and applies pending
// migrations.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "synchronous(NORMAL)")
	// BEGIN IMMEDIATE avoids lock-upgrade deadlocks between concurrent writers.
	q.Add("_txlock", "immediate")
	dsn := "file:" + path + "?" + q.Encode()

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping %s: %w", path, err)
	}
	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return sqlDB, nil
}

// Backup writes a consistent copy of the database to dest using
// VACUUM INTO, which is safe while the database is in WAL mode and in use.
// dest must not exist.
func Backup(ctx context.Context, sqlDB *sql.DB, dest string) error {
	if _, err := sqlDB.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		return fmt.Errorf("backup to %s: %w", dest, err)
	}
	return nil
}
