package mocks_test

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"
	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errSentinel = errors.New("sentinel")

func TestMockMailer(t *testing.T) {
	ctx := context.Background()
	msg := ports.MailMessage{To: "a@example.com"}

	// Fn set: delegates and does not touch Sent.
	var got ports.MailMessage
	m := &mocks.MockMailer{SendFn: func(ctx context.Context, msg ports.MailMessage) error {
		got = msg
		return errSentinel
	}}
	err := m.Send(ctx, msg)
	assert.Equal(t, errSentinel, err)
	assert.Equal(t, msg, got)
	assert.Empty(t, m.Sent)

	// Fn nil: records into Sent, returns nil.
	var z mocks.MockMailer
	require.NoError(t, z.Send(ctx, msg))
	require.Len(t, z.Sent, 1)
	assert.Equal(t, msg, z.Sent[0])
}

func TestMockStorage(t *testing.T) {
	ctx := context.Background()

	m := &mocks.MockStorage{
		PutFn: func(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
			return errSentinel
		},
		GetFn:    func(ctx context.Context, key string) (io.ReadCloser, error) { return nil, errSentinel },
		DeleteFn: func(ctx context.Context, key string) error { return errSentinel },
		PresignGetFn: func(ctx context.Context, key string, ttl time.Duration, opts ...ports.PresignOption) (string, error) {
			return "url", nil
		},
	}
	assert.Equal(t, errSentinel, m.Put(ctx, "k", nil, 0, "text/plain"))
	_, err := m.Get(ctx, "k")
	assert.Equal(t, errSentinel, err)
	assert.Equal(t, errSentinel, m.Delete(ctx, "k"))
	url, err := m.PresignGet(ctx, "k", time.Minute)
	require.NoError(t, err)
	assert.Equal(t, "url", url)

	// Opts are forwarded through to PresignGetFn.
	var gotOpts ports.PresignOptions
	m.PresignGetFn = func(ctx context.Context, key string, ttl time.Duration, opts ...ports.PresignOption) (string, error) {
		for _, o := range opts {
			o(&gotOpts)
		}
		return "url2", nil
	}
	url, err = m.PresignGet(ctx, "k", time.Minute,
		ports.WithResponseContentType("text/plain"),
		ports.WithResponseContentDisposition(`attachment; filename="a.txt"`))
	require.NoError(t, err)
	assert.Equal(t, "url2", url)
	assert.Equal(t, "text/plain", gotOpts.ResponseContentType)
	assert.Equal(t, `attachment; filename="a.txt"`, gotOpts.ResponseContentDisposition)

	var z mocks.MockStorage
	assert.NoError(t, z.Put(ctx, "k", nil, 0, "text/plain"))
	rc, err := z.Get(ctx, "k")
	assert.NoError(t, err)
	assert.Nil(t, rc)
	assert.NoError(t, z.Delete(ctx, "k"))
	zurl, err := z.PresignGet(ctx, "k", time.Minute)
	assert.NoError(t, err)
	assert.Empty(t, zurl)
}

func TestMockEncryptor(t *testing.T) {
	m := &mocks.MockEncryptor{
		EncryptFn: func(p string) (string, error) { return "enc:" + p, nil },
		DecryptFn: func(c string) (string, error) { return "dec:" + c, nil },
	}
	enc, err := m.Encrypt("plain")
	require.NoError(t, err)
	assert.Equal(t, "enc:plain", enc)
	dec, err := m.Decrypt("cipher")
	require.NoError(t, err)
	assert.Equal(t, "dec:cipher", dec)

	// Fn nil: pass-through unchanged.
	var z mocks.MockEncryptor
	enc, err = z.Encrypt("plain")
	require.NoError(t, err)
	assert.Equal(t, "plain", enc)
	dec, err = z.Decrypt("cipher")
	require.NoError(t, err)
	assert.Equal(t, "cipher", dec)
}

