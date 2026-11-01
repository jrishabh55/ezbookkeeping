package forecast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWalk_ShortfallOn9260BeforeEmi(t *testing.T) {
	accts := []Account{{Id: 9260, Balance: 16366_21, Cash: true}, {Id: 7731, Balance: 215060_45, Cash: true}}
	items := []Item{
		{Date: D(2026, 11, 5), Kind: KindTransfer, AccountId: 9260, DestinationAccountId: 999, Amount: 17512_00},
		{Date: D(2026, 11, 15), Kind: KindTransfer, AccountId: 9260, DestinationAccountId: 998, Amount: 10000_00},
	}
	months, sf := Walk(accts, items, D(2026, 11, 2), 1)
	assert.Len(t, sf, 1)
	assert.Equal(t, int64(9260), sf[0].AccountId)
	assert.Equal(t, D(2026, 11, 5), sf[0].Date)
	assert.Equal(t, D(2026, 11, 15), sf[0].LowestDate)
	assert.Equal(t, int64(16366_21-27512_00), sf[0].LowestBalance)
	assert.Equal(t, int64(231426_66), months[0].OpeningCash)
	assert.Equal(t, int64(231426_66-27512_00), months[0].ClosingCash)
}

func TestWalk_UncertainIncomeScenario(t *testing.T) {
	accts := []Account{{Id: 1, Balance: 100, Cash: true}}
	items := []Item{{Date: D(2026, 11, 28), Kind: KindIncome, AccountId: 1, Amount: 370000, Uncertain: true, Business: true}}
	months, _ := Walk(accts, items, D(2026, 11, 2), 2)
	assert.Equal(t, int64(370100), months[0].ClosingCash)
	assert.Equal(t, int64(100), months[0].ClosingCashWithoutUncertain)
	assert.Equal(t, int64(370100), months[1].OpeningCash)
	assert.Equal(t, int64(370000), months[0].BusinessIncome)
}

func TestWalk_AlreadyNegativeAccountNotAlerted(t *testing.T) {
	accts := []Account{{Id: 4071, Balance: -432_00, Cash: true}}
	items := []Item{{Date: D(2026, 11, 5), Kind: KindExpense, AccountId: 4071, Amount: 100}}
	_, sf := Walk(accts, items, D(2026, 11, 2), 1)
	assert.Empty(t, sf)
}

func TestWalk_CardSpendIsCostNotCash(t *testing.T) {
	accts := []Account{{Id: 1, Balance: 1000, Cash: true}, {Id: 4708, Balance: 0, Liability: true}}
	items := []Item{{Date: D(2026, 11, 5), Kind: KindExpense, AccountId: 4708, Amount: 300, Business: true}}
	months, _ := Walk(accts, items, D(2026, 11, 2), 1)
	assert.Equal(t, int64(1000), months[0].ClosingCash)
	assert.Equal(t, int64(300), months[0].BusinessCost)
}
