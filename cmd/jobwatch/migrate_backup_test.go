package main

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stepbeta/jobwatch/internal/db"
	"github.com/stepbeta/jobwatch/migrations"
)

// makeCLIOldDatabase creates path with only migration 0001 applied.
func makeCLIOldDatabase(t *testing.T, path string) {
	t.Helper()
	body, err := fs.ReadFile(migrations.FS, "0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := db.Migrate(context.Background(), sqlDB,
		fstest.MapFS{"0001_init.sql": &fstest.MapFile{Data: body}}); err != nil {
		t.Fatalf("apply first migration: %v", err)
	}
}

func TestCLIBacksUpBeforePendingMigrations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cli.db")
	makeCLIOldDatabase(t, path)

	if out, err := runCLI(t, path, "list"); err != nil {
		t.Fatalf("list: %v (%s)", err, out)
	}

	backups, err := filepath.Glob(filepath.Join(dir, "cli.db.*.pre-*.bkp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 {
		t.Fatalf("backups = %v, want one next to the database", backups)
	}
	if !strings.Contains(filepath.Base(backups[0]), ".pre-0002.bkp") {
		t.Errorf("backup name = %q, want pre-0002", backups[0])
	}
	if err := db.ValidateBackup(context.Background(), backups[0]); err != nil {
		t.Errorf("backup validation: %v", err)
	}
}
