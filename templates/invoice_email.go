// Package emailtemplates embeds the invoice document shared by previews and outgoing mail.
package emailtemplates

import _ "embed"

// InvoiceEmailDocument is the structural wrapper. All visible content belongs to its sanitized Body.
//
//go:embed email/invoice.tmpl
var InvoiceEmailDocument string
