package services

import (
	"strconv"
	"time"
	"unicode/utf8"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// estimateMilestoneQuantity is one unit (quantities are hundredths): a milestone invoice bills its share once
const estimateMilestoneQuantity = 100

// estimateShare is amount × basisPoints / 10000 rounded half up, split like the tax so it cannot overflow int64;
// src/lib/estimate.ts mirrors it for the Payment Terms table
func estimateShare(amount int64, basisPoints int32) int64 {
	whole, rest := amount/models.EstimateMilestonePercentTotal, amount%models.EstimateMilestonePercentTotal
	return whole*int64(basisPoints) + (rest*int64(basisPoints)+models.EstimateMilestonePercentTotal/2)/models.EstimateMilestonePercentTotal
}

// estimateMilestoneRate is the rate of the one line a milestone invoice gets: the milestone's share of the estimate subtotal, except
// that the last open milestone takes subtotal minus what the other milestones were really invoiced for (invoicedSubtotals, by
// milestone id), so the invoices always add up to the estimate exactly, in any order and after edits
func estimateMilestoneRate(subtotal int64, milestones []*models.EstimateMilestone, milestoneId int64, invoicedSubtotals map[int64]int64) (int64, *models.EstimateMilestone, error) {
	var target *models.EstimateMilestone
	open := 0

	for _, milestone := range milestones {
		if milestone.MilestoneId == milestoneId {
			target = milestone
		}

		if milestone.InvoiceId == 0 {
			open++
		}
	}

	if target == nil {
		return 0, nil, errs.ErrEstimateMilestoneNotFound
	}

	if target.InvoiceId > 0 {
		return 0, nil, errs.ErrEstimateAlreadyConverted
	}

	if open > 1 {
		return estimateShare(subtotal, target.Percent), target, nil
	}

	remainder := subtotal

	for _, milestone := range milestones {
		if milestone.MilestoneId != milestoneId {
			remainder -= invoicedSubtotals[milestone.MilestoneId]
		}
	}

	return remainder, target, nil
}

// estimateMilestoneLineText is the milestone line "<label>: <pct>% of Estimate <number>"; when that would not fit the
// 128-character item name, the label stays the name and the share goes to the description
func estimateMilestoneLineText(label string, percent int32, estimateNumber string) (string, string) {
	share := strconv.FormatFloat(float64(percent)/100, 'f', -1, 64) + "% of Estimate " + estimateNumber
	name := label + ": " + share

	if utf8.RuneCountInString(name) <= models.EstimateMilestoneLabelMaxLength {
		return name, ""
	}

	return label, share
}

// estimateInvoiceLines returns the lines an invoice made from the estimate gets, checking it can still be made: every line
// copied (milestoneId 0; only for an estimate without milestones and not invoiced yet) or the one line of an open milestone
func estimateInvoiceLines(sess *xorm.Session, estimate *models.Invoice, estimateLines []*models.InvoiceLine, milestoneId int64) ([]*models.InvoiceLine, error) {
	// a declined estimate is not invoiced, and only the latest version of a revised estimate can be
	if estimate.EstimateStatus == models.ESTIMATE_STATUS_DECLINED || estimate.EstimateStatus == models.ESTIMATE_STATUS_SUPERSEDED {
		return nil, errs.ErrEstimateStatusInvalid
	}

	var milestones []*models.EstimateMilestone
	if err := sess.Where("uid=? AND estimate_id=?", estimate.Uid, estimate.InvoiceId).OrderBy("display_order asc").Find(&milestones); err != nil {
		return nil, err
	}

	if milestoneId == 0 {
		if len(milestones) > 0 {
			return nil, errs.ErrEstimateMilestoneNotFound
		}

		invoiced, err := estimateHasInvoices(sess, estimate.Uid, estimate.InvoiceId)

		if err != nil {
			return nil, err
		} else if invoiced {
			return nil, errs.ErrEstimateAlreadyConverted
		}

		lines := make([]*models.InvoiceLine, len(estimateLines))

		for idx, estimateLine := range estimateLines {
			line := *estimateLine
			line.LineId = 0
			line.InvoiceId = 0
			lines[idx] = &line
		}

		return lines, nil
	}

	invoicedSubtotals := make(map[int64]int64)
	invoiceIds := make([]int64, 0, len(milestones))

	for _, milestone := range milestones {
		if milestone.InvoiceId > 0 {
			invoiceIds = append(invoiceIds, milestone.InvoiceId)
		}
	}

	if len(invoiceIds) > 0 {
		var invoices []*models.Invoice
		if err := sess.Where("uid=? AND deleted=?", estimate.Uid, false).In("invoice_id", invoiceIds).Cols("invoice_id", "subtotal").Find(&invoices); err != nil {
			return nil, err
		}

		subtotals := make(map[int64]int64, len(invoices))

		for _, invoice := range invoices {
			subtotals[invoice.InvoiceId] = invoice.Subtotal
		}

		for _, milestone := range milestones {
			if milestone.InvoiceId > 0 {
				invoicedSubtotals[milestone.MilestoneId] = subtotals[milestone.InvoiceId]
			}
		}
	}

	rate, milestone, err := estimateMilestoneRate(estimate.Subtotal, milestones, milestoneId, invoicedSubtotals)

	if err != nil {
		return nil, err
	} else if rate <= 0 {
		return nil, errs.ErrEstimateAlreadyConverted // the other invoices already add up to the whole estimate
	}

	sac := ""

	if len(estimateLines) > 0 {
		sac = estimateLines[0].Sac
	}

	itemName, description := estimateMilestoneLineText(milestone.Label, milestone.Percent, estimate.Number)

	return []*models.InvoiceLine{{ItemName: itemName, Description: description, Quantity: estimateMilestoneQuantity, Rate: rate, Sac: sac}}, nil
}

// GetEstimateInvoiceDraft builds, without saving, the invoice an estimate (milestoneId 0) or one of its milestones turns into, for the
// editor: dated today, due by the customer's or the profile's terms, numbered from the invoice series, in the estimate's currency with
// its tax settings, and the profile's default notes
func (s *InvoiceService) GetEstimateInvoiceDraft(c core.Context, uid int64, estimateId int64, milestoneId int64, today string) (*models.Invoice, []*models.InvoiceLine, error) {
	estimate, estimateLines, err := s.GetInvoice(c, uid, estimateId)

	if err != nil {
		return nil, nil, err
	} else if estimate.Kind != models.INVOICE_KIND_ESTIMATE {
		return nil, nil, errs.ErrInvoiceNotFound
	}

	lines, err := estimateInvoiceLines(s.UserDataDB(uid).NewSession(c), estimate, estimateLines, milestoneId)

	if err != nil {
		return nil, nil, err
	}

	todayTime, err := time.Parse(invoiceDateLayout, today)

	if err != nil {
		return nil, nil, errs.ErrInvoiceDateInvalid
	}

	customer, err := s.GetCustomer(c, uid, estimate.CustomerId)

	if err != nil {
		return nil, nil, err
	}

	profile, err := s.GetProfile(c, uid)

	if err != nil {
		return nil, nil, err
	}

	termsDays := profile.DefaultTermsDays

	if customer.TermsDays > 0 {
		termsDays = customer.TermsDays
	}

	number, err := s.NextInvoiceNumber(c, uid, today)

	if err != nil {
		return nil, nil, err
	}

	draft := &models.Invoice{Uid: uid, Kind: models.INVOICE_KIND_INVOICE, Number: number, CustomerId: estimate.CustomerId, Currency: estimate.Currency,
		BillToName: estimate.BillToName, InvoiceDate: today, DueDate: todayTime.AddDate(0, 0, int(termsDays)).Format(invoiceDateLayout),
		Notes: profile.DefaultNotes, TaxType: estimate.TaxType, TaxRate: estimate.TaxRate, EstimateId: estimateId}

	totals := computeInvoiceTotals(lines, draft.TaxType, draft.TaxRate)
	draft.Subtotal, draft.TaxAmount, draft.Total = totals.Subtotal, totals.TaxAmount, totals.Total

	return draft, lines, nil
}

// CreateInvoiceFromEstimate saves an invoice made from an estimate (milestoneId 0 = the whole estimate). It bills the estimate's customer
// in the estimate's currency. For a milestone the one line (text, SAC, quantity and rate) and the tax settings are rebuilt from the
// estimate inside the transaction, so a client cannot change the amount and the remainder rule sees every other conversion; the milestone is marked invoiced in the same
// transaction, conditional on still being open, so a second click cannot invoice it twice. A whole-estimate conversion keeps the lines as
// edited and is refused while another invoice made from the estimate exists.
func (s *InvoiceService) CreateInvoiceFromEstimate(c core.Context, invoice *models.Invoice, lines []*models.InvoiceLine, estimateId int64, milestoneId int64) error {
	if invoice.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if estimateId <= 0 {
		return errs.ErrInvoiceIdInvalid
	}

	estimate, err := s.getInvoiceHeader(c, invoice.Uid, estimateId)

	if err != nil {
		return err
	} else if estimate.Kind != models.INVOICE_KIND_ESTIMATE {
		return errs.ErrInvoiceNotFound
	}

	if milestoneId > 0 && len(lines) != 1 {
		return errs.ErrEstimateMilestoneInvoiceLineCountInvalid
	}

	invoice.CustomerId = estimate.CustomerId
	invoice.Currency = estimate.Currency

	if err := s.prepareNewInvoice(c, invoice, lines); err != nil {
		return err
	}

	invoice.EstimateId = estimateId

	return s.UserDataDB(invoice.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		// writing the estimate row first makes two conversions of the same estimate (or a conversion and an edit or delete of it)
		// wait for each other; everything below then re-reads the estimate and its milestones as the previous one left them.
		// Conversion locks only the estimate row: it reads existing invoices with plain SELECTs, writes milestone rows and inserts a
		// new invoice, but never updates an existing invoice row, so it cannot invert the invoice → estimate lock order
		if err := lockInvoiceRow(sess, invoice.Uid, estimateId, invoice.CreatedUnixTime); err != nil {
			return err
		}

		stored, err := getEstimateInSession(sess, invoice.Uid, estimateId)

		if err != nil {
			return err
		}

		var estimateLines []*models.InvoiceLine
		if err := sess.Where("uid=? AND invoice_id=?", invoice.Uid, estimateId).OrderBy("display_order asc").Find(&estimateLines); err != nil {
			return err
		}

		expected, err := estimateInvoiceLines(sess, stored, estimateLines, milestoneId)

		if err != nil {
			return err
		}

		if milestoneId > 0 {
			// the server owns the whole milestone line and bills it with the estimate's tax
			line := lines[0]
			line.ItemName, line.Description, line.Sac, line.ProductId = expected[0].ItemName, expected[0].Description, expected[0].Sac, expected[0].ProductId
			line.Quantity, line.Rate = expected[0].Quantity, expected[0].Rate
			invoice.TaxType, invoice.TaxRate = stored.TaxType, stored.TaxRate
			totals := computeInvoiceTotals(lines, invoice.TaxType, invoice.TaxRate)
			invoice.Subtotal, invoice.TaxAmount, invoice.Total = totals.Subtotal, totals.TaxAmount, totals.Total

			updatedRows, err := sess.Cols("invoice_id").Where("uid=? AND estimate_id=? AND milestone_id=? AND invoice_id=?", invoice.Uid, estimateId, milestoneId, 0).
				Update(&models.EstimateMilestone{InvoiceId: invoice.InvoiceId})

			if err != nil {
				return err
			} else if updatedRows < 1 {
				return errs.ErrEstimateAlreadyConverted
			}
		}

		return insertInvoice(sess, invoice, lines)
	})
}
