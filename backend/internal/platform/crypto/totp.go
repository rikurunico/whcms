package crypto

import (
	"fmt"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// totpStepCheckOpts checks a single exact time-step (Skew: 0). ValidateTOTPStep
// loops over the candidate steps itself (current, current±1) to reproduce
// totp.Validate's ±1-period-skew default; using Skew: 0 per candidate is
// deliberate - with Skew: 1 here, totp.ValidateCustom would additionally
// check candidate±1 *inside* each call, so a match could actually belong to
// a neighboring step and get misreported as the candidate's step.
var totpStepCheckOpts = totp.ValidateOpts{
	Period:    30,
	Skew:      0,
	Digits:    otp.DigitsSix,
	Algorithm: otp.AlgorithmSHA1,
}

// GenerateTOTPSecret creates a new TOTP secret for a user. Returns the base32
// secret and the otpauth:// provisioning URL (QR-encodable).
func GenerateTOTPSecret(issuer, accountEmail string) (secret, url string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountEmail,
	})
	if err != nil {
		return "", "", fmt.Errorf("crypto: totp generate: %w", err)
	}
	return key.Secret(), key.URL(), nil
}

// ValidateTOTP checks a 6-digit code against the secret at time now (allows
// the default ±1 period skew).
func ValidateTOTP(code, secret string) bool {
	return totp.Validate(code, secret)
}

// ValidateTOTPStep checks a 6-digit code against the secret using the same
// ±1 period-skew window as ValidateTOTP, but additionally reports which
// 30-second time-step (Unix time / 30) the code matched. Callers that need
// anti-replay protection (e.g. login) can reject a step that was already
// accepted for the same user. ok is false when no step in the skew window
// matches, in which case matchedStep is 0 and must be ignored.
func ValidateTOTPStep(code, secret string) (matchedStep int64, ok bool) {
	now := time.Now().UTC()
	current := now.Unix() / int64(totpStepCheckOpts.Period)
	// Check the current step first, then the ±1 skew steps - order doesn't
	// affect correctness (steps are disjoint) but keeps the common case fast.
	for _, step := range [...]int64{current, current - 1, current + 1} {
		at := time.Unix(step*int64(totpStepCheckOpts.Period), 0).UTC()
		valid, err := totp.ValidateCustom(code, secret, at, totpStepCheckOpts)
		if err != nil {
			continue
		}
		if valid {
			return step, true
		}
	}
	return 0, false
}

// TOTPCodeAt computes the code for a secret at a specific time (test helper
// for the auth module).
func TOTPCodeAt(secret string, at time.Time) (string, error) {
	code, err := totp.GenerateCode(secret, at)
	if err != nil {
		return "", fmt.Errorf("crypto: totp code: %w", err)
	}
	return code, nil
}
