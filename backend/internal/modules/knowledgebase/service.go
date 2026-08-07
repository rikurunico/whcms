// Package knowledgebase implements the knowledgebase module: KB categories and
// articles (CRUD + admin management) plus the public, published-only reader
// views used by the portal help center.
//
// Route table (mounted on the /api/v1 group by the composition root):
//
//	GET    /kb/categories                  public, non-hidden categories (sort, name)
//	GET    /kb/categories/:slug            public, category + its published articles
//	GET    /kb/articles                    public, ?category_id&search - published, paginated
//	GET    /kb/articles/:slug              public, one published article (+views bump)
//	GET    /admin/kb/categories            ?include_hidden                    [perm: knowledgebase]
//	POST   /admin/kb/categories                                               [perm: knowledgebase]
//	GET    /admin/kb/categories/:id                                           [perm: knowledgebase]
//	PATCH  /admin/kb/categories/:id                                           [perm: knowledgebase]
//	DELETE /admin/kb/categories/:id        blocked while articles exist       [perm: knowledgebase]
//	GET    /admin/kb/articles              ?search&status&category_id         [perm: knowledgebase]
//	POST   /admin/kb/articles                                                 [perm: knowledgebase]
//	GET    /admin/kb/articles/:id                                             [perm: knowledgebase]
//	PATCH  /admin/kb/articles/:id                                             [perm: knowledgebase]
//	DELETE /admin/kb/articles/:id          soft delete                        [perm: knowledgebase]
package knowledgebase

