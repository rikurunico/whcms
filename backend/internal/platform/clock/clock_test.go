package clock_test

import (
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/platform/clock"

	"github.com/stretchr/testify/assert"
)

func TestRealNowIsUTC(t *testing.T) {
	c := clock.New()
	now := c.Now()
	assert.Equal(t, time.UTC, now.Location())
	assert.WithinDuration(t, time.Now().UTC(), now, 2*time.Second)
}

func TestFixed(t *testing.T) {
	ts := time.Date(2026, 7, 3, 12, 0, 0, 0, time.UTC)
	c := clock.Fixed{T: ts}
	assert.Equal(t, ts, c.Now())
	assert.Equal(t, ts, c.Now(), "stable across calls")
}
