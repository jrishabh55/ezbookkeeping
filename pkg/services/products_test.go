package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func createTestProduct(t *testing.T, uid int64, name string, renewalMonths int32, mode string) *models.Product {
	t.Helper()
	product := &models.Product{Uid: uid, Name: name, DefaultRate: 150000, RenewalMonths: renewalMonths, DefaultRenewalMode: mode}
	assert.Nil(t, Invoices.CreateProduct(core.NewNullContext(), product))
	return product
}

func createTestInvoiceWithLines(t *testing.T, uid int64, customerId int64, number string, lines []*models.InvoiceLine) *models.Invoice {
	t.Helper()
	invoice := &models.Invoice{Uid: uid, Number: number, CustomerId: customerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30"}
	assert.Nil(t, Invoices.CreateInvoice(core.NewNullContext(), invoice, lines))
	return invoice
}

func TestProductCrudIsScopedToUser(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)

	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	createTestProduct(t, uid, "Audit", 0, "")

	all, err := Invoices.GetAllProducts(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(all))
	assert.Equal(t, "Audit", all[0].Name) // ordered by name

	others, err := Invoices.GetAllProducts(ctx, otherUid)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(others))

	domain.Uid = otherUid
	assert.Equal(t, errs.ErrProductNotFound, Invoices.ModifyProduct(ctx, domain))

	domain.Uid = uid
	domain.Archived = true
	assert.Nil(t, Invoices.ModifyProduct(ctx, domain))
	byId, err := Invoices.GetProductsByIds(ctx, uid, []int64{domain.ProductId})
	assert.Nil(t, err)
	assert.True(t, byId[domain.ProductId].Archived)
}

func TestProductValidatesRenewal(t *testing.T) {
	ctx, uid := newAlertTestUser(t)

	err := Invoices.CreateProduct(ctx, &models.Product{Uid: uid, Name: "Hosting", RenewalMonths: 121, DefaultRenewalMode: models.RECURRING_INVOICE_MODE_CREATE})
	assert.Equal(t, errs.ErrRecurringInvoicePeriodInvalid, err)

	err = Invoices.CreateProduct(ctx, &models.Product{Uid: uid, Name: "Hosting", RenewalMonths: 12, DefaultRenewalMode: "weekly"})
	assert.Equal(t, errs.ErrRecurringInvoiceModeInvalid, err)

	// a one-off product never carries a mode
	oneOff := &models.Product{Uid: uid, Name: "Audit", DefaultRenewalMode: "weekly"}
	assert.Nil(t, Invoices.CreateProduct(ctx, oneOff))
	assert.Equal(t, "", oneOff.DefaultRenewalMode)
}

func TestInvoiceLinesKeepProductAndRefuseOtherUsersProduct(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	foreign := createTestProduct(t, otherUid, "Foreign", 0, "")

	invoice := createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/01", []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 150000},
	})
	_, lines, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, domain.ProductId, lines[0].ProductId)

	bad := &models.Invoice{Uid: uid, Number: "INV/2026-27/02", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30"}
	err = Invoices.CreateInvoice(ctx, bad, []*models.InvoiceLine{{ItemName: "x", ProductId: foreign.ProductId, Quantity: 100, Rate: 1}})
	assert.Equal(t, errs.ErrInvoiceLineProductInvalid, err)
}

func TestDeleteProductRefusedWhileLinked(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	invoice := createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/01", []*models.InvoiceLine{
		{ItemName: "codeation.io", ProductId: domain.ProductId, Quantity: 100, Rate: 150000},
	})

	assert.Equal(t, errs.ErrProductInUse, Invoices.DeleteProduct(ctx, uid, domain.ProductId))

	// a deleted invoice no longer holds the product
	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))
	assert.Nil(t, Invoices.DeleteProduct(ctx, uid, domain.ProductId))
	assert.Equal(t, errs.ErrProductNotFound, Invoices.DeleteProduct(ctx, uid, domain.ProductId))
}

func TestLinkPastInvoiceLinesByItemName(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/01", []*models.InvoiceLine{
		{ItemName: "Domain Renewal", Quantity: 100, Rate: 1},
		{ItemName: "Support", Quantity: 100, Rate: 1},
	})
	createTestInvoiceWithLines(t, uid, customer.CustomerId, "INV/2026-27/02", []*models.InvoiceLine{
		{ItemName: "  domain renewal ", Quantity: 100, Rate: 1},
	})

	groups, err := Invoices.GetUnlinkedInvoiceLineGroups(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(groups))
	assert.Equal(t, "Domain Renewal", groups[0].ItemName) // most used first
	assert.Equal(t, 2, groups[0].Count)

	domain := createTestProduct(t, uid, "Domain", 12, models.RECURRING_INVOICE_MODE_DRAFT_AHEAD)
	linked, err := Invoices.LinkInvoiceLinesToProduct(ctx, uid, "DOMAIN RENEWAL", domain.ProductId)
	assert.Nil(t, err)
	assert.Equal(t, int64(2), linked)

	groups, err = Invoices.GetUnlinkedInvoiceLineGroups(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(groups))
	assert.Equal(t, "Support", groups[0].ItemName)
}
