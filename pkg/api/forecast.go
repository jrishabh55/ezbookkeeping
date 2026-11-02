package api

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/forecast"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// ForecastApi represents forecast api
type ForecastApi struct {
	ApiUsingConfig
	forecasts *services.ForecastService
}

// Initialize a forecast api singleton instance
var (
	Forecast = &ForecastApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		forecasts: services.Forecasts,
	}
)

// ForecastGetHandler returns the cash forecast for 1-3 months starting at start_month
func (a *ForecastApi) ForecastGetHandler(c *core.WebContext) (any, *errs.Error) {
	var forecastGetReq models.ForecastGetRequest
	err := c.ShouldBindQuery(&forecastGetReq)

	if err != nil {
		log.Warnf(c, "[forecast.ForecastGetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	startMonth, err := time.Parse("2006-01", forecastGetReq.StartMonth)

	if err != nil {
		log.Warnf(c, "[forecast.ForecastGetHandler] start_month \"%s\" is invalid, because %s", forecastGetReq.StartMonth, err.Error())
		return nil, errs.ErrParameterInvalid
	}

	clientTimezone, err := c.GetClientTimezone()

	if err != nil {
		log.Warnf(c, "[forecast.ForecastGetHandler] cannot get client timezone, because %s", err.Error())
		return nil, errs.ErrClientTimezoneOffsetInvalid
	}

	year, month, day := time.Now().In(clientTimezone).Date()
	today := forecast.D(year, month, day)
	start := forecast.D(startMonth.Year(), startMonth.Month(), 1)

	if forecast.MonthStart(today).Before(start) {
		today = start
	}

	uid := c.GetCurrentUid()
	result, err := a.forecasts.GetForecast(c, uid, today, forecastGetReq.Months)

	if err != nil {
		log.Errorf(c, "[forecast.ForecastGetHandler] failed to get forecast for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return models.NewForecastResponse(result), nil
}
