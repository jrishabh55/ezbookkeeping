package api

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/forecast"
)

func TestForecastToday_CurrentMonthGivesClientLocalDate(t *testing.T) {
	ist := time.FixedZone("IST", 330*60)
	now := time.Date(2026, 11, 1, 0, 30, 0, 0, ist) // still 31 Oct in UTC

	today, err := forecastToday("2026-11", now)

	assert.Nil(t, err)
	assert.Equal(t, forecast.D(2026, 11, 1), today)
}

func TestForecastToday_OtherMonthsRejected(t *testing.T) {
	now := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

	for _, month := range []string{"2026-10", "2026-12", "2025-11"} {
		_, err := forecastToday(month, now)
		assert.Equal(t, errs.ErrParameterInvalid, err, month)
	}

	_, err := forecastToday("Nov 2026", now)
	assert.NotNil(t, err)
}
