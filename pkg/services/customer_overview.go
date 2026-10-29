package services

import (
	"sort"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// GetCustomerOverview returns the products a customer bought and their timeline (invoices, emails, payments), newest first
func (s *InvoiceService) GetCustomerOverview(c core.Context, uid int64, customerId int64) (*models.CustomerOverviewResponse, error) {
	if _, err := s.GetCustomer(c, uid, customerId); err != nil {
		return nil, err
	}

	var invoices []*models.Invoice
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND customer_id=?", uid, false, customerId).Find(&invoices)

	if err != nil {
		return nil, err
	}

	overview := &models.CustomerOverviewResponse{Products: []*models.CustomerProductResponse{}, Timeline: []*models.CustomerTimelineItemResponse{}}

	if len(invoices) < 1 {
		return overview, nil
	}

	invoiceIds := make([]int64, len(invoices))
	invoiceMap := make(map[int64]*models.Invoice, len(invoices))

	for idx, invoice := range invoices {
		invoiceIds[idx] = invoice.InvoiceId
		invoiceMap[invoice.InvoiceId] = invoice
		overview.Timeline = append(overview.Timeline, &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_INVOICE, UnixTime: invoice.CreatedUnixTime,
			InvoiceId: utils.Int64ToString(invoice.InvoiceId), InvoiceNumber: invoice.Number, Amount: invoice.Total, Currency: invoice.Currency, Renewal: invoice.RecurringId > 0})
	}

	if err := s.addCustomerProducts(c, uid, invoiceIds, invoiceMap, overview); err != nil {
		return nil, err
	}

	var emails []*models.InvoiceEmail
	err = s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("invoice_id", invoiceIds).Omit("email_html").Find(&emails)

	if err != nil {
		return nil, err
	}

	for _, email := range emails {
		invoice := invoiceMap[email.InvoiceId]
		overview.Timeline = append(overview.Timeline, &models.CustomerTimelineItemResponse{Kind: models.CUSTOMER_TIMELINE_KIND_EMAIL, UnixTime: email.SentUnixTime,
			InvoiceId: utils.Int64ToString(invoice.InvoiceId), InvoiceNumber: invoice.Number, Text: email.Subject})
	}

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

	sort.SliceStable(overview.Timeline, func(i, j int) bool { return overview.Timeline[i].UnixTime > overview.Timeline[j].UnixTime })

	if len(overview.Timeline) > models.CustomerTimelineMaxItems {
		overview.Timeline = overview.Timeline[:models.CustomerTimelineMaxItems]
	}

	return overview, nil
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
