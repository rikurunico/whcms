package knowledgebase_test

import (
	"context"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/knowledgebase"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fakes

// fakeKBStore is knowledgebase.KnowledgebaseStore: mocks.MockKnowledgebaseRepo
// plus the module-local admin-list-by-category query.
type fakeKBStore struct {
	*mocks.MockKnowledgebaseRepo
	ListArticlesAdminFn func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error)
}

func (f *fakeKBStore) ListArticlesAdmin(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
	if f.ListArticlesAdminFn != nil {
		return f.ListArticlesAdminFn(ctx, categoryID, p)
	}
	return nil, 0, nil
}

// fixtures bundles the fakes plus the service under test.
type fixtures struct {
	repo  *fakeKBStore
	audit *mocks.MockAuditLogger
	now   time.Time
	svc   *knowledgebase.Service
}

func newFixture() *fixtures {
	f := &fixtures{
		repo:  &fakeKBStore{MockKnowledgebaseRepo: &mocks.MockKnowledgebaseRepo{}},
		audit: &mocks.MockAuditLogger{},
		now:   time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC),
	}
	f.svc = knowledgebase.New(knowledgebase.Deps{
		Repo:  f.repo,
		Tx:    &mocks.MockTxManager{},
		Audit: f.audit,
		Clock: &mocks.MockClock{FixedTime: f.now},
	})
	return f
}

func assertCode(t *testing.T, err error, code apperr.Code) {
	t.Helper()
	var e *apperr.Error
	require.ErrorAs(t, err, &e)
	assert.Equal(t, code, e.Code)
}

func ptr[T any](v T) *T { return &v }

// Public reader views

func TestPublicCategories(t *testing.T) {
	f := newFixture()
	var gotInclude bool
	f.repo.ListCategoriesFn = func(ctx context.Context, includeHidden bool) ([]domain.KBCategory, error) {
		gotInclude = includeHidden
		return []domain.KBCategory{{ID: 1, Name: "General", Slug: "general"}}, nil
	}
	cats, err := f.svc.PublicCategories(context.Background())
	require.NoError(t, err)
	assert.False(t, gotInclude, "public view excludes hidden")
	require.Len(t, cats, 1)
	assert.Equal(t, "general", cats[0].Slug)
}

func TestPublicCategoriesEmptyIsSlice(t *testing.T) {
	f := newFixture()
	f.repo.ListCategoriesFn = func(ctx context.Context, includeHidden bool) ([]domain.KBCategory, error) {
		return nil, nil
	}
	cats, err := f.svc.PublicCategories(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, cats)
	assert.Empty(t, cats)
}

func TestPublicCategoryBySlug(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryBySlugFn = func(ctx context.Context, slug string) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: 3, Name: "Billing", Slug: slug}, nil
	}
	var gotCat int64
	f.repo.ListPublishedFn = func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
		gotCat = categoryID
		return []domain.KBArticle{{ID: 9, CategoryID: categoryID, Title: "How to pay", Slug: "how-to-pay", Published: true}}, 1, nil
	}
	got, err := f.svc.PublicCategoryBySlug(context.Background(), "billing")
	require.NoError(t, err)
	assert.Equal(t, int64(3), gotCat)
	assert.Equal(t, "billing", got.Slug)
	require.Len(t, got.Articles, 1)
	assert.Equal(t, "how-to-pay", got.Articles[0].Slug)
}

func TestPublicCategoryBySlugEmptyArticles(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryBySlugFn = func(ctx context.Context, slug string) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: 3, Slug: slug}, nil
	}
	f.repo.ListPublishedFn = func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
		return nil, 0, nil
	}
	got, err := f.svc.PublicCategoryBySlug(context.Background(), "empty")
	require.NoError(t, err)
	assert.NotNil(t, got.Articles)
	assert.Empty(t, got.Articles)
}

func TestPublicCategoryBySlugHiddenIsNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryBySlugFn = func(ctx context.Context, slug string) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: 3, Slug: slug, Hidden: true}, nil
	}
	_, err := f.svc.PublicCategoryBySlug(context.Background(), "hidden")
	assertCode(t, err, apperr.CodeNotFound)
}

func TestPublicCategoryBySlugDeletedIsNotFound(t *testing.T) {
	f := newFixture()
	del := time.Now()
	f.repo.GetCategoryBySlugFn = func(ctx context.Context, slug string) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: 3, Slug: slug, DeletedAt: &del}, nil
	}
	_, err := f.svc.PublicCategoryBySlug(context.Background(), "gone")
	assertCode(t, err, apperr.CodeNotFound)
}

func TestPublicCategoryBySlugNilIsNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryBySlugFn = func(ctx context.Context, slug string) (*domain.KBCategory, error) {
		return nil, nil
	}
	_, err := f.svc.PublicCategoryBySlug(context.Background(), "nope")
	assertCode(t, err, apperr.CodeNotFound)
}

func TestPublicArticles(t *testing.T) {
	f := newFixture()
	var gotCat int64
	f.repo.ListPublishedFn = func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
		gotCat = categoryID
		return []domain.KBArticle{{ID: 1, Published: true}}, 1, nil
	}
	rows, total, err := f.svc.PublicArticles(context.Background(), 5, ports.ListParams{Search: "x"})
	require.NoError(t, err)
	assert.Equal(t, int64(5), gotCat)
	assert.Len(t, rows, 1)
	assert.Equal(t, int64(1), total)
}

func TestPublicArticlesEmptyIsSlice(t *testing.T) {
	f := newFixture()
	f.repo.ListPublishedFn = func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
		return nil, 0, nil
	}
	rows, _, err := f.svc.PublicArticles(context.Background(), 0, ports.ListParams{})
	require.NoError(t, err)
	assert.NotNil(t, rows)
	assert.Empty(t, rows)
}

func TestPublicArticleBySlugBumpsViews(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleBySlugFn = func(ctx context.Context, slug string) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: 7, Slug: slug, Published: true}, nil
	}
	var bumped int64
	f.repo.IncrementViewsFn = func(ctx context.Context, id int64) error {
		bumped = id
		return nil
	}
	a, err := f.svc.PublicArticleBySlug(context.Background(), "hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", a.Slug)
	assert.Equal(t, int64(7), bumped)
}

func TestPublicArticleBySlugSwallowsViewError(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleBySlugFn = func(ctx context.Context, slug string) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: 7, Slug: slug, Published: true}, nil
	}
	f.repo.IncrementViewsFn = func(ctx context.Context, id int64) error { return errBoom }
	a, err := f.svc.PublicArticleBySlug(context.Background(), "hello")
	require.NoError(t, err, "view-counter failure must not fail the read")
	assert.Equal(t, int64(7), a.ID)
}

func TestPublicArticleBySlugDraftIsNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleBySlugFn = func(ctx context.Context, slug string) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: 7, Slug: slug, Published: false}, nil
	}
	_, err := f.svc.PublicArticleBySlug(context.Background(), "draft")
	assertCode(t, err, apperr.CodeNotFound)
}

func TestPublicArticleBySlugDeletedIsNotFound(t *testing.T) {
	f := newFixture()
	del := time.Now()
	f.repo.GetArticleBySlugFn = func(ctx context.Context, slug string) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: 7, Slug: slug, Published: true, DeletedAt: &del}, nil
	}
	_, err := f.svc.PublicArticleBySlug(context.Background(), "gone")
	assertCode(t, err, apperr.CodeNotFound)
}

func TestPublicArticleBySlugNilIsNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleBySlugFn = func(ctx context.Context, slug string) (*domain.KBArticle, error) {
		return nil, nil
	}
	_, err := f.svc.PublicArticleBySlug(context.Background(), "nope")
	assertCode(t, err, apperr.CodeNotFound)
}