import (
	"context"
	"errors"
	"strings"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/validate"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

// publicArticlesPerCategory caps how many published articles the public
// category-detail view returns.
const publicArticlesPerCategory = 100

// KnowledgebaseStore is ports.KnowledgebaseRepo plus the module-local queries
// the service needs (implemented by *Repo). Mirrors catalog's ProductStore:
// the extra admin-list-by-category read is not part of the shared port.
type KnowledgebaseStore interface {
	ports.KnowledgebaseRepo
	// ListArticlesAdmin lists articles for the admin console with an optional
	// category filter (categoryID 0 = all categories); drafts are included.
	ListArticlesAdmin(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error)
}

// Deps are the service dependencies (wired by the composition root).
type Deps struct {
	Repo  KnowledgebaseStore
	Tx    ports.TxManager
	Audit ports.AuditLogger
	Clock ports.Clock
}

// Service implements the knowledgebase use-cases.
type Service struct {
	d   Deps
	val *validate.Validator
}

// New builds the knowledgebase Service.
func New(d Deps) *Service {
	return &Service{d: d, val: validate.New()}
}

// Helpers

// wrap passes *apperr.Error through and wraps anything else as INTERNAL.
func wrap(err error) error {
	if err == nil {
		return nil
	}
	var e *apperr.Error
	if errors.As(err, &e) {
		return e
	}
	return apperr.Internal(err)
}

// slugify lowercases s and replaces every non-alphanumeric run with '-'.
func slugify(s string) string {
	var b strings.Builder
	prevDash := true // trims leading dashes
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

// Public reader views

// PublicCategories returns the non-hidden categories (ordered by sort, name).
func (s *Service) PublicCategories(ctx context.Context) ([]domain.KBCategory, error) {
	cats, err := s.d.Repo.ListCategories(ctx, false)
	if err != nil {
		return nil, wrap(err)
	}
	if cats == nil {
		cats = []domain.KBCategory{}
	}
	return cats, nil
}

// PublicCategoryBySlug returns one non-hidden category with its published
// articles. Hidden/deleted (or unknown) categories are NOT_FOUND.
func (s *Service) PublicCategoryBySlug(ctx context.Context, slug string) (*CategoryWithArticles, error) {
	cat, err := s.d.Repo.GetCategoryBySlug(ctx, slug)
	if err != nil {
		return nil, wrap(err)
	}
	if cat == nil || cat.Hidden || cat.DeletedAt != nil {
		return nil, apperr.NotFound("category")
	}
	articles, _, err := s.d.Repo.ListPublished(ctx, cat.ID, ports.ListParams{Page: 1, PerPage: publicArticlesPerCategory})
	if err != nil {
		return nil, wrap(err)
	}
	if articles == nil {
		articles = []domain.KBArticle{}
	}
	return &CategoryWithArticles{KBCategory: *cat, Articles: articles}, nil
}

// PublicArticles returns published articles, optionally scoped to a category
// (categoryID 0 = all), paginated.
func (s *Service) PublicArticles(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
	rows, total, err := s.d.Repo.ListPublished(ctx, categoryID, p)
	if err != nil {
		return nil, 0, wrap(err)
	}
	if rows == nil {
		rows = []domain.KBArticle{}
	}
	return rows, total, nil
}

// PublicArticleBySlug returns one published article by slug and bumps its view
// counter (best effort). Draft/deleted (or unknown) articles are NOT_FOUND.
func (s *Service) PublicArticleBySlug(ctx context.Context, slug string) (*domain.KBArticle, error) {
	a, err := s.d.Repo.GetArticleBySlug(ctx, slug)
	if err != nil {
		return nil, wrap(err)
	}
	if a == nil || !a.Published || a.DeletedAt != nil {
		return nil, apperr.NotFound("article")
	}
	_ = s.d.Repo.IncrementViews(ctx, a.ID) // best effort; a cache/DB hiccup must not fail the read
	return a, nil
}

// Categories (admin)

// ListCategories lists categories (admin; includeHidden toggles hidden rows).
func (s *Service) ListCategories(ctx context.Context, includeHidden bool) ([]domain.KBCategory, error) {
	cats, err := s.d.Repo.ListCategories(ctx, includeHidden)
	if err != nil {
		return nil, wrap(err)
	}
	if cats == nil {
		cats = []domain.KBCategory{}
	}
	return cats, nil
}

// GetCategory returns one category.
func (s *Service) GetCategory(ctx context.Context, id int64) (*domain.KBCategory, error) {
	c, err := s.d.Repo.GetCategoryByID(ctx, id)
	if err != nil {
		return nil, wrap(err)
	}
	if c == nil {
		return nil, apperr.NotFound("category")
	}
	return c, nil
}

// CreateCategory creates a category (slug derived from name when omitted).
func (s *Service) CreateCategory(ctx context.Context, actorUserID int64, in CategoryInput) (*domain.KBCategory, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	slug := in.Slug
	if slug == "" {
		slug = slugify(in.Name)
	}
	if slug == "" {
		return nil, apperr.Validation("invalid slug", apperr.FieldError{Field: "slug", Message: "cannot be derived from name"})
	}
	c := &domain.KBCategory{Name: in.Name, Slug: slug, Description: in.Description, Sort: in.Sort, Hidden: in.Hidden}
	if err := s.d.Repo.CreateCategory(ctx, c); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "knowledgebase.category.create", "kb_category", c.ID, nil, c)
	return c, nil
}

// UpdateCategory patches a category; nil fields are left unchanged.
func (s *Service) UpdateCategory(ctx context.Context, actorUserID, id int64, in CategoryUpdateInput) (*domain.KBCategory, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	c, err := s.GetCategory(ctx, id)
	if err != nil {
		return nil, err
	}
	before := *c
	if in.Name != nil {
		c.Name = *in.Name
	}
	if in.Slug != nil {
		c.Slug = *in.Slug
	}
	if in.Description != nil {
		c.Description = *in.Description
	}
	if in.Sort != nil {
		c.Sort = *in.Sort
	}
	if in.Hidden != nil {
		c.Hidden = *in.Hidden
	}
	if err := s.d.Repo.UpdateCategory(ctx, c); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "knowledgebase.category.update", "kb_category", c.ID, before, c)
	return c, nil
}

