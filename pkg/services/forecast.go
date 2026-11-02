package services

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/alerts"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/forecast"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// pageCountForForecastHistory is the batch size used to page through history transactions;
// GetAllSpecifiedTransactions requires a page count of at least 1, so 0 cannot mean "all"
const pageCountForForecastHistory = 1000

// forecastHistoryMonths is how many full months of history feed everyday spending and matching
const forecastHistoryMonths = 3

// forecastBusinessIncomeLookbackMonths is how far back the default invoice income account looks
const forecastBusinessIncomeLookbackMonths = 6

// businessTagName is the user-created tag that marks an item as business
const businessTagName = "Business"

// categoryNameBusinessIncome is the income category whose transactions count as business income
// for picking the default invoice income account (the Business tag is used on expenses, not income)
const categoryNameBusinessIncome = "Business Income"

// ForecastService represents forecast service
type ForecastService struct {
	ServiceUsingDB
}

// Initialize a forecast service singleton instance
var (
	Forecasts = &ForecastService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
	}
)

// GetForecast loads this user's accounts, forecast templates, history, budgets, cards and
// invoices, maps them into the pure forecast engine's input and returns its result
func (s *ForecastService) GetForecast(c core.Context, uid int64, today time.Time, months int) (*forecast.Result, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	accounts, err := Accounts.GetAllAccountsByUid(c, uid)

	if err != nil {
		return nil, err
	}

	templates, err := TransactionTemplates.GetAllTemplatesByUid(c, uid, models.TRANSACTION_TEMPLATE_TYPE_FORECAST)

	if err != nil {
		return nil, err
	}

	categories, err := TransactionCategories.GetAllCategoriesByUid(c, uid, 0, -1)

	if err != nil {
		return nil, err
	}

	tags, err := TransactionTags.GetAllTagsByUid(c, uid)

	if err != nil {
		return nil, err
	}

	budgets, scopesByBudget, err := s.loadBudgets(c, uid)

	if err != nil {
		return nil, err
	}

	invoices, payments, recurring, sources, err := s.loadInvoices(c, uid)

	if err != nil {
		return nil, err
	}

	accountById := make(map[int64]*models.Account, len(accounts))

	for _, a := range accounts {
		accountById[a.AccountId] = a
	}

	categoryById := make(map[int64]*models.TransactionCategory, len(categories))
	repaymentCategoryId := int64(0)
	businessIncomeCategoryId := int64(0)

	for _, cat := range categories {
		categoryById[cat.CategoryId] = cat

		if cat.Name == categoryNameCardRepayment {
			repaymentCategoryId = cat.CategoryId
		}

		if cat.Name == categoryNameBusinessIncome {
			businessIncomeCategoryId = cat.CategoryId
		}
	}

	businessTagId := findTagIdByName(tags, businessTagName)

	history, err := s.loadHistory(c, uid, forecast.MonthStart(today).AddDate(0, -forecastHistoryMonths, 0), today, businessTagId, repaymentCategoryId)

	if err != nil {
		return nil, err
	}

	defaultIncomeHistory, err := s.loadIncomeHistory(c, uid, forecast.MonthStart(today).AddDate(0, -forecastBusinessIncomeLookbackMonths, 0), today, businessTagId)

	if err != nil {
		return nil, err
	}

	horizonStart := forecast.MonthStart(today)
	horizonEnd := horizonStart.AddDate(0, months, 0)
	validTemplates := s.filterTemplates(c, uid, templates, accountById, categoryById)
	defaultAccount := defaultIncomeAccount(defaultIncomeHistory, today, businessIncomeCategoryId)

	input := forecast.Input{
		Today:     today,
		Months:    months,
		Accounts:  toAccounts(accounts),
		Templates: toTemplates(validTemplates, businessTagId),
		Income:    toIncome(c, uid, invoices, payments, recurring, sources, defaultAccount, horizonEnd),
		History:   history,
		Budgets:   toBudgets(budgets, scopesByBudget, horizonStart, horizonEnd),
		Cards:     toCards(accounts),
	}

	result := forecast.Build(input)
	return &result, nil
}

