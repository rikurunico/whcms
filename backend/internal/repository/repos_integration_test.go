//go:build integration

// Integration tests against the real local Postgres (whmcs DB) with
// migrations applied. Run: go test -tags integration ./internal/repository/
package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/repository"

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

func TestSettingsRepoTypedGetters(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewSettingsRepo(testDB(t))

	// Seeded defaults (000002_seed_core).
	name, err := repo.GetString(ctx, "company.name", "fallback")
	require.NoError(t, err)
	assert.Equal(t, "WHCMS", name)

	days, err := repo.GetInt(ctx, "billing.invoice_due_days", -1)
	require.NoError(t, err)
	assert.Equal(t, 3, days)

	enabled, err := repo.GetBool(ctx, "billing.tax_enabled", true)
	require.NoError(t, err)
	assert.False(t, enabled)

	var reminders []int
	require.NoError(t, repo.GetJSON(ctx, "billing.reminder_days", &reminders))
	assert.Equal(t, []int{7, 3, 1}, reminders)

	// Missing keys return defaults.
	s, err := repo.GetString(ctx, "does.not.exist", "def")
	require.NoError(t, err)
	assert.Equal(t, "def", s)
	n, err := repo.GetInt(ctx, "does.not.exist", 99)
	require.NoError(t, err)
	assert.Equal(t, 99, n)
	b, err := repo.GetBool(ctx, "does.not.exist", true)
	require.NoError(t, err)
	assert.True(t, b)

	// Mistyped values return defaults.
	n, err = repo.GetInt(ctx, "company.name", 7) // string, not int
	require.NoError(t, err)
	assert.Equal(t, 7, n)
}

func TestSettingsRepoSetAndAll(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewSettingsRepo(testDB(t))

	require.NoError(t, repo.Set(ctx, "test.integration_key", map[string]any{"a": 1}))
	t.Cleanup(func() {
		_, _ = testDB(t).Querier(ctx).Exec(ctx, `DELETE FROM settings WHERE key = 'test.integration_key'`)
	})

	var got map[string]int
	require.NoError(t, repo.GetJSON(ctx, "test.integration_key", &got))
	assert.Equal(t, map[string]int{"a": 1}, got)

	// Upsert overwrites.
	require.NoError(t, repo.Set(ctx, "test.integration_key", "now-a-string"))
	s, err := repo.GetString(ctx, "test.integration_key", "")
	require.NoError(t, err)
	assert.Equal(t, "now-a-string", s)

	all, err := repo.All(ctx)
	require.NoError(t, err)
	keys := map[string]bool{}
	for _, row := range all {
		keys[row.Key] = true
	}
	assert.True(t, keys["company.name"])
	assert.True(t, keys["test.integration_key"])
}

func TestSettingsRepoJoinsTransaction(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := repository.NewSettingsRepo(d)
	tm := db.NewTxManager(d)

	// Rolled-back tx must not persist.
	sentinel := "test.tx_rollback_key"
	err := tm.WithinTx(ctx, func(txCtx context.Context) error {
		require.NoError(t, repo.Set(txCtx, sentinel, true))
		return assert.AnError
	})
	assert.ErrorIs(t, err, assert.AnError)

	got, err := repo.GetString(ctx, sentinel, "absent")
	require.NoError(t, err)
	assert.Equal(t, "absent", got, "rollback must discard the write")
}

func TestAuditRepoCreateAndList(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewAuditRepo(testDB(t))

	entry := &domain.AuditLog{
		Action: "test.integration", Entity: "test", EntityID: 123,
		Before: []byte(`{"v":1}`), After: []byte(`{"v":2}`), IP: "127.0.0.1",
	}
	require.NoError(t, repo.Create(ctx, entry))
	assert.NotZero(t, entry.ID)
	assert.False(t, entry.CreatedAt.IsZero())
	t.Cleanup(func() {
		_, _ = testDB(t).Querier(ctx).Exec(ctx, `DELETE FROM audit_logs WHERE action = 'test.integration'`)
	})

	rows, total, err := repo.List(ctx, ports.ListParams{Page: 1, PerPage: 10})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	require.NotEmpty(t, rows)
	assert.Equal(t, entry.ID, rows[0].ID, "newest first")
}

func TestEmailLogRepoLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewEmailLogRepo(testDB(t))

	e := &domain.EmailLogEntry{ToEmail: "it-test@example.com", TemplateKey: "verify_email", Subject: "Test"}
	require.NoError(t, repo.Create(ctx, e))
	assert.Equal(t, domain.EmailQueued, e.Status)
	t.Cleanup(func() {
		_, _ = testDB(t).Querier(ctx).Exec(ctx, `DELETE FROM email_log WHERE to_email = 'it-test@example.com'`)
	})

	got, err := repo.GetByID(ctx, e.ID)
	require.NoError(t, err)
	assert.Equal(t, "it-test@example.com", got.ToEmail)

	sentAt := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, repo.MarkSent(ctx, e.ID, sentAt))
	got, err = repo.GetByID(ctx, e.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.EmailSent, got.Status)
	require.NotNil(t, got.SentAt)

	require.NoError(t, repo.MarkFailed(ctx, e.ID, "smtp: boom"))
	got, _ = repo.GetByID(ctx, e.ID)
	assert.Equal(t, domain.EmailFailed, got.Status)
	assert.Equal(t, "smtp: boom", got.Error)

	rows, total, err := repo.List(ctx, ports.ListParams{Search: "it-test@", PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)

	// Missing rows.
	_, err = repo.GetByID(ctx, -1)
	assert.Error(t, err)
	assert.Error(t, repo.MarkSent(ctx, -1, sentAt))
	assert.Error(t, repo.MarkFailed(ctx, -1, "x"))
}

