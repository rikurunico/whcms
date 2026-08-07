package payments

// Failure and fallback behaviour of the payments service: dependency
// errors propagate without partial writes, omitted callback fields fall
// back to safe defaults, and optional dependencies may be absent.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func (h *fixture) invoiceRepo() *mocks.MockInvoiceRepo {
	return h.svc.d.Invoices.(*mocks.MockInvoiceRepo)
}

func (h *fixture) txRepo() *mocks.MockTransactionRepo {
	return h.svc.d.Transactions.(*mocks.MockTransactionRepo)
}

func TestApplyPaymentDefaultsPaidAtAndRaw(t *testing.T) {
	h := newFixture(t, unpaidInvoice())

	require.NoError(t, h.svc.ApplyPayment(context.Background(), 10, ports.ApplyTx{
		Gateway: domain.GatewayManual, MethodCode: "manual", Amount: 150000,
		// PaidAt zero, Raw nil -> defaulted
	}))
	require.Len(t, h.txs, 1)
	require.NotNil(t, h.txs[0].PaidAt)
	assert.Equal(t, testNow, *h.txs[0].PaidAt, "zero PaidAt defaults to clock now")
	assert.JSONEq(t, `{}`, string(h.txs[0].Raw), "nil raw defaults to empty object")
	require.NotNil(t, h.invoice.PaidAt)
	assert.Equal(t, testNow, *h.invoice.PaidAt)
}

func TestApplyPaymentLockErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.invoiceRepo().GetByIDForUpdateFn = func(ctx context.Context, id int64) (*domain.Invoice, error) {
		return nil, apperr.Internal(errors.New("lock timeout"))
	}
	err := h.svc.ApplyPayment(context.Background(), 10, applyTx(150000))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
}

func TestApplyPaymentNilInvoiceNotFound(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.invoiceRepo().GetByIDForUpdateFn = func(ctx context.Context, id int64) (*domain.Invoice, error) {
		return nil, nil
	}
	err := h.svc.ApplyPayment(context.Background(), 10, applyTx(150000))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestApplyPaymentMerchantOrderLookupErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.txRepo().GetByMerchantOrderIDFn = func(ctx context.Context, mo string) (*domain.Transaction, error) {
		return nil, apperr.Internal(errors.New("db down"))
	}
	err := h.svc.ApplyPayment(context.Background(), 10, applyTx(150000))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
	assert.Zero(t, h.processPaid)
}

func TestApplyPaymentUpdateStatusErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.invoiceRepo().UpdateStatusFn = func(ctx context.Context, id int64, st domain.InvoiceStatus, paidAt *time.Time) error {
		return apperr.Internal(errors.New("write failed"))
	}
	err := h.svc.ApplyPayment(context.Background(), 10, applyTx(150000))
	require.Error(t, err)
	assert.Zero(t, h.processPaid, "ProcessPaid never runs when the invoice write fails")
}

func TestApplyPaymentTxWriteErrorsPropagate(t *testing.T) {
	t.Run("update", func(t *testing.T) {
		h := newFixture(t, unpaidInvoice())
		h.addPendingTx("INV-202607-000001-01", 150000)
		h.txRepo().UpdateFn = func(ctx context.Context, tr *domain.Transaction) error {
			return apperr.Internal(errors.New("write failed"))
		}
		require.Error(t, h.svc.ApplyPayment(context.Background(), 10, applyTx(150000)))
		assert.Zero(t, h.processPaid)
	})
	t.Run("create", func(t *testing.T) {
		h := newFixture(t, unpaidInvoice())
		h.txRepo().CreateFn = func(ctx context.Context, tr *domain.Transaction) error {
			return apperr.Internal(errors.New("write failed"))
		}
		require.Error(t, h.svc.ApplyPayment(context.Background(), 10, applyTx(150000)))
		assert.Zero(t, h.processPaid)
	})
}

func TestCallbackUnknownCheckStatusIsExternalError(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{StatusCode: "99"}, nil
	}
	err := h.svc.HandleCallback(context.Background(), callbackPayload("INV-202607-000001-01"))
	require.Error(t, err)
	assert.Equal(t, apperr.CodeExternal, apperr.From(err).Code)
	assert.Zero(t, h.processPaid)
}

func TestCallbackCancelledOnSettledTxIsNoop(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	mo := "INV-202607-000001-01"
	h.txs = append(h.txs, domain.Transaction{
		ID: 60, InvoiceID: 10, Gateway: domain.GatewayDuitku,
		MerchantOrderID: &mo, Status: domain.TxFailed,
	})
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{StatusCode: ports.TxStatusCancelled}, nil
	}
	require.NoError(t, h.svc.HandleCallback(context.Background(), callbackPayload(mo)))
	assert.Equal(t, domain.TxFailed, h.tx(60).Status, "settled tx untouched")
}