func TestMockPasswordHasher(t *testing.T) {
	m := &mocks.MockPasswordHasher{
		HashFn:   func(p string) (string, error) { return "custom:" + p, nil },
		VerifyFn: func(p, phc string) (bool, error) { return p == "right", nil },
	}
	h, err := m.Hash("pw")
	require.NoError(t, err)
	assert.Equal(t, "custom:pw", h)
	ok, err := m.Verify("right", "anything")
	require.NoError(t, err)
	assert.True(t, ok)

	// Fn nil: default reversible "hashed:" scheme.
	var z mocks.MockPasswordHasher
	h, err = z.Hash("pw")
	require.NoError(t, err)
	assert.Equal(t, "hashed:pw", h)

	ok, err = z.Verify("pw", "hashed:pw")
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = z.Verify("pw", "wrong")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestMockTokenStore(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockTokenStore{
		CreateFn: func(ctx context.Context, kind string, userID int64, ttl time.Duration) (string, error) {
			return "tok", nil
		},
		ConsumeFn: func(ctx context.Context, kind, token string) (int64, error) { return 42, nil },
	}
	tok, err := m.Create(ctx, "reset", 1, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, "tok", tok)
	uid, err := m.Consume(ctx, "reset", "tok")
	require.NoError(t, err)
	assert.EqualValues(t, 42, uid)

	var z mocks.MockTokenStore
	tok, err = z.Create(ctx, "reset", 1, time.Hour)
	require.NoError(t, err)
	assert.Empty(t, tok)
	uid, err = z.Consume(ctx, "reset", "tok")
	require.NoError(t, err)
	assert.Zero(t, uid)
}

func TestMockEnqueuer(t *testing.T) {
	ctx := context.Background()
	var gotOpts []ports.JobOption
	m := &mocks.MockEnqueuer{EnqueueFn: func(ctx context.Context, taskType string, payload any, opts ...ports.JobOption) error {
		gotOpts = opts
		return errSentinel
	}}
	err := m.Enqueue(ctx, "send_email", map[string]string{"a": "b"})
	assert.Equal(t, errSentinel, err)
	assert.Empty(t, gotOpts)
	assert.Empty(t, m.Tasks)

	// Fn nil: records into Tasks.
	var z mocks.MockEnqueuer
	require.NoError(t, z.Enqueue(ctx, "send_email", 7))
	require.Len(t, z.Tasks, 1)
	assert.Equal(t, "send_email", z.Tasks[0].TaskType)
	assert.Equal(t, 7, z.Tasks[0].Payload)
}

func TestMockLocker(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockLocker{WithLockFn: func(ctx context.Context, key string, ttl time.Duration, fn func() error) error {
		return errSentinel
	}}
	err := m.WithLock(ctx, "lock:1", time.Second, func() error { return nil })
	assert.Equal(t, errSentinel, err)

	// Fn nil: runs fn directly and returns its error.
	var z mocks.MockLocker
	ran := false
	err = z.WithLock(ctx, "lock:1", time.Second, func() error { ran = true; return nil })
	assert.NoError(t, err)
	assert.True(t, ran)

	err = z.WithLock(ctx, "lock:1", time.Second, func() error { return errSentinel })
	assert.Equal(t, errSentinel, err)
}

func TestMockRateLimiter(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockRateLimiter{
		AllowFn: func(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
			return false, errSentinel
		},
		ResetFn: func(ctx context.Context, key string) error { return errSentinel },
	}
	ok, err := m.Allow(ctx, "k", 1, time.Second)
	assert.Equal(t, errSentinel, err)
	assert.False(t, ok)
	assert.Equal(t, errSentinel, m.Reset(ctx, "k"))

	// Fn nil: allow everything, reset succeeds.
	var z mocks.MockRateLimiter
	ok, err = z.Allow(ctx, "k", 1, time.Second)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.NoError(t, z.Reset(ctx, "k"))
}

func TestMockCache(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockCache{
		GetJSONFn: func(ctx context.Context, key string, out any) (bool, error) { return true, nil },
		SetJSONFn: func(ctx context.Context, key string, val any, ttl time.Duration) error { return errSentinel },
		DeleteFn:  func(ctx context.Context, key string) error { return errSentinel },
	}
	var out string
	hit, err := m.GetJSON(ctx, "k", &out)
	require.NoError(t, err)
	assert.True(t, hit)
	assert.Equal(t, errSentinel, m.SetJSON(ctx, "k", "v", time.Minute))
	assert.Equal(t, errSentinel, m.Delete(ctx, "k"))

	// Fn nil: always a miss, Set/Delete succeed.
	var z mocks.MockCache
	hit, err = z.GetJSON(ctx, "k", &out)
	require.NoError(t, err)
	assert.False(t, hit)
	assert.NoError(t, z.SetJSON(ctx, "k", "v", time.Minute))
	assert.NoError(t, z.Delete(ctx, "k"))
}

func TestMockAuditLogger(t *testing.T) {
	ctx := context.Background()
	called := false
	m := &mocks.MockAuditLogger{LogFn: func(ctx context.Context, actorUserID int64, action, entity string, entityID int64, before, after any) {
		called = true
	}}
	m.Log(ctx, 1, "update", "client", 2, nil, nil)
	assert.True(t, called)
	assert.Empty(t, m.Entries)

	// Fn nil: appends an AuditEntry.
	var z mocks.MockAuditLogger
	z.Log(ctx, 1, "update", "client", 2, "before", "after")
	require.Len(t, z.Entries, 1)
	e := z.Entries[0]
	assert.EqualValues(t, 1, e.ActorUserID)
	assert.Equal(t, "update", e.Action)
	assert.Equal(t, "client", e.Entity)
	assert.EqualValues(t, 2, e.EntityID)
	assert.Equal(t, "before", e.Before)
	assert.Equal(t, "after", e.After)
}

func TestMockIntegrationLogger(t *testing.T) {
	ctx := context.Background()
	called := false
	m := &mocks.MockIntegrationLogger{LogFn: func(ctx context.Context, call ports.IntegrationCall) {
		called = true
	}}
	m.Log(ctx, ports.IntegrationCall{Provider: "duitku"})
	assert.True(t, called)
	assert.Empty(t, m.Calls)

	var z mocks.MockIntegrationLogger
	z.Log(ctx, ports.IntegrationCall{Provider: "duitku"})
	require.Len(t, z.Calls, 1)
	assert.Equal(t, "duitku", z.Calls[0].Provider)
}

func TestMockClock(t *testing.T) {
	fixed := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)

	// NowFn set takes priority.
	m := &mocks.MockClock{
		NowFn:     func() time.Time { return fixed.Add(time.Hour) },
		FixedTime: fixed,
	}
	assert.Equal(t, fixed.Add(time.Hour), m.Now())

	// NowFn nil, FixedTime set: returns FixedTime.
	f := &mocks.MockClock{FixedTime: fixed}
	assert.Equal(t, fixed, f.Now())

	// Both zero: falls back to time.Now().
	var z mocks.MockClock
	before := time.Now()
	got := z.Now()
	assert.WithinDuration(t, before, got, 2*time.Second)
}

