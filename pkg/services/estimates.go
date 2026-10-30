package services

import (
	"strings"
	"unicode/utf8"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// validateEstimateMilestones checks an estimate's payment terms: none, or 1 to 10, each with a label (trimmed here) and a
// positive percent, the percents (basis points) adding up to exactly 100%
func validateEstimateMilestones(milestones []*models.EstimateMilestone) error {
	if len(milestones) == 0 {
		return nil
	}

	if len(milestones) > models.EstimateMaxMilestones {
		return errs.ErrEstimateMilestonesInvalid
	}

	total := int32(0)

	for _, milestone := range milestones {
		milestone.Label = strings.TrimSpace(milestone.Label)

		if milestone.Label == "" || utf8.RuneCountInString(milestone.Label) > models.EstimateMilestoneLabelMaxLength ||
			milestone.Percent <= 0 || milestone.Percent > models.EstimateMilestonePercentTotal {
			return errs.ErrEstimateMilestonesInvalid
		}

		total += milestone.Percent
	}

	if total != models.EstimateMilestonePercentTotal {
		return errs.ErrEstimateMilestonesInvalid
	}

	return nil
}

// prepareEstimateMilestones gives the milestones of a saved estimate their ids, owner and order; nothing is invoiced yet
func (s *InvoiceService) prepareEstimateMilestones(estimate *models.Invoice, milestones []*models.EstimateMilestone) error {
	for idx, milestone := range milestones {
		milestone.MilestoneId = s.GenerateUuid(uuid.UUID_TYPE_INVOICE)

		if milestone.MilestoneId < 1 {
			return errs.ErrSystemIsBusy
		}

		milestone.Uid = estimate.Uid
		milestone.EstimateId = estimate.InvoiceId
		milestone.DisplayOrder = int32(idx + 1)
		milestone.InvoiceId = 0
	}

	return nil
}

// CreateEstimate saves a new draft estimate with its lines and milestones. It is numbered, totalled and snapshotted
// exactly like an invoice (prepareNewInvoice), then marked as an estimate.
func (s *InvoiceService) CreateEstimate(c core.Context, estimate *models.Invoice, lines []*models.InvoiceLine, milestones []*models.EstimateMilestone) error {
	if err := validateEstimateMilestones(milestones); err != nil {
		return err
	}

	if err := s.prepareNewInvoice(c, estimate, lines); err != nil {
		return err
	}

	estimate.Kind = models.INVOICE_KIND_ESTIMATE
	estimate.EstimateStatus = models.ESTIMATE_STATUS_DRAFT

	if err := s.prepareEstimateMilestones(estimate, milestones); err != nil {
		return err
	}

	return s.UserDataDB(estimate.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := insertInvoice(sess, estimate, lines); err != nil {
			return err
		}

		if len(milestones) == 0 {
			return nil
		}

		_, err := sess.Insert(milestones)
		return err
	})
}

// GetEstimateMilestones returns the milestones of an estimate of user in display order; an invoice has none
func (s *InvoiceService) GetEstimateMilestones(c core.Context, uid int64, estimateId int64) ([]*models.EstimateMilestone, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var milestones []*models.EstimateMilestone
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND estimate_id=?", uid, estimateId).OrderBy("display_order asc").Find(&milestones)

	return milestones, err
}