// loadBudgets returns all budgets of the user and their scopes keyed by budget id
func (s *ForecastService) loadBudgets(c core.Context, uid int64) ([]*models.Budget, map[int64][]*models.BudgetScope, error) {
	budgets, err := Budgets.GetAllBudgetsByUid(c, uid)

	if err != nil {
		return nil, nil, err
	}

	scopesByBudget := make(map[int64][]*models.BudgetScope, len(budgets))

	for _, b := range budgets {
		scopes, err := Budgets.GetBudgetScopesByBudgetId(c, uid, b.BudgetId)

		if err != nil {
			return nil, nil, err
		}

		scopesByBudget[b.BudgetId] = scopes
	}

	return budgets, scopesByBudget, nil
}

// loadInvoices returns all invoices and their payments, all recurring invoices and, for each
// enabled recurring invoice whose source invoice still exists, that source invoice keyed by
// recurring invoice id; a recurring invoice whose source was deleted is skipped and logged
func (s *ForecastService) loadInvoices(c core.Context, uid int64) ([]*models.Invoice, map[int64][]*models.InvoicePaymentInfo, []*models.RecurringInvoice, map[int64]*models.Invoice, error) {
	invoices, err := Invoices.GetAllInvoices(c, uid)

	if err != nil {
		return nil, nil, nil, nil, err
	}

	invoiceIds := make([]int64, len(invoices))

	for i, inv := range invoices {
		invoiceIds[i] = inv.InvoiceId
	}

	payments, err := Invoices.GetInvoicePayments(c, uid, invoiceIds)

	if err != nil {
		return nil, nil, nil, nil, err
	}

	recurring, err := Invoices.GetAllRecurringInvoices(c, uid)

	if err != nil {
		return nil, nil, nil, nil, err
	}

	sources := make(map[int64]*models.Invoice, len(recurring))

	for _, r := range recurring {
		if !r.Enabled {
			continue
		}

		source, _, err := Invoices.GetInvoice(c, uid, r.SourceInvoiceId)

		if err == errs.ErrInvoiceNotFound {
			log.Warnf(c, "[forecast.loadInvoices] source invoice \"id:%d\" of recurring invoice \"id:%d\" of user \"uid:%d\" is deleted, skipped", r.SourceInvoiceId, r.RecurringId, uid)
			continue
		} else if err != nil {
			return nil, nil, nil, nil, err
		}

		sources[r.RecurringId] = source
	}

	return invoices, payments, recurring, sources, nil
}

// loadHistory returns forecast history transactions whose local date falls in [from, to]
func (s *ForecastService) loadHistory(c core.Context, uid int64, from time.Time, to time.Time, businessTagId int64, repaymentCategoryId int64) ([]forecast.Txn, error) {
	txns, err := s.loadTransactions(c, uid, from, to, 0)

	if err != nil {
		return nil, err
	}

	tagIdsByTxn, err := s.tagIdsOfTransactions(c, uid, txns)

	if err != nil {
		return nil, err
	}

	return clipToDateRange(toHistory(txns, tagIdsByTxn, businessTagId, repaymentCategoryId), from, to), nil
}

// loadIncomeHistory returns income-only forecast history, used to find the default invoice income account
func (s *ForecastService) loadIncomeHistory(c core.Context, uid int64, from time.Time, to time.Time, businessTagId int64) ([]forecast.Txn, error) {
	txns, err := s.loadTransactions(c, uid, from, to, models.TRANSACTION_TYPE_INCOME)

	if err != nil {
		return nil, err
	}

	tagIdsByTxn, err := s.tagIdsOfTransactions(c, uid, txns)

	if err != nil {
		return nil, err
	}

	return clipToDateRange(toHistory(txns, tagIdsByTxn, businessTagId, 0), from, to), nil
}

// loadTransactions pages through GetAllSpecifiedTransactions for the given local-date range and type
// (0 = any). The query window is widened by a day on each side because [from, to] are UTC-midnight
// calendar dates: a client ahead of UTC can have local-date `from` transactions stored before UTC
// midnight of `from`, and a client behind UTC can have local-date `to` transactions stored after UTC
// midnight of `to+1`. clipToDateRange (applied by the caller, after converting to local dates) trims
// the over-fetch back down to the real [from, to] window.
func (s *ForecastService) loadTransactions(c core.Context, uid int64, from time.Time, to time.Time, transactionType models.TransactionType) ([]*models.Transaction, error) {
	minTime := utils.GetMinTransactionTimeFromUnixTime(from.AddDate(0, 0, -1).Unix())
	maxTime := utils.GetMaxTransactionTimeFromUnixTime(to.AddDate(0, 0, 1).Unix())

	return Transactions.GetAllSpecifiedTransactions(c, uid, maxTime, minTime, transactionType, nil, nil, nil, false, "", "", core.MATCH_MODE_DEFAULT, false, pageCountForForecastHistory, true)
}

