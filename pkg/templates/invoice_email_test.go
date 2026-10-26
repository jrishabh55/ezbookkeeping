package templates

import (
	"bytes"
	"html/template"
	"testing"

	"github.com/stretchr/testify/require"
)

// A fixed heading, button or footer in the outer document would make an intentionally empty body impossible.
func TestInvoiceEmailTemplateWrapsOnlyEditableBody(t *testing.T) {
	t.Chdir("../..")
	tmpl, err := GetTemplate(TEMPLATE_INVOICE_EMAIL)
	require.NoError(t, err)
	var body bytes.Buffer
	err = tmpl.Execute(&body, map[string]any{"Subject": "My <subject>", "Body": template.HTML(`<h1>Custom heading</h1><p>Custom footer</p>`)})
	require.NoError(t, err)
	require.Contains(t, body.String(), "<title>My &lt;subject&gt;</title>")
	require.Contains(t, body.String(), "<h1>Custom heading</h1><p>Custom footer</p>")
	require.Contains(t, body.String(), "max-width: 600px")
	for _, fixed := range []string{"View invoice", "Amount due", "Invoice ", "Due "} {
		require.NotContains(t, body.String(), fixed)
	}
	body.Reset()
	require.NoError(t, tmpl.Execute(&body, map[string]any{"Subject": "Invoice", "Body": template.HTML("")}))
	require.NotContains(t, body.String(), "Custom heading")
	require.NotContains(t, body.String(), "<a ")
}
