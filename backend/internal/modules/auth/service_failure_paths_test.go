package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/modules/auth"
	"github.com/tsdlamongan/whcms/backend/internal/platform/crypto"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errBoom = errors.New("boom")

// Register error paths

func TestRegisterHasherError(t *testing.T) {
	e := newFixture()
	e.hasher.HashFn = func(string) (string, error) { return "", errBoom }
	_, err := e.service().Register(ctx, validRegister())
	assertCode(t, err, apperr.CodeInternal)
}

func TestRegisterGetByEmailRepoError(t *testing.T) {
	e := newFixture()
	e.users.GetByEmailFn = func(context.Context, string) (*domain.User, error) {
		return nil, apperr.Internal(errBoom)
	}
	_, err := e.service().Register(ctx, validRegister())
	assertCode(t, err, apperr.CodeInternal)
}

func TestRegisterClientCreateError(t *testing.T) {
	e := newFixture()
	e.clients.CreateFn = func(context.Context, *domain.Client) error { return errBoom }
	_, err := e.service().Register(ctx, validRegister())
	assertCode(t, err, apperr.CodeInternal)
}

func TestRegisterIssueTokenError(t *testing.T) {
	e := newFixture()
	e.tokens.issueErr = errBoom
	_, err := e.service().Register(ctx, validRegister())
	assertCode(t, err, apperr.CodeInternal)
}

func TestRegisterNotifyErrorIgnored(t *testing.T) {
	e := newFixture()
	e.notify.SendTemplateFn = func(context.Context, int64, string, map[string]any) error {
		return errBoom
	}
	_, err := e.service().Register(ctx, validRegister())
	require.NoError(t, err)
}

// Login error paths

func TestLoginGetByEmailRepoError(t *testing.T) {
	e := newFixture()
	e.users.GetByEmailFn = func(context.Context, string) (*domain.User, error) {
		return nil, apperr.Internal(errBoom)
	}
	_, err := e.service().Login(ctx, login("user@example.com", "correct-password"))
	assertCode(t, err, apperr.CodeInternal)
}

func TestLoginVerifyError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.hasher.VerifyFn = func(string, string) (bool, error) { return false, errBoom }
	_, err := e.service().Login(ctx, login("user@example.com", "correct-password"))
	assertCode(t, err, apperr.CodeInternal)
}

func TestLoginDecryptError(t *testing.T) {
	e := newFixture()
	u := activeUser()
	u.TwoFAEnabled = true
	u.TwoFASecretEnc = "broken"
	withUser(e, u)
	e.enc.DecryptFn = func(string) (string, error) { return "", errBoom }
	in := login("user@example.com", "correct-password")
	in.TOTPCode = "123456"
	_, err := e.service().Login(ctx, in)
	assertCode(t, err, apperr.CodeInternal)
}

func TestLoginSetLastLoginError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	withClient(e, &domain.Client{ID: 77, UserID: 10})
	e.users.SetLastLoginFn = func(context.Context, int64, time.Time) error {
		return apperr.Internal(errBoom)
	}
	_, err := e.service().Login(ctx, login("user@example.com", "correct-password"))
	assertCode(t, err, apperr.CodeInternal)
}

func TestLoginClientLookupError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.clients.GetByUserIDFn = func(context.Context, int64) (*domain.Client, error) {
		return nil, apperr.Internal(errBoom)
	}
	_, err := e.service().Login(ctx, login("user@example.com", "correct-password"))
	assertCode(t, err, apperr.CodeInternal)
}

func TestLoginClientProfileMissingStillLogsIn(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.clients.GetByUserIDFn = func(context.Context, int64) (*domain.Client, error) {
		return nil, apperr.NotFound("client")
	}
	resp, err := e.service().Login(ctx, login("user@example.com", "correct-password"))
	require.NoError(t, err)
	assert.Equal(t, int64(0), resp.User.ClientID)
}

func TestLoginRefreshCreateError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.tokens.createErr = errBoom
	_, err := e.service().Login(ctx, login("user@example.com", "correct-password"))
	assertCode(t, err, apperr.CodeInternal)
}

// Refresh / Logout error paths

