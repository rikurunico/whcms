package domain_test

import (
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"

	"github.com/stretchr/testify/assert"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func TestCycleMonths(t *testing.T) {
	tests := []struct {
		cycle domain.BillingCycle
		want  int
	}{
		{domain.CycleOneTime, 0},
		{domain.CycleMonthly, 1},
		{domain.CycleQuarterly, 3},
		{domain.CycleSemiannually, 6},
		{domain.CycleAnnually, 12},
		{domain.CycleBiennially, 24},
		{domain.BillingCycle("bogus"), 0},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, domain.CycleMonths(tt.cycle), string(tt.cycle))
	}
}

func TestAddCycle(t *testing.T) {
	tests := []struct {
		name  string
		start time.Time
		cycle domain.BillingCycle
		want  time.Time
	}{
		{"monthly simple", date(2026, 7, 3), domain.CycleMonthly, date(2026, 8, 3)},
		{"quarterly", date(2026, 7, 3), domain.CycleQuarterly, date(2026, 10, 3)},
		{"semiannually", date(2026, 7, 3), domain.CycleSemiannually, date(2027, 1, 3)},
		{"annually", date(2026, 7, 3), domain.CycleAnnually, date(2027, 7, 3)},
		{"biennially", date(2026, 7, 3), domain.CycleBiennially, date(2028, 7, 3)},
		{"one_time unchanged", date(2026, 7, 3), domain.CycleOneTime, date(2026, 7, 3)},
		{"jan31 monthly clamps to feb28", date(2026, 1, 31), domain.CycleMonthly, date(2026, 2, 28)},
		{"jan31 monthly leap year clamps to feb29", date(2028, 1, 31), domain.CycleMonthly, date(2028, 2, 29)},
		{"aug31 monthly clamps to sep30", date(2026, 8, 31), domain.CycleMonthly, date(2026, 9, 30)},
		{"nov30 quarterly stays 28 feb", date(2026, 11, 30), domain.CycleQuarterly, date(2027, 2, 28)},
		{"feb29 annually clamps to feb28", date(2028, 2, 29), domain.CycleAnnually, date(2029, 2, 28)},
		{"dec crosses year", date(2026, 12, 15), domain.CycleMonthly, date(2027, 1, 15)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, domain.AddCycle(tt.start, tt.cycle))
		})
	}
}

func TestAddCyclePreservesClock(t *testing.T) {
	start := time.Date(2026, 7, 3, 13, 45, 30, 0, time.UTC)
	got := domain.AddCycle(start, domain.CycleMonthly)
	assert.Equal(t, time.Date(2026, 8, 3, 13, 45, 30, 0, time.UTC), got)
}

func TestBillingCycleValid(t *testing.T) {
	assert.True(t, domain.CycleMonthly.Valid())
	assert.True(t, domain.CycleOneTime.Valid())
	assert.False(t, domain.BillingCycle("weekly").Valid())
}

func TestUserRoleValid(t *testing.T) {
	assert.True(t, domain.RoleAdmin.Valid())
	assert.True(t, domain.RoleClient.Valid())
	assert.False(t, domain.UserRole("root").Valid())
}