// clipToDateRange keeps only the history items whose local date falls within [from, to], inclusive
func clipToDateRange(txns []forecast.Txn, from time.Time, to time.Time) []forecast.Txn {
	var out []forecast.Txn

	for _, tx := range txns {
		if !tx.Date.Before(from) && !tx.Date.After(to) {
			out = append(out, tx)
		}
	}

	return out
}

func (s *ForecastService) tagIdsOfTransactions(c core.Context, uid int64, txns []*models.Transaction) (map[int64][]int64, error) {
	transactionIds := make([]int64, len(txns))

	for i, tx := range txns {
		transactionIds[i] = tx.TransactionId
	}

	return TransactionTags.GetAllTagIdsOfTransactions(c, uid, transactionIds)
}

// filterTemplates drops forecast templates whose account, destination account or category no
// longer exists, or whose account (or destination account) is hidden; each drop is logged
func (s *ForecastService) filterTemplates(c core.Context, uid int64, tpls []*models.TransactionTemplate, accountById map[int64]*models.Account, categoryById map[int64]*models.TransactionCategory) []*models.TransactionTemplate {
	var out []*models.TransactionTemplate

	for _, t := range tpls {
		if acct, ok := accountById[t.AccountId]; !ok || acct.Hidden {
			log.Warnf(c, "[forecast.filterTemplates] forecast template \"id:%d\" of user \"uid:%d\" has a missing or hidden account, skipped", t.TemplateId, uid)
			continue
		}

		if t.Type == models.TRANSACTION_TYPE_TRANSFER {
			if dest, ok := accountById[t.RelatedAccountId]; !ok || dest.Hidden {
				log.Warnf(c, "[forecast.filterTemplates] forecast template \"id:%d\" of user \"uid:%d\" has a missing or hidden destination account, skipped", t.TemplateId, uid)
				continue
			}
		}

		if _, ok := categoryById[t.CategoryId]; !ok {
			log.Warnf(c, "[forecast.filterTemplates] forecast template \"id:%d\" of user \"uid:%d\" has a missing category, skipped", t.TemplateId, uid)
			continue
		}

		out = append(out, t)
	}

	return out
}

// findTagIdByName returns the id of the user's non-hidden tag with the given name (case-insensitive), or 0
func findTagIdByName(tags []*models.TransactionTag, name string) int64 {
	for _, t := range tags {
		if !t.Hidden && strings.EqualFold(t.Name, name) {
			return t.TagId
		}
	}

	return 0
}

// localDate converts a unix time and a utc offset in minutes to the local calendar date
func localDate(unixTime int64, utcOffsetMinutes int16) time.Time {
	loc := time.FixedZone("", int(utcOffsetMinutes)*60)
	t := time.Unix(unixTime, 0).In(loc)
	return forecast.D(t.Year(), t.Month(), t.Day())
}

// toForecastKind maps a transaction/template type to the engine's Kind, ok is false when there is none
func toForecastKind(t models.TransactionType) (forecast.Kind, bool) {
	switch t {
	case models.TRANSACTION_TYPE_EXPENSE:
		return forecast.KindExpense, true
	case models.TRANSACTION_TYPE_INCOME:
		return forecast.KindIncome, true
	case models.TRANSACTION_TYPE_TRANSFER:
		return forecast.KindTransfer, true
	default:
		return 0, false
	}
}

// historyKind maps a stored transaction's db type to the engine's Kind, ok is false when there is none
func historyKind(t models.TransactionDbType) (forecast.Kind, bool) {
	switch t {
	case models.TRANSACTION_DB_TYPE_EXPENSE:
		return forecast.KindExpense, true
	case models.TRANSACTION_DB_TYPE_INCOME:
		return forecast.KindIncome, true
	case models.TRANSACTION_DB_TYPE_TRANSFER_OUT:
		return forecast.KindTransfer, true
	default:
		return 0, false
	}
}

