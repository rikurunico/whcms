package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/jackc/pgx/v5"
)

// Product specs (dynamic/custom-spec products)

const specCols = `id, product_id, key, label, provision_key, unit, included_qty,
	min_qty, max_qty, step_qty, default_qty, allow_unlimited, sort, created_at, updated_at`

func scanSpec(row pgx.Row) (*domain.ProductSpec, error) {
	var s domain.ProductSpec
	err := row.Scan(&s.ID, &s.ProductID, &s.Key, &s.Label, &s.ProvisionKey, &s.Unit,
		&s.IncludedQty, &s.MinQty, &s.MaxQty, &s.StepQty, &s.DefaultQty,
		&s.AllowUnlimited, &s.Sort, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// ListSpecs returns all specs of a product ordered by sort, id.
func (r *Repo) ListSpecs(ctx context.Context, productID int64) ([]domain.ProductSpec, error) {
	rows, err := r.db.Querier(ctx).Query(ctx,
		`SELECT `+specCols+` FROM product_specs WHERE product_id = $1 ORDER BY sort, id`, productID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list specs %d: %w", productID, err)
	}
	defer rows.Close()

	var out []domain.ProductSpec
	for rows.Next() {
		s, err := scanSpec(rows)
		if err != nil {
			return nil, fmt.Errorf("catalog: scan spec: %w", err)
		}
		out = append(out, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: spec rows: %w", err)
	}
	return out, nil
}

// GetSpecByID returns one spec.
func (r *Repo) GetSpecByID(ctx context.Context, id int64) (*domain.ProductSpec, error) {
	s, err := scanSpec(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+specCols+` FROM product_specs WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("product spec")
	}
	if err != nil {
		return nil, fmt.Errorf("catalog: get spec %d: %w", id, err)
	}
	return s, nil
}

// CreateSpec inserts a product spec.
func (r *Repo) CreateSpec(ctx context.Context, s *domain.ProductSpec) error {
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO product_specs (product_id, key, label, provision_key, unit,
			included_qty, min_qty, max_qty, step_qty, default_qty, allow_unlimited, sort)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at, updated_at`,
		s.ProductID, s.Key, s.Label, s.ProvisionKey, s.Unit, s.IncludedQty,
		s.MinQty, s.MaxQty, s.StepQty, s.DefaultQty, s.AllowUnlimited, s.Sort,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return mapPgErr("product spec", fmt.Errorf("catalog: create spec: %w", err))
	}
	return nil
}

// UpdateSpec saves all mutable spec columns.
func (r *Repo) UpdateSpec(ctx context.Context, s *domain.ProductSpec) error {
	tag, err := r.db.Querier(ctx).Exec(ctx, `
		UPDATE product_specs SET key=$2, label=$3, provision_key=$4, unit=$5,
			included_qty=$6, min_qty=$7, max_qty=$8, step_qty=$9, default_qty=$10,
			allow_unlimited=$11, sort=$12
		WHERE id = $1`,
		s.ID, s.Key, s.Label, s.ProvisionKey, s.Unit, s.IncludedQty, s.MinQty,
		s.MaxQty, s.StepQty, s.DefaultQty, s.AllowUnlimited, s.Sort)
	if err != nil {
		return mapPgErr("product spec", fmt.Errorf("catalog: update spec %d: %w", s.ID, err))
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("product spec")
	}
	return nil
}

// DeleteSpec removes a spec (its pricing cascades).
func (r *Repo) DeleteSpec(ctx context.Context, id int64) error {
	tag, err := r.db.Querier(ctx).Exec(ctx, `DELETE FROM product_specs WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("catalog: delete spec %d: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("product spec")
	}
	return nil
}

// Product spec pricing (per-cycle IDR unit pricing)

const specPricingCols = `id, spec_id, cycle, unit_price, unlimited_price, currency, created_at, updated_at`

func scanSpecPricing(row pgx.Row) (*domain.ProductSpecPricing, error) {
	var p domain.ProductSpecPricing
	err := row.Scan(&p.ID, &p.SpecID, &p.Cycle, &p.UnitPrice, &p.UnlimitedPrice,
		&p.Currency, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertSpecPricing inserts or updates the per-unit price for (spec, cycle).
func (r *Repo) UpsertSpecPricing(ctx context.Context, p *domain.ProductSpecPricing) error {
	if p.Currency == "" {
		p.Currency = "IDR"
	}
	if p.Currency != "IDR" {
		return apperr.Validation("only IDR pricing is supported",
			apperr.FieldError{Field: "currency", Message: "must be IDR"})
	}
	err := r.db.Querier(ctx).QueryRow(ctx, `
		INSERT INTO product_spec_pricing (spec_id, cycle, unit_price, unlimited_price, currency)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (spec_id, cycle, currency)
		DO UPDATE SET unit_price = EXCLUDED.unit_price, unlimited_price = EXCLUDED.unlimited_price
		RETURNING id, created_at, updated_at`,
		p.SpecID, p.Cycle, p.UnitPrice, p.UnlimitedPrice, p.Currency,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return mapPgErr("spec pricing", fmt.Errorf("catalog: upsert spec pricing: %w", err))
	}
	return nil
}

// GetSpecPricing returns the IDR unit-price row for (spec, cycle).
func (r *Repo) GetSpecPricing(ctx context.Context, specID int64, cycle domain.BillingCycle) (*domain.ProductSpecPricing, error) {
	p, err := scanSpecPricing(r.db.Querier(ctx).QueryRow(ctx,
		`SELECT `+specPricingCols+` FROM product_spec_pricing
		 WHERE spec_id = $1 AND cycle = $2 AND currency = 'IDR'`, specID, cycle))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.NotFound("spec pricing")
	}
	if err != nil {
		return nil, fmt.Errorf("catalog: get spec pricing %d/%s: %w", specID, cycle, err)
	}
	return p, nil
}

// ListSpecPricing returns all IDR unit-price rows of a spec.
func (r *Repo) ListSpecPricing(ctx context.Context, specID int64) ([]domain.ProductSpecPricing, error) {
	rows, err := r.db.Querier(ctx).Query(ctx,
		`SELECT `+specPricingCols+` FROM product_spec_pricing
		 WHERE spec_id = $1 AND currency = 'IDR' ORDER BY id`, specID)
	if err != nil {
		return nil, fmt.Errorf("catalog: list spec pricing %d: %w", specID, err)
	}
	defer rows.Close()

	var out []domain.ProductSpecPricing
	for rows.Next() {
		p, err := scanSpecPricing(rows)
		if err != nil {
			return nil, fmt.Errorf("catalog: scan spec pricing: %w", err)
		}
		out = append(out, *p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("catalog: spec pricing rows: %w", err)
	}
	return out, nil
}

// DeleteSpecPricing removes the IDR unit-price row for (spec, cycle).
func (r *Repo) DeleteSpecPricing(ctx context.Context, specID int64, cycle domain.BillingCycle) error {
	tag, err := r.db.Querier(ctx).Exec(ctx,
		`DELETE FROM product_spec_pricing WHERE spec_id = $1 AND cycle = $2 AND currency = 'IDR'`,
		specID, cycle)
	if err != nil {
		return fmt.Errorf("catalog: delete spec pricing %d/%s: %w", specID, cycle, err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("spec pricing")
	}
	return nil
}
