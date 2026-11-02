package forecast

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuild_NovemberShape(t *testing.T) {
	in := Input{
		Today:  D(2026, 11, 2),
		Months: 1,
		Accounts: []Account{
			{Id: 7731, Balance: 215060_45, Cash: true}, {Id: 9260, Balance: 16366_21, Cash: true},
			{Id: 4708, Balance: 32704_35, Liability: true}, {Id: 8765, Balance: 182659_00, Liability: true},
		},
		Templates: []Template{
			{Id: 1, Kind: KindExpense, AccountId: 7731, CategoryId: 9, Amount: 18000_00, FrequencyType: FreqMonthly, Frequency: "3", Business: true, Label: "Nikhil"},
			{Id: 2, Kind: KindTransfer, AccountId: 9260, DestinationAccountId: 8765, Amount: 17512_00, FrequencyType: FreqMonthly, Frequency: "5", Label: "Car EMI"},
			{Id: 3, Kind: KindExpense, AccountId: 7731, CategoryId: 9, Amount: 10000_00, FrequencyType: FreqMonthly, Frequency: "2", End: D(2026, 10, 31), Label: "Rakshit"},
		},
		Income: []ExpectedIncome{{Date: D(2026, 11, 28), AccountId: 7731, Amount: 370000_00, Source: SourceRecurringInvoice, SourceId: 5, Label: "Quber"}},
		Cards:  []Card{{AccountId: 4708, PayFromAccountId: 7731, StatementDay: 14, DueDay: 3, Outstanding: 32704_35}, {AccountId: 2922}},
	}
	r := Build(in)

	labels := map[string]bool{}
	for _, it := range r.Items {
		labels[it.Label] = true
	}
	assert.True(t, labels["Nikhil"] && labels["Car EMI"] && labels["Quber"])
	assert.False(t, labels["Rakshit"]) // ended
	assert.Equal(t, []int64{2922}, r.SkippedCards)
	assert.Len(t, r.Shortfalls, 1)
	assert.Equal(t, int64(9260), r.Shortfalls[0].AccountId)
	// the 14-Oct statement in Outstanding is unpaid and due 3 Nov, so it is paid in November
	assert.Equal(t, int64(215060_45+16366_21-18000_00-17512_00-32704_35), r.Months[0].ClosingCashWithoutUncertain)
	var bill Item
	for _, it := range r.Items {
		if it.Source == SourceCardBill {
			bill = it
		}
	}
	assert.Equal(t, D(2026, 11, 3), bill.Date)
	assert.Equal(t, int64(32704_35), bill.Amount)
	assert.Equal(t, r.Months[0].ClosingCashWithoutUncertain+370000_00, r.Months[0].ClosingCash)
}

func TestBuild_OverdueUnfulfilledTemplateMovesToToday(t *testing.T) {
	in := Input{Today: D(2026, 11, 10), Months: 1, Accounts: []Account{{Id: 1, Balance: 100000, Cash: true}},
		Templates: []Template{{Id: 1, Kind: KindExpense, AccountId: 1, CategoryId: 2, Amount: 500, FrequencyType: FreqMonthly, Frequency: "3"}}}
	r := Build(in)
	assert.Len(t, r.Items, 1)
	assert.Equal(t, D(2026, 11, 10), r.Items[0].Date)

	in.History = []Txn{{Date: D(2026, 11, 3), Kind: KindExpense, AccountId: 1, CategoryId: 2, Amount: 500}}
	assert.Empty(t, Build(in).Items)
}

// Current-month everyday spending is spread over the remaining days: 9260 runs short on the EMI
// date (5 Nov), not today, and the month's totals are the same as with one lump
func TestBuild_EverydaySpreadShortfallOnEmiDate(t *testing.T) {
	var history []Txn
	for _, m := range []int{8, 9, 10} {
		history = append(history, exp(2026, m, 10, 9260, 5, 20000_00))
	}
	in := Input{
		Today: D(2026, 11, 2), Months: 1, History: history,
		Accounts:  []Account{{Id: 9260, Balance: 16366_21, Cash: true}},
		Templates: []Template{{Id: 2, Kind: KindTransfer, AccountId: 9260, DestinationAccountId: 8765, Amount: 17512_00, FrequencyType: FreqMonthly, Frequency: "5", Label: "Car EMI"}},
	}
	r := Build(in)

	everyday := 0
	for _, it := range r.Items {
		if it.Source == SourceEveryday {
			everyday++
			assert.Equal(t, int64(20000_00), it.Amount) // one display item per account
			assert.Equal(t, D(2026, 11, 2), it.Date)
		}
	}
	assert.Equal(t, 1, everyday)
	assert.Len(t, r.Shortfalls, 1)
	assert.Equal(t, D(2026, 11, 5), r.Shortfalls[0].Date)
	assert.Equal(t, D(2026, 11, 30), r.Shortfalls[0].LowestDate)
	assert.Equal(t, int64(16366_21-17512_00-20000_00), r.Months[0].ClosingCash)
	assert.Equal(t, int64(20000_00), r.Months[0].PersonalCost)
}

// Everyday spending on a card is billed by day: the part up to the 14-Nov statement is due 3 Dec,
// the rest of November is on the 14-Dec statement (due 3 Jan)
func TestBuild_CardEverydaySplitAcrossStatements(t *testing.T) {
	var history []Txn
	for _, m := range []int{8, 9, 10} {
		history = append(history, exp(2026, m, 10, 4708, 5, 3000_00))
	}
	in := Input{
		Today: D(2026, 11, 1), Months: 2, History: history,
		Accounts: []Account{{Id: 7731, Balance: 100000_00, Cash: true}, {Id: 4708, Liability: true}},
		Cards:    []Card{{AccountId: 4708, PayFromAccountId: 7731, StatementDay: 14, DueDay: 3, Label: "HDFC 4708"}},
	}
	r := Build(in)
	var bills []Item
	for _, it := range r.Items {
		if it.Source == SourceCardBill {
			bills = append(bills, it)
		}
	}
	assert.Len(t, bills, 1)
	assert.Equal(t, D(2026, 12, 3), bills[0].Date)
	assert.Equal(t, int64(3000_00*14/30), bills[0].Amount) // 1-14 Nov of a 100/day spread
}
