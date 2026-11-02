// Package forecast turns planned items, invoices, budgets, history and card cycles into a dated cash forecast.
// It is pure: callers map database rows into these structs.
package forecast

import "time"

type Kind byte

const (
	KindExpense  Kind = 1
	KindIncome   Kind = 2
	KindTransfer Kind = 3
)

type Source string

const (
	SourceTemplate         Source = "template"
	SourceInvoice          Source = "invoice"
	SourceRecurringInvoice Source = "recurring_invoice"
	SourceEveryday         Source = "everyday"
	SourceCardBill         Source = "card_bill"
)

// Frequency types mirror models.TransactionScheduleFrequencyType values
const (
	FreqWeekly    byte = 1
	FreqMonthly   byte = 2
	FreqDaily     byte = 3
	FreqYearly    byte = 4
	FreqEveryNDay byte = 5
)

// Template is a forecast-only transaction template
type Template struct {
	Id, AccountId, DestinationAccountId, CategoryId int64
	Kind                                            Kind
	Amount                                          int64
	Business                                        bool
	Label                                           string
	FrequencyType                                   byte
	Frequency                                       string    // as stored: "5" or "1,15" or "-1"
	Start, End                                      time.Time // zero = unbounded
}

// Txn is a real transaction (transfers: AccountId → DestinationAccountId)
type Txn struct {
	Id                                          int64
	Date                                        time.Time
	Kind                                        Kind
	AccountId, DestinationAccountId, CategoryId int64
	Amount                                      int64
	Business                                    bool
	PayeeKey                                    string
	CardRepayment                               bool // transfer category is Credit Card Repayment
}

// Item is one expected money movement
type Item struct {
	Date                                        time.Time
	Kind                                        Kind
	AccountId, DestinationAccountId, CategoryId int64
	Amount                                      int64
	Source                                      Source
	SourceId                                    int64
	Uncertain, Business                         bool
	Label                                       string
	spread                                      bool // a per-day slice made by SpreadEveryday
}

func D(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }

func MonthStart(t time.Time) time.Time { return D(t.Year(), t.Month(), 1) }

func NextMonth(t time.Time) time.Time { return MonthStart(t).AddDate(0, 1, 0) }

func daysIn(t time.Time) int { return NextMonth(t).AddDate(0, 0, -1).Day() }
