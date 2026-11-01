package forecast

import "time"

// Card is a credit card with its billing cycle
type Card struct {
	AccountId, PayFromAccountId int64
	StatementDay, DueDay        int
	Outstanding                 int64
	Label                       string
}

const billPaidShare = 0.9

// CardBills turns card charges into repayment transfers on each due date before `to`
func CardBills(cards []Card, items []Item, repayments []Txn, today, to time.Time) (bills []Item, skipped []int64) {
	for _, c := range cards {
		if c.DueDay < 1 || c.StatementDay < 1 || c.PayFromAccountId == 0 {
			skipped = append(skipped, c.AccountId)
			continue
		}
		prev := today
		carry := c.Outstanding
		for s := statementOnOrAfter(today, c.StatementDay); ; s = statementOnOrAfter(s.AddDate(0, 0, 1), c.StatementDay) {
			due := dayAfter(s, c.DueDay)
			if !due.Before(to) {
				break
			}
			amount := carry
			carry = 0
			for _, it := range items {
				if it.Kind == KindExpense && it.AccountId == c.AccountId && it.Date.After(prev) && !it.Date.After(s) {
					amount += it.Amount
				}
			}
			prev = s
			if amount <= 0 || repaid(repayments, c.AccountId, s, due, amount) {
				continue
			}
			bills = append(bills, Item{Date: due, Kind: KindTransfer, AccountId: c.PayFromAccountId, DestinationAccountId: c.AccountId,
				Amount: amount, Source: SourceCardBill, SourceId: c.AccountId, Label: c.Label + " bill"})
		}
	}
	return bills, skipped
}

func statementOnOrAfter(t time.Time, day int) time.Time {
	s := D(t.Year(), t.Month(), day)
	if s.Before(t) {
		s = s.AddDate(0, 1, 0)
	}
	return s
}

func dayAfter(s time.Time, day int) time.Time {
	d := D(s.Year(), s.Month(), day)
	if !d.After(s) {
		d = d.AddDate(0, 1, 0)
	}
	return d
}

func repaid(repayments []Txn, card int64, from, to time.Time, amount int64) bool {
	for _, r := range repayments {
		if r.DestinationAccountId == card && r.Date.After(from) && !r.Date.After(to) && float64(r.Amount) >= billPaidShare*float64(amount) {
			return true
		}
	}
	return false
}
