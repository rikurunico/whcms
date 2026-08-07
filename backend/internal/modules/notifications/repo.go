package notifications

import (
	"context"
	"errors"
	"fmt"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// TemplateRepo is the pgx ports.EmailTemplateRepo (email_templates table).
type TemplateRepo struct {
	db *db.DB
}

var _ ports.EmailTemplateRepo = (*TemplateRepo)(nil)

// NewTemplateRepo builds a TemplateRepo.
func NewTemplateRepo(d *db.DB) *TemplateRepo { return &TemplateRepo{db: d} }

// Get fetches one template by exact key+locale.
func (r *TemplateRepo) Get(ctx context.Context, key, locale string) (*domain.EmailTemplate, error) {
	var t domain.EmailTemplate
	err := r.db.Querier(ctx).QueryRow(ctx, `
		SELECT id, key, locale, subject, body_html, body_text, created_at, updated_at
		FROM email_templates WHERE key = $1 AND locale = $2`, key, locale,
	).Scan(&t.ID, &t.Key, &t.Locale, &t.Subject, &t.BodyHTML, &t.BodyText,
		&t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("email template")
	}
	if err != nil {
		return nil, fmt.Errorf("emailtemplate: get %s/%s: %w", key, locale, err)
	}
	return &t, nil
}

// Upsert inserts or updates the (key, locale) variant and fills the ID and
// timestamps on t.
func (r *TemplateRepo) Upsert(ctx context.Context, t *domain.EmailTemplate) error {
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO email_templates (key, locale, subject, body_html, body_text)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (key, locale) DO UPDATE SET
			subject   = EXCLUDED.subject,
			body_html = EXCLUDED.body_html,
			body_text = EXCLUDED.body_text
		RETURNING id, created_at, updated_at`,
		t.Key, t.Locale, t.Subject, t.BodyHTML, t.BodyText,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("emailtemplate: upsert %s/%s: %w", t.Key, t.Locale, err)
	}
	return nil
}

// List returns every template ordered by key then locale.
func (r *TemplateRepo) List(ctx context.Context) ([]domain.EmailTemplate, error) {
	rows, err := r.db.Querier(ctx).Query(ctx, `
		SELECT id, key, locale, subject, body_html, body_text, created_at, updated_at
		FROM email_templates ORDER BY key, locale`)
	if err != nil {
		return nil, fmt.Errorf("emailtemplate: list: %w", err)
	}
	defer rows.Close()

	var out []domain.EmailTemplate
	for rows.Next() {
		var t domain.EmailTemplate
		if err := rows.Scan(&t.ID, &t.Key, &t.Locale, &t.Subject, &t.BodyHTML,
			&t.BodyText, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("emailtemplate: scan: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("emailtemplate: rows: %w", err)
	}
	return out, nil
}

// Delete removes one (key, locale) variant; NOT_FOUND when absent.
func (r *TemplateRepo) Delete(ctx context.Context, key, locale string) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`DELETE FROM email_templates WHERE key = $1 AND locale = $2`, key, locale)
	if err != nil {
		return fmt.Errorf("emailtemplate: delete %s/%s: %w", key, locale, err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("email template")
	}
	return nil
}
