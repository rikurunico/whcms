package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// EmailLogRepo is the pgx ports.EmailLogRepo.
type EmailLogRepo struct {
	db *db.DB
}

// NewEmailLogRepo builds an EmailLogRepo.
func NewEmailLogRepo(d *db.DB) *EmailLogRepo { return &EmailLogRepo{db: d} }

// Create inserts an email log row (status defaults to queued when empty).
func (r *EmailLogRepo) Create(ctx context.Context, e *domain.EmailLogEntry) error {
	if e.Status == "" {
		e.Status = domain.EmailQueued
	}
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO email_log (user_id, to_email, template_key, subject, status, error, sent_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		e.UserID, e.ToEmail, e.TemplateKey, e.Subject, e.Status, e.Error, e.SentAt,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return fmt.Errorf("emaillog: create: %w", err)
	}
	return nil
}

// GetByID fetches one email log row.
func (r *EmailLogRepo) GetByID(ctx context.Context, id int64) (*domain.EmailLogEntry, error) {
	var e domain.EmailLogEntry
	err := r.db.Querier(ctx).QueryRow(ctx, `
		SELECT id, user_id, to_email, template_key, subject, status, error, sent_at, created_at, updated_at
		FROM email_log WHERE id = $1`, id,
	).Scan(&e.ID, &e.UserID, &e.ToEmail, &e.TemplateKey, &e.Subject, &e.Status, &e.Error,
		&e.SentAt, &e.CreatedAt, &e.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, apperr.NotFound("email log entry")
	}
	if err != nil {
		return nil, fmt.Errorf("emaillog: get %d: %w", id, err)
	}
	return &e, nil
}

// MarkSent sets status=sent and the sent timestamp.
func (r *EmailLogRepo) MarkSent(ctx context.Context, id int64, at time.Time) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE email_log SET status = 'sent', sent_at = $2, error = '' WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("emaillog: mark sent %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("email log entry")
	}
	return nil
}

// MarkFailed sets status=failed with the error message.
func (r *EmailLogRepo) MarkFailed(ctx context.Context, id int64, errMsg string) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE email_log SET status = 'failed', error = $2 WHERE id = $1`, id, errMsg)
	if err != nil {
		return fmt.Errorf("emaillog: mark failed %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("email log entry")
	}
	return nil
}

// List returns email log rows newest first; Search matches to_email ILIKE,
// Status filters exact, UserID (when non-zero) restricts to one recipient
// account.
func (r *EmailLogRepo) List(ctx context.Context, p ports.ListParams) ([]domain.EmailLogEntry, int64, error) {
	q := r.db.Querier(ctx)
	search := "%" + p.Search + "%"

	var total int64
	if err := q.QueryRow(ctx, `
		SELECT count(*) FROM email_log
		WHERE ($1 = '%%' OR to_email ILIKE $1) AND ($2 = '' OR status = $2)
		  AND ($3 = 0 OR user_id = $3)`,
		search, p.Status, p.UserID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("emaillog: count: %w", err)
	}
	rows, err := q.Query(ctx, `
		SELECT id, user_id, to_email, template_key, subject, status, error, sent_at, created_at, updated_at
		FROM email_log
		WHERE ($1 = '%%' OR to_email ILIKE $1) AND ($2 = '' OR status = $2)
		  AND ($3 = 0 OR user_id = $3)
		ORDER BY id DESC
		LIMIT $4 OFFSET $5`, search, p.Status, p.UserID, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("emaillog: list: %w", err)
	}
	defer rows.Close()

	var out []domain.EmailLogEntry
	for rows.Next() {
		var e domain.EmailLogEntry
		if err := rows.Scan(&e.ID, &e.UserID, &e.ToEmail, &e.TemplateKey, &e.Subject, &e.Status,
			&e.Error, &e.SentAt, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, 0, fmt.Errorf("emaillog: scan: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("emaillog: rows: %w", err)
	}
	return out, total, nil
}
