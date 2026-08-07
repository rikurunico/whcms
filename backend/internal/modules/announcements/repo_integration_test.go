//go:build integration

// Integration tests against the real local Postgres (whmcs DB, migrations
// applied). Every fixture uses uuid-suffixed identifiers and is cleaned up
// afterwards - shared/seeded data is never touched.
// Run: go test -tags integration ./internal/modules/announcements/
package announcements_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/announcements"
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

// newAnnouncementFixture inserts an announcement and schedules a hard cleanup.
func newAnnouncementFixture(t *testing.T, repo *announcements.Repo, d *db.DB, suffix string, mut func(*domain.Announcement)) *domain.Announcement {
	t.Helper()
	ctx := context.Background()
	a := &domain.Announcement{
		Title: "IT Announcement " + suffix,
		Slug:  "it-announcement-" + suffix,
		Body:  "body " + suffix,
	}
	if mut != nil {
		mut(a)
	}
	require.NoError(t, repo.Create(ctx, a))
	t.Cleanup(func() {
		_, _ = d.Pool().Exec(ctx, `DELETE FROM announcements WHERE id = $1`, a.ID)
	})
	return a
}

func TestRepoAnnouncementLifecycle(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := announcements.NewRepo(d)
	suffix := tag()

	a := newAnnouncementFixture(t, repo, d, suffix, nil)
	require.NotZero(t, a.ID)

	got, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, a.Slug, got.Slug)

	bySlug, err := repo.GetBySlug(ctx, a.Slug)
	require.NoError(t, err)
	assert.Equal(t, a.ID, bySlug.ID)

	// Duplicate slug -> CONFLICT (unique_violation).
	dup := *a
	dup.ID = 0
	err = repo.Create(ctx, &dup)
	assertCode(t, err, apperr.CodeConflict)

	// Update.
	got.Title = "IT Announcement Updated " + suffix
	got.Published = true
	now := time.Now().UTC().Truncate(time.Second)
	got.PublishedAt = &now
	require.NoError(t, repo.Update(ctx, got))
	again, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	assert.True(t, again.Published)
	require.NotNil(t, again.PublishedAt)

	// List (admin) with search finds it.
	rows, total, err := repo.List(ctx, ports.ListParams{Search: "it-announcement-" + suffix})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))
	require.NotEmpty(t, rows)

	// Status filter: it is now published.
	rows, _, err = repo.List(ctx, ports.ListParams{Search: "it-announcement-" + suffix, Status: "published"})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	rows, _, err = repo.List(ctx, ports.ListParams{Search: "it-announcement-" + suffix, Status: "draft"})
	require.NoError(t, err)
	assert.Empty(t, rows)

	// Soft delete hides it everywhere.
	require.NoError(t, repo.SoftDelete(ctx, a.ID))
	_, err = repo.GetByID(ctx, a.ID)
	assertCode(t, err, apperr.CodeNotFound)
	_, err = repo.GetBySlug(ctx, a.Slug)
	assertCode(t, err, apperr.CodeNotFound)
	err = repo.SoftDelete(ctx, a.ID)
	assertCode(t, err, apperr.CodeNotFound)
	err = repo.Update(ctx, got)
	assertCode(t, err, apperr.CodeNotFound)
}

func TestRepoListPublishedVisibility(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := announcements.NewRepo(d)
	suffix := tag()

	past := time.Now().Add(-time.Hour).UTC()
	future := time.Now().Add(time.Hour).UTC()

	live := newAnnouncementFixture(t, repo, d, suffix+"-live", func(a *domain.Announcement) {
		a.Published = true
		a.PublishedAt = &past
	})
	draft := newAnnouncementFixture(t, repo, d, suffix+"-draft", func(a *domain.Announcement) {
		a.Published = false
	})
	scheduled := newAnnouncementFixture(t, repo, d, suffix+"-sched", func(a *domain.Announcement) {
		a.Published = true
		a.PublishedAt = &future
	})

	rows, total, err := repo.ListPublished(ctx, ports.ListParams{Page: 1, PerPage: 100})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(1))

	ids := map[int64]bool{}
	for _, r := range rows {
		ids[r.ID] = true
	}
	assert.True(t, ids[live.ID], "published, past publish time is visible")
	assert.False(t, ids[draft.ID], "draft is hidden from the public view")
	assert.False(t, ids[scheduled.ID], "future publish time is hidden until it passes")
}

func TestRepoListPaging(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := announcements.NewRepo(d)
	suffix := tag()

	past := time.Now().Add(-time.Hour).UTC()
	for i := 0; i < 3; i++ {
		newAnnouncementFixture(t, repo, d, suffix+"-"+tag(), func(a *domain.Announcement) {
			a.Published = true
			a.PublishedAt = &past
		})
	}

	// Page 1 with per_page=2 returns at most 2 rows.
	rows, total, err := repo.ListPublished(ctx, ports.ListParams{Page: 1, PerPage: 2})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, total, int64(3))
	assert.Len(t, rows, 2)
}

func TestRepoForeignKeyViolation(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := announcements.NewRepo(d)
	suffix := tag()

	// author_id references a non-existent user -> CONFLICT (foreign_key_violation).
	badAuthor := int64(999_999_999)
	err := repo.Create(ctx, &domain.Announcement{
		Title:    "IT FK " + suffix,
		Slug:     "it-fk-" + suffix,
		AuthorID: &badAuthor,
	})
	assertCode(t, err, apperr.CodeConflict)
}

// TestRepoCanceledContextPropagatesErrors drives the generic "if err != nil"
// wrapping branch of every repo method by forcing pgx to fail fast on an
// already-canceled context, without ever reaching Postgres.
func TestRepoCanceledContextPropagatesErrors(t *testing.T) {
	d := testDB(t)
	repo := announcements.NewRepo(d)

	cctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.Error(t, repo.Create(cctx, &domain.Announcement{}))
	_, err := repo.GetByID(cctx, 1)
	assert.Error(t, err)
	_, err = repo.GetBySlug(cctx, "x")
	assert.Error(t, err)
	assert.Error(t, repo.Update(cctx, &domain.Announcement{ID: 1}))
	_, _, err = repo.List(cctx, ports.ListParams{})
	assert.Error(t, err)
	_, _, err = repo.ListPublished(cctx, ports.ListParams{})
	assert.Error(t, err)
	assert.Error(t, repo.SoftDelete(cctx, 1))
}
