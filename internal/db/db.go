package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"

	"github.com/pacorreia/fifa-wc-2026-card-trading/internal/config"
)

type Store struct {
	DB     *sql.DB
	Logger *slog.Logger
	driver string
}

func Connect(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Store, error) {
	var (
		database *sql.DB
		err      error
		drv      = strings.ToLower(strings.TrimSpace(cfg.DBDriver))
	)

	switch drv {
	case "sqlite", "":
		drv = "sqlite"
		dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys%%3D1", cfg.SQLitePath)
		database, err = sql.Open(sqliteDriverName, dsn)
		if err != nil {
			return nil, fmt.Errorf("open sqlite database: %w", err)
		}
		// SQLite performs best with a single writer connection.
		database.SetMaxOpenConns(1)
		database.SetMaxIdleConns(1)
		database.SetConnMaxLifetime(0)
	case "postgres":
		database, err = sql.Open("postgres", cfg.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("open database: %w", err)
		}
		database.SetMaxOpenConns(cfg.DBMaxOpenConns)
		database.SetMaxIdleConns(cfg.DBMaxIdleConns)
		database.SetConnMaxLifetime(cfg.DBConnMaxLifetime)
	default:
		return nil, fmt.Errorf("unsupported DB_DRIVER %q: use \"sqlite\" or \"postgres\"", cfg.DBDriver)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := database.PingContext(pingCtx); err != nil {
		database.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &Store{DB: database, Logger: logger, driver: drv}, nil
}

func (s *Store) Driver() string { return s.driver }

func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

func (s *Store) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.DB.ExecContext(ctx, query, args...)
}

func (s *Store) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.DB.QueryContext(ctx, query, args...)
}

func (s *Store) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.DB.QueryRowContext(ctx, query, args...)
}

func (s *Store) RunMigrations(ctx context.Context, migrationsDir string) error {
	var createMigrationsTable string
	if s.driver == "sqlite" {
		migrationsDir = filepath.Join(migrationsDir, "sqlite")
		createMigrationsTable = `
            CREATE TABLE IF NOT EXISTS schema_migrations (
                filename TEXT PRIMARY KEY,
                applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
            )`
	} else {
		createMigrationsTable = `
            CREATE TABLE IF NOT EXISTS schema_migrations (
                filename TEXT PRIMARY KEY,
                applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
            )`
	}

	if _, err := s.ExecContext(ctx, createMigrationsTable); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations directory: %w", err)
	}

	filenames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		filenames = append(filenames, entry.Name())
	}
	sort.Strings(filenames)

	for _, name := range filenames {
		var applied bool
		if err := s.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = $1)`, name).Scan(&applied); err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		path := filepath.Join(migrationsDir, name)
		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}

		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}

		s.Logger.Info("applied migration", "filename", name)
	}

	return nil
}

