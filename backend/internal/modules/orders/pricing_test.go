package orders_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/jobs"
	"github.com/tsdlamongan/whcms/backend/internal/modules/orders"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidDomainName(t *testing.T) {
	valid := []string{"example.com", "sub.example.co.id", "xn--nxasmq6b.com", "a-b.io", "123.dev"}
	for _, d := range valid {
		assert.True(t, orders.ValidDomainName(d), d)
	}
	invalid := []string{"", "nodots", ".com", "example.", "-x.com", "x-.com",
		"under_score.com", "spa ce.com", "x.c", "example..com"}
	for _, d := range invalid {
		assert.False(t, orders.ValidDomainName(d), d)
	}
	// Length limits: 63-char label ok, 64 not.
	long := ""
	for i := 0; i < 63; i++ {
		long += "a"
	}
	assert.True(t, orders.ValidDomainName(long+".com"))
	assert.False(t, orders.ValidDomainName(long+"a.com"))
}

func TestCouponAppliesToWrappedShape(t *testing.T) {
	e := newFixture()
	withCoupon(e, couponFixture(func(c *domain.Coupon) {
		c.AppliesTo = json.RawMessage(`{"product_ids":[10]}`)
	}))
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items:      []orders.OrderItemRequest{productItem()},
		CouponCode: "SAVE10",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5000), res.Order.Discount, "wrapped applies_to shape recognised")
}

func TestCouponAppliesToMalformedMeansAll(t *testing.T) {
	e := newFixture()
	withCoupon(e, couponFixture(func(c *domain.Coupon) {
		c.AppliesTo = json.RawMessage(`"garbage"`)
	}))
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "ip", orders.CreateOrderRequest{
		Items:      []orders.OrderItemRequest{productItem()},
		CouponCode: "SAVE10",
	})
	require.NoError(t, err)
	assert.Equal(t, int64(5000), res.Order.Discount, "unparseable applies_to falls back to all products")
}

func TestOptionDeltaMalformedIgnored(t *testing.T) {
	e := newFixture()
	e.products.ListOptionValuesFn = func(_ context.Context, optionID int64) ([]domain.ConfigurableOptionValue, error) {
		return []domain.ConfigurableOptionValue{
			{ID: 40, OptionID: optionID, Name: "NVMe", PriceDeltas: json.RawMessage(`not-json`)},
		}, nil
	}
	svc := orders.New(e.deps)

	it := productItem()
	it.Options = []orders.OptionSelectionRequest{{OptionID: 4, ValueID: 40}}
	res, err := svc.CreateOrder(context.Background(), 7, "ip",
		orders.CreateOrderRequest{Items: []orders.OrderItemRequest{it}})
	require.NoError(t, err)
	assert.Equal(t, int64(50000), res.Items[0].UnitPrice, "malformed deltas contribute 0")
}

func TestOptionDeltaEmptyDeltas(t *testing.T) {
	e := newFixture()
	e.products.ListOptionValuesFn = func(_ context.Context, optionID int64) ([]domain.ConfigurableOptionValue, error) {
		return []domain.ConfigurableOptionValue{{ID: 40, OptionID: optionID, Name: "Free addon"}}, nil
	}
	svc := orders.New(e.deps)

	it := productItem()
	it.Options = []orders.OptionSelectionRequest{{OptionID: 4, ValueID: 40}}
	res, err := svc.CreateOrder(context.Background(), 7, "ip",
		orders.CreateOrderRequest{Items: []orders.OrderItemRequest{it}})
	require.NoError(t, err)
	assert.Equal(t, int64(50000), res.Items[0].UnitPrice)
}

func TestRecurringCouponFixedClampsAtZero(t *testing.T) {
	e := newFixture()
	cid := int64(5)
	e.orderRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Order, error) {
		return &domain.Order{ID: id, ClientID: 7, Status: domain.OrderPending, CouponID: &cid}, nil
	}
	e.orderRepo.GetItemsFn = func(_ context.Context, _ int64) ([]domain.OrderItem, error) {
		return []domain.OrderItem{hostingItem(200, 10)}, nil
	}
	e.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) {
		return cpanelProduct(), nil
	}
	withCoupon(e, couponFixture(func(c *domain.Coupon) {
		c.Recurring = true
		c.Type = domain.CouponFixed
		c.Value = 999999 // more than the unit price
	}))
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.created, 1)
	assert.Equal(t, int64(0), e.created[0].RecurringAmount, "fixed recurring discount clamps at 0")
}

