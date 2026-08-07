package networkstatus

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

// Repo is the pgx persistence for the network status module. It implements
// ports.NetworkStatusRepo. Every query goes through db.Querier(ctx) so it joins
// any transaction started by TxManager.WithinTx.
type Repo struct {
	db *db.DB
}

// NewRepo builds the network status Repo.
func NewRepo(d *db.DB) *Repo { return &Repo{db: d} }

var _ ports.NetworkStatusRepo = (*Repo)(nil)

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

const issueCols = `id, title, body, type, severity, status, affected,
	starts_at, ends_at, deleted_at, created_at, updated_at`

func scanIssue(row pgx.Row) (*domain.NetworkIssue, error) {
	var n domain.NetworkIssue
	err := row.Scan(&n.ID, &n.Title, &n.Body, &n.Type, &n.Severity, &n.Status,
		&n.Affected, &n.StartsAt, &n.EndsAt, &n.DeletedAt, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// listSortColumns whitelists ORDER BY columns.
var listSortColumns = map[string]string{
	"id": "id", "title": "title", "type": "type", "severity": "severity",
	"status": "status", "starts_at": "starts_at", "created_at": "created_at",
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

// Create inserts a network status entry.
func (r *Repo) Create(ctx context.Context, n *domain.NetworkIssue) error {
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO network_issues (title, body, type, severity, status, affected, starts_at, ends_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		RETURNING id, created_at, updated_at`,
		n.Title, n.Body, n.Type, n.Severity, n.Status, n.Affected, n.StartsAt, n.EndsAt,
	).Scan(&n.ID, &n.CreatedAt, &n.UpdatedAt)
	if err != nil {
		return mapPgErr("network issue", fmt.Errorf("networkstatus: create issue: %w", err))
	}
	return nil
}

// GetByID returns one non-deleted network status entry.
func (r *Repo) GetByID(ctx context.Context, id int64) (*domain.NetworkIssue, error) {
	n, err := scanIssue(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+issueCols+` FROM network_issues WHERE id = $1 AND deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("network issue")
	}
	if err != nil {
		return nil, fmt.Errorf("networkstatus: get issue %d: %w", id, err)
	}
	return n, nil
}

// Update saves all mutable columns of a network status entry.
func (r *Repo) Update(ctx context.Context, n *domain.NetworkIssue) error {
	tag, err := r.db.Querier(ctx).Exec(ctx, `
		UPDATE network_issues SET title=$2, body=$3, type=$4, severity=$5,
			status=$6, affected=$7, starts_at=$8, ends_at=$9
		WHERE id = $1 AND deleted_at IS NULL`,
		n.ID, n.Title, n.Body, n.Type, n.Severity, n.Status, n.Affected, n.StartsAt, n.EndsAt)
	if err != nil {
		return mapPgErr("network issue", fmt.Errorf("networkstatus: update issue %d: %w", n.ID, err))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("network issue")
	}
	return nil
}

// List returns a page of entries + total. Search matches title/body/affected
// (ILIKE); Status filters an exact status value.
func (r *Repo) List(ctx context.Context, p ports.ListParams) ([]domain.NetworkIssue, int64, error) {
	where := `deleted_at IS NULL`
	args := []any{}
	if p.Search != "" {
		args = append(args, "%"+p.Search+"%")
		where += fmt.Sprintf(` AND (title ILIKE $%d OR body ILIKE $%d OR affected ILIKE $%d)`,
			len(args), len(args), len(args))
	}
	if p.Status != "" {
		args = append(args, p.Status)
		where += fmt.Sprintf(` AND status = $%d`, len(args))
	}

	var total int64
	if err := r.db.Querier(ctx).QueryRow(ctx,
		`SELECT count(*) FROM network_issues WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("networkstatus: count issues: %w", err)
	}

	args = append(args, p.Limit(), p.Offset())
	rows, err := r.db.Querier(ctx).Query(ctx,
		`SELECT `+issueCols+` FROM network_issues WHERE `+where+
			` ORDER BY `+orderBy(p.Sort, "starts_at DESC, id DESC")+
			fmt.Sprintf(` LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("networkstatus: list issues: %w", err)
	}
	defer rows.Close()

	out := []domain.NetworkIssue{}
	for rows.Next() {
		n, err := scanIssue(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("networkstatus: scan issue: %w", err)
		}
		out = append(out, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("networkstatus: list issues rows: %w", err)
	}
	return out, total, nil
}

// ListActive returns non-deleted entries that are unresolved, or were resolved
// within the last 7 days (by ends_at, falling back to updated_at), newest
// first - the public status view.
func (r *Repo) ListActive(ctx context.Context) ([]domain.NetworkIssue, error) {
	rows, err := r.db.Querier(ctx).Query(ctx,
		`SELECT `+issueCols+` FROM network_issues
		 WHERE deleted_at IS NULL
		   AND (status <> 'resolved'
		        OR COALESCE(ends_at, updated_at) >= now() - interval '7 days')
		 ORDER BY starts_at DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("networkstatus: list active issues: %w", err)
	}
	defer rows.Close()

	out := []domain.NetworkIssue{}
	for rows.Next() {
		n, err := scanIssue(rows)
		if err != nil {
			return nil, fmt.Errorf("networkstatus: scan issue: %w", err)
		}
		out = append(out, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("networkstatus: active issues rows: %w", err)
	}
	return out, nil
}

// SoftDelete marks a network status entry deleted.
func (r *Repo) SoftDelete(ctx context.Context, id int64) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE network_issues SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("networkstatus: soft delete issue %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("network issue")
	}
	return nil
}
