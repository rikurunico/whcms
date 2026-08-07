package domain

import "math"

// TaxResult is the outcome of a tax calculation on a taxable base amount.
// All values are whole IDR.
type TaxResult struct {
	Base  int64 // net (tax-exclusive) amount
	Tax   int64 // tax amount
	Gross int64 // Base + Tax
}

// CalcTax computes tax for a taxable amount at ratePercent (e.g. 11 for 11%).
//
//   - inclusive=false: amount is the net base; tax is added on top.
//   - inclusive=true:  amount already contains tax; the tax portion is
//     extracted (base = amount / (1+rate)).
//
// Amounts are rounded half-up to whole rupiah. For inclusive tax, Gross always
// equals the input amount exactly (base absorbs rounding).
func CalcTax(amount int64, ratePercent float64, inclusive bool) TaxResult {
	if ratePercent <= 0 || amount == 0 {
		return TaxResult{Base: amount, Tax: 0, Gross: amount}
	}
	rate := ratePercent / 100
	if inclusive {
		tax := roundHalfUp(float64(amount) * rate / (1 + rate))
		return TaxResult{Base: amount - tax, Tax: tax, Gross: amount}
	}
	tax := roundHalfUp(float64(amount) * rate)
	return TaxResult{Base: amount, Tax: tax, Gross: amount + tax}
}

func roundHalfUp(v float64) int64 {
	if v < 0 {
		return -int64(math.Floor(-v + 0.5))
	}
	return int64(math.Floor(v + 0.5))
}