// DeleteCategory soft-deletes a category. Categories that still contain
// non-deleted articles cannot be deleted (CONFLICT).
func (s *Service) DeleteCategory(ctx context.Context, actorUserID, id int64) error {
	c, err := s.GetCategory(ctx, id)
	if err != nil {
		return err
	}
	err = s.d.Tx.WithinTx(ctx, func(txCtx context.Context) error {
		n, err := s.d.Repo.CountArticlesInCategory(txCtx, id)
		if err != nil {
			return err
		}
		if n > 0 {
			return apperr.Conflict("category still has articles")
		}
		return s.d.Repo.SoftDeleteCategory(txCtx, id)
	})
	if err != nil {
		return wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "knowledgebase.category.delete", "kb_category", id, c, nil)
	return nil
}

// Articles (admin)

// ListArticles lists articles (admin; search on title/slug, status
// published|draft, optional category filter - categoryID 0 = all).
func (s *Service) ListArticles(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
	rows, total, err := s.d.Repo.ListArticlesAdmin(ctx, categoryID, p)
	if err != nil {
		return nil, 0, wrap(err)
	}
	if rows == nil {
		rows = []domain.KBArticle{}
	}
	return rows, total, nil
}

// GetArticle returns one article.
func (s *Service) GetArticle(ctx context.Context, id int64) (*domain.KBArticle, error) {
	a, err := s.d.Repo.GetArticleByID(ctx, id)
	if err != nil {
		return nil, wrap(err)
	}
	if a == nil {
		return nil, apperr.NotFound("article")
	}
	return a, nil
}

// CreateArticle creates an article in an existing category (NOT_FOUND when the
// category is missing). Slug is derived from the title when omitted.
func (s *Service) CreateArticle(ctx context.Context, actorUserID int64, in ArticleInput) (*domain.KBArticle, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	if _, err := s.GetCategory(ctx, in.CategoryID); err != nil {
		return nil, err
	}
	slug := in.Slug
	if slug == "" {
		slug = slugify(in.Title)
	}
	if slug == "" {
		return nil, apperr.Validation("invalid slug", apperr.FieldError{Field: "slug", Message: "cannot be derived from title"})
	}
	a := &domain.KBArticle{
		CategoryID: in.CategoryID,
		Title:      in.Title,
		Slug:       slug,
		Body:       in.Body,
		Published:  in.Published,
		Sort:       in.Sort,
	}
	if err := s.d.Repo.CreateArticle(ctx, a); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "knowledgebase.article.create", "kb_article", a.ID, nil, a)
	return a, nil
}

// UpdateArticle patches an article; nil fields are left unchanged. Moving an
// article to another category requires that category to exist (NOT_FOUND).
func (s *Service) UpdateArticle(ctx context.Context, actorUserID, id int64, in ArticleUpdateInput) (*domain.KBArticle, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	a, err := s.GetArticle(ctx, id)
	if err != nil {
		return nil, err
	}
	before := *a
	if in.CategoryID != nil {
		if _, err := s.GetCategory(ctx, *in.CategoryID); err != nil {
			return nil, err
		}
		a.CategoryID = *in.CategoryID
	}
	if in.Title != nil {
		a.Title = *in.Title
	}
	if in.Slug != nil {
		a.Slug = *in.Slug
	}
	if in.Body != nil {
		a.Body = *in.Body
	}
	if in.Published != nil {
		a.Published = *in.Published
	}
	if in.Sort != nil {
		a.Sort = *in.Sort
	}
	if err := s.d.Repo.UpdateArticle(ctx, a); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "knowledgebase.article.update", "kb_article", a.ID, before, a)
	return a, nil
}

// DeleteArticle soft-deletes an article.
func (s *Service) DeleteArticle(ctx context.Context, actorUserID, id int64) error {
	a, err := s.GetArticle(ctx, id)
	if err != nil {
		return err
	}
	if err := s.d.Repo.SoftDeleteArticle(ctx, id); err != nil {
		return wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "knowledgebase.article.delete", "kb_article", id, a, nil)
	return nil
}
