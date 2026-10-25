package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestInvoiceProfileDefaultsThenSave(t *testing.T) {
	ctx, uid := newAlertTestUser(t)

	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, "INV", profile.NumberPrefix)
	assert.Equal(t, int32(7), profile.DefaultTermsDays)

	profile.BusinessName = "Rishabh Jain"
	profile.NumberPrefix = "CA"
	assert.Nil(t, Invoices.SaveProfile(ctx, profile))

	profile.BusinessName = "Rishabh Jain (Codeation)"
	assert.Nil(t, Invoices.SaveProfile(ctx, profile))

	saved, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, "Rishabh Jain (Codeation)", saved.BusinessName)
	assert.Equal(t, "CA", saved.NumberPrefix)
}

func TestCustomerCrudIsScopedToUser(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)

	customer := &models.Customer{Uid: uid, Name: "Quber Tech", ContactName: "Kanika Batra"}
	assert.Nil(t, Invoices.CreateCustomer(ctx, customer))
	assert.True(t, customer.CustomerId > 0)

	_, err := Invoices.GetCustomer(ctx, otherUid, customer.CustomerId)
	assert.Equal(t, errs.ErrCustomerNotFound, err)

	customer.Email = "kanika.batra@qubertech.com"
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))

	all, err := Invoices.GetAllCustomers(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(all))
	assert.Equal(t, "kanika.batra@qubertech.com", all[0].Email)

	assert.Nil(t, Invoices.DeleteCustomer(ctx, uid, customer.CustomerId))
	all, err = Invoices.GetAllCustomers(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(all))
}

func createTestInvoice(t *testing.T, uid int64, customerId int64, number string, rate int64) *models.Invoice {
	t.Helper()
	invoice := &models.Invoice{Uid: uid, Number: number, CustomerId: customerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30"}
	lines := []*models.InvoiceLine{{ItemName: "Consulting", Description: "Software Consulting\nQuber Tech", Quantity: 1, Rate: rate}}
	assert.Nil(t, Invoices.CreateInvoice(core.NewNullContext(), invoice, lines))
	return invoice
}

func createTestCustomer(t *testing.T, uid int64) *models.Customer {
	t.Helper()
	customer := &models.Customer{Uid: uid, Name: "Quber Tech", ContactName: "Kanika Batra", AddressLines: "167-169 Great Portland Street\nLondon, W1W 5PF\nUnited Kingdom"}
	assert.Nil(t, Invoices.CreateCustomer(core.NewNullContext(), customer))
	return customer
}

func TestComputeInvoiceTotalFillsLineAmounts(t *testing.T) {
	lines := []*models.InvoiceLine{{Quantity: 1, Rate: 37000000}, {Quantity: 3, Rate: 150000}}
	assert.Equal(t, int64(37450000), computeInvoiceTotal(lines))
	assert.Equal(t, int64(450000), lines[1].Amount)
}

func TestInvoiceFinancialYear(t *testing.T) {
	fy, err := invoiceFinancialYear("2026-04-23")
	assert.Nil(t, err)
	assert.Equal(t, "2026-27", fy)

	fy, err = invoiceFinancialYear("2026-03-31")
	assert.Nil(t, err)
	assert.Equal(t, "2025-26", fy)

	fy, err = invoiceFinancialYear("2099-12-01")
	assert.Nil(t, err)
	assert.Equal(t, "2099-00", fy)

	_, err = invoiceFinancialYear("23/04/2026")
	assert.Equal(t, errs.ErrInvoiceDateInvalid, err)
}

func TestNextInvoiceSequenceSkipsGapsAndOtherSeries(t *testing.T) {
	numbers := []string{"CA/2026-27/01", "CA/2026-27/03", "CA/2025-26/09", "XX/2026-27/07", "CA/2026-27/draft"}
	assert.Equal(t, 4, nextInvoiceSequence(numbers, "CA/2026-27/"))
	assert.Equal(t, 1, nextInvoiceSequence(nil, "CA/2026-27/"))
}

func TestCreateInvoiceSnapshotsCustomerAndRejectsDuplicateNumber(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	assert.Equal(t, int64(37000000), invoice.Total)
	assert.Equal(t, models.INVOICE_DEFAULT_CURRENCY, invoice.Currency)
	assert.Equal(t, models.INVOICE_KIND_INVOICE, invoice.Kind)

	customer.AddressLines = "New address"
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))

	saved, lines, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "167-169 Great Portland Street\nLondon, W1W 5PF\nUnited Kingdom", saved.BillToAddressLines)
	assert.Equal(t, 1, len(lines))
	assert.Equal(t, int64(37000000), lines[0].Amount)

	duplicate := &models.Invoice{Uid: uid, Number: "CA/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-05-01", DueDate: "2026-05-08"}
	err = Invoices.CreateInvoice(ctx, duplicate, []*models.InvoiceLine{{ItemName: "x", Quantity: 1, Rate: 100}})
	assert.Equal(t, errs.ErrInvoiceNumberAlreadyExists, err)
}

