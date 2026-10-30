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

// checkEstimateDeletable refuses a superseded version (history the newer version depends on): only the latest version of a chain
// is deleted, a draft revision on its own (see restorePreviousEstimateVersion), anything else together with its whole chain
func checkEstimateDeletable(estimate *models.Invoice) error {
	if estimate.EstimateStatus == models.ESTIMATE_STATUS_SUPERSEDED {
		return errs.ErrEstimateStatusInvalid
	}

	return nil
}

// isDraftEstimateRevision reports whether deleting the estimate deletes just it and restores the version it superseded
func isDraftEstimateRevision(estimate *models.Invoice) bool {
	return estimate.RevisionNumber > 0 && estimate.EstimateStatus == models.ESTIMATE_STATUS_DRAFT
}

// getEstimateChainIdsToDelete returns the ids of every non-deleted version of the estimate's chain, ascending revision number, after
// locking each one (the caller has already locked the estimate itself; only estimate rows are locked, so the invoice → estimate lock
// order holds). It returns ErrEstimateHasInvoices when a non-deleted invoice was made from any of them.
func getEstimateChainIdsToDelete(sess *xorm.Session, estimate *models.Invoice, now int64) ([]int64, error) {
	rootId := estimateChainRootId(estimate)

	var versions []*models.Invoice
	err := sess.Cols("invoice_id").Where("uid=? AND deleted=? AND kind=? AND (invoice_id=? OR revision_of=?)", estimate.Uid, false, models.INVOICE_KIND_ESTIMATE, rootId, rootId).
		OrderBy("revision_number asc").Find(&versions)

	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0, len(versions))

	for _, version := range versions {
		if version.InvoiceId != estimate.InvoiceId {
			if err := lockInvoiceRow(sess, estimate.Uid, version.InvoiceId, now); err != nil {
				return nil, err
			}
		}

		invoiced, err := estimateHasInvoices(sess, estimate.Uid, version.InvoiceId)

		if err != nil {
			return nil, err
		} else if invoiced {
			return nil, errs.ErrEstimateHasInvoices
		}

		ids = append(ids, version.InvoiceId)
	}

	return ids, nil
}

// restorePreviousEstimateVersion returns the version a deleted draft revision superseded (the chain's highest lower revision number
// still there) to the status it had before Revise, inside the caller's transaction. The update is conditional on it still being
// superseded; a version stored without SupersededFromStatus goes back to sent.
func restorePreviousEstimateVersion(sess *xorm.Session, revision *models.Invoice, now int64) error {
	previous := &models.Invoice{}
	has, err := sess.Where("uid=? AND deleted=? AND kind=? AND estimate_status=? AND revision_number<? AND (invoice_id=? OR revision_of=?)",
		revision.Uid, false, models.INVOICE_KIND_ESTIMATE, models.ESTIMATE_STATUS_SUPERSEDED, revision.RevisionNumber, revision.RevisionOf, revision.RevisionOf).
		OrderBy("revision_number desc").Get(previous)

	if err != nil {
		return err
	} else if !has {
		return nil
	}

	status := previous.SupersededFromStatus

	if status == "" {
		status = models.ESTIMATE_STATUS_SENT
	}

	_, err = sess.Cols("estimate_status", "superseded_from_status", "updated_unix_time").
		Where("uid=? AND deleted=? AND invoice_id=? AND estimate_status=?", revision.Uid, false, previous.InvoiceId, models.ESTIMATE_STATUS_SUPERSEDED).
		Update(&models.Invoice{EstimateStatus: status, SupersededFromStatus: "", UpdatedUnixTime: now})

	return err
}

// GetEstimateRevisions returns every non-deleted version of the estimate's revision chain, newest (highest revision number) first;
// a first version that was never revised returns just itself
func (s *InvoiceService) GetEstimateRevisions(c core.Context, uid int64, estimate *models.Invoice) ([]*models.Invoice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	rootId := estimateChainRootId(estimate)

	var revisions []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND kind=? AND (invoice_id=? OR revision_of=?)", uid, false, models.INVOICE_KIND_ESTIMATE, rootId, rootId).
		OrderBy("revision_number desc").Find(&revisions)

	return revisions, err
}

// GetLatestEstimates returns the latest version of every estimate of user (never a superseded one), newest estimate date first
func (s *InvoiceService) GetLatestEstimates(c core.Context, uid int64) ([]*models.Invoice, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var estimates []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND kind=? AND estimate_status<>?", uid, false, models.INVOICE_KIND_ESTIMATE, models.ESTIMATE_STATUS_SUPERSEDED).
		OrderBy("invoice_date desc, number desc").Find(&estimates)

	return estimates, err
}

// GetEstimateSupersededBy returns, for a superseded version, the public token of the latest version of its chain when that version was
// sent (sent, accepted or declined) and its link is enabled; otherwise empty. The public page links to the token only, never to an id,
// and never to a draft the client has not been sent.
func (s *InvoiceService) GetEstimateSupersededBy(c core.Context, estimate *models.Invoice) (string, error) {
	if estimate.Kind != models.INVOICE_KIND_ESTIMATE || estimate.EstimateStatus != models.ESTIMATE_STATUS_SUPERSEDED {
		return "", nil
	}

	rootId := estimateChainRootId(estimate)
	latest := &models.Invoice{}
	has, err := s.UserDataDB(estimate.Uid).NewSession(c).
		Where("uid=? AND deleted=? AND kind=? AND estimate_status<>? AND (invoice_id=? OR revision_of=?)",
			estimate.Uid, false, models.INVOICE_KIND_ESTIMATE, models.ESTIMATE_STATUS_SUPERSEDED, rootId, rootId).
		OrderBy("revision_number desc").Get(latest)

	if err != nil {
		return "", err
	} else if !has || latest.EstimateStatus == models.ESTIMATE_STATUS_DRAFT {
		return "", nil
	} else if latest.CustomerId != estimate.CustomerId {
		// a version made out to someone else is never advertised to this version's client
		return "", nil
	}

	return latest.PublicToken, nil
}
