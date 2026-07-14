package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

func Open(ctx context.Context, dataSource string) (*sql.DB, error) {
	database, err := sql.Open("sqlite", dataSource)
	if err != nil {
		return nil, fmt.Errorf("opening SQLite database: %w", err)
	}
	database.SetMaxOpenConns(1)

	if _, err := database.ExecContext(ctx, "PRAGMA foreign_keys = ON; PRAGMA busy_timeout = 5000;"); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("configuring SQLite database: %w", err)
	}
	if err := Migrate(ctx, database); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

func Migrate(ctx context.Context, database *sql.DB) error {
	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("opening embedded migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, database, migrationFS)
	if err != nil {
		return fmt.Errorf("creating migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("applying database migrations: %w", err)
	}
	return nil
}
