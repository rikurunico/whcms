//go:build integration

// Integration tests against the real local Postgres (whmcs DB, migrations
// applied). Every fixture uses uuid-suffixed identifiers and is cleaned up
// afterwards - shared/seeded data is never touched.
// Run: go test -tags integration ./internal/modules/networkstatus/
package networkstatus_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/networkstatus"
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
	if err != nil {
		t.Skipf("skipping integration test: cannot connect to postgres: %v", err)
	}
	t.Cleanup(d.Close)
	return d
}

func tag() string { return uuid.NewString()[:8] }

// newIssueFixture inserts a network issue and schedules a hard cleanup.
func newIssueFixture(t *testing.T, repo *networkstatus.Repo, d *db.DB, suffix string, mut func(*domain.NetworkIssue)) *domain.NetworkIssue {
	t.Helper()
	ctx := context.Background()
	n := &domain.NetworkIssue{
		Title:    "IT Network " + suffix,
		Body:     "body " + suffix,
		Type:     domain.NetworkTypeIssue,
		Severity: domain.NetworkSeverityMinor,
		Status:   domain.NetworkStatusInvestigating,
		Affected: "affected " + suffix,
		StartsAt: time.Now().UTC(),
	}
	if mut != nil {
		mut(n)
	}
	require.NoError(t, repo.Create(ctx, n))
	t.Cleanup(func() {
		_, _ = d.Pool().Exec(ctx, `DELETE FROM network_issues WHERE id = $1`, n.ID)
	})
	return n
}

func TestRepoIssueLifecycle(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := networkstatus.NewRepo(d)
	suffix := tag()

	n := newIssueFixture(t, repo, d, suffix, nil)
	require.NotZero(t, n.ID)
	require.False(t, n.CreatedAt.IsZero())

	got, err := repo.GetByID(ctx, n.ID)
	require.NoError(t, err)
	assert.Equal(t, n.Title, got.Title)
	assert.Equal(t, domain.NetworkStatusInvestigating, got.Status)

	// Update: move to resolved with an ends_at.
	end := time.Now().UTC().Truncate(time.Second)
	got.Title = "IT Network Updated " + suffix
	got.Type = domain.NetworkTypeOutage
	got.Severity = domain.NetworkSeverityCritical
	got.Status = domain.NetworkStatusResolved
	got.Affected = "web-01"
	got.EndsAt = &end
	require.NoError(t, repo.Update(ctx, got))

	again, err := repo.GetByID(ctx, n.ID)
	require.NoError(t, err)
	assert.Equal(t, "IT Network Updated "+suffix, again.Title)
	assert.Equal(t, domain.NetworkTypeOutage, again.Type)
	assert.Equal(t, domain.NetworkSeverityCritical, again.Severity)
	assert.Equal(t, domain.NetworkStatusResolved, again.Status)
	require.NotNil(t, again.EndsAt)

	// Soft delete hides it everywhere and is not repeatable.
	require.NoError(t, repo.SoftDelete(ctx, n.ID))
	_, err = repo.GetByID(ctx, n.ID)
	assertCode(t, err, apperr.CodeNotFound)
	err = repo.SoftDelete(ctx, n.ID)
	assertCode(t, err, apperr.CodeNotFound)
	err = repo.Update(ctx, again)
	assertCode(t, err, apperr.CodeNotFound)
}

