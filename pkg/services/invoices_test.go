package services

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/invoicepdf"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
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

func TestCustomerExtraFieldsPersistAndSnapshotOnInvoice(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	customer.Phone = "+91 98765 43210"
	customer.Website = "https://qubertech.com"
	customer.AccountNumber = "QT-001"
	customer.Pan = "ABCDE1234F"
	customer.TermsDays = 15
	customer.Notes = "pays by RTGS"
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))

	saved, err := Invoices.GetCustomer(ctx, uid, customer.CustomerId)
	assert.Nil(t, err)
	assert.Equal(t, "+91 98765 43210", saved.Phone)
	assert.Equal(t, "https://qubertech.com", saved.Website)
	assert.Equal(t, "QT-001", saved.AccountNumber)
	assert.Equal(t, "ABCDE1234F", saved.Pan)
	assert.Equal(t, int32(15), saved.TermsDays)
	assert.Equal(t, "pays by RTGS", saved.Notes)

	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	assert.Equal(t, "+91 98765 43210", invoice.BillToPhone)
	assert.Equal(t, "ABCDE1234F", invoice.BillToPan)

	customer.Pan = "ZZZZZ9999Z"
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))
	invoice.InvoiceDate, invoice.DueDate = "2026-04-23", "2026-04-30"
	assert.Nil(t, Invoices.ModifyInvoice(ctx, invoice, []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 100, Rate: 37000000}}))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "ZZZZZ9999Z", reloaded.BillToPan)
}

