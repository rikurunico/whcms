package duitku

import (
	"sync"
	"time"
)

const (
	// breakerThreshold is the number of consecutive transport/5xx failures
	// that opens the circuit.
	breakerThreshold = 3
	// breakerCooldown is how long the circuit stays open (fast-fail) after
	// the failure that tripped it.
	breakerCooldown = 60 * time.Second
)

// breaker is a circuit-breaker-lite: a consecutive-failure counter that
// fast-fails calls for breakerCooldown once breakerThreshold consecutive
// failures accumulate. Any success closes the circuit and resets the counter.
type breaker struct {
	mu          sync.Mutex
	consecutive int
	openUntil   time.Time
}

func newBreaker() *breaker { return &breaker{} }

// allow reports whether a call may proceed at the given time.
func (b *breaker) allow(now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !now.Before(b.openUntil)
}

// success closes the circuit and resets the failure counter.
func (b *breaker) success() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutive = 0
	b.openUntil = time.Time{}
}

// failure records one failed call; on reaching the threshold the circuit
// opens until now + breakerCooldown.
func (b *breaker) failure(now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.consecutive++
	if b.consecutive >= breakerThreshold {
		b.openUntil = now.Add(breakerCooldown)
	}
}
