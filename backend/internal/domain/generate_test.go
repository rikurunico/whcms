package domain_test

import (
	"strings"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePassword(t *testing.T) {
	classes := func(s string) (lower, upper, digit, symbol bool) {
		for _, r := range s {
			switch {
			case r >= 'a' && r <= 'z':
				lower = true
			case r >= 'A' && r <= 'Z':
				upper = true
			case r >= '0' && r <= '9':
				digit = true
			case strings.ContainsRune("!@#$%^&*", r):
				symbol = true
			}
		}
		return
	}

	t.Run("length and character classes", func(t *testing.T) {
		for range 50 {
			pw, err := domain.GeneratePassword(16)
			require.NoError(t, err)
			assert.Len(t, pw, 16)
			lo, up, di, sy := classes(pw)
			assert.True(t, lo, "missing lowercase in %q", pw)
			assert.True(t, up, "missing uppercase in %q", pw)
			assert.True(t, di, "missing digit in %q", pw)
			assert.True(t, sy, "missing symbol in %q", pw)
		}
	})

	t.Run("minimum length enforced", func(t *testing.T) {
		pw4, err := domain.GeneratePassword(4)
		require.NoError(t, err)
		assert.Len(t, pw4, 12)
		pw0, err := domain.GeneratePassword(0)
		require.NoError(t, err)
		assert.Len(t, pw0, 12)
	})

	t.Run("unique across calls", func(t *testing.T) {
		seen := map[string]bool{}
		for range 100 {
			pw, err := domain.GeneratePassword(16)
			require.NoError(t, err)
			assert.False(t, seen[pw], "duplicate password generated")
			seen[pw] = true
		}
	})
}

func TestUsernameFromDomain(t *testing.T) {
	tests := []struct {
		domain string
		want   string
	}{
		{"example.com", "example"},
		{"www.example.com", "example"},
		{"EXAMPLE.COM", "example"},
		{"my-cool-site.co.id", "mycoolsi"},
		{"toko123.id", "toko123"},
		{"123shop.com", "u123shop"},
		{"verylongdomainname.com", "verylong"},
		{"a.b.c", "a"},
		{"", "u"},
		{"---.com", "u"},
		{"  spaced.com  ", "spaced"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, domain.UsernameFromDomain(tt.domain), "domain=%q", tt.domain)
	}
}

func TestDisambiguateUsername(t *testing.T) {
	tests := []struct {
		base      string
		serviceID int64
	}{
		{"whcms", 82},
		{"example1", 42},
		{"verylong", 7},
		{"a", 999},
	}
	for _, tt := range tests {
		got := domain.DisambiguateUsername(tt.base, tt.serviceID)
		assert.LessOrEqual(t, len(got), 8, "base=%q serviceID=%d", tt.base, tt.serviceID)
		assert.Equal(t, got, domain.DisambiguateUsername(tt.base, tt.serviceID),
			"must be deterministic for the same (base, serviceID)")
	}
}

// TestDisambiguateUsername_EscalatesOnRepeatedCollision reproduces a real
// incident against a live WHM: a service's UsernameFromDomain candidate
// ("testhost") was rejected as a reserved username, disambiguated to
// "testh103", which a real WHM reserved-username list ALSO rejected - and
// the old serviceID-only suffix scheme made DisambiguateUsername a fixed
// point (truncating "testh103" to 5 chars gives back "testh", the same
// prefix, plus the identical serviceID-derived suffix), so every further
// retry proposed the exact same doomed "testh103" forever.
func TestDisambiguateUsername_EscalatesOnRepeatedCollision(t *testing.T) {
	base := "testhost"
	serviceID := int64(103)

	first := domain.DisambiguateUsername(base, serviceID)
	second := domain.DisambiguateUsername(first, serviceID)

	assert.NotEqual(t, first, second,
		"escalating on an already-disambiguated candidate that collided again must propose a new candidate, not the same string forever")
	assert.LessOrEqual(t, len(second), 8)
}
