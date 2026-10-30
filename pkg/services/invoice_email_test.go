package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestInvoiceEmailProfileDefaultsAndIntentionalEmptyBodyPersist(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	profile, err := Invoices.GetProfile(ctx, uid)
	require.NoError(t, err)
	encoded, err := json.Marshal(profile.ToInvoiceProfileResponse())
	require.NoError(t, err)
	var response map[string]any
	require.NoError(t, json.Unmarshal(encoded, &response))
	require.Equal(t, "Invoice {{invoice_number}} from {{business_name}}", response["emailSubjectTemplate"])
	require.Contains(t, response["emailBodyTemplate"], "data-invoice-button")
	require.NoError(t, json.Unmarshal([]byte(`{"EmailSubjectTemplate":"A custom subject","EmailBodyTemplate":""}`), profile))
	require.NoError(t, Invoices.SaveProfile(ctx, profile))
	saved, err := Invoices.GetProfile(ctx, uid)
	require.NoError(t, err)
	encoded, err = json.Marshal(saved.ToInvoiceProfileResponse())
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(encoded, &response))
	require.Equal(t, "A custom subject", response["emailSubjectTemplate"])
	require.Equal(t, "", response["emailBodyTemplate"])
	public, err := json.Marshal((&models.Invoice{}).ToPublicInvoiceResponse(nil, 0, saved))
	require.NoError(t, err)
	require.NotContains(t, string(public), "emailSubjectTemplate")
	require.NotContains(t, string(public), "emailBodyTemplate")
	require.NotContains(t, string(public), "A custom subject")
}

func TestInvoiceEmailProfileRejectsInvalidTemplates(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	for _, fields := range []map[string]string{
		{"EmailSubjectTemplate": "A\nB"},
		{"EmailSubjectTemplate": "{{unknown}}"},
		{"EmailSubjectTemplate": "Invoice", "EmailBodyTemplate": `<span data-invoice-variable="unknown">x</span>`},
		{"EmailSubjectTemplate": "Invoice", "EmailBodyTemplate": strings.Repeat("x", 32001)},
	} {
		profile := &models.InvoiceProfile{Uid: uid, NumberPrefix: "INV"}
		encoded, err := json.Marshal(fields)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(encoded, profile))
		require.Error(t, Invoices.SaveProfile(ctx, profile))
	}
}

func stringPointer(value string) *string { return &value }

func TestInvoiceEmailPreviewSampleIsDeterministicAndUnsavedOverridesAreNotStored(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	first, err := Invoices.PreviewInvoiceEmail(ctx, uid, 0, nil, nil)
	require.NoError(t, err)
	second, err := Invoices.PreviewInvoiceEmail(ctx, uid, 0, nil, nil)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.True(t, first.Sample)
	require.Contains(t, first.Subject, "INV-2026-001")
	require.Contains(t, first.BodyHtml, "Sample Business")
	require.Len(t, first.Variables, 16)
	empty, err := Invoices.PreviewInvoiceEmail(ctx, uid, 0, stringPointer("Unsaved subject"), stringPointer(""))
	require.NoError(t, err)
	require.Empty(t, empty.BodyHtml)
	require.Equal(t, "Unsaved subject", empty.Subject)
	profile, err := Invoices.GetProfile(ctx, uid)
	require.NoError(t, err)
	require.Empty(t, profile.EmailSubjectTemplate)
}

func TestInvoiceEmailPreviewUsesOwnedInvoiceAndHasNoPublicTokenOrTimeSideEffects(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "REAL-123", 100000)
	accountId := accountIdByName(t, ctx, uid, "Savings")
	payment := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 25000, "2026-04-28")
	require.NoError(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, payment.TransactionId, 0))
	result, err := Invoices.PreviewInvoiceEmail(ctx, uid, invoice.InvoiceId, stringPointer("{{invoice_number}}"), stringPointer("<p>{{amount_paid}} {{amount_due}}</p>"))
	require.NoError(t, err)
	require.False(t, result.Sample)
	require.Equal(t, "REAL-123", result.Subject)
	require.Contains(t, result.BodyHtml, "₹250.00 ₹750.00")
	reloaded, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	require.NoError(t, err)
	require.Empty(t, reloaded.PublicToken)
	require.Zero(t, reloaded.SentUnixTime)
	require.Equal(t, invoice.UpdatedUnixTime, reloaded.UpdatedUnixTime)
	_, otherUID := newAlertTestUser(t)
	_, err = Invoices.PreviewInvoiceEmail(ctx, otherUID, invoice.InvoiceId, nil, nil)
	require.ErrorIs(t, err, errs.ErrInvoiceNotFound)
}

type invoiceEmailCaptureMailer struct {
	messages []*mail.MailMessage
	failure  error
}

