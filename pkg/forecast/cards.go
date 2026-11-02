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

// CardBills turns card charges into repayment transfers on each due date before `to`.
//
// history is the real transactions up to today (expenses on the card and Credit Card Repayment
// transfers into it). The last statement S0 is the latest statement date strictly before today: on a
// statement day itself, today's statement is treated as still to come, so charges dated today land in
// it. If S0's due day is today or later, the part of Outstanding that predates S0 (Outstanding less the
// card's real expenses dated after S0, floored at 0) is billed on that due day, unless Credit Card
// Repayment transfers into the card after S0 already total at least 90% of that statement. Everything
// else (spend after S0, any credit balance) carries into the next statement. A cycle whose amount is
// not positive emits no bill and carries the amount (e.g. a credit balance) into the next cycle.
func CardBills(cards []Card, items []Item, history []Txn, today, to time.Time) (bills []Item, skipped []int64) {
	for _, c := range cards {
		if c.DueDay < 1 || c.StatementDay < 1 || c.PayFromAccountId == 0 {
			skipped = append(skipped, c.AccountId)
			continue
		}
		bill := func(date time.Time, amount int64) {
			bills = append(bills, Item{Date: date, Kind: KindTransfer, AccountId: c.PayFromAccountId, DestinationAccountId: c.AccountId,
				Amount: amount, Source: SourceCardBill, SourceId: c.AccountId, Label: c.Label + " bill"})
		}

		s0 := statementOnOrAfter(today, c.StatementDay).AddDate(0, -1, 0)
		carry := c.Outstanding
		if due0 := dayAfter(s0, c.DueDay); !due0.Before(today) && due0.Before(to) {
			var spentSince, repaidSince int64
			for _, tx := range history {
				if !tx.Date.After(s0) || tx.Date.After(today) {
					continue
				}
				if tx.Kind == KindExpense && tx.AccountId == c.AccountId {
					spentSince += tx.Amount
				} else if tx.Kind == KindTransfer && tx.CardRepayment && tx.DestinationAccountId == c.AccountId {
					repaidSince += tx.Amount
				}
			}
			unpaid := max(c.Outstanding-spentSince, 0)
			statement := unpaid + repaidSince // what S0 billed, before the repayments since
			if unpaid > 0 && float64(repaidSince) < billPaidShare*float64(statement) {
				bill(due0, unpaid)
				carry -= unpaid
			}
		}

		prev := s0 // first window (S0, S1] includes today
		for s := statementOnOrAfter(today, c.StatementDay); ; s = statementOnOrAfter(s.AddDate(0, 0, 1), c.StatementDay) {
			due := dayAfter(s, c.DueDay)
			if !due.Before(to) {
				break
			}
			amount := carry
			for _, it := range items {
				if it.Kind == KindExpense && it.AccountId == c.AccountId && it.Date.After(prev) && !it.Date.After(s) {
					amount += it.Amount
				}
			}
			prev = s
			if amount <= 0 {
				carry = amount // a credit balance offsets the next cycle
				continue
			}
			carry = 0
			bill(due, amount)
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
