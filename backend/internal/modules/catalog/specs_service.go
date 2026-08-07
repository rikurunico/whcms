package catalog

import (
	"context"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

// ListProductSpecs returns a product's spec knobs each with their per-cycle
// pricing (admin).
func (s *Service) ListProductSpecs(ctx context.Context, productID int64) ([]SpecWithPricing, error) {
	if _, err := s.GetProduct(ctx, productID); err != nil {
		return nil, err
	}
	specs, err := s.d.Specs.ListSpecs(ctx, productID)
	if err != nil {
		return nil, wrap(err)
	}
	out := make([]SpecWithPricing, 0, len(specs))
	for _, sp := range specs {
		pricing, err := s.d.Specs.ListSpecPricing(ctx, sp.ID)
		if err != nil {
			return nil, wrap(err)
		}
		out = append(out, SpecWithPricing{Spec: sp, Pricing: pricing})
	}
	return out, nil
}

// validateSpec enforces the cross-field rules the struct tags cannot express.
func validateSpec(in SpecInput) error {
	var details []apperr.FieldError
	if in.MaxQty != 0 && in.MaxQty < in.MinQty {
		details = append(details, apperr.FieldError{Field: "max_qty", Message: "must be 0 (unbounded) or >= min_qty"})
	}
	if in.DefaultQty < in.MinQty {
		details = append(details, apperr.FieldError{Field: "default_qty", Message: "must be >= min_qty"})
	}
	if in.MaxQty != 0 && in.DefaultQty > in.MaxQty {
		details = append(details, apperr.FieldError{Field: "default_qty", Message: "must be <= max_qty"})
	}
	if len(details) > 0 {
		return apperr.Validation("invalid product spec", details...)
	}
	return nil
}

// CreateSpec adds a spec knob to a product.
func (s *Service) CreateSpec(ctx context.Context, actorUserID, productID int64, in SpecInput) (*domain.ProductSpec, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	if err := validateSpec(in); err != nil {
		return nil, err
	}
	p, err := s.GetProduct(ctx, productID)
	if err != nil {
		return nil, err
	}
	if p.Module == domain.ModuleNone {
		return nil, apperr.Validation("invalid product spec",
			apperr.FieldError{Field: "product_id", Message: "product has no provisioning module"})
	}
	sp := specFromInput(productID, in)
	if err := s.d.Specs.CreateSpec(ctx, sp); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "catalog.spec.create", "product_spec", sp.ID, nil, sp)
	s.invalidateCatalogCache(ctx)
	return sp, nil
}

// UpdateSpec replaces all mutable fields of a spec knob.
func (s *Service) UpdateSpec(ctx context.Context, actorUserID, specID int64, in SpecInput) (*domain.ProductSpec, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	if err := validateSpec(in); err != nil {
		return nil, err
	}
	before, err := s.d.Specs.GetSpecByID(ctx, specID)
	if err != nil {
		return nil, wrap(err)
	}
	sp := specFromInput(before.ProductID, in)
	sp.ID = before.ID
	if err := s.d.Specs.UpdateSpec(ctx, sp); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "catalog.spec.update", "product_spec", sp.ID, before, sp)
	s.invalidateCatalogCache(ctx)
	return sp, nil
}

// DeleteSpec removes a spec knob (its pricing cascades).
func (s *Service) DeleteSpec(ctx context.Context, actorUserID, specID int64) error {
	if err := s.d.Specs.DeleteSpec(ctx, specID); err != nil {
		return wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "catalog.spec.delete", "product_spec", specID, nil, nil)
	s.invalidateCatalogCache(ctx)
	return nil
}

// UpsertSpecPricing sets the per-unit IDR price for one billing cycle of a spec.
func (s *Service) UpsertSpecPricing(ctx context.Context, actorUserID, specID int64, in SpecPricingInput) (*domain.ProductSpecPricing, error) {
	if err := s.val.Struct(in); err != nil {
		return nil, err
	}
	currency := in.Currency
	if currency == "" {
		currency = "IDR"
	}
	if currency != "IDR" {
		return nil, apperr.Validation("only IDR pricing is supported",
			apperr.FieldError{Field: "currency", Message: "must be IDR"})
	}
	if _, err := s.d.Specs.GetSpecByID(ctx, specID); err != nil {
		return nil, wrap(err)
	}
	p := &domain.ProductSpecPricing{
		SpecID:         specID,
		Cycle:          domain.BillingCycle(in.Cycle),
		UnitPrice:      in.UnitPrice,
		UnlimitedPrice: in.UnlimitedPrice,
		Currency:       currency,
	}
	if err := s.d.Specs.UpsertSpecPricing(ctx, p); err != nil {
		return nil, wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "catalog.spec_pricing.upsert", "product_spec", specID, nil, p)
	s.invalidateCatalogCache(ctx)
	return p, nil
}

// DeleteSpecPricing removes one billing-cycle price from a spec.
func (s *Service) DeleteSpecPricing(ctx context.Context, actorUserID, specID int64, cycle string) error {
	c := domain.BillingCycle(cycle)
	if !c.Valid() {
		return apperr.Validation("invalid billing cycle",
			apperr.FieldError{Field: "cycle", Message: "unknown billing cycle"})
	}
	if err := s.d.Specs.DeleteSpecPricing(ctx, specID, c); err != nil {
		return wrap(err)
	}
	s.d.Audit.Log(ctx, actorUserID, "catalog.spec_pricing.delete", "product_spec", specID, map[string]string{"cycle": cycle}, nil)
	s.invalidateCatalogCache(ctx)
	return nil
}

func specFromInput(productID int64, in SpecInput) *domain.ProductSpec {
	return &domain.ProductSpec{
		ProductID:      productID,
		Key:            in.Key,
		Label:          in.Label,
		ProvisionKey:   domain.ProvisionKey(in.ProvisionKey),
		Unit:           domain.SpecUnit(in.Unit),
		IncludedQty:    in.IncludedQty,
		MinQty:         in.MinQty,
		MaxQty:         in.MaxQty,
		StepQty:        in.StepQty,
		DefaultQty:     in.DefaultQty,
		AllowUnlimited: in.AllowUnlimited,
		Sort:           in.Sort,
	}
}