func (f *invoiceEmailCaptureMailer) SendMail(message *mail.MailMessage) error {
	f.messages = append(f.messages, message)
	return f.failure
}

func TestInvoiceEmailRichSendMatchesActualPreviewAndAttachesPDF(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	withSmtpEnabled(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "REAL-42", 100000)
	token, err := Invoices.SetInvoicePublicToken(ctx, uid, invoice.InvoiceId, true)
	require.NoError(t, err)
	require.NotEmpty(t, token)
	profile, err := Invoices.GetProfile(ctx, uid)
	require.NoError(t, err)
	profile.BusinessName = "Actual Business"
	profile.EmailSubjectTemplate = "Saved {{invoice_number}}"
	profile.EmailBodyTemplate = "<h1>{{business_name}}</h1>"
	require.NoError(t, Invoices.SaveProfile(ctx, profile))
	subject, body := "Hello {{contact_name}}", `<p>{{invoice_number}}</p><div data-invoice-button="true">View actual invoice</div>`
	preview, err := Invoices.PreviewInvoiceEmail(ctx, uid, invoice.InvoiceId, &subject, &body)
	require.NoError(t, err)
	fake := &invoiceEmailCaptureMailer{}
	service := *Invoices
	service.ServiceUsingMailer.container = fake
	require.NoError(t, service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "recipient@example.com", subject, nil, &body))
	require.Len(t, fake.messages, 1)
	message := fake.messages[0]
	require.Equal(t, preview.Subject, message.Subject)
	require.Equal(t, preview.EmailHtml, message.Body)
	require.Equal(t, "recipient@example.com", message.To)
	require.Len(t, message.Attachments, 1)
	require.Equal(t, "Invoice_REAL-42_2026-04-30.pdf", message.Attachments[0].Name)
	require.True(t, bytes.HasPrefix(message.Attachments[0].Content, []byte("%PDF")))
	sent, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	require.NoError(t, err)
	require.Positive(t, sent.SentUnixTime)
	// A present empty rich body wins over even a supplied legacy message.
	require.NoError(t, service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "recipient@example.com", "Invoice", stringPointer("Legacy should not appear"), stringPointer("")))
	require.NotContains(t, fake.messages[1].Body, "Legacy should not appear")
	require.NotContains(t, fake.messages[1].Body, "View actual invoice")
	require.NoError(t, service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "recipient@example.com", "", nil, nil))
	require.Equal(t, "Saved REAL-42", fake.messages[2].Subject)
	require.Contains(t, fake.messages[2].Body, "Actual Business")
}

func TestInvoiceEmailLegacySendEscapesTextAndFailedRichSendPreservesTimestamp(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	withSmtpEnabled(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "LEGACY-1", 100000)
	fake := &invoiceEmailCaptureMailer{}
	service := *Invoices
	service.ServiceUsingMailer.container = fake
	require.NoError(t, service.SendInvoice(ctx, uid, invoice.InvoiceId, "recipient@example.com", "", "Line one\n<script>text</script> {{unknown}}"))
	require.Contains(t, fake.messages[0].Body, "Line one")
	require.Contains(t, fake.messages[0].Body, "&lt;script&gt;text&lt;/script&gt; {{unknown}}")
	require.Contains(t, fake.messages[0].Body, "View invoice")
	before, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	require.NoError(t, err)
	fake.failure = errors.New("SMTP delivery failed")
	err = service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "recipient@example.com", "Invoice", nil, stringPointer("<p>Rich body</p>"))
	require.EqualError(t, err, "SMTP delivery failed")
	after, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	require.NoError(t, err)
	require.Equal(t, before.SentUnixTime, after.SentUnixTime)
	require.Error(t, service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "recipient@example.com", "{{unknown}}", nil, stringPointer("")))
	require.Len(t, fake.messages, 2)
}

// Creating the public link before the template renders breaks this test.
func TestInvoiceEmailRejectedTemplateDoesNotPublishInvoice(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	withSmtpEnabled(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "PRIVATE-1", 100000)
	service := *Invoices
	service.ServiceUsingMailer.container = &invoiceEmailCaptureMailer{}
	require.Error(t, service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "recipient@example.com", "Invoice", nil, stringPointer("<p>{{unknown}}</p>")))
	reloaded, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	require.NoError(t, err)
	require.Empty(t, reloaded.PublicToken)
	require.NoError(t, service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "recipient@example.com", "Invoice", nil, stringPointer("<p>ok</p>")))
	reloaded, _, err = Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	require.NoError(t, err)
	require.NotEmpty(t, reloaded.PublicToken)
}