// Categories (admin)

func TestListCategoriesAdminIncludesHidden(t *testing.T) {
	f := newFixture()
	var gotInclude bool
	f.repo.ListCategoriesFn = func(ctx context.Context, includeHidden bool) ([]domain.KBCategory, error) {
		gotInclude = includeHidden
		return nil, nil
	}
	cats, err := f.svc.ListCategories(context.Background(), true)
	require.NoError(t, err)
	assert.True(t, gotInclude)
	assert.NotNil(t, cats)
}

func TestGetCategoryNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return nil, apperr.NotFound("category")
	}
	_, err := f.svc.GetCategory(context.Background(), 99)
	assertCode(t, err, apperr.CodeNotFound)
}

func TestCreateCategoryGeneratesSlugAndAudits(t *testing.T) {
	f := newFixture()
	var created *domain.KBCategory
	f.repo.CreateCategoryFn = func(ctx context.Context, c *domain.KBCategory) error {
		c.ID = 5
		created = c
		return nil
	}
	c, err := f.svc.CreateCategory(context.Background(), 42, knowledgebase.CategoryInput{Name: "Getting Started!"})
	require.NoError(t, err)
	assert.Equal(t, "getting-started", created.Slug)
	assert.Equal(t, int64(5), c.ID)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "knowledgebase.category.create", f.audit.Entries[0].Action)
	assert.Equal(t, "kb_category", f.audit.Entries[0].Entity)
	assert.Equal(t, int64(42), f.audit.Entries[0].ActorUserID)
}

func TestCreateCategoryHonorsExplicitSlug(t *testing.T) {
	f := newFixture()
	var created *domain.KBCategory
	f.repo.CreateCategoryFn = func(ctx context.Context, c *domain.KBCategory) error {
		created = c
		return nil
	}
	_, err := f.svc.CreateCategory(context.Background(), 1, knowledgebase.CategoryInput{Name: "Billing", Slug: "money-stuff"})
	require.NoError(t, err)
	assert.Equal(t, "money-stuff", created.Slug)
}

func TestCreateCategoryValidation(t *testing.T) {
	f := newFixture()
	_, err := f.svc.CreateCategory(context.Background(), 1, knowledgebase.CategoryInput{Name: "x"})
	assertCode(t, err, apperr.CodeValidation)
}

func TestCreateCategorySlugUnderivable(t *testing.T) {
	f := newFixture()
	_, err := f.svc.CreateCategory(context.Background(), 1, knowledgebase.CategoryInput{Name: "!!"})
	assertCode(t, err, apperr.CodeValidation)
}

func TestUpdateCategoryPatchesFields(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id, Name: "Old", Slug: "old", Sort: 1}, nil
	}
	var saved *domain.KBCategory
	f.repo.UpdateCategoryFn = func(ctx context.Context, c *domain.KBCategory) error {
		saved = c
		return nil
	}
	c, err := f.svc.UpdateCategory(context.Background(), 1, 3, knowledgebase.CategoryUpdateInput{
		Name: ptr("New"), Description: ptr("desc"), Sort: ptr(9), Hidden: ptr(true),
	})
	require.NoError(t, err)
	assert.Equal(t, "New", saved.Name)
	assert.Equal(t, "desc", saved.Description)
	assert.Equal(t, 9, saved.Sort)
	assert.Equal(t, "old", saved.Slug, "unset fields unchanged")
	assert.True(t, c.Hidden)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "knowledgebase.category.update", f.audit.Entries[0].Action)
}

func TestUpdateCategorySlugPatch(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id, Name: "Old", Slug: "old"}, nil
	}
	var saved *domain.KBCategory
	f.repo.UpdateCategoryFn = func(ctx context.Context, c *domain.KBCategory) error {
		saved = c
		return nil
	}
	_, err := f.svc.UpdateCategory(context.Background(), 1, 3, knowledgebase.CategoryUpdateInput{Slug: ptr("brand-new")})
	require.NoError(t, err)
	assert.Equal(t, "brand-new", saved.Slug)
}

