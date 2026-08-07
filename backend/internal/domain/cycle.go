package domain

import "time"

// CycleMonths returns the number of months in one billing cycle.
// one_time returns 0.
func CycleMonths(c BillingCycle) int {
	switch c {
	case CycleMonthly:
		return 1
	case CycleQuarterly:
		return 3
	case CycleSemiannually:
		return 6
	case CycleAnnually:
		return 12
	case CycleBiennially:
		return 24
	default: // one_time or unknown
		return 0
	}
}

// AddCycle advances date by one billing cycle. Month-end dates clamp to the
// last day of the target month (Jan 31 + monthly = Feb 28/29), matching
// billing expectations rather than time.AddDate's normalization overflow.
// one_time returns the date unchanged.
func AddCycle(date time.Time, c BillingCycle) time.Time {
	months := CycleMonths(c)
	if months == 0 {
		return date
	}
	return addMonthsClamped(date, months)
}

func addMonthsClamped(t time.Time, months int) time.Time {
	y, m, d := t.Date()
	// First day of target month.
	first := time.Date(y, m+time.Month(months), 1, 0, 0, 0, 0, t.Location())
	// Clamp day-of-month to target month's length.
	last := daysIn(first.Year(), first.Month())
	if d > last {
		d = last
	}
	hh, mm, ss := t.Clock()
	return time.Date(first.Year(), first.Month(), d, hh, mm, ss, t.Nanosecond(), t.Location())
}

func daysIn(year int, month time.Month) int {
	// Day 0 of next month is the last day of this month.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
