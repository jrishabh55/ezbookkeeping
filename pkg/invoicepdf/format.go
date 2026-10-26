package invoicepdf

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// quantityScale mirrors INVOICE_QUANTITY_SCALE in src/lib/invoice.ts (quantities are stored in hundredths)
const quantityScale = 100

// currencySymbols are the symbols printed before an amount; other currencies print their code and a space
var currencySymbols = map[string]string{
	"INR": "₹",
	"USD": "$",
	"EUR": "€",
	"GBP": "£",
	"JPY": "¥",
	"AUD": "A$",
	"CAD": "C$",
	"SGD": "S$",
}

var fileNameUnsafeChars = regexp.MustCompile(`[\\/:*?"<>|\s]+`)

// groupDigits inserts commas into a string of digits: Indian style (3, then 2s) or western style (3s)
func groupDigits(digits string, indian bool) string {
	if len(digits) <= 3 {
		return digits
	}

	head, tail := digits[:len(digits)-3], digits[len(digits)-3:]
	groupSize := 3

	if indian {
		groupSize = 2
	}

	var groups []string

	for len(head) > groupSize {
		groups = append([]string{head[len(head)-groupSize:]}, groups...)
		head = head[:len(head)-groupSize]
	}

	groups = append([]string{head}, groups...)

	return strings.Join(groups, ",") + "," + tail
}

// formatAmount prints minor units the way the Vue document does: the currency symbol, then the amount with two decimals.
// INR uses Indian digit grouping (₹3,70,000.00); other currencies use en-US grouping ($1,000.00).
func formatAmount(minorUnits int64, currency string) string {
	negative := minorUnits < 0

	if negative {
		minorUnits = -minorUnits
	}

	symbol, ok := currencySymbols[currency]

	if !ok {
		symbol = currency + " "
	}

	whole := groupDigits(fmt.Sprintf("%d", minorUnits/100), currency == "INR")
	text := fmt.Sprintf("%s%s.%02d", symbol, whole, minorUnits%100)

	if negative {
		return "-" + text
	}

	return text
}

// formatQuantity prints hundredths with trailing zeros trimmed, mirroring formatInvoiceQuantity in src/lib/invoice.ts
func formatQuantity(hundredths int64) string {
	text := fmt.Sprintf("%d.%02d", hundredths/quantityScale, hundredths%quantityScale)
	text = strings.TrimRight(text, "0")

	return strings.TrimSuffix(text, ".")
}

// formatTaxRate prints basis points as a percentage with trailing zeros trimmed (1800 → "18%", 925 → "9.25%"),
// mirroring formatInvoiceTaxRate in src/lib/invoice.ts; the rate is a float because CGST / SGST get half the rate
func formatTaxRate(basisPoints float64) string {
	return strconv.FormatFloat(basisPoints/100, 'f', -1, 64) + "%"
}

// taxRow is one tax line between subtotal and total, mirroring InvoiceTaxRow in src/lib/invoice.ts
type taxRow struct {
	name   string
	rate   float64 // basis points
	amount int64
}

// taxRows mirrors getInvoiceTaxRows: one row for IGST, two halves for CGST + SGST (the odd paisa goes to SGST), none otherwise
func taxRows(invoice *models.Invoice) []taxRow {
	switch invoice.TaxType {
	case models.INVOICE_TAX_TYPE_IGST:
		return []taxRow{{"IGST", float64(invoice.TaxRate), invoice.TaxAmount}}
	case models.INVOICE_TAX_TYPE_CGST_SGST:
		cgst := invoice.TaxAmount / 2
		return []taxRow{{"CGST", float64(invoice.TaxRate) / 2, cgst}, {"SGST", float64(invoice.TaxRate) / 2, invoice.TaxAmount - cgst}}
	}

	return nil
}

// FormatAmount is formatAmount for callers outside the package (the invoice email summary)
func FormatAmount(minorUnits int64, currency string) string {
	return formatAmount(minorUnits, currency)
}

// FormatDate is formatDate for callers outside the package (the invoice email summary)
func FormatDate(date string) string {
	return formatDate(date)
}

// BalanceDue is balanceDue for callers outside the package (the invoice email summary)
func BalanceDue(invoice *models.Invoice, received int64) int64 {
	return balanceDue(invoice, received)
}

// formatDate prints "2026-04-30" as "April 30, 2026", mirroring formatInvoiceDocumentDate in src/lib/invoice.ts
func formatDate(date string) string {
	parsed, err := time.Parse("2006-01-02", date)

	if err != nil {
		return date
	}

	return parsed.Format("January 2, 2006")
}

// FileName returns the download file name of an invoice PDF, mirroring getInvoicePdfFileName in src/lib/invoice.ts
func FileName(number string, dueDate string) string {
	return "Invoice_" + fileNameUnsafeChars.ReplaceAllString(number, "_") + "_" + dueDate + ".pdf"
}

// balanceDue mirrors getInvoiceDisplayBalanceDue in src/lib/invoice.ts: nothing is owed once the invoice is paid
// (closed manually or within the tolerance), otherwise total minus received, never negative
func balanceDue(invoice *models.Invoice, received int64) int64 {
	due := invoice.Total - received

	if due < 0 {
		due = 0
	}

	if invoice.ClosedManually || float64(due) <= float64(invoice.Total)*models.InvoicePaidTolerance {
		return 0
	}

	return due
}
