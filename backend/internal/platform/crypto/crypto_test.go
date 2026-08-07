package crypto_test

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/platform/crypto"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes

func TestNewEncryptorKeyLength(t *testing.T) {
	_, err := crypto.NewEncryptor([]byte("short"))
	assert.Error(t, err)
	_, err = crypto.NewEncryptor(testKey)
	assert.NoError(t, err)
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	e, err := crypto.NewEncryptor(testKey)
	require.NoError(t, err)

	vectors := []string{
		"",
		"secret",
		"panel-password-P@ssw0rd!",
		strings.Repeat("long-", 200),
		"unicode: héllo 世界 🎉",
	}
	for _, plain := range vectors {
		ct, err := e.Encrypt(plain)
		require.NoError(t, err)
		assert.NotEqual(t, plain, ct)
		// Output is valid base64.
		_, err = base64.StdEncoding.DecodeString(ct)
		require.NoError(t, err)

		got, err := e.Decrypt(ct)
		require.NoError(t, err)
		assert.Equal(t, plain, got)
	}
}

func TestEncryptIsNonDeterministic(t *testing.T) {
	e, _ := crypto.NewEncryptor(testKey)
	a, _ := e.Encrypt("same input")
	b, _ := e.Encrypt("same input")
	assert.NotEqual(t, a, b, "random nonce must give different ciphertexts")
}

func TestDecryptRejectsBadInput(t *testing.T) {
	e, _ := crypto.NewEncryptor(testKey)

	t.Run("not base64", func(t *testing.T) {
		_, err := e.Decrypt("!!!!")
		assert.Error(t, err)
	})
	t.Run("too short", func(t *testing.T) {
		_, err := e.Decrypt(base64.StdEncoding.EncodeToString([]byte("ab")))
		assert.Error(t, err)
	})
	t.Run("tampered", func(t *testing.T) {
		ct, _ := e.Encrypt("secret")
		raw, _ := base64.StdEncoding.DecodeString(ct)
		raw[len(raw)-1] ^= 0xFF
		_, err := e.Decrypt(base64.StdEncoding.EncodeToString(raw))
		assert.Error(t, err)
	})
	t.Run("wrong key", func(t *testing.T) {
		other, _ := crypto.NewEncryptor([]byte("ffffffffffffffffffffffffffffffff"))
		ct, _ := e.Encrypt("secret")
		_, err := other.Decrypt(ct)
		assert.Error(t, err)
	})
}

func TestPasswordHasherRoundTrip(t *testing.T) {
	h := crypto.NewPasswordHasher()
	phc, err := h.Hash("S3cure-Pass!")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(phc, "$argon2id$v=19$m=65536,t=3,p=2$"), phc)

	ok, err := h.Verify("S3cure-Pass!", phc)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = h.Verify("wrong-password", phc)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPasswordHashUniqueSalt(t *testing.T) {
	h := crypto.NewPasswordHasher()
	a, _ := h.Hash("same")
	b, _ := h.Hash("same")
	assert.NotEqual(t, a, b, "salts must differ")
}

// Known-vector test: PHC string produced with the package parameters
// (m=65536,t=3,p=2) for password "billpanel-test" and fixed salt
// "0123456789abcdef". Guards against accidental parameter drift.
func TestPasswordVerifyKnownVector(t *testing.T) {
	h := crypto.NewPasswordHasher()
	// Generated once with argon2.IDKey([]byte("billpanel-test"),
	// []byte("0123456789abcdef"), 3, 65536, 2, 32).
	const phc = "$argon2id$v=19$m=65536,t=3,p=2$MDEyMzQ1Njc4OWFiY2RlZg$2gcUosd5JiCDb6ztrLr+zTat8CqfyDDWt3dqu8xSWgs"
	ok, err := h.Verify("billpanel-test", phc)
	require.NoError(t, err)
	assert.True(t, ok, "known vector must verify")

	ok, err = h.Verify("not-the-password", phc)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPasswordVerifyMalformed(t *testing.T) {
	h := crypto.NewPasswordHasher()
	cases := []string{
		"",
		"plaintext",
		"$bcrypt$whatever",
		"$argon2id$v=19$m=65536,t=3,p=2$only-five-parts",
		"$argon2id$v=99$m=65536,t=3,p=2$MDEyMw$MDEyMw",    // bad version
		"$argon2id$vv=19$m=65536,t=3,p=2$MDEyMw$MDEyMw",   // unparsable version field
		"$argon2id$v=19$m=65536,t=3,p=2$!!notb64$MDEyMw",  // bad salt
		"$argon2id$v=19$m=65536,t=3,p=2$MDEyMw$!!notb64",  // bad hash
		"$argon2id$v=19$mm=65536,tt=3,pp=2$MDEyMw$MDEyMw", // bad params
	}
	for _, phc := range cases {
		_, err := h.Verify("x", phc)
		assert.Error(t, err, "phc=%q", phc)
	}
}
