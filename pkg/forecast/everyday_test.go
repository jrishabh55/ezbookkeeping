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