func createTestInvoice(t *testing.T, uid int64, customerId int64, number string, rate int64) *models.Invoice {
	t.Helper()
	invoice := &models.Invoice{Uid: uid, Number: number, CustomerId: customerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30"}
	lines := []*models.InvoiceLine{{ItemName: "Consulting", Description: "Software Consulting\nQuber Tech", Quantity: 100, Rate: rate}}
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
	lines := []*models.InvoiceLine{{Quantity: 100, Rate: 37000000}, {Quantity: 300, Rate: 150000}}
	assert.Equal(t, int64(37450000), computeInvoiceTotals(lines, models.INVOICE_TAX_TYPE_NONE, 0).Total)
	assert.Equal(t, int64(450000), lines[1].Amount)
}

func TestComputeInvoiceTotalRoundsFractionalQuantityToPaisa(t *testing.T) {
	// 7.5 h × ₹333.33 = ₹2,499.975 → ₹2,499.98
	lines := []*models.InvoiceLine{{Quantity: 750, Rate: 33333}}
	assert.Equal(t, int64(249998), computeInvoiceTotals(lines, models.INVOICE_TAX_TYPE_NONE, 0).Total)

	// 0.01 × ₹0.01 rounds to zero; this is the only way a line amount can be zero
	lines = []*models.InvoiceLine{{Quantity: 1, Rate: 1}}
	assert.Equal(t, int64(0), computeInvoiceTotals(lines, models.INVOICE_TAX_TYPE_NONE, 0).Total)
	assert.Equal(t, int64(0), lines[0].Amount)
}

func TestComputeInvoiceTotalsGst(t *testing.T) {
	// 18% on ₹1,00,000.01 rounds once: tax ₹18,000.00, split evenly
	lines := []*models.InvoiceLine{{Quantity: 100, Rate: 10000001}}
	totals := computeInvoiceTotals(lines, models.INVOICE_TAX_TYPE_CGST_SGST, 1800)
	assert.Equal(t, invoiceTotals{Subtotal: 10000001, TaxAmount: 1800000, Cgst: 900000, Sgst: 900000, Total: 11800001}, totals)

	// IGST uses the same tax amount, no split needed but reported the same way
	totals = computeInvoiceTotals(lines, models.INVOICE_TAX_TYPE_IGST, 1800)
	assert.Equal(t, int64(1800000), totals.TaxAmount)
	assert.Equal(t, int64(11800001), totals.Total)

	// an odd tax paisa goes to SGST: 18% of ₹0.05 = 0.9 paisa → 1 paisa
	totals = computeInvoiceTotals([]*models.InvoiceLine{{Quantity: 100, Rate: 5}}, models.INVOICE_TAX_TYPE_CGST_SGST, 1800)
	assert.Equal(t, invoiceTotals{Subtotal: 5, TaxAmount: 1, Cgst: 0, Sgst: 1, Total: 6}, totals)

	// half a paisa rounds up: 18% of ₹0.25 = 4.5 paise → 5
	totals = computeInvoiceTotals([]*models.InvoiceLine{{Quantity: 100, Rate: 25}}, models.INVOICE_TAX_TYPE_IGST, 1800)
	assert.Equal(t, int64(5), totals.TaxAmount)

	// none yields zero tax whatever the rate; an empty type (rows saved before GST) is none
	totals = computeInvoiceTotals(lines, models.INVOICE_TAX_TYPE_NONE, 1800)
	assert.Equal(t, invoiceTotals{Subtotal: 10000001, TaxAmount: 0, Cgst: 0, Sgst: 0, Total: 10000001}, totals)
	totals = computeInvoiceTotals(lines, "", 1800)
	assert.Equal(t, int64(0), totals.TaxAmount)
	assert.Equal(t, int64(10000001), totals.Total)
}

func TestCustomerGstFieldsPersistAndInvoiceStoresTax(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	customer.Gstin = "07AAAAA0000A1Z5"
	customer.TaxType = models.INVOICE_TAX_TYPE_CGST_SGST
	customer.TaxRate = 1800
	assert.Nil(t, Invoices.ModifyCustomer(ctx, customer))

	saved, err := Invoices.GetCustomer(ctx, uid, customer.CustomerId)
	assert.Nil(t, err)
	assert.Equal(t, "07AAAAA0000A1Z5", saved.Gstin)
	assert.Equal(t, models.INVOICE_TAX_TYPE_CGST_SGST, saved.TaxType)
	assert.Equal(t, int32(1800), saved.TaxRate)

	invoice := &models.Invoice{Uid: uid, Number: "CA/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30",
		TaxType: models.INVOICE_TAX_TYPE_CGST_SGST, TaxRate: 1800}
	lines := []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 100, Rate: 10000001, Sac: "998314"}}
	assert.Nil(t, Invoices.CreateInvoice(ctx, invoice, lines))

	reloaded, savedLines, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "07AAAAA0000A1Z5", reloaded.BillToGstin)
	assert.Equal(t, models.INVOICE_TAX_TYPE_CGST_SGST, reloaded.TaxType)
	assert.Equal(t, int32(1800), reloaded.TaxRate)
	assert.Equal(t, int64(10000001), reloaded.Subtotal)
	assert.Equal(t, int64(1800000), reloaded.TaxAmount)
	assert.Equal(t, int64(11800001), reloaded.Total)
	assert.Equal(t, "998314", savedLines[0].Sac)

	// modifying to IGST at 5% recomputes and stores the new tax
	modified := &models.Invoice{Uid: uid, InvoiceId: invoice.InvoiceId, Number: "CA/2026-27/01", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30",
		TaxType: models.INVOICE_TAX_TYPE_IGST, TaxRate: 500}
	assert.Nil(t, Invoices.ModifyInvoice(ctx, modified, []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 100, Rate: 10000000}}))

	reloaded, _, err = Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_TAX_TYPE_IGST, reloaded.TaxType)
	assert.Equal(t, int32(500), reloaded.TaxRate)
	assert.Equal(t, int64(10000000), reloaded.Subtotal)
	assert.Equal(t, int64(500000), reloaded.TaxAmount)
	assert.Equal(t, int64(10500000), reloaded.Total)

	// back to none clears the tax but keeps the subtotal
	modified.TaxType, modified.TaxRate = models.INVOICE_TAX_TYPE_NONE, 0
	assert.Nil(t, Invoices.ModifyInvoice(ctx, modified, []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 100, Rate: 10000000}}))
	reloaded, _, err = Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), reloaded.TaxAmount)
	assert.Equal(t, int64(10000000), reloaded.Subtotal)
	assert.Equal(t, int64(10000000), reloaded.Total)
}