func TestInvoiceEmailValuesUseSettledPaymentCurrencyAndPaidTolerance(t *testing.T) {
	invoice := &models.Invoice{Currency: "USD", Total: 100000, InvoiceDate: "2026-04-23", DueDate: "2026-04-30", BillToName: "Fallback name"}
	values := invoiceEmailValues(&models.InvoiceProfile{}, invoice, 25000, "https://example.com/i/real")
	require.Equal(t, "$1,000.00", values["invoice_total"])
	require.Equal(t, "$250.00", values["amount_paid"])
	require.Equal(t, "$750.00", values["amount_due"])
	require.Equal(t, "April 23, 2026", values["invoice_date"])
	require.Equal(t, "Fallback name", values["contact_name"])
	values = invoiceEmailValues(&models.InvoiceProfile{}, invoice, 98000, "")
	require.Equal(t, "$0.00", values["amount_due"])
	invoice.ClosedManually = true
	values = invoiceEmailValues(&models.InvoiceProfile{}, invoice, 0, "")
	require.Equal(t, "$0.00", values["amount_due"])
}

func TestInvoiceEmailRichSendRejectsWhitespaceHeaderLineBreaks(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	withSmtpEnabled(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INV-LINEBREAK", 100000)
	fake := &invoiceEmailCaptureMailer{}
	service := *Invoices
	service.ServiceUsingMailer.container = fake
	require.Error(t, service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "recipient@example.com", "\r\n", nil, stringPointer("")))
	require.Empty(t, fake.messages)
}

func TestInvoiceEmailPreviewUsesCrossCurrencySettledPayments(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	accountID := createTestAccount(t, ctx, uid, "USD receipts", "USD")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "INR-SETTLED", 100000)
	payment := insertTestTransaction(t, uid, accountID, models.TRANSACTION_DB_TYPE_INCOME, 500, "2026-04-28")
	require.NoError(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, payment.TransactionId, 25000))
	preview, err := Invoices.PreviewInvoiceEmail(ctx, uid, invoice.InvoiceId, stringPointer("Invoice"), stringPointer("<p>{{amount_paid}} {{amount_due}}</p>"))
	require.NoError(t, err)
	require.Contains(t, preview.BodyHtml, "₹250.00 ₹750.00")
}

// Loading the preview profile with a defaults-filled struct turns those defaults into query filters and drops business values.
func TestInvoiceEmailPreviewFillsBusinessValuesForNonDefaultProfile(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	require.NoError(t, Invoices.SaveBusinessProfile(ctx, &models.InvoiceProfile{Uid: uid, NumberPrefix: "CDN", DefaultTermsDays: 15, BusinessName: "Codeation", Phone: "98765", Website: "https://codeation.io"}))
	preview, err := Invoices.PreviewInvoiceEmail(ctx, uid, 0, nil, nil)
	require.NoError(t, err)
	for _, value := range []string{"Codeation", "98765", "https://codeation.io"} {
		require.Contains(t, preview.BodyHtml, value)
	}
}

// Overwriting a single sent time, logging failed sends, or storing anything but the delivered HTML breaks this test.
func TestInvoiceEmailHistoryKeepsEverySuccessfulSend(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	withSmtpEnabled(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "HISTORY-1", 100000)
	fake := &invoiceEmailCaptureMailer{}
	service := *Invoices
	service.ServiceUsingMailer.container = fake
	require.NoError(t, service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "first@example.com", "First", nil, stringPointer("<p>One</p>")))
	require.NoError(t, service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "second@example.com", "Second", nil, stringPointer("<p>Two</p>")))
	fake.failure = errors.New("SMTP delivery failed")
	require.Error(t, service.SendInvoiceWithContent(ctx, uid, invoice.InvoiceId, "third@example.com", "Third", nil, stringPointer("<p>Three</p>")))

	emails, err := Invoices.GetInvoiceEmails(ctx, uid, invoice.InvoiceId)
	require.NoError(t, err)
	require.Len(t, emails, 2)
	subjects := []string{emails[0].Subject, emails[1].Subject}
	require.ElementsMatch(t, []string{"First", "Second"}, subjects)
	require.Empty(t, emails[0].EmailHtml, "the list omits the content")
	for _, listed := range emails {
		full, err := Invoices.GetInvoiceEmail(ctx, uid, listed.EmailId)
		require.NoError(t, err)
		sentBody := fake.messages[0].Body
		if full.Subject == "Second" {
			sentBody = fake.messages[1].Body
			require.Equal(t, "second@example.com", full.ToAddress)
		}
		require.Equal(t, sentBody, full.EmailHtml)
	}
	_, err = Invoices.GetInvoiceEmail(ctx, uid+1, emails[0].EmailId)
	require.Error(t, err, "another user cannot read the email")
}
