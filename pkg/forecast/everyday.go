package forecast

import (
	"sort"
	"time"
)

// Budget is a monthly budget scoped to exactly one expense category
type Budget struct {
	CategoryId int64
	Amount     int64
}

const oneOffFactor = 3

// Everyday estimates non-template spending for month, as one item per (account, business) pair
func Everyday(history []Txn, templates []Template, budgets []Budget, today, month time.Time) []Item {
	histStart := MonthStart(today).AddDate(0, -3, 0)
	histEnd := MonthStart(today)

	type key struct {
		acct     int64
		business bool
	}
	perCat := map[int64][]Txn{}
	spentThisMonth := map[int64]int64{}
	for _, tx := range history {
		if tx.Kind != KindExpense || coveredByTemplate(tx, templates) {
			continue
		}
		if !tx.Date.Before(histStart) && tx.Date.Before(histEnd) {
			perCat[tx.CategoryId] = append(perCat[tx.CategoryId], tx)
		} else if MonthStart(tx.Date).Equal(MonthStart(today)) {
			spentThisMonth[tx.CategoryId] += tx.Amount
		}
	}
	budgetOf := map[int64]int64{}
	for _, b := range budgets {
		budgetOf[b.CategoryId] = b.Amount
	}

	current := MonthStart(month).Equal(MonthStart(today))
	totals := map[key]int64{}
	for cat, txs := range perCat {
		txs = withoutOneOffs(txs)
		var total int64
		share := map[key]int64{}
		for _, tx := range txs {
			total += tx.Amount
			share[key{tx.AccountId, tx.Business}] += tx.Amount
		}
		if total == 0 {
			continue
		}
		monthly := total / 3
		if b, ok := budgetOf[cat]; ok {
			monthly = b
		}
		if current {
			monthly -= spentThisMonth[cat]
		}
		if monthly <= 0 {
			continue
		}
		for k, s := range share {
			totals[k] += monthly * s / total
		}
	}
	for cat, b := range budgetOf { // budgets for categories with no history
		if _, ok := perCat[cat]; !ok && b > 0 {
			rest := b
			if current {
				rest -= spentThisMonth[cat]
			}
			if rest > 0 {
				totals[key{0, false}] += rest // ponytail: no account history → account 0, shown as "unassigned"
			}
		}
	}

	date := MonthStart(month)
	if current {
		date = today
	}
	var items []Item
	for k, amt := range totals {
		if amt > 0 {
			items = append(items, Item{Date: date, Kind: KindExpense, AccountId: k.acct, Amount: amt, Source: SourceEveryday, Business: k.business, Label: "Everyday spending"})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].AccountId < items[j].AccountId })
	return items
}

func coveredByTemplate(tx Txn, templates []Template) bool {
	for _, t := range templates {
		if t.Kind == tx.Kind && t.AccountId == tx.AccountId && t.CategoryId == tx.CategoryId && within(tx.Amount, t.Amount) {
			return true
		}
	}
	return false
}

func withoutOneOffs(txs []Txn) []Txn {
	amounts := make([]int64, len(txs))
	for i, tx := range txs {
		amounts[i] = tx.Amount
	}
	sort.Slice(amounts, func(i, j int) bool { return amounts[i] < amounts[j] })
	median := amounts[len(amounts)/2]
	var kept []Txn
	for _, tx := range txs {
		if len(txs) < 3 || tx.Amount < oneOffFactor*median {
			kept = append(kept, tx)
		}
	}
	return kept
}
