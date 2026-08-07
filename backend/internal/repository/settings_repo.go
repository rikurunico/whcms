// Package repository contains the pgx implementations of the ports
// repository interfaces. One file-set per module: <mod>_repo.go (+ tests).
// Every query goes through db.Querier(ctx) so it automatically joins the
// transaction started by TxManager.WithinTx.
//
// settings_repo.go is the foundation reference implementation - copy this
// pattern for new repositories.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/platform/db"

	"github.com/jackc/pgx/v5"
)

// SettingsRepo is the pgx ports.SettingsRepo.
type SettingsRepo struct {
	db *db.DB
}

// NewSettingsRepo builds a SettingsRepo.
func NewSettingsRepo(d *db.DB) *SettingsRepo { return &SettingsRepo{db: d} }

// get fetches the raw JSONB value; found=false when the key is absent.
func (r *SettingsRepo) get(ctx context.Context, key string) (json.RawMessage, bool, error) {
	var raw json.RawMessage
	err := r.db.Querier(ctx).
		QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).
		Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("settings: get %s: %w", key, err)
	}
	return raw, true, nil
}

// GetString returns the string value for key, or def when missing/mistyped.
func (r *SettingsRepo) GetString(ctx context.Context, key, def string) (string, error) {
	raw, found, err := r.get(ctx, key)
	if err != nil {
		return def, err
	}
	if !found {
		return def, nil
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		return def, nil
	}
	return s, nil
}

// GetInt returns the int value for key, or def when missing/mistyped.
func (r *SettingsRepo) GetInt(ctx context.Context, key string, def int) (int, error) {
	raw, found, err := r.get(ctx, key)
	if err != nil {
		return def, err
	}
	if !found {
		return def, nil
	}
	var n int
	if json.Unmarshal(raw, &n) != nil {
		return def, nil
	}
	return n, nil
}

// GetBool returns the bool value for key, or def when missing/mistyped.
func (r *SettingsRepo) GetBool(ctx context.Context, key string, def bool) (bool, error) {
	raw, found, err := r.get(ctx, key)
	if err != nil {
		return def, err
	}
	if !found {
		return def, nil
	}
	var b bool
	if json.Unmarshal(raw, &b) != nil {
		return def, nil
	}
	return b, nil
}

// GetJSON unmarshals the value into out; missing keys leave out untouched.
func (r *SettingsRepo) GetJSON(ctx context.Context, key string, out any) error {
	raw, found, err := r.get(ctx, key)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("settings: decode %s: %w", key, err)
	}
	return nil
}

// Set upserts a settings key with any JSON-marshalable value.
func (r *SettingsRepo) Set(ctx context.Context, key string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("settings: marshal %s: %w", key, err)
	}
	_, err = r.db.Querier(ctx).Exec(ctx, `
		INSERT INTO settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		key, raw)
	if err != nil {
		return fmt.Errorf("settings: set %s: %w", key, err)
	}
	return nil
}

// All returns every settings row ordered by key.
func (r *SettingsRepo) All(ctx context.Context) ([]domain.Setting, error) {
	rows, err := r.db.Querier(ctx).Query(ctx, `SELECT key, value FROM settings ORDER BY key`)
	if err != nil {
		return nil, fmt.Errorf("settings: all: %w", err)
	}
	defer rows.Close()

	var out []domain.Setting
	for rows.Next() {
		var s domain.Setting
		if err := rows.Scan(&s.Key, &s.Value); err != nil {
			return nil, fmt.Errorf("settings: scan: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("settings: rows: %w", err)
	}
	return out, nil
}