func TestUpdateCategoryValidation(t *testing.T) {
	f := newFixture()
	_, err := f.svc.UpdateCategory(context.Background(), 1, 3, knowledgebase.CategoryUpdateInput{Name: ptr("x")})
	assertCode(t, err, apperr.CodeValidation)
}

func TestDeleteCategoryBlockedWhileArticlesExist(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id}, nil
	}
	f.repo.CountArticlesInCategoryFn = func(ctx context.Context, categoryID int64) (int64, error) {
		return 3, nil
	}
	deleted := false
	f.repo.SoftDeleteCategoryFn = func(ctx context.Context, id int64) error {
		deleted = true
		return nil
	}
	err := f.svc.DeleteCategory(context.Background(), 1, 9)
	assertCode(t, err, apperr.CodeConflict)
	assert.False(t, deleted)
	assert.Empty(t, f.audit.Entries)
}

func TestDeleteCategorySoftDeletes(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id}, nil
	}
	var deletedID int64
	f.repo.SoftDeleteCategoryFn = func(ctx context.Context, id int64) error {
		deletedID = id
		return nil
	}
	require.NoError(t, f.svc.DeleteCategory(context.Background(), 1, 9))
	assert.Equal(t, int64(9), deletedID)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "knowledgebase.category.delete", f.audit.Entries[0].Action)
}

// Articles (admin)

func articleInput(mut func(*knowledgebase.ArticleInput)) knowledgebase.ArticleInput {
	in := knowledgebase.ArticleInput{
		CategoryID: 1,
		Title:      "How to reset your password",
	}
	if mut != nil {
		mut(&in)
	}
	return in
}

func TestListArticlesPassthrough(t *testing.T) {
	f := newFixture()
	var gotCat int64
	var gotParams ports.ListParams
	f.repo.ListArticlesAdminFn = func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
		gotCat = categoryID
		gotParams = p
		return []domain.KBArticle{{ID: 1}}, 1, nil
	}
	rows, total, err := f.svc.ListArticles(context.Background(), 4, ports.ListParams{Search: "pass", Status: "draft"})
	require.NoError(t, err)
	assert.Equal(t, int64(4), gotCat)
	assert.Equal(t, "draft", gotParams.Status)
	assert.Len(t, rows, 1)
	assert.Equal(t, int64(1), total)
}

func TestListArticlesEmptyIsSlice(t *testing.T) {
	f := newFixture()
	f.repo.ListArticlesAdminFn = func(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
		return nil, 0, nil
	}
	rows, _, err := f.svc.ListArticles(context.Background(), 0, ports.ListParams{})
	require.NoError(t, err)
	assert.NotNil(t, rows)
	assert.Empty(t, rows)
}

func TestGetArticleNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
		return nil, apperr.NotFound("article")
	}
	_, err := f.svc.GetArticle(context.Background(), 99)
	assertCode(t, err, apperr.CodeNotFound)
}

func TestCreateArticleGeneratesSlugAndAudits(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id}, nil
	}
	var created *domain.KBArticle
	f.repo.CreateArticleFn = func(ctx context.Context, a *domain.KBArticle) error {
		a.ID = 11
		created = a
		return nil
	}
	a, err := f.svc.CreateArticle(context.Background(), 7, articleInput(func(in *knowledgebase.ArticleInput) {
		in.Body = "Click the reset link."
		in.Published = true
	}))
	require.NoError(t, err)
	assert.Equal(t, "how-to-reset-your-password", created.Slug)
	assert.True(t, created.Published)
	assert.Equal(t, int64(11), a.ID)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "knowledgebase.article.create", f.audit.Entries[0].Action)
	assert.Equal(t, "kb_article", f.audit.Entries[0].Entity)
}

