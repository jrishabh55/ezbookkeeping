package models

import (
	"github.com/mayswind/ezbookkeeping/pkg/forecast"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// forecastDateFormat is the calendar date format used in forecast API responses
const forecastDateFormat = "2006-01-02"

// forecastMonthFormat is the year-month format used in forecast API responses
const forecastMonthFormat = "2006-01"

// ForecastGetRequest represents all parameters of forecast getting request
type ForecastGetRequest struct {
	StartMonth string `form:"start_month" binding:"required,len=7"`
	Months     int    `form:"months" binding:"required,min=1,max=3"`
}

// ForecastMonthResponse represents one month of the forecast summary
type ForecastMonthResponse struct {
	Month                       string `json:"month"`
	OpeningCash                 int64  `json:"openingCash"`
	ClosingCash                 int64  `json:"closingCash"`
	ClosingCashWithoutUncertain int64  `json:"closingCashWithoutUncertain"`
	BusinessIncome              int64  `json:"businessIncome"`
	BusinessCost                int64  `json:"businessCost"`
	PersonalCost                int64  `json:"personalCost"`
}

// ForecastItemResponse represents one expected money movement
type ForecastItemResponse struct {
	Date                 string        `json:"date"`
	Type                 forecast.Kind `json:"type"`
	AccountId            string        `json:"accountId"`
	DestinationAccountId string        `json:"destinationAccountId,omitempty"`
	CategoryId           string        `json:"categoryId,omitempty"`
	Amount               int64         `json:"amount"`
	Source               string        `json:"source"`
	SourceId             string        `json:"sourceId,omitempty"`
	Uncertain            bool          `json:"uncertain"`
	Business             bool          `json:"business"`
	Label                string        `json:"label"`
}

// ForecastShortfallResponse represents the first date an account is expected to go below zero
type ForecastShortfallResponse struct {
	AccountId     string `json:"accountId"`
	Date          string `json:"date"`
	LowestBalance int64  `json:"lowestBalance"`
	LowestDate    string `json:"lowestDate"`
}

// ForecastResponse represents the forecast result
type ForecastResponse struct {
	Months       []*ForecastMonthResponse     `json:"months"`
	Items        []*ForecastItemResponse      `json:"items"`
	Shortfalls   []*ForecastShortfallResponse `json:"shortfalls"`
	SkippedCards []string                     `json:"skippedCards"`
}

// NewForecastResponse returns a view-object according to the forecast engine result
func NewForecastResponse(r *forecast.Result) *ForecastResponse {
	resp := &ForecastResponse{
		Months:       make([]*ForecastMonthResponse, 0, len(r.Months)),
		Items:        make([]*ForecastItemResponse, 0, len(r.Items)),
		Shortfalls:   make([]*ForecastShortfallResponse, 0, len(r.Shortfalls)),
		SkippedCards: make([]string, 0, len(r.SkippedCards)),
	}

	for _, m := range r.Months {
		resp.Months = append(resp.Months, &ForecastMonthResponse{
			Month:                       m.Month.Format(forecastMonthFormat),
			OpeningCash:                 m.OpeningCash,
			ClosingCash:                 m.ClosingCash,
			ClosingCashWithoutUncertain: m.ClosingCashWithoutUncertain,
			BusinessIncome:              m.BusinessIncome,
			BusinessCost:                m.BusinessCost,
			PersonalCost:                m.PersonalCost,
		})
	}

	for _, it := range r.Items {
		item := &ForecastItemResponse{
			Date:      it.Date.Format(forecastDateFormat),
			Type:      it.Kind,
			AccountId: utils.Int64ToString(it.AccountId),
			Amount:    it.Amount,
			Source:    string(it.Source),
			Uncertain: it.Uncertain,
			Business:  it.Business,
			Label:     it.Label,
		}

		if it.DestinationAccountId != 0 {
			item.DestinationAccountId = utils.Int64ToString(it.DestinationAccountId)
		}

		if it.CategoryId != 0 {
			item.CategoryId = utils.Int64ToString(it.CategoryId)
		}

		if it.SourceId != 0 {
			item.SourceId = utils.Int64ToString(it.SourceId)
		}

		resp.Items = append(resp.Items, item)
	}

	for _, s := range r.Shortfalls {
		resp.Shortfalls = append(resp.Shortfalls, &ForecastShortfallResponse{
			AccountId:     utils.Int64ToString(s.AccountId),
			Date:          s.Date.Format(forecastDateFormat),
			LowestBalance: s.LowestBalance,
			LowestDate:    s.LowestDate.Format(forecastDateFormat),
		})
	}

	for _, id := range r.SkippedCards {
		resp.SkippedCards = append(resp.SkippedCards, utils.Int64ToString(id))
	}

	return resp
}
