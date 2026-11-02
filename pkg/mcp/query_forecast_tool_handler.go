package mcp

import (
	"encoding/json/v2"
	"reflect"
	"sort"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/forecast"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// forecastDefaultMonths is the number of months the forecast covers when the caller omits it
const forecastDefaultMonths = 1

// forecastMinMonths and forecastMaxMonths bound the number of months the forecast can cover
const forecastMinMonths = 1
const forecastMaxMonths = 3

// forecastMaxItems is the maximum number of expected items returned, earliest first
const forecastMaxItems = 50

// forecastMonthFormat and forecastDateFormat match the ones used by the forecast HTTP API
const forecastMonthFormat = "2006-01"
const forecastDateFormat = "2006-01-02"

// MCPQueryForecastRequest represents all parameters of the query forecast request
type MCPQueryForecastRequest struct {
	Months int `json:"months,omitempty" jsonschema:"minimum=1,maximum=3,default=1" jsonschema_description:"Number of months to forecast, 1-3 (optional, default 1)"`
}

// MCPQueryForecastResponse represents the response structure for querying the cash forecast
type MCPQueryForecastResponse struct {
	Months       []*MCPForecastMonthInfo     `json:"months" jsonschema_description:"Month-by-month cash forecast summary"`
	Shortfalls   []*MCPForecastShortfallInfo `json:"shortfalls,omitempty" jsonschema_description:"Accounts expected to go below zero during the forecast horizon"`
	Items        []*MCPForecastItemInfo      `json:"items,omitempty" jsonschema_description:"Upcoming expected items, earliest first (at most 50)"`
	SkippedCards []string                    `json:"skippedCards,omitempty" jsonschema_description:"Names of credit card accounts skipped from the forecast (e.g. missing statement/due day or pay-from account)"`
}

// MCPForecastMonthInfo defines the structure of one month of the forecast summary
type MCPForecastMonthInfo struct {
	Month                       string `json:"month" jsonschema_description:"Month in YYYY-MM format"`
	OpeningCash                 string `json:"openingCash" jsonschema_description:"Total cash balance at the start of the month"`
	ClosingCash                 string `json:"closingCash" jsonschema_description:"Total cash balance at the end of the month, including uncertain expected income"`
	ClosingCashWithoutUncertain string `json:"closingCashWithoutUncertain" jsonschema_description:"Total cash balance at the end of the month, excluding uncertain expected income"`
	BusinessIncome              string `json:"businessIncome" jsonschema_description:"Expected business income for the month"`
	BusinessCost                string `json:"businessCost" jsonschema_description:"Expected business cost for the month"`
	PersonalCost                string `json:"personalCost" jsonschema_description:"Expected personal cost for the month"`
}

// MCPForecastShortfallInfo defines the structure of an account expected to go below zero
type MCPForecastShortfallInfo struct {
	Account       string `json:"account" jsonschema_description:"Name of the account expected to go below zero"`
	Date          string `json:"date" jsonschema_description:"First date the account is expected to go below zero, in YYYY-MM-DD format"`
	LowestBalance string `json:"lowestBalance" jsonschema_description:"Lowest balance the account is expected to reach"`
	LowestDate    string `json:"lowestDate" jsonschema_description:"Date the account reaches its lowest expected balance, in YYYY-MM-DD format"`
}

// MCPForecastItemInfo defines the structure of one expected money movement
type MCPForecastItemInfo struct {
	Date      string `json:"date" jsonschema_description:"Date of the expected item, in YYYY-MM-DD format"`
	Label     string `json:"label" jsonschema_description:"Description of the expected item"`
	Account   string `json:"account" jsonschema_description:"Name of the account involved"`
	Amount    string `json:"amount" jsonschema_description:"Amount of the expected item"`
	Uncertain bool   `json:"uncertain" jsonschema_description:"Whether this item is uncertain (e.g. expected invoice income that has not been received yet)"`
}

type mcpQueryForecastToolHandler struct{}

var MCPQueryForecastToolHandler = &mcpQueryForecastToolHandler{}

// ToolType returns the MCP tool type
func (h *mcpQueryForecastToolHandler) ToolType() core.MCPToolType {
	return core.MCP_TOOL_TYPE_QUERY_FORECAST
}

// Name returns the name of the MCP tool
func (h *mcpQueryForecastToolHandler) Name() string {
	return "query_forecast"
}

// Description returns the description of the MCP tool
func (h *mcpQueryForecastToolHandler) Description() string {
	return "Query the cash forecast: month-end cash with and without expected invoice income, accounts that will go below zero, and upcoming expected items"
}

// InputType returns the input type for the MCP tool request
func (h *mcpQueryForecastToolHandler) InputType() reflect.Type {
	return reflect.TypeOf(&MCPQueryForecastRequest{})
}

// OutputType returns the output type for the MCP tool response
func (h *mcpQueryForecastToolHandler) OutputType() reflect.Type {
	return reflect.TypeOf(&MCPQueryForecastResponse{})
}

// Handle processes the MCP call tool request and returns the response
func (h *mcpQueryForecastToolHandler) Handle(c *core.WebContext, callToolReq *MCPCallToolRequest, user *models.User, currentConfig *settings.Config, services MCPAvailableServices) (any, []*MCPTextContent, error) {
	var queryForecastRequest MCPQueryForecastRequest

	if callToolReq.Arguments != nil {
		if err := json.Unmarshal(callToolReq.Arguments, &queryForecastRequest); err != nil {
			return nil, nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
		}
	}

	months, err := resolveForecastMonths(queryForecastRequest.Months)

	if err != nil {
		return nil, nil, err
	}

	uid := user.Uid
	clientTimezone, err := forecastClientTimezone(c, func() (*models.Transaction, error) {
		transactions, err := services.GetTransactionService().GetTransactionsByMaxTime(c, uid, 0, 0, 0, nil, nil, nil, false, "", "", core.MATCH_MODE_DEFAULT, false, 1, 1, false, true)

		if err != nil || len(transactions) < 1 {
			return nil, err
		}

		return transactions[0], nil
	})

	if err != nil {
		log.Warnf(c, "[query_forecast_tool_handler.Handle] cannot get client timezone, because %s", err.Error())
		return nil, nil, errs.ErrClientTimezoneOffsetInvalid
	}

	year, month, day := time.Now().In(clientTimezone).Date()
	today := forecast.D(year, month, day)

	result, err := services.GetForecastService().GetForecast(c, uid, today, months)

	if err != nil {
		log.Errorf(c, "[query_forecast_tool_handler.Handle] failed to get forecast for user \"uid:%d\", because %s", uid, err.Error())
		return nil, nil, err
	}

	accounts, err := services.GetAccountService().GetAllAccountsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[query_forecast_tool_handler.Handle] failed to get all accounts for user \"uid:%d\", because %s", uid, err.Error())
		return nil, nil, err
	}

	accountNameById := make(map[int64]string, len(accounts))

	for _, a := range accounts {
		accountNameById[a.AccountId] = a.Name
	}

	response := newMCPQueryForecastResponse(result, accountNameById)

	content, err := json.Marshal(response)

	if err != nil {
		return nil, nil, err
	}

	return response, []*MCPTextContent{
		NewMCPTextContent(string(content)),
	}, nil
}

