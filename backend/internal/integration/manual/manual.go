// Package manual implements ports.PaymentGateway as a client-selectable
// "Bank Transfer" checkout option that makes no external HTTP calls at all:
// CreateTransaction simply echoes back the admin-configured bank account
// instructions, and the resulting transaction stays pending until an admin
// manually confirms receipt (payments.Service.ConfirmManualTransaction).
//
// Unlike duitku/rdash there is nothing to poll or verify externally, so
// CheckTransaction/VerifyCallbackSignature are unsupported/false - neither
// is ever invoked in practice (no webhook route or reconciliation entry
// exists for this gateway).
package manual

import (
	"context"
	"errors"

	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

const (
	providerName = "manual"
	// MethodCode is this gateway's one payment channel - matches
	// payments.ManualMethodBankTransfer.
	MethodCode = "bank_transfer"
	methodName = "Bank Transfer"
)

var errNotSupported = errors.New("manual gateway does not support external status checks")

// Config is the live, admin-configurable manual-gateway settings - resolved
// fresh per call via the resolve function passed to New (same live/
// no-restart pattern as duitku/rdash). No encryption needed: bank account
// numbers here are display instructions, not credentials.
type Config struct {
	Enabled      bool
	Accounts     []ports.BankAccount
	Instructions string
}

// Client is the manual bank-transfer payment gateway adapter.
type Client struct {
	resolve func(ctx context.Context) (Config, error)
}

// Compile-time interface assertion.
var _ ports.PaymentGateway = (*Client)(nil)

// New builds a Client. resolve is consulted fresh on every call (no
// caching) so an admin's saved bank accounts/instructions take effect with
// no restart. A nil resolve (or a resolve that errors) behaves as
// completely unconfigured (Enabled: false).
func New(resolve func(ctx context.Context) (Config, error)) *Client {
	return &Client{resolve: resolve}
}

func (c *Client) config(ctx context.Context) Config {
	if c.resolve == nil {
		return Config{}
	}
	cfg, err := c.resolve(ctx)
	if err != nil {
		return Config{}
	}
	return cfg
}

// GetPaymentMethods returns the one "Bank Transfer" channel when enabled
// with at least one account configured, else an empty list - never an
// error, so payments.Service silently omits this optional gateway from the
// aggregate whenever it's unconfigured or disabled.
func (c *Client) GetPaymentMethods(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
	cfg := c.config(ctx)
	if !cfg.Enabled || len(cfg.Accounts) == 0 {
		return nil, nil
	}
	return []ports.PaymentMethod{{Code: MethodCode, Name: methodName}}, nil
}

// CreateTransaction makes no HTTP call - it returns the configured bank
// accounts + instructions for the client to transfer to, with
// req.MerchantOrderID surfaced as the reference the client should quote in
// their transfer note.
func (c *Client) CreateTransaction(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
	cfg := c.config(ctx)
	if !cfg.Enabled || len(cfg.Accounts) == 0 {
		return nil, apperr.Conflict("bank transfer is not currently available")
	}
	return &ports.CreateTxResult{
		Reference:    req.MerchantOrderID,
		Amount:       req.Amount,
		BankAccounts: cfg.Accounts,
		Note:         cfg.Instructions,
	}, nil
}

// CheckTransaction is not supported: a manual bank-transfer payment has no
// external system to poll - never called in practice, since no
// reconciliation entry exists for this gateway (payments.ReconcilePending
// only ever scans domain.GatewayDuitku transactions).
func (c *Client) CheckTransaction(ctx context.Context, merchantOrderID string) (*ports.TxStatus, error) {
	return nil, apperr.External(providerName, errNotSupported)
}

// VerifyCallbackSignature always returns false: no webhook route is
// registered for the manual gateway, so this is never called in practice.
func (c *Client) VerifyCallbackSignature(p ports.CallbackPayload) bool {
	return false
}
