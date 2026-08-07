package orders_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/orders"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// configurableEnv makes product 10 a configurable cPanel product with a disk
// (gb, included 5, 5..100 step 5, 1000/GB) and a bandwidth spec that allows
// unlimited (flat 200000).
func configurableEnv() *testEnv {
	e := newFixture()
	e.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) {
		if id != 10 {
			return nil, apperr.NotFound("product")
		}
		return &domain.Product{
			ID: 10, Name: "Custom Hosting", Type: domain.ProductSharedHosting,
			Module: domain.ModuleCpanel, AutoSetup: domain.SetupOnPayment, Configurable: true,
		}, nil
	}
	e.products.ListSpecsFn = func(_ context.Context, productID int64) ([]domain.ProductSpec, error) {
		return []domain.ProductSpec{
			{ID: 1, ProductID: 10, Key: "disk", ProvisionKey: domain.SpecDisk, Unit: domain.UnitGB,
				IncludedQty: 5, MinQty: 5, MaxQty: 100, StepQty: 5, DefaultQty: 10},
			{ID: 2, ProductID: 10, Key: "bandwidth", ProvisionKey: domain.SpecBandwidth, Unit: domain.UnitGB,
				IncludedQty: 100, MinQty: 100, MaxQty: 1000, StepQty: 100, DefaultQty: 100, AllowUnlimited: true},
		}, nil
	}
	e.products.GetSpecPricingFn = func(_ context.Context, specID int64, cycle domain.BillingCycle) (*domain.ProductSpecPricing, error) {
		if cycle != domain.CycleMonthly {
			return nil, apperr.NotFound("spec pricing")
		}
		switch specID {
		case 1:
			return &domain.ProductSpecPricing{SpecID: 1, Cycle: cycle, UnitPrice: 1000, Currency: "IDR"}, nil
		case 2:
			return &domain.ProductSpecPricing{SpecID: 2, Cycle: cycle, UnitPrice: 500, UnlimitedPrice: 200000, Currency: "IDR"}, nil
		}
		return nil, apperr.NotFound("spec pricing")
	}
	return e
}

func customItem() orders.OrderItemRequest {
	return orders.OrderItemRequest{ItemType: "product", ProductID: 10, Cycle: "monthly", Domain: "budi.example.com"}
}

func TestSpecPricing_ChargeableAboveIncluded(t *testing.T) {
	e := configurableEnv()
	svc := orders.New(e.deps)

	it := customItem()
	it.Specs = []orders.SpecSelectionRequest{
		{Key: "disk", Qty: 20},       // (20-5)*1000 = 15000
		{Key: "bandwidth", Qty: 300}, // (300-100)*500 = 100000
	}
	res, err := svc.CreateOrder(context.Background(), 7, "ip",
		orders.CreateOrderRequest{Items: []orders.OrderItemRequest{it}})
	require.NoError(t, err)
	// base 50000 + 15000 + 100000
	assert.Equal(t, int64(165000), res.Items[0].UnitPrice)

	var opts struct {
		Specs []orders.SpecSelection `json:"specs"`
	}
	require.NoError(t, json.Unmarshal(res.Items[0].Options, &opts))
	require.Len(t, opts.Specs, 2)
	assert.Equal(t, "disk", opts.Specs[0].Key)
	assert.Equal(t, int64(15000), opts.Specs[0].Amount)
}

func TestSpecPricing_DefaultsWhenOmitted(t *testing.T) {
	e := configurableEnv()
	svc := orders.New(e.deps)

	// No specs sent -> defaults (disk 10 -> (10-5)*1000=5000; bandwidth 100 -> 0).
	res, err := svc.CreateOrder(context.Background(), 7, "ip",
		orders.CreateOrderRequest{Items: []orders.OrderItemRequest{customItem()}})
	require.NoError(t, err)
	assert.Equal(t, int64(55000), res.Items[0].UnitPrice)
}

func TestSpecPricing_Unlimited(t *testing.T) {
	e := configurableEnv()
	svc := orders.New(e.deps)

	it := customItem()
	it.Specs = []orders.SpecSelectionRequest{{Key: "bandwidth", Unlimited: true}}
	res, err := svc.CreateOrder(context.Background(), 7, "ip",
		orders.CreateOrderRequest{Items: []orders.OrderItemRequest{it}})
	require.NoError(t, err)
	// base 50000 + disk default (5000) + bandwidth unlimited flat 200000
	assert.Equal(t, int64(255000), res.Items[0].UnitPrice)

	var opts struct {
		Specs []orders.SpecSelection `json:"specs"`
	}
	require.NoError(t, json.Unmarshal(res.Items[0].Options, &opts))
	for _, sp := range opts.Specs {
		if sp.Key == "bandwidth" {
			assert.True(t, sp.Unlimited)
			assert.Equal(t, domain.UnlimitedQty, sp.Qty)
		}
	}
}

func TestSpecPricing_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		spec orders.SpecSelectionRequest
	}{
		{"below min", orders.SpecSelectionRequest{Key: "disk", Qty: 1}},
		{"above max", orders.SpecSelectionRequest{Key: "disk", Qty: 500}},
		{"bad step", orders.SpecSelectionRequest{Key: "disk", Qty: 12}},
		{"unlimited not allowed", orders.SpecSelectionRequest{Key: "disk", Unlimited: true}},
		{"unknown key", orders.SpecSelectionRequest{Key: "cpu", Qty: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := configurableEnv()
			svc := orders.New(e.deps)
			it := customItem()
			it.Specs = []orders.SpecSelectionRequest{tt.spec}
			_, err := svc.CreateOrder(context.Background(), 7, "ip",
				orders.CreateOrderRequest{Items: []orders.OrderItemRequest{it}})
			assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
		})
	}
}

func TestSpecPricing_SpecsOnNonConfigurable(t *testing.T) {
	e := newFixture() // product 10 is not configurable in the default fixture
	svc := orders.New(e.deps)
	it := customItem()
	it.Specs = []orders.SpecSelectionRequest{{Key: "disk", Qty: 20}}
	_, err := svc.CreateOrder(context.Background(), 7, "ip",
		orders.CreateOrderRequest{Items: []orders.OrderItemRequest{it}})
	assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
}

func TestActivate_CopiesSpecsToPanelMeta(t *testing.T) {
	e := newFixture()
	specs := []orders.SpecSelection{
		{Key: "disk", ProvisionKey: "disk", Unit: "gb", Qty: 20, Amount: 15000},
	}
	optsJSON, _ := json.Marshal(map[string]any{"specs": specs})
	item := hostingItem(200, 10)
	item.Options = optsJSON
	activationEnv(e, []domain.OrderItem{item}, map[int64]*domain.Product{
		10: {ID: 10, Name: "Custom", Module: domain.ModuleCpanel, AutoSetup: domain.SetupOnPayment, Configurable: true},
	})
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.created, 1)
	require.NotEmpty(t, e.created[0].PanelMeta)

	var meta struct {
		ChosenSpecs []orders.SpecSelection `json:"chosen_specs"`
	}
	require.NoError(t, json.Unmarshal(e.created[0].PanelMeta, &meta))
	require.Len(t, meta.ChosenSpecs, 1)
	assert.Equal(t, "disk", meta.ChosenSpecs[0].Key)
	assert.Equal(t, int64(20), meta.ChosenSpecs[0].Qty)
}
