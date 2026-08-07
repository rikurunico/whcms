package domain_test

import (
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestCalcTaxExclusive(t *testing.T) {
	tests := []struct {
		name      string
		amount    int64
		rate      float64
		wantTax   int64
		wantGross int64
	}{
		{"11% of 100000", 100000, 11, 11000, 111000},
		{"11% of 99999 rounds", 99999, 11, 11000, 110999}, // 10999.89 -> 11000
		{"11% of 50 rounds", 50, 11, 6, 56},               // 5.5 -> 6 (half-up)
		{"zero rate", 100000, 0, 0, 100000},
		{"negative rate treated as zero", 100000, -5, 0, 100000},
		{"zero amount", 0, 11, 0, 0},
		{"10% simple", 150000, 10, 15000, 165000},
		{"negative amount (credit note)", -100000, 11, -11000, -111000},
		{"negative rounds half away from zero", -50, 11, -6, -56},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.CalcTax(tt.amount, tt.rate, false)
			assert.Equal(t, tt.amount, got.Base)
			assert.Equal(t, tt.wantTax, got.Tax)
			assert.Equal(t, tt.wantGross, got.Gross)
		})
	}
}

func TestCalcTaxInclusive(t *testing.T) {
	tests := []struct {
		name    string
		amount  int64
		rate    float64
		wantTax int64
	}{
		{"11% inside 111000", 111000, 11, 11000},
		{"11% inside 100000", 100000, 11, 9910}, // 100000*11/111 = 9909.909... -> 9910
		{"10% inside 110000", 110000, 10, 10000},
		{"zero rate", 100000, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.CalcTax(tt.amount, tt.rate, true)
			assert.Equal(t, tt.wantTax, got.Tax)
			assert.Equal(t, tt.amount, got.Gross, "inclusive gross must equal input")
			assert.Equal(t, tt.amount-tt.wantTax, got.Base)
			assert.Equal(t, got.Gross, got.Base+got.Tax, "parts must sum exactly")
		})
	}
}
