package catalog_test

import (
	"context"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/catalog"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSpecStore is a nil-safe function-field catalog.SpecStore.
type fakeSpecStore struct {
	ListSpecsFn                func(ctx context.Context, productID int64) ([]domain.ProductSpec, error)
	GetSpecByIDFn              func(ctx context.Context, id int64) (*domain.ProductSpec, error)
	CreateSpecFn               func(ctx context.Context, s *domain.ProductSpec) error
	UpdateSpecFn               func(ctx context.Context, s *domain.ProductSpec) error
	DeleteSpecFn               func(ctx context.Context, id int64) error
	ListSpecPricingFn          func(ctx context.Context, specID int64) ([]domain.ProductSpecPricing, error)
	ListSpecPricingBySpecIDsFn func(ctx context.Context, specIDs []int64) (map[int64][]domain.ProductSpecPricing, error)
	GetSpecPricingFn           func(ctx context.Context, specID int64, cycle domain.BillingCycle) (*domain.ProductSpecPricing, error)
	UpsertSpecPriceFn          func(ctx context.Context, p *domain.ProductSpecPricing) error
	DeleteSpecPriceFn          func(ctx context.Context, specID int64, cycle domain.BillingCycle) error
}

func (f *fakeSpecStore) ListSpecs(ctx context.Context, productID int64) ([]domain.ProductSpec, error) {
	if f.ListSpecsFn != nil {
		return f.ListSpecsFn(ctx, productID)
	}
	return nil, nil
}
func (f *fakeSpecStore) GetSpecByID(ctx context.Context, id int64) (*domain.ProductSpec, error) {
	if f.GetSpecByIDFn != nil {
		return f.GetSpecByIDFn(ctx, id)
	}
	return &domain.ProductSpec{ID: id, ProductID: 1}, nil
}
func (f *fakeSpecStore) CreateSpec(ctx context.Context, s *domain.ProductSpec) error {
	if f.CreateSpecFn != nil {
		return f.CreateSpecFn(ctx, s)
	}
	s.ID = 10
	return nil
}
func (f *fakeSpecStore) UpdateSpec(ctx context.Context, s *domain.ProductSpec) error {
	if f.UpdateSpecFn != nil {
		return f.UpdateSpecFn(ctx, s)
	}
	return nil
}
func (f *fakeSpecStore) DeleteSpec(ctx context.Context, id int64) error {
	if f.DeleteSpecFn != nil {
		return f.DeleteSpecFn(ctx, id)
	}
	return nil
}
func (f *fakeSpecStore) ListSpecPricing(ctx context.Context, specID int64) ([]domain.ProductSpecPricing, error) {
	if f.ListSpecPricingFn != nil {
		return f.ListSpecPricingFn(ctx, specID)
	}
	return nil, nil
}
func (f *fakeSpecStore) ListSpecPricingBySpecIDs(ctx context.Context, specIDs []int64) (map[int64][]domain.ProductSpecPricing, error) {
	if f.ListSpecPricingBySpecIDsFn != nil {
		return f.ListSpecPricingBySpecIDsFn(ctx, specIDs)
	}
	out := make(map[int64][]domain.ProductSpecPricing)
	for _, id := range specIDs {
		pricing, err := f.ListSpecPricing(ctx, id)
		if err != nil {
			return nil, err
		}
		out[id] = pricing
	}
	return out, nil
}
func (f *fakeSpecStore) GetSpecPricing(ctx context.Context, specID int64, cycle domain.BillingCycle) (*domain.ProductSpecPricing, error) {
	if f.GetSpecPricingFn != nil {
		return f.GetSpecPricingFn(ctx, specID, cycle)
	}
	return nil, nil
}
func (f *fakeSpecStore) UpsertSpecPricing(ctx context.Context, p *domain.ProductSpecPricing) error {
	if f.UpsertSpecPriceFn != nil {
		return f.UpsertSpecPriceFn(ctx, p)
	}
	return nil
}
func (f *fakeSpecStore) DeleteSpecPricing(ctx context.Context, specID int64, cycle domain.BillingCycle) error {
	if f.DeleteSpecPriceFn != nil {
		return f.DeleteSpecPriceFn(ctx, specID, cycle)
	}
	return nil
}

func validSpecInput() catalog.SpecInput {
	return catalog.SpecInput{
		Key:          "disk",
		Label:        "Disk space",
		ProvisionKey: "disk",
		Unit:         "gb",
		IncludedQty:  5,
		MinQty:       5,
		MaxQty:       100,
		StepQty:      5,
		DefaultQty:   10,
		Sort:         1,
	}
}

func cpanelProduct(id int64) *domain.Product {
	return &domain.Product{ID: id, Module: domain.ModuleCpanel, Configurable: true}
}

func TestCreateSpec_Success(t *testing.T) {
	f := newFixture()
	f.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) { return cpanelProduct(id), nil }
	var created *domain.ProductSpec
	f.specs.CreateSpecFn = func(_ context.Context, s *domain.ProductSpec) error {
		s.ID = 7
		created = s
		return nil
	}

	sp, err := f.svc.CreateSpec(context.Background(), 1, 1, validSpecInput())
	require.NoError(t, err)
	assert.Equal(t, int64(7), sp.ID)
	assert.Equal(t, domain.SpecDisk, created.ProvisionKey)
	assert.Equal(t, domain.UnitGB, created.Unit)
	assert.Len(t, f.audit.Entries, 1)
}

