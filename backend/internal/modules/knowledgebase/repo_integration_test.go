//go:build integration

// Integration tests against the real local Postgres (whmcs DB, migrations
// applied). Every fixture uses uuid-suffixed identifiers and is cleaned up
// afterwards - shared/seeded data is never touched.
// Run: go test -tags integration ./internal/modules/knowledgebase/
package knowledgebase_test

import (
	"context"
	"os"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/knowledgebase"
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

// newCategoryFixture inserts a category and schedules a hard cleanup.
func newCategoryFixture(t *testing.T, repo *knowledgebase.Repo, d *db.DB, suffix string) *domain.KBCategory {
	t.Helper()
	ctx := context.Background()
	c := &domain.KBCategory{Name: "IT Cat " + suffix, Slug: "it-cat-" + suffix, Description: "d", Sort: 1}
	require.NoError(t, repo.CreateCategory(ctx, c))
	t.Cleanup(func() {
		_, _ = d.Pool().Exec(ctx, `DELETE FROM kb_categories WHERE id = $1`, c.ID)
	})
	return c
}

func newArticleFixture(t *testing.T, repo *knowledgebase.Repo, d *db.DB, categoryID int64, suffix string, mut func(*domain.KBArticle)) *domain.KBArticle {
	t.Helper()
	ctx := context.Background()
	a := &domain.KBArticle{
		CategoryID: categoryID,
		Title:      "IT Article " + suffix,
		Slug:       "it-article-" + suffix,
		Body:       "body",
	}
	if mut != nil {
		mut(a)
	}
	require.NoError(t, repo.CreateArticle(ctx, a))
	t.Cleanup(func() {
		_, _ = d.Pool().Exec(ctx, `DELETE FROM kb_articles WHERE id = $1`, a.ID)
	})
	return a
}

func TestRepoCategoryLifecycle(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := knowledgebase.NewRepo(d)
	suffix := tag()

	c := newCategoryFixture(t, repo, d, suffix)
	require.NotZero(t, c.ID)

	got, err := repo.GetCategoryByID(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, c.Slug, got.Slug)

	bySlug, err := repo.GetCategoryBySlug(ctx, c.Slug)
	require.NoError(t, err)
	assert.Equal(t, c.ID, bySlug.ID)

	// Duplicate slug -> CONFLICT.
	dup := *c
	dup.ID = 0
	err = repo.CreateCategory(ctx, &dup)
	assertCode(t, err, apperr.CodeConflict)

	// Update.
	got.Name = "IT Cat Updated " + suffix
	got.Hidden = true
	require.NoError(t, repo.UpdateCategory(ctx, got))
	again, err := repo.GetCategoryByID(ctx, c.ID)
	require.NoError(t, err)
	assert.True(t, again.Hidden)

	// includeHidden toggles visibility.
	all, err := repo.ListCategories(ctx, true)
	require.NoError(t, err)
	found := false
	for _, row := range all {
		if row.ID == c.ID {
			found = true
		}
	}
	assert.True(t, found)

	visible, err := repo.ListCategories(ctx, false)
	require.NoError(t, err)
	for _, row := range visible {
		assert.NotEqual(t, c.ID, row.ID, "hidden category excluded from visible listing")
	}

	// Soft delete hides it everywhere.
	require.NoError(t, repo.SoftDeleteCategory(ctx, c.ID))
	_, err = repo.GetCategoryByID(ctx, c.ID)
	assertCode(t, err, apperr.CodeNotFound)
	_, err = repo.GetCategoryBySlug(ctx, c.Slug)
	assertCode(t, err, apperr.CodeNotFound)
	err = repo.SoftDeleteCategory(ctx, c.ID)
	assertCode(t, err, apperr.CodeNotFound)
	// Updating a deleted category is NOT_FOUND (no rows affected).
	assertCode(t, repo.UpdateCategory(ctx, got), apperr.CodeNotFound)
}

func TestRepoArticleLifecycle(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := knowledgebase.NewRepo(d)
	suffix := tag()

	c := newCategoryFixture(t, repo, d, suffix)
	a := newArticleFixture(t, repo, d, c.ID, suffix, func(a *domain.KBArticle) { a.Published = true })
	require.NotZero(t, a.ID)

	got, err := repo.GetArticleByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, a.Slug, got.Slug)

	bySlug, err := repo.GetArticleBySlug(ctx, a.Slug)
	require.NoError(t, err)
	assert.Equal(t, a.ID, bySlug.ID)

	// Duplicate slug -> CONFLICT.
	dup := *a
	dup.ID = 0
	err = repo.CreateArticle(ctx, &dup)
	assertCode(t, err, apperr.CodeConflict)

	// FK violation: unknown category -> CONFLICT.
	err = repo.CreateArticle(ctx, &domain.KBArticle{
		CategoryID: 999_999_999, Title: "IT Orphan " + suffix, Slug: "it-orphan-" + suffix,
	})
	assertCode(t, err, apperr.CodeConflict)

	// CountArticlesInCategory.
	n, err := repo.CountArticlesInCategory(ctx, c.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	// Update.
	got.Title = "IT Article Updated " + suffix
	got.Published = false
	require.NoError(t, repo.UpdateArticle(ctx, got))
	again, err := repo.GetArticleByID(ctx, a.ID)
	require.NoError(t, err)
	assert.False(t, again.Published)

	// IncrementViews bumps the counter.
	require.NoError(t, repo.IncrementViews(ctx, a.ID))
	viewed, err := repo.GetArticleByID(ctx, a.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), viewed.Views)

	// Soft delete hides it everywhere.
	require.NoError(t, repo.SoftDeleteArticle(ctx, a.ID))
	_, err = repo.GetArticleByID(ctx, a.ID)
	assertCode(t, err, apperr.CodeNotFound)
	_, err = repo.GetArticleBySlug(ctx, a.Slug)
	assertCode(t, err, apperr.CodeNotFound)
	n, err = repo.CountArticlesInCategory(ctx, c.ID)
	require.NoError(t, err)
	assert.Zero(t, n)
	err = repo.SoftDeleteArticle(ctx, a.ID)
	assertCode(t, err, apperr.CodeNotFound)
	err = repo.IncrementViews(ctx, a.ID)
	assertCode(t, err, apperr.CodeNotFound)
	// Updating a deleted article is NOT_FOUND (no rows affected).
	assertCode(t, repo.UpdateArticle(ctx, again), apperr.CodeNotFound)
}

