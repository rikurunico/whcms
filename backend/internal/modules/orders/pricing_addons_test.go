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

func TestCreateOrderDomainTLDMatrixPricing(t *testing.T) {
	e := newFixture()
	e.tldPricing.GetByTLDFn = func(_ context.Context, tld string) (*domain.TLDPricing, error) {
		assert.Equal(t, "com", tld)
		return &domain.TLDPricing{
			TLD: "com", Active: true, MinYears: 1, MaxYears: 5,
			RegisterPrices: map[string]int64{"1": 150000, "2": 280000, "3": 400000},
			RenewPrices:    map[string]int64{"1": 160000},
			TransferPrice:  120000,
		}, nil
	}
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items: []orders.OrderItemRequest{{ItemType: "domain_register", Domain: "example.com", DomainYears: 3}},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(400000), res.Items[0].UnitPrice, "3-year matrix total, not perYear*years")

	var opts struct {
		DomainRenewPerYear int64 `json:"domain_renew_per_year"`
	}
	require.NoError(t, json.Unmarshal(res.Items[0].Options, &opts))
	assert.Equal(t, int64(160000), opts.DomainRenewPerYear, "renewal rate frozen from the TLD's year-1 renew price")
}

func TestCreateOrderDomainTLDTransferFlatPrice(t *testing.T) {
	e := newFixture()
	e.tldPricing.GetByTLDFn = func(_ context.Context, tld string) (*domain.TLDPricing, error) {
		return &domain.TLDPricing{TLD: "com", Active: true, MinYears: 1, MaxYears: 10, TransferPrice: 120000}, nil
	}
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items: []orders.OrderItemRequest{{ItemType: "domain_transfer", Domain: "example.com", EPPCode: "epp"}},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(120000), res.Items[0].UnitPrice)
}

func TestCreateOrderDomainTLDYearsOutOfBounds(t *testing.T) {
	e := newFixture()
	e.tldPricing.GetByTLDFn = func(_ context.Context, tld string) (*domain.TLDPricing, error) {
		return &domain.TLDPricing{
			TLD: "com", Active: true, MinYears: 1, MaxYears: 2,
			RegisterPrices: map[string]int64{"1": 150000, "2": 280000},
		}, nil
	}
	svc := orders.New(e.deps)

	_, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items: []orders.OrderItemRequest{{ItemType: "domain_register", Domain: "example.com", DomainYears: 5}},
	})
	require.Error(t, err)
	ae := apperr.From(err)
	assert.Equal(t, apperr.CodeValidation, ae.Code)
	require.NotEmpty(t, ae.Details)
	assert.Equal(t, "items[0].domain_years", ae.Details[0].Field)
}

func TestCreateOrderDomainTLDMissingYearPrice(t *testing.T) {
	e := newFixture()
	e.tldPricing.GetByTLDFn = func(_ context.Context, tld string) (*domain.TLDPricing, error) {
		return &domain.TLDPricing{
			TLD: "com", Active: true, MinYears: 1, MaxYears: 10,
			RegisterPrices: map[string]int64{"1": 150000},
		}, nil
	}
	svc := orders.New(e.deps)

	_, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items: []orders.OrderItemRequest{{ItemType: "domain_register", Domain: "example.com", DomainYears: 4}},
	})
	require.Error(t, err)
	assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
}

func TestCreateOrderDomainInactiveTLDFallsBackToRegistrarQuote(t *testing.T) {
	e := newFixture()
	e.tldPricing.GetByTLDFn = func(_ context.Context, tld string) (*domain.TLDPricing, error) {
		return &domain.TLDPricing{TLD: "com", Active: false, RegisterPrices: map[string]int64{"1": 999999}}, nil
	}
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items: []orders.OrderItemRequest{{ItemType: "domain_register", Domain: "example.com"}},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(150000), res.Items[0].UnitPrice, "inactive TLD row is ignored, registrar quote used")
}

