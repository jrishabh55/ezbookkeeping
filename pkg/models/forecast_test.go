package models

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/forecast"
)

// TestNewForecastResponse_JSONContract pins the forecast API response shape: item "type" is the
// numeric forecast.Kind, every date is "YYYY-MM-DD", every month is "YYYY-MM", every id is a
// string, amounts are plain numbers, and the top-level keys are months/items/shortfalls/skippedCards.
func TestNewForecastResponse_JSONContract(t *testing.T) {
	result := &forecast.Result{
		Months: []forecast.Month{
			{Month: forecast.D(2026, 11, 1), OpeningCash: 100, ClosingCash: 200, ClosingCashWithoutUncertain: 150,
				BusinessIncome: 10, BusinessCost: 5, PersonalCost: 3},
		},
		Items: []forecast.Item{
			{Date: forecast.D(2026, 11, 5), Kind: forecast.KindExpense, AccountId: 7731, CategoryId: 9,
				Amount: 1000, Source: forecast.SourceTemplate, SourceId: 1, Uncertain: false, Business: true, Label: "x"},
			{Date: forecast.D(2026, 12, 3), Kind: forecast.KindTransfer, AccountId: 7731, DestinationAccountId: 4708,
				Amount: 32704, Source: forecast.SourceCardBill, Label: "HDFC 4708 bill"},
		},
		Shortfalls: []forecast.Shortfall{
			{AccountId: 9260, Date: forecast.D(2026, 11, 5), LowestBalance: -500, LowestDate: forecast.D(2026, 11, 5)},
		},
		SkippedCards: []int64{2922},
	}

	b, err := json.Marshal(NewForecastResponse(result))
	assert.Nil(t, err)

	assert.JSONEq(t, `{
		"months": [
			{"month":"2026-11","openingCash":100,"closingCash":200,"closingCashWithoutUncertain":150,"businessIncome":10,"businessCost":5,"personalCost":3}
		],
		"items": [
			{"date":"2026-11-05","type":1,"accountId":"7731","categoryId":"9","amount":1000,"source":"template","sourceId":"1","uncertain":false,"business":true,"label":"x"},
			{"date":"2026-12-03","type":3,"accountId":"7731","destinationAccountId":"4708","amount":32704,"source":"card_bill","uncertain":false,"business":false,"label":"HDFC 4708 bill"}
		],
		"shortfalls": [
			{"accountId":"9260","date":"2026-11-05","lowestBalance":-500,"lowestDate":"2026-11-05"}
		],
		"skippedCards": ["2922"]
	}`, string(b))
}
