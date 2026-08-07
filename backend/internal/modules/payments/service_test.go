package payments

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testNow = time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)

// deduction records one CreditDeducter call.
type deduction struct {
	ClientID  int64
	Amount    int64
	Reason    string
	InvoiceID int64
}

// fakeCredits is a recording CreditDeducter.
type fakeCredits struct {
	calls []deduction
	err   error
}

func (f *fakeCredits) DeductCredit(ctx context.Context, clientID, amount int64, reason string, relatedInvoiceID int64) error {
	if f.err != nil {
		return f.err
	}
	f.calls = append(f.calls, deduction{clientID, amount, reason, relatedInvoiceID})
	return nil
}

// fixture is a stateful in-memory backend for the service under test. The
// invoice and transactions behave like real rows: UpdateStatus flips the
// stored invoice, transaction upserts persist, so repeated calls observe
// prior effects (idempotency tests depend on this).
//
// mu guards txs/nextTxID/attempts so the concurrent-PayInvoice regression
// test (TestPayInvoiceConcurrentAttemptsGetDistinctMerchantOrderIDs) can run
// two goroutines against one fixture under -race. attempts models the real
// counters-table row backing TransactionRepo.NextAttempt: a value that is
// reserved atomically and independently of whether/when the corresponding
// transaction row is ever inserted - NOT derived from len(txs), since that
// would reintroduce the very race being tested for.
type fixture struct {
	t *testing.T

	svc *Service

	mu       sync.Mutex
	invoice  *domain.Invoice
	items    []domain.InvoiceItem
	txs      []domain.Transaction
	nextTxID int64
	attempts map[int64]int64

	processPaid    int
	processPaidErr error
	credits        *fakeCredits
	gateway        *mocks.MockPaymentGateway
	intlog         *mocks.MockIntegrationLogger
	audit          *mocks.MockAuditLogger
	cache          *mocks.MockCache
	settings       *mocks.MockSettingsRepo

	checkCalls int
}

func copyTx(t domain.Transaction) *domain.Transaction {
	c := t
	if t.MerchantOrderID != nil {
		mo := *t.MerchantOrderID
		c.MerchantOrderID = &mo
	}
	return &c
}

func newFixture(t *testing.T, inv *domain.Invoice) *fixture {
	t.Helper()
	h := &fixture{t: t, invoice: inv, nextTxID: 100, attempts: map[int64]int64{}}
	h.credits = &fakeCredits{}
	h.intlog = &mocks.MockIntegrationLogger{}
	h.audit = &mocks.MockAuditLogger{}
	h.cache = &mocks.MockCache{}
	h.settings = &mocks.MockSettingsRepo{}
	h.gateway = &mocks.MockPaymentGateway{
		VerifyCallbackSignatureFn: func(p ports.CallbackPayload) bool { return true },
	}

	invoices := &mocks.MockInvoiceRepo{
		GetByIDFn: func(ctx context.Context, id int64) (*domain.Invoice, error) {
			if h.invoice != nil && h.invoice.ID == id {
				c := *h.invoice
				return &c, nil
			}
			return nil, apperr.NotFound("invoice")
		},
		GetItemsFn: func(ctx context.Context, invoiceID int64) ([]domain.InvoiceItem, error) {
			return h.items, nil
		},
		UpdateStatusFn: func(ctx context.Context, id int64, status domain.InvoiceStatus, paidAt *time.Time) error {
			if h.invoice == nil || h.invoice.ID != id {
				return apperr.NotFound("invoice")
			}
			h.invoice.Status = status
			h.invoice.PaidAt = paidAt
			return nil
		},
	}
	invoices.GetByIDForUpdateFn = invoices.GetByIDFn

	transactions := &mocks.MockTransactionRepo{
		CreateFn: func(ctx context.Context, tr *domain.Transaction) error {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.nextTxID++
			tr.ID = h.nextTxID
			h.txs = append(h.txs, *copyTx(*tr))
			return nil
		},
		GetByIDFn: func(ctx context.Context, id int64) (*domain.Transaction, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			for _, tr := range h.txs {
				if tr.ID == id {
					return copyTx(tr), nil
				}
			}
			return nil, apperr.NotFound("transaction")
		},
		GetByMerchantOrderIDFn: func(ctx context.Context, mo string) (*domain.Transaction, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			for _, tr := range h.txs {
				if tr.MerchantOrderID != nil && *tr.MerchantOrderID == mo {
					return copyTx(tr), nil
				}
			}
			return nil, apperr.NotFound("transaction")
		},
		UpdateFn: func(ctx context.Context, tr *domain.Transaction) error {
			h.mu.Lock()
			defer h.mu.Unlock()
			for i := range h.txs {
				if h.txs[i].ID == tr.ID {
					h.txs[i] = *copyTx(*tr)
					return nil
				}
			}
			return apperr.NotFound("transaction")
		},
		ListByInvoiceFn: func(ctx context.Context, invoiceID int64) ([]domain.Transaction, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			var out []domain.Transaction
			for _, tr := range h.txs {
				if tr.InvoiceID == invoiceID {
					out = append(out, tr)
				}
			}
			return out, nil
		},
		ListPendingFn: func(ctx context.Context, gw domain.Gateway, olderThan time.Time) ([]domain.Transaction, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			var out []domain.Transaction
			for _, tr := range h.txs {
				if tr.Gateway == gw && tr.Status == domain.TxPending {
					out = append(out, tr)
				}
			}
			return out, nil
		},
		// NextAttemptFn models the real counters-table row: a persistent,
		// atomically-incremented value scoped to invoiceID, independent of
		// how many transaction rows currently exist (mirrors
		// TransactionRepo.NextAttempt / Repo.NextAttempt in repo.go).
		NextAttemptFn: func(ctx context.Context, invoiceID int64) (int64, error) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.attempts[invoiceID]++
			return h.attempts[invoiceID], nil
		},
	}

	processor := &mocks.MockPaidInvoiceProcessor{
		ProcessPaidFn: func(ctx context.Context, invoiceID int64) error {
			if h.processPaidErr != nil {
				return h.processPaidErr
			}
			h.processPaid++
			return nil
		},
	}

	clients := &mocks.MockClientRepo{
		GetByIDFn: func(ctx context.Context, id int64) (*domain.Client, error) {
			return &domain.Client{
				ID: id, UserID: 7, FirstName: "Budi", LastName: "Santoso",
				Phone: "0812345678", Currency: "IDR",
			}, nil
		},
	}
	users := &mocks.MockUserRepo{
		GetByIDFn: func(ctx context.Context, id int64) (*domain.User, error) {
			return &domain.User{ID: id, Email: "budi@example.com"}, nil
		},
	}

	h.svc = New(Deps{
		Tx:           &mocks.MockTxManager{},
		Invoices:     invoices,
		Transactions: transactions,
		Clients:      clients,
		Users:        users,
		Gateways: map[string]ports.PaymentGateway{
			string(domain.GatewayDuitku): h.gateway,
		},
		GatewayOrder:   []string{string(domain.GatewayDuitku)},
		DefaultGateway: string(domain.GatewayDuitku),
		Processor:      processor,
		Credits:        h.credits,
		Cache:          h.cache,
		Clock:          &mocks.MockClock{FixedTime: testNow},
		Audit:          h.audit,
		IntLog:         h.intlog,
		Settings:       h.settings,
		AppBaseURL:     "http://api.test",
		FrontendURL:    "http://front.test",
	})
	return h
}

