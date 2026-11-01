package forecast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func salary(day int, amount int64) Item {
	return Item{Date: D(2026, 11, day), Kind: KindExpense, AccountId: 7731, CategoryId: 9, Amount: amount, Source: SourceTemplate}
}

func TestDropFulfilled_WithinTwentyPercent(t *testing.T) {
	items := []Item{salary(3, 18000_00)}
	actual := []Txn{{Id: 1, Date: D(2026, 11, 4), Kind: KindExpense, AccountId: 7731, CategoryId: 9, Amount: 20000_00}}
	assert.Empty(t, DropFulfilled(items, actual))
}

func TestDropFulfilled_OutsideTolerance_DifferentMonth_DifferentAccount(t *testing.T) {
	items := []Item{salary(3, 18000_00)}
	assert.Len(t, DropFulfilled(items, []Txn{{Date: D(2026, 11, 4), Kind: KindExpense, AccountId: 7731, CategoryId: 9, Amount: 25000_00}}), 1)
	assert.Len(t, DropFulfilled(items, []Txn{{Date: D(2026, 10, 30), Kind: KindExpense, AccountId: 7731, CategoryId: 9, Amount: 18000_00}}), 1)
	assert.Len(t, DropFulfilled(items, []Txn{{Date: D(2026, 11, 4), Kind: KindExpense, AccountId: 9260, CategoryId: 9, Amount: 18000_00}}), 1)
}

func TestDropFulfilled_OneTransactionFulfilsOneItem(t *testing.T) {
	items := []Item{salary(3, 15000_00), salary(3, 15000_00)}
	actual := []Txn{{Id: 1, Date: D(2026, 11, 3), Kind: KindExpense, AccountId: 7731, CategoryId: 9, Amount: 15000_00}}
	assert.Len(t, DropFulfilled(items, actual), 1)
}

func TestDropFulfilled_NonTemplateItemsKept(t *testing.T) {
	it := salary(3, 100)
	it.Source = SourceEveryday
	assert.Len(t, DropFulfilled([]Item{it}, []Txn{{Date: D(2026, 11, 3), Kind: KindExpense, AccountId: 7731, CategoryId: 9, Amount: 100}}), 1)
}
