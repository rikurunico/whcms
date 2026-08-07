// Package db provides the pgx connection pool, the context-transaction
// Querier pattern, the ports.TxManager implementation and the
// migrate-on-start runner (CONTRACTS.md §6).
//
// Pattern: repositories hold *db.DB and always run SQL through
// d.Querier(ctx). When the context carries a transaction (started by
// TxManager.WithinTx) the query joins it; otherwise it runs on the pool.
package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier is the subset of pgx used by repositories; satisfied by both
// *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// DB wraps the pgx pool.
type DB struct {
	pool *pgxpool.Pool
}

// Connect opens a pgx pool and verifies connectivity.
func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return &DB{pool: pool}, nil
}

// Pool exposes the underlying pgxpool (health checks, advanced use).
func (d *DB) Pool() *pgxpool.Pool { return d.pool }

// Close closes the pool.
func (d *DB) Close() { d.pool.Close() }

// Ping verifies connectivity.
func (d *DB) Ping(ctx context.Context) error { return d.pool.Ping(ctx) }

type txCtxKey struct{}

// TxFromContext returns the transaction stored by TxManager.WithinTx, or nil.
func TxFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(txCtxKey{}).(pgx.Tx)
	return tx
}

// withTx stores tx in ctx.
func withTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txCtxKey{}, tx)
}

// Querier returns the context transaction when present, else the pool.
// All repository SQL must go through this.
func (d *DB) Querier(ctx context.Context) Querier {
	if tx := TxFromContext(ctx); tx != nil {
		return tx
	}
	return d.pool
}

// TxManager implements ports.TxManager over the pool.
type TxManager struct {
	db *DB
}

// NewTxManager builds a TxManager.
func NewTxManager(db *DB) *TxManager { return &TxManager{db: db} }

// WithinTx runs fn inside a transaction stored in the context. Nested calls
// join the outer transaction (no savepoints). Rollback on error, commit on
// success.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if TxFromContext(ctx) != nil {
		return fn(ctx) // join existing transaction
	}
	tx, err := m.db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: begin tx: %w", err)
	}
	txCtx := withTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: commit tx: %w", err)
	}
	return nil
}
