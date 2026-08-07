package repository

import (
	"context"
	"fmt"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
)

// AuditRepo is the pgx ports.AuditRepo.
type AuditRepo struct {
	db *db.DB
}

// NewAuditRepo builds an AuditRepo.
func NewAuditRepo(d *db.DB) *AuditRepo { return &AuditRepo{db: d} }

// Create inserts an audit log row.
func (r *AuditRepo) Create(ctx context.Context, a *domain.AuditLog) error {
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO audit_logs (user_id, action, entity, entity_id, before, after, ip)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		a.UserID, a.Action, a.Entity, a.EntityID, a.Before, a.After, a.IP,
	).Scan(&a.ID, &a.CreatedAt)
	if err != nil {
		return fmt.Errorf("audit: create: %w", err)
	}
	return nil
}

// List returns audit rows newest first with total count.
func (r *AuditRepo) List(ctx context.Context, p ports.ListParams) ([]domain.AuditLog, int64, error) {
	q := r.db.Querier(ctx)
	var total int64
	if err := q.QueryRow(ctx, `SELECT count(*) FROM audit_logs`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit: count: %w", err)
	}
	rows, err := q.Query(ctx, `
		SELECT id, user_id, action, entity, entity_id, before, after, ip, created_at
		FROM audit_logs
		ORDER BY id DESC
		LIMIT $1 OFFSET $2`, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("audit: list: %w", err)
	}
	defer rows.Close()

	var out []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		if err := rows.Scan(&a.ID, &a.UserID, &a.Action, &a.Entity, &a.EntityID,
			&a.Before, &a.After, &a.IP, &a.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("audit: scan: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("audit: rows: %w", err)
	}
	return out, total, nil
}