// addPendingTx seeds a pending duitku transaction and returns it.
func (h *fixture) addPendingTx(merchantOrderID string, amount int64) domain.Transaction {
	h.nextTxID++
	mo := merchantOrderID
	tr := domain.Transaction{
		ID: h.nextTxID, InvoiceID: h.invoice.ID, Gateway: domain.GatewayDuitku,
		MethodCode: "VA", MerchantOrderID: &mo, Amount: amount, Status: domain.TxPending,
	}
	h.txs = append(h.txs, tr)
	return tr
}

func (h *fixture) tx(id int64) domain.Transaction {
	for _, tr := range h.txs {
		if tr.ID == id {
			return tr
		}
	}
	h.t.Fatalf("transaction %d not found", id)
	return domain.Transaction{}
}

func unpaidInvoice() *domain.Invoice {
	return &domain.Invoice{
		ID: 10, InvoiceNumber: "INV-202607-000001", ClientID: 5,
		Status: domain.InvoiceUnpaid, Subtotal: 150000, Total: 150000,
		Currency: "IDR", DueDate: testNow.AddDate(0, 0, 3),
	}
}

// ApplyPayment

func applyTx(amount int64) ports.ApplyTx {
	return ports.ApplyTx{
		Gateway: domain.GatewayDuitku, MethodCode: "VA",
		MerchantOrderID: "INV-202607-000001-01", GatewayReference: "REF-1",
		Amount: amount, PaidAt: testNow,
	}
}

func TestApplyPaymentMarksPaidAndProcessesOnce(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)

	require.NoError(t, h.svc.ApplyPayment(context.Background(), 10, applyTx(150000)))

	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	require.NotNil(t, h.invoice.PaidAt)
	assert.Equal(t, 1, h.processPaid, "billing.ProcessPaid dispatched in same tx")

	got := h.tx(tr.ID)
	assert.Equal(t, domain.TxSuccess, got.Status, "pending tx upgraded to success")
	assert.Equal(t, "REF-1", got.GatewayReference)
	require.NotNil(t, got.PaidAt)
	assert.Len(t, h.txs, 1, "no duplicate transaction created")
	assert.NotEmpty(t, h.audit.Entries, "invoice.paid audited")
}

func TestApplyPaymentAlreadyPaidIsNoop(t *testing.T) {
	inv := unpaidInvoice()
	inv.Status = domain.InvoicePaid
	h := newFixture(t, inv)

	require.NoError(t, h.svc.ApplyPayment(context.Background(), 10, applyTx(150000)))
	assert.Zero(t, h.processPaid, "no re-processing of a paid invoice")
	assert.Empty(t, h.txs, "no transaction written")
}

func TestApplyPaymentRefundedIsNoop(t *testing.T) {
	inv := unpaidInvoice()
	inv.Status = domain.InvoiceRefunded
	h := newFixture(t, inv)

	require.NoError(t, h.svc.ApplyPayment(context.Background(), 10, applyTx(150000)))
	assert.Zero(t, h.processPaid)
}

func TestApplyPaymentCancelledInvoiceConflict(t *testing.T) {
	inv := unpaidInvoice()
	inv.Status = domain.InvoiceCancelled
	h := newFixture(t, inv)

	err := h.svc.ApplyPayment(context.Background(), 10, applyTx(150000))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code, "state machine rejects cancelled->paid")
	assert.Zero(t, h.processPaid)
}

func TestApplyPaymentAmountShortRejected(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)

	err := h.svc.ApplyPayment(context.Background(), 10, applyTx(100000))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	assert.Equal(t, domain.InvoiceUnpaid, h.invoice.Status, "invoice untouched")
	assert.Zero(t, h.processPaid)
}

func TestApplyPaymentShortConsidersCreditApplied(t *testing.T) {
	inv := unpaidInvoice()
	inv.CreditApplied = 50000 // remaining = 100000
	h := newFixture(t, inv)

	require.NoError(t, h.svc.ApplyPayment(context.Background(), 10, applyTx(100000)))
	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	assert.Equal(t, 1, h.processPaid)
}

func TestApplyPaymentOverpayAcceptedAndLogged(t *testing.T) {
	h := newFixture(t, unpaidInvoice())

	require.NoError(t, h.svc.ApplyPayment(context.Background(), 10, applyTx(200000)))
	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	assert.Equal(t, 1, h.processPaid)
}

func TestApplyPaymentCreatesRowWithoutMerchantOrder(t *testing.T) {
	h := newFixture(t, unpaidInvoice())

	require.NoError(t, h.svc.ApplyPayment(context.Background(), 10, ports.ApplyTx{
		Gateway: domain.GatewayCredit, MethodCode: "credit", Amount: 150000, PaidAt: testNow,
	}))
	require.Len(t, h.txs, 1)
	got := h.txs[0]
	assert.Equal(t, domain.GatewayCredit, got.Gateway)
	assert.Equal(t, domain.TxSuccess, got.Status)
	assert.Nil(t, got.MerchantOrderID)
	assert.Equal(t, 1, h.processPaid)
}

func TestApplyPaymentMerchantOrderOfOtherInvoiceConflict(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	mo := "INV-202607-000001-01"
	h.txs = append(h.txs, domain.Transaction{
		ID: 999, InvoiceID: 42, Gateway: domain.GatewayDuitku,
		MerchantOrderID: &mo, Status: domain.TxPending,
	})

	err := h.svc.ApplyPayment(context.Background(), 10, applyTx(150000))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	assert.Zero(t, h.processPaid)
}

func TestApplyPaymentProcessorErrorRollsUp(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.processPaidErr = errors.New("boom")

	err := h.svc.ApplyPayment(context.Background(), 10, applyTx(150000))
	require.Error(t, err, "ProcessPaid failure aborts the transaction")
}

