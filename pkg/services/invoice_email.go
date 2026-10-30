package services

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/invoiceemail"
	"github.com/mayswind/ezbookkeeping/pkg/invoicepdf"
	"github.com/mayswind/ezbookkeeping/pkg/mail"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

func invoiceEmailTemplateError(err error) error {
	return errs.NewNormalError(errs.NormalSubcategoryInvoice, 50, http.StatusBadRequest, err.Error())
}

// SaveBusinessProfile saves the business fields; templates are only written by SaveInvoiceEmailTemplate.
func (s *InvoiceService) SaveBusinessProfile(c core.Context, profile *models.InvoiceProfile) error {
	return s.saveProfile(c, profile, true)
}

// invoiceEmailValues uses invoice snapshots and the same formatting/balance rules as the printed document.
func invoiceEmailValues(profile *models.InvoiceProfile, invoice *models.Invoice, received int64, publicURL string) map[string]string {
	contactName := invoice.BillToContactName
	if contactName == "" {
		contactName = invoice.BillToName
	}
	return map[string]string{
		"customer_name": invoice.BillToName, "contact_name": contactName, "customer_email": invoice.BillToEmail,
		"invoice_number": invoice.Number, "invoice_date": invoicepdf.FormatDate(invoice.InvoiceDate), "due_date": invoicepdf.FormatDate(invoice.DueDate),
		"invoice_total": invoicepdf.FormatAmount(invoice.Total, invoice.Currency), "amount_paid": invoicepdf.FormatAmount(received, invoice.Currency),
		"amount_due": invoicepdf.FormatAmount(invoicepdf.BalanceDue(invoice, received), invoice.Currency), "currency": invoice.Currency, "invoice_url": publicURL,
		"business_name": profile.BusinessName, "business_address": profile.AddressLines, "business_phone": profile.Phone, "business_website": profile.Website,
		"document_type": invoicepdf.DocumentTitle(invoice),
	}
}

func renderInvoiceEmail(profile *models.InvoiceProfile, invoice *models.Invoice, received int64, publicURL string, subject, body *string) (*invoiceemail.Result, error) {
	subjectTemplate, bodyTemplate := invoiceemail.EffectiveTemplates(profile.EmailSubjectTemplate, profile.EmailBodyTemplate)
	if profile.EmailSubjectTemplate == "" && profile.BusinessName == "" {
		subjectTemplate, bodyTemplate = invoiceemail.PresetWithoutBusinessName()
	}
	if subject != nil {
		subjectTemplate = *subject
	}
	if body != nil {
		bodyTemplate = *body
	}
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		subjectTemplate, bodyTemplate = invoiceemail.EstimateTemplates(subjectTemplate, bodyTemplate)
	}
	rendered, err := invoiceemail.Render(subjectTemplate, bodyTemplate, invoiceEmailValues(profile, invoice, received, publicURL))
	if err != nil {
		return nil, invoiceEmailTemplateError(err)
	}
	return rendered, nil
}

