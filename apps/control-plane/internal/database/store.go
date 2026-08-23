// Package database owns the Control Plane PostgreSQL connection and explicit
// transaction boundary.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/GuardianPot/guardian/apps/control-plane/internal/database/dbgen"
	"github.com/GuardianPot/guardian/apps/control-plane/internal/database/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrSchemaNotReady indicates that the explicit migration command has not
// brought the database to the version required by this binary.
var ErrSchemaNotReady = errors.New("database schema is not ready")

// Store owns the PostgreSQL pool and generated typed queries.
type Store struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
}

// Open connects to PostgreSQL without applying migrations.
func Open(ctx context.Context, databaseURL string, maxConns int32) (*Store, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, errors.New("parse database configuration")
	}
	poolConfig.MaxConns = maxConns
	poolConfig.MinConns = 0
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "guardian-control-plane"

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return &Store{pool: pool, queries: dbgen.New(pool)}, nil
}

// Close releases every pooled connection.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// Ready checks connectivity and exact migration compatibility.
func (s *Store) Ready(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	var version int64
	err := s.pool.QueryRow(ctx, `
SELECT COALESCE(MAX(version_id), 0)
FROM goose_db_version
WHERE is_applied = true`).Scan(&version)
	if err != nil {
		return fmt.Errorf("%w: migration metadata unavailable", ErrSchemaNotReady)
	}
	if version != migrations.LatestVersion {
		return fmt.Errorf("%w: have version %d, require %d", ErrSchemaNotReady, version, migrations.LatestVersion)
	}
	return nil
}

// WithTx executes generated queries in one explicit transaction. Callback
// errors always roll back; only a successful commit returns nil.
func (s *Store) WithTx(ctx context.Context, options pgx.TxOptions, fn func(*dbgen.Queries) error) error {
	if fn == nil {
		return errors.New("transaction callback is required")
	}
	tx, err := s.pool.BeginTx(ctx, options)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(dbgen.New(tx)); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return errors.Join(err, fmt.Errorf("rollback transaction: %w", rollbackErr))
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}
