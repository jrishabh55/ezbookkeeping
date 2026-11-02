package forecast

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var hdfc4708 = Card{AccountId: 4708, PayFromAccountId: 7731, StatementDay: 14, DueDay: 3, Outstanding: 32704_00, Label: "HDFC 4708"}

func cardSpend(day int, amt int64) Item {
	return Item{Date: D(2026, 11, day), Kind: KindExpense, AccountId: 4708, Amount: amt}
}

func cardTxn(month, day int, amt int64) Txn {
	return exp(2026, month, day, 4708, 1, amt)
}

func cardRepayment(month, day int, amt int64) Txn {
	return Txn{Date: D(2026, time.Month(month), day), Kind: KindTransfer, AccountId: 7731, DestinationAccountId: 4708, Amount: amt, CardRepayment: true}
}

// On 2 Nov the 14-Oct statement (in Outstanding, unpaid) is due on 3 Nov, not 3 Dec; real spend
// after 14 Oct is not part of it and carries into the 14-Nov statement, due 3 Dec
func TestCardBills_UnpaidLastStatementPaysOnNextDueDay(t *testing.T) {
	history := []Txn{cardTxn(10, 10, 999_00), cardTxn(10, 25, 2000_00), cardTxn(11, 2, 300_00)} // 10 Oct is before S0
	bills, skipped := CardBills([]Card{hdfc4708}, []Item{cardSpend(10, 5000_00)}, history, D(2026, 11, 2), D(2027, 1, 1))
	assert.Empty(t, skipped)
	assert.Len(t, bills, 2)
	assert.Equal(t, D(2026, 11, 3), bills[0].Date)
	assert.Equal(t, int64(32704_00-2300_00), bills[0].Amount)
	assert.Equal(t, KindTransfer, bills[0].Kind)
	assert.Equal(t, int64(7731), bills[0].AccountId)
	assert.Equal(t, int64(4708), bills[0].DestinationAccountId)
	assert.Equal(t, D(2026, 12, 3), bills[1].Date)
	assert.Equal(t, int64(2300_00+5000_00), bills[1].Amount)
}

func TestCardBills_DueDayBeforeStatementDayIsNextMonth(t *testing.T) {
	c := hdfc4708
	c.Outstanding = 0
	bills, skipped := CardBills([]Card{c}, []Item{cardSpend(10, 5000_00), cardSpend(20, 1000_00)}, nil, D(2026, 11, 2), D(2027, 1, 1))
	assert.Empty(t, skipped)
	assert.Len(t, bills, 1) // the 20-Nov spend is on the 14-Dec statement, due 3 Jan
	assert.Equal(t, D(2026, 12, 3), bills[0].Date)
	assert.Equal(t, int64(5000_00), bills[0].Amount)
}

func TestCardBills_SecondCycleOnlyNewSpend(t *testing.T) {
	bills, _ := CardBills([]Card{hdfc4708}, []Item{cardSpend(10, 500_00), cardSpend(20, 1000_00)}, nil, D(2026, 11, 2), D(2027, 2, 1))
	assert.Len(t, bills, 3)
	assert.Equal(t, int64(32704_00), bills[0].Amount)
	assert.Equal(t, D(2026, 12, 3), bills[1].Date)
	assert.Equal(t, int64(500_00), bills[1].Amount)
	assert.Equal(t, D(2027, 1, 3), bills[2].Date)
	assert.Equal(t, int64(1000_00), bills[2].Amount)
}

// The 14-Oct statement (32,704) was paid on 28 Oct: no 3-Nov bill; the unpaid rest and the
// post-statement spend carry into the 14-Nov statement
func TestCardBills_AlreadyPaidCycleNotBilledAgain(t *testing.T) {
	c := hdfc4708
	c.Outstanding = 32704_00 - 30000_00 + 1500_00 // after the repayment and a 1,500 spend on 25 Oct
	history := []Txn{cardTxn(10, 25, 1500_00), cardRepayment(10, 28, 30000_00)}
	bills, _ := CardBills([]Card{c}, []Item{cardSpend(10, 2000_00)}, history, D(2026, 11, 2), D(2027, 1, 1))
	assert.Len(t, bills, 1)
	assert.Equal(t, D(2026, 12, 3), bills[0].Date)
	assert.Equal(t, c.Outstanding+2000_00, bills[0].Amount)
}

// A repayment below 90% of the statement does not settle it: the unpaid rest is billed on the due day
func TestCardBills_PartlyPaidStatementBillsTheRest(t *testing.T) {
	c := hdfc4708
	c.Outstanding = 32704_00 - 10000_00
	bills, _ := CardBills([]Card{c}, nil, []Txn{cardRepayment(10, 20, 10000_00)}, D(2026, 11, 2), D(2026, 12, 1))
	assert.Len(t, bills, 1)
	assert.Equal(t, D(2026, 11, 3), bills[0].Date)
	assert.Equal(t, int64(22704_00), bills[0].Amount)
}

// Items dated today (current-month everyday slices, overdue templates moved to today) are billed
// exactly once, in the next statement; also on a statement day, where today's statement is still to come
func TestCardBills_ItemsDatedTodayBilledOnce(t *testing.T) {
	c := hdfc4708
	c.Outstanding = 0
	bills, _ := CardBills([]Card{c}, []Item{cardSpend(2, 700_00)}, nil, D(2026, 11, 2), D(2027, 2, 1))
	assert.Len(t, bills, 1)
	assert.Equal(t, D(2026, 12, 3), bills[0].Date)
	assert.Equal(t, int64(700_00), bills[0].Amount)

	bills, _ = CardBills([]Card{c}, []Item{cardSpend(14, 700_00)}, nil, D(2026, 11, 14), D(2027, 2, 1))
	assert.Len(t, bills, 1)
	assert.Equal(t, D(2026, 12, 3), bills[0].Date)
	assert.Equal(t, int64(700_00), bills[0].Amount)
}

// A credit balance is not lost: it offsets later cycles until used up
func TestCardBills_CreditBalanceCarriesForward(t *testing.T) {
	c := hdfc4708
	c.Outstanding = -500_00
	items := []Item{cardSpend(10, 200_00), cardSpend(20, 1000_00)}
	bills, _ := CardBills([]Card{c}, items, nil, D(2026, 11, 2), D(2027, 2, 1))
	assert.Len(t, bills, 1)
	assert.Equal(t, D(2027, 1, 3), bills[0].Date)
	assert.Equal(t, int64(700_00), bills[0].Amount)
}

func TestCardBills_MissingFieldsSkipped(t *testing.T) {
	c := hdfc4708
	c.DueDay = 0
	bills, skipped := CardBills([]Card{c}, nil, nil, D(2026, 11, 2), D(2027, 1, 1))
	assert.Empty(t, bills)
	assert.Equal(t, []int64{4708}, skipped)
}
