package invoicepdf

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestFormatAmount(t *testing.T) {
	assert.Equal(t, "₹3,70,000.00", formatAmount(37000000, "INR"))
	assert.Equal(t, "₹2,499.98", formatAmount(249998, "INR"))
	assert.Equal(t, "₹0.00", formatAmount(0, "INR"))
	assert.Equal(t, "₹999.00", formatAmount(99900, "INR"))
	assert.Equal(t, "₹1,000.00", formatAmount(100000, "INR"))
	assert.Equal(t, "₹12,34,56,789.05", formatAmount(123456789_05, "INR"))
	assert.Equal(t, "$1,000.00", formatAmount(100000, "USD"))
	assert.Equal(t, "$1,234,567.89", formatAmount(123456789, "USD"))
	assert.Equal(t, "€50.00", formatAmount(5000, "EUR"))
	assert.Equal(t, "AED 1,000.00", formatAmount(100000, "AED"))
}

func TestFormatQuantity(t *testing.T) {
	assert.Equal(t, "1", formatQuantity(100))
	assert.Equal(t, "7.5", formatQuantity(750))
	assert.Equal(t, "2.25", formatQuantity(225))
	assert.Equal(t, "0.01", formatQuantity(1))
	assert.Equal(t, "10000", formatQuantity(1000000))
}

func TestFormatDate(t *testing.T) {
	assert.Equal(t, "April 30, 2026", formatDate("2026-04-30"))
	assert.Equal(t, "January 5, 2026", formatDate("2026-01-05"))
	assert.Equal(t, "bad", formatDate("bad"))
}

func TestFileName(t *testing.T) {
	assert.Equal(t, "Invoice_CA_2026-27_01_2026-04-30.pdf", FileName("CA/2026-27/01", "2026-04-30"))
	assert.Equal(t, "Invoice_INV_1_2026-04-30.pdf", FileName("INV 1", "2026-04-30"))
}

func TestBalanceDue(t *testing.T) {
	assert.Equal(t, int64(37000000), balanceDue(&models.Invoice{Total: 37000000}, 0))
	assert.Equal(t, int64(27000000), balanceDue(&models.Invoice{Total: 37000000}, 10000000))
	assert.Equal(t, int64(0), balanceDue(&models.Invoice{Total: 37000000}, 40000000))
	// within the 3% tolerance counts as paid
	assert.Equal(t, int64(0), balanceDue(&models.Invoice{Total: 37000000}, 36000000))
	// closed manually owes nothing even with a large shortfall
	assert.Equal(t, int64(0), balanceDue(&models.Invoice{Total: 37000000, ClosedManually: true}, 100))
}

func testLogoDataUrl(t *testing.T) string {
	img := image.NewRGBA(image.Rect(0, 0, 160, 40))

	for x := 0; x < 160; x++ {
		for y := 0; y < 40; y++ {
			img.Set(x, y, color.RGBA{R: 61, G: 134, B: 245, A: 255})
		}
	}

	var buf bytes.Buffer
	assert.Nil(t, png.Encode(&buf, img))

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func sampleProfile() *models.InvoiceProfile {
	return &models.InvoiceProfile{
		BusinessName: "Rishabh Jain",
		AddressLines: "Codeation\nSector 62, Noida\nUttar Pradesh 201301\nIndia",
		Phone:        "+91 98765 43210",
		Website:      "https://codeation.io",
	}
}

func sampleInvoice() (*models.Invoice, []*models.InvoiceLine) {
	invoice := &models.Invoice{
		Number:             "CA/2026-27/01",
		BillToName:         "Quber Tech",
		BillToContactName:  "Kanika Batra",
		BillToAddressLines: "Plot 12, Udyog Vihar\nGurugram, Haryana 122016",
		BillToEmail:        "accounts@qubertech.com",
		BillToPhone:        "+91 99999 00000",
		BillToPan:          "AAAPB1234C",
		Currency:           "INR",
		InvoiceDate:        "2026-04-01",
		DueDate:            "2026-04-30",
		Notes:              "Please pay by NEFT to the account above.\nThank you for your business.",
		Total:              37000000 + 249998,
	}

	lines := []*models.InvoiceLine{
		{ItemName: "Consulting", Description: "April 2026 retainer\nIncludes architecture reviews", Quantity: 100, Rate: 37000000, Amount: 37000000},
		{ItemName: "Hourly support", Description: "", Quantity: 750, Rate: 33333, Amount: 249998},
	}

	return invoice, lines
}

func TestRenderProducesPdf(t *testing.T) {
	profile := sampleProfile()
	profile.LogoDataUrl = testLogoDataUrl(t)
	invoice, lines := sampleInvoice()

	pdf, err := Render(profile, invoice, lines, 10000000, "https://books.codeation.io/public/invoice/abc")
	assert.Nil(t, err)
	assert.True(t, len(pdf) > 1000, "pdf too small: %d bytes", len(pdf))
	assert.True(t, bytes.HasPrefix(pdf, []byte("%PDF")))

	if dir := os.Getenv("INVOICE_PDF_SAMPLE_DIR"); dir != "" {
		assert.Nil(t, os.WriteFile(filepath.Join(dir, "invoice-sample.pdf"), pdf, 0644))
	}
}

func TestRenderWithoutLogoOrFooter(t *testing.T) {
	invoice, lines := sampleInvoice()
	invoice.BillToPan = ""
	invoice.Notes = ""
	lines = lines[:1] // every quantity is 1, so the Quantity column is hidden

	pdf, err := Render(sampleProfile(), invoice, lines, 0, "")
	assert.Nil(t, err)
	assert.True(t, bytes.HasPrefix(pdf, []byte("%PDF")))
}

func TestRenderIgnoresBadLogo(t *testing.T) {
	invoice, lines := sampleInvoice()

	for _, logo := range []string{"data:image/png;base64,not-base64!!", "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("not a png")), "https://example.com/logo.png"} {
		profile := sampleProfile()
		profile.LogoDataUrl = logo

		pdf, err := Render(profile, invoice, lines, 0, "")
		assert.Nil(t, err, logo)
		assert.True(t, bytes.HasPrefix(pdf, []byte("%PDF")), logo)
	}
}

func TestRenderManyLinesSpansPages(t *testing.T) {
	invoice, lines := sampleInvoice()

	for i := 0; i < 40; i++ {
		lines = append(lines, &models.InvoiceLine{ItemName: "Extra line", Description: strings.Repeat("Long description that wraps across the services column. ", 3), Quantity: 100, Rate: 100000, Amount: 100000})
	}

	pdf, err := Render(sampleProfile(), invoice, lines, 0, "https://books.codeation.io/public/invoice/abc")
	assert.Nil(t, err)
	// more than one page object
	assert.True(t, bytes.Count(pdf, []byte("/Type /Page\n")) >= 2 || bytes.Count(pdf, []byte("/Type /Page/")) >= 2 || bytes.Count(pdf, []byte("/Type /Page ")) >= 2)
}