func TestApplyPaymentUnknownInvoiceNotFound(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	err := h.svc.ApplyPayment(context.Background(), 404, applyTx(150000))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

// Webhook

func callbackPayload(mo string) ports.CallbackPayload {
	return ports.CallbackPayload{
		MerchantCode: "DEMO", Amount: "150000", MerchantOrderID: mo,
		ResultCode: "00", Reference: "DREF-1", Signature: "sig",
	}
}

func TestCallbackDeliveredThreeTimesExactlyOneProcessPaid(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		h.checkCalls++
		return &ports.TxStatus{Reference: "DREF-1", Amount: 150000, StatusCode: ports.TxStatusSuccess}, nil
	}

	for i := 0; i < 3; i++ {
		require.NoError(t, h.svc.HandleCallback(context.Background(), callbackPayload("INV-202607-000001-01")),
			"re-delivery %d must be a 200 OK no-op", i+1)
	}

	assert.Equal(t, 1, h.processPaid, "exactly one ProcessPaid despite 3 deliveries")
	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	assert.Len(t, h.txs, 1, "single success transaction")
	assert.Equal(t, domain.TxSuccess, h.txs[0].Status)
}

// TestCallbackAppliesFeeFromCheckTransaction covers a real bug found
// 2026-07-27 against a live invoice (real Duitku sandbox, BRI VA): Duitku's
// transactionStatus response already reports the invoice-applicable amount
// directly in "amount" (confirmed via integration_logs: {"amount":"20000",
// "fee":"3000.00"} for a Rp20.000 invoice - NOT a gross 23000 requiring a
// subtraction). An earlier version of this code wrongly treated Fee as
// something to subtract FROM Amount (conflating this endpoint's semantics
// with the INQUIRY endpoint's genuinely gross-inclusive "amount", which BR's
// inquiry response separately does return as 23000 - see
// TestPayInvoiceGatewaySurchargesFeeOntoGrossAmount), which under-counted a
// fully-paid invoice as short by exactly the fee and rejected it with a
// CONFLICT, leaving a real payment stuck pending forever. Fee is purely
// informational here - Amount must be stored and compared as reported, with
// no arithmetic applied to it.
func TestCallbackAppliesFeeFromCheckTransaction(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{Reference: "DREF-1", Amount: 150000, Fee: 5000, StatusCode: ports.TxStatusSuccess}, nil
	}

	require.NoError(t, h.svc.HandleCallback(context.Background(), callbackPayload("INV-202607-000001-01")))

	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	require.Len(t, h.txs, 1)
	assert.Equal(t, int64(150000), h.txs[0].Amount, "amount is stored exactly as Duitku reported it, no fee arithmetic")
	assert.Equal(t, int64(5000), h.txs[0].Fee)
}

// noopMW is the minimal payments.Middlewares implementation for wiring a
// real *fiber.App around the real Service in tests - every handler on the
// webhook route is a no-op passthrough since /webhooks/duitku carries no
// session/role checks of its own.
type noopMW struct{}

func (noopMW) RequireAuth() fiber.Handler { return func(c fiber.Ctx) error { return c.Next() } }
func (noopMW) RequireRole(...string) fiber.Handler {
	return func(c fiber.Ctx) error { return c.Next() }
}
func (noopMW) RequirePermission(string) fiber.Handler {
	return func(c fiber.Ctx) error { return c.Next() }
}
func (noopMW) RequireClient() fiber.Handler { return func(c fiber.Ctx) error { return c.Next() } }
func (noopMW) RateLimit(string, int, time.Duration) fiber.Handler {
	return func(c fiber.Ctx) error { return c.Next() }
}

// TestWebhookHTTPReplayDeliveredThreeTimesExactlyOneProcessPaid closes the
// idempotency loop fully at the transport layer: unlike
// TestCallbackDeliveredThreeTimesExactlyOneProcessPaid (which calls
// Service.HandleCallback directly), this posts the IDENTICAL captured
// form-urlencoded body 3 times through a real *fiber.App wrapping the real
// Service via Handler.Webhook - proving a raw HTTP replay (the actual attack
// shape: a captured request replayed byte-for-byte) is a no-op end to end,
// not just at the Go-function-call level.
func TestWebhookHTTPReplayDeliveredThreeTimesExactlyOneProcessPaid(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{Reference: "DREF-1", Amount: 150000, StatusCode: ports.TxStatusSuccess}, nil
	}
	h.gateway.VerifyCallbackSignatureFn = func(p ports.CallbackPayload) bool {
		return p.MerchantCode == "DEMO" && p.MerchantOrderID == "INV-202607-000001-01"
	}

	app := fiber.New()
	NewHandler(h.svc, noopMW{}).RegisterRoutes(app.Group("/api/v1"))

	form := url.Values{
		"merchantCode":    {"DEMO"},
		"amount":          {"150000"},
		"merchantOrderId": {"INV-202607-000001-01"},
		"resultCode":      {"00"},
		"reference":       {"DREF-1"},
		"signature":       {"sig"},
	}
	body := form.Encode()

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("POST", "/api/v1/webhooks/duitku", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := app.Test(req)
		require.NoError(t, err)
		respBody, _ := io.ReadAll(resp.Body)
		assert.Equal(t, 200, resp.StatusCode, "replay %d must answer 200 OK", i+1)
		assert.Equal(t, "OK", string(respBody), "replay %d body", i+1)
	}

	assert.Equal(t, 1, h.processPaid, "exactly one ProcessPaid despite 3 raw HTTP replays")
	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	assert.Len(t, h.txs, 1, "single success transaction")
	assert.Equal(t, domain.TxSuccess, h.txs[0].Status)
}

func TestCallbackBadSignatureRejected(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.VerifyCallbackSignatureFn = func(p ports.CallbackPayload) bool { return false }
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		t.Fatal("CheckTransaction must not be called for a bad signature")
		return nil, nil
	}

	err := h.svc.HandleCallback(context.Background(), callbackPayload("INV-202607-000001-01"))
	require.ErrorIs(t, err, ErrBadSignature)
	assert.Zero(t, h.processPaid)
	assert.Equal(t, domain.InvoiceUnpaid, h.invoice.Status)
	require.Len(t, h.intlog.Calls, 1, "bad signature is integration-logged")
	assert.Equal(t, "duitku", h.intlog.Calls[0].Provider)
	assert.False(t, h.intlog.Calls[0].Success)
}

func TestCallbackUnknownMerchantOrderNotFound(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	err := h.svc.HandleCallback(context.Background(), callbackPayload("NOPE-01"))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)

	require.Len(t, h.intlog.Calls, 1, "even an unknown merchant order id is integration-logged")
	assert.False(t, h.intlog.Calls[0].Success)
	assert.Equal(t, 404, h.intlog.Calls[0].StatusCode)
}

