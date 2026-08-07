// Package payments implements the M-PAYMENTS module: Duitku payment
// initiation, the signed webhook, the idempotent ApplyPayment core
// (ports.PaymentApplier), credit payments, reconciliation of pending
// transactions and the admin transaction listing. It owns the transactions
// repository (ports.TransactionRepo).
//
// Route table (mounted on the /api/v1 group by the composition root):
//
//	GET  /payments/methods?invoice_id=   payable methods for an invoice (cached 10m per amount)  [auth]
//	POST /invoices/:id/pay               {method} start a Duitku payment or pay with credit      [auth+client]
//	POST /webhooks/duitku                Duitku callback (form-urlencoded, signature-verified)   [public]
//	GET  /payments/return                ?merchantOrderId= redirect back to the frontend invoice [auth, rate-limited, own invoice only]
//	GET  /admin/transactions             ?search&status&page&per_page                            [role admin/staff, perm payments]
//
// Cron/worker entrypoints: ReconcilePending(ctx) (int, error) and
// ReconcileOne(ctx, transactionID) error (jobs.TypePaymentReconcileOne).
package payments

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
	"github.com/tsdlamongan/whcms/backend/pkg/httpx"
)

// Duitku callback result codes ("00" success, anything else failed).
const resultCodeSuccess = "00"

// methodExpiryMinutesKey is the optional settings key (JSON object mapping
// Duitku method code -> expiry minutes) consulted by expiryMinutesFor
// (FR-PAY-011). Not part of the generic typed Settings UI (settings.Spec) -
// it's a free-form map, not a scalar/list - so it's configured directly via
// SettingsRepo.Set / a seed row for now, same as the gateway.* keys.
const methodExpiryMinutesKey = "payments.method_expiry_minutes"

const (
	// defaultExpiryMinutes is the payment expiry passed to the gateway when
	// the method has no specific override (24h).
	defaultExpiryMinutes = 1440
	// methodsCacheTTL is how long payment-method lists are cached per amount.
	methodsCacheTTL = 10 * time.Minute
	// reconcileMinAge is how old a pending transaction must be before the
	// reconciliation cron re-checks it.
	reconcileMinAge = 5 * time.Minute
)

// ErrBadSignature is returned by HandleCallback when the webhook signature
// does not verify; the handler maps it to HTTP 400.
var ErrBadSignature = errors.New("invalid callback signature")

// CreditDeducter is the narrow consumer-side interface onto the clients
// module (MODULES.md §2 exact signature; satisfied by the clients service at
// wiring time). It must fail with a CONFLICT/VALIDATION apperr when the
// client's balance is insufficient.
type CreditDeducter interface {
	DeductCredit(ctx context.Context, clientID int64, amount int64, reason string, relatedInvoiceID int64) error
}

// ManualMethodBankTransfer is the sentinel method code for the manual
// bank-transfer gateway's one payment channel - recognized in PayInvoice the
// same way "credit" is, routing to the "manual" entry in Deps.Gateways.
// Cannot collide with a real Duitku channel code (those are short 1-2
// char uppercase codes, e.g. "VA","OV","BC","QR" - PRD.md §8.1).
const ManualMethodBankTransfer = "bank_transfer"

// Deps are the service dependencies, wired by the composition root.
type Deps struct {
	Tx           ports.TxManager
	Invoices     ports.InvoiceRepo
	Transactions ports.TransactionRepo // *Repo in production
	Clients      ports.ClientRepo
	Users        ports.UserRepo
	// Gateways is the payment gateway registry, keyed by domain.Gateway code
	// ("duitku", "manual", ...). GatewayOrder controls aggregation/display
	// order in GetPaymentMethods; DefaultGateway is where any method code
	// that isn't one of PayInvoice's special sentinels ("credit",
	// ManualMethodBankTransfer) routes - preserving raw Duitku channel codes
	// (e.g. "VA"/"OV") unprefixed, exactly as before this registry existed.
	Gateways       map[string]ports.PaymentGateway
	GatewayOrder   []string
	DefaultGateway string
	Processor      ports.PaidInvoiceProcessor
	Credits        CreditDeducter
	Cache          ports.Cache
	Clock          ports.Clock
	Audit          ports.AuditLogger
	IntLog         ports.IntegrationLogger
	Log            *slog.Logger
	// Settings resolves the optional payments.method_expiry_minutes override
	// (FR-PAY-011). Nil-safe: a nil Settings (e.g. in older tests) behaves
	// exactly as if no override were configured.
	Settings    ports.SettingsRepo
	AppBaseURL  string // e.g. http://localhost:8080 (callback URL base)
	FrontendURL string // e.g. http://localhost:5173 (return/redirect base)
}

