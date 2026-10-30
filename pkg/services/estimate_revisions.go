package services

import (
	"fmt"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// estimateRevisionNumber is a revision's number: the first version's number plus "-R<n>", e.g. EST/2026-27/01-R2
func estimateRevisionNumber(rootNumber string, revisionNumber int32) string {
	return fmt.Sprintf("%s-R%d", rootNumber, revisionNumber)
}

// nextEstimateRevisionNumber is one more than the highest revision number of a chain; the caller passes every row of the
// chain, deleted ones included, so a deleted draft revision's number is never handed out again
func nextEstimateRevisionNumber(existing []int32) int32 {
	highest := int32(0)

	for _, revisionNumber := range existing {
		if revisionNumber > highest {
			highest = revisionNumber
		}
	}

	return highest + 1
}

// estimateChainRootId is the id of the first version of the estimate's revision chain
func estimateChainRootId(estimate *models.Invoice) int64 {
	if estimate.RevisionOf > 0 {
		return estimate.RevisionOf
	}

	return estimate.InvoiceId
}

// checkEstimateRevisable returns ErrEstimateCannotBeRevised unless the estimate is sent (expired included), declined or accepted,
// nothing was invoiced from it, and no non-deleted version of its chain has a higher revision number
func checkEstimateRevisable(sess *xorm.Session, estimate *models.Invoice) error {
	switch estimate.EstimateStatus {
	case models.ESTIMATE_STATUS_SENT, models.ESTIMATE_STATUS_DECLINED, models.ESTIMATE_STATUS_ACCEPTED:
	default:
		return errs.ErrEstimateCannotBeRevised
	}

	invoiced, err := estimateHasInvoices(sess, estimate.Uid, estimate.InvoiceId)

	if err != nil {
		return err
	} else if invoiced {
		return errs.ErrEstimateCannotBeRevised
	}

	rootId := estimateChainRootId(estimate)
	newer, err := sess.Where("uid=? AND deleted=? AND kind=? AND revision_number>? AND (invoice_id=? OR revision_of=?)",
		estimate.Uid, false, models.INVOICE_KIND_ESTIMATE, estimate.RevisionNumber, rootId, rootId).Exist(&models.Invoice{})

	if err != nil {
		return err
	} else if newer {
		return errs.ErrEstimateCannotBeRevised
	}

	return nil
}

// ReviseEstimate creates the next version of a sent or answered estimate as a new draft and supersedes the old version, which keeps
// everything it had (lines, milestones, PDF, emails, recorded answer). It returns the new version for the editor.
func (s *InvoiceService) ReviseEstimate(c core.Context, uid int64, estimateId int64, today string) (*models.Invoice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if estimateId <= 0 {
		return nil, errs.ErrInvoiceIdInvalid
	}

	source, lines, err := s.GetInvoice(c, uid, estimateId)

	if err != nil {
		return nil, err
	} else if source.Kind != models.INVOICE_KIND_ESTIMATE {
		return nil, errs.ErrInvoiceNotFound
	}

	milestones, err := s.GetEstimateMilestones(c, uid, estimateId)

	if err != nil {
		return nil, err
	}

	return s.reviseEstimate(c, source, lines, milestones, today)
}

// reviseEstimate copies source (as read before the transaction) into a new draft version. Inside one transaction it locks the source
// row (only estimate rows are locked, so the invoice → estimate lock order holds), re-reads and re-checks it, numbers and inserts the
// copy, and supersedes the source conditionally on the status it just read: of two concurrent Revise clicks exactly one goes through.
func (s *InvoiceService) reviseEstimate(c core.Context, source *models.Invoice, sourceLines []*models.InvoiceLine, sourceMilestones []*models.EstimateMilestone, today string) (*models.Invoice, error) {
	todayTime, err := time.Parse(invoiceDateLayout, today)

	if err != nil {
		return nil, errs.ErrInvoiceDateInvalid
	}

	profile, err := s.GetProfile(c, source.Uid)

	if err != nil {
		return nil, err
	}

	validDays := profile.EstimateValidDays

	if validDays <= 0 {
		validDays = models.EstimateValidDaysDefault
	}

	revision := &models.Invoice{Uid: source.Uid, Number: source.Number, CustomerId: source.CustomerId, Currency: source.Currency, InvoiceDate: today,
		DueDate: todayTime.AddDate(0, 0, int(validDays)).Format(invoiceDateLayout), Notes: source.Notes, TaxType: source.TaxType, TaxRate: source.TaxRate}

	lines := make([]*models.InvoiceLine, len(sourceLines))

	for idx, line := range sourceLines {
		lines[idx] = &models.InvoiceLine{ItemName: line.ItemName, Description: line.Description, Quantity: line.Quantity, Rate: line.Rate, Sac: line.Sac, ProductId: line.ProductId}
	}

	milestones := make([]*models.EstimateMilestone, len(sourceMilestones))

	for idx, milestone := range sourceMilestones {
		milestones[idx] = &models.EstimateMilestone{Label: milestone.Label, Percent: milestone.Percent}
	}

	if err := s.prepareNewInvoice(c, revision, lines); err != nil {
		return nil, err
	}

	revision.Kind = models.INVOICE_KIND_ESTIMATE
	revision.EstimateStatus = models.ESTIMATE_STATUS_DRAFT

	if err := s.prepareEstimateMilestones(revision, milestones); err != nil {
		return nil, err
	}

	err = s.UserDataDB(source.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := lockInvoiceRow(sess, source.Uid, source.InvoiceId, revision.CreatedUnixTime); err != nil {
			return err
		}

		stored, err := getEstimateInSession(sess, source.Uid, source.InvoiceId)

		if err != nil {
			return err
		}

		if err := checkEstimateRevisable(sess, stored); err != nil {
			return err
		}

		rootId := estimateChainRootId(stored)
		root := &models.Invoice{}
		has, err := sess.Where("uid=? AND invoice_id=?", stored.Uid, rootId).Get(root)

		if err != nil {
			return err
		} else if !has {
			return errs.ErrInvoiceNotFound
		}

		// deleted versions count too, so a deleted draft revision's number is never reused
		var revisionNumbers []int32
		err = sess.Table(&models.Invoice{}).Where("uid=? AND kind=? AND (invoice_id=? OR revision_of=?)", stored.Uid, models.INVOICE_KIND_ESTIMATE, rootId, rootId).
			Cols("revision_number").Find(&revisionNumbers)

		if err != nil {
			return err
		}

		revision.RevisionOf = rootId
		revision.RevisionNumber = nextEstimateRevisionNumber(revisionNumbers)
		revision.Number = estimateRevisionNumber(root.Number, revision.RevisionNumber)

		if err := insertInvoice(sess, revision, lines); err != nil {
			return err
		}

		if len(milestones) > 0 {
			if _, err := sess.Insert(milestones); err != nil {
				return err
			}
		}

		updatedRows, err := sess.Cols("estimate_status", "superseded_from_status", "updated_unix_time").
			Where("uid=? AND deleted=? AND kind=? AND invoice_id=? AND estimate_status=?", stored.Uid, false, models.INVOICE_KIND_ESTIMATE, stored.InvoiceId, stored.EstimateStatus).
			Update(&models.Invoice{EstimateStatus: models.ESTIMATE_STATUS_SUPERSEDED, SupersededFromStatus: stored.EstimateStatus, UpdatedUnixTime: revision.CreatedUnixTime})

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrEstimateCannotBeRevised
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return revision, nil
}
