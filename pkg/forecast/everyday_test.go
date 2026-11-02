package forecast

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func exp(y, m, d int, acct, cat, amt int64) Txn {
	return Txn{Date: D(y, time.Month(m), d), Kind: KindExpense, AccountId: acct, CategoryId: cat, Amount: amt}
}

func sum(items []Item) (total int64) {
	for _, it := range items {
		total += it.Amount
	}
	return
}

func TestEveryday_ThreeMonthAverageSplitByAccount(t *testing.T) {
	// food: Aug 300 (card 1), Sep 300 (card 1), Oct 300 (bank 2) → avg 300; split 2/3 card, 1/3 bank
	h := []Txn{exp(2026, 8, 5, 1, 5, 300), exp(2026, 9, 5, 1, 5, 300), exp(2026, 10, 5, 2, 5, 300)}
	got := Everyday(h, nil, nil, D(2026, 11, 1), D(2026, 12, 1))
	assert.Equal(t, int64(300), sum(got))
	byAcct := map[int64]int64{}
	for _, it := range got {
		byAcct[it.AccountId] += it.Amount
		assert.Equal(t, SourceEveryday, it.Source)
		assert.Equal(t, D(2026, 12, 1), it.Date)
	}
	assert.Equal(t, int64(200), byAcct[1])
	assert.Equal(t, int64(100), byAcct[2])
}

func TestEveryday_BudgetWins(t *testing.T) {
	h := []Txn{exp(2026, 8, 5, 1, 5, 300), exp(2026, 9, 5, 1, 5, 300), exp(2026, 10, 5, 1, 5, 300)}
	got := Everyday(h, nil, []Budget{{CategoryId: 5, Amount: 1000}}, D(2026, 11, 1), D(2026, 12, 1))
	assert.Equal(t, int64(1000), sum(got))
}

func TestEveryday_ExcludesOneOffsAndTemplateMatches(t *testing.T) {
	h := []Txn{
		exp(2026, 8, 5, 1, 7, 1000), exp(2026, 9, 5, 1, 7, 1000), exp(2026, 10, 5, 1, 7, 1000), exp(2026, 10, 22, 1, 7, 35000), // phone one-off
		exp(2026, 8, 3, 2, 9, 18000), exp(2026, 9, 3, 2, 9, 18000), exp(2026, 10, 3, 2, 9, 18000), // salary covered by a template
	}
	tpl := []Template{{Kind: KindExpense, AccountId: 2, CategoryId: 9, Amount: 18000}}
	assert.Equal(t, int64(1000), sum(Everyday(h, tpl, nil, D(2026, 11, 1), D(2026, 12, 1))))
}

func TestEveryday_CurrentMonthOnlyRemainder(t *testing.T) {
	h := []Txn{exp(2026, 8, 5, 1, 5, 300), exp(2026, 9, 5, 1, 5, 300), exp(2026, 10, 5, 1, 5, 300), exp(2026, 11, 2, 1, 5, 250)}
	got := Everyday(h, nil, nil, D(2026, 11, 2), D(2026, 11, 1))
	assert.Equal(t, int64(50), sum(got))
	assert.Equal(t, D(2026, 11, 2), got[0].Date)
	h = append(h, exp(2026, 11, 2, 1, 5, 500))
	assert.Empty(t, Everyday(h, nil, nil, D(2026, 11, 2), D(2026, 11, 1)))
}

// A budget for a category with no history goes to the account with the most expense history
func TestEveryday_BudgetOnlyCategoryGoesToBusiestAccount(t *testing.T) {
	h := []Txn{exp(2026, 8, 5, 1, 5, 300), exp(2026, 9, 5, 2, 6, 900), exp(2026, 10, 5, 1, 5, 300)}
	got := Everyday(h, nil, []Budget{{CategoryId: 8, Amount: 1000}}, D(2026, 11, 1), D(2026, 12, 1))
	byAcct := map[int64]int64{}
	for _, it := range got {
		byAcct[it.AccountId] += it.Amount
	}
	assert.Equal(t, int64(1000+300), byAcct[2])
	assert.Equal(t, int64(200), byAcct[1])
	assert.Zero(t, byAcct[0])
}

// With no history at all a budget stays unassigned (account 0): listed and counted as cost, but no
// balance moves
func TestEveryday_BudgetWithoutAnyHistoryIsUnassigned(t *testing.T) {
	got := Everyday(nil, nil, []Budget{{CategoryId: 8, Amount: 1000}}, D(2026, 11, 1), D(2026, 12, 1))
	assert.Len(t, got, 1)
	assert.Equal(t, int64(0), got[0].AccountId)
	months, _ := Walk([]Account{{Id: 1, Balance: 500, Cash: true}}, got, D(2026, 11, 1), 2)
	assert.Equal(t, int64(500), months[1].ClosingCash)
	assert.Equal(t, int64(1000), months[1].PersonalCost)
}

func TestSpreadEveryday_ExactTotalsRemainderOnLastDay(t *testing.T) {
	items := []Item{{Date: D(2026, 11, 2), Kind: KindExpense, AccountId: 1, Amount: 1000, Source: SourceEveryday}, {Date: D(2026, 11, 5), Amount: 7}}
	got := SpreadEveryday(items)
	assert.Len(t, got, 29+1)
	assert.Equal(t, int64(1007), sum(got))
	assert.Equal(t, D(2026, 11, 2), got[0].Date)
	assert.Equal(t, int64(34), got[0].Amount)
	assert.Equal(t, D(2026, 11, 30), got[28].Date)
	assert.Equal(t, int64(1000-34*28), got[28].Amount)
	assert.Equal(t, got, SpreadEveryday(got)) // slices are not spread again
}