func TestCreateSpec_ModuleNone(t *testing.T) {
	f := newFixture()
	f.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) {
		return &domain.Product{ID: id, Module: domain.ModuleNone}, nil
	}
	_, err := f.svc.CreateSpec(context.Background(), 1, 1, validSpecInput())
	assertCode(t, err, apperr.CodeValidation)
}

func TestCreateSpec_ValidationBranches(t *testing.T) {
	f := newFixture()
	f.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) { return cpanelProduct(id), nil }

	tests := []struct {
		name string
		mut  func(in *catalog.SpecInput)
	}{
		{"bad provision_key", func(in *catalog.SpecInput) { in.ProvisionKey = "cpu" }},
		{"bad unit", func(in *catalog.SpecInput) { in.Unit = "tb" }},
		{"step below 1", func(in *catalog.SpecInput) { in.StepQty = 0 }},
		{"max below min", func(in *catalog.SpecInput) { in.MinQty = 50; in.MaxQty = 10 }},
		{"default below min", func(in *catalog.SpecInput) { in.MinQty = 20; in.DefaultQty = 5 }},
		{"default above max", func(in *catalog.SpecInput) { in.MaxQty = 10; in.DefaultQty = 50 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := validSpecInput()
			tt.mut(&in)
			_, err := f.svc.CreateSpec(context.Background(), 1, 1, in)
			assertCode(t, err, apperr.CodeValidation)
		})
	}
}

func TestUpdateSpec_Success(t *testing.T) {
	f := newFixture()
	f.specs.GetSpecByIDFn = func(_ context.Context, id int64) (*domain.ProductSpec, error) {
		return &domain.ProductSpec{ID: id, ProductID: 3}, nil
	}
	var saved *domain.ProductSpec
	f.specs.UpdateSpecFn = func(_ context.Context, s *domain.ProductSpec) error { saved = s; return nil }

	in := validSpecInput()
	in.DefaultQty = 20
	sp, err := f.svc.UpdateSpec(context.Background(), 1, 9, in)
	require.NoError(t, err)
	assert.Equal(t, int64(9), sp.ID)
	assert.Equal(t, int64(3), saved.ProductID)
	assert.Equal(t, int64(20), saved.DefaultQty)
}

func TestDeleteSpec(t *testing.T) {
	f := newFixture()
	called := false
	f.specs.DeleteSpecFn = func(_ context.Context, id int64) error { called = true; return nil }
	require.NoError(t, f.svc.DeleteSpec(context.Background(), 1, 9))
	assert.True(t, called)
	assert.Len(t, f.audit.Entries, 1)
}