func TestRepoArticleListings(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := knowledgebase.NewRepo(d)
	suffix := tag()

	c1 := newCategoryFixture(t, repo, d, suffix+"-c1")
	c2 := newCategoryFixture(t, repo, d, suffix+"-c2")
	published := newArticleFixture(t, repo, d, c1.ID, suffix+"-pub", func(a *domain.KBArticle) { a.Published = true })
	draft := newArticleFixture(t, repo, d, c1.ID, suffix+"-draft", func(a *domain.KBArticle) { a.Published = false })
	other := newArticleFixture(t, repo, d, c2.ID, suffix+"-other", func(a *domain.KBArticle) { a.Published = true })

	// Admin List (all categories): search matches all three by shared suffix.
	rows, total, err := repo.List(ctx, ports.ListParams{Search: "it-article-" + suffix})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total)
	require.Len(t, rows, 3)

	// Status filter: draft only.
	rows, _, err = repo.List(ctx, ports.ListParams{Search: "it-article-" + suffix, Status: "draft"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, draft.ID, rows[0].ID)

	// Status filter: published only.
	rows, _, err = repo.List(ctx, ports.ListParams{Search: "it-article-" + suffix, Status: "published"})
	require.NoError(t, err)
	assert.Len(t, rows, 2)

	// ListArticlesAdmin scoped to a category includes drafts.
	rows, total, err = repo.ListArticlesAdmin(ctx, c1.ID, ports.ListParams{Search: "it-article-" + suffix})
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	ids := map[int64]bool{}
	for _, r := range rows {
		ids[r.ID] = true
	}
	assert.True(t, ids[published.ID])
	assert.True(t, ids[draft.ID])
	assert.False(t, ids[other.ID])

	// ListPublished across all categories excludes the draft.
	rows, _, err = repo.ListPublished(ctx, 0, ports.ListParams{Search: "it-article-" + suffix})
	require.NoError(t, err)
	ids = map[int64]bool{}
	for _, r := range rows {
		ids[r.ID] = true
	}
	assert.True(t, ids[published.ID])
	assert.False(t, ids[draft.ID], "public listing hides drafts")
	assert.True(t, ids[other.ID])

	// ListPublished scoped to a category.
	rows, total, err = repo.ListPublished(ctx, c1.ID, ports.ListParams{Search: "it-article-" + suffix})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	require.Len(t, rows, 1)
	assert.Equal(t, published.ID, rows[0].ID)

	// Sort override is honored (whitelist).
	rows, _, err = repo.List(ctx, ports.ListParams{Search: "it-article-" + suffix, Sort: "-created_at"})
	require.NoError(t, err)
	assert.Len(t, rows, 3)
}

// TestRepoCanceledContextPropagatesErrors drives the generic "if err != nil"
// wrapping branch of every repo method (and mapPgErr's final passthrough) by
// forcing pgx to fail fast on an already-canceled context, without ever
// reaching Postgres.
func TestRepoCanceledContextPropagatesErrors(t *testing.T) {
	d := testDB(t)
	repo := knowledgebase.NewRepo(d)

	cctx, cancel := context.WithCancel(context.Background())
	cancel()

	assert.Error(t, repo.CreateCategory(cctx, &domain.KBCategory{}))
	_, err := repo.GetCategoryByID(cctx, 1)
	assert.Error(t, err)
	_, err = repo.GetCategoryBySlug(cctx, "x")
	assert.Error(t, err)
	assert.Error(t, repo.UpdateCategory(cctx, &domain.KBCategory{ID: 1}))
	_, err = repo.ListCategories(cctx, true)
	assert.Error(t, err)
	assert.Error(t, repo.SoftDeleteCategory(cctx, 1))
	_, err = repo.CountArticlesInCategory(cctx, 1)
	assert.Error(t, err)

	assert.Error(t, repo.CreateArticle(cctx, &domain.KBArticle{}))
	_, err = repo.GetArticleByID(cctx, 1)
	assert.Error(t, err)
	_, err = repo.GetArticleBySlug(cctx, "x")
	assert.Error(t, err)
	assert.Error(t, repo.UpdateArticle(cctx, &domain.KBArticle{ID: 1}))
	_, _, err = repo.List(cctx, ports.ListParams{})
	assert.Error(t, err)
	_, _, err = repo.ListArticlesAdmin(cctx, 1, ports.ListParams{})
	assert.Error(t, err)
	_, _, err = repo.ListPublished(cctx, 1, ports.ListParams{})
	assert.Error(t, err)
	assert.Error(t, repo.SoftDeleteArticle(cctx, 1))
	assert.Error(t, repo.IncrementViews(cctx, 1))
}