func TestInvoiceProfileGstFieldsPersist(t *testing.T) {
	ctx, uid := newAlertTestUser(t)

	assert.Nil(t, Invoices.SaveProfile(ctx, &models.InvoiceProfile{Uid: uid, NumberPrefix: "CA", Gstin: "07AAAAA0000A1Z5", DefaultSac: "998314"}))

	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, "07AAAAA0000A1Z5", profile.Gstin)
	assert.Equal(t, "998314", profile.DefaultSac)
}

func TestInvoiceQuantityIsStoredAsGiven(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	invoice := &models.Invoice{Uid: uid, Number: "CA/2026-27/90", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30"}
	lines := []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 750, Rate: 33333}}
	assert.Nil(t, Invoices.CreateInvoice(ctx, invoice, lines))

	_, savedLines, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(750), savedLines[0].Quantity)
	assert.Equal(t, int64(249998), savedLines[0].Amount)
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
	err = Invoices.CreateInvoice(ctx, duplicate, []*models.InvoiceLine{{ItemName: "x", Quantity: 100, Rate: 100}})
	assert.Equal(t, errs.ErrInvoiceNumberAlreadyExists, err)
}

func TestCreateInvoiceValidatesDatesAndCustomer(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	otherCustomer := createTestCustomer(t, otherUid)
	lines := []*models.InvoiceLine{{ItemName: "x", Quantity: 100, Rate: 100}}

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
	newLines := []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 100, Rate: 30000000}, {ItemName: "Support", Quantity: 200, Rate: 100000}}
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

// insertTestTransaction writes a transaction row directly (no category or balance bookkeeping needed for invoice tests)
var transactionTimeCounter int64

func insertTestTransaction(t *testing.T, uid int64, accountId int64, typ models.TransactionDbType, amount int64, date string) *models.Transaction {
	t.Helper()
	at, err := time.Parse("2006-01-02", date)
	assert.Nil(t, err)
	// Add a large offset to ensure unique transaction times (unit: seconds, so *1000 for millisecond variations)
	timeOffset := time.Duration(transactionTimeCounter) * time.Second
	transactionTimeCounter++
	transaction := &models.Transaction{
		TransactionId: uuid.Container.GenerateUuid(uuid.UUID_TYPE_TRANSACTION), Uid: uid, Type: typ, AccountId: accountId,
		Amount: amount, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(at.Add(12 * time.Hour).Add(timeOffset).Unix()), TimezoneUtcOffset: 330,
	}
	_, err = datastore.Container.UserDataStore.Choose(uid).NewSession(core.NewNullContext()).Insert(transaction)
	assert.Nil(t, err)
	return transaction
}

func TestRankInvoicePaymentSuggestions(t *testing.T) {
	small := &models.Transaction{TransactionId: 1, Amount: 10000000, TransactionTime: 3}
	short := &models.Transaction{TransactionId: 2, Amount: 36000000, TransactionTime: 2}
	exact := &models.Transaction{TransactionId: 3, Amount: 37000000, TransactionTime: 1}
	ranked := rankInvoicePaymentSuggestions([]*models.Transaction{small, short, exact}, 37000000)
	assert.Equal(t, []int64{3, 2, 1}, []int64{ranked[0].TransactionId, ranked[1].TransactionId, ranked[2].TransactionId})
}

