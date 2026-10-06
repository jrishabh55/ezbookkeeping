// Package alerts turns bank / card transaction SMS into structured alerts and classifies them.
package alerts

import (
	"math"
	"regexp"
	"strconv"
	"strings"
)

type Outcome string

const (
	OutcomeParsed   Outcome = "parsed"
	OutcomeIgnored  Outcome = "ignored"
	OutcomeUnparsed Outcome = "unparsed"
)

type Direction string

const (
	Debit  Direction = "debit"
	Credit Direction = "credit"
)

// ParsedAlert is one bank / card SMS reduced to the fields a transaction needs
type ParsedAlert struct {
	Outcome      Outcome
	Direction    Direction
	Amount       int64
	Last4        string
	Counterparty string
	Reference    string
	Balance      int64
	HasBalance   bool
}

var (
	reIgnore  = regexp.MustCompile(`(?i)\b(otp|one time password|cashback|offer|apply now|is due|due on|mandate|will be debited|request(ed)? (money|payment)|limit (has been )?(increased|enhanced)|statement (is|has been) generated)\b`)
	reAmount  = regexp.MustCompile(`(?i)(?:rs\.?|inr)\s*([\d,]+(?:\.\d{1,2})?)`)
	reDebit   = regexp.MustCompile(`(?i)\b(debited|sent|spent|withdrawn|paid|purchase|transferred to|dr\b)`)
	reCredit  = regexp.MustCompile(`(?i)\b(credited|deposited|received|refund(ed)?|cr\b)`)
	reLast4   = regexp.MustCompile(`(?i)(?:a/?c|acct|account|card)(?:\s+(?:no\.?|number|ending(?:\s+with)?))?\s*[:\-]?\s*(?:[x*]+\s*)?(\d{4})\b`)
	reRef     = regexp.MustCompile(`(?i)\b(?:ref(?:erence)?(?:\s*no\.?)?|rrn|upi(?:\s*ref)?|imps(?:\s*ref)?)\s*[:\-]?\s*(\d{6,})`)
	reBalance = regexp.MustCompile(`(?i)\b(?:avl\.?|avail(?:able)?)?\s*bal(?:ance)?\s*(?:is)?\s*[:\-]?\s*(?:rs\.?|inr)\s*([\d,]+(?:\.\d{1,2})?)`)
	reParty   = regexp.MustCompile(`(?i)(?:\bto\b|\bat\b|\bfor\s+(?:neft|imps|rtgs|upi)\s+cr-|\bfrom\b|\bby\b)\s*:?\s*([A-Z][A-Z0-9.&' ]{2,40}?)\s*(?:\n|\.(?:\s|$)| on\b| ref\b| via\b| upi\b|,|$)`)
)

// Parse reads one SMS; it never guesses: anything financial it cannot read fully is OutcomeUnparsed
func Parse(sender string, text string) ParsedAlert {
	if reIgnore.MatchString(text) {
		return ParsedAlert{Outcome: OutcomeIgnored}
	}

	amountMatch := reAmount.FindStringSubmatch(text)

	if amountMatch == nil {
		return ParsedAlert{Outcome: OutcomeIgnored}
	}

	isDebit, isCredit := reDebit.MatchString(text), reCredit.MatchString(text)

	if isDebit == isCredit {
		return ParsedAlert{Outcome: OutcomeUnparsed}
	}

	amount, ok := parseRupees(amountMatch[1])

	if !ok || amount <= 0 {
		return ParsedAlert{Outcome: OutcomeUnparsed}
	}

	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: amount}

	if isCredit {
		alert.Direction = Credit
	}

	if m := reLast4.FindStringSubmatch(text); m != nil {
		alert.Last4 = m[1]
	}

	if m := reRef.FindStringSubmatch(text); m != nil {
		alert.Reference = m[1]
	}

	if m := reBalance.FindStringSubmatch(text); m != nil && m[0] != amountMatch[0] {
		if b, ok := parseRupees(m[1]); ok {
			alert.Balance, alert.HasBalance = b, true
		}
	}

	if m := reParty.FindStringSubmatch(text); m != nil {
		alert.Counterparty = strings.ToUpper(strings.TrimSpace(m[1]))
	}

	if alert.Last4 == "" {
		return ParsedAlert{Outcome: OutcomeUnparsed}
	}

	return alert
}

// parseRupees reads "1,23,456.78" / "500" / "500.5" into paise
func parseRupees(s string) (int64, bool) {
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)

	if err != nil {
		return 0, false
	}

	return int64(math.Round(v * 100)), true
}