func TestRefreshUserGone(t *testing.T) {
	e := newFixture()
	e.tokens.consumeID = 42
	e.users.GetByIDFn = func(context.Context, int64) (*domain.User, error) {
		return nil, apperr.NotFound("user")
	}
	_, err := e.service().Refresh(ctx, "tok")
	assertCode(t, err, apperr.CodeUnauthorized)
}

func TestRefreshUserNilWithoutError(t *testing.T) {
	e := newFixture() // MockUserRepo.GetByID nil fn -> (nil, nil)
	e.tokens.consumeID = 42
	_, err := e.service().Refresh(ctx, "tok")
	assertCode(t, err, apperr.CodeUnauthorized)
}

func TestLogoutRevokeError(t *testing.T) {
	e := newFixture()
	e.tokens.revokeErr = errBoom
	err := e.service().Logout(ctx, 42, "tok")
	assertCode(t, err, apperr.CodeInternal)
}

// Verify / reset error paths

func TestVerifyEmailConsumeInternalError(t *testing.T) {
	e := newFixture()
	e.onetime.ConsumeFn = func(context.Context, string, string) (int64, error) {
		return 0, apperr.Internal(errBoom)
	}
	err := e.service().VerifyEmail(ctx, "tok")
	assertCode(t, err, apperr.CodeInternal)
}

func TestVerifyEmailSetVerifiedError(t *testing.T) {
	e := newFixture()
	e.onetime.ConsumeFn = func(context.Context, string, string) (int64, error) { return 10, nil }
	e.users.SetEmailVerifiedFn = func(context.Context, int64, time.Time) error {
		return apperr.NotFound("user")
	}
	err := e.service().VerifyEmail(ctx, "tok")
	assertCode(t, err, apperr.CodeNotFound)
}

func TestResendVerificationTokenErrorStillOK(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.onetime.CreateFn = func(context.Context, string, int64, time.Duration) (string, error) {
		return "", errors.New("redis down")
	}
	require.NoError(t, e.service().ResendVerification(ctx, "user@example.com"))
	assert.Empty(t, e.sent)
}

func TestForgotPasswordTokenErrorStillOK(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.onetime.CreateFn = func(context.Context, string, int64, time.Duration) (string, error) {
		return "", errors.New("redis down")
	}
	require.NoError(t, e.service().ForgotPassword(ctx, "user@example.com"))
	assert.Empty(t, e.sent)
}

func TestResetPasswordErrors(t *testing.T) {
	e := newFixture()
	e.onetime.ConsumeFn = func(context.Context, string, string) (int64, error) { return 10, nil }
	e.hasher.HashFn = func(string) (string, error) { return "", errBoom }
	err := e.service().ResetPassword(ctx, auth.ResetPasswordRequest{Token: "t", Password: "longenough1"})
	assertCode(t, err, apperr.CodeInternal)

	e = newFixture()
	e.onetime.ConsumeFn = func(context.Context, string, string) (int64, error) { return 10, nil }
	e.users.UpdatePasswordFn = func(context.Context, int64, string) error {
		return apperr.NotFound("user")
	}
	err = e.service().ResetPassword(ctx, auth.ResetPasswordRequest{Token: "t", Password: "longenough1"})
	assertCode(t, err, apperr.CodeNotFound)

	e = newFixture()
	e.onetime.ConsumeFn = func(context.Context, string, string) (int64, error) {
		return 0, apperr.Internal(errBoom)
	}
	err = e.service().ResetPassword(ctx, auth.ResetPasswordRequest{Token: "t", Password: "longenough1"})
	assertCode(t, err, apperr.CodeInternal)
}

// Me / UpdateMe error paths

func TestMeNilUserWithoutError(t *testing.T) {
	e := newFixture() // (nil, nil) mock default
	_, err := e.service().Me(ctx, 5)
	assertCode(t, err, apperr.CodeNotFound)
}

func TestMeClientLookupError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.clients.GetByUserIDFn = func(context.Context, int64) (*domain.Client, error) {
		return nil, apperr.Internal(errBoom)
	}
	_, err := e.service().Me(ctx, 10)
	assertCode(t, err, apperr.CodeInternal)
}