func TestLinkPaymentsAccumulatesReceived(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	first := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 20000000, "2026-04-28")
	second := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 16850000, "2026-05-03")

	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, first.TransactionId, 0))
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, second.TransactionId, 0))

	payments, err := Invoices.GetInvoicePayments(ctx, uid, []int64{invoice.InvoiceId})
	assert.Nil(t, err)
	assert.Equal(t, 2, len(payments[invoice.InvoiceId]))
	assert.Equal(t, first.TransactionId, payments[invoice.InvoiceId][0].Transaction.TransactionId)

	assert.Equal(t, errs.ErrInvoicePaymentAlreadyLinked, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, first.TransactionId, 0))

	assert.Nil(t, Invoices.UnlinkPayment(ctx, uid, invoice.InvoiceId, second.TransactionId))
	assert.Equal(t, errs.ErrInvoicePaymentNotFound, Invoices.UnlinkPayment(ctx, uid, invoice.InvoiceId, second.TransactionId))

	payments, err = Invoices.GetInvoicePayments(ctx, uid, []int64{invoice.InvoiceId})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(payments[invoice.InvoiceId]))
}

func TestLinkPaymentValidatesTransaction(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	expense := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_EXPENSE, 1000, "2026-04-28")
	assert.Equal(t, errs.ErrInvoicePaymentTransactionInvalid, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, expense.TransactionId, 0))

	tooBig := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000001, "2026-04-28")
	assert.Equal(t, errs.ErrInvoicePaymentExceedsBalanceDue, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, tooBig.TransactionId, 0))
}

// createTestAccount creates a single account in the given currency and returns its id
func createTestAccount(t *testing.T, ctx core.Context, uid int64, name string, currency string) int64 {
	t.Helper()
	account := &models.Account{Uid: uid, Name: name, Category: models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT, Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Icon: 1, Color: "000000", Currency: currency}
	assert.Nil(t, Accounts.CreateAccounts(ctx, account, 0, nil, nil, time.UTC))
	return account.AccountId
}

func TestLinkPaymentCrossCurrencyRequiresSettledAmount(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	usdAccountId := createTestAccount(t, ctx, uid, "Wise USD", "USD")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	usdIncome := insertTestTransaction(t, uid, usdAccountId, models.TRANSACTION_DB_TYPE_INCOME, 440000, "2026-04-28")
	assert.Equal(t, errs.ErrInvoicePaymentSettledAmountRequired, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, usdIncome.TransactionId, 0))
	assert.Equal(t, errs.ErrInvoicePaymentExceedsBalanceDue, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, usdIncome.TransactionId, 37000001))
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, usdIncome.TransactionId, 36850000))
}

func TestLinkPaymentSameCurrencyRefusesSettledAmount(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	income := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-04-28")
	assert.Equal(t, errs.ErrInvoicePaymentSettledAmountNotAllowed, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, income.TransactionId, 37000000))
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, income.TransactionId, 0))
}

func TestInvoiceReceivedUsesSettledAmounts(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	inrAccountId := accountIdByName(t, ctx, uid, "PNB Savings")
	usdAccountId := createTestAccount(t, ctx, uid, "Wise USD", "USD")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	usdIncome := insertTestTransaction(t, uid, usdAccountId, models.TRANSACTION_DB_TYPE_INCOME, 440000, "2026-04-28")
	inrIncome := insertTestTransaction(t, uid, inrAccountId, models.TRANSACTION_DB_TYPE_INCOME, 150000, "2026-05-03")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, usdIncome.TransactionId, 36850000))
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, inrIncome.TransactionId, 0))

	payments, err := Invoices.GetInvoicePayments(ctx, uid, []int64{invoice.InvoiceId})
	assert.Nil(t, err)
	assert.Equal(t, 2, len(payments[invoice.InvoiceId]))
	assert.Equal(t, "USD", payments[invoice.InvoiceId][0].Currency)
	assert.Equal(t, int64(440000), payments[invoice.InvoiceId][0].Transaction.Amount)
	assert.Equal(t, int64(36850000), payments[invoice.InvoiceId][0].SettledAmount)
	assert.Equal(t, "INR", payments[invoice.InvoiceId][1].Currency)
	assert.Equal(t, int64(0), payments[invoice.InvoiceId][1].SettledAmount)
	assert.Equal(t, int64(37000000), models.InvoiceReceivedAmount(payments[invoice.InvoiceId]))

	// the detail / list response reports the same received amount
	resp := invoice.ToInvoiceInfoResponse(nil, payments[invoice.InvoiceId])
	assert.Equal(t, int64(37000000), resp.Received)

	// a further same-currency link is refused: nothing is due any more
	extra := insertTestTransaction(t, uid, inrAccountId, models.TRANSACTION_DB_TYPE_INCOME, 1, "2026-05-04")
	assert.Equal(t, errs.ErrInvoicePaymentExceedsBalanceDue, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, extra.TransactionId, 0))
}