// PreviewInvoiceEmail reads sample or owned invoice data. It never creates a public token or updates an invoice.
func (s *InvoiceService) PreviewInvoiceEmail(c core.Context, uid, invoiceID int64, subject, body *string) (*models.InvoiceEmailPreviewResponse, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	// Runs on every debounced keystroke: skip the logo blob and invoice lines, which the email never uses.
	profile := &models.InvoiceProfile{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Omit("logo_data_url").Get(profile)
	if err != nil {
		return nil, err
	} else if !has {
		profile = defaultInvoiceProfile(uid)
	}
	sample := invoiceID == 0
	var invoice *models.Invoice
	received := int64(0)
	publicURL := ""
	if sample {
		sampleProfile := *profile
		if sampleProfile.BusinessName == "" {
			sampleProfile.BusinessName = "Sample Business"
		}
		profile = &sampleProfile
		invoice = &models.Invoice{Number: "INV-2026-001", BillToName: "Sample Customer", BillToContactName: "Alex", BillToEmail: "alex@example.com", Currency: models.INVOICE_DEFAULT_CURRENCY, InvoiceDate: "2026-10-01", DueDate: "2026-10-15", Total: 100000}
		if profile.DefaultCurrency != "" {
			invoice.Currency = profile.DefaultCurrency
		}
		received = 25000
		publicURL = "https://example.com/i/sample-invoice"
	} else {
		invoice, err = s.getInvoiceHeader(c, uid, invoiceID)
		if err != nil {
			return nil, err
		}
		payments, err := s.GetInvoicePayments(c, uid, []int64{invoiceID})
		if err != nil {
			return nil, err
		}
		received = models.InvoiceReceivedAmount(payments[invoiceID])
		publicURL = s.InvoicePublicUrl(invoice.PublicToken)
	}
	rendered, err := renderInvoiceEmail(profile, invoice, received, publicURL, subject, body)
	if err != nil {
		return nil, err
	}
	return &models.InvoiceEmailPreviewResponse{Subject: rendered.Subject, BodyHtml: rendered.BodyHTML, EmailHtml: rendered.EmailHTML, Variables: rendered.Variables, DefaultSubjectTemplate: invoiceemail.DefaultSubjectTemplate, DefaultBodyTemplate: invoiceemail.DefaultBodyTemplate, Sample: sample}, nil
}

// SendInvoiceWithContent supports an intentional empty rich body, legacy plain text and saved defaults.
func (s *InvoiceService) SendInvoiceWithContent(c core.Context, uid, invoiceID int64, to, subject string, message, body *string) error {
	if !s.CurrentConfig().EnableSMTP {
		return errs.ErrSMTPServerNotEnabled
	}
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	if invoiceID <= 0 {
		return errs.ErrInvoiceIdInvalid
	}
	if strings.ContainsAny(subject, "\r\n") {
		return invoiceEmailTemplateError(fmt.Errorf("email subject must be a single line"))
	}
	invoice, lines, err := s.GetInvoice(c, uid, invoiceID)
	if err != nil {
		return err
	}
	// a superseded version is read-only: the client already saw it and the latest version is what gets sent
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE && invoice.EstimateStatus == models.ESTIMATE_STATUS_SUPERSEDED {
		return errs.ErrEstimateStatusInvalid
	}
	payments, err := s.GetInvoicePayments(c, uid, []int64{invoiceID})
	if err != nil {
		return err
	}
	received := models.InvoiceReceivedAmount(payments[invoiceID])
	profile, err := s.GetProfile(c, uid)
	if err != nil {
		return err
	}
	// Render with a provisional token first; the link is only persisted once the email is known to render.
	newToken := invoice.PublicToken == ""
	if newToken {
		token, err := generateInvoicePublicToken()
		if err != nil {
			return err
		}
		invoice.PublicToken = token
	}
	publicURL := s.InvoicePublicUrl(invoice.PublicToken)
	var rendered *invoiceemail.Result
	if body == nil && message != nil {
		values := invoiceEmailValues(profile, invoice, received, publicURL)
		if strings.TrimSpace(subject) == "" {
			if profile.EmailSubjectTemplate != "" {
				emptyBody := ""
				resolved, err := renderInvoiceEmail(profile, invoice, received, publicURL, nil, &emptyBody)
				if err != nil {
					return err
				}
				subject = resolved.Subject
			} else {
				subject = invoiceEmailSubject(invoice.Kind, invoice.Number, profile.BusinessName)
			}
		}
		rendered, err = invoiceemail.RenderPlainText(subject, *message, values)
		if err != nil {
			return invoiceEmailTemplateError(err)
		}
	} else {
		var subjectOverride *string
		if strings.TrimSpace(subject) != "" {
			subjectOverride = &subject
		}
		rendered, err = renderInvoiceEmail(profile, invoice, received, publicURL, subjectOverride, body)
		if err != nil {
			return err
		}
	}
	var milestones []*models.EstimateMilestone
	if invoice.Kind == models.INVOICE_KIND_ESTIMATE {
		if milestones, err = s.GetEstimateMilestones(c, uid, invoiceID); err != nil {
			return err
		}
	}
	pdf, err := invoicepdf.RenderWithMilestones(profile, invoice, lines, milestones, received, publicURL)
	if err != nil {
		return err
	}
	if newToken {
		if err := s.saveInvoicePublicToken(c, uid, invoiceID, invoice.PublicToken); err != nil {
			return err
		}
	}
	err = s.SendMail(&mail.MailMessage{To: to, Subject: rendered.Subject, Body: rendered.EmailHTML, Attachments: []mail.MailAttachment{{Name: invoicepdf.DocumentFileName(invoice), Content: pdf}}})
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	sent := &models.InvoiceEmail{EmailId: s.GenerateUuid(uuid.UUID_TYPE_INVOICE), Uid: uid, InvoiceId: invoiceID, ToAddress: to, Subject: rendered.Subject, EmailHtml: rendered.EmailHTML, SentUnixTime: now}
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := sess.Insert(sent); err != nil {
			return err
		}
		if _, err := sess.Cols("sent_unix_time", "updated_unix_time").Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceID).Update(&models.Invoice{SentUnixTime: now, UpdatedUnixTime: now}); err != nil {
			return err
		}
		// emailing a draft estimate sends it; an answered one keeps its answer
		_, err := sess.Cols("estimate_status").Where("uid=? AND deleted=? AND invoice_id=? AND kind=? AND estimate_status=?", uid, false, invoiceID, models.INVOICE_KIND_ESTIMATE, models.ESTIMATE_STATUS_DRAFT).
			Update(&models.Invoice{EstimateStatus: models.ESTIMATE_STATUS_SENT})
		return err
	})
}

