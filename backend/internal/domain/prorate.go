package domain

import "time"

// Prorate returns the prorated charge for using a service from `from`
// (inclusive) to `to` (exclusive), where cycleAmount is the price of one full
// billing cycle starting at `from`. Result is rounded half-up to whole IDR.
//
// If to <= from the charge is 0. If to is beyond the cycle end the full
// cycleAmount is returned (no over-charging beyond one cycle). one_time
// cycles always return the full amount.
func Prorate(cycleAmount int64, cycle BillingCycle, from, to time.Time) int64 {
	if CycleMonths(cycle) == 0 {
		return cycleAmount
	}
	if !to.After(from) {
		return 0
	}
	cycleEnd := AddCycle(from, cycle)
	totalDays := daysBetween(from, cycleEnd)
	if totalDays <= 0 {
		return cycleAmount
	}
	usedDays := daysBetween(from, to)
	if usedDays >= totalDays {
		return cycleAmount
	}
	return roundHalfUp(float64(cycleAmount) * float64(usedDays) / float64(totalDays))
}

// daysBetween returns whole calendar days from a to b (date component, UTC-naive).
func daysBetween(a, b time.Time) int {
	ad := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, time.UTC)
	bd := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, time.UTC)
	return int(bd.Sub(ad) / (24 * time.Hour))
}
