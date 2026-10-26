package services

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// Updating an entire stale/partial profile instead of two template columns would lose these saved business details.
func TestSaveInvoiceEmailTemplatePreservesEveryOtherProfileField(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	profile := &models.InvoiceProfile{Uid: uid, BusinessName: "Saved Business", AddressLines: "Line 1\nLine 2", Phone: "123456", Website: "https://saved.example", LogoDataUrl: "data:image/png;base64,aGVsbG8=", HideLogo: true, DefaultNotes: "Business notes", NumberPrefix: "SAVED", DefaultTermsDays: 31, Gstin: "07AAAAA0000A1Z5", DefaultSac: "998314", DefaultCurrency: "USD", EmailSubjectTemplate: "Original", EmailBodyTemplate: "<p>Original</p>"}
	require.NoError(t, Invoices.SaveProfile(ctx, profile))
	before, err := Invoices.GetProfile(ctx, uid)
	require.NoError(t, err)
	result, err := Invoices.SaveInvoiceEmailTemplate(ctx, uid, "Updated {{invoice_number}}", `<p>Changed <span data-invoice-variable="invoice_number">Sample value</span><script>bad()</script></p>`)
	require.NoError(t, err)
	require.NotNil(t, result)
	expected := *before
	expected.EmailSubjectTemplate = "Updated {{invoice_number}}"
	expected.EmailBodyTemplate = `<p>Changed <span data-invoice-variable="invoice_number">{{invoice_number}}</span></p>`
	require.Equal(t, &expected, result)
	stored, err := Invoices.GetProfile(ctx, uid)
	require.NoError(t, err)
	require.Equal(t, &expected, stored)
	// Updating just the template also persists an intentionally empty rich body.
	result, err = Invoices.SaveInvoiceEmailTemplate(ctx, uid, "Empty body", "")
	require.NoError(t, err)
	expected.EmailSubjectTemplate = "Empty body"
	expected.EmailBodyTemplate = ""
	require.Equal(t, &expected, result)
	stored, err = Invoices.GetProfile(ctx, uid)
	require.NoError(t, err)
	require.Equal(t, &expected, stored)
}

func TestSaveInvoiceEmailTemplateCreatesProfileWithExistingDefaultsAndScopesOwnership(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUID := newAlertTestUser(t)
	other := &models.InvoiceProfile{Uid: otherUID, NumberPrefix: "OTHER", BusinessName: "Other Business", EmailSubjectTemplate: "Other template", EmailBodyTemplate: "<p>Other body</p>"}
	require.NoError(t, Invoices.SaveProfile(ctx, other))
	result, err := Invoices.SaveInvoiceEmailTemplate(ctx, uid, "Invoice {{invoice_number}}", "")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, uid, result.Uid)
	require.Equal(t, "INV", result.NumberPrefix)
	require.Equal(t, int32(7), result.DefaultTermsDays)
	require.Empty(t, result.BusinessName)
	require.Empty(t, result.DefaultCurrency)
	require.Positive(t, result.CreatedUnixTime)
	require.Equal(t, result.CreatedUnixTime, result.UpdatedUnixTime)
	require.Equal(t, "Invoice {{invoice_number}}", result.EmailSubjectTemplate)
	require.Empty(t, result.EmailBodyTemplate)
	savedOther, err := Invoices.GetProfile(ctx, otherUID)
	require.NoError(t, err)
	require.Equal(t, other, savedOther)
	_, err = Invoices.SaveInvoiceEmailTemplate(ctx, 0, "Invoice", "")
	require.ErrorIs(t, err, errs.ErrUserIdInvalid)
}

