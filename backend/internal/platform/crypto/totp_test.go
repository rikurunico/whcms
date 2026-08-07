package crypto_test

import (
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTOTPRoundTrip(t *testing.T) {
	secret, url, err := crypto.GenerateTOTPSecret("WHCMS", "user@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, secret)
	assert.Contains(t, url, "otpauth://totp/")
	assert.Contains(t, url, "WHCMS")

	code, err := crypto.TOTPCodeAt(secret, time.Now())
	require.NoError(t, err)
	assert.Len(t, code, 6)
	assert.True(t, crypto.ValidateTOTP(code, secret), "current code must validate")
	assert.False(t, crypto.ValidateTOTP("000000", secret), "wrong code must fail")
}

func TestTOTPOldCodeRejected(t *testing.T) {
	secret, _, err := crypto.GenerateTOTPSecret("WHCMS", "user@example.com")
	require.NoError(t, err)
	old, err := crypto.TOTPCodeAt(secret, time.Now().Add(-10*time.Minute))
	require.NoError(t, err)
	assert.False(t, crypto.ValidateTOTP(old, secret))
}

func TestTOTPCodeAtInvalidSecret(t *testing.T) {
	_, err := crypto.TOTPCodeAt("not-base32-!!!!", time.Now())
	assert.Error(t, err)
}

func TestValidateTOTPStepMatchesGeneratedStep(t *testing.T) {
	secret, _, err := crypto.GenerateTOTPSecret("WHCMS", "user@example.com")
	require.NoError(t, err)

	now := time.Now()
	wantStep := now.Unix() / 30
	code, err := crypto.TOTPCodeAt(secret, now)
	require.NoError(t, err)

	gotStep, ok := crypto.ValidateTOTPStep(code, secret)
	require.True(t, ok, "code generated for the current step must validate")
	assert.Equal(t, wantStep, gotStep, "matched step must equal the step the code was generated for")
}

func TestValidateTOTPStepMatchesSkewStep(t *testing.T) {
	secret, _, err := crypto.GenerateTOTPSecret("WHCMS", "user@example.com")
	require.NoError(t, err)

	// One period in the past is still within the library's default ±1 skew.
	past := time.Now().Add(-30 * time.Second)
	wantStep := past.Unix() / 30
	code, err := crypto.TOTPCodeAt(secret, past)
	require.NoError(t, err)

	gotStep, ok := crypto.ValidateTOTPStep(code, secret)
	require.True(t, ok, "code from one period ago must still validate within skew")
	assert.Equal(t, wantStep, gotStep, "matched step must equal the older step, not the current one")
}

func TestValidateTOTPStepRejectsWrongCode(t *testing.T) {
	secret, _, err := crypto.GenerateTOTPSecret("WHCMS", "user@example.com")
	require.NoError(t, err)

	step, ok := crypto.ValidateTOTPStep("000000", secret)
	assert.False(t, ok)
	assert.Zero(t, step)
}

func TestValidateTOTPStepRejectsOldCode(t *testing.T) {
	secret, _, err := crypto.GenerateTOTPSecret("WHCMS", "user@example.com")
	require.NoError(t, err)

	old, err := crypto.TOTPCodeAt(secret, time.Now().Add(-10*time.Minute))
	require.NoError(t, err)

	_, ok := crypto.ValidateTOTPStep(old, secret)
	assert.False(t, ok, "code well outside the skew window must not match any step")
}

func TestGenerateTOTPSecretRequiresIssuerAndAccount(t *testing.T) {
	t.Run("missing issuer", func(t *testing.T) {
		_, _, err := crypto.GenerateTOTPSecret("", "user@example.com")
		assert.Error(t, err)
	})
	t.Run("missing account name", func(t *testing.T) {
		_, _, err := crypto.GenerateTOTPSecret("WHCMS", "")
		assert.Error(t, err)
	})
}
