package services

import (
	"sort"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// GetCustomerOverview returns the products a customer bought (from invoices only) and their timeline: invoices, estimates with their
// answers, emails and payments, newest first
func (s *InvoiceService) GetCustomerOverview(c core.Context, uid int64, customerId int64) (*models.CustomerOverviewResponse, error) {
	if _, err := s.GetCustomer(c, uid, customerId); err != nil {
		return nil, err
	}

	var rows []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND customer_id=?", uid, false, customerId).Find(&rows)

	if err != nil {
		return nil, err
	}

	overview := &models.CustomerOverviewResponse{Products: []*models.CustomerProductResponse{}, Timeline: []*models.CustomerTimelineItemResponse{}}

	if len(rows) < 1 {
		return overview, nil
	}

	rowIds := make([]int64, len(rows))
	invoiceIds := make([]int64, 0, len(rows)) // invoices only: an estimate is never bought or paid
	invoiceMap := make(map[int64]*models.Invoice, len(rows))

	for idx, row := range rows {
		rowIds[idx] = row.InvoiceId
		invoiceMap[row.InvoiceId] = row
	}

	for _, row := range rows {
		if row.Kind == models.INVOICE_KIND_ESTIMATE {
			overview.Timeline = append(overview.Timeline, estimateTimelineItems(row)...)
			continue
		}

		invoiceIds = append(invoiceIds, row.InvoiceId)
		item := &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_INVOICE, UnixTime: row.CreatedUnixTime,
			InvoiceId: utils.Int64ToString(row.InvoiceId), InvoiceNumber: row.Number, Amount: row.Total, Currency: row.Currency, Renewal: row.RecurringId > 0}

		if estimate := invoiceMap[row.EstimateId]; row.EstimateId > 0 && estimate != nil {
			item.EstimateNumber = estimate.Number
		}

		overview.Timeline = append(overview.Timeline, item)
	}

	if len(invoiceIds) > 0 {
		if err := s.addCustomerProducts(c, uid, invoiceIds, invoiceMap, overview); err != nil {
			return nil, err
		}
	}

	var emails []*models.InvoiceEmail
	err = s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("invoice_id", rowIds).Omit("email_html").Find(&emails)

	if err != nil {
		return nil, err
	}

	for _, email := range emails {
		invoice := invoiceMap[email.InvoiceId]
		overview.Timeline = append(overview.Timeline, &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_EMAIL, UnixTime: email.SentUnixTime,
			InvoiceId: utils.Int64ToString(invoice.InvoiceId), InvoiceNumber: invoice.Number, Text: email.Subject})
	}

	if len(invoiceIds) > 0 {
		var links []*models.InvoicePayment
		err = s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("invoice_id", invoiceIds).Find(&links)

		if err != nil {
			return nil, err
		}

		payments, err := s.GetInvoicePayments(c, uid, invoiceIds)

		if err != nil {
			return nil, err
		}

		for _, link := range links {
			invoice := invoiceMap[link.InvoiceId]

			for _, payment := range payments[link.InvoiceId] {
				if payment.Transaction.TransactionId == link.TransactionId {
					overview.Timeline = append(overview.Timeline, &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_PAYMENT, UnixTime: link.LinkedUnixTime,
						InvoiceId: utils.Int64ToString(invoice.InvoiceId), InvoiceNumber: invoice.Number, Amount: payment.ReceivedAmount(), Currency: invoice.Currency})
				}
			}
		}
	}

	sort.SliceStable(overview.Timeline, func(i, j int) bool { return overview.Timeline[i].UnixTime > overview.Timeline[j].UnixTime })

	if len(overview.Timeline) > models.CustomerTimelineMaxItems {
		overview.Timeline = overview.Timeline[:models.CustomerTimelineMaxItems]
	}

	return overview, nil
}

// estimateTimelineItems are an estimate's events on the customer page: created, then accepted or declined (with the client's note)
func estimateTimelineItems(estimate *models.Invoice) []*models.CustomerTimelineItemResponse {
	id := utils.Int64ToString(estimate.InvoiceId)
	items := []*models.CustomerTimelineItemResponse{{Kind: models.CUSTOMER_TIMELINE_KIND_ESTIMATE, UnixTime: estimate.CreatedUnixTime,
		InvoiceId: id, InvoiceNumber: estimate.Number, Amount: estimate.Total, Currency: estimate.Currency}}

	kind := ""

	switch estimate.EstimateStatus {
	case models.ESTIMATE_STATUS_ACCEPTED:
		kind = models.CUSTOMER_TIMELINE_KIND_ESTIMATE_ACCEPTED
	case models.ESTIMATE_STATUS_DECLINED:
		kind = models.CUSTOMER_TIMELINE_KIND_ESTIMATE_DECLINED
	}

	if kind != "" && estimate.RespondedUnixTime > 0 {
		items = append(items, &models.CustomerTimelineItemResponse{Kind: kind, UnixTime: estimate.RespondedUnixTime, InvoiceId: id, InvoiceNumber: estimate.Number,
			Amount: estimate.Total, Currency: estimate.Currency, Text: estimate.ResponseNote, ByClient: estimate.RespondedByClient})
	}

	return items
}

// addCustomerProducts fills the products bought from the product-linked lines of the customer's invoices, most recently bought first
func (s *InvoiceService) addCustomerProducts(c core.Context, uid int64, invoiceIds []int64, invoiceMap map[int64]*models.Invoice, overview *models.CustomerOverviewResponse) error {
	var lines []*models.InvoiceLine
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND product_id>?", uid, 0).In("invoice_id", invoiceIds).Find(&lines)

	if err != nil {
		return err
	}

	invoicesByProduct := make(map[int64]map[int64]bool)
	productIds := make([]int64, 0)

	for _, line := range lines {
		if invoicesByProduct[line.ProductId] == nil {
			invoicesByProduct[line.ProductId] = make(map[int64]bool)
			productIds = append(productIds, line.ProductId)
		}

		invoicesByProduct[line.ProductId][line.InvoiceId] = true
	}

	products, err := s.GetProductsByIds(c, uid, productIds)

	if err != nil {
		return err
	}

	for _, productId := range productIds {
		product := products[productId]

		if product == nil {
			continue
		}

		item := &models.CustomerProductResponse{ProductId: utils.Int64ToString(productId), Name: product.Name, Archived: product.Archived}

		for invoiceId := range invoicesByProduct[productId] {
			item.TimesBought++

			if date := invoiceMap[invoiceId].InvoiceDate; date > item.LastBoughtDate {
				item.LastBoughtDate = date
			}
		}

		overview.Products = append(overview.Products, item)
	}

	sort.SliceStable(overview.Products, func(i, j int) bool { return overview.Products[i].LastBoughtDate > overview.Products[j].LastBoughtDate })

	return nil
}
