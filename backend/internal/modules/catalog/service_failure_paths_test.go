package catalog_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/catalog"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errBoom = errors.New("boom")

// TestServiceErrorPropagation drives every repo failure branch: unexpected
// repo errors surface as INTERNAL apperr.
func TestServiceErrorPropagation(t *testing.T) {
	ctx := context.Background()

	group := func(f *fixtures) {
		f.products.GetGroupByIDFn = func(ctx context.Context, id int64) (*domain.ProductGroup, error) {
			return &domain.ProductGroup{ID: id}, nil
		}
	}
	product := func(f *fixtures) {
		f.products.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
			return &domain.Product{ID: id, GroupID: 1, Slug: "p", Type: domain.ProductOther,
				Module: domain.ModuleNone, AutoSetup: domain.SetupOnPayment}, nil
		}
	}

	tests := []struct {
		name string
		prep func(f *fixtures)
		call func(f *fixtures) error
	}{
		{"PublicCatalog groups", func(f *fixtures) {
			f.products.ListGroupsFn = func(context.Context, bool) ([]domain.ProductGroup, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicCatalog(ctx); return err }},
		{"PublicCatalog products", func(f *fixtures) {
			f.products.ListVisibleProductsFn = func(context.Context) ([]domain.Product, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicCatalog(ctx); return err }},
		{"PublicCatalog pricing", func(f *fixtures) {
			f.products.ListPricingByProductsFn = func(context.Context, []int64) (map[int64][]domain.ProductPricing, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicCatalog(ctx); return err }},
		{"PublicGroups", func(f *fixtures) {
			f.products.ListGroupsFn = func(context.Context, bool) ([]domain.ProductGroup, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicGroups(ctx); return err }},
		{"PublicProductBySlug get", func(f *fixtures) {
			f.products.GetBySlugFn = func(context.Context, string) (*domain.Product, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicProductBySlug(ctx, "s"); return err }},
		{"PublicProductBySlug group", func(f *fixtures) {
			f.products.GetBySlugFn = func(ctx context.Context, s string) (*domain.Product, error) {
				return &domain.Product{ID: 1, GroupID: 1, Slug: s}, nil
			}
			f.products.GetGroupByIDFn = func(context.Context, int64) (*domain.ProductGroup, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicProductBySlug(ctx, "s"); return err }},
		{"PublicProductBySlug pricing", func(f *fixtures) {
			f.products.GetBySlugFn = func(ctx context.Context, s string) (*domain.Product, error) {
				return &domain.Product{ID: 1, GroupID: 1, Slug: s}, nil
			}
			group(f)
			f.products.ListPricingFn = func(context.Context, int64) ([]domain.ProductPricing, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicProductBySlug(ctx, "s"); return err }},
		{"PublicProductBySlug options", func(f *fixtures) {
			f.products.GetBySlugFn = func(ctx context.Context, s string) (*domain.Product, error) {
				return &domain.Product{ID: 1, GroupID: 1, Slug: s}, nil
			}
			group(f)
			f.products.ListOptionGroupsFn = func(context.Context) ([]domain.ConfigurableOptionGroup, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.PublicProductBySlug(ctx, "s"); return err }},
		{"GetGroup", func(f *fixtures) {
			f.products.GetGroupByIDFn = func(context.Context, int64) (*domain.ProductGroup, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.GetGroup(ctx, 1); return err }},
		{"CreateGroup", func(f *fixtures) {
			f.products.CreateGroupFn = func(context.Context, *domain.ProductGroup) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.CreateGroup(ctx, 1, catalog.GroupInput{Name: "Hosting"})
			return err
		}},
		{"UpdateGroup get", func(f *fixtures) {
			f.products.GetGroupByIDFn = func(context.Context, int64) (*domain.ProductGroup, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateGroup(ctx, 1, 2, catalog.GroupUpdateInput{})
			return err
		}},
		{"UpdateGroup save", func(f *fixtures) {
			group(f)
			f.products.UpdateGroupFn = func(context.Context, *domain.ProductGroup) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateGroup(ctx, 1, 2, catalog.GroupUpdateInput{Sort: ptr(2)})
			return err
		}},
		{"DeleteGroup count", func(f *fixtures) {
			group(f)
			f.products.CountProductsInGroupFn = func(context.Context, int64) (int64, error) { return 0, errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteGroup(ctx, 1, 2) }},
		{"DeleteGroup delete", func(f *fixtures) {
			group(f)
			f.products.SoftDeleteGroupFn = func(context.Context, int64) error { return errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteGroup(ctx, 1, 2) }},
		{"GetProduct", func(f *fixtures) {
			f.products.GetByIDFn = func(context.Context, int64) (*domain.Product, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.GetProduct(ctx, 1); return err }},
		{"CreateProduct save", func(f *fixtures) {
			group(f)
			f.products.CreateFn = func(context.Context, *domain.Product) error { return errBoom }
		}, func(f *fixtures) error { _, err := f.svc.CreateProduct(ctx, 1, productInput(nil)); return err }},
		{"UpdateProduct get", func(f *fixtures) {
			f.products.GetByIDFn = func(context.Context, int64) (*domain.Product, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateProduct(ctx, 1, 2, catalog.ProductUpdateInput{})
			return err
		}},
		{"UpdateProduct save", func(f *fixtures) {
			product(f)
			f.products.UpdateFn = func(context.Context, *domain.Product) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateProduct(ctx, 1, 2, catalog.ProductUpdateInput{Sort: ptr(3)})
			return err
		}},
		{"DeleteProduct", func(f *fixtures) {
			product(f)
			f.products.SoftDeleteFn = func(context.Context, int64) error { return errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteProduct(ctx, 1, 2) }},
		{"ListPricing", func(f *fixtures) {
			product(f)
			f.products.ListPricingFn = func(context.Context, int64) ([]domain.ProductPricing, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.ListPricing(ctx, 1); return err }},
		{"UpsertPricing", func(f *fixtures) {
			product(f)
			f.products.UpsertPricingFn = func(context.Context, *domain.ProductPricing) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpsertPricing(ctx, 1, 2, catalog.PricingInput{Cycle: "monthly", Price: 1})
			return err
		}},
		{"DeletePricing", func(f *fixtures) {
			f.products.DeletePricingFn = func(context.Context, int64, domain.BillingCycle) error { return errBoom }
		}, func(f *fixtures) error { return f.svc.DeletePricing(ctx, 1, 2, "monthly") }},
		{"OptionTree options", func(f *fixtures) {
			f.products.ListOptionGroupsFn = func(context.Context) ([]domain.ConfigurableOptionGroup, error) {
				return []domain.ConfigurableOptionGroup{{ID: 1}}, nil
			}
			f.products.ListOptionsFn = func(context.Context, int64) ([]domain.ConfigurableOption, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.OptionTree(ctx); return err }},
		{"OptionTree values", func(f *fixtures) {
			f.products.ListOptionGroupsFn = func(context.Context) ([]domain.ConfigurableOptionGroup, error) {
				return []domain.ConfigurableOptionGroup{{ID: 1}}, nil
			}
			f.products.ListOptionsFn = func(context.Context, int64) ([]domain.ConfigurableOption, error) {
				return []domain.ConfigurableOption{{ID: 2}}, nil
			}
			f.products.ListOptionValuesFn = func(context.Context, int64) ([]domain.ConfigurableOptionValue, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.OptionTree(ctx); return err }},
		{"CreateOptionGroup", func(f *fixtures) {
			f.options.CreateOptionGroupFn = func(context.Context, *domain.ConfigurableOptionGroup) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.CreateOptionGroup(ctx, 1, catalog.OptionGroupInput{Name: "Extras"})
			return err
		}},
		{"UpdateOptionGroup get", func(f *fixtures) {
			f.options.GetOptionGroupByIDFn = func(context.Context, int64) (*domain.ConfigurableOptionGroup, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateOptionGroup(ctx, 1, 2, catalog.OptionGroupInput{Name: "Extras"})
			return err
		}},
		{"UpdateOptionGroup save", func(f *fixtures) {
			f.options.UpdateOptionGroupFn = func(context.Context, *domain.ConfigurableOptionGroup) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateOptionGroup(ctx, 1, 2, catalog.OptionGroupInput{Name: "Extras"})
			return err
		}},
		{"DeleteOptionGroup", func(f *fixtures) {
			f.options.DeleteOptionGroupFn = func(context.Context, int64) error { return errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteOptionGroup(ctx, 1, 2) }},
		{"CreateOption group err", func(f *fixtures) {
			f.options.GetOptionGroupByIDFn = func(context.Context, int64) (*domain.ConfigurableOptionGroup, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.CreateOption(ctx, 1, 2, catalog.OptionInput{Name: "RAM"})
			return err
		}},
		{"CreateOption save", func(f *fixtures) {
			f.options.CreateOptionFn = func(context.Context, *domain.ConfigurableOption) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.CreateOption(ctx, 1, 2, catalog.OptionInput{Name: "RAM"})
			return err
		}},
		{"UpdateOption get", func(f *fixtures) {
			f.options.GetOptionByIDFn = func(context.Context, int64) (*domain.ConfigurableOption, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateOption(ctx, 1, 2, catalog.OptionInput{Name: "RAM"})
			return err
		}},
		{"UpdateOption save", func(f *fixtures) {
			f.options.UpdateOptionFn = func(context.Context, *domain.ConfigurableOption) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateOption(ctx, 1, 2, catalog.OptionInput{Name: "RAM"})
			return err
		}},
		{"DeleteOption", func(f *fixtures) {
			f.options.DeleteOptionFn = func(context.Context, int64) error { return errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteOption(ctx, 1, 2) }},
		{"CreateOptionValue option err", func(f *fixtures) {
			f.options.GetOptionByIDFn = func(context.Context, int64) (*domain.ConfigurableOption, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.CreateOptionValue(ctx, 1, 2, catalog.OptionValueInput{Name: "2GB"})
			return err
		}},
		{"CreateOptionValue save", func(f *fixtures) {
			f.options.CreateOptionValueFn = func(context.Context, *domain.ConfigurableOptionValue) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.CreateOptionValue(ctx, 1, 2, catalog.OptionValueInput{Name: "2GB"})
			return err
		}},
		{"UpdateOptionValue get", func(f *fixtures) {
			f.options.GetOptionValueByIDFn = func(context.Context, int64) (*domain.ConfigurableOptionValue, error) { return nil, errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateOptionValue(ctx, 1, 2, catalog.OptionValueInput{Name: "2GB"})
			return err
		}},
		{"UpdateOptionValue save", func(f *fixtures) {
			f.options.UpdateOptionValueFn = func(context.Context, *domain.ConfigurableOptionValue) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.UpdateOptionValue(ctx, 1, 2, catalog.OptionValueInput{Name: "2GB"})
			return err
		}},
		{"DeleteOptionValue", func(f *fixtures) {
			f.options.DeleteOptionValueFn = func(context.Context, int64) error { return errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteOptionValue(ctx, 1, 2) }},
		{"GetCoupon", func(f *fixtures) {
			f.coupons.GetByIDFn = func(context.Context, int64) (*domain.Coupon, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, err := f.svc.GetCoupon(ctx, 1); return err }},
		{"CreateCoupon", func(f *fixtures) {
			f.coupons.CreateFn = func(context.Context, *domain.Coupon) error { return errBoom }
		}, func(f *fixtures) error {
			_, err := f.svc.CreateCoupon(ctx, 1, catalog.CouponInput{Code: "AA", Type: "fixed", Value: 1})
			return err
		}},
		{"UpdateCoupon save", func(f *fixtures) {
			f.coupons.GetByIDFn = func(ctx context.Context, id int64) (*domain.Coupon, error) {
				return &domain.Coupon{ID: id, Type: domain.CouponFixed, Value: 1}, nil
			}
			f.coupons.UpdateFn = func(context.Context, *domain.Coupon) error { return errBoom }
		}, func(f *fixtures) error {
			exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
			_, err := f.svc.UpdateCoupon(ctx, 1, 2, catalog.CouponUpdateInput{
				Type: ptr("percentage"), Value: ptr(int64(10)), MaxUses: ptr(3),
				Recurring: ptr(true), ExpiresAt: &exp,
			})
			return err
		}},
		{"DeleteCoupon", func(f *fixtures) {
			f.coupons.GetByIDFn = func(ctx context.Context, id int64) (*domain.Coupon, error) {
				return &domain.Coupon{ID: id}, nil
			}
			f.coupons.DeleteFn = func(context.Context, int64) error { return errBoom }
		}, func(f *fixtures) error { return f.svc.DeleteCoupon(ctx, 1, 2) }},
		{"ValidateCoupon repo", func(f *fixtures) {
			f.coupons.GetByCodeFn = func(context.Context, string) (*domain.Coupon, error) { return nil, errBoom }
		}, func(f *fixtures) error { _, _, err := f.svc.ValidateCoupon(ctx, "X1", nil, 1); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture()
			tt.prep(f)
			assertCode(t, tt.call(f), apperr.CodeInternal)
		})
	}
}

// A (nil, nil) row from the repo (the mocks' default) must surface as
// NOT_FOUND, never as a nil dereference.
func TestServiceNilRowsAreNotFound(t *testing.T) {
	ctx := context.Background()
	f := newFixture()

	_, err := f.svc.GetProduct(ctx, 1)
	assertCode(t, err, apperr.CodeNotFound)
	_, err = f.svc.GetGroup(ctx, 1)
	assertCode(t, err, apperr.CodeNotFound)
	_, err = f.svc.GetCoupon(ctx, 1)
	assertCode(t, err, apperr.CodeNotFound)
	_, err = f.svc.PublicProductBySlug(ctx, "s")
	assertCode(t, err, apperr.CodeNotFound)
	_, _, err = f.svc.ValidateCoupon(ctx, "X1", nil, 1)
	assertCode(t, err, apperr.CodeNotFound)

	fo := newFixture()
	fo.options.GetOptionGroupByIDFn = func(context.Context, int64) (*domain.ConfigurableOptionGroup, error) { return nil, nil }
	_, err = fo.svc.UpdateOptionGroup(ctx, 1, 2, catalog.OptionGroupInput{Name: "Extras"})
	assertCode(t, err, apperr.CodeNotFound)
	_, err = fo.svc.CreateOption(ctx, 1, 2, catalog.OptionInput{Name: "RAM"})
	assertCode(t, err, apperr.CodeNotFound)

	fo.options.GetOptionByIDFn = func(context.Context, int64) (*domain.ConfigurableOption, error) { return nil, nil }
	_, err = fo.svc.UpdateOption(ctx, 1, 2, catalog.OptionInput{Name: "RAM"})
	assertCode(t, err, apperr.CodeNotFound)
	_, err = fo.svc.CreateOptionValue(ctx, 1, 2, catalog.OptionValueInput{Name: "2GB"})
	assertCode(t, err, apperr.CodeNotFound)

	fo.options.GetOptionValueByIDFn = func(context.Context, int64) (*domain.ConfigurableOptionValue, error) { return nil, nil }
	_, err = fo.svc.UpdateOptionValue(ctx, 1, 2, catalog.OptionValueInput{Name: "2GB"})
	assertCode(t, err, apperr.CodeNotFound)
}

// Every mutation rejects invalid input with VALIDATION before touching
// the repo.
func TestServiceInputValidation(t *testing.T) {
	ctx := context.Background()
	f := newFixture()

	_, err := f.svc.UpdateGroup(ctx, 1, 2, catalog.GroupUpdateInput{Name: ptr("x")})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.UpdateProduct(ctx, 1, 2, catalog.ProductUpdateInput{Type: ptr("vps")})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.CreateOptionGroup(ctx, 1, catalog.OptionGroupInput{Name: "x"})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.UpdateOptionGroup(ctx, 1, 2, catalog.OptionGroupInput{Name: "x"})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.CreateOption(ctx, 1, 2, catalog.OptionInput{})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.UpdateOption(ctx, 1, 2, catalog.OptionInput{})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.CreateOptionValue(ctx, 1, 2, catalog.OptionValueInput{})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.UpdateOptionValue(ctx, 1, 2, catalog.OptionValueInput{})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.UpdateOptionValue(ctx, 1, 2, catalog.OptionValueInput{Name: "2GB", PriceDeltas: map[string]int64{"weekly": 1}})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.CreateCoupon(ctx, 1, catalog.CouponInput{Type: "fixed", Value: 1})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.UpdateCoupon(ctx, 1, 2, catalog.CouponUpdateInput{Type: ptr("bogus")})
	assertCode(t, err, apperr.CodeValidation)
	_, err = f.svc.CreateProduct(ctx, 1, catalog.ProductInput{Name: "OK Name"})
	assertCode(t, err, apperr.CodeValidation)

	// Slug cannot be derived from a symbol-only name.
	_, err = f.svc.CreateGroup(ctx, 1, catalog.GroupInput{Name: "!!"})
	assertCode(t, err, apperr.CodeValidation)
	f.products.GetGroupByIDFn = func(ctx context.Context, id int64) (*domain.ProductGroup, error) {
		return &domain.ProductGroup{ID: id}, nil
	}
	_, err = f.svc.CreateProduct(ctx, 1, productInput(func(in *catalog.ProductInput) { in.Name = "!!" }))
	assertCode(t, err, apperr.CodeValidation)
}

// A patch carrying every updatable field applies all of them in one call.
func TestUpdateProductAllFields(t *testing.T) {
	f := newFixture()
	f.products.GetGroupByIDFn = func(ctx context.Context, id int64) (*domain.ProductGroup, error) {
		return &domain.ProductGroup{ID: id}, nil
	}
	f.products.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return &domain.Product{ID: id, GroupID: 1, Name: "Old", Slug: "old",
			Type: domain.ProductOther, Module: domain.ModuleNone, AutoSetup: domain.SetupOnPayment}, nil
	}
	var saved *domain.Product
	f.products.UpdateFn = func(ctx context.Context, pr *domain.Product) error {
		saved = pr
		return nil
	}
	_, err := f.svc.UpdateProduct(context.Background(), 1, 10, catalog.ProductUpdateInput{
		GroupID:              ptr(int64(2)),
		Name:                 ptr("New"),
		Slug:                 ptr("new"),
		Description:          ptr("desc"),
		Type:                 ptr("shared_hosting"),
		Module:               ptr("cpanel"),
		ServerGroupID:        ptr(int64(4)),
		PackageName:          ptr("pkg"),
		AutoSetup:            ptr("manual"),
		StockEnabled:         ptr(true),
		StockQty:             ptr(5),
		Hidden:               ptr(true),
		Sort:                 ptr(9),
		WelcomeEmailTemplate: ptr("welcome"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), saved.GroupID)
	assert.Equal(t, "new", saved.Slug)
	assert.Equal(t, domain.ModuleCpanel, saved.Module)
	assert.Equal(t, domain.SetupManual, saved.AutoSetup)
	require.NotNil(t, saved.ServerGroupID)
	assert.Equal(t, int64(4), *saved.ServerGroupID)
	assert.Equal(t, 5, saved.StockQty)
	assert.Equal(t, "welcome", saved.WelcomeEmailTemplate)
}

// Moving a product to an unknown group is rejected: the target group is
// re-checked on update.
func TestUpdateProductUnknownGroup(t *testing.T) {
	f := newFixture()
	f.products.GetByIDFn = func(ctx context.Context, id int64) (*domain.Product, error) {
		return &domain.Product{ID: id, GroupID: 1, Slug: "p", Type: domain.ProductOther,
			Module: domain.ModuleNone, AutoSetup: domain.SetupOnPayment}, nil
	}
	f.products.GetGroupByIDFn = func(ctx context.Context, id int64) (*domain.ProductGroup, error) {
		return nil, apperr.NotFound("product group")
	}
	_, err := f.svc.UpdateProduct(context.Background(), 1, 10, catalog.ProductUpdateInput{GroupID: ptr(int64(2))})
	assertCode(t, err, apperr.CodeValidation)
}

// TestListParamsPassthrough covers the plain list delegates.
func TestListParamsPassthrough(t *testing.T) {
	f := newFixture()
	f.products.ListFn = func(ctx context.Context, p ports.ListParams) ([]domain.Product, int64, error) {
		return []domain.Product{{ID: 1}}, 1, nil
	}
	rows, total, err := f.svc.ListProducts(context.Background(), ports.ListParams{})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, int64(1), total)

	f.coupons.ListFn = func(ctx context.Context, p ports.ListParams) ([]domain.Coupon, int64, error) {
		return nil, 0, errBoom
	}
	_, _, err = f.svc.ListCoupons(context.Background(), ports.ListParams{})
	assertCode(t, err, apperr.CodeInternal)

	f.products.ListGroupsFn = func(context.Context, bool) ([]domain.ProductGroup, error) { return nil, errBoom }
	_, err = f.svc.ListGroups(context.Background(), true)
	assertCode(t, err, apperr.CodeInternal)
}

// Delete operations propagate a failing existence lookup.
func TestDeleteGetErrorBranches(t *testing.T) {
	ctx := context.Background()
	f := newFixture()
	f.products.GetByIDFn = func(context.Context, int64) (*domain.Product, error) { return nil, errBoom }
	assertCode(t, f.svc.DeleteProduct(ctx, 1, 2), apperr.CodeInternal)

	f.coupons.GetByIDFn = func(context.Context, int64) (*domain.Coupon, error) { return nil, errBoom }
	assertCode(t, f.svc.DeleteCoupon(ctx, 1, 2), apperr.CodeInternal)
}