func TestMockPDFGenerator(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockPDFGenerator{InvoicePDFFn: func(ctx context.Context, inv ports.InvoicePDFData) ([]byte, error) {
		return []byte("custom"), nil
	}}
	b, err := m.InvoicePDF(ctx, ports.InvoicePDFData{})
	require.NoError(t, err)
	assert.Equal(t, "custom", string(b))

	var z mocks.MockPDFGenerator
	b, err = z.InvoicePDF(ctx, ports.InvoicePDFData{})
	require.NoError(t, err)
	assert.Contains(t, string(b), "%PDF")
}

func TestMockInvoiceCreator(t *testing.T) {
	ctx := context.Background()
	inv := &domain.Invoice{ID: 1}
	m := &mocks.MockInvoiceCreator{CreateInvoiceFn: func(ctx context.Context, in ports.CreateInvoiceInput) (*domain.Invoice, error) {
		return inv, nil
	}}
	got, err := m.CreateInvoice(ctx, ports.CreateInvoiceInput{})
	require.NoError(t, err)
	assert.Same(t, inv, got)

	var z mocks.MockInvoiceCreator
	got, err = z.CreateInvoice(ctx, ports.CreateInvoiceInput{})
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestMockPaidInvoiceProcessor(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockPaidInvoiceProcessor{ProcessPaidFn: func(ctx context.Context, invoiceID int64) error { return errSentinel }}
	assert.Equal(t, errSentinel, m.ProcessPaid(ctx, 1))

	var z mocks.MockPaidInvoiceProcessor
	assert.NoError(t, z.ProcessPaid(ctx, 1))
}

func TestMockServiceRenewer(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockServiceRenewer{
		RenewServiceFn: func(ctx context.Context, serviceID int64) error { return errSentinel },
		ApplyUpgradeFn: func(ctx context.Context, serviceID int64) error { return errSentinel },
	}
	assert.Equal(t, errSentinel, m.RenewService(ctx, 1))
	assert.Equal(t, errSentinel, m.ApplyUpgrade(ctx, 1))

	var z mocks.MockServiceRenewer
	assert.NoError(t, z.RenewService(ctx, 1))
	assert.NoError(t, z.ApplyUpgrade(ctx, 1))
}

func TestMockDomainRenewer(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockDomainRenewer{RenewDomainAfterPaymentFn: func(ctx context.Context, domainID int64) error { return errSentinel }}
	assert.Equal(t, errSentinel, m.RenewDomainAfterPayment(ctx, 1))

	var z mocks.MockDomainRenewer
	assert.NoError(t, z.RenewDomainAfterPayment(ctx, 1))
}

func TestMockPaymentApplier(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockPaymentApplier{ApplyPaymentFn: func(ctx context.Context, invoiceID int64, tx ports.ApplyTx) error { return errSentinel }}
	assert.Equal(t, errSentinel, m.ApplyPayment(ctx, 1, ports.ApplyTx{}))

	var z mocks.MockPaymentApplier
	assert.NoError(t, z.ApplyPayment(ctx, 1, ports.ApplyTx{}))
}

func TestMockServiceActivator(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockServiceActivator{ActivateOrderFn: func(ctx context.Context, orderID int64) error { return errSentinel }}
	assert.Equal(t, errSentinel, m.ActivateOrder(ctx, 1))

	var z mocks.MockServiceActivator
	assert.NoError(t, z.ActivateOrder(ctx, 1))
}

func TestMockNotificationSender(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockNotificationSender{
		SendTemplateFn: func(ctx context.Context, userID int64, templateKey string, data map[string]any) error {
			return errSentinel
		},
		AlertAdminFn: func(ctx context.Context, subject, message string) error { return errSentinel },
	}
	assert.Equal(t, errSentinel, m.SendTemplate(ctx, 1, "welcome", nil))
	assert.Equal(t, errSentinel, m.AlertAdmin(ctx, "subj", "msg"))

	var z mocks.MockNotificationSender
	assert.NoError(t, z.SendTemplate(ctx, 1, "welcome", nil))
	assert.NoError(t, z.AlertAdmin(ctx, "subj", "msg"))
}

func TestMockTxManager(t *testing.T) {
	ctx := context.Background()
	m := &mocks.MockTxManager{WithinTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
		return errSentinel
	}}
	err := m.WithinTx(ctx, func(ctx context.Context) error { return nil })
	assert.Equal(t, errSentinel, err)

	// Fn nil: runs fn(ctx) directly and returns its error.
	var z mocks.MockTxManager
	ran := false
	err = z.WithinTx(ctx, func(ctx context.Context) error { ran = true; return nil })
	assert.NoError(t, err)
	assert.True(t, ran)

	err = z.WithinTx(ctx, func(ctx context.Context) error { return errSentinel })
	assert.Equal(t, errSentinel, err)
}
