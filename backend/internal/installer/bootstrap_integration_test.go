//go:build integration

package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These exercise the bootstrap handlers' success paths against the real
// local Postgres/Redis/RustFS/mockserver (docs/E2E.md), complementing the
// unit tests in bootstrap_test.go which only cover the validation-failure
// branches without needing live infra.

func testDatabaseURL() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}
	return "postgres://root:postgres@localhost:5432/whmcs_e2e?sslmode=disable"
}

func TestIntegrationTestDBSucceeds(t *testing.T) {
	app, _ := newBootstrapApp(discardLogger(), filepath.Join(t.TempDir(), "app.env"), nil)
	status, env := doJSON(t, app, "POST", "/api/v1/install/bootstrap/test-db",
		`{"database_url":"`+testDatabaseURL()+`"}`)
	require.Equal(t, fiber.StatusOK, status, "response: %s", env.Data)
	assert.Nil(t, env.Error)
}

func TestIntegrationTestRedisSucceeds(t *testing.T) {
	app, _ := newBootstrapApp(discardLogger(), filepath.Join(t.TempDir(), "app.env"), nil)
	status, env := doJSON(t, app, "POST", "/api/v1/install/bootstrap/test-redis",
		`{"redis_addr":"localhost:6379"}`)
	require.Equal(t, fiber.StatusOK, status, "response: %s", env.Data)
	assert.Nil(t, env.Error)
}

func TestIntegrationTestS3Succeeds(t *testing.T) {
	app, _ := newBootstrapApp(discardLogger(), filepath.Join(t.TempDir(), "app.env"), nil)
	body, err := json.Marshal(testS3Request{
		RustFSEndpoint:  "http://localhost:9000",
		RustFSAccessKey: "rustfsadmin",
		RustFSSecretKey: "rustfsadmin",
		RustFSBucket:    "whmcs",
		RustFSUseSSL:    false,
	})
	require.NoError(t, err)
	status, env := doJSON(t, app, "POST", "/api/v1/install/bootstrap/test-s3", string(body))
	require.Equal(t, fiber.StatusOK, status, "response: %s", env.Data)
	assert.Nil(t, env.Error)
}

func TestIntegrationTestMailHTTPSucceeds(t *testing.T) {
	app, _ := newBootstrapApp(discardLogger(), filepath.Join(t.TempDir(), "app.env"), nil)
	body, err := json.Marshal(testMailRequest{
		MailDriver:    "http",
		MailHTTPURL:   "http://localhost:9090/mail/send",
		TestRecipient: "installer-bootstrap-test@example.test",
	})
	require.NoError(t, err)
	status, env := doJSON(t, app, "POST", "/api/v1/install/bootstrap/test-mail", string(body))
	require.Equal(t, fiber.StatusOK, status, "response: %s", env.Data)
	assert.Nil(t, env.Error)
}
