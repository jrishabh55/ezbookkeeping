// Package invoicepdf renders an invoice as a PDF that mirrors the printable document
// in src/components/common/InvoiceDocumentContent.vue (same blocks, same amount and date formats).
package invoicepdf

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"image"
	_ "image/jpeg" // logo decoding
	_ "image/png"  // logo decoding
	"strings"

	"github.com/go-pdf/fpdf"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// DejaVu Sans 2.37 (fonts/LICENSE); the core PDF fonts are WinAnsi only and cannot print ₹
//
//go:embed fonts/DejaVuSans.ttf
var fontRegular []byte

//go:embed fonts/DejaVuSans-Bold.ttf
var fontBold []byte

const (
	pageMargin   = 18.0 // mm, A4
	bottomMargin = 20.0
	lineHeight   = 4.8 // mm at 10pt, like the document's 13px / 1.35
	baseFontSize = 10.0
	blockGap     = 3.0 // the document's 12px spacer between address and contact lines
	pxToMm       = 0.2646
)

type rgb struct{ r, g, b int }

var (
	colorBlack      = rgb{0, 0, 0}
	colorAccent     = rgb{61, 134, 245}  // #3d86f5
	colorMuted      = rgb{107, 114, 128} // #6b7280
	colorHighlight  = rgb{243, 244, 246} // #f3f4f6
	colorTotalsRule = rgb{229, 231, 235} // #e5e7eb
)

type renderer struct {
	pdf    *fpdf.Fpdf
	left   float64
	right  float64
	width  float64
	breakY float64 // y beyond which content must go to a new page
}

// Render draws the invoice on A4 pages and returns the PDF bytes.
// received is the sum of linked payments; verifyUrl, when not empty, is printed in the footer of every page.
func Render(profile *models.InvoiceProfile, invoice *models.Invoice, lines []*models.InvoiceLine, received int64, verifyUrl string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(pageMargin, pageMargin, pageMargin)
	pdf.SetAutoPageBreak(false, bottomMargin)
	pdf.AddUTF8FontFromBytes("DejaVu", "", fontRegular)
	pdf.AddUTF8FontFromBytes("DejaVu", "B", fontBold)
	pdf.SetTitle(invoice.Number, true)
	pdf.SetAuthor(profile.BusinessName, true)

	pageWidth, pageHeight := pdf.GetPageSize()
	r := &renderer{pdf: pdf, left: pageMargin, right: pageWidth - pageMargin, width: pageWidth - 2*pageMargin, breakY: pageHeight - bottomMargin}

	if verifyUrl != "" {
		pdf.SetFooterFunc(func() {
			pdf.SetY(-13)
			r.font(false, 8, colorMuted)
			pdf.CellFormat(0, 4, "Verify this invoice at "+verifyUrl, "", 0, "C", false, 0, verifyUrl)
		})
	}

	pdf.AddPage()

	y := r.drawHeader(profile)
	y = r.drawBanner(y + 10)
	y = r.drawParties(invoice, received, y+14)
	y = r.drawLines(invoice, lines, y+10)
	y = r.drawTotals(invoice, received, y+4)
	r.drawNotes(invoice, y+14)

	var buf bytes.Buffer

	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (r *renderer) font(bold bool, size float64, color rgb) {
	style := ""

	if bold {
		style = "B"
	}

	r.pdf.SetFont("DejaVu", style, size)
	r.pdf.SetTextColor(color.r, color.g, color.b)
}

// wrap splits pre-line text ("\n" kept) into lines that fit width with the current font
func (r *renderer) wrap(text string, width float64) []string {
	var lines []string

	for _, paragraph := range strings.Split(strings.TrimRight(text, "\r\n"), "\n") {
		paragraph = strings.TrimRight(paragraph, "\r")

		if paragraph == "" {
			lines = append(lines, "")
			continue
		}

		lines = append(lines, r.pdf.SplitText(paragraph, width)...)
	}

	return lines
}

// cell prints one line at an absolute position; align is "L", "R" or "C"
func (r *renderer) cell(x float64, y float64, width float64, text string, align string) {
	r.pdf.SetXY(x, y)
	r.pdf.CellFormat(width, lineHeight, text, "", 0, align, false, 0, "")
}

// column prints wrapped lines downwards from y and returns the y below them
func (r *renderer) column(x float64, y float64, width float64, text string, align string) float64 {
	for _, line := range r.wrap(text, width) {
		r.cell(x, y, width, line, align)
		y += lineHeight
	}

	return y
}

// ensureSpace starts a new page when height does not fit below y and returns the (possibly new) y
func (r *renderer) ensureSpace(y float64, height float64) float64 {
	if y+height <= r.breakY {
		return y
	}

	r.pdf.AddPage()

	return pageMargin
}

func (r *renderer) dottedLine(y float64) {
	r.pdf.SetDrawColor(colorBlack.r, colorBlack.g, colorBlack.b)
	r.pdf.SetLineWidth(0.25)
	r.pdf.SetDashPattern([]float64{0.4, 0.8}, 0)
	r.pdf.Line(r.left, y, r.right, y)
	r.pdf.SetDashPattern([]float64{}, 0)
}

// logo decodes the profile's data URL and registers it; it returns false (and leaves no error behind) when the logo is missing or unusable
func (r *renderer) logo(dataUrl string) (width float64, height float64, ok bool) {
	var imageType string

	switch {
	case strings.HasPrefix(dataUrl, "data:image/png;base64,"):
		imageType = "PNG"
	case strings.HasPrefix(dataUrl, "data:image/jpeg;base64,"):
		imageType = "JPEG"
	default:
		return 0, 0, false
	}

	data, err := base64.StdEncoding.DecodeString(dataUrl[strings.Index(dataUrl, ",")+1:])

	if err != nil {
		return 0, 0, false
	}

	config, _, err := image.DecodeConfig(bytes.NewReader(data))

	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return 0, 0, false
	}

	info := r.pdf.RegisterImageOptionsReader("logo", fpdf.ImageOptions{ImageType: imageType}, bytes.NewReader(data))

	if info == nil || r.pdf.Err() {
		r.pdf.ClearError() // ponytail: an fpdf-unsupported PNG variant (16-bit, interlaced) prints no logo rather than failing the invoice
		return 0, 0, false
	}

	// the document shows the logo at its natural size, capped at 48px high and 320px wide
	width, height = float64(config.Width)*pxToMm, float64(config.Height)*pxToMm
	scale := 1.0

	if maxHeight := 48 * pxToMm; height*scale > maxHeight {
		scale = maxHeight / height
	}

	if maxWidth := 320 * pxToMm; width*scale > maxWidth {
		scale = maxWidth / width
	}

	return width * scale, height * scale, true
}