func TestPaymentSuggestionsIncludeOtherCurrencyAfterSameCurrency(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	inrAccountId := accountIdByName(t, ctx, uid, "PNB Savings")
	usdAccountId := createTestAccount(t, ctx, uid, "Wise USD", "USD")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	usdOlder := insertTestTransaction(t, uid, usdAccountId, models.TRANSACTION_DB_TYPE_INCOME, 440000, "2026-04-28")
	usdNewer := insertTestTransaction(t, uid, usdAccountId, models.TRANSACTION_DB_TYPE_INCOME, 99999999, "2026-05-05") // not capped by the balance due
	inrShort := insertTestTransaction(t, uid, inrAccountId, models.TRANSACTION_DB_TYPE_INCOME, 36850000, "2026-05-06")
	insertTestTransaction(t, uid, usdAccountId, models.TRANSACTION_DB_TYPE_INCOME, 440000, "2026-03-01")  // before invoice date
	insertTestTransaction(t, uid, usdAccountId, models.TRANSACTION_DB_TYPE_EXPENSE, 440000, "2026-05-02") // not income

	suggestions, err := Invoices.GetPaymentSuggestions(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(suggestions))
	assert.Equal(t, inrShort.TransactionId, suggestions[0].Transaction.TransactionId)
	assert.Equal(t, "INR", suggestions[0].Currency)
	assert.Equal(t, usdNewer.TransactionId, suggestions[1].Transaction.TransactionId)
	assert.Equal(t, "USD", suggestions[1].Currency)
	assert.Equal(t, usdOlder.TransactionId, suggestions[2].Transaction.TransactionId)
}

func TestCreateInvoiceCurrencyDefaultsAndValidation(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)

	// no profile saved: INR, exactly as before
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	assert.Equal(t, "INR", invoice.Currency)

	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	profile.DefaultCurrency = "USD"
	assert.Nil(t, Invoices.SaveProfile(ctx, profile))
	saved, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, "USD", saved.DefaultCurrency)

	invoice = createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/02", 100000)
	assert.Equal(t, "USD", invoice.Currency)

	explicit := &models.Invoice{Uid: uid, Number: "CA/2026-27/03", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30", Currency: "GBP"}
	assert.Nil(t, Invoices.CreateInvoice(ctx, explicit, []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 100, Rate: 100000}}))
	assert.Equal(t, "GBP", explicit.Currency)

	unknown := &models.Invoice{Uid: uid, Number: "CA/2026-27/04", CustomerId: customer.CustomerId, InvoiceDate: "2026-04-23", DueDate: "2026-04-30", Currency: "XXX"}
	assert.Equal(t, errs.ErrInvoiceCurrencyInvalid, Invoices.CreateInvoice(ctx, unknown, []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 100, Rate: 100000}}))
}

func TestModifyInvoiceIgnoresCurrency(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	assert.Equal(t, "INR", invoice.Currency)

	invoice.Currency = "USD"
	assert.Nil(t, Invoices.ModifyInvoice(ctx, invoice, []*models.InvoiceLine{{ItemName: "Consulting", Quantity: 100, Rate: 37000000}}))

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "INR", reloaded.Currency)
}

