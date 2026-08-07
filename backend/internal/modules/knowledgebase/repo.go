package knowledgebase

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Repo is the pgx persistence for the knowledgebase module. It implements
// ports.KnowledgebaseRepo plus the module-local KnowledgebaseStore interface.
// Every query goes through db.Querier(ctx) so it joins any transaction started
// by TxManager.WithinTx.
type Repo struct {
	db *db.DB
}

// NewRepo builds the knowledgebase Repo.
func NewRepo(d *db.DB) *Repo { return &Repo{db: d} }

// Compile-time interface checks.
var (
	_ ports.KnowledgebaseRepo = (*Repo)(nil)
	_ KnowledgebaseStore      = (*Repo)(nil)
)

// mapPgErr converts common Postgres constraint violations into apperr codes.
func mapPgErr(entity string, err error) error {
	var pge *pgconn.PgError
	if errors.As(err, &pge) {
		switch pge.Code {
		case "23505": // unique_violation
			return apperr.Conflict(entity + " already exists")
		case "23503": // foreign_key_violation
			return apperr.Conflict(entity + " is referenced by or references other records")
		case "23514": // check_violation
			return apperr.Validation("invalid " + entity)
		}
	}
	return err
}

// listSortColumns whitelists ORDER BY columns for the article listings.
var listSortColumns = map[string]string{
	"id": "id", "title": "title", "slug": "slug", "sort": "sort",
	"views": "views", "created_at": "created_at",
}

func orderBy(sort, def string) string {
	col, dir := sort, "ASC"
	if strings.HasPrefix(col, "-") {
		col, dir = col[1:], "DESC"
	}
	if c, ok := listSortColumns[col]; ok {
		return c + " " + dir
	}
	return def
}

// Categories

const categoryCols = `id, name, slug, description, sort, hidden, deleted_at,
	created_at, updated_at`

func scanCategory(row pgx.Row) (*domain.KBCategory, error) {
	var c domain.KBCategory
	err := row.Scan(&c.ID, &c.Name, &c.Slug, &c.Description, &c.Sort, &c.Hidden,
		&c.DeletedAt, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// CreateCategory inserts a category.
func (r *Repo) CreateCategory(ctx context.Context, c *domain.KBCategory) error {
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO kb_categories (name, slug, description, sort, hidden)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at, updated_at`,
		c.Name, c.Slug, c.Description, c.Sort, c.Hidden,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return mapPgErr("category", fmt.Errorf("knowledgebase: create category: %w", err))
	}
	return nil
}

// GetCategoryByID returns one non-deleted category.
func (r *Repo) GetCategoryByID(ctx context.Context, id int64) (*domain.KBCategory, error) {
	c, err := scanCategory(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+categoryCols+` FROM kb_categories WHERE id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("category")
	}
	if err != nil {
		return nil, fmt.Errorf("knowledgebase: get category %d: %w", id, err)
	}
	return c, nil
}

// GetCategoryBySlug returns one non-deleted category by slug.
func (r *Repo) GetCategoryBySlug(ctx context.Context, slug string) (*domain.KBCategory, error) {
	c, err := scanCategory(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+categoryCols+` FROM kb_categories WHERE slug = $1 AND deleted_at IS NULL`, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("category")
	}
	if err != nil {
		return nil, fmt.Errorf("knowledgebase: get category %q: %w", slug, err)
	}
	return c, nil
}

// UpdateCategory saves all mutable category columns.
func (r *Repo) UpdateCategory(ctx context.Context, c *domain.KBCategory) error {
	tag, err := r.db.Querier(ctx).Exec(ctx, `
		UPDATE kb_categories SET name=$2, slug=$3, description=$4, sort=$5, hidden=$6
		WHERE id = $1 AND deleted_at IS NULL`,
		c.ID, c.Name, c.Slug, c.Description, c.Sort, c.Hidden)
	if err != nil {
		return mapPgErr("category", fmt.Errorf("knowledgebase: update category %d: %w", c.ID, err))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("category")
	}
	return nil
}

// ListCategories returns non-deleted categories ordered by sort, name.
func (r *Repo) ListCategories(ctx context.Context, includeHidden bool) ([]domain.KBCategory, error) {
	rows, err := r.db.Querier(ctx).Query(ctx,
		`SELECT `+categoryCols+` FROM kb_categories
		 WHERE deleted_at IS NULL AND ($1 OR NOT hidden) ORDER BY sort, name`,
		includeHidden)
	if err != nil {
		return nil, fmt.Errorf("knowledgebase: list categories: %w", err)
	}
	defer rows.Close()

	var out []domain.KBCategory
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("knowledgebase: scan category: %w", err)
		}
		out = append(out, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("knowledgebase: categories rows: %w", err)
	}
	return out, nil
}

// SoftDeleteCategory marks a category deleted.
func (r *Repo) SoftDeleteCategory(ctx context.Context, id int64) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE kb_categories SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("knowledgebase: soft delete category %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("category")
	}
	return nil
}

// CountArticlesInCategory counts non-deleted articles in a category.
func (r *Repo) CountArticlesInCategory(ctx context.Context, categoryID int64) (int64, error) {
	var n int64
	err := r.db.Querier(ctx).QueryRow(ctx,
		`SELECT count(*) FROM kb_articles WHERE category_id = $1 AND deleted_at IS NULL`,
		categoryID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("knowledgebase: count articles in category %d: %w", categoryID, err)
	}
	return n, nil
}

// Articles