// drawHeader prints the logo top-left and the business block top-right; it returns the y below both
func (r *renderer) drawHeader(profile *models.InvoiceProfile) float64 {
	top := pageMargin
	bottom := top

	if width, height, ok := r.logo(profile.LogoDataUrl); ok {
		r.pdf.ImageOptions("logo", r.left, top, width, height, false, fpdf.ImageOptions{}, 0, "")
		bottom = top + height
	}

	x, width := r.left+r.width/2, r.width/2
	y := top

	r.font(true, baseFontSize, colorBlack)
	y = r.column(x, y, width, profile.BusinessName, "R")
	r.font(false, baseFontSize, colorBlack)

	if profile.AddressLines != "" {
		y = r.column(x, y, width, profile.AddressLines, "R")
	}

	if profile.Phone != "" || profile.Website != "" || profile.Gstin != "" {
		y += blockGap
	}

	if profile.Phone != "" {
		y = r.column(x, y, width, "Mobile: "+profile.Phone, "R")
	}

	if profile.Website != "" {
		y = r.column(x, y, width, profile.Website, "R")
	}

	if profile.Gstin != "" {
		y = r.column(x, y, width, "GSTIN: "+profile.Gstin, "R")
	}

	return max(bottom, y)
}

// drawBanner prints "Invoice" in a double-bordered box between two double rules; it returns the y below it
func (r *renderer) drawBanner(y float64) float64 {
	const (
		titleSize  = 22.0
		paddingX   = 9.0 // 34px
		paddingY   = 1.6 // 6px
		outlineGap = 1.3 // 3px offset + 2px line
		ruleGap    = 4.0 // 16px
		ruleHalf   = 0.8 // the 6px rule: two lines 1.6mm apart
	)

	r.font(false, titleSize, colorAccent)
	textWidth := r.pdf.GetStringWidth("Invoice")
	textHeight := titleSize * 0.3528 // pt to mm
	boxWidth, boxHeight := textWidth+2*paddingX, textHeight+2*paddingY
	boxX := r.left + (r.width-boxWidth)/2

	r.pdf.SetDrawColor(colorAccent.r, colorAccent.g, colorAccent.b)
	r.pdf.SetLineWidth(0.5)
	r.pdf.RoundedRect(boxX, y, boxWidth, boxHeight, 1.5, "1234", "D")
	r.pdf.RoundedRect(boxX-outlineGap, y-outlineGap, boxWidth+2*outlineGap, boxHeight+2*outlineGap, 2.5, "1234", "D")

	r.pdf.SetXY(boxX, y)
	r.pdf.CellFormat(boxWidth, boxHeight, "Invoice", "", 0, "C", false, 0, "")

	middle := y + boxHeight/2

	for _, offset := range []float64{-ruleHalf, ruleHalf} {
		r.pdf.Line(r.left, middle+offset, boxX-outlineGap-ruleGap, middle+offset)
		r.pdf.Line(boxX+boxWidth+outlineGap+ruleGap, middle+offset, r.right, middle+offset)
	}

	return y + boxHeight + outlineGap
}