// TestCallbackSuccessIsIntegrationLogged covers a real observability gap
// found 2026-07-27: only a BAD SIGNATURE webhook delivery was ever recorded
// via IntLog - a successfully processed callback left no trace at all, so an
// operator had no way to positively confirm "Duitku actually reached this
// endpoint" short of the transaction's status flipping. HandleCallback now
// logs every outcome via a defer, success included.
func TestCallbackSuccessIsIntegrationLogged(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{Reference: "DREF-1", Amount: 150000, StatusCode: ports.TxStatusSuccess}, nil
	}

	require.NoError(t, h.svc.HandleCallback(context.Background(), callbackPayload("INV-202607-000001-01")))

	require.Len(t, h.intlog.Calls, 1)
	assert.True(t, h.intlog.Calls[0].Success)
	assert.Equal(t, 200, h.intlog.Calls[0].StatusCode)
	assert.Equal(t, "duitku", h.intlog.Calls[0].Provider)
}

func TestCallbackFailedResultMarksTxFailed(t *testing.T) {
	// resultCode says failed, and CheckTransaction independently confirms
	// the failure (cancelled) - this is the true-failure case, unchanged
	// from before the double-verification fix.
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{StatusCode: ports.TxStatusCancelled}, nil
	}

	p := callbackPayload("INV-202607-000001-01")
	p.ResultCode = "01"
	require.NoError(t, h.svc.HandleCallback(context.Background(), p), "failed callback still answers OK")
	assert.Equal(t, domain.TxFailed, h.tx(tr.ID).Status)
	assert.Zero(t, h.processPaid)
	assert.Equal(t, domain.InvoiceUnpaid, h.invoice.Status)
}

// TestCallbackFailedResultButCheckPendingLeavesPending is the regression
// test for the double-verification fix: a callback with a valid signature
// and resultCode != "00" must NOT mark the transaction failed on the
// callback's word alone. Duitku's callback signature (MD5 of
// merchantCode+amount+merchantOrderId+apiKey) does not bind resultCode, so a
// forged/replayed resultCode cannot be trusted without CheckTransaction
// independently confirming a real failure. When CheckTransaction reports
// pending, the transaction must stay pending so the reconciliation cron
// (which only scans status='pending') can still follow up and self-heal if
// the payment later settles.
func TestCallbackFailedResultButCheckPendingLeavesPending(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{StatusCode: ports.TxStatusPending}, nil
	}

	p := callbackPayload("INV-202607-000001-01")
	p.ResultCode = "01"
	require.NoError(t, h.svc.HandleCallback(context.Background(), p), "ambiguous callback still answers OK")
	assert.Equal(t, domain.TxPending, h.tx(tr.ID).Status, "must stay pending for the reconciliation cron to follow up")
	assert.Zero(t, h.processPaid)
	assert.Equal(t, domain.InvoiceUnpaid, h.invoice.Status)
}

func TestCallbackWithoutCheckConfirmDoesNotPay(t *testing.T) {
	// resultCode says success but Check Transaction still reports pending -
	// the callback alone must never pay the invoice.
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{StatusCode: ports.TxStatusPending}, nil
	}

	require.NoError(t, h.svc.HandleCallback(context.Background(), callbackPayload("INV-202607-000001-01")))
	assert.Zero(t, h.processPaid, "no payment without check confirmation")
	assert.Equal(t, domain.InvoiceUnpaid, h.invoice.Status)
	assert.Equal(t, domain.TxPending, h.tx(tr.ID).Status, "left pending for reconciliation")
}

func TestCallbackCheckCancelledMarksFailed(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{StatusCode: ports.TxStatusCancelled}, nil
	}

	require.NoError(t, h.svc.HandleCallback(context.Background(), callbackPayload("INV-202607-000001-01")))
	assert.Equal(t, domain.TxFailed, h.tx(tr.ID).Status)
	assert.Zero(t, h.processPaid)
}

func TestCallbackCheckErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return nil, apperr.External("duitku", errors.New("timeout"))
	}

	err := h.svc.HandleCallback(context.Background(), callbackPayload("INV-202607-000001-01"))
	require.Error(t, err)
	assert.Zero(t, h.processPaid)
}

func TestCallbackAmountShortAfterCheckRejected(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{Amount: 50000, StatusCode: ports.TxStatusSuccess}, nil
	}

	err := h.svc.HandleCallback(context.Background(), callbackPayload("INV-202607-000001-01"))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	assert.Equal(t, domain.InvoiceUnpaid, h.invoice.Status)
	assert.Zero(t, h.processPaid)
}

// Credit payment

func TestPayWithCreditSuccess(t *testing.T) {
	inv := unpaidInvoice()
	inv.CreditApplied = 30000 // remaining 120000
	h := newFixture(t, inv)

	res, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "credit")
	require.NoError(t, err)
	assert.Equal(t, "paid", res.Status)
	assert.Equal(t, int64(120000), res.Amount)

	require.Len(t, h.credits.calls, 1)
	assert.Equal(t, deduction{ClientID: 5, Amount: 120000,
		Reason: "Payment for invoice INV-202607-000001", InvoiceID: 10}, h.credits.calls[0])

	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	assert.Equal(t, 1, h.processPaid)
	require.Len(t, h.txs, 1)
	assert.Equal(t, domain.GatewayCredit, h.txs[0].Gateway)
	assert.Equal(t, domain.TxSuccess, h.txs[0].Status)
}

func TestPayWithCreditDepositGuard(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.items = []domain.InvoiceItem{
		{InvoiceID: 10, Description: "Deposit Rp150.000", Amount: 150000, RelatedType: domain.RelatedDeposit},
	}

	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "credit")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	assert.Empty(t, h.credits.calls, "no credit deducted for deposit invoices")
	assert.Zero(t, h.processPaid)
	assert.Equal(t, domain.InvoiceUnpaid, h.invoice.Status)
}

func TestPayWithCreditInsufficientBalancePropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.credits.err = apperr.Conflict("insufficient credit balance")

	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "credit")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	assert.Zero(t, h.processPaid)
	assert.Equal(t, domain.InvoiceUnpaid, h.invoice.Status)
}

// Gateway payment initiation