// Service implements the payments use-cases and ports.PaymentApplier.
type Service struct {
	d   Deps
	log *slog.Logger
}

// Compile-time cross-service contract check.
var _ ports.PaymentApplier = (*Service)(nil)

// New builds the payments Service.
func New(d Deps) *Service {
	log := d.Log
	if log == nil {
		log = slog.Default()
	}
	return &Service{d: d, log: log}
}

func (s *Service) now() time.Time {
	if s.d.Clock != nil {
		return s.d.Clock.Now()
	}
	return time.Now().UTC()
}

func (s *Service) audit(ctx context.Context, actorUserID int64, action string, entityID int64, before, after any) {
	if s.d.Audit != nil {
		s.d.Audit.Log(ctx, actorUserID, action, "invoice", entityID, before, after)
	}
}

// duitkuGateway returns the fixed Duitku adapter. HandleCallback/ReconcileOne
// are inherently Duitku-specific (webhook payload shape, external status
// polling) - a manual (or any future non-polled) gateway has neither, so
// this deliberately looks up a FIXED registry key rather than dispatching on
// a transaction's own Gateway field. Returns an apperr (never panics in the
// request path) if wiring ever omits the "duitku" entry.
func (s *Service) duitkuGateway() (ports.PaymentGateway, error) {
	gw, ok := s.d.Gateways[string(domain.GatewayDuitku)]
	if !ok || gw == nil {
		return nil, apperr.Internal(errors.New("payments: duitku gateway not wired"))
	}
	return gw, nil
}

func isNotFound(err error) bool {
	var ae *apperr.Error
	return errors.As(err, &ae) && ae.Code == apperr.CodeNotFound
}

// getOwnedInvoice loads the invoice and enforces client ownership: clients
// (clientID != 0) get NOT_FOUND - never FORBIDDEN - for other clients'
// invoices; admins/staff pass clientID 0.
func (s *Service) getOwnedInvoice(ctx context.Context, clientID, invoiceID int64) (*domain.Invoice, error) {
	inv, err := s.d.Invoices.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, err
	}
	if inv == nil || (clientID != 0 && inv.ClientID != clientID) {
		return nil, apperr.NotFound("invoice")
	}
	return inv, nil
}

func invoicePayable(status domain.InvoiceStatus) bool {
	return status == domain.InvoiceUnpaid || status == domain.InvoiceOverdue
}

// Payment methods

// GetPaymentMethods returns the payment channels available for the
// invoice's remaining amount, aggregated across every registered gateway
// (in Deps.GatewayOrder) and cached 10 minutes per amount. clientID 0 =
// admin/staff. If the DEFAULT gateway errors, that error propagates (today's
// exact single-gateway behavior); a non-default gateway erroring or
// returning an empty list (e.g. manual disabled/unconfigured) is simply
// omitted from the aggregate - checkout still works with what's left.
func (s *Service) GetPaymentMethods(ctx context.Context, clientID, invoiceID int64) ([]ports.PaymentMethod, error) {
	inv, err := s.getOwnedInvoice(ctx, clientID, invoiceID)
	if err != nil {
		return nil, err
	}
	if !invoicePayable(inv.Status) {
		return nil, apperr.Conflict("invoice is not payable")
	}
	amount := inv.Total - inv.CreditApplied
	if amount <= 0 {
		// Nothing left to collect (e.g. a fully-discounted or Rp0 invoice) -
		// no gateway has a channel list for a non-positive amount (Duitku's
		// getpaymentmethod rejects amount<=0) and none is needed: the client
		// settles via credit (payWithCredit already treats remaining<=0 as an
		// instant no-op payment).
		return []ports.PaymentMethod{}, nil
	}

	key := fmt.Sprintf("payments:methods:%d", amount)
	var methods []ports.PaymentMethod
	if s.d.Cache != nil {
		if hit, err := s.d.Cache.GetJSON(ctx, key, &methods); err == nil && hit {
			return methods, nil
		}
	}
	methods = nil
	for _, code := range s.d.GatewayOrder {
		gw := s.d.Gateways[code]
		if gw == nil {
			continue
		}
		ms, err := gw.GetPaymentMethods(ctx, amount)
		if err != nil {
			if code == s.d.DefaultGateway {
				return nil, err
			}
			s.log.WarnContext(ctx, "payments: optional gateway methods lookup failed, omitting",
				"gateway", code, "error", err)
			continue
		}
		for i := range ms {
			ms[i].Gateway = code
		}
		methods = append(methods, ms...)
	}
	if s.d.Cache != nil {
		if err := s.d.Cache.SetJSON(ctx, key, methods, methodsCacheTTL); err != nil {
			s.log.WarnContext(ctx, "payments: cache methods failed", "error", err)
		}
	}
	return methods, nil
}

