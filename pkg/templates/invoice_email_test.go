package templates

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvoiceEmailTemplateRendersParagraphsButtonAndSummary(t *testing.T) {
	t.Chdir("../..") // templates are read relative to the repository root

	tmpl, err := GetTemplate(TEMPLATE_INVOICE_EMAIL)
	assert.Nil(t, err)

	var body bytes.Buffer
	err = tmpl.Execute(&body, map[string]any{
		"Subject":    "Invoice CA/2026-27/01 from Rishabh Jain",
		"Paragraphs": []string{"Hi Kanika,", "Please find invoice <b>CA/2026-27/01</b> attached."},
		"ViewUrl":    "https://books.codeation.io/i/abc",
		"Number":     "CA/2026-27/01",
		"AmountDue":  "₹1,180.00",
		"DueDate":    "April 30, 2026",
	})
	assert.Nil(t, err)

	html := body.String()
	assert.Contains(t, html, "<title>Invoice CA/2026-27/01 from Rishabh Jain</title>")
	assert.Contains(t, html, ">Hi Kanika,</p>")
	// the message is plain text: markup in it is escaped, never rendered
	assert.Contains(t, html, "&lt;b&gt;CA/2026-27/01&lt;/b&gt;")
	assert.NotContains(t, html, "<b>CA")
	assert.Contains(t, html, `href="https://books.codeation.io/i/abc"`)
	assert.Contains(t, html, "View invoice")
	assert.Contains(t, html, "₹1,180.00")
	assert.Contains(t, html, "April 30, 2026")
}