func TestLinkPaymentRejectsOtherUsersTransaction(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	otherCtx, otherUid := newAlertTestUser(t, "Other Savings")
	otherAccountId := accountIdByName(t, otherCtx, otherUid, "Other Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	othersIncome := insertTestTransaction(t, otherUid, otherAccountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-04-28")
	assert.Equal(t, errs.ErrTransactionNotFound, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, othersIncome.TransactionId, 0))
}

func TestInvoiceReceivedIgnoresDeletedTransaction(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	income := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-04-28")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, income.TransactionId, 0))

	_, err := datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).ID(income.TransactionId).Cols("deleted").Update(&models.Transaction{Deleted: true})
	assert.Nil(t, err)

	payments, err := Invoices.GetInvoicePayments(ctx, uid, []int64{invoice.InvoiceId})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(payments[invoice.InvoiceId]))
}

func TestPaymentSuggestionsFilterAndRank(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	other := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/02", 100000)

	exact := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-05-02")
	short := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 36850000, "2026-04-29")
	insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-03-01")  // before invoice date
	insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-07-30")  // more than 60 days after due
	insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 40000000, "2026-05-02")  // more than balance due
	insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_EXPENSE, 37000000, "2026-05-02") // not income
	linkedElsewhere := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 100000, "2026-05-02")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, other.InvoiceId, linkedElsewhere.TransactionId, 0))

	suggestions, err := Invoices.GetPaymentSuggestions(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(suggestions))
	assert.Equal(t, exact.TransactionId, suggestions[0].Transaction.TransactionId)
	assert.Equal(t, short.TransactionId, suggestions[1].Transaction.TransactionId)
}

func TestGetLinkedInvoicesByTransactionIds(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	linked := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 20000000, "2026-04-28")
	unlinked := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 1000, "2026-04-29")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, linked.TransactionId, 0))

	otherCtx, otherUid := newAlertTestUser(t, "Other Savings")
	otherAccountId := accountIdByName(t, otherCtx, otherUid, "Other Savings")
	otherCustomer := createTestCustomer(t, otherUid)
	otherInvoice := createTestInvoice(t, otherUid, otherCustomer.CustomerId, "CA/2026-27/01", 37000000)
	othersLinked := insertTestTransaction(t, otherUid, otherAccountId, models.TRANSACTION_DB_TYPE_INCOME, 20000000, "2026-04-28")
	assert.Nil(t, Invoices.LinkPayment(otherCtx, otherUid, otherInvoice.InvoiceId, othersLinked.TransactionId, 0))

	linkedInvoices, err := Invoices.GetLinkedInvoicesByTransactionIds(ctx, uid, []int64{linked.TransactionId, unlinked.TransactionId, othersLinked.TransactionId})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(linkedInvoices))
	assert.Equal(t, invoice.InvoiceId, linkedInvoices[linked.TransactionId].InvoiceId)
	assert.Equal(t, "CA/2026-27/01", linkedInvoices[linked.TransactionId].Number)
	assert.Nil(t, linkedInvoices[unlinked.TransactionId])
	assert.Nil(t, linkedInvoices[othersLinked.TransactionId])

	assert.Nil(t, Invoices.DeleteInvoice(ctx, uid, invoice.InvoiceId))
	linkedInvoices, err = Invoices.GetLinkedInvoicesByTransactionIds(ctx, uid, []int64{linked.TransactionId})
	assert.Nil(t, err)
	assert.Equal(t, 0, len(linkedInvoices))

	empty, err := Invoices.GetLinkedInvoicesByTransactionIds(ctx, uid, nil)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(empty))
}

