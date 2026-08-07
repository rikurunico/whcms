package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// UserRepo is the pgx implementation of ports.UserRepo (owned by the auth
// module; reused by M-ADMINOPS for staff management via the port).
type UserRepo struct {
	db *db.DB
}

// Compile-time check.
var _ ports.UserRepo = (*UserRepo)(nil)

// NewUserRepo builds a UserRepo.
func NewUserRepo(d *db.DB) *UserRepo { return &UserRepo{db: d} }

const userCols = `id, email, password_hash, role, status, permissions,
	twofa_secret_enc, twofa_enabled, locale, email_verified_at, last_login_at,
	created_at, updated_at`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.Status,
		&u.Permissions, &u.TwoFASecretEnc, &u.TwoFAEnabled, &u.Locale,
		&u.EmailVerifiedAt, &u.LastLoginAt, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("user")
	}
	if err != nil {
		return nil, fmt.Errorf("users: scan: %w", err)
	}
	return &u, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// Create inserts the user and fills ID/CreatedAt/UpdatedAt. A duplicate email
// returns a CONFLICT apperr.
func (r *UserRepo) Create(ctx context.Context, u *domain.User) error {
	perms := u.Permissions
	if len(perms) == 0 {
		perms = json.RawMessage(`{}`)
	}
	locale := u.Locale
	if locale == "" {
		locale = defaultLocale
	}
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO users (email, password_hash, role, status, permissions,
			twofa_secret_enc, twofa_enabled, locale, email_verified_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`,
		u.Email, u.PasswordHash, u.Role, u.Status, perms,
		u.TwoFASecretEnc, u.TwoFAEnabled, locale, u.EmailVerifiedAt,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if isUniqueViolation(err) {
		return apperr.Conflict("email already registered")
	}
	if err != nil {
		return fmt.Errorf("users: create: %w", err)
	}
	u.Permissions = perms
	u.Locale = locale
	return nil
}

// GetByID returns the user or NOT_FOUND.
func (r *UserRepo) GetByID(ctx context.Context, id int64) (*domain.User, error) {
	return scanUser(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

// GetByEmail returns the user (case-insensitive email) or NOT_FOUND.
func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	return scanUser(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+userCols+` FROM users WHERE lower(email) = lower($1)`, email))
}

// Update persists the mutable fields (email, role, status, permissions, 2FA
// fields, locale). Password changes go through UpdatePassword.
func (r *UserRepo) Update(ctx context.Context, u *domain.User) error {
	perms := u.Permissions
	if len(perms) == 0 {
		perms = json.RawMessage(`{}`)
	}
	tag, err := r.db.Querier(ctx).Exec(ctx, `
		UPDATE users SET email = $2, role = $3, status = $4, permissions = $5,
			twofa_secret_enc = $6, twofa_enabled = $7, locale = $8
		WHERE id = $1`,
		u.ID, u.Email, u.Role, u.Status, perms,
		u.TwoFASecretEnc, u.TwoFAEnabled, u.Locale)
	if isUniqueViolation(err) {
		return apperr.Conflict("email already registered")
	}
	if err != nil {
		return fmt.Errorf("users: update: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("user")
	}
	return nil
}

// UpdatePassword sets a new password hash.
func (r *UserRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE users SET password_hash = $2 WHERE id = $1`, id, passwordHash)
	if err != nil {
		return fmt.Errorf("users: update password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("user")
	}
	return nil
}

// SetEmailVerified marks the email verified at the given time.
func (r *UserRepo) SetEmailVerified(ctx context.Context, id int64, at time.Time) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE users SET email_verified_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("users: set verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("user")
	}
	return nil
}

// SetLastLogin records the last successful login time.
func (r *UserRepo) SetLastLogin(ctx context.Context, id int64, at time.Time) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`UPDATE users SET last_login_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("users: set last login: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("user")
	}
	return nil
}

// List returns users filtered by Search (email ILIKE) and Status, newest
// first, with the total count. Role filtering: pass the role name via
// p.Status ("admin"/"staff"/"client" are not user statuses) is NOT supported;
// callers filter by user status only.
func (r *UserRepo) List(ctx context.Context, p ports.ListParams) ([]domain.User, int64, error) {
	q := r.db.Querier(ctx)

	where := ` WHERE ($1 = '' OR email ILIKE '%' || $1 || '%')
		AND ($2 = '' OR status = $2)`

	var total int64
	if err := q.QueryRow(ctx, `SELECT count(*) FROM users`+where, p.Search, p.Status).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("users: count: %w", err)
	}

	rows, err := q.Query(ctx, `SELECT `+userCols+` FROM users`+where+`
		ORDER BY id DESC LIMIT $3 OFFSET $4`,
		p.Search, p.Status, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("users: list: %w", err)
	}
	defer rows.Close()

	var out []domain.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("users: rows: %w", err)
	}
	return out, total, nil
}

// Delete removes the user row (cascades to the client profile).
func (r *UserRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Querier(ctx).Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("users: delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("user")
	}
	return nil
}

// ExistsAnyAdmin reports whether any role='admin' user exists.
func (r *UserRepo) ExistsAnyAdmin(ctx context.Context) (bool, error) {
	var exists bool
	err := r.db.Querier(ctx).QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE role = $1)`, domain.RoleAdmin,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("users: exists any admin: %w", err)
	}
	return exists, nil
}
