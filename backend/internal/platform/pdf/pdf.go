// Package pdf implements ports.PDFGenerator using go-pdf/fpdf: a clean
// invoice layout with company header, client address block, items table,
// totals and a status stamp.
package pdf

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/tsdlamongan/whcms/backend/internal/domain"
	"github.com/tsdlamongan/whcms/backend/internal/ports"

	"github.com/go-pdf/fpdf"
)

// Generator renders invoice PDFs.
type Generator struct{}

// New builds a Generator.
func New() *Generator { return &Generator{} }

const (
	pageMargin = 15.0
	colDesc    = 130.0
	colAmount  = 50.0
)

// InvoicePDF renders inv into PDF bytes.
func (g *Generator) InvoicePDF(_ context.Context, inv ports.InvoicePDFData) ([]byte, error) {
	p := fpdf.New("P", "mm", "A4", "")
	p.SetMargins(pageMargin, pageMargin, pageMargin)
	p.SetAutoPageBreak(true, 20)
	p.AddPage()

	// --- Company header ------------------------------------------------
	p.SetFont("Helvetica", "B", 18)
	p.SetTextColor(20, 27, 45) // dark sidebar tone
	p.CellFormat(0, 9, inv.CompanyName, "", 1, "L", false, 0, "")
	p.SetFont("Helvetica", "", 9)
	p.SetTextColor(90, 90, 90)
	for _, line := range splitLines(inv.CompanyAddr) {
		p.CellFormat(0, 4.5, line, "", 1, "L", false, 0, "")
	}
	if inv.CompanyEmail != "" {
		p.CellFormat(0, 4.5, inv.CompanyEmail, "", 1, "L", false, 0, "")
	}

	// --- Invoice title + status stamp ------------------------------------
	p.SetY(pageMargin)
	p.SetFont("Helvetica", "B", 16)
	p.SetTextColor(51, 102, 153) // WHMCS blue
	p.CellFormat(0, 8, "INVOICE", "", 1, "R", false, 0, "")
	p.SetFont("Helvetica", "", 10)
	p.SetTextColor(60, 60, 60)
	p.CellFormat(0, 5, inv.InvoiceNumber, "", 1, "R", false, 0, "")
	p.CellFormat(0, 5, "Issued: "+inv.IssuedAt.Format("02 Jan 2006"), "", 1, "R", false, 0, "")
	p.CellFormat(0, 5, "Due: "+inv.DueDate.Format("02 Jan 2006"), "", 1, "R", false, 0, "")

	// Status stamp.
	stampR, stampG, stampB := statusColor(inv.Status)
	p.SetFont("Helvetica", "B", 14)
	p.SetTextColor(stampR, stampG, stampB)
	p.SetDrawColor(stampR, stampG, stampB)
	p.SetLineWidth(0.8)
	stamp := strings.ToUpper(string(inv.Status))
	stampW := p.GetStringWidth(stamp) + 8
	x := 210 - pageMargin - stampW
	y := p.GetY() + 3
	p.Rect(x, y, stampW, 10, "D")
	p.SetXY(x, y+2)
	p.CellFormat(stampW, 6, stamp, "", 1, "C", false, 0, "")

	// --- Client address block ---------------------------------------------
	p.SetXY(pageMargin, 62)
	p.SetFont("Helvetica", "B", 10)
	p.SetTextColor(120, 120, 120)
	p.CellFormat(0, 5, "BILLED TO", "", 1, "L", false, 0, "")
	p.SetFont("Helvetica", "B", 11)
	p.SetTextColor(30, 30, 30)
	p.CellFormat(0, 5.5, inv.ClientName, "", 1, "L", false, 0, "")
	p.SetFont("Helvetica", "", 10)
	p.SetTextColor(70, 70, 70)
	if inv.ClientCompany != "" {
		p.CellFormat(0, 5, inv.ClientCompany, "", 1, "L", false, 0, "")
	}
	for _, line := range splitLines(inv.ClientAddr) {
		p.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
	}
	if inv.ClientEmail != "" {
		p.CellFormat(0, 5, inv.ClientEmail, "", 1, "L", false, 0, "")
	}

	// --- Items table -------------------------------------------------------
	p.Ln(6)
	p.SetFont("Helvetica", "B", 10)
	p.SetFillColor(51, 102, 153)
	p.SetTextColor(255, 255, 255)
	p.CellFormat(colDesc, 8, "  Description", "", 0, "L", true, 0, "")
	p.CellFormat(colAmount, 8, "Amount  ", "", 1, "R", true, 0, "")

	p.SetFont("Helvetica", "", 10)
	p.SetTextColor(40, 40, 40)
	fill := false
	for _, item := range inv.Items {
		p.SetFillColor(245, 247, 250)
		p.CellFormat(colDesc, 7, "  "+item.Description, "", 0, "L", fill, 0, "")
		p.CellFormat(colAmount, 7, domain.FormatIDR(item.Amount)+"  ", "", 1, "R", fill, 0, "")
		fill = !fill
	}
	p.SetDrawColor(200, 200, 200)
	p.SetLineWidth(0.2)
	p.Line(pageMargin, p.GetY(), 210-pageMargin, p.GetY())

	// --- Totals -------------------------------------------------------------
	totals := []struct {
		label string
		value int64
		show  bool
	}{
		{"Subtotal", inv.Subtotal, true},
		{"Discount", -inv.Discount, inv.Discount != 0},
		{fmt.Sprintf("Tax (%.2f%%)", inv.TaxRate), inv.TaxTotal, inv.TaxTotal != 0},
		{"Credit applied", -inv.CreditApplied, inv.CreditApplied != 0},
	}
	p.Ln(2)
	for _, row := range totals {
		if !row.show {
			continue
		}
		p.SetFont("Helvetica", "", 10)
		p.CellFormat(colDesc, 6, row.label, "", 0, "R", false, 0, "")
		p.CellFormat(colAmount, 6, domain.FormatIDR(row.value)+"  ", "", 1, "R", false, 0, "")
	}
	p.SetFont("Helvetica", "B", 12)
	p.SetTextColor(51, 102, 153)
	p.CellFormat(colDesc, 8, "TOTAL", "", 0, "R", false, 0, "")
	p.CellFormat(colAmount, 8, domain.FormatIDR(inv.Total)+"  ", "", 1, "R", false, 0, "")

	if inv.PaidAt != nil {
		p.SetFont("Helvetica", "", 9)
		p.SetTextColor(46, 125, 50)
		p.CellFormat(0, 6, "Paid at "+inv.PaidAt.Format("02 Jan 2006 15:04 MST"), "", 1, "R", false, 0, "")
	}

	// --- Notes ----------------------------------------------------------------
	if inv.Notes != "" {
		p.Ln(6)
		p.SetFont("Helvetica", "B", 9)
		p.SetTextColor(120, 120, 120)
		p.CellFormat(0, 5, "NOTES", "", 1, "L", false, 0, "")
		p.SetFont("Helvetica", "", 9)
		p.SetTextColor(70, 70, 70)
		p.MultiCell(0, 4.5, inv.Notes, "", "L", false)
	}

	var buf bytes.Buffer
	if err := p.Output(&buf); err != nil {
		return nil, fmt.Errorf("pdf: render: %w", err)
	}
	return buf.Bytes(), nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}

func statusColor(s domain.InvoiceStatus) (int, int, int) {
	switch s {
	case domain.InvoicePaid:
		return 46, 125, 50 // green
	case domain.InvoiceOverdue:
		return 198, 40, 40 // red
	case domain.InvoiceCancelled, domain.InvoiceRefunded:
		return 117, 117, 117 // gray
	default: // draft, unpaid
		return 249, 168, 37 // amber
	}
}