// drawParties prints the BILL TO block on the left and the meta table on the right; it returns the y below both
func (r *renderer) drawParties(invoice *models.Invoice, received int64, y float64) float64 {
	const (
		labelWidth = 44.0
		valueWidth = 46.0
		rowPadding = 0.6 // 2px
	)

	width := r.width - labelWidth - valueWidth - 6
	left := y

	r.font(false, baseFontSize, colorMuted)
	left = r.column(r.left, left, width, "BILL TO", "L")
	r.font(true, baseFontSize, colorBlack)
	left = r.column(r.left, left, width, invoice.BillToName, "L")
	r.font(false, baseFontSize, colorBlack)

	if invoice.BillToContactName != "" {
		left = r.column(r.left, left, width, invoice.BillToContactName, "L")
	}

	if invoice.BillToAddressLines != "" {
		left = r.column(r.left, left, width, invoice.BillToAddressLines, "L")
	}

	if invoice.BillToEmail != "" || invoice.BillToPhone != "" {
		left += blockGap
	}

	if invoice.BillToPhone != "" {
		left = r.column(r.left, left, width, invoice.BillToPhone, "L")
	}

	if invoice.BillToEmail != "" {
		left = r.column(r.left, left, width, invoice.BillToEmail, "L")
	}

	if invoice.BillToPan != "" {
		left = r.column(r.left, left, width, "PAN: "+invoice.BillToPan, "L")
	}

	if invoice.BillToGstin != "" {
		left = r.column(r.left, left, width, "GSTIN: "+invoice.BillToGstin, "L")
	}

	rows := []struct {
		label     string
		value     string
		highlight bool
	}{
		{"Invoice Number:", invoice.Number, false},
		{"Invoice Date:", formatDate(invoice.InvoiceDate), false},
		{"Payment Due:", formatDate(invoice.DueDate), false},
		{"Amount Due (" + invoice.Currency + "):", formatAmount(balanceDue(invoice, received), invoice.Currency), true},
	}

	tableX := r.right - labelWidth - valueWidth
	rowHeight := lineHeight + 2*rowPadding
	right := y

	for _, row := range rows {
		if row.highlight {
			r.pdf.SetFillColor(colorHighlight.r, colorHighlight.g, colorHighlight.b)
			r.pdf.Rect(tableX, right, labelWidth+valueWidth, rowHeight, "F")
		}

		r.font(true, baseFontSize, colorBlack)
		r.cell(tableX, right+rowPadding, labelWidth-3, row.label, "R")
		r.font(row.highlight, baseFontSize, colorBlack)
		r.cell(tableX+labelWidth, right+rowPadding, valueWidth-4, row.value, "L")
		right += rowHeight
	}

	return max(left, right)
}

type lineColumns struct {
	services, sac, quantity, rate, amount float64
}

func (r *renderer) drawLinesHeader(columns lineColumns, y float64) float64 {
	const padding = 2.0 // 8px

	r.font(true, baseFontSize, colorBlack)
	x := r.left
	r.cell(x, y+padding, columns.services, "Services", "L")
	x += columns.services

	if columns.sac > 0 {
		r.cell(x, y+padding, columns.sac, "SAC", "R")
		x += columns.sac
	}

	if columns.quantity > 0 {
		r.cell(x, y+padding, columns.quantity, "Quantity", "R")
		x += columns.quantity
	}

	r.cell(x, y+padding, columns.rate, "Rate", "R")
	x += columns.rate
	r.cell(x, y+padding, columns.amount, "Amount", "R")

	y += lineHeight + 2*padding
	r.dottedLine(y)

	return y
}

