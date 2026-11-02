package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/forecast"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func newForecastTestContext(headers map[string]string) *core.WebContext {
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/mcp", nil)
	ginContext.Request.Header.Set("Authorization", "Bearer token")

	for k, v := range headers {
		ginContext.Request.Header.Set(k, v)
	}

	return &core.WebContext{Context: ginContext}
}

// TestForecastClientTimezone_NoHeadersUsesLatestTransactionOffset verifies that a request with only
// Authorization (as MCP clients send) falls back to the latest transaction's utc offset
func TestForecastClientTimezone_NoHeadersUsesLatestTransactionOffset(t *testing.T) {
	loc, err := forecastClientTimezone(newForecastTestContext(nil), func() (*models.Transaction, error) {
		return &models.Transaction{TimezoneUtcOffset: 330}, nil
	})

	assert.Nil(t, err)
	_, offset := time.Date(2026, 11, 2, 0, 0, 0, 0, loc).Zone()
	assert.Equal(t, 330*60, offset)
}

// TestForecastClientTimezone_NoHeadersNoTransactionsIsUtc verifies the UTC fallback for a user without transactions
func TestForecastClientTimezone_NoHeadersNoTransactionsIsUtc(t *testing.T) {
	loc, err := forecastClientTimezone(newForecastTestContext(nil), func() (*models.Transaction, error) {
		return nil, nil
	})

	assert.Nil(t, err)
	assert.Equal(t, time.UTC, loc)
}

// TestForecastClientTimezone_HeadersWin verifies that timezone headers, when sent, are used and the
// latest transaction is not looked up; an invalid header is still an error
func TestForecastClientTimezone_HeadersWin(t *testing.T) {
	lookedUp := false
	latest := func() (*models.Transaction, error) {
		lookedUp = true
		return &models.Transaction{TimezoneUtcOffset: 330}, nil
	}

	loc, err := forecastClientTimezone(newForecastTestContext(map[string]string{core.ClientTimezoneOffsetHeaderName: "-300"}), latest)

	assert.Nil(t, err)
	_, offset := time.Date(2026, 11, 2, 0, 0, 0, 0, loc).Zone()
	assert.Equal(t, -300*60, offset)
	assert.False(t, lookedUp)

	_, err = forecastClientTimezone(newForecastTestContext(map[string]string{core.ClientTimezoneOffsetHeaderName: "abc"}), latest)
	assert.NotNil(t, err)
}

// TestResolveForecastMonths_Default verifies that an unset (zero) months value defaults to 1
func TestResolveForecastMonths_Default(t *testing.T) {
	months, err := resolveForecastMonths(0)

	assert.Nil(t, err)
	assert.Equal(t, forecastDefaultMonths, months)
}

// TestResolveForecastMonths_Valid verifies that values within [1, 3] pass through unchanged
func TestResolveForecastMonths_Valid(t *testing.T) {
	for _, months := range []int{1, 2, 3} {
		got, err := resolveForecastMonths(months)

		assert.Nil(t, err)
		assert.Equal(t, months, got)
	}
}

// TestResolveForecastMonths_OutOfRange verifies that values outside [1, 3] are rejected with a clear error
func TestResolveForecastMonths_OutOfRange(t *testing.T) {
	for _, months := range []int{-1, 4, 12} {
		_, err := resolveForecastMonths(months)

		assert.Equal(t, errs.ErrParameterInvalid, err)
	}
}

