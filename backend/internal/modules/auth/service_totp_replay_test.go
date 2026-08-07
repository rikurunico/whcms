package auth_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/modules/auth"
	"github.com/tsdlamongan/whcms/backend/internal/platform/crypto"
	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"
	"github.com/tsdlamongan/whcms/backend/internal/platform/totpguard"
	"github.com/tsdlamongan/whcms/backend/pkg/apperr"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func testRedisForAuth(t *testing.T) *redis.Client {
	t.Helper()
	rdb, err := redisx.Connect(context.Background(), "localhost:6379", "", 15) // isolated test DB
	if err != nil {
		t.Skipf("redis not available, skipping: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

// TestLoginTOTPReplayIntegration is the end-to-end regression test for the
// TOTP replay vulnerability: it drives the real Login flow (real TOTP
// generation/validation from platform/crypto, real Redis-backed
// TOTPReplayGuard) rather than mocking the 2FA hooks, so it proves the wiring
// - not just each piece in isolation.
func TestLoginTOTPReplayIntegration(t *testing.T) {
	rdb := testRedisForAuth(t)
	guard := totpguard.New(rdb)

	e := newFixture()
	// Use the real crypto TOTP hooks (leave GenerateTOTP/ValidateTOTP/
	// ValidateTOTPStep unset so auth.New defaults to platform/crypto) and a
	// real Redis-backed guard instead of the in-memory mock.
	secret, _, err := crypto.GenerateTOTPSecret("WHCMS", "user@example.com")
	require.NoError(t, err)

	u := activeUser()
	u.ID = int64(uuid.New().ID()) // unique per run to avoid cross-test key collisions
	u.TwoFAEnabled = true
	u.TwoFASecretEnc = secret // MockEncryptor passes secrets through unchanged
	withUser(e, u)
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), redisx.Key("totp", "step", fmt.Sprintf("%d", u.ID))).Err()
	})

	svc := auth.New(auth.Deps{
		Users:       e.users,
		Clients:     e.clients,
		Tx:          e.tx,
		Hasher:      e.hasher,
		Tokens:      e.tokens,
		OneTime:     e.onetime,
		Notify:      e.notify,
		Limiter:     e.limiter,
		Audit:       e.audit,
		Clock:       e.clock,
		Encryptor:   e.enc,
		TOTPGuard:   guard,
		FrontendURL: "http://fe",
		TOTPIssuer:  "TestPanel",
	})

	code, err := crypto.TOTPCodeAt(secret, time.Now())
	require.NoError(t, err)

	in := login("user@example.com", "correct-password")
	in.TOTPCode = code

	// First use of the code succeeds.
	_, err = svc.Login(context.Background(), in)
	require.NoError(t, err, "first use of a valid TOTP code must succeed")

	// Replaying the exact same code immediately afterwards must be rejected,
	// even though the code is still within its normal ±1 skew validity window.
	_, err = svc.Login(context.Background(), in)
	require.Error(t, err, "replaying the same TOTP code must be rejected")
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	require.Equal(t, apperr.CodeUnauthorized, appErr.Code)

	// A genuinely new code for a later step must still succeed - replay
	// protection must not lock the user out of subsequent logins. One period
	// (30s) ahead is a later step but still within the current ±1 skew
	// window, so it validates as "now" without needing to fake the clock.
	laterCode, err := crypto.TOTPCodeAt(secret, time.Now().Add(30*time.Second))
	require.NoError(t, err)
	in2 := login("user@example.com", "correct-password")
	in2.TOTPCode = laterCode
	_, err = svc.Login(context.Background(), in2)
	require.NoError(t, err, "a new code for a later step must still be accepted")
}