// GetInvoiceEmails returns the emails sent for an invoice, newest first, without their content
func (s *InvoiceService) GetInvoiceEmails(c core.Context, uid, invoiceID int64) ([]*models.InvoiceEmail, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	var emails []*models.InvoiceEmail
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND invoice_id=?", uid, invoiceID).Omit("email_html").OrderBy("sent_unix_time desc").Find(&emails)
	return emails, err
}

// GetInvoiceEmail returns one sent email of the user, including the HTML that was sent
func (s *InvoiceService) GetInvoiceEmail(c core.Context, uid, emailID int64) (*models.InvoiceEmail, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	email := &models.InvoiceEmail{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND email_id=?", uid, emailID).Get(email)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrInvoiceNotFound
	}
	return email, nil
}

// SaveInvoiceEmailTemplate saves only the authenticated user's email template.
func (s *InvoiceService) SaveInvoiceEmailTemplate(c core.Context, uid int64, subject, body string) (*models.InvoiceProfile, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if err := invoiceemail.ValidateSubject(subject); err != nil {
		return nil, invoiceEmailTemplateError(err)
	}
	sanitizedBody, err := invoiceemail.SanitizeTemplate(body)
	if err != nil {
		return nil, invoiceEmailTemplateError(err)
	}
	profile := &models.InvoiceProfile{}
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		exists, err := sess.Where("uid=?", uid).Get(profile)
		if err != nil {
			return err
		}
		now := time.Now().Unix()
		if !exists {
			profile = defaultInvoiceProfile(uid)
			profile.CreatedUnixTime = now
		}
		profile.EmailSubjectTemplate = subject
		profile.EmailBodyTemplate = sanitizedBody
		profile.UpdatedUnixTime = now
		if exists {
			_, err = sess.Cols("email_subject_template", "email_body_template", "updated_unix_time").Where("uid=?", uid).Update(profile)
		} else {
			_, err = sess.Insert(profile)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	return profile, nil
}
