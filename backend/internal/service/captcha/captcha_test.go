package captcha

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tsdlamongan/whcms/backend/internal/ports/mocks"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"
)

func settingsWith(enabled bool, provider, siteKey string) *mocks.MockSettingsRepo {
	return &mocks.MockSettingsRepo{
		GetBoolFn: func(_ context.Context, key string, def bool) (bool, error) {
			if key == SettingEnabled {
				return enabled, nil
			}
			return def, nil
		},
		GetStringFn: func(_ context.Context, key, def string) (string, error) {
			switch key {
			case SettingProvider:
				return provider, nil
			case SettingSiteKey:
				return siteKey, nil
			}
			return def, nil
		},
	}
}

func TestVerifyDisabledIsNoop(t *testing.T) {
	called := false
	g := New(&mocks.MockCaptchaVerifier{VerifyFn: func(context.Context, string, string) (bool, error) {
		called = true
		return false, nil
	}}, settingsWith(false, "", ""))

	// Disabled: even an empty token passes and the verifier is never hit.
	require.NoError(t, g.Verify(context.Background(), "", "1.2.3.4"))
	assert.False(t, called)
	assert.False(t, g.Enabled(context.Background()))
}

func TestVerifyEnabledEmptyToken(t *testing.T) {
	g := New(&mocks.MockCaptchaVerifier{}, settingsWith(true, "turnstile", "site"))
	err := g.Verify(context.Background(), "  ", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.New(apperr.CodeValidation, ""))
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	require.Len(t, ae.Details, 1)
	assert.Equal(t, "captcha_token", ae.Details[0].Field)
}

func TestVerifyEnabledValidToken(t *testing.T) {
	var gotToken, gotIP string
	g := New(&mocks.MockCaptchaVerifier{VerifyFn: func(_ context.Context, token, ip string) (bool, error) {
		gotToken, gotIP = token, ip
		return true, nil
	}}, settingsWith(true, "turnstile", "site"))

	require.NoError(t, g.Verify(context.Background(), "tok", "9.9.9.9"))
	assert.Equal(t, "tok", gotToken)
	assert.Equal(t, "9.9.9.9", gotIP)
}

func TestVerifyEnabledRejectedToken(t *testing.T) {
	g := New(&mocks.MockCaptchaVerifier{VerifyFn: func(context.Context, string, string) (bool, error) {
		return false, nil
	}}, settingsWith(true, "turnstile", "site"))
	err := g.Verify(context.Background(), "tok", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.New(apperr.CodeValidation, ""))
}

func TestVerifyEnabledProviderError(t *testing.T) {
	g := New(&mocks.MockCaptchaVerifier{VerifyFn: func(context.Context, string, string) (bool, error) {
		return false, apperr.External("turnstile", errors.New("boom"))
	}}, settingsWith(true, "turnstile", "site"))
	err := g.Verify(context.Background(), "tok", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.New(apperr.CodeExternal, ""))
}

func TestVerifyEnabledNoVerifier(t *testing.T) {
	g := New(nil, settingsWith(true, "turnstile", "site"))
	err := g.Verify(context.Background(), "tok", "")
	require.Error(t, err)
	assert.ErrorIs(t, err, apperr.New(apperr.CodeExternal, ""))
}

func TestPublicConfig(t *testing.T) {
	g := New(&mocks.MockCaptchaVerifier{}, settingsWith(true, "turnstile", "1x00000000000000000000AA"))
	cfg := g.PublicConfig(context.Background())
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "turnstile", cfg.Provider)
	assert.Equal(t, "1x00000000000000000000AA", cfg.SiteKey)

	// Blank provider defaults to turnstile.
	g2 := New(&mocks.MockCaptchaVerifier{}, settingsWith(false, "", ""))
	cfg2 := g2.PublicConfig(context.Background())
	assert.False(t, cfg2.Enabled)
	assert.Equal(t, "turnstile", cfg2.Provider)
	assert.Empty(t, cfg2.SiteKey)
}