const articleCols = `id, category_id, title, slug, body, published, views, sort,
	author_id, deleted_at, created_at, updated_at`

func scanArticle(row pgx.Row) (*domain.KBArticle, error) {
	var a domain.KBArticle
	err := row.Scan(&a.ID, &a.CategoryID, &a.Title, &a.Slug, &a.Body, &a.Published,
		&a.Views, &a.Sort, &a.AuthorID, &a.DeletedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// CreateArticle inserts an article.
func (r *Repo) CreateArticle(ctx context.Context, a *domain.KBArticle) error {
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO kb_articles (category_id, title, slug, body, published, sort, author_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, views, created_at, updated_at`,
		a.CategoryID, a.Title, a.Slug, a.Body, a.Published, a.Sort, a.AuthorID,
	).Scan(&a.ID, &a.Views, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return mapPgErr("article", fmt.Errorf("knowledgebase: create article: %w", err))
	}
	return nil
}

// GetArticleByID returns one non-deleted article.
func (r *Repo) GetArticleByID(ctx context.Context, id int64) (*domain.KBArticle, error) {
	a, err := scanArticle(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+articleCols+` FROM kb_articles WHERE id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("article")
	}
	if err != nil {
		return nil, fmt.Errorf("knowledgebase: get article %d: %w", id, err)
	}
	return a, nil
}

// GetArticleBySlug returns one non-deleted article by slug.
func (r *Repo) GetArticleBySlug(ctx context.Context, slug string) (*domain.KBArticle, error) {
	a, err := scanArticle(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+articleCols+` FROM kb_articles WHERE slug = $1 AND deleted_at IS NULL`, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("article")
	}
	if err != nil {
		return nil, fmt.Errorf("knowledgebase: get article %q: %w", slug, err)
	}
	return a, nil
}

// UpdateArticle saves all mutable article columns (views/author untouched).
func (r *Repo) UpdateArticle(ctx context.Context, a *domain.KBArticle) error {
	tag, err := r.db.Querier(ctx).Exec(ctx, `
		UPDATE kb_articles SET category_id=$2, title=$3, slug=$4, body=$5,
			published=$6, sort=$7
		WHERE id = $1 AND deleted_at IS NULL`,
		a.ID, a.CategoryID, a.Title, a.Slug, a.Body, a.Published, a.Sort)
	if err != nil {
		return mapPgErr("article", fmt.Errorf("knowledgebase: update article %d: %w", a.ID, err))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("article")
	}
	return nil
}

// listArticles is the shared query behind List/ListPublished/ListArticlesAdmin.
// categoryID 0 means "all categories"; publishedOnly restricts to published,
// non-deleted rows (the public view).
func (r *Repo) listArticles(ctx context.Context, categoryID int64, publishedOnly bool, p ports.ListParams) ([]domain.KBArticle, int64, error) {
	where := `deleted_at IS NULL`
	args := []any{}
	if publishedOnly {
		where += ` AND published`
	} else {
		switch p.Status {
		case "published":
			where += ` AND published`
		case "draft":
			where += ` AND NOT published`
		}
	}
	if categoryID > 0 {
		args = append(args, categoryID)
		where += fmt.Sprintf(` AND category_id = $%d`, len(args))
	}
	if p.Search != "" {
		args = append(args, "%"+p.Search+"%")
		where += fmt.Sprintf(` AND (title ILIKE $%d OR slug ILIKE $%d)`, len(args), len(args))
	}

	var total int64
	if err := r.db.Querier(ctx).QueryRow(ctx,
		`SELECT count(*) FROM kb_articles WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("knowledgebase: count articles: %w", err)
	}

	args = append(args, p.Limit(), p.Offset())
	rows, err := r.db.Querier(ctx).Query(ctx,
		`SELECT `+articleCols+` FROM kb_articles WHERE `+where+
			` ORDER BY `+orderBy(p.Sort, "sort ASC, id ASC")+
			fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("knowledgebase: list articles: %w", err)
	}
	defer rows.Close()

	var out []domain.KBArticle
	for rows.Next() {
		a, err := scanArticle(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("knowledgebase: scan article: %w", err)
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("knowledgebase: list articles rows: %w", err)
	}
	return out, total, nil
}

// List returns an admin page of articles across all categories. Search matches
// title/slug (ILIKE); Status filters "published" | "draft".
func (r *Repo) List(ctx context.Context, p ports.ListParams) ([]domain.KBArticle, int64, error) {
	return r.listArticles(ctx, 0, false, p)
}

// ListArticlesAdmin returns an admin page of articles, optionally scoped to a
// category (categoryID 0 = all). Drafts are included.
func (r *Repo) ListArticlesAdmin(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
	return r.listArticles(ctx, categoryID, false, p)
}

// ListPublished returns published, non-deleted articles, optionally scoped to a
// category (categoryID 0 = all) - the public view.
func (r *Repo) ListPublished(ctx context.Context, categoryID int64, p ports.ListParams) ([]domain.KBArticle, int64, error) {
	return r.listArticles(ctx, categoryID, true, p)
}

// SoftDeleteArticle marks an article deleted.
func (r *Repo) SoftDeleteArticle(ctx context.Context, id int64) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE kb_articles SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("knowledgebase: soft delete article %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("article")
	}
	return nil
}

// IncrementViews bumps a non-deleted article's view counter.
func (r *Repo) IncrementViews(ctx context.Context, id int64) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE kb_articles SET views = views + 1 WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("knowledgebase: increment views %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("article")
	}
	return nil
}
