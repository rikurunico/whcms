//go:build integration

// Integration tests for Migrate/MigrateDown against the real local Postgres
// cluster. Each test creates its own throwaway, uuid-suffixed database via
// the admin ("postgres") maintenance database and drops it during cleanup -
// the shared whmcs database and its seeded rows are never touched.
// Run: go test -tags integration ./internal/platform/db/
package db_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
	"github.com/tsdlamongan/whcms/backend/migrations"

	"github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adminDB connects to the cluster's maintenance database (never whmcs) so
// tests can issue CREATE DATABASE / DROP DATABASE for their own throwaway
// fixtures without touching any shared schema or rows.
func adminDB(t *testing.T) *db.DB {
	t.Helper()
	url := os.Getenv("DATABASE_ADMIN_URL")
	if url == "" {
		url = "postgres://root:postgres@localhost:5432/postgres?sslmode=disable"
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

// newEphemeralDatabaseURL creates a uuid-suffixed throwaway database on the
// local cluster and returns its connection URL. The database (and anything
// created inside it) is dropped via t.Cleanup - nothing shared is touched.
func newEphemeralDatabaseURL(t *testing.T) string {
	t.Helper()
	admin := adminDB(t)
	ctx := context.Background()
	name := "whcms_it_" + strings.ReplaceAll(uuid.NewString(), "-", "")

	_, err := admin.Querier(ctx).Exec(ctx, `CREATE DATABASE `+name)
	require.NoError(t, err)
	t.Cleanup(func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = admin.Querier(dropCtx).Exec(dropCtx, `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`)
	})
	return fmt.Sprintf("postgres://root:postgres@localhost:5432/%s?sslmode=disable", name)
}

func TestMigrateAppliesAndIsIdempotent(t *testing.T) {
	url := newEphemeralDatabaseURL(t)

	require.NoError(t, db.Migrate(url), "first Migrate applies all embedded migrations")

	ctx := context.Background()
	edb, err := db.Connect(ctx, url)
	require.NoError(t, err)
	defer edb.Close()

	var n int
	require.NoError(t, edb.Querier(ctx).QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'users'`).Scan(&n))
	assert.Equal(t, 1, n, "users table must exist after migrating up")

	// Idempotent: re-running on an up-to-date schema must not error
	// (golang-migrate's ErrNoChange is swallowed).
	assert.NoError(t, db.Migrate(url), "second Migrate call must be a no-op, not an error")
}

// migrationCount counts the embedded *.up.sql files so the full-rollback
// test never goes stale when a new migration is added.
func migrationCount(t *testing.T) int {
	t.Helper()
	entries, err := migrations.FS.ReadDir(".")
	require.NoError(t, err)
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			n++
		}
	}
	return n
}

func TestMigrateDownRollsBack(t *testing.T) {
	url := newEphemeralDatabaseURL(t)
	require.NoError(t, db.Migrate(url))

	require.NoError(t, db.MigrateDown(url, migrationCount(t)), "rolling back every migration must succeed")

	ctx := context.Background()
	edb, err := db.Connect(ctx, url)
	require.NoError(t, err)
	defer edb.Close()

	var n int
	require.NoError(t, edb.Querier(ctx).QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'users'`).Scan(&n))
	assert.Equal(t, 0, n, "users table must be gone after rolling back all migrations")
}

func TestMigrateInitError(t *testing.T) {
	err := db.Migrate("://not-a-valid-url")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db: migrate init:")
}

func TestMigrateDownInitError(t *testing.T) {
	err := db.MigrateDown("://not-a-valid-url", 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db: migrate init:")
}

func TestMigrateUpGenuineError(t *testing.T) {
	url := newEphemeralDatabaseURL(t)
	ctx := context.Background()
	edb, err := db.Connect(ctx, url)
	require.NoError(t, err)
	// Pre-create a conflicting table so the first embedded migration's
	// `CREATE TABLE users` fails with a real (non-ErrNoChange) error.
	_, err = edb.Querier(ctx).Exec(ctx, `CREATE TABLE users (id INT)`)
	require.NoError(t, err)
	edb.Close()

	err = db.Migrate(url)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db: migrate up:")
	assert.False(t, errors.Is(err, migrate.ErrNoChange))
}

func TestMigrateDownGenuineError(t *testing.T) {
	url := newEphemeralDatabaseURL(t)
	// No migrations have ever been applied to this fresh database, so
	// asking to roll back steps returns a real error, not ErrNoChange.
	err := db.MigrateDown(url, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db: migrate down:")
}
