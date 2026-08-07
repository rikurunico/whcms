package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters (OWASP-recommended baseline).
const (
	argonTime    uint32 = 3
	argonMemory  uint32 = 64 * 1024 // 64 MiB
	argonThreads uint8  = 2
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

// PasswordHasher is an Argon2id ports.PasswordHasher producing PHC strings:
// $argon2id$v=19$m=65536,t=3,p=2$<salt-b64>$<hash-b64>
type PasswordHasher struct{}

// NewPasswordHasher builds a PasswordHasher.
func NewPasswordHasher() *PasswordHasher { return &PasswordHasher{} }

// Hash derives an Argon2id PHC string from password.
func (h *PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("crypto: salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

// Verify reports whether password matches the PHC string (constant-time
// comparison). Malformed PHC strings return an error.
func (h *PasswordHasher) Verify(password, phc string) (bool, error) {
	parts := strings.Split(phc, "$")
	// ["", "argon2id", "v=19", "m=...,t=...,p=...", salt, hash]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("crypto: malformed argon2id hash")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("crypto: parse version: %w", err)
	}
	if version != argon2.Version {
		return false, fmt.Errorf("crypto: unsupported argon2 version %d", version)
	}
	var mem, iters uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &iters, &threads); err != nil {
		return false, fmt.Errorf("crypto: parse params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("crypto: decode salt: %w", err)
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("crypto: decode hash: %w", err)
	}
	got := argon2.IDKey([]byte(password), salt, iters, mem, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}