// TestNewMCPQueryForecastResponse_MapsFields verifies that months, shortfalls, items and skipped
// cards are mapped from the forecast engine result, amounts are formatted like other MCP tools,
// and account ids are resolved to names
func TestNewMCPQueryForecastResponse_MapsFields(t *testing.T) {
	result := &forecast.Result{
		Months: []forecast.Month{
			{Month: forecast.D(2026, 11, 1), OpeningCash: 100000, ClosingCash: 255200, ClosingCashWithoutUncertain: 150000,
				BusinessIncome: 100000, BusinessCost: 50000, PersonalCost: 30000},
		},
		Items: []forecast.Item{
			{Date: forecast.D(2026, 11, 5), Kind: forecast.KindExpense, AccountId: 7731, Amount: 100000,
				Source: forecast.SourceTemplate, Uncertain: false, Label: "Rent"},
		},
		Shortfalls: []forecast.Shortfall{
			{AccountId: 7731, Date: forecast.D(2026, 11, 5), LowestBalance: -50000, LowestDate: forecast.D(2026, 11, 10)},
		},
		SkippedCards: []int64{4708},
	}

	accountNameById := map[int64]string{7731: "HDFC Checking"}

	response := newMCPQueryForecastResponse(result, accountNameById)

	assert.Len(t, response.Months, 1)
	assert.Equal(t, "2026-11", response.Months[0].Month)
	assert.Equal(t, "1000.00", response.Months[0].OpeningCash)
	assert.Equal(t, "2552.00", response.Months[0].ClosingCash)
	assert.Equal(t, "1500.00", response.Months[0].ClosingCashWithoutUncertain)
	assert.Equal(t, "1000.00", response.Months[0].BusinessIncome)
	assert.Equal(t, "500.00", response.Months[0].BusinessCost)
	assert.Equal(t, "300.00", response.Months[0].PersonalCost)

	assert.Len(t, response.Shortfalls, 1)
	assert.Equal(t, "HDFC Checking", response.Shortfalls[0].Account)
	assert.Equal(t, "2026-11-05", response.Shortfalls[0].Date)
	assert.Equal(t, "-500.00", response.Shortfalls[0].LowestBalance)
	assert.Equal(t, "2026-11-10", response.Shortfalls[0].LowestDate)

	assert.Len(t, response.Items, 1)
	assert.Equal(t, "2026-11-05", response.Items[0].Date)
	assert.Equal(t, "Rent", response.Items[0].Label)
	assert.Equal(t, "HDFC Checking", response.Items[0].Account)
	assert.Equal(t, "1000.00", response.Items[0].Amount)
	assert.False(t, response.Items[0].Uncertain)

	assert.Equal(t, []string{"4708"}, response.SkippedCards)
}

// TestNewMCPQueryForecastResponse_UnknownAccountRendersAsId verifies that an account id with no
// matching name in the lookup map is rendered as the id itself, both for shortfalls/items and
// skipped cards
func TestNewMCPQueryForecastResponse_UnknownAccountRendersAsId(t *testing.T) {
	result := &forecast.Result{
		Items: []forecast.Item{
			{Date: forecast.D(2026, 11, 5), AccountId: 9999, Amount: 100, Label: "Unknown"},
		},
		Shortfalls: []forecast.Shortfall{
			{AccountId: 9999, Date: forecast.D(2026, 11, 5), LowestBalance: -100, LowestDate: forecast.D(2026, 11, 5)},
		},
		SkippedCards: []int64{9999},
	}

	response := newMCPQueryForecastResponse(result, map[int64]string{})

	assert.Equal(t, "9999", response.Items[0].Account)
	assert.Equal(t, "9999", response.Shortfalls[0].Account)
	assert.Equal(t, []string{"9999"}, response.SkippedCards)
}

// TestNewMCPQueryForecastResponse_CapsItemsAt50 verifies that only the first 50 items, ordered by
// date, are returned even when the engine result has more
func TestNewMCPQueryForecastResponse_CapsItemsAt50(t *testing.T) {
	result := &forecast.Result{}

	// insert out of date order to also verify sorting
	for i := 59; i >= 0; i-- {
		result.Items = append(result.Items, forecast.Item{
			Date: forecast.D(2026, 11, 1).AddDate(0, 0, i), AccountId: 1, Amount: int64(i), Label: "item",
		})
	}

	response := newMCPQueryForecastResponse(result, map[int64]string{})

	assert.Len(t, response.Items, forecastMaxItems)
	assert.Equal(t, "2026-11-01", response.Items[0].Date)
	assert.Equal(t, "2026-11-01", response.Items[0].Date)
	assert.Equal(t, forecast.D(2026, 11, 1).AddDate(0, 0, forecastMaxItems-1).Format(forecastDateFormat), response.Items[forecastMaxItems-1].Date)
}
