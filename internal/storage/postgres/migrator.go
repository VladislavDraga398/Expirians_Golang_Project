package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	migrationsGlob    = "sql/migrations/*.sql"
	migrationLockKey  = int64(10824701)
	migrationTableDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`
)

var (
	//go:embed sql/migrations/*.sql
	migrationsFS embed.FS

	migrationFilePattern = regexp.MustCompile(`^(\d+)_([a-zA-Z0-9_]+)\.(up|down)\.sql$`)
)

type migrationDirection string

const (
	migrationUp   migrationDirection = "up"
	migrationDown migrationDirection = "down"
)

type migration struct {
	Version int64
	Name    string
	UpSQL   string
	DownSQL string
}

type migrationBuilder struct {
	version int64
	name    string
	upSQL   string
	downSQL string
}

// MigrateUp применяет up-миграции.
// steps=0 означает "применить все доступные".
func (s *Store) MigrateUp(ctx context.Context, steps int) error {
	return s.migrate(ctx, migrationUp, steps)
}

// MigrateDown откатывает миграции.
// steps<=0 интерпретируется как 1 шаг для безопасного поведения.
func (s *Store) MigrateDown(ctx context.Context, steps int) error {
	if steps <= 0 {
		steps = 1
	}
	return s.migrate(ctx, migrationDown, steps)
}

// MigrationStatus возвращает текущую версию и количество применённых миграций.
func (s *Store) MigrationStatus(ctx context.Context) (int64, int, error) {
	if s == nil || s.db == nil {
		return 0, 0, fmt.Errorf("postgres store is not initialized")
	}

	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if _, err := s.db.ExecContext(queryCtx, migrationTableDDL); err != nil {
		return 0, 0, fmt.Errorf("ensure migration table: %w", err)
	}

	var (
		version int64
		count   int
	)
	if err := s.db.QueryRowContext(queryCtx, `
		SELECT COALESCE(MAX(version), 0), COUNT(*)
		FROM schema_migrations
	`).Scan(&version, &count); err != nil {
		return 0, 0, fmt.Errorf("query migration status: %w", err)
	}

	return version, count, nil
}

func (s *Store) migrate(ctx context.Context, direction migrationDirection, steps int) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres store is not initialized")
	}

	migrations, err := loadMigrationsFromFS(migrationsFS)
	if err != nil {
		return err
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire db connection: %w", err)
	}
	defer conn.Close()

	lockCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := conn.ExecContext(lockCtx, "SELECT pg_advisory_lock($1)", migrationLockKey); err != nil {
		return fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()

	if _, err := conn.ExecContext(ctx, migrationTableDDL); err != nil {
		return fmt.Errorf("ensure migration table: %w", err)
	}

	switch direction {
	case migrationUp:
		return applyUp(ctx, conn, migrations, steps)
	case migrationDown:
		return applyDown(ctx, conn, migrations, steps)
	default:
		return fmt.Errorf("unsupported migration direction: %s", direction)
	}
}

func applyUp(ctx context.Context, conn *sql.Conn, migrations []migration, steps int) error {
	applied, err := loadAppliedVersions(ctx, conn)
	if err != nil {
		return err
	}

	appliedSteps := 0
	for _, m := range migrations {
		if applied[m.Version] {
			continue
		}
		if err := applyOneUp(ctx, conn, m); err != nil {
			return err
		}
		appliedSteps++
		if steps > 0 && appliedSteps >= steps {
			break
		}
	}

	return nil
}

func applyDown(ctx context.Context, conn *sql.Conn, migrations []migration, steps int) error {
	versionMap := make(map[int64]migration, len(migrations))
	for _, m := range migrations {
		versionMap[m.Version] = m
	}

	versions, err := loadAppliedVersionsDesc(ctx, conn, steps)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return nil
	}

	for _, version := range versions {
		m, ok := versionMap[version]
		if !ok {
			return fmt.Errorf("cannot rollback unknown migration version %d", version)
		}
		if err := applyOneDown(ctx, conn, m); err != nil {
			return err
		}
	}

	return nil
}

func applyOneUp(ctx context.Context, conn *sql.Conn, m migration) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx (up %d): %w", m.Version, err)
	}

	if _, err := tx.ExecContext(ctx, m.UpSQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("execute up migration %d_%s: %w", m.Version, m.Name, err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO schema_migrations (version, name, applied_at)
		VALUES ($1, $2, NOW())
	`, m.Version, m.Name); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record up migration %d_%s: %w", m.Version, m.Name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit up migration %d_%s: %w", m.Version, m.Name, err)
	}

	return nil
}