// Pay

// PayInvoice starts a payment for an unpaid/overdue invoice. method "credit"
// settles from the client's credit balance; any other method opens a Duitku
// transaction with merchantOrderID = "<invoice_number>-<2-digit attempt>".
// The attempt number is reserved atomically per invoice (TransactionRepo.
// NextAttempt) so two concurrent calls for the SAME invoice - a double-clicked
// "Pay Now" or a client retry racing the original request - always get
// distinct merchantOrderIds instead of colliding on the unique constraint.
func (s *Service) PayInvoice(ctx context.Context, actorUserID, clientID, invoiceID int64, method string) (*PaymentInstruction, error) {
	method = strings.TrimSpace(method)
	if method == "" {
		return nil, apperr.Validation("method is required",
			apperr.FieldError{Field: "method", Message: "must not be empty"})
	}
	inv, err := s.getOwnedInvoice(ctx, clientID, invoiceID)
	if err != nil {
		return nil, err
	}
	if !invoicePayable(inv.Status) {
		return nil, apperr.Conflict("invoice is not payable")
	}
	if inv.Currency != "" && inv.Currency != "IDR" {
		return nil, apperr.Conflict("only IDR invoices are supported")
	}
	switch method {
	case "credit":
		return s.payWithCredit(ctx, actorUserID, inv.ID)
	case ManualMethodBankTransfer:
		return s.payWithGateway(ctx, actorUserID, inv, string(domain.GatewayManual), method)
	default:
		return s.payWithGateway(ctx, actorUserID, inv, string(domain.GatewayDuitku), method)
	}
}

// expiryMinutesFor resolves the payment expiry for a method, honoring an
// optional per-method override in methodExpiryMinutesKey (FR-PAY-011) and
// falling back to defaultExpiryMinutes when unconfigured, invalid, or when
// Settings is nil.
func (s *Service) expiryMinutesFor(ctx context.Context, method string) int {
	if s.d.Settings == nil {
		return defaultExpiryMinutes
	}
	overrides := map[string]int{}
	if err := s.d.Settings.GetJSON(ctx, methodExpiryMinutesKey, &overrides); err != nil {
		return defaultExpiryMinutes
	}
	if minutes, ok := overrides[method]; ok && minutes > 0 {
		return minutes
	}
	return defaultExpiryMinutes
}

