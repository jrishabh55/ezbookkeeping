package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// estimateHasInvoices reports whether a non-deleted invoice was made from the estimate
func estimateHasInvoices(sess *xorm.Session, uid int64, estimateId int64) (bool, error) {
	return sess.Where("uid=? AND deleted=? AND kind=? AND estimate_id=?", uid, false, models.INVOICE_KIND_INVOICE, estimateId).Exist(&models.Invoice{})
}

// lockInvoiceRow writes the (non-deleted) invoice or estimate row as the first statement of a transaction, so conversions, edits and
// deletes of one estimate wait for each other and every read after it sees what the one before committed; the checks that follow
// ("nothing invoiced yet", "milestone still open") are then not racy. It touches only updated_unix_time.
func lockInvoiceRow(sess *xorm.Session, uid int64, invoiceId int64, now int64) error {
	_, err := sess.Cols("updated_unix_time").Where("uid=? AND deleted=? AND invoice_id=?", uid, false, invoiceId).Update(&models.Invoice{UpdatedUnixTime: now})
	return err
}

// getEstimateInSession reads an owned, non-deleted estimate with the caller's session; an invoice id is not found
func getEstimateInSession(sess *xorm.Session, uid int64, estimateId int64) (*models.Invoice, error) {
	estimate := &models.Invoice{}
	has, err := sess.Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", uid, false, models.INVOICE_KIND_ESTIMATE, estimateId).Get(estimate)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrInvoiceNotFound
	}

	return estimate, nil
}

// checkEstimateEditable returns ErrEstimateLocked once the estimate is accepted or declined or anything was invoiced from it
func checkEstimateEditable(sess *xorm.Session, estimate *models.Invoice) error {
	if estimate.EstimateStatus != models.ESTIMATE_STATUS_DRAFT && estimate.EstimateStatus != models.ESTIMATE_STATUS_SENT {
		return errs.ErrEstimateLocked
	}

	invoiced, err := estimateHasInvoices(sess, estimate.Uid, estimate.InvoiceId)

	if err != nil {
		return err
	} else if invoiced {
		return errs.ErrEstimateLocked
	}

	return nil
}

// ModifyEstimate saves an editable estimate and replaces its lines and milestones, like ModifyInvoice does for lines.
// The update is conditional on the status still being draft or sent, so an answer that lands meanwhile wins.
func (s *InvoiceService) ModifyEstimate(c core.Context, estimate *models.Invoice, lines []*models.InvoiceLine, milestones []*models.EstimateMilestone) error {
	if estimate.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if estimate.InvoiceId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	if err := validateEstimateMilestones(milestones); err != nil {
		return err
	}

	if err := s.prepareInvoice(c, estimate, lines); err != nil {
		return err
	}

	if err := s.prepareEstimateMilestones(estimate, milestones); err != nil {
		return err
	}

	estimate.Kind = models.INVOICE_KIND_ESTIMATE
	estimate.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(estimate.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := lockInvoiceRow(sess, estimate.Uid, estimate.InvoiceId, estimate.UpdatedUnixTime); err != nil {
			return err
		}

		stored, err := getEstimateInSession(sess, estimate.Uid, estimate.InvoiceId)

		if err != nil {
			return err
		}

		if err := checkEstimateEditable(sess, stored); err != nil {
			return err
		}

		if err := checkInvoiceNumberUnique(sess, estimate); err != nil {
			return err
		}

		updatedRows, err := sess.Cols(invoiceModifyColumns...).
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", estimate.Uid, false, models.INVOICE_KIND_ESTIMATE, estimate.InvoiceId).
			In("estimate_status", models.ESTIMATE_STATUS_DRAFT, models.ESTIMATE_STATUS_SENT).
			Update(estimate)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrEstimateLocked
		}

		if _, err := sess.Where("uid=? AND invoice_id=?", estimate.Uid, estimate.InvoiceId).Delete(&models.InvoiceLine{}); err != nil {
			return err
		}

		if _, err := sess.Insert(lines); err != nil {
			return err
		}

		if _, err := sess.Where("uid=? AND estimate_id=?", estimate.Uid, estimate.InvoiceId).Delete(&models.EstimateMilestone{}); err != nil {
			return err
		}

		if len(milestones) == 0 {
			return nil
		}

		_, err = sess.Insert(milestones)
		return err
	})
}