// TestEmailLogRepoUserScoping covers the user_id column added for per-user
// email history (§ Part C): Create persists it, GetByID/List round-trip it,
// and List's UserID filter scopes to just that recipient's rows.
func TestEmailLogRepoUserScoping(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := repository.NewEmailLogRepo(d)

	var userID int64
	require.NoError(t, d.Querier(ctx).QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role) VALUES ($1, 'x', 'client')
		RETURNING id`, "emaillog-it@example.com").Scan(&userID))
	t.Cleanup(func() {
		_, _ = d.Querier(ctx).Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	})

	owned := &domain.EmailLogEntry{UserID: &userID, ToEmail: "owned@example.com", TemplateKey: "invoice_created", Subject: "Owned"}
	require.NoError(t, repo.Create(ctx, owned))
	other := &domain.EmailLogEntry{ToEmail: "unowned@example.com", TemplateKey: "admin_alert", Subject: "Unowned"}
	require.NoError(t, repo.Create(ctx, other))
	t.Cleanup(func() {
		_, _ = d.Querier(ctx).Exec(ctx, `DELETE FROM email_log WHERE to_email IN ('owned@example.com', 'unowned@example.com')`)
	})

	got, err := repo.GetByID(ctx, owned.ID)
	require.NoError(t, err)
	require.NotNil(t, got.UserID)
	assert.Equal(t, userID, *got.UserID)

	got, err = repo.GetByID(ctx, other.ID)
	require.NoError(t, err)
	assert.Nil(t, got.UserID)

	rows, total, err := repo.List(ctx, ports.ListParams{UserID: userID, PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, "owned@example.com", rows[0].ToEmail)
}

// canceledCtx returns a context that is already canceled, used to force
// pgx query/exec/scan calls down their error-return paths without touching
// any DB state.
func canceledCtx() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func TestSettingsRepoMistypedAndMissingValues(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewSettingsRepo(testDB(t))

	// GetString on a non-string JSON value (billing.invoice_due_days is a
	// number) must fall back to def instead of erroring.
	s, err := repo.GetString(ctx, "billing.invoice_due_days", "fallback")
	require.NoError(t, err)
	assert.Equal(t, "fallback", s)

	// GetBool on a non-bool JSON value (company.name is a string) must fall
	// back to def.
	b, err := repo.GetBool(ctx, "company.name", true)
	require.NoError(t, err)
	assert.True(t, b)

	// GetJSON on a missing key must leave out untouched and return nil error.
	out := map[string]int{"untouched": 1}
	require.NoError(t, repo.GetJSON(ctx, "does.not.exist", &out))
	assert.Equal(t, map[string]int{"untouched": 1}, out)

	// GetJSON with a target type incompatible with the stored value
	// (company.name is a JSON string, not an array) must return an error.
	var wrongType []int
	err = repo.GetJSON(ctx, "company.name", &wrongType)
	assert.Error(t, err)
}

func TestSettingsRepoSetMarshalError(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewSettingsRepo(testDB(t))

	// A channel cannot be JSON-marshaled, so Set must fail before ever
	// reaching the database.
	err := repo.Set(ctx, "test.unused_marshal_error_key", map[string]any{"bad": make(chan int)})
	assert.Error(t, err)
}

func TestSettingsRepoCanceledContextErrors(t *testing.T) {
	repo := repository.NewSettingsRepo(testDB(t))
	ctx := canceledCtx()

	_, err := repo.GetString(ctx, "company.name", "def")
	assert.Error(t, err)
	_, err = repo.GetInt(ctx, "company.name", 0)
	assert.Error(t, err)
	_, err = repo.GetBool(ctx, "company.name", false)
	assert.Error(t, err)
	assert.Error(t, repo.GetJSON(ctx, "company.name", &struct{}{}))
	assert.Error(t, repo.Set(ctx, "test.unused_canceled_key", "v"))
	_, err = repo.All(ctx)
	assert.Error(t, err)
}

func TestAuditRepoCanceledContextErrors(t *testing.T) {
	repo := repository.NewAuditRepo(testDB(t))
	ctx := canceledCtx()

	err := repo.Create(ctx, &domain.AuditLog{Action: "x", Entity: "y"})
	assert.Error(t, err)

	_, _, err = repo.List(ctx, ports.ListParams{Page: 1, PerPage: 10})
	assert.Error(t, err)
}

func TestEmailLogRepoCanceledContextErrors(t *testing.T) {
	repo := repository.NewEmailLogRepo(testDB(t))
	ctx := canceledCtx()

	err := repo.Create(ctx, &domain.EmailLogEntry{ToEmail: "x@example.com"})
	assert.Error(t, err)

	_, err = repo.GetByID(ctx, 1)
	assert.Error(t, err)
	assert.NotEqual(t, "email log entry not found", err.Error(), "must be a real DB error, not ErrNoRows mapping")

	assert.Error(t, repo.MarkSent(ctx, 1, time.Now()))
	assert.Error(t, repo.MarkFailed(ctx, 1, "boom"))

	_, _, err = repo.List(ctx, ports.ListParams{PerPage: 10})
	assert.Error(t, err)
}

func TestIntegrationLogRepoCanceledContextErrors(t *testing.T) {
	repo := repository.NewIntegrationLogRepo(testDB(t))
	ctx := canceledCtx()

	err := repo.Create(ctx, &domain.IntegrationLog{Provider: "x"})
	assert.Error(t, err)

	_, _, err = repo.List(ctx, ports.ListParams{PerPage: 10})
	assert.Error(t, err)
}

func TestIntegrationLogRepoCreateAndList(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewIntegrationLogRepo(testDB(t))

	l := &domain.IntegrationLog{
		Provider: "test-provider", Endpoint: "/x", Method: "POST",
		StatusCode: 200, Success: true, LatencyMS: 42,
		Request: []byte(`{"a":1}`), Response: []byte(`{"b":2}`),
	}
	require.NoError(t, repo.Create(ctx, l))
	assert.NotZero(t, l.ID)
	t.Cleanup(func() {
		_, _ = testDB(t).Querier(ctx).Exec(ctx, `DELETE FROM integration_logs WHERE provider = 'test-provider'`)
	})

	rows, total, err := repo.List(ctx, ports.ListParams{Search: "test-provider", PerPage: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, "/x", rows[0].Endpoint)
}