func TestRepoListSearchStatusAndPaging(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := networkstatus.NewRepo(d)
	suffix := tag()

	// One investigating + one resolved, both tagged with suffix.
	investigating := newIssueFixture(t, repo, d, suffix+"-inv", nil)
	resolved := newIssueFixture(t, repo, d, suffix+"-res", func(n *domain.NetworkIssue) {
		n.Status = domain.NetworkStatusResolved
		end := time.Now().UTC()
		n.EndsAt = &end
	})

	// Search matches both (shared suffix in title/affected).
	rows, total, err := repo.List(ctx, ports.ListParams{Search: suffix, Sort: "-starts_at"})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(2))
	require.GreaterOrEqual(t, len(rows), 2)

	// Status filter narrows to a single row (search keeps it to our fixtures).
	rows, _, err = repo.List(ctx, ports.ListParams{Search: suffix, Status: "resolved"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, resolved.ID, rows[0].ID)

	rows, _, err = repo.List(ctx, ports.ListParams{Search: suffix, Status: "investigating"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, investigating.ID, rows[0].ID)

	// A bogus sort column falls back to the default order (no error).
	_, _, err = repo.List(ctx, ports.ListParams{Search: suffix, Sort: "bogus"})
	require.NoError(t, err)

	// Add a third row, then page with per_page=2 -> at most 2 rows returned.
	newIssueFixture(t, repo, d, suffix+"-third", nil)
	rows, total, err = repo.List(ctx, ports.ListParams{Search: suffix, Page: 1, PerPage: 2})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(3))
	assert.Len(t, rows, 2)
}

func TestRepoListActiveVisibility(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := networkstatus.NewRepo(d)
	suffix := tag()

	// Unresolved -> always visible.
	active := newIssueFixture(t, repo, d, suffix+"-active", nil)
	// Resolved recently (ends_at within 7 days) -> visible.
	recent := newIssueFixture(t, repo, d, suffix+"-recent", func(n *domain.NetworkIssue) {
		n.Status = domain.NetworkStatusResolved
		end := time.Now().Add(-24 * time.Hour).UTC()
		n.EndsAt = &end
	})
	// Resolved long ago (ends_at older than 7 days) -> hidden.
	old := newIssueFixture(t, repo, d, suffix+"-old", func(n *domain.NetworkIssue) {
		n.Status = domain.NetworkStatusResolved
		end := time.Now().Add(-30 * 24 * time.Hour).UTC()
		n.EndsAt = &end
	})

	rows, err := repo.ListActive(ctx)
	require.NoError(t, err)

	ids := map[int64]bool{}
	for _, r := range rows {
		ids[r.ID] = true
	}
	assert.True(t, ids[active.ID], "unresolved issue is visible")
	assert.True(t, ids[recent.ID], "recently-resolved issue is still visible")
	assert.False(t, ids[old.ID], "long-resolved issue is hidden")

	// Soft-deleting the active row removes it from the public view too.
	require.NoError(t, repo.SoftDelete(ctx, active.ID))
	rows, err = repo.ListActive(ctx)
	require.NoError(t, err)
	for _, r := range rows {
		assert.NotEqual(t, active.ID, r.ID, "soft-deleted issue is hidden from ListActive")
	}
}

// TestRepoCreateInvalidEnumIsValidation drives mapPgErr's check_violation
// branch: a bad enum value hits the table CHECK constraint -> VALIDATION.
func TestRepoCreateInvalidEnumIsValidation(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := networkstatus.NewRepo(d)

	err := repo.Create(ctx, &domain.NetworkIssue{
		Title:    "IT bad enum " + tag(),
		Type:     domain.NetworkIssueType("meteor"),
		Severity: domain.NetworkSeverityMinor,
		Status:   domain.NetworkStatusInvestigating,
		StartsAt: time.Now().UTC(),
	})
	assertCode(t, err, apperr.CodeValidation)
}

// TestRepoCanceledContextPropagatesErrors drives the generic "if err != nil"
// wrapping branch of every repo method by forcing pgx to fail fast on an
// already-canceled context, without ever reaching Postgres.
func TestRepoCanceledContextPropagatesErrors(t *testing.T) {
	d := testDB(t)
	repo := networkstatus.NewRepo(d)

	cctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.Error(t, repo.Create(cctx, &domain.NetworkIssue{}))
	_, err := repo.GetByID(cctx, 1)
	assert.Error(t, err)
	assert.Error(t, repo.Update(cctx, &domain.NetworkIssue{ID: 1}))
	_, _, err = repo.List(cctx, ports.ListParams{})
	assert.Error(t, err)
	_, err = repo.ListActive(cctx)
	assert.Error(t, err)
	assert.Error(t, repo.SoftDelete(cctx, 1))
}