func TestCreateInvoiceValidatesDatesAndCustomer(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	otherCustomer := createTestCustomer(t, otherUid)
	lines := []*models.InvoiceLine{{ItemName: "x", Quantity: 1, Rate: 100}}

	err := Invoices.CreateInvoice(ctx, &models.Invoice{Uid: uid, Number: "A", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-30", DueDate: "2026-04-23"}, lines)
	assert.Equal(t, errs.ErrInvoiceDueDateBeforeInvoiceDate, err)

	err = Invoices.CreateInvoice(ctx, &models.Invoice{Uid: uid, Number: "B", CustomerId: customer.CustomerId, InvoiceDate: "2026-02-30", DueDate: "2026-04-23"}, lines)
	assert.Equal(t, errs.ErrInvoiceDateInvalid, err)

	err = Invoices.CreateInvoice(ctx, &models.Invoice{Uid: uid, Number: "C", CustomerId: otherCustomer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30"}, lines)
	assert.Equal(t, errs.ErrCustomerNotFound, err)
}

func TestModifyInvoiceReplacesLinesAndKeepsClosedFlag(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	assert.Nil(t, Invoices.SetInvoiceClosed(ctx, uid, invoice.InvoiceId, true))

	modified := &models.Invoice{Uid: uid, InvoiceId: invoice.InvoiceId, Number: "CA/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-05-07"}
	newLines := []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 1, Rate: 30000000}, {ItemName: "Support", Quantity: 2, Rate: 100000}}
	assert.Nil(t, Invoices.ModifyInvoice(ctx, modified, newLines))

	saved, lines, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(lines))
	assert.Equal(t, "Support", lines[1].ItemName)
	assert.Equal(t, int64(30200000), saved.Total)
	assert.Equal(t, "2026-05-07", saved.DueDate)
	assert.True(t, saved.ClosedManually)
}

func TestNextInvoiceNumberUsesProfilePrefixAndFinancialYear(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	profile, _ := Invoices.GetProfile(ctx, uid)
	profile.NumberPrefix = "CA"
	assert.Nil(t, Invoices.SaveProfile(ctx, profile))

	next, err := Invoices.NextInvoiceNumber(ctx, uid, "2026-04-23")
	assert.Nil(t, err)
	assert.Equal(t, "CA/2026-27/01", next)

	customer := createTestCustomer(t, uid)
	createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 100)
	createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/03", 100)

	next, err = Invoices.NextInvoiceNumber(ctx, uid, "2026-06-01")
	assert.Nil(t, err)
	assert.Equal(t, "CA/2026-27/04", next)

	next, err = Invoices.NextInvoiceNumber(ctx, uid, "2027-04-01")
	assert.Nil(t, err)
	assert.Equal(t, "CA/2027-28/01", next)
}

func TestDeleteInvoiceHidesIt(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 100)

	assert.Equal(t, errs.ErrCustomerInUse, Invoices.DeleteCustomer(ctx, uid, customer.CustomerId))
	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))

	_, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	all, err := Invoices.GetAllInvoices(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(all))

	// a deleted invoice frees its number
	createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 100)
}