func TestCreateOrderDomainPremiumOverride(t *testing.T) {
	e := newFixture()
	e.premiumPricing.GetByNameFn = func(_ context.Context, name string) (*domain.PremiumDomainPricing, error) {
		assert.Equal(t, "shop.com", name)
		return &domain.PremiumDomainPricing{DomainName: name, RegisterPrice: 9000000, RenewPrice: 8000000, TransferPrice: 5000000}, nil
	}
	// Even with a TLD row configured, the exact premium override must win.
	e.tldPricing.GetByTLDFn = func(_ context.Context, tld string) (*domain.TLDPricing, error) {
		return &domain.TLDPricing{TLD: "com", Active: true, RegisterPrices: map[string]int64{"1": 150000}}, nil
	}
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items: []orders.OrderItemRequest{{ItemType: "domain_register", Domain: "shop.com", DomainYears: 2}},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(9000000*2), res.Items[0].UnitPrice)
}

func TestCreateOrderDomainPremiumLengthTier(t *testing.T) {
	e := newFixture()
	e.premiumLengthPricing.GetByTLDAndLengthFn = func(_ context.Context, tld string, charLength int) (*domain.PremiumLengthPricing, error) {
		assert.Equal(t, "id", tld)
		assert.Equal(t, 2, charLength)
		return &domain.PremiumLengthPricing{TLD: "id", CharLength: 2, Price: 485000000}, nil
	}
	// Even with a TLD row configured, the length-tier match must win.
	e.tldPricing.GetByTLDFn = func(_ context.Context, tld string) (*domain.TLDPricing, error) {
		return &domain.TLDPricing{TLD: "id", Active: true, RegisterPrices: map[string]int64{"1": 150000}}, nil
	}
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items: []orders.OrderItemRequest{{ItemType: "domain_register", Domain: "ab.id"}},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(485000000), res.Items[0].UnitPrice)
}

func TestCreateOrderDomainWithAddons(t *testing.T) {
	e := newFixture()
	e.tldPricing.GetByTLDFn = func(_ context.Context, tld string) (*domain.TLDPricing, error) {
		return &domain.TLDPricing{
			TLD: "com", Active: true, MinYears: 1, MaxYears: 10,
			RegisterPrices: map[string]int64{"1": 150000, "2": 280000},
			RenewPrices:    map[string]int64{"1": 160000},
		}, nil
	}
	e.domainAddons.ListActiveFn = func(_ context.Context) ([]domain.DomainAddon, error) {
		return []domain.DomainAddon{
			{ID: 1, Key: "id_protection", Price: 20000, Active: true},
			{ID: 2, Key: "dns_management", Price: 15000, Active: true},
		}, nil
	}
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items: []orders.OrderItemRequest{{
			ItemType: "domain_register", Domain: "example.com", DomainYears: 2,
			DomainAddons: []string{"id_protection", "dns_management"},
		}},
	})
	require.NoError(t, err)
	// 280000 (2-year matrix total) + (20000 + 15000 addons) × 2 years.
	assert.Equal(t, int64(280000+(20000+15000)*2), res.Items[0].UnitPrice)

	var opts struct {
		DomainRenewPerYear int64    `json:"domain_renew_per_year"`
		DomainAddons       []string `json:"domain_addons"`
	}
	require.NoError(t, json.Unmarshal(res.Items[0].Options, &opts))
	assert.Equal(t, int64(160000+20000+15000), opts.DomainRenewPerYear)
	assert.ElementsMatch(t, []string{"id_protection", "dns_management"}, opts.DomainAddons)
}

func TestCreateOrderDomainUnknownAddonRejected(t *testing.T) {
	e := newFixture()
	e.domainAddons.ListActiveFn = func(_ context.Context) ([]domain.DomainAddon, error) {
		return []domain.DomainAddon{{ID: 1, Key: "id_protection", Price: 20000, Active: true}}, nil
	}
	svc := orders.New(e.deps)

	_, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items: []orders.OrderItemRequest{{
			ItemType: "domain_register", Domain: "example.com",
			DomainAddons: []string{"dns_management"},
		}},
	})
	require.Error(t, err)
	ae := apperr.From(err)
	assert.Equal(t, apperr.CodeValidation, ae.Code)
	require.NotEmpty(t, ae.Details)
	assert.Equal(t, "items[0].domain_addons", ae.Details[0].Field)
}
