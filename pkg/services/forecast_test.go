package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/forecast"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestForecastToAccounts(t *testing.T) {
	got := toAccounts([]*models.Account{
		{AccountId: 1, Category: models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT, Balance: 100},
		{AccountId: 2, Category: models.ACCOUNT_CATEGORY_CREDIT_CARD, Balance: -300},
		{AccountId: 3, Category: models.ACCOUNT_CATEGORY_INVESTMENT, Balance: 900},
		{AccountId: 4, Category: models.ACCOUNT_CATEGORY_CASH, Balance: 5, Hidden: true},
	})
	assert.Equal(t, []forecast.Account{
		{Id: 1, Balance: 100, Cash: true},
		{Id: 2, Balance: 300, Liability: true},
		{Id: 3, Balance: 900},
	}, got)
}

func TestForecastToBudgets_OnlyMonthlySingleCategory(t *testing.T) {
	b := []*models.Budget{{BudgetId: 1, PeriodType: models.BUDGET_PERIOD_TYPE_MONTHLY, Amount: 500}, {BudgetId: 2, PeriodType: models.BUDGET_PERIOD_TYPE_YEARLY, Amount: 9}}
	scopes := map[int64][]*models.BudgetScope{
		1: {{ScopeType: models.BUDGET_SCOPE_TYPE_CATEGORY, ScopeItemId: 7}},
		2: {{ScopeType: models.BUDGET_SCOPE_TYPE_CATEGORY, ScopeItemId: 8}},
	}
	assert.Equal(t, []forecast.Budget{{CategoryId: 7, Amount: 500}}, toBudgets(b, scopes))
}

func TestForecastToBudgets_MultiScopeOrNonCategoryScopeExcluded(t *testing.T) {
	b := []*models.Budget{{BudgetId: 1, PeriodType: models.BUDGET_PERIOD_TYPE_MONTHLY, Amount: 500}, {BudgetId: 2, PeriodType: models.BUDGET_PERIOD_TYPE_MONTHLY, Amount: 600}}
	scopes := map[int64][]*models.BudgetScope{
		1: {{ScopeType: models.BUDGET_SCOPE_TYPE_CATEGORY, ScopeItemId: 7}, {ScopeType: models.BUDGET_SCOPE_TYPE_CATEGORY, ScopeItemId: 8}},
		2: {{ScopeType: models.BUDGET_SCOPE_TYPE_ACCOUNT, ScopeItemId: 9}},
	}
	assert.Empty(t, toBudgets(b, scopes))
}

func TestForecastToCards(t *testing.T) {
	statementDay := 14
	dueDay := 3
	payFrom := int64(7731)

	got := toCards([]*models.Account{
		{AccountId: 4708, Category: models.ACCOUNT_CATEGORY_CREDIT_CARD, Balance: -32704_35, Name: "HDFC 4708", Extend: &models.AccountExtend{
			CreditCardStatementDate: &statementDay, CreditCardPaymentDueDay: &dueDay, CreditCardPaymentAccountId: &payFrom,
		}},
		{AccountId: 2922, Category: models.ACCOUNT_CATEGORY_CREDIT_CARD, Balance: -1000_00, Name: "ICICI 2922"},
		{AccountId: 1, Category: models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT, Balance: 100},
		{AccountId: 99, Category: models.ACCOUNT_CATEGORY_CREDIT_CARD, Balance: -50, Hidden: true},
	})

	assert.Equal(t, []forecast.Card{
		{AccountId: 4708, PayFromAccountId: 7731, StatementDay: 14, DueDay: 3, Outstanding: 32704_35, Label: "HDFC 4708"},
		{AccountId: 2922, Outstanding: 1000_00, Label: "ICICI 2922"},
	}, got)
}

func TestForecastToIncome_UnpaidInvoiceDueDateAndOutstandingAmount(t *testing.T) {
	invoices := []*models.Invoice{
		{InvoiceId: 1, CustomerId: 10, Number: "INV-1", Total: 100000, DueDate: "2026-11-20", InvoiceDate: "2026-11-01"},
	}
	payments := map[int64][]*models.InvoicePaymentInfo{
		1: {{Transaction: &models.Transaction{AccountId: 50, Amount: 20000}}},
	}

	got := toIncome(invoices, payments, nil, nil, 777, forecast.D(2026, 12, 1))

	assert.Equal(t, []forecast.ExpectedIncome{
		{Date: forecast.D(2026, 11, 20), AccountId: 50, Amount: 80000, Source: forecast.SourceInvoice, SourceId: 1, Label: "INV-1"},
	}, got)
}

