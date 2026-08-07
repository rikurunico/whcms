package announcements

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

// Repo is the pgx persistence for the announcements module. It implements
// ports.AnnouncementRepo. Every query goes through db.Querier(ctx) so it joins
// any transaction started by TxManager.WithinTx.
type Repo struct {
	db *db.DB
}

// NewRepo builds the announcements Repo.
func NewRepo(d *db.DB) *Repo { return &Repo{db: d} }

var _ ports.AnnouncementRepo = (*Repo)(nil)

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

const announcementCols = `id, title, slug, body, published, published_at,
	author_id, deleted_at, created_at, updated_at`

func scanAnnouncement(row pgx.Row) (*domain.Announcement, error) {
	var a domain.Announcement
	err := row.Scan(&a.ID, &a.Title, &a.Slug, &a.Body, &a.Published, &a.PublishedAt,
		&a.AuthorID, &a.DeletedAt, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// listSortColumns whitelists ORDER BY columns for the admin listing.
var listSortColumns = map[string]string{
	"id": "id", "title": "title", "slug": "slug",
	"created_at": "created_at", "published_at": "published_at",
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

// Create inserts an announcement.
func (r *Repo) Create(ctx context.Context, a *domain.Announcement) error {
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO announcements (title, slug, body, published, published_at, author_id)
		VALUES ($1,$2,$3,$4,$5,$6)
		RETURNING id, created_at, updated_at`,
		a.Title, a.Slug, a.Body, a.Published, a.PublishedAt, a.AuthorID,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return mapPgErr("announcement", fmt.Errorf("announcements: create: %w", err))
	}
	return nil
}

// GetByID returns one non-deleted announcement.
func (r *Repo) GetByID(ctx context.Context, id int64) (*domain.Announcement, error) {
	a, err := scanAnnouncement(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+announcementCols+` FROM announcements WHERE id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("announcement")
	}
	if err != nil {
		return nil, fmt.Errorf("announcements: get %d: %w", id, err)
	}
	return a, nil
}

// GetBySlug returns one non-deleted announcement by slug.
func (r *Repo) GetBySlug(ctx context.Context, slug string) (*domain.Announcement, error) {
	a, err := scanAnnouncement(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+announcementCols+` FROM announcements WHERE slug = $1 AND deleted_at IS NULL`, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("announcement")
	}
	if err != nil {
		return nil, fmt.Errorf("announcements: get %q: %w", slug, err)
	}
	return a, nil
}

// Update saves all mutable announcement columns (author_id is immutable).
func (r *Repo) Update(ctx context.Context, a *domain.Announcement) error {
	tag, err := r.db.Querier(ctx).Exec(ctx, `
		UPDATE announcements SET title=$2, slug=$3, body=$4, published=$5, published_at=$6
		WHERE id = $1 AND deleted_at IS NULL`,
		a.ID, a.Title, a.Slug, a.Body, a.Published, a.PublishedAt)
	if err != nil {
		return mapPgErr("announcement", fmt.Errorf("announcements: update %d: %w", a.ID, err))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("announcement")
	}
	return nil
}

// List returns announcements page + total (admin). Search matches title/slug
// (ILIKE); Status filters "published" | "draft".
func (r *Repo) List(ctx context.Context, p ports.ListParams) ([]domain.Announcement, int64, error) {
	where := `deleted_at IS NULL`
	args := []any{}
	if p.Search != "" {
		args = append(args, "%"+p.Search+"%")
		where += fmt.Sprintf(` AND (title ILIKE $%d OR slug ILIKE $%d)`, len(args), len(args))
	}
	switch p.Status {
	case "published":
		where += ` AND published`
	case "draft":
		where += ` AND NOT published`
	}

	var total int64
	if err := r.db.Querier(ctx).QueryRow(ctx,
		`SELECT count(*) FROM announcements WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("announcements: count: %w", err)
	}

	args = append(args, p.Limit(), p.Offset())
	rows, err := r.db.Querier(ctx).Query(ctx,
		`SELECT `+announcementCols+` FROM announcements WHERE `+where+
			` ORDER BY `+orderBy(p.Sort, "created_at DESC, id DESC")+
			fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("announcements: list: %w", err)
	}
	defer rows.Close()

	var out []domain.Announcement
	for rows.Next() {
		a, err := scanAnnouncement(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("announcements: scan: %w", err)
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("announcements: list rows: %w", err)
	}
	return out, total, nil
}

// ListPublished returns published, non-deleted announcements whose publish time
// has passed, newest first (public view).
func (r *Repo) ListPublished(ctx context.Context, p ports.ListParams) ([]domain.Announcement, int64, error) {
	const where = `deleted_at IS NULL AND published
		AND published_at IS NOT NULL AND published_at <= now()`

	var total int64
	if err := r.db.Querier(ctx).QueryRow(ctx,
		`SELECT count(*) FROM announcements WHERE `+where).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("announcements: count published: %w", err)
	}

	rows, err := r.db.Querier(ctx).Query(ctx,
		`SELECT `+announcementCols+` FROM announcements WHERE `+where+
			` ORDER BY published_at DESC, id DESC LIMIT $1 OFFSET $2`,
		p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("announcements: list published: %w", err)
	}
	defer rows.Close()

	var out []domain.Announcement
	for rows.Next() {
		a, err := scanAnnouncement(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("announcements: scan published: %w", err)
		}
		out = append(out, *a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("announcements: published rows: %w", err)
	}
	return out, total, nil
}

// SoftDelete marks an announcement deleted.
func (r *Repo) SoftDelete(ctx context.Context, id int64) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE announcements SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("announcements: soft delete %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("announcement")
	}
	return nil
}