func TestInvoicePublicTokenViewReturnsInvoice(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	profile.BusinessName = "Rishabh Jain"
	assert.Nil(t, Invoices.SaveProfile(ctx, profile))

	token, err := Invoices.SetInvoicePublicToken(ctx, uid, invoice.InvoiceId, true)
	assert.Nil(t, err)
	assert.Equal(t, 43, len(token))

	viewed, lines, received, viewedProfile, err := Invoices.GetInvoiceByPublicToken(ctx, token)
	assert.Nil(t, err)
	assert.Equal(t, invoice.InvoiceId, viewed.InvoiceId)
	assert.Equal(t, "CA/2026-27/01", viewed.Number)
	assert.Equal(t, 1, len(lines))
	assert.Equal(t, int64(0), received)
	assert.Equal(t, "Rishabh Jain", viewedProfile.BusinessName)

	// the token is also returned with the invoice so the owner can show the link
	reloaded, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, token, reloaded.PublicToken)
}

func TestInvoicePublicTokenClearedReturnsNotFound(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 100)

	_, _, _, _, err := Invoices.GetInvoiceByPublicToken(ctx, "")
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	token, err := Invoices.SetInvoicePublicToken(ctx, uid, invoice.InvoiceId, true)
	assert.Nil(t, err)

	cleared, err := Invoices.SetInvoicePublicToken(ctx, uid, invoice.InvoiceId, false)
	assert.Nil(t, err)
	assert.Equal(t, "", cleared)

	_, _, _, _, err = Invoices.GetInvoiceByPublicToken(ctx, token)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	reloaded, _, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, "", reloaded.PublicToken)
}

func TestInvoicePublicTokenResetInvalidatesOldToken(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 100)

	first, err := Invoices.SetInvoicePublicToken(ctx, uid, invoice.InvoiceId, true)
	assert.Nil(t, err)
	second, err := Invoices.SetInvoicePublicToken(ctx, uid, invoice.InvoiceId, true)
	assert.Nil(t, err)
	assert.NotEqual(t, first, second)

	_, _, _, _, err = Invoices.GetInvoiceByPublicToken(ctx, first)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	viewed, _, _, _, err := Invoices.GetInvoiceByPublicToken(ctx, second)
	assert.Nil(t, err)
	assert.Equal(t, invoice.InvoiceId, viewed.InvoiceId)
}

func TestInvoicePublicTokenCrossUserUpdateIsRefused(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 100)

	token, err := Invoices.SetInvoicePublicToken(ctx, uid, invoice.InvoiceId, true)
	assert.Nil(t, err)

	_, err = Invoices.SetInvoicePublicToken(ctx, otherUid, invoice.InvoiceId, true)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)
	_, err = Invoices.SetInvoicePublicToken(ctx, otherUid, invoice.InvoiceId, false)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	// the owner's link still works after the refused update
	viewed, _, _, _, err := Invoices.GetInvoiceByPublicToken(ctx, token)
	assert.Nil(t, err)
	assert.Equal(t, invoice.InvoiceId, viewed.InvoiceId)
}

// the PDF endpoint loads through GetInvoice / GetInvoicePayments / GetProfile, so a foreign id must stop there
func TestGetInvoiceRejectsOtherUser(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	_, otherUid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	_, _, err := Invoices.GetInvoice(ctx, otherUid, invoice.InvoiceId)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	_, _, err = Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
}

func TestRenderInvoicePdfFromServiceData(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	profile, err := Invoices.GetProfile(ctx, uid)
	assert.Nil(t, err)
	loaded, lines, err := Invoices.GetInvoice(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, int64(37000000), loaded.Total)

	pdf, err := invoicepdf.Render(profile, loaded, lines, 0, "")
	assert.Nil(t, err)
	assert.True(t, bytes.HasPrefix(pdf, []byte("%PDF")))
	assert.Equal(t, "Invoice_CA_2026-27_01_2026-04-30.pdf", invoicepdf.FileName(loaded.Number, loaded.DueDate))
}