func TestRecurringCouponFixedSubtracts(t *testing.T) {
	e := newFixture()
	cid := int64(5)
	e.orderRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Order, error) {
		return &domain.Order{ID: id, ClientID: 7, Status: domain.OrderPending, CouponID: &cid}, nil
	}
	e.orderRepo.GetItemsFn = func(_ context.Context, _ int64) ([]domain.OrderItem, error) {
		return []domain.OrderItem{hostingItem(200, 10)}, nil
	}
	e.products.GetByIDFn = func(_ context.Context, id int64) (*domain.Product, error) {
		return cpanelProduct(), nil
	}
	withCoupon(e, couponFixture(func(c *domain.Coupon) {
		c.Recurring = true
		c.Type = domain.CouponFixed
		c.Value = 20000
	}))
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.created, 1)
	assert.Equal(t, int64(30000), e.created[0].RecurringAmount)
}

func TestFraudSkipsBadEmailShapes(t *testing.T) {
	e := newFixture()
	e.users.GetByIDFn = func(_ context.Context, id int64) (*domain.User, error) {
		return &domain.User{ID: id, Email: "no-at-sign", EmailVerifiedAt: verifiedAt(testNow)}, nil
	}
	e.settings.GetJSONFn = func(_ context.Context, key string, out any) error {
		if key == "fraud.email_domain_blacklist" {
			*(out.(*[]string)) = []string{"spam.io"}
		}
		return nil
	}
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "ip",
		orders.CreateOrderRequest{Items: []orders.OrderItemRequest{productItem()}})
	require.NoError(t, err)
	assert.Equal(t, domain.OrderPending, res.Order.Status, "unparseable email cannot match blacklist")
}

func TestFraudDisabledDailyLimit(t *testing.T) {
	e := newFixture()
	e.settings.GetIntFn = func(_ context.Context, key string, def int) (int, error) {
		if key == "fraud.max_orders_per_day" {
			return 0, nil // 0 disables the check
		}
		return def, nil
	}
	counted := false
	e.store.CountByClientSinceFn = func(_ context.Context, _ int64, _ time.Time) (int64, error) {
		counted = true
		return 999, nil
	}
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "ip",
		orders.CreateOrderRequest{Items: []orders.OrderItemRequest{productItem()}})
	require.NoError(t, err)
	assert.Equal(t, domain.OrderPending, res.Order.Status)
	assert.False(t, counted, "limit 0 skips the count query")
}

func TestActivateDomainMissingRegistrar(t *testing.T) {
	e := newFixture()
	item := domain.OrderItem{
		ID: 201, OrderID: 100, ItemType: domain.ItemDomainRegister,
		Domain: "example.com", Cycle: domain.CycleAnnually, UnitPrice: 150000,
	}
	activationEnv(e, []domain.OrderItem{item}, nil)
	e.regRepo.GetByNameFn = func(_ context.Context, _ string) (*domain.Registrar, error) {
		return nil, apperr.NotFound("registrar")
	}
	svc := orders.New(e.deps)

	err := svc.ActivateOrder(context.Background(), 100)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestActivateDomainDefaultYears(t *testing.T) {
	e := newFixture()
	item := domain.OrderItem{
		ID: 201, OrderID: 100, ItemType: domain.ItemDomainRegister,
		Domain: "example.com", Cycle: domain.CycleAnnually, UnitPrice: 150000,
		// no options JSON at all
	}
	activationEnv(e, []domain.OrderItem{item}, nil)
	svc := orders.New(e.deps)

	require.NoError(t, svc.ActivateOrder(context.Background(), 100))
	require.Len(t, e.domCreated, 1)
	assert.Equal(t, int64(150000), e.domCreated[0].RecurringAmount, "1-year default keeps the unit price")
	require.Len(t, e.enqueuer.Tasks, 1)
	assert.Equal(t, jobs.DomainRegisterPayload{DomainID: e.domCreated[0].ID, Years: 1},
		e.enqueuer.Tasks[0].Payload)
}

func TestAcceptPropagatesActivationConflict(t *testing.T) {
	e := newFixture()
	e.orderRepo.GetByIDFn = func(_ context.Context, id int64) (*domain.Order, error) {
		return &domain.Order{ID: id, ClientID: 7, Status: domain.OrderCancelled}, nil
	}
	svc := orders.New(e.deps)

	_, err := svc.Accept(context.Background(), 1, 100)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	assert.Empty(t, e.audit.Entries, "no audit on failed accept")
}

func TestCancelWithoutInvoice(t *testing.T) {
	e := newFixture()
	pendingOrderEnv(e)
	e.store.InvoiceIDForOrderFn = func(_ context.Context, _ int64) (int64, error) { return 0, nil }
	svc := orders.New(e.deps)

	order, err := svc.Cancel(context.Background(), 1, 100)
	require.NoError(t, err)
	assert.Equal(t, domain.OrderCancelled, order.Status)
	assert.Empty(t, e.invStatus, "no invoice to cancel")
}
