package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
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

	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, first.TransactionId))
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, second.TransactionId))

	payments, err := Invoices.GetInvoicePayments(ctx, uid, []int64{invoice.InvoiceId})
	assert.Nil(t, err)
	assert.Equal(t, 2, len(payments[invoice.InvoiceId]))
	assert.Equal(t, first.TransactionId, payments[invoice.InvoiceId][0].TransactionId)

	assert.Equal(t, errs.ErrInvoicePaymentAlreadyLinked, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, first.TransactionId))

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
	assert.Equal(t, errs.ErrInvoicePaymentTransactionInvalid, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, expense.TransactionId))

	tooBig := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000001, "2026-04-28")
	assert.Equal(t, errs.ErrInvoicePaymentExceedsBalanceDue, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, tooBig.TransactionId))
}

func TestLinkPaymentRejectsNonInrAccount(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	usdAccount := &models.Account{Uid: uid, Name: "Wise USD", Category: models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT, Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Icon: 1, Color: "000000", Currency: "USD"}
	assert.Nil(t, Accounts.CreateAccounts(ctx, usdAccount, 0, nil, nil, time.UTC))
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	usdIncome := insertTestTransaction(t, uid, usdAccount.AccountId, models.TRANSACTION_DB_TYPE_INCOME, 1000, "2026-04-28")
	assert.Equal(t, errs.ErrInvoicePaymentTransactionInvalid, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, usdIncome.TransactionId))
}

func TestLinkPaymentRejectsOtherUsersTransaction(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	otherCtx, otherUid := newAlertTestUser(t, "Other Savings")
	otherAccountId := accountIdByName(t, otherCtx, otherUid, "Other Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)

	othersIncome := insertTestTransaction(t, otherUid, otherAccountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-04-28")
	assert.Equal(t, errs.ErrTransactionNotFound, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, othersIncome.TransactionId))
}

func TestInvoiceReceivedIgnoresDeletedTransaction(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "PNB Savings")
	accountId := accountIdByName(t, ctx, uid, "PNB Savings")
	customer := createTestCustomer(t, uid)
	invoice := createTestInvoice(t, uid, customer.CustomerId, "CA/2026-27/01", 37000000)
	income := insertTestTransaction(t, uid, accountId, models.TRANSACTION_DB_TYPE_INCOME, 37000000, "2026-04-28")
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, invoice.InvoiceId, income.TransactionId))

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
	assert.Nil(t, Invoices.LinkPayment(ctx, uid, other.InvoiceId, linkedElsewhere.TransactionId))

	suggestions, err := Invoices.GetPaymentSuggestions(ctx, uid, invoice.InvoiceId)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(suggestions))
	assert.Equal(t, exact.TransactionId, suggestions[0].TransactionId)
	assert.Equal(t, short.TransactionId, suggestions[1].TransactionId)
}
