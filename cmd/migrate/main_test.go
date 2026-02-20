package main

import (
	"context"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/vladislavdragonenkov/oms/internal/storage/postgres"
)

const defaultLocalMigrateTestDSN = "postgres://oms:oms@localhost:5432/oms?sslmode=disable"

func withMigrateCLIArgs(t *testing.T, args []string, fn func()) {
	t.Helper()

	oldArgs := os.Args
	oldCommandLine := flag.CommandLine

	os.Args = append([]string{"migrate"}, args...)
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	defer func() {
		os.Args = oldArgs
		flag.CommandLine = oldCommandLine
	}()

	fn()
}

func testPostgresDSN(t *testing.T) string {
	t.Helper()

	candidates := []string{
		strings.TrimSpace(os.Getenv("OMS_POSTGRES_TEST_DSN")),
		strings.TrimSpace(os.Getenv("OMS_POSTGRES_DSN")),
		defaultLocalMigrateTestDSN,
	}

	seen := map[string]struct{}{}
	for _, dsn := range candidates {
		dsn = strings.TrimSpace(dsn)
		if dsn == "" {
			continue
		}
		if _, ok := seen[dsn]; ok {
			continue
		}
		seen[dsn] = struct{}{}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		store, err := postgres.Open(ctx, dsn)
		cancel()
		if err != nil {
			continue
		}
		_ = store.Close()
		return dsn
	}

	t.Skip("postgres dsn is not available")
	return ""
}

func TestMainStatusAndMigratePaths(t *testing.T) {
	dsn := testPostgresDSN(t)

	// status
	withMigrateCLIArgs(t, []string{"-direction=status", "-dsn=" + dsn}, func() {
		main()
	})

	// up
	withMigrateCLIArgs(t, []string{"-direction=up", "-steps=1", "-dsn=" + dsn}, func() {
		main()
	})

	// down
	withMigrateCLIArgs(t, []string{"-direction=down", "-steps=1", "-dsn=" + dsn}, func() {
		main()
	})
}

func TestMainMissingDSNExits(t *testing.T) {
	if os.Getenv("MIGRATE_TEST_EXIT") == "1" {
		withMigrateCLIArgs(t, []string{"-direction=status", "-dsn="}, func() {
			_ = os.Unsetenv("OMS_POSTGRES_DSN")
			main()
		})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainMissingDSNExits")
	cmd.Env = append(os.Environ(), "MIGRATE_TEST_EXIT=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to exit with error")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit code, got %v", err)
	}
}

func TestFailExits(t *testing.T) {
	if os.Getenv("MIGRATE_TEST_FAIL_EXIT") == "1" {
		fail("forced failure %d", 42)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestFailExits")
	cmd.Env = append(os.Environ(), "MIGRATE_TEST_FAIL_EXIT=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to exit with error")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit code, got %v", err)
	}
}

func TestMainUnsupportedDirectionExits(t *testing.T) {
	dsn := testPostgresDSN(t)

	if os.Getenv("MIGRATE_TEST_BAD_DIRECTION") == "1" {
		withMigrateCLIArgs(t, []string{"-direction=bad", "-dsn=" + dsn}, func() {
			main()
		})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainUnsupportedDirectionExits")
	cmd.Env = append(os.Environ(), "MIGRATE_TEST_BAD_DIRECTION=1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected subprocess to exit with error")
	}
	if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() == 0 {
		t.Fatalf("expected non-zero exit code, got %v", err)
	}
}