// forecastClientTimezone returns the timezone "today" is computed in: the client's timezone headers
// when either is sent; otherwise (MCP clients usually send only Authorization) the utc offset of the
// user's most recent transaction, as returned by latestTransaction; with no transactions at all, UTC
func forecastClientTimezone(c *core.WebContext, latestTransaction func() (*models.Transaction, error)) (*time.Location, error) {
	if c.GetHeader(core.ClientTimezoneNameHeaderName) != "" || c.GetHeader(core.ClientTimezoneOffsetHeaderName) != "" {
		return c.GetClientTimezone()
	}

	transaction, err := latestTransaction()

	if err != nil {
		return nil, err
	}

	if transaction == nil {
		return time.UTC, nil
	}

	return time.FixedZone("Latest Transaction Timezone", int(transaction.TimezoneUtcOffset)*60), nil
}

// resolveForecastMonths returns forecastDefaultMonths when months is not set (zero), months itself
// when it is within [forecastMinMonths, forecastMaxMonths], or an error otherwise
func resolveForecastMonths(months int) (int, error) {
	if months == 0 {
		return forecastDefaultMonths, nil
	}

	if months < forecastMinMonths || months > forecastMaxMonths {
		return 0, errs.ErrParameterInvalid
	}

	return months, nil
}

// newMCPQueryForecastResponse maps a forecast engine result and an account id -> name map into the
// MCP tool's response; this is the pure part of the handler, covered directly by unit tests
func newMCPQueryForecastResponse(result *forecast.Result, accountNameById map[int64]string) *MCPQueryForecastResponse {
	response := &MCPQueryForecastResponse{
		Months: make([]*MCPForecastMonthInfo, 0, len(result.Months)),
	}

	for _, m := range result.Months {
		response.Months = append(response.Months, &MCPForecastMonthInfo{
			Month:                       m.Month.Format(forecastMonthFormat),
			OpeningCash:                 utils.FormatAmount(m.OpeningCash),
			ClosingCash:                 utils.FormatAmount(m.ClosingCash),
			ClosingCashWithoutUncertain: utils.FormatAmount(m.ClosingCashWithoutUncertain),
			BusinessIncome:              utils.FormatAmount(m.BusinessIncome),
			BusinessCost:                utils.FormatAmount(m.BusinessCost),
			PersonalCost:                utils.FormatAmount(m.PersonalCost),
		})
	}

	if len(result.Shortfalls) > 0 {
		response.Shortfalls = make([]*MCPForecastShortfallInfo, 0, len(result.Shortfalls))

		for _, s := range result.Shortfalls {
			response.Shortfalls = append(response.Shortfalls, &MCPForecastShortfallInfo{
				Account:       forecastAccountName(accountNameById, s.AccountId),
				Date:          s.Date.Format(forecastDateFormat),
				LowestBalance: utils.FormatAmount(s.LowestBalance),
				LowestDate:    s.LowestDate.Format(forecastDateFormat),
			})
		}
	}

	items := make([]forecast.Item, len(result.Items))
	copy(items, result.Items)
	sort.SliceStable(items, func(i, j int) bool { return items[i].Date.Before(items[j].Date) })

	if len(items) > forecastMaxItems {
		items = items[:forecastMaxItems]
	}

	if len(items) > 0 {
		response.Items = make([]*MCPForecastItemInfo, 0, len(items))

		for _, it := range items {
			response.Items = append(response.Items, &MCPForecastItemInfo{
				Date:      it.Date.Format(forecastDateFormat),
				Label:     it.Label,
				Account:   forecastAccountName(accountNameById, it.AccountId),
				Amount:    utils.FormatAmount(it.Amount),
				Uncertain: it.Uncertain,
			})
		}
	}

	if len(result.SkippedCards) > 0 {
		response.SkippedCards = make([]string, 0, len(result.SkippedCards))

		for _, id := range result.SkippedCards {
			response.SkippedCards = append(response.SkippedCards, forecastAccountName(accountNameById, id))
		}
	}

	return response
}

// forecastAccountName returns the account's name, or the id itself (as a string) when the account is unknown
func forecastAccountName(accountNameById map[int64]string, id int64) string {
	if name, ok := accountNameById[id]; ok {
		return name
	}

	return utils.Int64ToString(id)
}