func TestForecastToIncome_WithinToleranceIsNotUnpaid(t *testing.T) {
	invoices := []*models.Invoice{
		{InvoiceId: 1, CustomerId: 10, Number: "INV-1", Total: 100000, DueDate: "2026-11-20", InvoiceDate: "2026-11-01"},
	}
	payments := map[int64][]*models.InvoicePaymentInfo{
		1: {{Transaction: &models.Transaction{AccountId: 50, Amount: 99000}}}, // 1% short; tolerance is 3%
	}

	assert.Empty(t, toIncome(invoices, payments, nil, nil, 777, forecast.D(2026, 12, 1)))
}

func TestForecastToIncome_ClosedManuallyExcluded(t *testing.T) {
	invoices := []*models.Invoice{
		{InvoiceId: 1, CustomerId: 10, Number: "INV-1", Total: 100000, DueDate: "2026-11-20", InvoiceDate: "2026-11-01", ClosedManually: true},
	}

	assert.Empty(t, toIncome(invoices, nil, nil, nil, 777, forecast.D(2026, 12, 1)))
}

func TestForecastToIncome_AccountFromLatestCustomerPaymentElseDefault(t *testing.T) {
	invoices := []*models.Invoice{
		{InvoiceId: 1, CustomerId: 10, Number: "INV-1", Total: 100000, DueDate: "2026-11-05", InvoiceDate: "2026-10-05"},
		{InvoiceId: 2, CustomerId: 10, Number: "INV-2", Total: 50000, DueDate: "2026-11-10", InvoiceDate: "2026-10-10"},
		{InvoiceId: 3, CustomerId: 20, Number: "INV-3", Total: 60000, DueDate: "2026-11-15", InvoiceDate: "2026-10-15"},
	}
	payments := map[int64][]*models.InvoicePaymentInfo{
		1: {{Transaction: &models.Transaction{AccountId: 60, TransactionTime: 1000}}},
		2: {{Transaction: &models.Transaction{AccountId: 70, TransactionTime: 2000}}}, // more recent than invoice 1's payment
	}

	got := toIncome(invoices, payments, nil, nil, 999, forecast.D(2026, 12, 1))

	byInvoice := map[int64]int64{}
	for _, inc := range got {
		byInvoice[inc.SourceId] = inc.AccountId
	}

	assert.Equal(t, int64(70), byInvoice[1]) // customer 10's latest payment account, even though it paid invoice 2
	assert.Equal(t, int64(70), byInvoice[2])
	assert.Equal(t, int64(999), byInvoice[3]) // customer 20 has no payment, falls back to default
}

func TestForecastToIncome_RecurringOccurrencesWithinHorizon(t *testing.T) {
	source := &models.Invoice{InvoiceId: 5, CustomerId: 20, Number: "INV-5", Total: 370000_00, InvoiceDate: "2026-10-01", DueDate: "2026-10-28"}
	recurring := []*models.RecurringInvoice{
		{RecurringId: 9, SourceInvoiceId: 5, Enabled: true, DayOfMonth: 1, NextDate: "2026-11-01", PeriodMonths: 1},
	}
	sources := map[int64]*models.Invoice{9: source}

	got := toIncome(nil, nil, recurring, sources, 777, forecast.D(2027, 1, 1))

	assert.Equal(t, []forecast.ExpectedIncome{
		{Date: forecast.D(2026, 11, 28), AccountId: 777, Amount: 370000_00, Source: forecast.SourceRecurringInvoice, SourceId: 9, Label: "INV-5"},
		{Date: forecast.D(2026, 12, 28), AccountId: 777, Amount: 370000_00, Source: forecast.SourceRecurringInvoice, SourceId: 9, Label: "INV-5"},
	}, got)
}

func TestForecastToIncome_DisabledOrSourcelessRecurringSkipped(t *testing.T) {
	recurring := []*models.RecurringInvoice{
		{RecurringId: 1, SourceInvoiceId: 5, Enabled: false, DayOfMonth: 1, NextDate: "2026-11-01", PeriodMonths: 1},
		{RecurringId: 2, SourceInvoiceId: 6, Enabled: true, DayOfMonth: 1, NextDate: "2026-11-01", PeriodMonths: 1}, // no source (deleted)
	}

	assert.Empty(t, toIncome(nil, nil, recurring, map[int64]*models.Invoice{}, 777, forecast.D(2027, 1, 1)))
}

func TestForecastDefaultIncomeAccount_MostBusinessIncomeWinsWithinLookback(t *testing.T) {
	history := []forecast.Txn{
		{Date: forecast.D(2026, 9, 1), Kind: forecast.KindIncome, AccountId: 1, Amount: 500, Business: true},
		{Date: forecast.D(2026, 9, 1), Kind: forecast.KindIncome, AccountId: 2, Amount: 900, Business: true},
		{Date: forecast.D(2026, 9, 1), Kind: forecast.KindIncome, AccountId: 2, Amount: 100000, Business: false}, // not business, ignored
		{Date: forecast.D(2026, 1, 1), Kind: forecast.KindIncome, AccountId: 3, Amount: 100000, Business: true},  // older than 6 months, ignored
	}

	assert.Equal(t, int64(2), defaultIncomeAccount(history, forecast.D(2026, 11, 2)))
}

