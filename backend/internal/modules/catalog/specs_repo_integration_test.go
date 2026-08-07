//go:build integration

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

func TestRepoSpecLifecycle(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := catalog.NewRepo(d)
	suffix := tag()
	g := newGroupFixture(t, repo, d, suffix)
	p := newProductFixture(t, repo, d, g.ID, suffix, func(pr *domain.Product) {
		pr.Module = domain.ModuleCpanel
		pr.PackageName = "base"
		pr.Configurable = true
	})

	// products.configurable round-trips.
	got, err := repo.GetByID(ctx, p.ID)
	require.NoError(t, err)
	assert.True(t, got.Configurable)

	// Create a spec.
	spec := &domain.ProductSpec{
		ProductID: p.ID, Key: "disk", Label: "Disk", ProvisionKey: domain.SpecDisk,
		Unit: domain.UnitGB, IncludedQty: 5, MinQty: 5, MaxQty: 100, StepQty: 5, DefaultQty: 10,
	}
	require.NoError(t, repo.CreateSpec(ctx, spec))
	require.NotZero(t, spec.ID)

	// Duplicate provision_key on the same product -> CONFLICT.
	dup := &domain.ProductSpec{ProductID: p.ID, Key: "disk2", ProvisionKey: domain.SpecDisk, Unit: domain.UnitGB, StepQty: 1}
	err = repo.CreateSpec(ctx, dup)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)

	// List + Get.
	list, err := repo.ListSpecs(ctx, p.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "disk", list[0].Key)

	byID, err := repo.GetSpecByID(ctx, spec.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.SpecDisk, byID.ProvisionKey)

	// Update.
	spec.MaxQty = 200
	spec.Label = "Disk space"
	require.NoError(t, repo.UpdateSpec(ctx, spec))
	byID, _ = repo.GetSpecByID(ctx, spec.ID)
	assert.Equal(t, int64(200), byID.MaxQty)
	assert.Equal(t, "Disk space", byID.Label)

	// Spec pricing upsert + get + list.
	pr := &domain.ProductSpecPricing{SpecID: spec.ID, Cycle: domain.CycleMonthly, UnitPrice: 5000, UnlimitedPrice: 200000, Currency: "IDR"}
	require.NoError(t, repo.UpsertSpecPricing(ctx, pr))
	pr.UnitPrice = 6000
	require.NoError(t, repo.UpsertSpecPricing(ctx, pr)) // update path
	gotP, err := repo.GetSpecPricing(ctx, spec.ID, domain.CycleMonthly)
	require.NoError(t, err)
	assert.Equal(t, int64(6000), gotP.UnitPrice)

	pricing, err := repo.ListSpecPricing(ctx, spec.ID)
	require.NoError(t, err)
	require.Len(t, pricing, 1)

	// Non-IDR rejected.
	err = repo.UpsertSpecPricing(ctx, &domain.ProductSpecPricing{SpecID: spec.ID, Cycle: domain.CycleAnnually, Currency: "USD"})
	assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)

	// Delete pricing, then spec.
	require.NoError(t, repo.DeleteSpecPricing(ctx, spec.ID, domain.CycleMonthly))
	_, err = repo.GetSpecPricing(ctx, spec.ID, domain.CycleMonthly)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	require.NoError(t, repo.DeleteSpec(ctx, spec.ID))
	_, err = repo.GetSpecByID(ctx, spec.ID)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestRepoSpecCascadeOnProductDelete(t *testing.T) {
	ctx := context.Background()
	d := testDB(t)
	repo := catalog.NewRepo(d)
	suffix := tag()
	g := newGroupFixture(t, repo, d, suffix)
	p := newProductFixture(t, repo, d, g.ID, suffix, func(pr *domain.Product) {
		pr.Module = domain.ModuleDirectAdmin
		pr.PackageName = "base"
		pr.Configurable = true
	})
	spec := &domain.ProductSpec{ProductID: p.ID, Key: "bw", ProvisionKey: domain.SpecBandwidth, Unit: domain.UnitGB, StepQty: 1}
	require.NoError(t, repo.CreateSpec(ctx, spec))
	require.NoError(t, repo.UpsertSpecPricing(ctx, &domain.ProductSpecPricing{SpecID: spec.ID, Cycle: domain.CycleMonthly, UnitPrice: 100, Currency: "IDR"}))

	// Hard-delete the product row; specs + spec pricing cascade.
	_, err := d.Pool().Exec(ctx, `DELETE FROM products WHERE id = $1`, p.ID)
	require.NoError(t, err)

	list, err := repo.ListSpecs(ctx, p.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}
