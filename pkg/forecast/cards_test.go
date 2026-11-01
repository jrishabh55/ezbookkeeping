package forecast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var hdfc4708 = Card{AccountId: 4708, PayFromAccountId: 7731, StatementDay: 14, DueDay: 3, Outstanding: 32704_00, Label: "HDFC 4708"}

func cardSpend(day int, amt int64) Item {
	return Item{Date: D(2026, 11, day), Kind: KindExpense, AccountId: 4708, Amount: amt}
}

func TestCardBills_DueDayBeforeStatementDayIsNextMonth(t *testing.T) {
	bills, skipped := CardBills([]Card{hdfc4708}, []Item{cardSpend(10, 5000_00), cardSpend(20, 1000_00)}, nil, D(2026, 11, 2), D(2027, 1, 1))
	assert.Empty(t, skipped)
	assert.Len(t, bills, 1)
	assert.Equal(t, D(2026, 12, 3), bills[0].Date)
	assert.Equal(t, int64(37704_00), bills[0].Amount)
	assert.Equal(t, KindTransfer, bills[0].Kind)
	assert.Equal(t, int64(7731), bills[0].AccountId)
	assert.Equal(t, int64(4708), bills[0].DestinationAccountId)
}

func TestCardBills_SecondCycleOnlyNewSpend(t *testing.T) {
	bills, _ := CardBills([]Card{hdfc4708}, []Item{cardSpend(20, 1000_00)}, nil, D(2026, 11, 2), D(2027, 2, 1))
	assert.Len(t, bills, 2)
	assert.Equal(t, D(2027, 1, 3), bills[1].Date)
	assert.Equal(t, int64(1000_00), bills[1].Amount)
}

func TestCardBills_AlreadyPaidCycleNotBilledAgain(t *testing.T) {
	paid := []Txn{{Date: D(2026, 11, 28), Kind: KindTransfer, AccountId: 7731, DestinationAccountId: 4708, Amount: 34000_00, CardRepayment: true}}
	bills, _ := CardBills([]Card{hdfc4708}, []Item{cardSpend(10, 2000_00)}, paid, D(2026, 11, 2), D(2027, 1, 1))
	assert.Empty(t, bills)
}

func TestCardBills_MissingFieldsSkipped(t *testing.T) {
	c := hdfc4708
	c.DueDay = 0
	bills, skipped := CardBills([]Card{c}, nil, nil, D(2026, 11, 2), D(2027, 1, 1))
	assert.Empty(t, bills)
	assert.Equal(t, []int64{4708}, skipped)
}
