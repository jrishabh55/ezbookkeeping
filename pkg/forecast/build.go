package forecast

import (
	"sort"
	"time"
)

type ExpectedIncome struct {
	Date      time.Time
	AccountId int64
	Amount    int64
	Source    Source
	SourceId  int64
	Label     string
}

type Input struct {
	Today     time.Time
	Months    int
	Accounts  []Account
	Templates []Template
	Income    []ExpectedIncome
	History   []Txn
	Budgets   []Budget
	Cards     []Card
}

type Result struct {
	Months       []Month
	Items        []Item
	Shortfalls   []Shortfall
	SkippedCards []int64
}

// Build runs the whole forecast
func Build(in Input) Result {
	start := MonthStart(in.Today)
	end := start.AddDate(0, in.Months, 0)

	var planned []Item
	for _, t := range in.Templates {
		planned = append(planned, ExpandTemplate(t, start, end)...)
	}
	planned = DropFulfilled(planned, in.History)
	for i := range planned {
		if planned[i].Date.Before(in.Today) {
			planned[i].Date = in.Today
		}
	}

	for _, inc := range in.Income {
		d := inc.Date
		if d.Before(in.Today) {
			d = in.Today
		}
		if d.Before(end) {
			planned = append(planned, Item{Date: d, Kind: KindIncome, AccountId: inc.AccountId, Amount: inc.Amount,
				Source: inc.Source, SourceId: inc.SourceId, Uncertain: true, Business: true, Label: inc.Label})
		}
	}

	var expenses []Txn
	var repayments []Txn
	for _, tx := range in.History {
		if tx.Kind == KindExpense {
			expenses = append(expenses, tx)
		} else if tx.Kind == KindTransfer && tx.CardRepayment {
			repayments = append(repayments, tx)
		}
	}
	for m := start; m.Before(end); m = m.AddDate(0, 1, 0) {
		planned = append(planned, Everyday(expenses, in.Templates, in.Budgets, in.Today, m)...)
	}

	bills, skipped := CardBills(in.Cards, planned, repayments, in.Today, end)
	planned = append(planned, bills...)
	sort.SliceStable(planned, func(i, j int) bool { return planned[i].Date.Before(planned[j].Date) })

	months, shortfalls := Walk(in.Accounts, planned, in.Today, in.Months)
	return Result{Months: months, Items: planned, Shortfalls: shortfalls, SkippedCards: skipped}
}
