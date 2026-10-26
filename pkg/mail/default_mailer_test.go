package mail

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildMailMessageWithoutAttachments(t *testing.T) {
	mailer := &DefaultMailer{fromAddress: "books@codeation.io"}
	msg := mailer.buildMessage(&MailMessage{To: "accounts@qubertech.com", Subject: "Invoice CA/2026-27/01", Body: "<p>Hello</p>"})

	var buf bytes.Buffer
	_, err := msg.WriteTo(&buf)
	assert.Nil(t, err)

	text := buf.String()
	assert.Contains(t, text, "From: books@codeation.io")
	assert.Contains(t, text, "To: accounts@qubertech.com")
	assert.Contains(t, text, "Subject: Invoice CA/2026-27/01")
	assert.Contains(t, text, "Content-Type: text/html")
	assert.NotContains(t, text, "Content-Disposition: attachment")
}

func TestBuildMailMessageWithAttachments(t *testing.T) {
	mailer := &DefaultMailer{fromAddress: "books@codeation.io"}
	msg := mailer.buildMessage(&MailMessage{
		To:      "accounts@qubertech.com",
		Subject: "Invoice",
		Body:    "<p>Hello</p>",
		Attachments: []MailAttachment{
			{Name: "Invoice_CA_2026-27_01_2026-04-30.pdf", Content: []byte("%PDF-1.4 fake")},
			{Name: "notes.txt", Content: []byte("plain")},
		},
	})

	var buf bytes.Buffer
	_, err := msg.WriteTo(&buf)
	assert.Nil(t, err)

	text := buf.String()
	assert.Contains(t, text, `Content-Disposition: attachment; filename="Invoice_CA_2026-27_01_2026-04-30.pdf"`)
	assert.Contains(t, text, "Content-Type: application/pdf")
	assert.Contains(t, text, `Content-Disposition: attachment; filename="notes.txt"`)
	assert.Contains(t, text, "Content-Type: text/html")
}
