package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/stepbeta/jobwatch/internal/model"
)

var migrationName = regexp.MustCompile(`^(\d{4})_([a-z0-9_]+)\.sql$`)

type migration struct {
	version int
	name    string
	file    string
}

// loadMigrations lists NNNN_name.sql files at the root of fsys, sorted by
// version. Files not matching the pattern are ignored; duplicate
// versions are an error.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	var out []migration
	seen := map[int]string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationName.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v, _ := strconv.Atoi(m[1])
		if prev, dup := seen[v]; dup {
			return nil, fmt.Errorf("duplicate migration version %04d: %s and %s", v, prev, e.Name())
		}
		seen[v] = e.Name()
		out = append(out, migration{version: v, name: m[2], file: e.Name()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })
	return out, nil
}

// Migrate applies, in version order, every migration in fsys not yet
// recorded in schema_migrations. Each migration runs in its own
// transaction together with its schema_migrations row.
func Migrate(ctx context.Context, sqlDB *sql.DB, fsys fs.FS) error {
	if _, err := sqlDB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		name       TEXT NOT NULL,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	migs, err := loadMigrations(fsys)
	if err != nil {
		return err
	}

	applied := map[int]bool{}
	rows, err := sqlDB.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[v] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}

	known := map[int]bool{}
	for _, m := range migs {
		known[m.version] = true
	}
	for v := range applied {
		if !known[v] {
			return fmt.Errorf("database has migration %04d unknown to this binary (database is newer than the program)", v)
		}
	}

	for _, m := range migs {
		if applied[m.version] {
			continue
		}
		body, err := fs.ReadFile(fsys, m.file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", m.file, err)
		}
		if err := applyMigration(ctx, sqlDB, m, string(body)); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, sqlDB *sql.DB, m migration, body string) error {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", m.file, err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return fmt.Errorf("apply migration %s: %w", m.file, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version, name, applied_at) VALUES (?, ?, ?)`,
		m.version, m.name, model.FormatTime(time.Now())); err != nil {
		return fmt.Errorf("record migration %s: %w", m.file, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", m.file, err)
	}
	return nil
}