// toAccounts maps non-hidden accounts into the engine's accounts; a liability's balance (stored
// negative when owed) is negated so the engine sees how much is owed as a positive number
func toAccounts(accts []*models.Account) []forecast.Account {
	var out []forecast.Account

	for _, a := range accts {
		if a.Hidden {
			continue
		}

		fa := forecast.Account{Id: a.AccountId, Balance: a.Balance, Name: a.Name}

		switch a.Category {
		case models.ACCOUNT_CATEGORY_CASH, models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT, models.ACCOUNT_CATEGORY_SAVINGS_ACCOUNT, models.ACCOUNT_CATEGORY_VIRTUAL:
			fa.Cash = true
		}

		if a.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD || a.Category == models.ACCOUNT_CATEGORY_DEBT {
			fa.Liability = true
			fa.Balance = -a.Balance
		}

		out = append(out, fa)
	}

	return out
}

// toTemplates maps forecast-only transaction templates into the engine's templates
func toTemplates(tpls []*models.TransactionTemplate, businessTagId int64) []forecast.Template {
	var out []forecast.Template

	for _, t := range tpls {
		kind, ok := toForecastKind(t.Type)

		if !ok {
			continue
		}

		ft := forecast.Template{
			Id:                   t.TemplateId,
			AccountId:            t.AccountId,
			DestinationAccountId: t.RelatedAccountId,
			CategoryId:           t.CategoryId,
			Kind:                 kind,
			Amount:               t.Amount,
			Business:             businessTagId > 0 && slices.Contains(t.GetTagIds(), businessTagId),
			Label:                t.Name,
			FrequencyType:        byte(t.ScheduledFrequencyType),
			Frequency:            t.ScheduledFrequency,
		}

		if t.ScheduledStartTime != nil {
			ft.Start = localDate(*t.ScheduledStartTime, t.ScheduledTimezoneUtcOffset)
		}

		if t.ScheduledEndTime != nil {
			ft.End = localDate(*t.ScheduledEndTime, t.ScheduledTimezoneUtcOffset)
		}

		out = append(out, ft)
	}

	return out
}

// toHistory maps real transactions into the engine's history; only outgoing transfers are kept,
// since the matching incoming leg carries no independent information for the forecast
func toHistory(txns []*models.Transaction, tagIdsByTxn map[int64][]int64, businessTagId int64, repaymentCategoryId int64) []forecast.Txn {
	var out []forecast.Txn

	for _, tx := range txns {
		kind, ok := historyKind(tx.Type)

		if !ok {
			continue
		}

		htx := forecast.Txn{
			Id:                   tx.TransactionId,
			Date:                 localDate(utils.GetUnixTimeFromTransactionTime(tx.TransactionTime), tx.TimezoneUtcOffset),
			Kind:                 kind,
			AccountId:            tx.AccountId,
			DestinationAccountId: tx.RelatedAccountId,
			CategoryId:           tx.CategoryId,
			Amount:               tx.Amount,
			Business:             businessTagId > 0 && slices.Contains(tagIdsByTxn[tx.TransactionId], businessTagId),
			PayeeKey:             alerts.PayeeKey(tx.Comment),
		}

		if kind == forecast.KindTransfer {
			htx.CardRepayment = repaymentCategoryId > 0 && tx.CategoryId == repaymentCategoryId
		}

		out = append(out, htx)
	}

	return out
}

// toBudgets keeps visible, monthly budgets scoped to exactly one expense category that still cover
// the forecast horizon [horizonStart, horizonEnd); an auto-renewing budget always covers it, since
// its stored window is only its first cycle
func toBudgets(budgets []*models.Budget, scopesByBudget map[int64][]*models.BudgetScope, horizonStart time.Time, horizonEnd time.Time) []forecast.Budget {
	var out []forecast.Budget
	horizonStartUnix, horizonEndUnix := horizonStart.Unix(), horizonEnd.Unix()

	for _, b := range budgets {
		if b.PeriodType != models.BUDGET_PERIOD_TYPE_MONTHLY || !b.Visible {
			continue
		}

		if !b.AutoCreateNextCycle && (b.EndTime < horizonStartUnix || b.StartTime >= horizonEndUnix) {
			continue
		}

		scopes := scopesByBudget[b.BudgetId]

		if len(scopes) != 1 || scopes[0].ScopeType != models.BUDGET_SCOPE_TYPE_CATEGORY {
			continue
		}

		out = append(out, forecast.Budget{CategoryId: scopes[0].ScopeItemId, Amount: b.Amount})
	}

	return out
}