// SetEstimateResponse records your own answer (accepted or declined) for a draft or sent estimate, expired ones included.
// Nothing is emailed; RespondedByClient stays false.
func (s *InvoiceService) SetEstimateResponse(c core.Context, uid int64, estimateId int64, status string) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if estimateId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	if status != models.ESTIMATE_STATUS_ACCEPTED && status != models.ESTIMATE_STATUS_DECLINED {
		return errs.ErrEstimateStatusInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := getEstimateInSession(sess, uid, estimateId); err != nil {
			return err
		}

		updatedRows, err := sess.Cols("estimate_status", "responded_unix_time", "responded_by_client", "response_note", "updated_unix_time").
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", uid, false, models.INVOICE_KIND_ESTIMATE, estimateId).
			In("estimate_status", models.ESTIMATE_STATUS_DRAFT, models.ESTIMATE_STATUS_SENT).
			Update(&models.Invoice{EstimateStatus: status, RespondedUnixTime: now, RespondedByClient: false, ResponseNote: "", UpdatedUnixTime: now})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrEstimateStatusInvalid
		}

		return nil
	})
}

// ReopenEstimate returns an accepted or declined estimate to sent and clears the answer, while nothing is invoiced from it
func (s *InvoiceService) ReopenEstimate(c core.Context, uid int64, estimateId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if estimateId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := getEstimateInSession(sess, uid, estimateId); err != nil {
			return err
		}

		invoiced, err := estimateHasInvoices(sess, uid, estimateId)

		if err != nil {
			return err
		} else if invoiced {
			return errs.ErrEstimateHasInvoices
		}

		updatedRows, err := sess.Cols("estimate_status", "responded_unix_time", "responded_by_client", "response_note", "updated_unix_time").
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=?", uid, false, models.INVOICE_KIND_ESTIMATE, estimateId).
			In("estimate_status", models.ESTIMATE_STATUS_ACCEPTED, models.ESTIMATE_STATUS_DECLINED).
			Update(&models.Invoice{EstimateStatus: models.ESTIMATE_STATUS_SENT, RespondedUnixTime: 0, RespondedByClient: false, ResponseNote: "", UpdatedUnixTime: now})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrEstimateStatusInvalid
		}

		return nil
	})
}

// GetEstimateProgress returns, for each estimate id, its milestones and the non-deleted invoices made from it
func (s *InvoiceService) GetEstimateProgress(c core.Context, uid int64, estimateIds []int64) (map[int64]*models.EstimateProgress, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	result := make(map[int64]*models.EstimateProgress, len(estimateIds))

	for _, estimateId := range estimateIds {
		result[estimateId] = &models.EstimateProgress{Milestones: []*models.EstimateMilestone{}, Invoices: []*models.Invoice{}}
	}

	if len(estimateIds) < 1 {
		return result, nil
	}

	var milestones []*models.EstimateMilestone
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("estimate_id", estimateIds).OrderBy("display_order asc").Find(&milestones)

	if err != nil {
		return nil, err
	}

	for _, milestone := range milestones {
		result[milestone.EstimateId].Milestones = append(result[milestone.EstimateId].Milestones, milestone)
	}

	var invoices []*models.Invoice
	err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND kind=?", uid, false, models.INVOICE_KIND_INVOICE).
		In("estimate_id", estimateIds).Cols("invoice_id", "estimate_id", "number").OrderBy("number asc").Find(&invoices)

	if err != nil {
		return nil, err
	}

	for _, invoice := range invoices {
		result[invoice.EstimateId].Invoices = append(result[invoice.EstimateId].Invoices, invoice)
	}

	for _, progress := range result {
		if len(progress.Milestones) == 0 {
			progress.FullyInvoiced = len(progress.Invoices) > 0
			continue
		}

		progress.FullyInvoiced = true

		for _, milestone := range progress.Milestones {
			if milestone.InvoiceId == 0 {
				progress.FullyInvoiced = false
			}
		}
	}

	return result, nil
}