func TestUpdateMeUserNotFound(t *testing.T) {
	e := newFixture()
	_, err := e.service().UpdateMe(ctx, 5, auth.UpdateMeRequest{Locale: strPtr("en")})
	assertCode(t, err, apperr.CodeNotFound)
}

func TestUpdateMeVerifyError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.hasher.VerifyFn = func(string, string) (bool, error) { return false, errBoom }
	_, err := e.service().UpdateMe(ctx, 10, auth.UpdateMeRequest{
		CurrentPassword: "correct-password", NewPassword: "longenough1",
	})
	assertCode(t, err, apperr.CodeInternal)
}

func TestUpdateMeHashError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.hasher.HashFn = func(string) (string, error) { return "", errBoom }
	_, err := e.service().UpdateMe(ctx, 10, auth.UpdateMeRequest{
		CurrentPassword: "correct-password", NewPassword: "longenough1",
	})
	assertCode(t, err, apperr.CodeInternal)
}

func TestUpdateMeUpdatePasswordError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.users.UpdatePasswordFn = func(context.Context, int64, string) error {
		return apperr.Internal(errBoom)
	}
	_, err := e.service().UpdateMe(ctx, 10, auth.UpdateMeRequest{
		CurrentPassword: "correct-password", NewPassword: "longenough1",
	})
	assertCode(t, err, apperr.CodeInternal)
}

func TestUpdateMeUserUpdateError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.users.UpdateFn = func(context.Context, *domain.User) error { return apperr.Internal(errBoom) }
	_, err := e.service().UpdateMe(ctx, 10, auth.UpdateMeRequest{Locale: strPtr("en")})
	assertCode(t, err, apperr.CodeInternal)
}

func TestUpdateMeClientUpdateError(t *testing.T) {
	e := newFixture()
	u := activeUser()
	withUser(e, u)
	withClient(e, &domain.Client{ID: 77, UserID: u.ID})
	e.clients.UpdateFn = func(context.Context, *domain.Client) error { return apperr.Internal(errBoom) }
	_, err := e.service().UpdateMe(ctx, 10, auth.UpdateMeRequest{FirstName: strPtr("X")})
	assertCode(t, err, apperr.CodeInternal)
}

// 2FA error paths + default TOTP hooks

func TestTwoFASetupGenerateError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	svc := auth.New(auth.Deps{
		Users: e.users, Clients: e.clients, Tx: e.tx, Hasher: e.hasher,
		Tokens: e.tokens, OneTime: e.onetime, Notify: e.notify,
		Limiter: e.limiter, Audit: e.audit, Clock: e.clock, Encryptor: e.enc,
		GenerateTOTP: func(string, string) (string, string, error) { return "", "", errBoom },
	})
	_, err := svc.TwoFASetup(ctx, 10)
	assertCode(t, err, apperr.CodeInternal)
}

func TestTwoFASetupEncryptError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.enc.EncryptFn = func(string) (string, error) { return "", errBoom }
	_, err := e.service().TwoFASetup(ctx, 10)
	assertCode(t, err, apperr.CodeInternal)
}

func TestTwoFASetupUpdateError(t *testing.T) {
	e := newFixture()
	withUser(e, activeUser())
	e.users.UpdateFn = func(context.Context, *domain.User) error { return apperr.Internal(errBoom) }
	_, err := e.service().TwoFASetup(ctx, 10)
	assertCode(t, err, apperr.CodeInternal)
}

func TestTwoFAEnableDecryptError(t *testing.T) {
	e := newFixture()
	u := activeUser()
	u.TwoFASecretEnc = "SECRET"
	withUser(e, u)
	e.enc.DecryptFn = func(string) (string, error) { return "", errBoom }
	err := e.service().TwoFAEnable(ctx, 10, auth.TwoFAEnableRequest{Code: "123456"})
	assertCode(t, err, apperr.CodeInternal)
}