func TestSaveInvoiceEmailTemplateValidationRejectsWithoutCreatingOrChangingProfile(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	invalid := []struct{ subject, body string }{
		{"", ""}, {"Invoice\nBcc: x", ""}, {strings.Repeat("x", 257), ""}, {"{{unknown}}", ""},
		{"Invoice", `<span data-invoice-variable="unknown">x</span>`}, {"Invoice", strings.Repeat("x", 32001)},
	}
	for _, test := range invalid {
		_, err := Invoices.SaveInvoiceEmailTemplate(ctx, uid, test.subject, test.body)
		require.Error(t, err)
	}
	count, err := Invoices.UserDataDB(uid).NewSession(ctx).Where("uid=?", uid).Count(&models.InvoiceProfile{})
	require.NoError(t, err)
	require.Zero(t, count)
	profile := &models.InvoiceProfile{Uid: uid, NumberPrefix: "INV", BusinessName: "Keep me", EmailSubjectTemplate: "Original", EmailBodyTemplate: "<p>Original</p>"}
	require.NoError(t, Invoices.SaveProfile(ctx, profile))
	for _, test := range invalid {
		_, err := Invoices.SaveInvoiceEmailTemplate(ctx, uid, test.subject, test.body)
		require.Error(t, err)
		saved, err := Invoices.GetProfile(ctx, uid)
		require.NoError(t, err)
		require.Equal(t, profile, saved)
	}
}

// Reproduce the interleaving at the DB-write boundary without timing-dependent threads:
// the business request read template A, the dedicated editor saved B, then the business write resumes.
func TestBusinessProfileWritePreservesTemplateSavedAfterSnapshotRead(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	original := &models.InvoiceProfile{Uid: uid, NumberPrefix: "INV", BusinessName: "Original business", EmailSubjectTemplate: "Template A", EmailBodyTemplate: "<p>Body A</p>"}
	require.NoError(t, Invoices.SaveProfile(ctx, original))
	stale, err := Invoices.GetProfile(ctx, uid)
	require.NoError(t, err)
	_, err = Invoices.SaveInvoiceEmailTemplate(ctx, uid, "Template B", "<p>Body B</p>")
	require.NoError(t, err)
	stale.BusinessName = "Business form saved later"
	require.NoError(t, Invoices.saveProfile(ctx, stale, true))
	stored, err := Invoices.GetProfile(ctx, uid)
	require.NoError(t, err)
	require.Equal(t, "Template B", stored.EmailSubjectTemplate)
	require.Equal(t, "<p>Body B</p>", stored.EmailBodyTemplate)
	require.Equal(t, "Business form saved later", stored.BusinessName)
	require.Equal(t, stored, stale, "the response model must contain the preserved current template, not A")
}

func TestBusinessProfileOmittedTemplateRequestDoesNotWriteTemplateColumns(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	require.NoError(t, Invoices.SaveProfile(ctx, &models.InvoiceProfile{Uid: uid, NumberPrefix: "INV", EmailSubjectTemplate: "Keep template", EmailBodyTemplate: "<p>Keep body</p>"}))
	// The database rejects even a same-value template-column write, making omission observable deterministically.
	trigger := fmt.Sprintf("business_template_omission_%d", uid)
	_, err := Invoices.UserDataDB(uid).NewSession(ctx).Exec(fmt.Sprintf("CREATE TRIGGER %s BEFORE UPDATE OF email_subject_template, email_body_template ON invoice_profile WHEN OLD.uid = %d BEGIN SELECT RAISE(ABORT, 'business save wrote template columns'); END", trigger, uid))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := Invoices.UserDataDB(uid).NewSession(ctx).Exec("DROP TRIGGER IF EXISTS " + trigger)
		require.NoError(t, err)
	})
	incoming := &models.InvoiceProfile{Uid: uid, NumberPrefix: "NEW", BusinessName: "Updated business"}
	require.NoError(t, Invoices.SaveProfileWithEmailTemplates(ctx, incoming, nil, nil))
	require.Equal(t, "Keep template", incoming.EmailSubjectTemplate)
	require.Equal(t, "<p>Keep body</p>", incoming.EmailBodyTemplate)
	stored, err := Invoices.GetProfile(ctx, uid)
	require.NoError(t, err)
	require.Equal(t, "Updated business", stored.BusinessName)
	require.Equal(t, stored, incoming)
}
