package repository

import (
	"context"
	"fmt"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
)

// IntegrationLogRepo is the pgx ports.IntegrationLogRepo.
type IntegrationLogRepo struct {
	db *db.DB
}

// NewIntegrationLogRepo builds an IntegrationLogRepo.
func NewIntegrationLogRepo(d *db.DB) *IntegrationLogRepo { return &IntegrationLogRepo{db: d} }

// Create inserts an integration log row.
func (r *IntegrationLogRepo) Create(ctx context.Context, l *domain.IntegrationLog) error {
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO integration_logs
			(provider, endpoint, method, status_code, success, latency_ms, request, response, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at`,
		l.Provider, l.Endpoint, l.Method, l.StatusCode, l.Success,
		l.LatencyMS, l.Request, l.Response, l.Error,
	).Scan(&l.ID, &l.CreatedAt)
	if err != nil {
		return fmt.Errorf("integrationlog: create: %w", err)
	}
	return nil
}

// List returns integration log rows newest first; Search filters by provider.
func (r *IntegrationLogRepo) List(ctx context.Context, p ports.ListParams) ([]domain.IntegrationLog, int64, error) {
	q := r.db.Querier(ctx)
	provider := p.Search // provider filter ('' = all)

	var total int64
	if err := q.QueryRow(ctx,
		`SELECT count(*) FROM integration_logs WHERE ($1 = '' OR provider = $1)`,
		provider).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("integrationlog: count: %w", err)
	}
	rows, err := q.Query(ctx, `
		SELECT id, provider, endpoint, method, status_code, success, latency_ms,
		       request, response, error, created_at
		FROM integration_logs
		WHERE ($1 = '' OR provider = $1)
		ORDER BY id DESC
		LIMIT $2 OFFSET $3`, provider, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("integrationlog: list: %w", err)
	}
	defer rows.Close()

	var out []domain.IntegrationLog
	for rows.Next() {
		var l domain.IntegrationLog
		if err := rows.Scan(&l.ID, &l.Provider, &l.Endpoint, &l.Method, &l.StatusCode,
			&l.Success, &l.LatencyMS, &l.Request, &l.Response, &l.Error, &l.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("integrationlog: scan: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("integrationlog: rows: %w", err)
	}
	return out, total, nil
}
