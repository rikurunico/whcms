package manual

import (
	"context"
	"errors"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enabledConfig() Config {
	return Config{
		Enabled: true,
		Accounts: []ports.BankAccount{
			{BankName: "BCA", AccountNumber: "1234567890", AccountHolder: "WHCMS Hosting"},
		},
		Instructions: "Include the reference in your transfer note.",
	}
}

func TestGetPaymentMethodsEnabledWithAccounts(t *testing.T) {
	c := New(func(ctx context.Context) (Config, error) { return enabledConfig(), nil })
	methods, err := c.GetPaymentMethods(context.Background(), 150000)
	require.NoError(t, err)
	require.Len(t, methods, 1)
	assert.Equal(t, MethodCode, methods[0].Code)
	assert.Equal(t, methodName, methods[0].Name)
}

func TestGetPaymentMethodsDisabledReturnsEmpty(t *testing.T) {
	cfg := enabledConfig()
	cfg.Enabled = false
	c := New(func(ctx context.Context) (Config, error) { return cfg, nil })
	methods, err := c.GetPaymentMethods(context.Background(), 150000)
	require.NoError(t, err)
	assert.Empty(t, methods)
}

func TestGetPaymentMethodsNoAccountsReturnsEmpty(t *testing.T) {
	c := New(func(ctx context.Context) (Config, error) { return Config{Enabled: true}, nil })
	methods, err := c.GetPaymentMethods(context.Background(), 150000)
	require.NoError(t, err)
	assert.Empty(t, methods)
}

func TestGetPaymentMethodsResolverErrorTreatedAsUnconfigured(t *testing.T) {
	c := New(func(ctx context.Context) (Config, error) { return Config{}, errors.New("db down") })
	methods, err := c.GetPaymentMethods(context.Background(), 150000)
	require.NoError(t, err, "a resolver error must not fail the call, just behave unconfigured")
	assert.Empty(t, methods)
}

func TestGetPaymentMethodsNilResolverTreatedAsUnconfigured(t *testing.T) {
	c := New(nil)
	methods, err := c.GetPaymentMethods(context.Background(), 150000)
	require.NoError(t, err)
	assert.Empty(t, methods)
}

func TestCreateTransactionReturnsConfiguredAccountsNoHTTPCall(t *testing.T) {
	c := New(func(ctx context.Context) (Config, error) { return enabledConfig(), nil })
	res, err := c.CreateTransaction(context.Background(), ports.CreateTxRequest{
		MerchantOrderID: "INV-202607-000001-01",
		Amount:          150000,
	})
	require.NoError(t, err)
	assert.Equal(t, "INV-202607-000001-01", res.Reference)
	assert.Equal(t, int64(150000), res.Amount)
	assert.Empty(t, res.PaymentURL)
	assert.Empty(t, res.VANumber)
	assert.Empty(t, res.QRString)
	require.Len(t, res.BankAccounts, 1)
	assert.Equal(t, "BCA", res.BankAccounts[0].BankName)
	assert.Equal(t, "Include the reference in your transfer note.", res.Note)
}

func TestCreateTransactionDisabledRejected(t *testing.T) {
	cfg := enabledConfig()
	cfg.Enabled = false
	c := New(func(ctx context.Context) (Config, error) { return cfg, nil })
	_, err := c.CreateTransaction(context.Background(), ports.CreateTxRequest{MerchantOrderID: "X", Amount: 1})
	require.Error(t, err)
}

func TestCreateTransactionNoAccountsRejected(t *testing.T) {
	c := New(func(ctx context.Context) (Config, error) { return Config{Enabled: true}, nil })
	_, err := c.CreateTransaction(context.Background(), ports.CreateTxRequest{MerchantOrderID: "X", Amount: 1})
	require.Error(t, err)
}

func TestCheckTransactionNotSupported(t *testing.T) {
	c := New(func(ctx context.Context) (Config, error) { return enabledConfig(), nil })
	_, err := c.CheckTransaction(context.Background(), "INV-202607-000001-01")
	require.Error(t, err)
}

func TestVerifyCallbackSignatureAlwaysFalse(t *testing.T) {
	c := New(func(ctx context.Context) (Config, error) { return enabledConfig(), nil })
	assert.False(t, c.VerifyCallbackSignature(ports.CallbackPayload{Signature: "anything"}))
}
