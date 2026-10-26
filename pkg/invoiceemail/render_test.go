package invoiceemail

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testValues() map[string]string {
	return map[string]string{"customer_name": "A <Customer>", "contact_name": "Alex", "customer_email": "alex@example.com", "invoice_number": "INV-42", "invoice_date": "October 1, 2026", "due_date": "October 15, 2026", "invoice_total": "₹1,000.00", "amount_paid": "₹250.00", "amount_due": "₹750.00", "currency": "INR", "invoice_url": "https://books.example/i/trusted", "business_name": "Business & Co", "business_address": "First floor\nMain Street", "business_phone": "", "business_website": "https://example.com"}
}

// Replacing variables with raw HTML, omitting a catalog key, or recursively expanding values breaks this test.
func TestRenderExpandsCatalogAsEscapedNonRecursiveText(t *testing.T) {
	values := testValues()
	values["business_name"] = "{{invoice_number}} & Co"
	var body strings.Builder
	for _, key := range []string{"customer_name", "contact_name", "customer_email", "invoice_number", "invoice_date", "due_date", "invoice_total", "amount_paid", "amount_due", "currency", "invoice_url", "business_name", "business_address", "business_phone", "business_website"} {
		body.WriteString(`<p><span data-invoice-variable="` + key + `">ignored editor label</span></p>`)
	}
	result, err := Render("Invoice {{invoice_number}}", body.String(), values)
	require.NoError(t, err)
	require.Equal(t, "Invoice INV-42", result.Subject)
	require.Len(t, result.Variables, 15)
	for _, value := range []string{"A &lt;Customer&gt;", "Alex", "alex@example.com", "October 1, 2026", "October 15, 2026", "₹1,000.00", "₹250.00", "₹750.00", "INR", "{{invoice_number}} &amp; Co", "First floor<br/>Main Street"} {
		require.Contains(t, result.BodyHTML, value)
	}
	require.NotContains(t, result.BodyHTML, "ignored editor label")
	require.Contains(t, result.EmailHTML, "<!DOCTYPE html>")
	require.Contains(t, result.EmailHTML, result.BodyHTML)
}

// Allowing active HTML, arbitrary CSS or unsafe destinations into a preview/email breaks this test.
func TestRenderSanitizesMarkupAndLinks(t *testing.T) {
	body := `<script>evil()</script><img src=x onerror=evil()><iframe>bad</iframe><h2 style="color:red;text-align: right" onclick="evil()">Heading</h2><p><strong>Bold</strong><em>Italic</em><u>Underlined</u><br><a href="javascript:alert(1)">unsafe</a><a href="https://example.com/path" onclick="evil()">safe</a><a href="mailto:alex@example.com">mail</a><a href="//tracker.example/">relative</a><a href="{{invoice_url}}">invoice</a></p><ol><li>One</li></ol><ul><li>Two</li></ul>`
	result, err := Render("Invoice", body, testValues())
	require.NoError(t, err)
	for _, unsafe := range []string{"evil", "<script", "<img", "<iframe", "onclick", "javascript:", "color:red", "//tracker.example"} {
		require.NotContains(t, result.BodyHTML, unsafe)
	}
	for _, safe := range []string{`text-align: right`, "<h2", "<strong>", "<em>", "<u>", "<ol", "<ul", "<li", `href="https://example.com/path"`, `href="mailto:alex@example.com"`, `href="https://books.example/i/trusted"`} {
		require.Contains(t, result.BodyHTML, safe)
	}
}

func TestRenderButtonIsMovableEscapedAndAlwaysUsesInvoiceURL(t *testing.T) {
	result, err := Render("Invoice", `<p>Before</p><div data-invoice-button="true" href="https://evil.example" style="text-align: right"><b>Pay &amp; view</b></div><p>After</p>`, testValues())
	require.NoError(t, err)
	require.Contains(t, result.BodyHTML, `href="https://books.example/i/trusted"`)
	require.Contains(t, result.BodyHTML, ">Pay &amp; view</a>")
	require.NotContains(t, result.BodyHTML, "evil.example")
	require.Less(t, strings.Index(result.BodyHTML, "Before"), strings.Index(result.BodyHTML, "Pay &amp; view"))
	require.Less(t, strings.Index(result.BodyHTML, "Pay &amp; view"), strings.Index(result.BodyHTML, "After"))
	empty, err := Render("Invoice", "", testValues())
	require.NoError(t, err)
	require.Empty(t, empty.BodyHTML)
	require.NotContains(t, empty.EmailHTML, "INV-42")
	require.NotContains(t, empty.EmailHTML, "View Invoice")
}

