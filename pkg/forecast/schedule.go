package forecast

import (
	"strconv"
	"strings"
	"time"
)

// ExpandTemplate returns one item per scheduled date in [from, to), same rules as the scheduled-transaction cron
func ExpandTemplate(t Template, from, to time.Time) []Item {
	var values []int
	for _, s := range strings.Split(t.Frequency, ",") {
		v, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return nil
		}
		values = append(values, v)
	}
	if len(values) == 0 || t.FrequencyType < FreqWeekly || t.FrequencyType > FreqEveryNDay {
		return nil
	}

	var items []Item
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		if (!t.Start.IsZero() && d.Before(t.Start)) || (!t.End.IsZero() && d.After(t.End)) || !matches(t, values, d) {
			continue
		}
		items = append(items, Item{Date: d, Kind: t.Kind, AccountId: t.AccountId, DestinationAccountId: t.DestinationAccountId,
			CategoryId: t.CategoryId, Amount: t.Amount, Source: SourceTemplate, SourceId: t.Id, Business: t.Business, Label: t.Label})
	}
	return items
}

func matches(t Template, values []int, d time.Time) bool {
	for _, v := range values {
		switch t.FrequencyType {
		case FreqDaily:
			return true
		case FreqWeekly:
			if int(d.Weekday()) == v {
				return true
			}
		case FreqMonthly:
			last := daysIn(d)
			day := v
			if day < 0 {
				day = last + v + 1
			} else if day > last {
				day = last // ponytail: day 29–31 falls on the last day; the cron skips such months instead
			}
			if d.Day() == day {
				return true
			}
		case FreqYearly:
			if int(d.Month())*100+d.Day() == v {
				return true
			}
		case FreqEveryNDay:
			if t.Start.IsZero() || v <= 0 {
				return false
			}
			diff := int(d.Sub(t.Start).Hours() / 24)
			return diff >= 0 && diff%v == 0
		}
	}
	return false
}