func TestPayInvoiceCreatesGatewayTransaction(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	var gotReq ports.CreateTxRequest
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		gotReq = req
		return &ports.CreateTxResult{
			Reference: "DREF-9", PaymentURL: "https://pay.example/9",
			VANumber: "8888001", QRString: "qr", Amount: req.Amount,
		}, nil
	}

	res, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.NoError(t, err)

	assert.Equal(t, "INV-202607-000001-01", gotReq.MerchantOrderID, "first attempt is -01")
	assert.Equal(t, int64(150000), gotReq.Amount)
	assert.Equal(t, "budi@example.com", gotReq.Email)
	assert.Equal(t, "Budi Santoso", gotReq.CustomerName)
	assert.Equal(t, "http://api.test/api/v1/webhooks/duitku", gotReq.CallbackURL)
	assert.Equal(t, "http://front.test/payments/return", gotReq.ReturnURL)
	assert.Equal(t, defaultExpiryMinutes, gotReq.ExpiryMinutes)

	assert.Equal(t, "pending", res.Status)
	assert.Equal(t, "INV-202607-000001-01", res.MerchantOrderID)
	assert.Equal(t, "https://pay.example/9", res.PaymentURL)
	assert.Equal(t, "8888001", res.VANumber)
	assert.Equal(t, int64(150000), res.Amount)
	require.NotNil(t, res.ExpiresAt)
	assert.Equal(t, testNow.Add(defaultExpiryMinutes*time.Minute), *res.ExpiresAt)

	require.Len(t, h.txs, 1)
	assert.Equal(t, domain.TxPending, h.txs[0].Status)
	assert.Equal(t, "DREF-9", h.txs[0].GatewayReference)
	require.NotNil(t, h.txs[0].MerchantOrderID)
	assert.Equal(t, "INV-202607-000001-01", *h.txs[0].MerchantOrderID)
	require.NotNil(t, h.txs[0].ExpiresAt, "expiry must be persisted so a reload can resume this transaction")
	assert.Equal(t, testNow.Add(defaultExpiryMinutes*time.Minute), *h.txs[0].ExpiresAt)
}

// TestPayInvoiceGatewaySurchargesFeeOntoGrossAmount covers a real bug found
// 2026-07-27: a merchant's Duitku contract can pass a VA/QRIS channel's fee
// on to the customer, so the actual amount the customer must transfer (the
// VA's own registered "closed amount") is larger than the invoice's own
// total - Duitku's inquiry response reports this gross figure directly. The
// service must surface that gross amount to the client (PaymentInstruction),
// while still recording the invoice-applicable NET amount plus a separate
// Fee on the transaction row, so invoice accounting is never inflated by a
// payment-processing surcharge.
func TestPayInvoiceGatewaySurchargesFeeOntoGrossAmount(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		return &ports.CreateTxResult{
			Reference: "DREF-FEE", VANumber: "8888002", Amount: req.Amount + 5000,
		}, nil
	}

	res, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "BC")
	require.NoError(t, err)

	assert.Equal(t, int64(155000), res.Amount, "gross amount shown to the client includes the surcharged fee")
	assert.Equal(t, int64(5000), res.Fee)

	require.Len(t, h.txs, 1)
	assert.Equal(t, int64(150000), h.txs[0].Amount, "transaction amount stays the invoice-applicable NET amount")
	assert.Equal(t, int64(5000), h.txs[0].Fee)
}

// TestPayInvoiceGatewayNoSurchargeFeeIsZero covers the complementary case: a
// gateway (or a Duitku contract that absorbs the fee instead of passing it
// on) echoes the requested amount back unchanged - no surcharge should ever
// be synthesized.
func TestPayInvoiceGatewayNoSurchargeFeeIsZero(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		return &ports.CreateTxResult{Reference: "DREF-NOFEE", Amount: req.Amount}, nil
	}

	res, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VC")
	require.NoError(t, err)

	assert.Equal(t, int64(150000), res.Amount)
	assert.Equal(t, int64(0), res.Fee)
	require.Len(t, h.txs, 1)
	assert.Equal(t, int64(0), h.txs[0].Fee)
}

func TestPayInvoiceHonorsPerMethodExpiryOverride(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.settings.GetJSONFn = func(ctx context.Context, key string, out any) error {
		assert.Equal(t, methodExpiryMinutesKey, key)
		m, ok := out.(*map[string]int)
		require.True(t, ok)
		*m = map[string]int{"OV": 60, "SP": 30}
		return nil
	}
	var gotReq ports.CreateTxRequest
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		gotReq = req
		return &ports.CreateTxResult{Reference: "DREF-11", PaymentURL: "u", Amount: req.Amount}, nil
	}

	res, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "OV")
	require.NoError(t, err)

	assert.Equal(t, 60, gotReq.ExpiryMinutes, "OV has a configured 60-minute override")
	require.NotNil(t, res.ExpiresAt)
	assert.Equal(t, testNow.Add(60*time.Minute), *res.ExpiresAt)
}

func TestPayInvoiceFallsBackToDefaultExpiryForUnlistedMethod(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.settings.GetJSONFn = func(ctx context.Context, key string, out any) error {
		m := out.(*map[string]int)
		*m = map[string]int{"OV": 60}
		return nil
	}
	var gotReq ports.CreateTxRequest
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		gotReq = req
		return &ports.CreateTxResult{Reference: "DREF-12", PaymentURL: "u", Amount: req.Amount}, nil
	}

	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.NoError(t, err)
	assert.Equal(t, defaultExpiryMinutes, gotReq.ExpiryMinutes, "VA has no override, must fall back to default")
}

func TestPayInvoiceFallsBackToDefaultExpiryWhenSettingsNil(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.svc.d.Settings = nil
	var gotReq ports.CreateTxRequest
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		gotReq = req
		return &ports.CreateTxResult{Reference: "DREF-13", PaymentURL: "u", Amount: req.Amount}, nil
	}

	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.NoError(t, err)
	assert.Equal(t, defaultExpiryMinutes, gotReq.ExpiryMinutes)
}

func TestPayInvoiceSecondAttemptTwoDigitSuffix(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	var gotMO string
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		gotMO = req.MerchantOrderID
		return &ports.CreateTxResult{Reference: "DREF-10", PaymentURL: "u"}, nil
	}

	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "OV")
	require.NoError(t, err)
	assert.Equal(t, "INV-202607-000001-01", gotMO, "first attempt is -01")

	_, err = h.svc.PayInvoice(context.Background(), 7, 5, 10, "OV")
	require.NoError(t, err)
	assert.Equal(t, "INV-202607-000001-02", gotMO,
		"attempt is an atomically reserved, monotonically increasing per-invoice counter, 2-digit")
}

// TestPayInvoiceConcurrentAttemptsGetDistinctMerchantOrderIDs is the
// regression test for the "Pay Now" double-click / client-retry race: two
// concurrent PayInvoice calls for the SAME invoice must never compute the
// same attempt number. Before the fix, payWithGateway derived attempt from
// len(ListByInvoice(invoiceID))+1 - an unlocked read with a TOCTOU window
// between the read and the eventual Create, so two overlapping callers could
// both read the same count and collide on merchantOrderId (surfacing as an
// ugly unique-constraint 500 on the second insert instead of a clean
// response). NextAttempt closes that window by reserving the number
// atomically (single UPDATE ... RETURNING against the counters table; see
// Repo.NextAttempt in repo.go), independent of when/whether Create runs.
func TestPayInvoiceConcurrentAttemptsGetDistinctMerchantOrderIDs(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	// No shared audit-entry slice access across goroutines: audit logging is
	// not the subject of this race test, so disable it to keep the mock
	// simple and race-clean.
	h.svc.d.Audit = nil
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		return &ports.CreateTxResult{
			Reference: "DREF-" + req.MerchantOrderID, PaymentURL: "https://pay.example/" + req.MerchantOrderID,
		}, nil
	}

	const n = 8
	results := make([]*PaymentInstruction, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, n)
	for i, err := range errs {
		require.NoError(t, err, "call %d", i)
		require.NotNil(t, results[i])
		mo := results[i].MerchantOrderID
		require.NotEmpty(t, mo)
		assert.False(t, seen[mo], "merchantOrderId %q collided across concurrent PayInvoice calls", mo)
		seen[mo] = true
	}
	assert.Len(t, seen, n, "every concurrent call got a distinct merchantOrderId")
}