// toCards maps non-hidden credit card accounts into the engine's cards; a due day, statement day
// or pay-from account left unset becomes 0, which the engine treats as "skip this card"
func toCards(accts []*models.Account) []forecast.Card {
	var out []forecast.Card

	for _, a := range accts {
		if a.Hidden || a.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD {
			continue
		}

		card := forecast.Card{AccountId: a.AccountId, Outstanding: -a.Balance, Label: a.Name}

		if a.Extend != nil {
			if a.Extend.CreditCardStatementDate != nil {
				card.StatementDay = *a.Extend.CreditCardStatementDate
			}

			if a.Extend.CreditCardPaymentDueDay != nil {
				card.DueDay = *a.Extend.CreditCardPaymentDueDay
			}

			if a.Extend.CreditCardPaymentAccountId != nil {
				card.PayFromAccountId = *a.Extend.CreditCardPaymentAccountId
			}
		}

		out = append(out, card)
	}

	return out
}

// toIncome maps unpaid invoices and enabled recurring invoices' future occurrences into expected
// income; sources maps a recurring invoice id to its (still existing) source invoice. The account
// is the account of the most recent payment linked to any invoice of that customer, else
// defaultAccount; an item with neither is dropped (never emitted with account id 0) and logged
func toIncome(c core.Context, uid int64, invoices []*models.Invoice, payments map[int64][]*models.InvoicePaymentInfo, recurring []*models.RecurringInvoice, sources map[int64]*models.Invoice, defaultAccount int64, horizonEnd time.Time) []forecast.ExpectedIncome {
	accountByCustomer := latestPaymentAccountByCustomer(invoices, payments)
	accountFor := func(customerId int64) (int64, bool) {
		if acct, ok := accountByCustomer[customerId]; ok {
			return acct, true
		}

		return defaultAccount, defaultAccount > 0
	}

	var out []forecast.ExpectedIncome
	out = append(out, unpaidInvoiceIncome(c, uid, invoices, payments, accountFor)...)

	for _, r := range recurring {
		source, ok := sources[r.RecurringId]

		if !r.Enabled || !ok {
			continue
		}

		accountId, ok := accountFor(source.CustomerId)

		if !ok {
			log.Warnf(c, "[forecast.toIncome] recurring invoice \"id:%d\" of user \"uid:%d\" has no payment history and no default income account, dropped", r.RecurringId, uid)
			continue
		}

		out = append(out, recurringOccurrences(r, source, horizonEnd, accountId)...)
	}

	return out
}

// unpaidInvoiceIncome returns one expected income item per unpaid invoice, on its due date, for the
// outstanding amount; an invoice whose customer has no payment history and no default account is dropped
func unpaidInvoiceIncome(c core.Context, uid int64, invoices []*models.Invoice, payments map[int64][]*models.InvoicePaymentInfo, accountFor func(customerId int64) (int64, bool)) []forecast.ExpectedIncome {
	var out []forecast.ExpectedIncome

	for _, inv := range invoices {
		balanceDue := inv.Total - models.InvoiceReceivedAmount(payments[inv.InvoiceId])

		if inv.ClosedManually || float64(balanceDue) <= float64(inv.Total)*models.InvoicePaidTolerance {
			continue
		}

		dueDate, err := time.Parse(invoiceDateLayout, inv.DueDate)

		if err != nil {
			continue
		}

		accountId, ok := accountFor(inv.CustomerId)

		if !ok {
			log.Warnf(c, "[forecast.unpaidInvoiceIncome] invoice \"id:%d\" of user \"uid:%d\" has no payment history and no default income account, dropped", inv.InvoiceId, uid)
			continue
		}

		out = append(out, forecast.ExpectedIncome{
			Date: forecast.D(dueDate.Year(), dueDate.Month(), dueDate.Day()), AccountId: accountId,
			Amount: balanceDue, Source: forecast.SourceInvoice, SourceId: inv.InvoiceId, Label: inv.Number,
		})
	}

	return out
}

