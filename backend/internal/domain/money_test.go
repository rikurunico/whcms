package domain_test

import (
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestFormatIDR(t *testing.T) {
	tests := []struct {
		amount int64
		want   string
	}{
		{0, "Rp0,-"},
		{1, "Rp1,-"},
		{999, "Rp999,-"},
		{1000, "Rp1.000,-"},
		{10500, "Rp10.500,-"},
		{100000, "Rp100.000,-"},
		{1234567, "Rp1.234.567,-"},
		{25000000, "Rp25.000.000,-"},
		{1000000000000, "Rp1.000.000.000.000,-"},
		{-1, "-Rp1,-"},
		{-1234567, "-Rp1.234.567,-"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, domain.FormatIDR(tt.amount), "amount=%d", tt.amount)
	}
}
