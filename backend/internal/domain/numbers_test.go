package domain_test

import (
	"testing"
	"time"

	"github.com/tsdlamongan/whcms/backend/internal/domain"

	"github.com/stretchr/testify/assert"
)

var july2026 = time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)

func TestNumberFormats(t *testing.T) {
	assert.Equal(t, "INV-202607-000001", domain.FormatInvoiceNumber(july2026, 1))
	assert.Equal(t, "INV-202607-123456", domain.FormatInvoiceNumber(july2026, 123456))
	assert.Equal(t, "INV-202607-1234567", domain.FormatInvoiceNumber(july2026, 1234567), "overflow keeps digits")
	assert.Equal(t, "ORD-202607-000042", domain.FormatOrderNumber(july2026, 42))
	assert.Equal(t, "TKT-000007", domain.FormatTicketNumber(7))
}

func TestNumberFormatsUseUTCMonth(t *testing.T) {
	// 2026-07-31 23:00 Jakarta is 2026-07-31 16:00 UTC - same month, but
	// 2026-08-01 02:00 Jakarta is 2026-07-31 19:00 UTC: format must follow UTC.
	jakarta := time.FixedZone("WIB", 7*3600)
	local := time.Date(2026, 8, 1, 2, 0, 0, 0, jakarta)
	assert.Equal(t, "INV-202607-000001", domain.FormatInvoiceNumber(local, 1))
}

func TestCounterScopes(t *testing.T) {
	assert.Equal(t, "invoice:202607", domain.InvoiceCounterScope(july2026))
	assert.Equal(t, "order:202607", domain.OrderCounterScope(july2026))
	assert.Equal(t, "ticket", domain.TicketCounterScope)
}
