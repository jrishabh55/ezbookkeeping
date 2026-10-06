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
	// reHardIgnore matches messages that are never a completed transaction, regardless of
	// whether they otherwise look like one (OTP prompts, mandate setup, future-tense
	// reminders, collect requests, limit/statement notices).
	reHardIgnore = regexp.MustCompile(`(?i)\botp\b|\bone time password\b|\be-mandate\b|\bmandate\b|\bis due\b|\bdue on\b|\bwill be debited\b|\blimit (?:has been )?(?:increased|enhanced)\b|\btransaction limit\b|\b(?:payment|money)\s+request\b|\brequest(?:ed)?\s+(?:money|payment)\b|\bstatement\s*:`)
	// reSoftIgnore matches promo-ish words that only mean "ignore" when the message does
	// not otherwise look like a real transaction (debit/credit verb + amount + last-4).
	reSoftIgnore = regexp.MustCompile(`(?i)\bcashback\b|\boffer\b|\bapply now\b`)
	// reTxnWord flags a message as "clearly about a transaction" even when neither a debit
	// nor a credit verb was found, so such a message is reported unparsed rather than
	// silently dropped.
	reTxnWord = regexp.MustCompile(`(?i)\btxn\b|\btransaction\b`)
	reAmount  = regexp.MustCompile(`(?i)\b(?:rs\.?|inr)\s*(\d[\d,]*(?:\.\d{1,2})?)`)
	// Bare "dr"/"cr" markers were dropped: they false-positive too easily (e.g. "Rs. 3,989
	// cr." meaning crore, not credit) and every fixture that needs a direction already
	// spells out debited/credited/sent/spent/etc.
	reDebit   = regexp.MustCompile(`(?i)\b(debited|sent|spent|withdrawn|paid|purchase|transferred to)`)
	reCredit  = regexp.MustCompile(`(?i)\b(credited|deposited|received|refund(ed)?|posted)`)
	reLast4   = regexp.MustCompile(`(?i)(?:a/?c|acct|account|acc\b|card)(?:\s+(?:no\.?|number|ending(?:\s+with)?))?\s*[:\-]?\s*(?:[x*]+[\s\-]*)?(\d{4})\b`)
	reRef     = regexp.MustCompile(`(?i)\b(?:ref(?:erence)?(?:\s*no\.?)?|rrn|upi(?:\s*ref)?|imps(?:\s*ref)?)\s*[:\-]?\s*(\d{6,})`)
	reBalance = regexp.MustCompile(`(?i)\b(?:avl\.?|avail(?:able)?)?\s*bal(?:ance)?\s*(?:is)?\s*[:\-]?\s*(?:rs\.?|inr)\s*(\d[\d,]*(?:\.\d{1,2})?)`)
	// The capture itself is deliberately case-sensitive (?-i: ...): every genuine name in
	// these SMS (merchant, payee, bank name) is written in ALL CAPS, and turning case
	// sensitivity back on for just this group stops it from swallowing lowercase filler
	// words (e.g. "to block", "subject to clearing") as a false counterparty.
	reParty = regexp.MustCompile(`(?i)(?:\bto\b|\bat\b|\bfor\s+(?:neft|imps|rtgs|upi)\s+cr-|\bfrom\b|\bby\b)\s*:?\s*((?-i:[A-Z][A-Z0-9.&' ]{2,40}?))\s*(?:\n|\.(?:\s|$)| on\b| ref\b| via\b| upi\b|\b(?:a/?c|acc(?:t|ount)?|card)\b| \d{1,2}/\d{1,2}/\d{2,4}\b|,|$)`)
	// reRejectWord recognises text that starts (ignoring leading whitespace) with an account
	// keyword or a currency marker; used to reject a party-regex match that actually names
	// the account itself ("to Ac XX7664", "from VALLEY PHARMACY Acc XX6282") or restates the
	// amount ("debited by INR 214.66") rather than naming a counterparty.
	reRejectWord = regexp.MustCompile(`(?i)^\s*(?:a/?c|acc(?:t|ount)?|card|rs\.?|inr)\b`)
)