func TestCallbackFailedResultOnSettledTxIsNoop(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	mo := "INV-202607-000001-01"
	h.txs = append(h.txs, domain.Transaction{
		ID: 61, InvoiceID: 10, Gateway: domain.GatewayDuitku,
		MerchantOrderID: &mo, Status: domain.TxExpired,
	})
	p := callbackPayload(mo)
	p.ResultCode = "01"
	require.NoError(t, h.svc.HandleCallback(context.Background(), p))
	assert.Equal(t, domain.TxExpired, h.tx(61).Status)
}

func TestCallbackUsesPaymentCodeAndCallbackAmountFallback(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		// Gateway omits amount + reference: fall back to the callback values.
		return &ports.TxStatus{StatusCode: ports.TxStatusSuccess}, nil
	}
	p := callbackPayload("INV-202607-000001-01")
	p.PaymentCode = "OV"
	require.NoError(t, h.svc.HandleCallback(context.Background(), p))

	got := h.tx(tr.ID)
	assert.Equal(t, domain.TxSuccess, got.Status)
	assert.Equal(t, "OV", got.MethodCode, "paymentCode from callback wins")
	assert.Equal(t, int64(150000), got.Amount, "amount parsed from callback form value")
	assert.Equal(t, "DREF-1", got.GatewayReference, "reference from callback fallback")
	assert.Equal(t, 1, h.processPaid)
}

func TestCallbackWithoutIntegrationLoggerStillRejects(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.addPendingTx("INV-202607-000001-01", 150000)
	h.svc.d.IntLog = nil
	h.gateway.VerifyCallbackSignatureFn = func(p ports.CallbackPayload) bool { return false }

	err := h.svc.HandleCallback(context.Background(), callbackPayload("INV-202607-000001-01"))
	require.ErrorIs(t, err, ErrBadSignature)
}

func TestPayWithCreditFullyCoveredByAppliedCredit(t *testing.T) {
	inv := unpaidInvoice()
	inv.CreditApplied = inv.Total // remaining 0
	h := newFixture(t, inv)

	res, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "credit")
	require.NoError(t, err)
	assert.Equal(t, "paid", res.Status)
	assert.Zero(t, res.Amount)
	assert.Empty(t, h.credits.calls, "no deduction when nothing remains")
	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	assert.Equal(t, 1, h.processPaid)
}

func TestPayWithCreditLockErrors(t *testing.T) {
	t.Run("repo error", func(t *testing.T) {
		h := newFixture(t, unpaidInvoice())
		h.invoiceRepo().GetByIDForUpdateFn = func(ctx context.Context, id int64) (*domain.Invoice, error) {
			return nil, apperr.Internal(errors.New("lock timeout"))
		}
		_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "credit")
		require.Error(t, err)
	})
	t.Run("nil invoice", func(t *testing.T) {
		h := newFixture(t, unpaidInvoice())
		h.invoiceRepo().GetByIDForUpdateFn = func(ctx context.Context, id int64) (*domain.Invoice, error) {
			return nil, nil
		}
		_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "credit")
		require.Error(t, err)
		assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
	})
	t.Run("status raced to paid inside tx", func(t *testing.T) {
		h := newFixture(t, unpaidInvoice())
		h.invoiceRepo().GetByIDForUpdateFn = func(ctx context.Context, id int64) (*domain.Invoice, error) {
			c := *h.invoice
			c.Status = domain.InvoicePaid
			return &c, nil
		}
		_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "credit")
		require.Error(t, err)
		assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
		assert.Empty(t, h.credits.calls)
	})
	t.Run("items error", func(t *testing.T) {
		h := newFixture(t, unpaidInvoice())
		h.invoiceRepo().GetItemsFn = func(ctx context.Context, invoiceID int64) ([]domain.InvoiceItem, error) {
			return nil, apperr.Internal(errors.New("db down"))
		}
		_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "credit")
		require.Error(t, err)
		assert.Empty(t, h.credits.calls)
	})
}

func TestPayInvoiceGetInvoiceErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.invoiceRepo().GetByIDFn = func(ctx context.Context, id int64) (*domain.Invoice, error) {
		return nil, apperr.Internal(errors.New("db down"))
	}
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeInternal, apperr.From(err).Code)
}

func TestPayInvoiceNilClientNotFound(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.svc.d.Clients.(*mocks.MockClientRepo).GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) {
		return nil, nil
	}
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeNotFound, apperr.From(err).Code)
}

func TestPayInvoiceClientRepoErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.svc.d.Clients.(*mocks.MockClientRepo).GetByIDFn = func(ctx context.Context, id int64) (*domain.Client, error) {
		return nil, apperr.Internal(errors.New("db down"))
	}
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.Error(t, err)
}

