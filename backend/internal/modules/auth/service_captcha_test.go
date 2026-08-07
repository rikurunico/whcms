package auth_test

import (
	"context"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeCaptcha implements auth.CaptchaGuard.
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

func TestRegisterCaptchaRejected(t *testing.T) {
	e := newFixture()
	cap := &fakeCaptcha{err: apperr.Validation("captcha verification failed")}
	e.captcha = cap
	created := false
	e.users.CreateFn = func(context.Context, *domain.User) error { created = true; return nil }

	in := validRegister()
	in.CaptchaToken = "bad"
	in.IP = "9.9.9.9"
	_, err := e.service().Register(ctx, in)

	assertCode(t, err, apperr.CodeValidation)
	assert.Equal(t, 1, cap.calls)
	assert.Equal(t, "bad", cap.lastToken)
	assert.Equal(t, "9.9.9.9", cap.lastIP)
	assert.False(t, created, "user must not be created when captcha fails")
}

func TestRegisterCaptchaPassthrough(t *testing.T) {
	e := newFixture()
	cap := &fakeCaptcha{}
	e.captcha = cap
	e.users.CreateFn = func(_ context.Context, u *domain.User) error { u.ID = 10; return nil }
	e.clients.CreateFn = func(_ context.Context, c *domain.Client) error { c.ID = 7; return nil }

	in := validRegister()
	in.CaptchaToken = "ok"
	_, err := e.service().Register(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, 1, cap.calls)
	assert.Equal(t, "ok", cap.lastToken)
}

func TestLoginCaptchaRejected(t *testing.T) {
	e := newFixture()
	cap := &fakeCaptcha{err: apperr.Validation("captcha verification failed")}
	e.captcha = cap
	u := activeUser()
	withUser(e, u)
	withClient(e, &domain.Client{ID: 77, UserID: u.ID})

	in := login("user@example.com", "correct-password")
	in.CaptchaToken = "bad"
	_, err := e.service().Login(ctx, in)

	assertCode(t, err, apperr.CodeValidation)
	assert.Equal(t, 1, cap.calls)
	assert.Empty(t, e.tokens.issued, "no token issued when captcha fails")
}

func TestLoginCaptchaPassthrough(t *testing.T) {
	e := newFixture()
	cap := &fakeCaptcha{}
	e.captcha = cap
	u := activeUser()
	withUser(e, u)
	withClient(e, &domain.Client{ID: 77, UserID: u.ID})

	in := login("user@example.com", "correct-password")
	in.CaptchaToken = "ok"
	_, err := e.service().Login(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, 1, cap.calls)
	assert.Equal(t, "1.2.3.4", cap.lastIP)
}