func TestCreateArticleSanitizesBody(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id}, nil
	}
	var created *domain.KBArticle
	f.repo.CreateArticleFn = func(ctx context.Context, a *domain.KBArticle) error {
		created = a
		return nil
	}
	_, err := f.svc.CreateArticle(context.Background(), 7, articleInput(func(in *knowledgebase.ArticleInput) {
		in.Body = `<p>ok</p><script>alert(document.cookie)</script>`
	}))
	require.NoError(t, err)
	assert.Equal(t, "<p>ok</p>", created.Body)
}

func TestCreateArticleUnknownCategoryIsNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return nil, apperr.NotFound("category")
	}
	_, err := f.svc.CreateArticle(context.Background(), 1, articleInput(nil))
	assertCode(t, err, apperr.CodeNotFound)
}

func TestCreateArticleNilCategoryIsNotFound(t *testing.T) {
	f := newFixture()
	// Mock default GetCategoryByID returns (nil, nil) -> service maps to NotFound.
	_, err := f.svc.CreateArticle(context.Background(), 1, articleInput(nil))
	assertCode(t, err, apperr.CodeNotFound)
}

func TestCreateArticleValidation(t *testing.T) {
	f := newFixture()
	_, err := f.svc.CreateArticle(context.Background(), 1, knowledgebase.ArticleInput{CategoryID: 1, Title: "x"})
	assertCode(t, err, apperr.CodeValidation)

	_, err = f.svc.CreateArticle(context.Background(), 1, knowledgebase.ArticleInput{Title: "Valid Title"})
	assertCode(t, err, apperr.CodeValidation)
}

func TestCreateArticleSlugUnderivable(t *testing.T) {
	f := newFixture()
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id}, nil
	}
	_, err := f.svc.CreateArticle(context.Background(), 1, articleInput(func(in *knowledgebase.ArticleInput) {
		in.Title = "!!"
	}))
	assertCode(t, err, apperr.CodeValidation)
}

func TestUpdateArticlePatchesFields(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: id, CategoryID: 1, Title: "Old", Slug: "old", Sort: 1}, nil
	}
	var saved *domain.KBArticle
	f.repo.UpdateArticleFn = func(ctx context.Context, a *domain.KBArticle) error {
		saved = a
		return nil
	}
	a, err := f.svc.UpdateArticle(context.Background(), 1, 10, knowledgebase.ArticleUpdateInput{
		Title:     ptr("New Title"),
		Slug:      ptr("new-title"),
		Body:      ptr("new body"),
		Published: ptr(true),
		Sort:      ptr(4),
	})
	require.NoError(t, err)
	assert.Equal(t, "New Title", saved.Title)
	assert.Equal(t, "new-title", saved.Slug)
	assert.Equal(t, "new body", saved.Body)
	assert.True(t, a.Published)
	assert.Equal(t, 4, saved.Sort)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "knowledgebase.article.update", f.audit.Entries[0].Action)
}

func TestUpdateArticleSanitizesBody(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: id, CategoryID: 1, Title: "Old", Slug: "old", Sort: 1}, nil
	}
	var saved *domain.KBArticle
	f.repo.UpdateArticleFn = func(ctx context.Context, a *domain.KBArticle) error {
		saved = a
		return nil
	}
	_, err := f.svc.UpdateArticle(context.Background(), 1, 10, knowledgebase.ArticleUpdateInput{
		Body: ptr(`<img src=x onerror=alert(1)>`),
	})
	require.NoError(t, err)
	assert.Equal(t, `<img src="x">`, saved.Body)
}

func TestUpdateArticleMovesCategory(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: id, CategoryID: 1, Title: "T", Slug: "t"}, nil
	}
	var checkedCat int64
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		checkedCat = id
		return &domain.KBCategory{ID: id}, nil
	}
	var saved *domain.KBArticle
	f.repo.UpdateArticleFn = func(ctx context.Context, a *domain.KBArticle) error {
		saved = a
		return nil
	}
	_, err := f.svc.UpdateArticle(context.Background(), 1, 10, knowledgebase.ArticleUpdateInput{CategoryID: ptr(int64(2))})
	require.NoError(t, err)
	assert.Equal(t, int64(2), checkedCat)
	assert.Equal(t, int64(2), saved.CategoryID)
}

func TestUpdateArticleUnknownCategoryIsNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: id, CategoryID: 1, Title: "T", Slug: "t"}, nil
	}
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return nil, apperr.NotFound("category")
	}
	_, err := f.svc.UpdateArticle(context.Background(), 1, 10, knowledgebase.ArticleUpdateInput{CategoryID: ptr(int64(2))})
	assertCode(t, err, apperr.CodeNotFound)
}

func TestUpdateArticleValidation(t *testing.T) {
	f := newFixture()
	_, err := f.svc.UpdateArticle(context.Background(), 1, 10, knowledgebase.ArticleUpdateInput{Title: ptr("x")})
	assertCode(t, err, apperr.CodeValidation)
}

func TestUpdateArticleNotFound(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
		return nil, apperr.NotFound("article")
	}
	_, err := f.svc.UpdateArticle(context.Background(), 1, 10, knowledgebase.ArticleUpdateInput{Title: ptr("New Title")})
	assertCode(t, err, apperr.CodeNotFound)
}

func TestDeleteArticleSoftDeletes(t *testing.T) {
	f := newFixture()
	f.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
		return &domain.KBArticle{ID: id}, nil
	}
	var deletedID int64
	f.repo.SoftDeleteArticleFn = func(ctx context.Context, id int64) error {
		deletedID = id
		return nil
	}
	require.NoError(t, f.svc.DeleteArticle(context.Background(), 1, 10))
	assert.Equal(t, int64(10), deletedID)
	require.Len(t, f.audit.Entries, 1)
	assert.Equal(t, "knowledgebase.article.delete", f.audit.Entries[0].Action)
}

var errBoom = errors.New("boom")

