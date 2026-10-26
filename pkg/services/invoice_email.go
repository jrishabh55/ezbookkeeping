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
)

func invoiceEmailTemplateError(err error) error {
	return errs.NewNormalError(errs.NormalSubcategoryInvoice, 50, http.StatusBadRequest, err.Error())
}

// SaveProfileWithEmailTemplates preserves fields omitted by older clients and permits an empty rich body.
func (s *InvoiceService) SaveProfileWithEmailTemplates(c core.Context, profile *models.InvoiceProfile, subject, body *string) error {
	if subject == nil && body == nil {
		return s.saveProfile(c, profile, true)
	}
	previous, err := s.GetProfile(c, profile.Uid)
	if err != nil {
		return err
	}
	profile.EmailSubjectTemplate, profile.EmailBodyTemplate = previous.EmailSubjectTemplate, previous.EmailBodyTemplate
	if subject != nil {
		if err := invoiceemail.ValidateSubject(*subject); err != nil {
			return invoiceEmailTemplateError(err)
		}
		if previous.EmailSubjectTemplate == "" && body == nil {
			_, profile.EmailBodyTemplate = invoiceemail.EffectiveTemplates(previous.EmailSubjectTemplate, previous.EmailBodyTemplate)
		}
		profile.EmailSubjectTemplate = *subject
	}
	if body != nil {
		if profile.EmailSubjectTemplate == "" {
			profile.EmailSubjectTemplate = invoiceemail.DefaultSubjectTemplate
		}
		profile.EmailBodyTemplate = *body
	}
	return s.SaveProfile(c, profile)
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
	rendered, err := invoiceemail.Render(subjectTemplate, bodyTemplate, invoiceEmailValues(profile, invoice, received, publicURL))
	if err != nil {
		return nil, invoiceEmailTemplateError(err)
	}
	return rendered, nil
}

// PreviewInvoiceEmail reads sample or owned invoice data. It never creates a public token or updates an invoice.
func (s *InvoiceService) PreviewInvoiceEmail(c core.Context, uid, invoiceID int64, subject, body *string) (*models.InvoiceEmailPreviewResponse, error) {
	profile, err := s.GetProfile(c, uid)
	if err != nil {
		return nil, err
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
		invoice, _, err = s.GetInvoice(c, uid, invoiceID)
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
				subject = invoiceEmailSubject(invoice.Number, profile.BusinessName)
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
	pdf, err := invoicepdf.Render(profile, invoice, lines, received, publicURL)
	if err != nil {
		return err
	}
	if newToken {
		if err := s.saveInvoicePublicToken(c, uid, invoiceID, invoice.PublicToken); err != nil {
			return err
		}
	}
	err = s.SendMail(&mail.MailMessage{To: to, Subject: rendered.Subject, Body: rendered.EmailHTML, Attachments: []mail.MailAttachment{{Name: invoicepdf.FileName(invoice.Number, invoice.DueDate), Content: pdf}}})
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Cols("sent_unix_time", "updated_unix_time").Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceID).Update(&models.Invoice{SentUnixTime: now, UpdatedUnixTime: now})
		return err
	})
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
		if !exists {
			now := time.Now().Unix()
			profile = &models.InvoiceProfile{Uid: uid, NumberPrefix: "INV", DefaultTermsDays: 7, CreatedUnixTime: now, UpdatedUnixTime: now}
		}
		profile.EmailSubjectTemplate = subject
		profile.EmailBodyTemplate = sanitizedBody
		if exists {
			_, err = sess.Cols("email_subject_template", "email_body_template").Where("uid=?", uid).Update(profile)
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