// latestPaymentAccountByCustomer returns, for each customer, the account of their most recent linked payment
func latestPaymentAccountByCustomer(invoices []*models.Invoice, payments map[int64][]*models.InvoicePaymentInfo) map[int64]int64 {
	latestTime := map[int64]int64{}
	account := map[int64]int64{}

	for _, inv := range invoices {
		for _, p := range payments[inv.InvoiceId] {
			if p.Transaction == nil {
				continue
			}

			if t, ok := latestTime[inv.CustomerId]; !ok || p.Transaction.TransactionTime > t {
				latestTime[inv.CustomerId] = p.Transaction.TransactionTime
				account[inv.CustomerId] = p.Transaction.AccountId
			}
		}
	}

	return account
}

// recurringOccurrences expands one recurring invoice's future occurrences up to (not including) horizonEnd
func recurringOccurrences(r *models.RecurringInvoice, source *models.Invoice, horizonEnd time.Time, accountId int64) []forecast.ExpectedIncome {
	invoiceDate, err := time.Parse(invoiceDateLayout, source.InvoiceDate)

	if err != nil {
		return nil
	}

	dueDate, err := time.Parse(invoiceDateLayout, source.DueDate)

	if err != nil {
		return nil
	}

	offsetDays := int(dueDate.Sub(invoiceDate).Hours() / 24)
	period := max(r.PeriodMonths, 1)
	occurrence := r.NextDate

	var out []forecast.ExpectedIncome

	// a 1-3 month horizon and a monthly-or-slower period never need more than a few dozen occurrences;
	// the cap only guards against a malformed row advancing by less than it should
	for i := 0; i < 60; i++ {
		t, err := time.Parse(invoiceDateLayout, occurrence)

		if err != nil {
			break
		}

		occDate := forecast.D(t.Year(), t.Month(), t.Day())

		if !occDate.Before(horizonEnd) {
			break
		}

		out = append(out, forecast.ExpectedIncome{
			Date: occDate.AddDate(0, 0, offsetDays), AccountId: accountId, Amount: source.Total,
			Source: forecast.SourceRecurringInvoice, SourceId: r.RecurringId, Label: source.Number,
		})

		next, err := nextRecurringDate(occurrence, r.DayOfMonth, period)

		if err != nil {
			break
		}

		occurrence = next
	}

	return out
}

// defaultIncomeAccount returns the account with the most business income (category
// businessIncomeCategoryId, or the Business tag) in the lookback window; if there is none, the
// account with the most income of any kind in the same window; if there is no income at all, 0 (the
// caller must not use 0 as a destination account). Ties break on the lowest account id.
func defaultIncomeAccount(history []forecast.Txn, today time.Time, businessIncomeCategoryId int64) int64 {
	since := forecast.MonthStart(today).AddDate(0, -forecastBusinessIncomeLookbackMonths, 0)
	businessTotals := map[int64]int64{}
	anyTotals := map[int64]int64{}

	for _, tx := range history {
		if tx.Kind != forecast.KindIncome || tx.Date.Before(since) {
			continue
		}

		anyTotals[tx.AccountId] += tx.Amount

		if tx.Business || (businessIncomeCategoryId > 0 && tx.CategoryId == businessIncomeCategoryId) {
			businessTotals[tx.AccountId] += tx.Amount
		}
	}

	if best := bestAccountByTotal(businessTotals); best > 0 {
		return best
	}

	return bestAccountByTotal(anyTotals)
}

// bestAccountByTotal returns the account id with the highest total, breaking ties by the lowest id, or 0
func bestAccountByTotal(totals map[int64]int64) int64 {
	ids := make([]int64, 0, len(totals))

	for id := range totals {
		ids = append(ids, id)
	}

	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	best := int64(0)
	bestAmount := int64(0)

	for _, id := range ids {
		if totals[id] > bestAmount {
			bestAmount = totals[id]
			best = id
		}
	}

	return best
}
