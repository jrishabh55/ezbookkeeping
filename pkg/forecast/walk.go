package forecast

import (
	"sort"
	"time"
)

type Account struct {
	Id              int64
	Balance         int64
	Cash, Liability bool
	Name            string
}

type Month struct {
	Month                                                 time.Time
	OpeningCash, ClosingCash, ClosingCashWithoutUncertain int64
	BusinessIncome, BusinessCost, PersonalCost            int64
}

type Shortfall struct {
	AccountId     int64
	Date          time.Time
	LowestBalance int64
	LowestDate    time.Time
}

// Walk applies items in date order and summarises each month starting at from's month
func Walk(accounts []Account, items []Item, from time.Time, months int) ([]Month, []Shortfall) {
	sorted := append([]Item(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Date.Before(sorted[j].Date) })

	byId := map[int64]*Account{}
	bal, balSure := map[int64]int64{}, map[int64]int64{}
	for i := range accounts {
		a := &accounts[i]
		byId[a.Id] = a
		bal[a.Id], balSure[a.Id] = a.Balance, a.Balance
	}
	cash := func(m map[int64]int64) (total int64) {
		for _, a := range accounts {
			if a.Cash {
				total += m[a.Id]
			}
		}
		return
	}
	apply := func(m map[int64]int64, id int64, delta int64) {
		if a, ok := byId[id]; ok && a.Liability {
			m[id] -= delta
		} else {
			m[id] += delta
		}
	}

	sf := map[int64]*Shortfall{}
	var out []Month
	idx := 0
	for k := 0; k < months; k++ {
		start := MonthStart(from).AddDate(0, k, 0)
		end := start.AddDate(0, 1, 0)
		m := Month{Month: start, OpeningCash: cash(bal)}
		for ; idx < len(sorted) && sorted[idx].Date.Before(end); idx++ {
			it := sorted[idx]
			for i, mm := range []map[int64]int64{bal, balSure} {
				if i == 1 && it.Uncertain {
					continue
				}
				switch it.Kind {
				case KindExpense:
					apply(mm, it.AccountId, -it.Amount)
				case KindIncome:
					apply(mm, it.AccountId, it.Amount)
				case KindTransfer:
					apply(mm, it.AccountId, -it.Amount)
					apply(mm, it.DestinationAccountId, it.Amount)
				}
			}
			switch {
			case it.Kind == KindIncome && it.Business:
				m.BusinessIncome += it.Amount
			case it.Kind == KindExpense && it.Business:
				m.BusinessCost += it.Amount
			case it.Kind == KindExpense:
				m.PersonalCost += it.Amount
			}
			for _, id := range []int64{it.AccountId, it.DestinationAccountId} {
				a, ok := byId[id]
				if !ok || !a.Cash || a.Balance < 0 || balSure[id] >= 0 {
					continue
				}
				if s := sf[id]; s == nil {
					sf[id] = &Shortfall{AccountId: id, Date: it.Date, LowestBalance: balSure[id], LowestDate: it.Date}
				} else if balSure[id] < s.LowestBalance {
					s.LowestBalance, s.LowestDate = balSure[id], it.Date
				}
			}
		}
		m.ClosingCash, m.ClosingCashWithoutUncertain = cash(bal), cash(balSure)
		out = append(out, m)
	}

	var shortfalls []Shortfall
	for _, s := range sf {
		shortfalls = append(shortfalls, *s)
	}
	sort.Slice(shortfalls, func(i, j int) bool { return shortfalls[i].Date.Before(shortfalls[j].Date) })
	return out, shortfalls
}