func TestUpsertSpecPricing(t *testing.T) {
	f := newFixture()
	var saved *domain.ProductSpecPricing
	f.specs.UpsertSpecPriceFn = func(_ context.Context, p *domain.ProductSpecPricing) error { saved = p; return nil }

	p, err := f.svc.UpsertSpecPricing(context.Background(), 1, 9, catalog.SpecPricingInput{
		Cycle: "monthly", UnitPrice: 5000, UnlimitedPrice: 200000,
	})
	require.NoError(t, err)
	assert.Equal(t, "IDR", p.Currency)
	assert.Equal(t, int64(5000), saved.UnitPrice)
	assert.Equal(t, int64(200000), saved.UnlimitedPrice)
}

func TestUpsertSpecPricing_NonIDR(t *testing.T) {
	f := newFixture()
	_, err := f.svc.UpsertSpecPricing(context.Background(), 1, 9, catalog.SpecPricingInput{
		Cycle: "monthly", UnitPrice: 5000, Currency: "USD",
	})
	assertCode(t, err, apperr.CodeValidation)
}

func TestDeleteSpecPricing_BadCycle(t *testing.T) {
	f := newFixture()
	err := f.svc.DeleteSpecPricing(context.Background(), 1, 9, "weekly")
	assertCode(t, err, apperr.CodeValidation)
}

func TestListProductSpecs(t *testing.T) {
	f := newFixture()
	f.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) { return cpanelProduct(id), nil }
	f.specs.ListSpecsFn = func(_ context.Context, productID int64) ([]domain.ProductSpec, error) {
		return []domain.ProductSpec{{ID: 1, Key: "disk"}}, nil
	}
	f.specs.ListSpecPricingFn = func(_ context.Context, specID int64) ([]domain.ProductSpecPricing, error) {
		return []domain.ProductSpecPricing{{ID: 1, SpecID: specID, Cycle: domain.CycleMonthly, UnitPrice: 5000}}, nil
	}
	rows, err := f.svc.ListProductSpecs(context.Background(), 1)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "disk", rows[0].Spec.Key)
	require.Len(t, rows[0].Pricing, 1)
	assert.Equal(t, int64(5000), rows[0].Pricing[0].UnitPrice)
}

func TestPublicProductBySlug_IncludesSpecs(t *testing.T) {
	f := newFixture()
	f.products.GetBySlugFn = func(_ context.Context, slug string) (*domain.Product, error) {
		return &domain.Product{ID: 1, Slug: slug, GroupID: 2, Module: domain.ModuleCpanel, Configurable: true}, nil
	}
	f.products.GetGroupByIDFn = func(_ context.Context, id int64) (*domain.ProductGroup, error) {
		return &domain.ProductGroup{ID: id}, nil
	}
	f.specs.ListSpecsFn = func(_ context.Context, productID int64) ([]domain.ProductSpec, error) {
		return []domain.ProductSpec{{ID: 1, Key: "disk", ProvisionKey: domain.SpecDisk, Unit: domain.UnitGB, MinQty: 5, MaxQty: 100, StepQty: 5, DefaultQty: 10}}, nil
	}
	f.specs.ListSpecPricingFn = func(_ context.Context, specID int64) ([]domain.ProductSpecPricing, error) {
		return []domain.ProductSpecPricing{{Cycle: domain.CycleMonthly, UnitPrice: 5000, Currency: "IDR"}}, nil
	}
	detail, err := f.svc.PublicProductBySlug(context.Background(), "custom-hosting")
	require.NoError(t, err)
	assert.True(t, detail.Configurable)
	require.Len(t, detail.Specs, 1)
	assert.Equal(t, "disk", detail.Specs[0].Key)
	require.Len(t, detail.Specs[0].Pricing, 1)
	assert.Equal(t, int64(5000), detail.Specs[0].Pricing[0].UnitPrice)
}
