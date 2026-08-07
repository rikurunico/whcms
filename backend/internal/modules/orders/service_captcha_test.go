package orders_test

import (
	"context"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/modules/orders"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCaptcha implements orders.CaptchaGuard.
type fakeCaptcha struct {
	err       error
	calls     int
	lastToken string
	lastIP    string
}

func (f *fakeCaptcha) Verify(_ context.Context, token, ip string) error {
	f.calls++
	f.lastToken, f.lastIP = token, ip
	return f.err
}

func TestCreateOrderCaptchaRejected(t *testing.T) {
	e := newFixture()
	cap := &fakeCaptcha{err: apperr.Validation("captcha verification failed")}
	e.deps.Captcha = cap
	svc := orders.New(e.deps)

	_, err := svc.CreateOrder(context.Background(), 7, "10.0.0.1", orders.CreateOrderRequest{
		Items:        []orders.OrderItemRequest{productItem()},
		CaptchaToken: "bad",
	})

	require.Error(t, err)
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, apperr.CodeValidation, ae.Code)
	assert.Equal(t, 1, cap.calls)
	assert.Equal(t, "bad", cap.lastToken)
	assert.Equal(t, "10.0.0.1", cap.lastIP)
	assert.Empty(t, e.decremented, "no stock touched when captcha fails")
}

func TestCreateOrderCaptchaPassthrough(t *testing.T) {
	e := newFixture()
	cap := &fakeCaptcha{}
	e.deps.Captcha = cap
	svc := orders.New(e.deps)

	res, err := svc.CreateOrder(context.Background(), 7, "10.0.0.1", orders.CreateOrderRequest{
		Items:        []orders.OrderItemRequest{productItem()},
		CaptchaToken: "ok",
	})
	require.NoError(t, err)
	require.NotNil(t, res.Invoice)
	assert.Equal(t, 1, cap.calls)
	assert.Equal(t, "ok", cap.lastToken)
	assert.Equal(t, "10.0.0.1", cap.lastIP)
}
