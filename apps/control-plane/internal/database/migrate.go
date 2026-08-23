package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/database/migrations"
	"github.com/pressly/goose/v3"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// MigrationReport is safe to render in operational output.
type MigrationReport struct {
	Applied int
	Version int64
}

// Migrate applies all embedded forward migrations. Serving the application
// never calls this function implicitly.
func Migrate(ctx context.Context, databaseURL string) (MigrationReport, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := db.PingContext(ctx); err != nil {
		return MigrationReport{}, fmt.Errorf("ping migration database: %w", err)
	}
	return applyMigrations(ctx, db, migrations.Files)
}

func applyMigrations(ctx context.Context, db *sql.DB, migrationFS fs.FS) (MigrationReport, error) {
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationFS)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("create migration provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("apply forward migrations: %w", err)
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return MigrationReport{}, fmt.Errorf("read migration version: %w", err)
	}
	return MigrationReport{Applied: len(results), Version: version}, nil
}
