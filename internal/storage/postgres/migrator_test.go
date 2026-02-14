package postgres

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadMigrationsFromFS_Success(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"sql/migrations/0001_init.up.sql": {
			Data: []byte("CREATE TABLE test_a (id INT);"),
		},
		"sql/migrations/0001_init.down.sql": {
			Data: []byte("DROP TABLE IF EXISTS test_a;"),
		},
		"sql/migrations/0002_more.up.sql": {
			Data: []byte("CREATE TABLE test_b (id INT);"),
		},
		"sql/migrations/0002_more.down.sql": {
			Data: []byte("DROP TABLE IF EXISTS test_b;"),
		},
	}

	migrations, err := loadMigrationsFromFS(fsys)
	if err != nil {
		t.Fatalf("loadMigrationsFromFS failed: %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("expected 2 migrations, got %d", len(migrations))
	}

	if migrations[0].Version != 1 || migrations[0].Name != "init" {
		t.Fatalf("unexpected first migration: %+v", migrations[0])
	}
	if migrations[1].Version != 2 || migrations[1].Name != "more" {
		t.Fatalf("unexpected second migration: %+v", migrations[1])
	}
}

func TestLoadMigrationsFromFS_MissingDown(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"sql/migrations/0001_init.up.sql": {
			Data: []byte("CREATE TABLE test_a (id INT);"),
		},
	}

	_, err := loadMigrationsFromFS(fsys)
	if err == nil {
		t.Fatal("expected error for missing down migration")
	}
	if !strings.Contains(err.Error(), "both up and down") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadMigrationsFromFS_InvalidFilename(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"sql/migrations/not_a_migration.sql": {
			Data: []byte("SELECT 1;"),
		},
	}

	_, err := loadMigrationsFromFS(fsys)
	if err == nil {
		t.Fatal("expected error for invalid migration file name")
	}
}

func TestLoadMigrationsFromFS_EmptyFile(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"sql/migrations/0001_init.up.sql": {
			Data: []byte("   \n"),
		},
		"sql/migrations/0001_init.down.sql": {
			Data: []byte("DROP TABLE IF EXISTS test;"),
		},
	}

	_, err := loadMigrationsFromFS(fsys)
	if err == nil {
		t.Fatal("expected error for empty migration file body")
	}
}