func TestForecastDefaultIncomeAccount_NoHistoryReturnsZero(t *testing.T) {
	assert.Equal(t, int64(0), defaultIncomeAccount(nil, forecast.D(2026, 11, 2)))
}

func TestForecastFilterTemplates_SkipsDeletedOrHiddenAccountsAndCategories(t *testing.T) {
	ctx := core.NewNullContext()
	accountById := map[int64]*models.Account{
		1: {AccountId: 1},
		2: {AccountId: 2, Hidden: true},
	}
	categoryById := map[int64]*models.TransactionCategory{9: {CategoryId: 9}}
	tpls := []*models.TransactionTemplate{
		{TemplateId: 1, AccountId: 1, CategoryId: 9, Type: models.TRANSACTION_TYPE_EXPENSE},                       // kept
		{TemplateId: 2, AccountId: 2, CategoryId: 9, Type: models.TRANSACTION_TYPE_EXPENSE},                       // hidden account
		{TemplateId: 3, AccountId: 1, CategoryId: 99, Type: models.TRANSACTION_TYPE_EXPENSE},                      // missing category
		{TemplateId: 4, AccountId: 1, RelatedAccountId: 2, CategoryId: 9, Type: models.TRANSACTION_TYPE_TRANSFER}, // hidden destination
		{TemplateId: 5, AccountId: 3, CategoryId: 9, Type: models.TRANSACTION_TYPE_EXPENSE},                       // missing account
		{TemplateId: 6, AccountId: 1, RelatedAccountId: 1, CategoryId: 9, Type: models.TRANSACTION_TYPE_TRANSFER}, // kept: valid transfer
	}

	got := Forecasts.filterTemplates(ctx, 1, tpls, accountById, categoryById)

	ids := make([]int64, len(got))
	for i, tpl := range got {
		ids[i] = tpl.TemplateId
	}

	assert.Equal(t, []int64{1, 6}, ids)
}

func TestForecastToTemplates_KindBusinessAndLabel(t *testing.T) {
	start := int64(1793577600) // 2026-11-02T00:00:00Z
	tpls := []*models.TransactionTemplate{
		{TemplateId: 1, Type: models.TRANSACTION_TYPE_EXPENSE, AccountId: 7731, CategoryId: 9, Amount: 1800000,
			TagIds: "42", Name: "Nikhil", ScheduledFrequencyType: models.TRANSACTION_SCHEDULE_FREQUENCY_TYPE_MONTHLY,
			ScheduledFrequency: "3", ScheduledStartTime: &start, ScheduledTimezoneUtcOffset: 330},
		{TemplateId: 2, Type: models.TRANSACTION_TYPE_MODIFY_BALANCE, AccountId: 1, CategoryId: 1}, // unsupported kind, dropped
	}

	got := toTemplates(tpls, 42)

	assert.Equal(t, []forecast.Template{
		{Id: 1, AccountId: 7731, CategoryId: 9, Kind: forecast.KindExpense, Amount: 1800000, Business: true, Label: "Nikhil",
			FrequencyType: forecast.FreqMonthly, Frequency: "3", Start: forecast.D(2026, 11, 2)},
	}, got)
}

func TestForecastToHistory_CardRepaymentAndTransferInDropped(t *testing.T) {
	txns := []*models.Transaction{
		{TransactionId: 1, Type: models.TRANSACTION_DB_TYPE_TRANSFER_OUT, AccountId: 1, RelatedAccountId: 2, CategoryId: 5, Amount: 1000, TransactionTime: 1793577600000},
		{TransactionId: 2, Type: models.TRANSACTION_DB_TYPE_TRANSFER_IN, AccountId: 2, RelatedAccountId: 1, CategoryId: 5, Amount: 1000, TransactionTime: 1793577600000},
		{TransactionId: 3, Type: models.TRANSACTION_DB_TYPE_EXPENSE, AccountId: 1, CategoryId: 6, Amount: 500, TransactionTime: 1793577600000, Comment: "UPI-JOHN DOE-abc@upi"},
	}

	got := toHistory(txns, nil, 0, 5)

	assert.Len(t, got, 2) // the TRANSFER_IN row is dropped
	assert.Equal(t, forecast.KindTransfer, got[0].Kind)
	assert.True(t, got[0].CardRepayment) // category 5 is the repayment category
	assert.Equal(t, "JOHN DOE", got[1].PayeeKey)
}
