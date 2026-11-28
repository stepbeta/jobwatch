// Package db opens the SQLite database, applies migrations and holds the
// queries used by the rest of the application.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/stepbeta/jobwatch/migrations"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// Options controls optional database open behavior.
type Options struct {
	// BackupBeforeMigrate writes a validated copy of an existing database
	// next to it before the first pending migration runs. A fresh database
	// with no schema history is never backed up.
	BackupBeforeMigrate bool
}

// Open opens (creating if needed) the SQLite database at path, enables
// WAL mode and foreign keys on every connection, and applies pending
// migrations.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	return OpenWithOptions(ctx, path, Options{})
}

// OpenWithOptions is Open with opt-in behavior such as the pre-migration
// backup.
func OpenWithOptions(ctx context.Context, path string, opts Options) (*sql.DB, error) {
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
	if opts.BackupBeforeMigrate {
		if err := backupBeforeMigrate(ctx, sqlDB, path, migrations.FS); err != nil {
			sqlDB.Close()
			return nil, err
		}
	}
	if err := Migrate(ctx, sqlDB, migrations.FS); err != nil {
		sqlDB.Close()
		return nil, err
	}
	return sqlDB, nil
}

// backupBeforeMigrate writes one validated backup next to an existing
// database when a migration is pending. It is a no-op for a fresh database
// (no schema history), a database that is already current, and in-memory or
// DSN paths.
func backupBeforeMigrate(ctx context.Context, sqlDB *sql.DB, dbPath string, fsys fs.FS) error {
	pending, hasHistory, err := PendingMigrations(ctx, sqlDB, fsys)
	if err != nil {
		return fmt.Errorf("pre-migration backup: %w", err)
	}
	if !hasHistory || len(pending) == 0 || memoryOrDSN(dbPath) {
		return nil
	}
	dest, err := preMigrationBackupPath(dbPath, pending[0])
	if err != nil {
		return fmt.Errorf("pre-migration backup: %w", err)
	}
	if err := Backup(ctx, sqlDB, dest); err != nil {
		return fmt.Errorf("pre-migration backup: %w", err)
	}
	return nil
}

// memoryOrDSN reports paths that must never be backed up because they do
// not name a single file on disk.
func memoryOrDSN(path string) bool {
	return path == "" || path == ":memory:" || strings.HasPrefix(path, "file:") || strings.Contains(path, "?")
}

// preMigrationBackupPath builds <db>.<UTC timestamp>.pre-<version>.bkp and
// adds a numeric suffix if that exact name already exists, so a retry never
// overwrites the previous backup.
func preMigrationBackupPath(dbPath string, version int) (string, error) {
	stamp := time.Now().UTC().Format("20060102T150405.000000000Z")
	base := fmt.Sprintf("%s.%s.pre-%04d", dbPath, stamp, version)
	dest := base + ".bkp"
	for i := 2; ; i++ {
		_, err := os.Stat(dest)
		switch {
		case os.IsNotExist(err):
			return dest, nil
		case err != nil:
			return "", err
		}
		dest = fmt.Sprintf("%s-%d.bkp", base, i)
	}
}

// Backup writes a consistent copy of the database to dest using// VACUUM INTO, which is safe while the database is in WAL mode and in use.
// dest must not exist. The copy is reopened read-only and checked with
// PRAGMA integrity_check and foreign_key_check before success is reported.
func Backup(ctx context.Context, sqlDB *sql.DB, dest string) error {
	if _, err := sqlDB.ExecContext(ctx, "VACUUM INTO ?", dest); err != nil {
		return fmt.Errorf("backup to %s: %w", dest, err)
	}
	if err := ValidateBackup(ctx, dest); err != nil {
		_ = os.Remove(dest)
		return fmt.Errorf("backup validation: %w", err)
	}
	return nil
}

// ValidateBackup opens path read-only and verifies that it is a complete,
// internally consistent JobWatch database.
func ValidateBackup(ctx context.Context, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return fmt.Errorf("%s is empty", path)
	}
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=query_only(1)")
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer sqlDB.Close()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	var integrity string
	if err := sqlDB.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
		return fmt.Errorf("integrity check %s: %w", path, err)
	}
	if integrity != "ok" {
		return fmt.Errorf("integrity check %s: %s", path, integrity)
	}
	rows, err := sqlDB.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return fmt.Errorf("foreign key check %s: %w", path, err)
	}
	violations := 0
	for rows.Next() {
		violations++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("foreign key check %s: %w", path, err)
	}
	if violations > 0 {
		return fmt.Errorf("%s has %d foreign key violation(s)", path, violations)
	}
	var version int
	if err := sqlDB.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil {
		return fmt.Errorf("read schema_migrations from %s: %w", path, err)
	}
	if version == 0 {
		return fmt.Errorf("%s has no applied migrations", path)
	}
	return nil
}
