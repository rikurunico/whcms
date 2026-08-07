// Package clock implements ports.Clock. Inject it everywhere time matters so
// tests can freeze time.
package clock

import "time"

// Real is the production ports.Clock backed by time.Now (UTC).
type Real struct{}

// New builds the real clock.
func New() Real { return Real{} }

// Now returns the current UTC time.
func (Real) Now() time.Time { return time.Now().UTC() }

// Fixed is a test ports.Clock that always returns T.
type Fixed struct {
	T time.Time
}

// Now returns the fixed time.
func (f Fixed) Now() time.Time { return f.T }