func TestTwoFAEnableUpdateError(t *testing.T) {
	e := newFixture()
	u := activeUser()
	u.TwoFASecretEnc = "SECRET"
	withUser(e, u)
	e.users.UpdateFn = func(context.Context, *domain.User) error { return apperr.Internal(errBoom) }
	err := e.service().TwoFAEnable(ctx, 10, auth.TwoFAEnableRequest{Code: "123456"})
	assertCode(t, err, apperr.CodeInternal)
}

func TestTwoFADisableErrors(t *testing.T) {
	e := newFixture()
	u := activeUser()
	u.TwoFAEnabled = true
	u.TwoFASecretEnc = "SECRET"
	withUser(e, u)
	e.hasher.VerifyFn = func(string, string) (bool, error) { return false, errBoom }
	err := e.service().TwoFADisable(ctx, 10, auth.TwoFADisableRequest{Password: "x", Code: "123456"})
	assertCode(t, err, apperr.CodeInternal)

	e = newFixture()
	withUser(e, u)
	e.enc.DecryptFn = func(string) (string, error) { return "", errBoom }
	err = e.service().TwoFADisable(ctx, 10, auth.TwoFADisableRequest{Password: "correct-password", Code: "123456"})
	assertCode(t, err, apperr.CodeInternal)

	e = newFixture()
	withUser(e, u)
	e.users.UpdateFn = func(context.Context, *domain.User) error { return apperr.Internal(errBoom) }
	err = e.service().TwoFADisable(ctx, 10, auth.TwoFADisableRequest{Password: "correct-password", Code: "123456"})
	assertCode(t, err, apperr.CodeInternal)

	e = newFixture()
	err = e.service().TwoFADisable(ctx, 10, auth.TwoFADisableRequest{Password: "x", Code: "12"})
	assertCode(t, err, apperr.CodeValidation)
}

// TestTwoFADefaultHooks exercises the real pquerna/otp-backed defaults filled
// by New(): setup generates a working secret and enable accepts a code
// computed for it.
func TestTwoFADefaultHooks(t *testing.T) {
	e := newFixture()
	u := activeUser()
	withUser(e, u)
	var stored *domain.User
	e.users.UpdateFn = func(_ context.Context, uu *domain.User) error {
		cp := *uu
		stored = &cp
		return nil
	}

	svc := auth.New(auth.Deps{
		Users: e.users, Clients: e.clients, Tx: e.tx, Hasher: e.hasher,
		Tokens: e.tokens, OneTime: e.onetime, Notify: e.notify,
		Limiter: e.limiter, Audit: e.audit, Clock: e.clock, Encryptor: e.enc,
		// TOTPIssuer/GenerateTOTP/ValidateTOTP intentionally nil -> defaults.
	})

	resp, err := svc.TwoFASetup(ctx, u.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Secret)
	assert.Contains(t, resp.OTPAuthURL, "otpauth://totp/WHCMS")
	require.NotNil(t, stored)

	// The MockEncryptor passes through, so the stored secret equals resp.Secret.
	u.TwoFASecretEnc = stored.TwoFASecretEnc
	code, err := crypto.TOTPCodeAt(resp.Secret, time.Now())
	require.NoError(t, err)
	require.NoError(t, svc.TwoFAEnable(ctx, u.ID, auth.TwoFAEnableRequest{Code: code}))
	assert.True(t, stored.TwoFAEnabled)
}

// Impersonate error paths

func TestImpersonateUserFetchError(t *testing.T) {
	e := newFixture()
	withClient(e, &domain.Client{ID: 55, UserID: 10})
	e.users.GetByIDFn = func(context.Context, int64) (*domain.User, error) {
		return nil, apperr.Internal(errBoom)
	}
	_, err := e.service().Impersonate(ctx, 1, 55, "")
	assertCode(t, err, apperr.CodeInternal)
}

func TestImpersonateIssueError(t *testing.T) {
	e := newFixture()
	u := activeUser()
	withUser(e, u)
	withClient(e, &domain.Client{ID: 55, UserID: u.ID})
	e.tokens.issueErr = errBoom
	_, err := e.service().Impersonate(ctx, 1, 55, "")
	assertCode(t, err, apperr.CodeInternal)
	assert.Empty(t, e.audit.Entries, "failed impersonation is not audited")
}