func (s *Service) payWithGateway(ctx context.Context, actorUserID int64, inv *domain.Invoice, gatewayCode, method string) (*PaymentInstruction, error) {
	gw, ok := s.d.Gateways[gatewayCode]
	if !ok || gw == nil {
		return nil, apperr.Internal(fmt.Errorf("payments: gateway %q not wired", gatewayCode))
	}

	remaining := inv.Total - inv.CreditApplied
	if remaining <= 0 {
		return nil, apperr.Conflict("invoice has no remaining amount due")
	}

	// Atomic reservation (counters table, scope "payment_attempt:<id>"): a
	// single UPDATE ... RETURNING statement, so two concurrent callers for
	// this invoice always observe distinct attempt numbers. Deriving the
	// attempt from len(ListByInvoice(...)) instead would race - both callers
	// could read the same count before either had inserted its row.
	attempt, err := s.d.Transactions.NextAttempt(ctx, inv.ID)
	if err != nil {
		return nil, err
	}
	merchantOrderID := fmt.Sprintf("%s-%02d", inv.InvoiceNumber, attempt)

	client, err := s.d.Clients.GetByID(ctx, inv.ClientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, apperr.NotFound("client")
	}
	email := ""
	if user, err := s.d.Users.GetByID(ctx, client.UserID); err == nil && user != nil {
		email = user.Email
	}

	expiryMinutes := s.expiryMinutesFor(ctx, method)
	res, err := gw.CreateTransaction(ctx, ports.CreateTxRequest{
		MerchantOrderID: merchantOrderID,
		Amount:          remaining,
		Method:          method,
		ProductDetails:  "Invoice " + inv.InvoiceNumber,
		Email:           email,
		Phone:           client.Phone,
		CustomerName:    client.FullName(),
		ReturnURL:       s.d.FrontendURL + "/payments/return",
		CallbackURL:     s.d.AppBaseURL + "/api/v1/webhooks/duitku",
		ExpiryMinutes:   expiryMinutes,
	})
	if err != nil {
		return nil, err
	}

	// Some channels (real Duitku VA/QRIS when the merchant's contract passes
	// the fee to the customer) come back with a gross amount larger than what
	// we requested - trust the gateway's own number rather than recomputing a
	// fee ourselves from the getpaymentmethod catalogue, since only the
	// gateway knows whether this merchant's contract surcharges the customer
	// or absorbs the fee. A gateway that never surcharges (manual, or Duitku
	// itself when the contract absorbs the fee) simply echoes back Amount
	// unchanged, so fee computes to 0.
	fee := res.Amount - remaining
	if fee < 0 {
		fee = 0
	}
	grossAmount := remaining + fee

	raw, _ := json.Marshal(res)
	mo := merchantOrderID
	expiresAt := s.now().Add(time.Duration(expiryMinutes) * time.Minute)
	t := &domain.Transaction{
		InvoiceID:        inv.ID,
		Gateway:          domain.Gateway(gatewayCode),
		MethodCode:       method,
		MerchantOrderID:  &mo,
		GatewayReference: res.Reference,
		Amount:           remaining,
		Fee:              fee,
		Status:           domain.TxPending,
		Raw:              raw,
		ExpiresAt:        &expiresAt,
	}
	if err := s.d.Transactions.Create(ctx, t); err != nil {
		return nil, err
	}

	s.audit(ctx, actorUserID, "payment.initiated", inv.ID,
		nil, map[string]any{"merchant_order_id": merchantOrderID, "gateway": gatewayCode, "method": method, "amount": remaining, "fee": fee})

	return &PaymentInstruction{
		Status:          "pending",
		TransactionID:   t.ID,
		MerchantOrderID: merchantOrderID,
		Reference:       res.Reference,
		PaymentURL:      res.PaymentURL,
		VANumber:        res.VANumber,
		QRString:        res.QRString,
		BankAccounts:    res.BankAccounts,
		Note:            res.Note,
		Amount:          grossAmount,
		Fee:             fee,
		ExpiresAt:       &expiresAt,
	}, nil
}

