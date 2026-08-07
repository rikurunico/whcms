//go:build integration

// Integration test for the composition root against the real local Postgres
// (whmcs DB, migrations applied) - same convention as
// internal/repository/repos_integration_test.go. Build itself performs no
// writes (every constructor here only stores the *db.DB handle), so no
// fixture rows are created and nothing needs cleanup.
package composition

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/tsdlamongan/whcms/backend/internal/platform/config"
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
	d, err := db.Connect(context.Background(), url)
	require.NoError(t, err, "integration tests need the local whmcs database")
	t.Cleanup(d.Close)
	return d
}

func validConfig() config.Config {
	return config.Config{
		AppEnv:             "test",
		AppPort:            8080,
		AppBaseURL:         "http://localhost:8080",
		FrontendURL:        "http://localhost:5173",
		JWTSecret:          "test-jwt-secret",
		EncryptionKey:      make([]byte, 32), // 32 zero bytes: valid AES-256 key length
		DatabaseURL:        "postgres://root:postgres@localhost:5432/whmcs_e2e?sslmode=disable",
		RedisAddr:          "localhost:6379",
		DuitkuMerchantCode: "MC1",
		DuitkuAPIKey:       "key",
		DuitkuEnv:          "sandbox",
		RDashResellerID:    "reseller",
		RDashAPIKey:        "key",
		RDashBaseURL:       "https://api.rdash.id/v1",
		MailDriver:         "log",
		WorkerConcurrency:  10,
		AdminAlertEmail:    "admin@example.com",
	}
}

// TestBuild_Success wires the full App against a real DB connection (Redis
// and S3 handles are lazy/never dialed during Build - see docs/WIRING.md
// §1) and asserts every field is populated.
func TestBuild_Success(t *testing.T) {
	database := testDB(t)
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	t.Cleanup(func() { _ = rdb.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	app, err := Build(context.Background(), validConfig(), database, rdb, nil, log)
	require.NoError(t, err)
	require.NotNil(t, app)

	assert.NotNil(t, app.Auth)
	assert.NotNil(t, app.Clients)
	assert.NotNil(t, app.Catalog)
	assert.NotNil(t, app.Orders)
	assert.NotNil(t, app.Billing)
	assert.NotNil(t, app.Payments)
	assert.NotNil(t, app.Provisioning)
	assert.NotNil(t, app.Domains)
	assert.NotNil(t, app.Tickets)
	assert.NotNil(t, app.Notifications)
	assert.NotNil(t, app.AdminOps)
	assert.NotNil(t, app.Settings)
	assert.NotNil(t, app.AuthUsersRepo)
	assert.NotNil(t, app.Tokens)
	assert.NotNil(t, app.Enqueuer)
}
