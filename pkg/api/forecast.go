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

	clientTimezone, err := c.GetClientTimezone()

	if err != nil {
		log.Warnf(c, "[forecast.ForecastGetHandler] cannot get client timezone, because %s", err.Error())
		return nil, errs.ErrClientTimezoneOffsetInvalid
	}

	today, err := forecastToday(forecastGetReq.StartMonth, time.Now().In(clientTimezone))

	if err != nil {
		log.Warnf(c, "[forecast.ForecastGetHandler] start_month \"%s\" is invalid, because %s", forecastGetReq.StartMonth, err.Error())
		return nil, errs.ErrParameterInvalid
	}

	uid := c.GetCurrentUid()
	result, err := a.forecasts.GetForecast(c, uid, today, forecastGetReq.Months)

	if err != nil {
		log.Errorf(c, "[forecast.ForecastGetHandler] failed to get forecast for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return models.NewForecastResponse(result), nil
}

// forecastToday returns the client's local date for clientNow, provided startMonth ("2006-01") is the
// client's current month; the forecast always starts from today's balances, so any other start
// month would give wrong numbers and is rejected (months 2-3 still give the forward view)
func forecastToday(startMonth string, clientNow time.Time) (time.Time, error) {
	month, err := time.Parse("2006-01", startMonth)

	if err != nil {
		return time.Time{}, err
	}

	year, mon, day := clientNow.Date()

	if month.Year() != year || month.Month() != mon {
		return time.Time{}, errs.ErrParameterInvalid
	}

	return forecast.D(year, mon, day), nil
}
