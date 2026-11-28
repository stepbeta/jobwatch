package db

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stepbeta/jobwatch/migrations"
)

// firstMigrationFS returns a filesystem with only the real 0001 migration,
// used to build a database that has schema history and pending versions.
func firstMigrationFS(t *testing.T) fstest.MapFS {
	t.Helper()
	body, err := fs.ReadFile(migrations.FS, "0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{"0001_init.sql": &fstest.MapFile{Data: body}}
}

// makeOldDatabase creates path with only migration 0001 applied.
func makeOldDatabase(t *testing.T, path string) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	if err := Migrate(context.Background(), sqlDB, firstMigrationFS(t)); err != nil {
		t.Fatalf("apply first migration: %v", err)
	}
}

func maxVersion(t *testing.T, path string) int {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	var v int
	if err := sqlDB.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func backupFiles(t *testing.T, pattern string) []string {
	t.Helper()
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatal(err)
	}
	return matches
}

func TestOpenWithOptionsBacksUpBeforePendingMigration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "jobwatch.db")
	makeOldDatabase(t, path)

	sqlDB, err := OpenWithOptions(ctx, path, Options{BackupBeforeMigrate: true})
	if err != nil {
		t.Fatalf("OpenWithOptions: %v", err)
	}
	defer sqlDB.Close()

	backups := backupFiles(t, filepath.Join(dir, "jobwatch.db.*.pre-*.bkp"))
	if len(backups) != 1 {
		t.Fatalf("backups = %v, want exactly 1", backups)
	}
	if !strings.Contains(filepath.Base(backups[0]), ".pre-0002.bkp") {
		t.Errorf("backup name = %q, want the first pending version 0002", backups[0])
	}
	if err := ValidateBackup(ctx, backups[0]); err != nil {
		t.Errorf("backup validation: %v", err)
	}
	// The copy was taken before the migration: it records only version 1.
	if v := maxVersion(t, backups[0]); v != 1 {
		t.Errorf("backup schema version = %d, want 1", v)
	}
	// The live database was migrated to the latest version.
	pending, hasHistory, err := PendingMigrations(ctx, sqlDB, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if !hasHistory || len(pending) != 0 {
		t.Errorf("after open: hasHistory=%v pending=%v, want true and none", hasHistory, pending)
	}
}

func TestOpenWithOptionsSkipsFreshDatabase(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "fresh.db")

	sqlDB, err := OpenWithOptions(ctx, path, Options{BackupBeforeMigrate: true})
	if err != nil {
		t.Fatalf("OpenWithOptions: %v", err)
	}
	defer sqlDB.Close()

	if backups := backupFiles(t, filepath.Join(dir, "*.bkp")); len(backups) != 0 {
		t.Errorf("fresh database wrote backups: %v", backups)
	}
}

func TestMigrationRetryWritesSecondBackup(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "jobwatch.db")
	makeOldDatabase(t, path)

	firstBody, err := fs.ReadFile(migrations.FS, "0001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	// The pending migration always fails, so the database stays at 0001.
	brokenFS := fstest.MapFS{
		"0001_init.sql":   &fstest.MapFile{Data: firstBody},
		"0002_broken.sql": &fstest.MapFile{Data: []byte("THIS IS NOT SQL;")},
	}

	sqlDB, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()

	if err := backupBeforeMigrate(ctx, sqlDB, path, brokenFS); err != nil {
		t.Fatalf("first backup: %v", err)
	}
	if err := Migrate(ctx, sqlDB, brokenFS); err == nil {
		t.Fatal("broken migration unexpectedly succeeded")
	}
	if err := backupBeforeMigrate(ctx, sqlDB, path, brokenFS); err != nil {
		t.Fatalf("second backup: %v", err)
	}

	backups := backupFiles(t, filepath.Join(dir, "jobwatch.db.*.pre-0002*.bkp"))
	if len(backups) != 2 {
		t.Fatalf("backups = %v, want two distinct files", backups)
	}
	if backups[0] == backups[1] {
		t.Fatal("the retry reused the first backup name")
	}
	for _, b := range backups {
		if err := ValidateBackup(ctx, b); err != nil {
			t.Errorf("backup %s validation: %v", b, err)
		}
	}
	var v int
	if err := sqlDB.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	if v != 1 {
		t.Errorf("database version = %d, want 1 after the failed migration", v)
	}
}

func TestBackupFailureAbortsMigration(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	// A legal file name whose backup name exceeds NAME_MAX, so VACUUM INTO
	// fails while the database itself opens and pings normally.
	path := filepath.Join(dir, strings.Repeat("a", 240)+".db")
	makeOldDatabase(t, path)

	_, err := OpenWithOptions(ctx, path, Options{BackupBeforeMigrate: true})
	if err == nil {
		t.Fatal("OpenWithOptions succeeded although the backup could not be written")
	}
	if !strings.Contains(err.Error(), "backup") {
		t.Errorf("error = %v, want a backup error", err)
	}
	if v := maxVersion(t, path); v != 1 {
		t.Errorf("database version = %d, want 1 (migration must not run)", v)
	}
}