// drawLines prints the services table; it returns the y below the last row
func (r *renderer) drawLines(invoice *models.Invoice, lines []*models.InvoiceLine, y float64) float64 {
	const padding = 3.0 // 12px

	columns := lineColumns{rate: 34, amount: 34}

	for _, line := range lines {
		if line.Quantity != quantityScale {
			columns.quantity = 24
		}

		if line.Sac != "" {
			columns.sac = 22
		}
	}

	columns.services = r.width - columns.sac - columns.quantity - columns.rate - columns.amount

	y = r.ensureSpace(y, 3*lineHeight+4*padding)
	y = r.drawLinesHeader(columns, y)

	for _, line := range lines {
		r.font(true, baseFontSize, colorBlack)
		nameLines := r.wrap(line.ItemName, columns.services-3)
		r.font(false, baseFontSize, colorMuted)
		descriptionLines := []string{}

		if line.Description != "" {
			descriptionLines = r.wrap(line.Description, columns.services-3)
		}

		height := float64(len(nameLines)+len(descriptionLines))*lineHeight + 2*padding

		if y+height > r.breakY {
			y = r.ensureSpace(y, height)
			y = r.drawLinesHeader(columns, y)
		}

		textY := y + padding
		r.font(true, baseFontSize, colorBlack)

		for _, text := range nameLines {
			r.cell(r.left, textY, columns.services-3, text, "L")
			textY += lineHeight
		}

		r.font(false, baseFontSize, colorMuted)

		for _, text := range descriptionLines {
			r.cell(r.left, textY, columns.services-3, text, "L")
			textY += lineHeight
		}

		r.font(false, baseFontSize, colorBlack)
		x := r.left + columns.services

		if columns.sac > 0 {
			r.cell(x, y+padding, columns.sac, line.Sac, "R")
			x += columns.sac
		}

		if columns.quantity > 0 {
			r.cell(x, y+padding, columns.quantity, formatQuantity(line.Quantity), "R")
			x += columns.quantity
		}

		r.cell(x, y+padding, columns.rate, formatAmount(line.Rate, invoice.Currency), "R")
		x += columns.rate
		r.cell(x, y+padding, columns.amount, formatAmount(line.Amount, invoice.Currency), "R")

		y += height
		r.dottedLine(y)
	}

	return y
}

// totalsRow is one line of the totals block; due marks the Amount Due line (rule above, bold value)
type totalsRow struct {
	label string
	value string
	due   bool
}

// totalsRows mirrors the totals table of InvoiceDocumentContent.vue: Subtotal and the tax rows only when the
// invoice is taxed (invoices saved before GST have no tax type and render as untaxed), then Total,
// Payments received when something was received, and Amount Due
func totalsRows(invoice *models.Invoice, received int64) []totalsRow {
	var rows []totalsRow

	if taxes := taxRows(invoice); len(taxes) > 0 {
		rows = append(rows, totalsRow{"Subtotal:", formatAmount(invoice.Subtotal, invoice.Currency), false})

		for _, tax := range taxes {
			rows = append(rows, totalsRow{tax.name + " (" + formatTaxRate(tax.rate) + "):", formatAmount(tax.amount, invoice.Currency), false})
		}
	}

	rows = append(rows, totalsRow{"Total:", formatAmount(invoice.Total, invoice.Currency), false})

	if received > 0 {
		rows = append(rows, totalsRow{"Payments received:", "−" + formatAmount(min(received, invoice.Total), invoice.Currency), false})
	}

	return append(rows, totalsRow{"Amount Due (" + invoice.Currency + "):", formatAmount(balanceDue(invoice, received), invoice.Currency), true})
}

// drawTotals prints the right-aligned totals block; it returns the y below it
func (r *renderer) drawTotals(invoice *models.Invoice, received int64, y float64) float64 {
	const (
		labelPadding = 12.0 // 48px
		rowPadding   = 2.0  // 8px
	)

	rows := totalsRows(invoice, received)

	r.font(true, baseFontSize, colorBlack)
	labelWidth, valueWidth := 0.0, 0.0

	for _, row := range rows {
		labelWidth = max(labelWidth, r.pdf.GetStringWidth(row.label))
		valueWidth = max(valueWidth, r.pdf.GetStringWidth(row.value))
	}

	labelWidth += labelPadding
	valueWidth += 1
	rowHeight := lineHeight + 2*rowPadding
	blockX := r.right - labelWidth - valueWidth

	y = r.ensureSpace(y, float64(len(rows))*rowHeight)

	for _, row := range rows {
		if row.due {
			r.pdf.SetDrawColor(colorTotalsRule.r, colorTotalsRule.g, colorTotalsRule.b)
			r.pdf.SetLineWidth(0.8)
			r.pdf.Line(blockX, y, r.right, y)
		}

		r.font(true, baseFontSize, colorBlack)
		r.cell(blockX, y+rowPadding, labelWidth-labelPadding, row.label, "R")
		r.font(row.due, baseFontSize, colorBlack)
		r.cell(blockX+labelWidth, y+rowPadding, valueWidth, row.value, "R")
		y += rowHeight
	}

	return y
}

// drawNotes prints the Notes / Terms block when the invoice has notes
func (r *renderer) drawNotes(invoice *models.Invoice, y float64) {
	if invoice.Notes == "" {
		return
	}

	y = r.ensureSpace(y, 3*lineHeight)
	r.font(true, baseFontSize, colorMuted)
	y = r.column(r.left, y, r.width, "Notes / Terms", "L")
	r.font(false, baseFontSize, colorMuted)

	for _, line := range r.wrap(invoice.Notes, r.width) {
		y = r.ensureSpace(y, lineHeight)
		r.cell(r.left, y, r.width, line, "L")
		y += lineHeight
	}
}
