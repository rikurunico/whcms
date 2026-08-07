package db_test

// These tests use the real local Postgres. They auto-skip when the database
// is unreachable, so the unit gate stays green without infrastructure.

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testDB(t *testing.T) *db.DB {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = "postgres://root:postgres@localhost:5432/whmcs_e2e?sslmode=disable"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	d, err := db.Connect(ctx, url)
	if err != nil {
		t.Skipf("postgres not available, skipping: %v", err)
	}
	t.Cleanup(d.Close)
	return d
}

func TestConnectInvalidURL(t *testing.T) {
	_, err := db.Connect(context.Background(), "not-a-url://%%%")
	assert.Error(t, err)
}

func TestConnectPingFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// URL parses fine but the target database does not exist, so pgxpool
	// connects successfully at the wire level and only the Ping call fails.
	_, err := db.Connect(ctx, "postgres://root:postgres@localhost:5432/whcms_definitely_missing_db?sslmode=disable")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db: ping:")
}

func TestQuerierOnPool(t *testing.T) {
	d := testDB(t)
	var one int
	err := d.Querier(context.Background()).QueryRow(context.Background(), "SELECT 1").Scan(&one)
	require.NoError(t, err)
	assert.Equal(t, 1, one)
	assert.NoError(t, d.Ping(context.Background()))
	assert.NotNil(t, d.Pool())
}

func TestTxManagerCommitAndRollback(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()
	tm := db.NewTxManager(d)

	_, err := d.Querier(ctx).Exec(ctx, `CREATE TABLE IF NOT EXISTS tx_test (v TEXT)`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = d.Querier(context.Background()).Exec(context.Background(), `DROP TABLE IF EXISTS tx_test`)
	})

	t.Run("commit persists", func(t *testing.T) {
		err := tm.WithinTx(ctx, func(txCtx context.Context) error {
			require.NotNil(t, db.TxFromContext(txCtx), "tx must be in context")
			_, err := d.Querier(txCtx).Exec(txCtx, `INSERT INTO tx_test (v) VALUES ('committed')`)
			return err
		})
		require.NoError(t, err)
		var n int
		require.NoError(t, d.Querier(ctx).QueryRow(ctx,
			`SELECT count(*) FROM tx_test WHERE v='committed'`).Scan(&n))
		assert.Equal(t, 1, n)
	})

	t.Run("error rolls back", func(t *testing.T) {
		boom := errors.New("boom")
		err := tm.WithinTx(ctx, func(txCtx context.Context) error {
			_, err := d.Querier(txCtx).Exec(txCtx, `INSERT INTO tx_test (v) VALUES ('rolled-back')`)
			require.NoError(t, err)
			return boom
		})
		assert.ErrorIs(t, err, boom)
		var n int
		require.NoError(t, d.Querier(ctx).QueryRow(ctx,
			`SELECT count(*) FROM tx_test WHERE v='rolled-back'`).Scan(&n))
		assert.Equal(t, 0, n)
	})

	t.Run("nested joins outer tx", func(t *testing.T) {
		err := tm.WithinTx(ctx, func(outer context.Context) error {
			outerTx := db.TxFromContext(outer)
			return tm.WithinTx(outer, func(inner context.Context) error {
				assert.Equal(t, outerTx, db.TxFromContext(inner), "inner must join outer tx")
				return nil
			})
		})
		assert.NoError(t, err)
	})
}

func TestTxFromContextEmpty(t *testing.T) {
	assert.Nil(t, db.TxFromContext(context.Background()))
}

func TestWithinTxBeginError(t *testing.T) {
	d := testDB(t)
	tm := db.NewTxManager(d)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already done before WithinTx even calls pool.Begin

	called := false
	err := tm.WithinTx(ctx, func(context.Context) error {
		called = true
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db: begin tx:")
	assert.False(t, called, "fn must not run when Begin fails")
}

func TestWithinTxCommitError(t *testing.T) {
	d := testDB(t)
	tm := db.NewTxManager(d)

	// The deadline expires between the (successful) Begin and the Commit
	// call, which reuses the same outer context, so Commit fails.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := tm.WithinTx(ctx, func(context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db: commit tx:")
}