func applyOneDown(ctx context.Context, conn *sql.Conn, m migration) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration tx (down %d): %w", m.Version, err)
	}

	if _, err := tx.ExecContext(ctx, m.DownSQL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("execute down migration %d_%s: %w", m.Version, m.Name, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version = $1`, m.Version); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete migration record %d_%s: %w", m.Version, m.Name, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit down migration %d_%s: %w", m.Version, m.Name, err)
	}

	return nil
}

func loadAppliedVersions(ctx context.Context, conn *sql.Conn) (map[int64]bool, error) {
	rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]bool)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration version: %w", err)
		}
		result[version] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return result, nil
}

func loadAppliedVersionsDesc(ctx context.Context, conn *sql.Conn, limit int) ([]int64, error) {
	rows, err := conn.QueryContext(ctx, `
		SELECT version
		FROM schema_migrations
		ORDER BY version DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations desc: %w", err)
	}
	defer rows.Close()

	versions := make([]int64, 0, limit)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration desc: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations desc: %w", err)
	}

	return versions, nil
}

func loadMigrationsFromFS(fsys fs.FS) ([]migration, error) {
	files, err := fs.Glob(fsys, migrationsGlob)
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}
	if len(files) == 0 {
		return nil, errors.New("no migration files found")
	}

	builders := make(map[int64]*migrationBuilder)
	for _, file := range files {
		base := filepath.Base(file)
		matches := migrationFilePattern.FindStringSubmatch(base)
		if len(matches) != 4 {
			return nil, fmt.Errorf("invalid migration file name: %s", base)
		}

		version, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse migration version from %s: %w", base, err)
		}
		name := matches[2]
		direction := matches[3]

		bodyRaw, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, fmt.Errorf("read migration file %s: %w", file, err)
		}
		body := strings.TrimSpace(string(bodyRaw))
		if body == "" {
			return nil, fmt.Errorf("migration file is empty: %s", base)
		}

		builder, ok := builders[version]
		if !ok {
			builder = &migrationBuilder{version: version, name: name}
			builders[version] = builder
		} else if builder.name != name {
			return nil, fmt.Errorf("migration name mismatch for version %d: %s vs %s", version, builder.name, name)
		}

		switch direction {
		case "up":
			if builder.upSQL != "" {
				return nil, fmt.Errorf("duplicate up migration for version %d", version)
			}
			builder.upSQL = body
		case "down":
			if builder.downSQL != "" {
				return nil, fmt.Errorf("duplicate down migration for version %d", version)
			}
			builder.downSQL = body
		default:
			return nil, fmt.Errorf("unsupported migration direction in file: %s", base)
		}
	}

	versions := make([]int64, 0, len(builders))
	for version := range builders {
		versions = append(versions, version)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })

	migrations := make([]migration, 0, len(versions))
	for _, version := range versions {
		b := builders[version]
		if b.upSQL == "" || b.downSQL == "" {
			return nil, fmt.Errorf("migration %d_%s must have both up and down files", b.version, b.name)
		}
		migrations = append(migrations, migration{
			Version: b.version,
			Name:    b.name,
			UpSQL:   b.upSQL,
			DownSQL: b.downSQL,
		})
	}

	return migrations, nil
}