// Parse reads one SMS; it never guesses: anything financial it cannot read fully is OutcomeUnparsed
func Parse(sender string, text string) ParsedAlert {
	if reHardIgnore.MatchString(text) {
		return ParsedAlert{Outcome: OutcomeIgnored}
	}

	amountMatch := reAmount.FindStringSubmatch(text)

	if amountMatch == nil {
		return ParsedAlert{Outcome: OutcomeIgnored}
	}

	isDebit, isCredit := reDebit.MatchString(text), reCredit.MatchString(text)

	last4 := ""
	if m := reLast4.FindStringSubmatch(text); m != nil {
		last4 = m[1]
	}

	if isDebit && isCredit {
		// Both verbs present (e.g. a same-message transfer between two accounts): genuinely
		// ambiguous, can't be safely reduced to one direction/one amount/one account. This
		// check must run before the soft-ignore check below: a dual-verb message is never
		// silently dropped just because it also contains a promo word.
		return ParsedAlert{Outcome: OutcomeUnparsed}
	}

	// A message that has a clear single direction and an identifiable account/card is a
	// transaction even if it also contains a promo word (e.g. a cashback credit, or a card
	// spend at a merchant whose name happens to contain "offer").
	looksLikeTxn := isDebit != isCredit && last4 != ""

	if !looksLikeTxn && reSoftIgnore.MatchString(text) {
		return ParsedAlert{Outcome: OutcomeIgnored}
	}

	if !isDebit && !isCredit {
		// No direction verb at all: only report unparsed when the message explicitly frames
		// the amount as a transaction; otherwise it's informational (balance update, reward
		// expiry, etc.) and should be ignored, not surfaced as unreadable.
		if reTxnWord.MatchString(text) {
			return ParsedAlert{Outcome: OutcomeUnparsed}
		}
		return ParsedAlert{Outcome: OutcomeIgnored}
	}

	amount, ok := parseRupees(amountMatch[1])

	if !ok || amount <= 0 {
		return ParsedAlert{Outcome: OutcomeUnparsed}
	}

	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: amount, Last4: last4}

	if isCredit {
		alert.Direction = Credit
	}

	if m := reRef.FindStringSubmatch(text); m != nil {
		alert.Reference = m[1]
	}

	if m := reBalance.FindStringSubmatch(text); m != nil && m[0] != amountMatch[0] {
		if b, ok := parseRupees(m[1]); ok {
			alert.Balance, alert.HasBalance = b, true
		}
	}

	alert.Counterparty = extractParty(text)

	if alert.Last4 == "" {
		return ParsedAlert{Outcome: OutcomeUnparsed}
	}

	return alert
}

// extractParty returns the first reParty match that isn't actually naming the account/card or
// restating the amount (see reRejectWord): a match is rejected if the captured text starts
// with a reject word ("Ac XX7664", "INR 214.66"), or if a reject word immediately follows it
// ("VALLEY PHARMACY Acc XX6282"), since in both cases it isn't a genuine counterparty.
func extractParty(text string) string {
	for _, loc := range reParty.FindAllStringSubmatchIndex(text, -1) {
		name, after := text[loc[2]:loc[3]], text[loc[3]:]

		if reRejectWord.MatchString(name) || reRejectWord.MatchString(after) {
			continue
		}

		return strings.ToUpper(strings.TrimSpace(name))
	}

	return ""
}

// parseRupees reads "1,23,456.78" / "500" / "500.5" into paise
func parseRupees(s string) (int64, bool) {
	v, err := strconv.ParseFloat(strings.ReplaceAll(s, ",", ""), 64)

	if err != nil {
		return 0, false
	}

	return int64(math.Round(v * 100)), true
}
