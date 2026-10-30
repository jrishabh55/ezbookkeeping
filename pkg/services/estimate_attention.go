package services

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// GetEstimatesToInvoice returns the accepted estimates of user that are not fully invoiced yet, oldest answer first, with their
// progress: the home card's "Estimate accepted by <customer>: Convert" rows
func (s *InvoiceService) GetEstimatesToInvoice(c core.Context, uid int64) ([]*models.Invoice, map[int64]*models.EstimateProgress, error) {
	if uid <= 0 {
		return nil, nil, errs.ErrUserIdInvalid
	}

	var accepted []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND kind=? AND estimate_status=?", uid, false, models.INVOICE_KIND_ESTIMATE, models.ESTIMATE_STATUS_ACCEPTED).
		OrderBy("responded_unix_time asc, number asc").Find(&accepted)

	if err != nil {
		return nil, nil, err
	}

	estimateIds := make([]int64, len(accepted))

	for idx, estimate := range accepted {
		estimateIds[idx] = estimate.InvoiceId
	}

	progress, err := s.GetEstimateProgress(c, uid, estimateIds)

	if err != nil {
		return nil, nil, err
	}

	open := make([]*models.Invoice, 0, len(accepted))

	for _, estimate := range accepted {
		if !progress[estimate.InvoiceId].FullyInvoiced {
			open = append(open, estimate)
		}
	}

	return open, progress, nil
}