// payWithCredit settles the remaining amount from the client's credit
// balance inside ONE transaction: row-lock invoice -> deposit-item guard ->
// DeductCredit -> ApplyPayment (gateway credit).
func (s *Service) payWithCredit(ctx context.Context, actorUserID, invoiceID int64) (*PaymentInstruction, error) {
	var out *PaymentInstruction
	err := s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		inv, err := s.d.Invoices.GetByIDForUpdate(ctx, invoiceID)
		if err != nil {
			return err
		}
		if inv == nil {
			return apperr.NotFound("invoice")
		}
		if !invoicePayable(inv.Status) {
			return apperr.Conflict("invoice is not payable")
		}
		items, err := s.d.Invoices.GetItems(ctx, invoiceID)
		if err != nil {
			return err
		}
		for _, it := range items {
			if it.RelatedType == domain.RelatedDeposit {
				return apperr.Conflict("deposit invoices cannot be paid with credit")
			}
		}
		remaining := inv.Total - inv.CreditApplied
		if remaining > 0 {
			if err := s.d.Credits.DeductCredit(ctx, inv.ClientID, remaining,
				"Payment for invoice "+inv.InvoiceNumber, invoiceID); err != nil {
				return err
			}
		}
		if err := s.ApplyPayment(ctx, invoiceID, ports.ApplyTx{
			Gateway:    domain.GatewayCredit,
			MethodCode: "credit",
			Amount:     remaining,
			PaidAt:     s.now(),
		}); err != nil {
			return err
		}
		s.audit(ctx, actorUserID, "payment.credit", invoiceID,
			nil, map[string]any{"amount": remaining})
		out = &PaymentInstruction{Status: "paid", Amount: remaining}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ApplyPayment - THE idempotent core (ports.PaymentApplier)

// ApplyPayment marks an invoice paid exactly once. Inside one DB transaction
// it row-locks the invoice; already paid/refunded invoices are a silent
// no-op (safe webhook re-delivery). Otherwise it validates the amount covers
// the remaining total (short -> CONFLICT, overpay -> logged and accepted),
// upserts the success transaction (matching merchant_order_id, or a new row
// for credit/manual), transitions the invoice to paid via the domain state
// machine, and dispatches billing.ProcessPaid in the SAME transaction.
func (s *Service) ApplyPayment(ctx context.Context, invoiceID int64, tx ports.ApplyTx) error {
	return s.d.Tx.WithinTx(ctx, func(ctx context.Context) error {
		inv, err := s.d.Invoices.GetByIDForUpdate(ctx, invoiceID)
		if err != nil {
			return err
		}
		if inv == nil {
			return apperr.NotFound("invoice")
		}
		if inv.Status == domain.InvoicePaid || inv.Status == domain.InvoiceRefunded {
			return nil // idempotent no-op: already settled
		}
		if _, err := domain.TransitionInvoice(inv.Status, domain.InvoicePaid); err != nil {
			return err // CONFLICT for draft/cancelled
		}

		remaining := inv.Total - inv.CreditApplied
		if tx.Amount < remaining {
			return apperr.Newf(apperr.CodeConflict,
				"payment amount %d is less than the %d due on invoice %s",
				tx.Amount, remaining, inv.InvoiceNumber)
		}
		if tx.Amount > remaining {
			s.log.WarnContext(ctx, "payments: amount exceeds due; accepting",
				"invoice_id", invoiceID, "amount", tx.Amount, "due", remaining)
		}

		paidAt := tx.PaidAt
		if paidAt.IsZero() {
			paidAt = s.now()
		}
		raw := tx.Raw
		if len(raw) == 0 {
			raw = []byte("{}")
		}

		var existing *domain.Transaction
		if tx.MerchantOrderID != "" {
			existing, err = s.d.Transactions.GetByMerchantOrderID(ctx, tx.MerchantOrderID)
			if err != nil && !isNotFound(err) {
				return err
			}
		}
		if existing != nil {
			if existing.InvoiceID != invoiceID {
				return apperr.Conflict("transaction belongs to a different invoice")
			}
			if existing.Status != domain.TxSuccess {
				existing.Status = domain.TxSuccess
				if tx.MethodCode != "" {
					existing.MethodCode = tx.MethodCode
				}
				if tx.GatewayReference != "" {
					existing.GatewayReference = tx.GatewayReference
				}
				existing.Amount = tx.Amount
				existing.Fee = tx.Fee
				existing.Raw = raw
				existing.PaidAt = &paidAt
				if err := s.d.Transactions.Update(ctx, existing); err != nil {
					return err
				}
			}
		} else {
			t := &domain.Transaction{
				InvoiceID:        invoiceID,
				Gateway:          tx.Gateway,
				MethodCode:       tx.MethodCode,
				GatewayReference: tx.GatewayReference,
				Amount:           tx.Amount,
				Fee:              tx.Fee,
				Status:           domain.TxSuccess,
				Raw:              raw,
				PaidAt:           &paidAt,
			}
			if tx.MerchantOrderID != "" {
				mo := tx.MerchantOrderID
				t.MerchantOrderID = &mo
			}
			if err := s.d.Transactions.Create(ctx, t); err != nil {
				return err
			}
		}

		if err := s.d.Invoices.UpdateStatus(ctx, invoiceID, domain.InvoicePaid, &paidAt); err != nil {
			return err
		}
		s.audit(ctx, 0, "invoice.paid", invoiceID,
			map[string]any{"status": inv.Status},
			map[string]any{"status": domain.InvoicePaid, "gateway": tx.Gateway, "amount": tx.Amount})

		// Post-payment effects (activation, renewals, PDF, receipt) - same tx.
		return s.d.Processor.ProcessPaid(ctx, invoiceID)
	})
}

// Webhook

// HandleCallback processes one Duitku webhook delivery. Unknown
// merchantOrderId -> NOT_FOUND (404); bad signature -> ErrBadSignature (400).
// No resultCode is ever trusted alone - CheckTransaction double-verifies
// before ApplyPayment (resultCode "00") or before marking a transaction
// Failed (resultCode != "00"), because Duitku's callback signature does not
// bind resultCode. A nil return means the handler answers 200 "OK"
// (including idempotent re-deliveries and still-pending no-ops).
//
// Every invocation - success, no-op, or failure - is recorded via IntLog
// (deferred so it fires on every return path exactly once), since this is
// the only positive evidence an operator has that Duitku actually reached
// this endpoint at all: unlike every other integration in this codebase,
// there is no matching outbound call to compare against.
func (s *Service) HandleCallback(ctx context.Context, p ports.CallbackPayload) (err error) {
	if s.d.IntLog != nil {
		defer func() {
			s.d.IntLog.Log(ctx, ports.IntegrationCall{
				Provider:   "duitku",
				Endpoint:   "/api/v1/webhooks/duitku",
				Method:     "POST",
				StatusCode: webhookLogStatus(err),
				Success:    err == nil,
				Request: map[string]any{
					"merchantOrderId": p.MerchantOrderID,
					"merchantCode":    p.MerchantCode,
					"amount":          p.Amount,
					"resultCode":      p.ResultCode,
					"reference":       p.Reference,
				},
				Error: errString(err),
			})
		}()
	}

	gw, err := s.duitkuGateway()
	if err != nil {
		return err
	}
	t, err := s.d.Transactions.GetByMerchantOrderID(ctx, p.MerchantOrderID)
	if err != nil {
		return err
	}
	if t == nil {
		return apperr.NotFound("transaction")
	}
	if !gw.VerifyCallbackSignature(p) {
		return ErrBadSignature
	}

	raw, _ := json.Marshal(p)
	if p.ResultCode != resultCodeSuccess {
		// Settled transactions are a no-op regardless of what the callback
		// says.
		if t.Status != domain.TxPending {
			return nil
		}

		// Double verification: never mark failed from the callback alone.
		// Duitku's callback signature (MD5 of merchantCode+amount+
		// merchantOrderId+apiKey) does not bind resultCode, so a signature
		// valid for one resultCode is valid for any resultCode on the same
		// order - CheckTransaction must independently confirm the failure
		// before we close the transaction out.
		st, err := gw.CheckTransaction(ctx, p.MerchantOrderID)
		if err != nil {
			return err
		}
		if st.StatusCode != ports.TxStatusCancelled {
			// Pending, success, or any other/ambiguous status: leave the
			// transaction pending so the reconciliation cron (which only
			// ever scans status='pending') keeps following up rather than
			// prematurely - and irreversibly - closing it out.
			return nil
		}
		t.Status = domain.TxFailed
		t.Raw = raw
		if p.Reference != "" {
			t.GatewayReference = p.Reference
		}
		return s.d.Transactions.Update(ctx, t)
	}

	// Double verification: never mark paid from the callback alone.
	st, err := gw.CheckTransaction(ctx, p.MerchantOrderID)
	if err != nil {
		return err
	}
	switch st.StatusCode {
	case ports.TxStatusSuccess:
		amount := st.Amount
		if amount <= 0 {
			amount = parseAmount(p.Amount)
		}
		ref := st.Reference
		if ref == "" {
			ref = p.Reference
		}
		method := p.PaymentCode
		if method == "" {
			method = t.MethodCode
		}
		return s.ApplyPayment(ctx, t.InvoiceID, ports.ApplyTx{
			Gateway:          domain.GatewayDuitku,
			MethodCode:       method,
			MerchantOrderID:  p.MerchantOrderID,
			GatewayReference: ref,
			Amount:           amount,
			Fee:              st.Fee,
			Raw:              raw,
			PaidAt:           s.now(),
		})
	case ports.TxStatusPending:
		return nil // leave the transaction pending; reconciliation follows up
	case ports.TxStatusCancelled:
		if t.Status == domain.TxPending {
			t.Status = domain.TxFailed
			t.Raw = raw
			return s.d.Transactions.Update(ctx, t)
		}
		return nil
	default:
		return apperr.Newf(apperr.CodeExternal, "duitku returned unknown status code %q", st.StatusCode)
	}
}

// webhookLogStatus approximates the HTTP status Handler.Webhook would answer
// with for err, purely for the integration log entry (mirrors handler.go's
// own error->status mapping: ErrBadSignature -> 400, apperr codes ->
// httpx.StatusFor, anything else -> 500).
func webhookLogStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	if errors.Is(err, ErrBadSignature) {
		return http.StatusBadRequest
	}
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return httpx.StatusFor(ae.Code)
	}
	return http.StatusInternalServerError
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// parseAmount parses a Duitku amount string ("150000" or "150000.00") into
// whole IDR; 0 when unparseable.
func parseAmount(raw string) int64 {
	raw = strings.TrimSpace(raw)
	if i := strings.IndexByte(raw, '.'); i >= 0 {
		raw = raw[:i]
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// Return URL

// ResolveReturn maps a gateway return visit to the frontend invoice page:
// FRONTEND_URL/billing/invoices/<id>?paid=<status>. Unknown or missing
// merchantOrderId falls back to the billing overview.
//
// Kept for backward compatibility (and its own tests); no longer used by the
// GET /payments/return HTTP handler - see ResolveReturnInfo below.
func (s *Service) ResolveReturn(ctx context.Context, merchantOrderID string) string {
	fallback := s.d.FrontendURL + "/billing"
	if merchantOrderID == "" {
		return fallback
	}
	t, err := s.d.Transactions.GetByMerchantOrderID(ctx, merchantOrderID)
	if err != nil || t == nil {
		if err != nil && !isNotFound(err) {
			s.log.WarnContext(ctx, "payments: return lookup failed", "error", err)
		}
		return fallback
	}
	status := string(t.Status)
	if inv, err := s.d.Invoices.GetByID(ctx, t.InvoiceID); err == nil && inv != nil && inv.Status == domain.InvoicePaid {
		status = "success"
	}
	return fmt.Sprintf("%s/billing/invoices/%d?paid=%s", s.d.FrontendURL, t.InvoiceID, status)
}

// ReturnInfo is the GET /payments/return JSON payload (RECONCILE.md/CONTRACTS.md
// §9: `{invoice_id, status}`). The gateway's own returnUrl points straight at
// the FRONTEND's /payments/return page (not this endpoint), so this is a pure
// JSON lookup API for that page to poll, not a browser-visited redirect.
type ReturnInfo struct {
	InvoiceID int64  `json:"invoice_id"`
	Status    string `json:"status"`
}

// ResolveReturnInfo maps a gateway return visit's merchantOrderId to its
// invoice + the INVOICE's own current status (e.g. "paid", "unpaid") - not the
// transaction's - so the frontend's `status === 'paid'` poll check lines up
// with every other invoice-status field in the API. Unknown/missing
// merchantOrderId, invoice lookup failure, or the invoice belonging to a
// different client (clientID 0 = admin/staff, bypasses this check - same
// convention as getOwnedInvoice/GetPaymentMethods) all yield the same zero
// value (invoice_id 0, status "") rather than an error, so the frontend's
// poll loop just keeps retrying instead of surfacing a hard failure - and so
// an unauthorized caller cannot distinguish "not yours" from "doesn't exist"
// (merchantOrderId is sequential/guessable, so this would otherwise let
// anyone enumerate every invoice's status).
func (s *Service) ResolveReturnInfo(ctx context.Context, merchantOrderID string, clientID int64) (*ReturnInfo, error) {
	if merchantOrderID == "" {
		return &ReturnInfo{}, nil
	}
	t, err := s.d.Transactions.GetByMerchantOrderID(ctx, merchantOrderID)
	if err != nil || t == nil {
		if err != nil && !isNotFound(err) {
			s.log.WarnContext(ctx, "payments: return lookup failed", "error", err)
		}
		return &ReturnInfo{}, nil
	}
	inv, err := s.d.Invoices.GetByID(ctx, t.InvoiceID)
	if err != nil || inv == nil || (clientID != 0 && inv.ClientID != clientID) {
		return &ReturnInfo{}, nil
	}
	return &ReturnInfo{InvoiceID: t.InvoiceID, Status: string(inv.Status)}, nil
}

// Reconciliation (cron)

// ReconcilePending re-checks every Duitku transaction pending for more than
// five minutes and returns how many were reconciled without error. Item
// failures are logged and skipped so one bad transaction never blocks the
// rest.
func (s *Service) ReconcilePending(ctx context.Context) (int, error) {
	cutoff := s.now().Add(-reconcileMinAge)
	pending, err := s.d.Transactions.ListPending(ctx, domain.GatewayDuitku, cutoff)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, t := range pending {
		if err := s.ReconcileOne(ctx, t.ID); err != nil {
			s.log.WarnContext(ctx, "payments: reconcile failed",
				"transaction_id", t.ID, "error", err)
			continue
		}
		n++
	}
	return n, nil
}

// ReconcileOne re-checks one pending Duitku transaction: statusCode "00" ->
// ApplyPayment (pending -> paid); "02" -> expired; "01"/other -> left pending.
// Non-pending transactions are a no-op.
func (s *Service) ReconcileOne(ctx context.Context, transactionID int64) error {
	t, err := s.d.Transactions.GetByID(ctx, transactionID)
	if err != nil {
		return err
	}
	if t == nil {
		return apperr.NotFound("transaction")
	}
	if t.Status != domain.TxPending || t.Gateway != domain.GatewayDuitku || t.MerchantOrderID == nil {
		return nil
	}
	gw, err := s.duitkuGateway()
	if err != nil {
		return err
	}
	st, err := gw.CheckTransaction(ctx, *t.MerchantOrderID)
	if err != nil {
		return err
	}
	switch st.StatusCode {
	case ports.TxStatusSuccess:
		amount := st.Amount
		if amount <= 0 {
			amount = t.Amount
		}
		ref := st.Reference
		if ref == "" {
			ref = t.GatewayReference
		}
		raw, _ := json.Marshal(st)
		return s.ApplyPayment(ctx, t.InvoiceID, ports.ApplyTx{
			Gateway:          domain.GatewayDuitku,
			MethodCode:       t.MethodCode,
			MerchantOrderID:  *t.MerchantOrderID,
			GatewayReference: ref,
			Amount:           amount,
			Fee:              st.Fee,
			Raw:              raw,
			PaidAt:           s.now(),
		})
	case ports.TxStatusCancelled:
		t.Status = domain.TxExpired
		return s.d.Transactions.Update(ctx, t)
	default:
		return nil // still pending at the gateway
	}
}

// Admin

// ListTransactions returns a filtered page of transactions for the admin UI.
func (s *Service) ListTransactions(ctx context.Context, p ports.ListParams) ([]domain.Transaction, int64, error) {
	return s.d.Transactions.List(ctx, p)
}

// ListMyTransactions returns clientID's own transaction history.
func (s *Service) ListMyTransactions(ctx context.Context, clientID int64, p ports.ListParams) ([]domain.Transaction, int64, error) {
	return s.d.Transactions.ListForClient(ctx, clientID, p)
}

// ConfirmManualTransaction settles a pending manual bank-transfer
// transaction once an admin has confirmed the funds arrived, via the same
// idempotent ApplyPayment core the Duitku webhook uses - keyed by the
// transaction's own merchant_order_id so it settles that exact pending row
// rather than creating an orphan/new one. Only ever callable on a
// Gateway=manual, Status=pending row: billing.AddManualPayment's freeform
// admin-recorded payments always create a TxSuccess row directly (never
// TxPending), so there is no ambiguity between the two admin-facing manual
// payment flows.
func (s *Service) ConfirmManualTransaction(ctx context.Context, actorUserID, transactionID int64) error {
	t, err := s.d.Transactions.GetByID(ctx, transactionID)
	if err != nil {
		return err
	}
	if t == nil {
		return apperr.NotFound("transaction")
	}
	if t.Gateway != domain.GatewayManual || t.Status != domain.TxPending {
		return apperr.Conflict("transaction is not a pending manual bank-transfer payment")
	}
	mo := ""
	if t.MerchantOrderID != nil {
		mo = *t.MerchantOrderID
	}
	if err := s.ApplyPayment(ctx, t.InvoiceID, ports.ApplyTx{
		Gateway:          domain.GatewayManual,
		MethodCode:       t.MethodCode,
		MerchantOrderID:  mo,
		GatewayReference: t.GatewayReference,
		Amount:           t.Amount,
		PaidAt:           s.now(),
	}); err != nil {
		return err
	}
	s.audit(ctx, actorUserID, "payment.manual_confirmed", t.InvoiceID, nil,
		map[string]any{"transaction_id": transactionID})
	return nil
}