func TestPayInvoiceOwnershipNotFoundForOtherClient(t *testing.T) {
	h := newFixture(t, unpaidInvoice()) // invoice belongs to client 5
	_, err := h.svc.PayInvoice(context.Background(), 7, 99, 10, "VA")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code, "404, never 403, for other clients")
}

func TestPayInvoiceNotPayableConflict(t *testing.T) {
	inv := unpaidInvoice()
	inv.Status = domain.InvoicePaid
	h := newFixture(t, inv)
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
}

func TestPayInvoiceOverdueIsPayable(t *testing.T) {
	inv := unpaidInvoice()
	inv.Status = domain.InvoiceOverdue
	h := newFixture(t, inv)
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		return &ports.CreateTxResult{Reference: "R", PaymentURL: "u"}, nil
	}
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.NoError(t, err)
}

func TestPayInvoiceNonIDRRejected(t *testing.T) {
	inv := unpaidInvoice()
	inv.Currency = "USD"
	h := newFixture(t, inv)
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
}

func TestPayInvoiceEmptyMethodValidation(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "  ")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeValidation, apperr.From(err).Code)
}

func TestPayInvoiceGatewayErrorNoTransactionStored(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		return nil, apperr.External("duitku", errors.New("down"))
	}
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.Error(t, err)
	assert.Empty(t, h.txs)
}

// Gateway registry

func TestPayInvoiceRoutesManualSentinelToManualGateway(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	manualGW := &mocks.MockPaymentGateway{
		CreateTransactionFn: func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
			return &ports.CreateTxResult{
				Reference: "MANUAL-1", Amount: req.Amount,
				BankAccounts: []ports.BankAccount{{BankName: "BCA", AccountNumber: "123", AccountHolder: "WHCMS"}},
				Note:         "Include the reference in your transfer note.",
			}, nil
		},
	}
	h.svc.d.Gateways[string(domain.GatewayManual)] = manualGW

	res, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, ManualMethodBankTransfer)
	require.NoError(t, err)
	assert.Equal(t, "pending", res.Status)
	assert.Empty(t, res.PaymentURL, "manual gateway makes no HTTP call, no hosted page")
	assert.Empty(t, res.VANumber)
	require.Len(t, res.BankAccounts, 1)
	assert.Equal(t, "BCA", res.BankAccounts[0].BankName)
	assert.NotEmpty(t, res.Note)

	require.Len(t, h.txs, 1)
	assert.Equal(t, domain.GatewayManual, h.txs[0].Gateway)
	assert.Equal(t, ManualMethodBankTransfer, h.txs[0].MethodCode)
}

func TestPayInvoiceUnwiredGatewayIsInternalError(t *testing.T) {
	h := newFixture(t, unpaidInvoice()) // fixture only wires "duitku"
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, ManualMethodBankTransfer)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	assert.Empty(t, h.txs, "no transaction row for a gateway that was never wired")
}

func TestGetPaymentMethodsAggregatesAcrossGateways(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	manualGW := &mocks.MockPaymentGateway{
		GetPaymentMethodsFn: func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
			return []ports.PaymentMethod{{Code: ManualMethodBankTransfer, Name: "Bank Transfer"}}, nil
		},
	}
	h.svc.d.Gateways[string(domain.GatewayManual)] = manualGW
	h.svc.d.GatewayOrder = append(h.svc.d.GatewayOrder, string(domain.GatewayManual))
	h.gateway.GetPaymentMethodsFn = func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
		return []ports.PaymentMethod{{Code: "VA", Name: "Virtual Account"}}, nil
	}

	methods, err := h.svc.GetPaymentMethods(context.Background(), 5, 10)
	require.NoError(t, err)
	require.Len(t, methods, 2)
	assert.Equal(t, "VA", methods[0].Code)
	assert.Equal(t, "duitku", methods[0].Gateway)
	assert.Equal(t, ManualMethodBankTransfer, methods[1].Code)
	assert.Equal(t, "manual", methods[1].Gateway)
}

func TestGetPaymentMethodsNonDefaultGatewayErrorOmitted(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	manualGW := &mocks.MockPaymentGateway{
		GetPaymentMethodsFn: func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
			return nil, apperr.External("manual", errors.New("disabled"))
		},
	}
	h.svc.d.Gateways[string(domain.GatewayManual)] = manualGW
	h.svc.d.GatewayOrder = append(h.svc.d.GatewayOrder, string(domain.GatewayManual))
	h.gateway.GetPaymentMethodsFn = func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
		return []ports.PaymentMethod{{Code: "VA", Name: "Virtual Account"}}, nil
	}

	methods, err := h.svc.GetPaymentMethods(context.Background(), 5, 10)
	require.NoError(t, err, "a non-default gateway erroring must not fail the whole call")
	require.Len(t, methods, 1, "the erroring non-default gateway is omitted, not propagated")
	assert.Equal(t, "VA", methods[0].Code)
}

func TestGetPaymentMethodsDefaultGatewayErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.gateway.GetPaymentMethodsFn = func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
		return nil, apperr.External("duitku", errors.New("down"))
	}
	_, err := h.svc.GetPaymentMethods(context.Background(), 5, 10)
	require.Error(t, err, "the default gateway's error must propagate, not be swallowed")
}

// TestGetPaymentMethodsZeroRemainingSkipsGateways covers a fully-discounted
// or Rp0 invoice: Duitku's getpaymentmethod rejects amount<=0, so calling it
// would surface as a spurious "methods could not load" error even though the
// client has nothing left to pay. No gateway should be called at all.
func TestGetPaymentMethodsZeroRemainingSkipsGateways(t *testing.T) {
	inv := unpaidInvoice()
	inv.CreditApplied = inv.Total // remaining = 0
	h := newFixture(t, inv)
	h.gateway.GetPaymentMethodsFn = func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
		t.Fatal("gateway must not be called when nothing remains due")
		return nil, nil
	}

	methods, err := h.svc.GetPaymentMethods(context.Background(), 5, 10)
	require.NoError(t, err)
	assert.Empty(t, methods)
}