func TestPayInvoiceUserLookupFailureMeansEmptyEmail(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.svc.d.Users.(*mocks.MockUserRepo).GetByIDFn = func(ctx context.Context, id int64) (*domain.User, error) {
		return nil, apperr.Internal(errors.New("db down"))
	}
	var gotEmail string
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		gotEmail = req.Email
		return &ports.CreateTxResult{Reference: "R", PaymentURL: "u"}, nil
	}
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.NoError(t, err, "missing user email must not block payment")
	assert.Empty(t, gotEmail)
}

func TestPayInvoiceNextAttemptErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.txRepo().NextAttemptFn = func(ctx context.Context, invoiceID int64) (int64, error) {
		return 0, apperr.Internal(errors.New("db down"))
	}
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.Error(t, err)
}

func TestPayInvoiceZeroRemainingConflict(t *testing.T) {
	inv := unpaidInvoice()
	inv.CreditApplied = inv.Total
	h := newFixture(t, inv)
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.Error(t, err)
	assert.Equal(t, apperr.CodeConflict, apperr.From(err).Code)
}

func TestPayInvoiceTxCreateErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.gateway.CreateTransactionFn = func(ctx context.Context, req ports.CreateTxRequest) (*ports.CreateTxResult, error) {
		return &ports.CreateTxResult{Reference: "R", PaymentURL: "u"}, nil
	}
	h.txRepo().CreateFn = func(ctx context.Context, tr *domain.Transaction) error {
		return apperr.Internal(errors.New("write failed"))
	}
	_, err := h.svc.PayInvoice(context.Background(), 7, 5, 10, "VA")
	require.Error(t, err)
}

func TestGetPaymentMethodsGatewayErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.gateway.GetPaymentMethodsFn = func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
		return nil, apperr.External("duitku", errors.New("down"))
	}
	_, err := h.svc.GetPaymentMethods(context.Background(), 5, 10)
	require.Error(t, err)
	assert.Equal(t, apperr.CodeExternal, apperr.From(err).Code)
}

func TestGetPaymentMethodsWithoutCache(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.svc.d.Cache = nil
	h.gateway.GetPaymentMethodsFn = func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
		return []ports.PaymentMethod{{Code: "VA"}}, nil
	}
	methods, err := h.svc.GetPaymentMethods(context.Background(), 5, 10)
	require.NoError(t, err)
	assert.Len(t, methods, 1)
}

func TestGetPaymentMethodsCacheSetFailureIsNonFatal(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.cache.SetJSONFn = func(ctx context.Context, key string, val any, ttl time.Duration) error {
		return errors.New("redis down")
	}
	h.gateway.GetPaymentMethodsFn = func(ctx context.Context, amount int64) ([]ports.PaymentMethod, error) {
		return []ports.PaymentMethod{{Code: "VA"}}, nil
	}
	methods, err := h.svc.GetPaymentMethods(context.Background(), 5, 10)
	require.NoError(t, err)
	assert.Len(t, methods, 1)
}

func TestResolveReturnRepoErrorFallsBack(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.txRepo().GetByMerchantOrderIDFn = func(ctx context.Context, mo string) (*domain.Transaction, error) {
		return nil, apperr.Internal(errors.New("db down"))
	}
	assert.Equal(t, "http://front.test/billing",
		h.svc.ResolveReturn(context.Background(), "INV-202607-000001-01"))
}

func TestReconcileOneCheckErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return nil, apperr.External("duitku", errors.New("timeout"))
	}
	require.Error(t, h.svc.ReconcileOne(context.Background(), tr.ID))
	assert.Equal(t, domain.TxPending, h.tx(tr.ID).Status)
}

func TestReconcileOneUsesStoredAmountWhenCheckOmitsIt(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	tr := h.addPendingTx("INV-202607-000001-01", 150000)
	h.gateway.CheckTransactionFn = func(ctx context.Context, mo string) (*ports.TxStatus, error) {
		return &ports.TxStatus{StatusCode: ports.TxStatusSuccess}, nil
	}
	require.NoError(t, h.svc.ReconcileOne(context.Background(), tr.ID))
	assert.Equal(t, domain.InvoicePaid, h.invoice.Status)
	assert.Equal(t, int64(150000), h.tx(tr.ID).Amount)
}

func TestReconcilePendingListErrorPropagates(t *testing.T) {
	h := newFixture(t, unpaidInvoice())
	h.txRepo().ListPendingFn = func(ctx context.Context, gw domain.Gateway, olderThan time.Time) ([]domain.Transaction, error) {
		return nil, apperr.Internal(errors.New("db down"))
	}
	_, err := h.svc.ReconcilePending(context.Background())
	require.Error(t, err)
}

func TestNowFallsBackWithoutClock(t *testing.T) {
	svc := New(Deps{})
	got := svc.now()
	assert.WithinDuration(t, time.Now().UTC(), got, 5*time.Second)
}
