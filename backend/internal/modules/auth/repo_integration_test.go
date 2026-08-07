//go:build integration

// Integration tests for UserRepo against the real local Postgres (whmcs DB,
// migrations applied). Run: go test -tags integration ./internal/modules/auth/
// Fixtures use uuid-suffixed emails and are cleaned up per test; shared/seeded
// rows are never touched.
package auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/auth"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/google/uuid"
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

// newFixtureUser inserts a unique user and registers cleanup.
func newFixtureUser(t *testing.T, repo *auth.UserRepo, mutate func(*domain.User)) *domain.User {
	t.Helper()
	u := &domain.User{
		Email:        fmt.Sprintf("auth-it-%s@example.com", uuid.NewString()),
		PasswordHash: "hashed:integration",
		Role:         domain.RoleClient,
		Status:       domain.UserActive,
		Locale:       "id",
	}
	if mutate != nil {
		mutate(u)
	}
	require.NoError(t, repo.Create(context.Background(), u))
	t.Cleanup(func() { _ = repo.Delete(context.Background(), u.ID) })
	return u
}

func TestUserRepoCreateAndGet(t *testing.T) {
	ctx := context.Background()
	repo := auth.NewUserRepo(testDB(t))

	u := newFixtureUser(t, repo, nil)
	require.NotZero(t, u.ID)
	assert.False(t, u.CreatedAt.IsZero())
	assert.Equal(t, json.RawMessage(`{}`), u.Permissions)

	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, u.Email, got.Email)
	assert.Equal(t, domain.RoleClient, got.Role)
	assert.Equal(t, "id", got.Locale)
	assert.Nil(t, got.EmailVerifiedAt)

	// Case-insensitive email lookup.
	byEmail, err := repo.GetByEmail(ctx, "AUTH-IT-"+u.Email[8:])
	require.NoError(t, err)
	assert.Equal(t, u.ID, byEmail.ID)

	_, err = repo.GetByID(ctx, -1)
	assert.ErrorIs(t, err, apperr.New(apperr.CodeNotFound, ""))
	_, err = repo.GetByEmail(ctx, "missing-"+u.Email)
	assert.ErrorIs(t, err, apperr.New(apperr.CodeNotFound, ""))
}

func TestUserRepoDuplicateEmailConflict(t *testing.T) {
	ctx := context.Background()
	repo := auth.NewUserRepo(testDB(t))

	u := newFixtureUser(t, repo, nil)
	dup := &domain.User{
		Email:        u.Email,
		PasswordHash: "hashed:x",
		Role:         domain.RoleClient,
		Status:       domain.UserActive,
	}
	err := repo.Create(ctx, dup)
	assert.ErrorIs(t, err, apperr.New(apperr.CodeConflict, ""))
}

func TestUserRepoUpdate(t *testing.T) {
	ctx := context.Background()
	repo := auth.NewUserRepo(testDB(t))

	u := newFixtureUser(t, repo, nil)
	u.Locale = "en"
	u.TwoFASecretEnc = "enc-secret"
	u.TwoFAEnabled = true
	u.Permissions = json.RawMessage(`{"billing":true}`)
	u.Role = domain.RoleStaff
	require.NoError(t, repo.Update(ctx, u))

	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "en", got.Locale)
	assert.Equal(t, "enc-secret", got.TwoFASecretEnc)
	assert.True(t, got.TwoFAEnabled)
	assert.Equal(t, domain.RoleStaff, got.Role)
	assert.JSONEq(t, `{"billing":true}`, string(got.Permissions))

	missing := *got
	missing.ID = -1
	assert.ErrorIs(t, repo.Update(ctx, &missing), apperr.New(apperr.CodeNotFound, ""))
}

func TestUserRepoPasswordVerifiedLastLogin(t *testing.T) {
	ctx := context.Background()
	repo := auth.NewUserRepo(testDB(t))

	u := newFixtureUser(t, repo, nil)
	require.NoError(t, repo.UpdatePassword(ctx, u.ID, "hashed:changed"))

	at := time.Date(2026, 7, 3, 9, 30, 0, 0, time.UTC)
	require.NoError(t, repo.SetEmailVerified(ctx, u.ID, at))
	require.NoError(t, repo.SetLastLogin(ctx, u.ID, at))

	got, err := repo.GetByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "hashed:changed", got.PasswordHash)
	require.NotNil(t, got.EmailVerifiedAt)
	assert.True(t, got.EmailVerifiedAt.Equal(at))
	require.NotNil(t, got.LastLoginAt)
	assert.True(t, got.LastLoginAt.Equal(at))

	assert.ErrorIs(t, repo.UpdatePassword(ctx, -1, "x"), apperr.New(apperr.CodeNotFound, ""))
	assert.ErrorIs(t, repo.SetEmailVerified(ctx, -1, at), apperr.New(apperr.CodeNotFound, ""))
	assert.ErrorIs(t, repo.SetLastLogin(ctx, -1, at), apperr.New(apperr.CodeNotFound, ""))
}

func TestUserRepoListAndDelete(t *testing.T) {
	ctx := context.Background()
	repo := auth.NewUserRepo(testDB(t))

	u1 := newFixtureUser(t, repo, nil)
	u2 := newFixtureUser(t, repo, func(u *domain.User) { u.Status = domain.UserInactive })

	// Search narrows to our unique fixture rows.
	frag := u1.Email[:20]
	users, total, err := repo.List(ctx, ports.ListParams{Search: frag})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, users, 1)
	assert.Equal(t, u1.ID, users[0].ID)

	// Status filter.
	_, total, err = repo.List(ctx, ports.ListParams{Search: u2.Email, Status: "inactive"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	users, total, err = repo.List(ctx, ports.ListParams{Search: u2.Email, Status: "active"})
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, users)

	// Delete removes the row; second delete is NOT_FOUND.
	require.NoError(t, repo.Delete(ctx, u2.ID))
	assert.ErrorIs(t, repo.Delete(ctx, u2.ID), apperr.New(apperr.CodeNotFound, ""))
}

// TestUserRepoExistsAnyAdmin only asserts the "true" branch: the shared
// whmcs_e2e database already has admin rows from cmd/seed and other test
// runs, so there is no way to observe "no admin exists" here without a fresh
// empty database. The "false" case (and the installer's gating logic around
// it) is covered with a fully-controlled mock in the install module's
// service tests instead.
func TestUserRepoExistsAnyAdmin(t *testing.T) {
	ctx := context.Background()
	repo := auth.NewUserRepo(testDB(t))

	newFixtureUser(t, repo, func(u *domain.User) { u.Role = domain.RoleAdmin })

	exists, err := repo.ExistsAnyAdmin(ctx)
	require.NoError(t, err)
	assert.True(t, exists)
}
