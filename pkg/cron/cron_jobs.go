package cron

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

// RemoveExpiredTokensJob represents the cron job which periodically remove expired user tokens from the database
var RemoveExpiredTokensJob = &CronJob{
	Name:        "RemoveExpiredTokens",
	Description: "Periodically remove expired user tokens from the database.",
	Period: CronJobFixedHourPeriod{
		Hour: 0,
	},
	Run: func(c *core.CronContext) error {
		return services.Tokens.DeleteAllExpiredTokens(c)
	},
}

// CreateScheduledTransactionJob represents the cron job which periodically create transaction by scheduled transaction template
var CreateScheduledTransactionJob = &CronJob{
	Name:        "CreateScheduledTransaction",
	Description: "Periodically create transaction by scheduled transaction template.",
	Period: CronJobEvery15MinutesPeriod{
		Second: 0,
	},
	Run: func(c *core.CronContext) error {
		return services.Transactions.CreateScheduledTransactions(c, time.Now().Unix(), c.GetInterval())
	},
}

// CreateRecurringInvoicesJob represents the cron job which creates the invoices of recurring invoices that are due
var CreateRecurringInvoicesJob = &CronJob{
	Name:        "CreateRecurringInvoices",
	Description: "Periodically create invoices from recurring invoices whose next date has been reached.",
	Period: CronJobFixedHourPeriod{
		Hour: 1,
	},
	Run: func(c *core.CronContext) error {
		// the server's local date, so a user's invoice dated the 15th is created in the early hours of the 15th
		return services.Invoices.CreateRecurringInvoices(c, time.Now().Format("2006-01-02"))
	},
}