func TestRenderRejectsInvalidVariablesAndSubject(t *testing.T) {
	tests := []struct{ subject, body string }{
		{"Invoice {{unknown}}", ""}, {"Invoice", `<span data-invoice-variable="unknown">x</span>`},
		{"Invoice", "{{unknown}}"}, {"Invoice", `<a href="{{unknown}}">x</a>`},
		{"Invoice\nBcc: someone@example.com", ""}, {"Invoice\rBcc: x", ""}, {" ", ""}, {strings.Repeat("x", 257), ""},
	}
	for _, tc := range tests {
		_, err := Render(tc.subject, tc.body, testValues())
		require.Error(t, err, "%q %q", tc.subject, tc.body)
	}
	values := testValues()
	values["customer_name"] = strings.Repeat("x", 257)
	_, err := Render("{{customer_name}}", "", values)
	require.Error(t, err)
	values["customer_name"] = "A\nB"
	_, err = Render("{{customer_name}}", "", values)
	require.Error(t, err)
}

func TestSanitizeTemplatePreservesStableMarkersAndEmptyBody(t *testing.T) {
	body, err := SanitizeTemplate(`<p><span data-invoice-variable="customer_name">Sample Person</span></p><div data-invoice-button="true" style="text-align: center">Read invoice</div>`)
	require.NoError(t, err)
	require.Contains(t, body, `data-invoice-variable="customer_name">{{customer_name}}</span>`)
	require.Contains(t, body, `data-invoice-button="true"`)
	require.NotContains(t, body, "Sample Person")
	empty, err := SanitizeTemplate("")
	require.NoError(t, err)
	require.Empty(t, empty)
	_, err = SanitizeTemplate(strings.Repeat("x", 32001))
	require.Error(t, err)
}

func TestTemplateCanonicalSerializationMustFitStorageLimit(t *testing.T) {
	_, err := SanitizeTemplate(strings.Repeat("<br>", 8000))
	require.Error(t, err)
}

func TestEmptyVariableMarkerIsRejected(t *testing.T) {
	_, err := SanitizeTemplate(`<span data-invoice-variable="">unknown</span>`)
	require.Error(t, err)
}

func TestPlainTextLegacyRenderingKeepsTextLiteralAndNonemptyParagraphs(t *testing.T) {
	result, err := RenderPlainText("Invoice {{literal subject}}", " Line one \n\n <script>literal</script> {{unknown}} \n", testValues())
	require.NoError(t, err)
	require.Equal(t, "Invoice {{literal subject}}", result.Subject)
	require.Contains(t, result.BodyHTML, `>Line one</p>`)
	require.Contains(t, result.BodyHTML, `>&lt;script&gt;literal&lt;/script&gt; {{unknown}}</p>`)
	require.NotContains(t, result.BodyHTML, `<script>`)
	require.Contains(t, result.BodyHTML, "View invoice")
	require.Contains(t, result.BodyHTML, "₹750.00")
	require.Contains(t, result.EmailHTML, result.BodyHTML)
}

// Dropping the no-business preset, the empty-value tidy, the formatting-whitespace rule or the empty-button check breaks this test.
func TestRenderHandlesEmptyValuesFormattingWhitespaceAndEmptyButtons(t *testing.T) {
	values := testValues()
	values["business_name"], values["business_address"], values["business_website"] = "", "", ""
	subject, body := PresetWithoutBusinessName()
	result, err := Render(subject, body, values)
	require.NoError(t, err)
	require.Equal(t, "Invoice INV-42", result.Subject)
	require.Contains(t, result.BodyHTML, "You have received an invoice")
	require.NotContains(t, result.BodyHTML, "<br/>")
	require.NotContains(t, result.BodyHTML, "<strong></strong>")

	sanitized, err := SanitizeTemplate("<ul>\n<li>a</li>\n</ul><p>x</p>\n<p>y<br>z</p>")
	require.NoError(t, err)
	require.Equal(t, "<ul> <li>a</li> </ul><p>x</p> <p>y<br/>z</p>", sanitized)

	_, err = SanitizeTemplate(`<div data-invoice-button="true"> </div>`)
	require.Error(t, err)
}
