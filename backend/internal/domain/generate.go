package domain

import (
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"math/big"
	"strconv"
	"strings"
)

const (
	pwLower   = "abcdefghijkmnopqrstuvwxyz" // no 'l'
	pwUpper   = "ABCDEFGHJKLMNPQRSTUVWXYZ"  // no 'I', 'O'
	pwDigits  = "23456789"                  // no '0', '1'
	pwSymbols = "!@#$%^&*"
)

// GeneratePassword returns a cryptographically random strong password of the
// given length (minimum 12 enforced) containing at least one lowercase,
// uppercase, digit and symbol character. It returns an error only if the system
// CSPRNG (crypto/rand) is unavailable - never on a healthy host - so callers in
// the request path can surface it instead of the process panicking.
func GeneratePassword(length int) (string, error) {
	if length < 12 {
		length = 12
	}
	all := pwLower + pwUpper + pwDigits + pwSymbols
	buf := make([]byte, length)
	// Guarantee one of each class.
	var err error
	if buf[0], err = randByte(pwLower); err != nil {
		return "", err
	}
	if buf[1], err = randByte(pwUpper); err != nil {
		return "", err
	}
	if buf[2], err = randByte(pwDigits); err != nil {
		return "", err
	}
	if buf[3], err = randByte(pwSymbols); err != nil {
		return "", err
	}
	for i := 4; i < length; i++ {
		if buf[i], err = randByte(all); err != nil {
			return "", err
		}
	}
	// Shuffle (Fisher–Yates with crypto/rand).
	for i := len(buf) - 1; i > 0; i-- {
		j, err := randInt(i + 1)
		if err != nil {
			return "", err
		}
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf), nil
}

func randByte(set string) (byte, error) {
	i, err := randInt(len(set))
	if err != nil {
		return 0, err
	}
	return set[i], nil
}

func randInt(n int) (int, error) {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, fmt.Errorf("domain: crypto/rand unavailable: %w", err)
	}
	return int(v.Int64()), nil
}

// UsernameFromDomain derives a panel username from a domain name:
// lowercase, strips the TLD and all non-alphanumerics, ensures it starts with
// a letter and truncates to 8 characters (cPanel-safe). Empty/degenerate
// inputs yield a random letter-prefixed fallback.
func UsernameFromDomain(domain string) string {
	d := strings.ToLower(strings.TrimSpace(domain))
	d = strings.TrimPrefix(d, "www.")
	if i := strings.IndexByte(d, '.'); i >= 0 {
		d = d[:i]
	}
	var b strings.Builder
	for _, r := range d {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	u := b.String()
	if u == "" || u[0] >= '0' && u[0] <= '9' {
		u = "u" + u
	}
	if len(u) > 8 {
		u = u[:8]
	}
	return u
}

// DisambiguateUsername returns a variant of base suffixed with a short tag
// derived from base+serviceID, staying within the 8-character cPanel-safe
// budget. Two services whose domains differ only past the first label
// (e.g. "acme.com" and "acme.id") collide on the same UsernameFromDomain
// result; this gives the loser a distinct name to retry panel account
// creation with. Deterministic on (base, serviceID) - retrying the exact
// same rejected candidate always proposes the same next one - but folding
// the CURRENT base into the tag (not just serviceID) means escalating again
// on an already-disambiguated candidate that itself got rejected (a real
// panel's reserved-username list can reject more than one truncated variant
// of the same domain - confirmed against a live WHM) produces a genuinely
// new candidate instead of converging back to the identical string forever.
func DisambiguateUsername(base string, serviceID int64) string {
	h := fnv.New32a()
	fmt.Fprintf(h, "%s:%d", base, serviceID)
	suffix := strconv.FormatUint(uint64(h.Sum32())%1000, 10)
	if max := 8 - len(suffix); len(base) > max {
		base = base[:max]
	}
	return base + suffix
}