// Payment methods

func TestGetPaymentMethodsCacheMissThenHit(t *testing.T) {
	inv := unpaidInvoice()
	inv.CreditApplied = 50000 // amount bucket = 100000
	h := newFixture(t, inv)

	store := map[string]any{}
	h.cache.GetJSONFn = func(ctx context.Context, key string, out any) (bool, error) {
		v, ok := store[key]
		if !ok {
			return false, nil
		}
		*(out.(*[]ports.PaymentMethod)) = v.([]ports.PaymentMethod)
		return true, nil
	}
	h.cache.SetJSONFn = func(ctx context.Context, key string, val any, ttl time.Duration) error {
		assert.Equal(t, methodsCacheTTL, ttl)
		store[key] = val
		return nil
	}
	gwCalls := 0
	h.gateway.GetPaymentMethodsFn = func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
		gwCalls++
		assert.Equal(t, int64(100000), amount, "remaining after credit_applied")
		return []ports.PaymentMethod{{Code: "VA", Name: "Virtual Account", Fee: 4000}}, nil
	}

	first, err := h.svc.GetPaymentMethods(context.Background(), 5, 10)
	require.NoError(t, err)
	second, err := h.svc.GetPaymentMethods(context.Background(), 5, 10)
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Equal(t, 1, gwCalls, "second call served from cache")
	assert.Contains(t, store, fmt.Sprintf("payments:methods:%d", 100000))
}

func TestGetPaymentMethodsOwnershipNotFound(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	_, err := h.svc.GetPaymentMethods(context.Background(), 99, 10)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestGetPaymentMethodsAdminBypassesOwnership(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.gateway.GetPaymentMethodsFn = func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
		return []ports.PaymentMethod{{Code: "VA"}}, nil
	}
	methods, err := h.svc.GetPaymentMethods(context.Background(), 0, 10) // clientID 0 = admin/staff
	require.NoError(t, err)
	assert.Len(t, methods, 1)
}

func TestGetPaymentMethodsUnpayableConflict(t *testing.T) {
	inv := unpaidInvoice()
	inv.Status = domain.InvoiceCancelled
	h := newFixture(t, inv)
	_, err := h.svc.GetPaymentMethods(context.Background(), 5, 10)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
}

// Return URL

func TestResolveReturnPaidInvoice(t *testing.T) {
	inv := unpaidInvoice()
	inv.Status = domain.InvoicePaid
	h := newFixture(t, inv)
	mo := "INV-202607-000001-01"
	h.txs = append(h.txs, domain.Transaction{
		ID: 1, InvoiceID: 10, Gateway: domain.GatewayDuitku,
		MerchantOrderID: &mo, Status: domain.TxSuccess,
	})

	url := h.svc.ResolveReturn(context.Background(), mo)
	assert.Equal(t, "http://front.test/billing/invoices/10?paid=success", url)
}

func TestResolveReturnPendingTx(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)
	url := h.svc.ResolveReturn(context.Background(), "INV-202607-000001-01")
	assert.Equal(t, "http://front.test/billing/invoices/10?paid=pending", url)
}

func TestResolveReturnUnknownFallsBack(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	assert.Equal(t, "http://front.test/billing", h.svc.ResolveReturn(context.Background(), "NOPE"))
	assert.Equal(t, "http://front.test/billing", h.svc.ResolveReturn(context.Background(), ""))
}

// TestResolveReturnInfoOwnerSees is the happy path: the invoice's own client
// (ClientID 5, see unpaidInvoice) resolves their own merchantOrderId.
func TestResolveReturnInfoOwnerSees(t *testing.T) {
	inv := unpaidInvoice()
	inv.Status = domain.InvoicePaid
	h := newFixture(t, inv)
	mo := "INV-202607-000001-01"
	h.txs = append(h.txs, domain.Transaction{
		ID: 1, InvoiceID: 10, Gateway: domain.GatewayDuitku,
		MerchantOrderID: &mo, Status: domain.TxSuccess,
	})

	info, err := h.svc.ResolveReturnInfo(context.Background(), mo, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(10), info.InvoiceID)
	assert.Equal(t, "paid", info.Status)
}

// TestResolveReturnInfoOwnershipEnforced is the regression test for the
// disclosure bug: merchantOrderId is sequential/guessable
// (INV-<month>-<counter>-<attempt>), so without an ownership check any
// authenticated-but-unrelated client (or, before the fix, anyone
// unauthenticated) could resolve any other client's invoice_id/status. A
// caller whose ClientID doesn't own the invoice must get the same zero value
// as an unknown merchantOrderId - not the real data, and not a distinguishable
// error either (so existence can't be inferred).
func TestResolveReturnInfoOwnershipEnforced(t *testing.T) {
	inv := unpaidInvoice() // ClientID 5
	inv.Status = domain.InvoicePaid
	h := newFixture(t, inv)
	mo := "INV-202607-000001-01"
	h.txs = append(h.txs, domain.Transaction{
		ID: 1, InvoiceID: 10, Gateway: domain.GatewayDuitku,
		MerchantOrderID: &mo, Status: domain.TxSuccess,
	})

	info, err := h.svc.ResolveReturnInfo(context.Background(), mo, 999) // a different client
	require.NoError(t, err)
	assert.Equal(t, &ReturnInfo{}, info, "must not leak another client's invoice_id/status")
}

// TestResolveReturnInfoAdminBypassesOwnership mirrors
// TestGetPaymentMethodsAdminBypassesOwnership: clientID 0 means admin/staff,
// which may resolve any invoice's return info.
func TestResolveReturnInfoAdminBypassesOwnership(t *testing.T) {
	inv := unpaidInvoice()
	inv.Status = domain.InvoicePaid
	h := newFixture(t, inv)
	mo := "INV-202607-000001-01"
	h.txs = append(h.txs, domain.Transaction{
		ID: 1, InvoiceID: 10, Gateway: domain.GatewayDuitku,
		MerchantOrderID: &mo, Status: domain.TxSuccess,
	})

	info, err := h.svc.ResolveReturnInfo(context.Background(), mo, 0)
	require.NoError(t, err)
	assert.Equal(t, int64(10), info.InvoiceID)
	assert.Equal(t, "paid", info.Status)
}

func TestResolveReturnInfoUnknownFallsBack(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	info, err := h.svc.ResolveReturnInfo(context.Background(), "NOPE", 5)
	require.NoError(t, err)
	assert.Equal(t, &ReturnInfo{}, info)

	info, err = h.svc.ResolveReturnInfo(context.Background(), "", 5)
	require.NoError(t, err)
	assert.Equal(t, &ReturnInfo{}, info)
}

// Reconciliation

