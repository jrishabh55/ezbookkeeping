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
	assert.Equal(t, int64(215060_45+16366_21-18000_00-17512_00), r.Months[0].ClosingCashWithoutUncertain) // card bill due 3 Dec, outside Nov
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
