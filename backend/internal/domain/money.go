package domain

import "strconv"

// FormatIDR formats whole-rupiah int64 amounts as "Rp1.234.567,-" - the
// canonical app-wide Rupiah nominal (matches the frontend $lib/money.formatIDR).
// Negative amounts render as "-Rp1.234.567,-".
func FormatIDR(amount int64) string {
	neg := amount < 0
	if neg {
		amount = -amount
	}
	digits := strconv.FormatInt(amount, 10)

	// Insert '.' thousands separators from the right, then a trailing ",-".
	n := len(digits)
	groups := (n - 1) / 3
	out := make([]byte, 0, n+groups+5)
	if neg {
		out = append(out, '-')
	}
	out = append(out, 'R', 'p')
	lead := n - groups*3
	out = append(out, digits[:lead]...)
	for i := lead; i < n; i += 3 {
		out = append(out, '.')
		out = append(out, digits[i:i+3]...)
	}
	out = append(out, ',', '-')
	return string(out)
}