func TestReconcileOnePendingToPaid(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{Reference: "DREF-1", Amount: 150000, StatusCode: ports.TxStatusSuccess}, nil
	}

	require.NoError(t, h.svc.ReconcileOne(context.Background(), tr.ID))
	assert.Equal(t, domain.InvoicePaid, h.invoice.Status, "reconciliation pays without a callback")
	assert.Equal(t, 1, h.processPaid)
	assert.Equal(t, domain.TxSuccess, h.tx(tr.ID).Status)
}

// TestReconcileOneAppliesFeeFromCheckTransaction mirrors
// TestCallbackAppliesFeeFromCheckTransaction for the reconciliation-cron
// path: Duitku's transactionStatus "amount" is already the invoice-applicable
// figure and must be stored as-is (Fee is a separate, purely informational
// value, never subtracted from it).
func TestReconcileOneAppliesFeeFromCheckTransaction(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{Reference: "DREF-1", Amount: 150000, Fee: 5000, StatusCode: ports.TxStatusSuccess}, nil
	}

	require.NoError(t, h.svc.ReconcileOne(context.Background(), tr.ID))
	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	got := h.tx(tr.ID)
	assert.Equal(t, int64(150000), got.Amount, "amount is stored exactly as Duitku reported it, no fee arithmetic")
	assert.Equal(t, int64(5000), got.Fee)
}

func TestReconcileOneCancelledToExpired(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{StatusCode: ports.TxStatusCancelled}, nil
	}

	require.NoError(t, h.svc.ReconcileOne(context.Background(), tr.ID))
	assert.Equal(t, domain.TxExpired, h.tx(tr.ID).Status)
	assert.Zero(t, h.processPaid)
}

func TestReconcileOneStillPendingNoop(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{StatusCode: ports.TxStatusPending}, nil
	}

	require.NoError(t, h.svc.ReconcileOne(context.Background(), tr.ID))
	assert.Equal(t, domain.TxPending, h.tx(tr.ID).Status)
	assert.Zero(t, h.processPaid)
}

func TestReconcileOneNonPendingNoop(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	mo := "INV-202607-000001-01"
	h.txs = append(h.txs, domain.Transaction{
		ID: 55, InvoiceID: 10, Gateway: domain.GatewayDuitku,
		MerchantOrderID: &mo, Status: domain.TxSuccess,
	})
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		t.Fatal("no gateway call for settled transactions")
		return nil, nil
	}
	require.NoError(t, h.svc.ReconcileOne(context.Background(), 55))
}

func TestReconcileOneUnknownNotFound(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	err := h.svc.ReconcileOne(context.Background(), 12345)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestReconcilePendingCountsAndSkipsFailures(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	good := h.addPendingTx("INV-202607-000001-01", 150000)
	bad := h.addPendingTx("INV-202607-000001-02", 150000)

	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		if bad.MerchantOrderID != nil && mo == *bad.MerchantOrderID {
			return nil, apperr.External("duitku", errors.New("timeout"))
		}
		return &ports.TxStatus{Amount: 150000, StatusCode: ports.TxStatusSuccess}, nil
	}

	n, err := h.svc.ReconcilePending(context.Background())
	require.NoError(t, err, "item failures never abort the cron")
	assert.Equal(t, 1, n)
	assert.Equal(t, domain.TxSuccess, h.tx(good.ID).Status)
	assert.Equal(t, domain.TxPending, h.tx(bad.ID).Status)
}

// Misc

func TestListTransactionsPassthrough(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.svc.d.Transactions.(*mocks.MockTransactionRepo).ListFn =
		func(ctx context.Context, p ports.ListParams) ([]domain.Transaction, int64, error) {
			return []domain.Transaction{{ID: 1}}, 1, nil
		}
	rows, total, err := h.svc.ListTransactions(context.Background(), ports.ListParams{})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, int64(1), total)
}

func TestListMyTransactionsScoped(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.svc.d.Transactions.(*mocks.MockTransactionRepo).ListForClientFn =
		func(_ context.Context, clientID int64, p ports.ListParams) ([]domain.Transaction, int64, error) {
			assert.Equal(t, int64(3), clientID)
			return []domain.Transaction{{ID: 9}}, 1, nil
		}
	rows, total, err := h.svc.ListMyTransactions(context.Background(), 3, ports.ListParams{})
	require.NoError(t, err)
	assert.Len(t, rows, 1)
	assert.Equal(t, int64(1), total)
}

// ConfirmManualTransaction

func addPendingManualTx(h *fixture, id int64, merchantOrderID string, amount int64) {
	mo := merchantOrderID
	h.txs = append(h.txs, domain.Transaction{
		ID: id, InvoiceID: h.invoice.ID, Gateway: domain.GatewayManual,
		MethodCode: ManualMethodBankTransfer, MerchantOrderID: &mo, Amount: amount, Status: domain.TxPending,
	})
}

func TestConfirmManualTransactionSettlesPendingRow(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	addPendingManualTx(h, 501, "INV-202607-000001-01", 150000)

	err := h.svc.ConfirmManualTransaction(context.Background(), 1, 501)
	require.NoError(t, err)

	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	assert.Equal(t, 1, h.processPaid)
	require.Len(t, h.txs, 1, "settles the SAME row, does not create a new one")
	assert.Equal(t, domain.TxSuccess, h.tx(501).Status)
}

func TestConfirmManualTransactionWrongGatewayConflict(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000) // Gateway: duitku, not manual

	err := h.svc.ConfirmManualTransaction(context.Background(), 1, h.txs[0].ID)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	assert.Zero(t, h.processPaid)
}

func TestConfirmManualTransactionNotPendingConflict(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	addPendingManualTx(h, 501, "INV-202607-000001-01", 150000)
	h.txs[0].Status = domain.TxSuccess

	err := h.svc.ConfirmManualTransaction(context.Background(), 1, 501)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
	assert.Zero(t, h.processPaid)
}

func TestConfirmManualTransactionUnknownNotFound(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	err := h.svc.ConfirmManualTransaction(context.Background(), 1, 999)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestConfirmManualTransactionRepoErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.svc.d.Transactions.(*mocks.MockTransactionRepo).GetByIDFn =
		func(ctx context.Context, id int64) (*domain.Transaction, error) {
			return nil, errors.New("db down")
		}
	err := h.svc.ConfirmManualTransaction(context.Background(), 1, 501)
	require.Error(t, err)
}

func TestParseAmount(t *testing.T) {
	assert.Equal(t, int64(150000), parseAmount("150000"))
	assert.Equal(t, int64(150000), parseAmount("150000.00"))
	assert.Equal(t, int64(150000), parseAmount(" 150000 "))
	assert.Equal(t, int64(0), parseAmount("abc"))
	assert.Equal(t, int64(0), parseAmount(""))
}
