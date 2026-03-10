package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	defaultConnTimeout     = 5 * time.Second
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 25
	defaultConnMaxLifetime = 30 * time.Minute
	defaultConnMaxIdleTime = 5 * time.Minute
)

// Store оборачивает SQL-подключение к PostgreSQL.
type Store struct {
	db *sql.DB
}

// Open открывает подключение к PostgreSQL и проверяет доступность базы.
func Open(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres connection: %w", err)
	}
	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)
	db.SetConnMaxIdleTime(defaultConnMaxIdleTime)

	pingCtx, cancel := context.WithTimeout(ctx, defaultConnTimeout)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Store{db: db}, nil
}

// DB возвращает raw SQL DB, когда нужен низкоуровневый доступ.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Ping проверяет доступность подключения.
func (s *Store) Ping(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres store is not initialized")
	}

	pingCtx, cancel := context.WithTimeout(ctx, defaultConnTimeout)
	defer cancel()
	return s.db.PingContext(pingCtx)
}

// EnsureSchema сохраняет обратную совместимость со старым интерфейсом
// и применяет все up-миграции.
func (s *Store) EnsureSchema(ctx context.Context) error {
	return s.MigrateUp(ctx, 0)
}

// Close закрывает подключение к БД.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}