// TestServiceErrorPropagation drives every repo failure branch: unexpected repo
// errors surface as INTERNAL apperr.
func TestServiceErrorPropagation(t *testing.T) {
	ctx := context.Background()

	category := func(f *fixtures) {
		f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
			return &domain.KBCategory{ID: id}, nil
		}
	}
	article := func(f *fixtures) {
		f.repo.GetArticleByIDFn = func(ctx context.Context, id int64) (*domain.KBArticle, error) {
			return &domain.KBArticle{ID: id, CategoryID: 1, Title: "T", Slug: "t"}, nil
		}
	}

	tests := []struct {
		name string
		prep func(f *fixtures)
		call func(f *fixtures) error
	}{
		{"PublicCategories", func(f *fixtures) {
			f.repo.ListCategoriesFn = func(context.Context, bool) ([]domain.KBCategory, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicCategories(ctx); return err }},
		{"PublicCategoryBySlug get", func(f *fixtures) {
			f.repo.GetCategoryBySlugFn = func(context.Context, string) (*domain.KBCategory, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicCategoryBySlug(ctx, "s"); return err }},
		{"PublicCategoryBySlug articles", func(f *fixtures) {
			f.repo.GetCategoryBySlugFn = func(ctx context.Context, s string) (*domain.KBCategory, error) {
				return &domain.KBCategory{ID: 1, Slug: s}, nil
			}
			f.repo.ListPublishedFn = func(context.Context, int64, ports.ListParams) ([]domain.KBArticle, int64, error) {
				return nil, 0, errBoom
			}
		}, func(f *fixtures) error { _, err := f.svc.PublicCategoryBySlug(ctx, "s"); return err }},
		{"PublicArticles", func(f *fixtures) {
			f.repo.ListPublishedFn = func(context.Context, int64, ports.ListParams) ([]domain.KBArticle, int64, error) {
				return nil, 0, errBoom
			}
		}, func(f *fixtures) error { _, _, err := f.svc.PublicArticles(ctx, 0, ports.ListParams{}); return err }},
		{"PublicArticleBySlug", func(f *fixtures) {
			f.repo.GetArticleBySlugFn = func(context.Context, string) (*domain.KBArticle, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicArticleBySlug(ctx, "s"); return err }},
		{"ListCategories", func(f *fixtures) {
			f.repo.ListCategoriesFn = func(context.Context, bool) ([]domain.KBCategory, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.ListCategories(ctx, true); return err }},
		{"GetCategory", func(f *fixtures) {
			f.repo.GetCategoryByIDFn = func(context.Context, int64) (*domain.KBCategory, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.GetCategory(ctx, 1); return err }},
		{"CreateCategory", func(f *fixtures) {
			f.repo.CreateCategoryFn = func(context.Context, *domain.KBCategory) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.CreateCategory(ctx, 1, knowledgebase.CategoryInput{Name: "General"})
			return err
		}},
		{"UpdateCategory get", func(f *fixtures) {
			f.repo.GetCategoryByIDFn = func(context.Context, int64) (*domain.KBCategory, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateCategory(ctx, 1, 2, knowledgebase.CategoryUpdateInput{})
			return err
		}},
		{"UpdateCategory save", func(f *fixtures) {
			category(f)
			f.repo.UpdateCategoryFn = func(context.Context, *domain.KBCategory) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateCategory(ctx, 1, 2, knowledgebase.CategoryUpdateInput{Sort: ptr(2)})
			return err
		}},
		{"DeleteCategory count", func(f *fixtures) {
			category(f)
			f.repo.CountArticlesInCategoryFn = func(context.Context, int64) (int64, error) { return 0, errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteCategory(ctx, 1, 2) }},
		{"DeleteCategory delete", func(f *fixtures) {
			category(f)
			f.repo.SoftDeleteCategoryFn = func(context.Context, int64) error { return errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteCategory(ctx, 1, 2) }},
		{"DeleteCategory get", func(f *fixtures) {
			f.repo.GetCategoryByIDFn = func(context.Context, int64) (*domain.KBCategory, error) { return nil, errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteCategory(ctx, 1, 2) }},
		{"ListArticles", func(f *fixtures) {
			f.repo.ListArticlesAdminFn = func(context.Context, int64, ports.ListParams) ([]domain.KBArticle, int64, error) {
				return nil, 0, errBoom
			}
		}, func(f *fixtures) error { _, _, err := f.svc.ListArticles(ctx, 0, ports.ListParams{}); return err }},
		{"GetArticle", func(f *fixtures) {
			f.repo.GetArticleByIDFn = func(context.Context, int64) (*domain.KBArticle, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.GetArticle(ctx, 1); return err }},
		{"CreateArticle category err", func(f *fixtures) {
			f.repo.GetCategoryByIDFn = func(context.Context, int64) (*domain.KBCategory, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.CreateArticle(ctx, 1, articleInput(nil)); return err }},
		{"CreateArticle save", func(f *fixtures) {
			category(f)
			f.repo.CreateArticleFn = func(context.Context, *domain.KBArticle) error { return errBoom }
		}, func(f *fixtures) error { _, err := f.svc.CreateArticle(ctx, 1, articleInput(nil)); return err }},
		{"UpdateArticle get", func(f *fixtures) {
			f.repo.GetArticleByIDFn = func(context.Context, int64) (*domain.KBArticle, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateArticle(ctx, 1, 2, knowledgebase.ArticleUpdateInput{})
			return err
		}},
		{"UpdateArticle category err", func(f *fixtures) {
			article(f)
			f.repo.GetCategoryByIDFn = func(context.Context, int64) (*domain.KBCategory, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateArticle(ctx, 1, 2, knowledgebase.ArticleUpdateInput{CategoryID: ptr(int64(3))})
			return err
		}},
		{"UpdateArticle save", func(f *fixtures) {
			article(f)
			f.repo.UpdateArticleFn = func(context.Context, *domain.KBArticle) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateArticle(ctx, 1, 2, knowledgebase.ArticleUpdateInput{Sort: ptr(3)})
			return err
		}},
		{"DeleteArticle get", func(f *fixtures) {
			f.repo.GetArticleByIDFn = func(context.Context, int64) (*domain.KBArticle, error) { return nil, errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteArticle(ctx, 1, 2) }},
		{"DeleteArticle delete", func(f *fixtures) {
			article(f)
			f.repo.SoftDeleteArticleFn = func(context.Context, int64) error { return errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteArticle(ctx, 1, 2) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			tt.prep(f)
			assertCode(t, tt.call(f), apperr.CodeInternal)
		})
	}
}

// A (nil, nil) row from the repo (the mocks' default) must surface as
// NOT_FOUND, never as a nil dereference.
func TestServiceNilRowsAreNotFound(t *testing.T) {
	ctx := context.Background()
	f := newFixture()

	_, err := f.svc.GetCategory(ctx, 1)
	assertCode(t, err, apperr.CodeNotFound)
	_, err = f.svc.GetArticle(ctx, 1)
	assertCode(t, err, apperr.CodeNotFound)
	_, err = f.svc.PublicCategoryBySlug(ctx, "s")
	assertCode(t, err, apperr.CodeNotFound)
	_, err = f.svc.PublicArticleBySlug(ctx, "s")
	assertCode(t, err, apperr.CodeNotFound)

	// UpdateCategory / UpdateArticle / DeleteCategory / DeleteArticle bubble the
	// same NOT_FOUND up from their Get* precondition.
	_, err = f.svc.UpdateCategory(ctx, 1, 2, knowledgebase.CategoryUpdateInput{Name: ptr("New Name")})
	assertCode(t, err, apperr.CodeNotFound)
	_, err = f.svc.UpdateArticle(ctx, 1, 2, knowledgebase.ArticleUpdateInput{Title: ptr("New Title")})
	assertCode(t, err, apperr.CodeNotFound)
	assertCode(t, f.svc.DeleteCategory(ctx, 1, 2), apperr.CodeNotFound)
	assertCode(t, f.svc.DeleteArticle(ctx, 1, 2), apperr.CodeNotFound)

	// CreateArticle's category precondition (nil, nil default) -> NOT_FOUND.
	_, err = f.svc.CreateArticle(ctx, 1, articleInput(nil))
	assertCode(t, err, apperr.CodeNotFound)
}

// Every mutation rejects invalid input with VALIDATION before touching
// the repo.
func TestServiceInputValidation(t *testing.T) {
	ctx := context.Background()
	f := newFixture()

	_, err := f.svc.CreateCategory(ctx, 1, knowledgebase.CategoryInput{Name: "x"})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.UpdateCategory(ctx, 1, 2, knowledgebase.CategoryUpdateInput{Name: ptr("x")})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.CreateArticle(ctx, 1, knowledgebase.ArticleInput{CategoryID: 1, Title: "x"})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.UpdateArticle(ctx, 1, 2, knowledgebase.ArticleUpdateInput{Title: ptr("x")})
	assertCode(t, err, apperr.CodeValidation)

	// Slug cannot be derived from a symbol-only name/title.
	_, err = f.svc.CreateCategory(ctx, 1, knowledgebase.CategoryInput{Name: "!!"})
	assertCode(t, err, apperr.CodeValidation)
	f.repo.GetCategoryByIDFn = func(ctx context.Context, id int64) (*domain.KBCategory, error) {
		return &domain.KBCategory{ID: id}, nil
	}
	_, err = f.svc.CreateArticle(ctx, 1, articleInput(func(in *knowledgebase.ArticleInput) { in.Title = "!!" }))
	assertCode(t, err, apperr.CodeValidation)
}
