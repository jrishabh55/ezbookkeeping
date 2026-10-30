package services

import (
	"html"
	"strings"
	"time"
	"unicode/utf8"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/invoicepdf"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/mail"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// RespondToEstimateByPublicToken records the client's answer from the public link. Only a sent estimate that is still valid
// (today on or before its valid-until date) can be answered; a decline may carry a plain-text note of at most 500 characters.
// An empty, unknown or disabled token is a plain not-found. The owner is emailed afterwards; a mail failure is only logged.
func (s *InvoiceService) RespondToEstimateByPublicToken(c core.Context, token string, status string, note string, today string) error {
	if status != models.ESTIMATE_STATUS_ACCEPTED && status != models.ESTIMATE_STATUS_DECLINED {
		return errs.ErrEstimateStatusInvalid
	}

	note = strings.TrimSpace(note)

	if utf8.RuneCountInString(note) > models.EstimateResponseNoteMaxLength {
		return errs.ErrEstimateResponseNoteTooLong
	}

	if status == models.ESTIMATE_STATUS_ACCEPTED {
		note = ""
	}

	if len(token) != models.InvoicePublicTokenLength {
		return errs.ErrInvoiceNotFound
	}

	// ponytail: a public link carries no uid, so the lookup goes to the first user-data store, as GetInvoiceByPublicToken does
	estimate := &models.Invoice{}
	has, err := s.UserDataDB(0).NewSession(c).Where("public_token=? AND deleted=?", token, false).Get(estimate)

	if err != nil {
		return err
	} else if !has {
		return errs.ErrInvoiceNotFound
	}

	if estimate.Kind != models.INVOICE_KIND_ESTIMATE || estimate.EstimateStatus != models.ESTIMATE_STATUS_SENT || estimate.DueDate < today {
		return errs.ErrEstimateCannotBeAnswered
	}

	recorded, err := s.recordEstimateClientResponse(c, estimate, token, status, note, today)

	if err != nil {
		return err
	} else if !recorded {
		return errs.ErrEstimateCannotBeAnswered
	}

	s.notifyEstimateResponse(c, estimate)
	return nil
}

// recordEstimateClientResponse writes the answer only while the stored estimate is still sent, still valid today and its link
// unchanged, so of two concurrent clicks exactly one is recorded, and an owner who shortens the valid-until date while a click is
// in flight cannot let an expired answer through; it reports whether this one was. The estimate is updated in memory when
// recorded. Like SetEstimateResponse, it locks the estimate's own row first (lock order: an invoice's row before its estimate,
// never the reverse - this path only ever locks the estimate) and refuses a decline once a non-deleted invoice references the
// estimate; an accept is always fine.
func (s *InvoiceService) recordEstimateClientResponse(c core.Context, estimate *models.Invoice, token string, status string, note string, today string) (bool, error) {
	now := time.Now().Unix()
	recorded := false

	err := s.UserDataDB(estimate.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := lockInvoiceRow(sess, estimate.Uid, estimate.InvoiceId, now); err != nil {
			return err
		}

		if status == models.ESTIMATE_STATUS_DECLINED {
			invoiced, err := estimateHasInvoices(sess, estimate.Uid, estimate.InvoiceId)

			if err != nil {
				return err
			} else if invoiced {
				return errs.ErrEstimateCannotBeAnswered
			}
		}

		updatedRows, err := sess.Cols("estimate_status", "responded_unix_time", "responded_by_client", "response_note", "updated_unix_time").
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=? AND public_token=? AND estimate_status=? AND due_date>=?",
				estimate.Uid, false, models.INVOICE_KIND_ESTIMATE, estimate.InvoiceId, token, models.ESTIMATE_STATUS_SENT, today).
			Update(&models.Invoice{EstimateStatus: status, RespondedUnixTime: now, RespondedByClient: true, ResponseNote: note, UpdatedUnixTime: now})

		recorded = updatedRows > 0
		return err
	})

	if err != nil || !recorded {
		return false, err
	}

	estimate.EstimateStatus = status
	estimate.RespondedUnixTime = now
	estimate.RespondedByClient = true
	estimate.ResponseNote = note

	return true, nil
}

// estimateResponseEmail is the owner notification: "<customer> accepted Estimate <number> (<total>)" (or declined, with the
// client's note as plain text) and a link that opens the estimate in the app
func estimateResponseEmail(estimate *models.Invoice, rootUrl string) (string, string) {
	verb := "accepted"

	if estimate.EstimateStatus == models.ESTIMATE_STATUS_DECLINED {
		verb = "declined"
	}

	customer := strings.NewReplacer("\r", " ", "\n", " ").Replace(estimate.BillToName)
	subject := customer + " " + verb + " Estimate " + estimate.Number + " (" + invoicepdf.FormatAmount(estimate.Total, estimate.Currency) + ")"
	link := rootUrl + "desktop#/invoice/list?invoiceId=" + utils.Int64ToString(estimate.InvoiceId)

	var body strings.Builder
	body.WriteString(`<p>` + html.EscapeString(subject) + `</p>`)

	if estimate.ResponseNote != "" {
		body.WriteString(`<p>Note from the client:<br/>` + strings.ReplaceAll(html.EscapeString(estimate.ResponseNote), "\n", "<br/>") + `</p>`)
	}

	body.WriteString(`<p><a href="` + html.EscapeString(link) + `">Open the estimate</a></p>`)

	return subject, body.String()
}

// notifyEstimateResponse emails the owner's account address about a client's answer; SMTP off or a failed send is only logged
func (s *InvoiceService) notifyEstimateResponse(c core.Context, estimate *models.Invoice) {
	if !s.CurrentConfig().EnableSMTP {
		log.Infof(c, "[estimate_public.notifyEstimateResponse] SMTP is off, the answer to estimate \"id:%d\" of user \"uid:%d\" was not emailed", estimate.InvoiceId, estimate.Uid)
		return
	}

	user, err := Users.GetUserById(c, estimate.Uid)

	if err != nil || user.Email == "" {
		log.Warnf(c, "[estimate_public.notifyEstimateResponse] no account email for user \"uid:%d\", the answer to estimate \"id:%d\" was not emailed", estimate.Uid, estimate.InvoiceId)
		return
	}

	subject, body := estimateResponseEmail(estimate, s.CurrentConfig().RootUrl)

	if err := s.SendMail(&mail.MailMessage{To: user.Email, Subject: subject, Body: body}); err != nil {
		log.Warnf(c, "[estimate_public.notifyEstimateResponse] failed to email the answer to estimate \"id:%d\" of user \"uid:%d\", because %s", estimate.InvoiceId, estimate.Uid, err.Error())
	}
}
