package alerts

import (
	"regexp"
	"strings"
)

var narrationPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^UPI-(.+?)-[^-]*@`),
	regexp.MustCompile(`^UPI-([^-]+)-`),
	regexp.MustCompile(`^IMPS-\d+-([^-]+)-`),
	regexp.MustCompile(`^(?:NEFT|RTGS) (?:CR|DR)-[A-Z0-9]+-([^-]+)`),
	regexp.MustCompile(`^\d+-TPT-[^-]*-([^-]+)$`),
	regexp.MustCompile(`^ACH [CD]- ?([^-]+)`),
	regexp.MustCompile(`^(?:POS|ME DC SI) [\dX]+ ([A-Z0-9 .&*]+)`),
}

var reFeeWords = regexp.MustCompile(`\b(?:INTEREST|SWEEP|INT\.?|MONTHLY|EMI|CC|ATW|NWD|SMS|CHARGES?|CHGS?|FEES?|GST|AMC|ATM|WDL|WITHDRAWAL|PENALTY|CASHBACK)\b`)
var reDigit = regexp.MustCompile(`\d`)

// PayeeKey normalises a counterparty so that the SMS form ("JOHN DOE") and the statement narration
// form ("UPI-JOHN  DOE-...") of the same payee produce the same key
func PayeeKey(text string) string {
	u := strings.ToUpper(strings.TrimSpace(text))

	patternMatched := false
	for _, p := range narrationPatterns {
		if m := p.FindStringSubmatch(u); m != nil {
			u = m[1]
			patternMatched = true
			break
		}
	}

	u = strings.Join(strings.Fields(u), " ")

	if u == "" {
		return ""
	}

	// Always reject fee-related patterns
	if reFeeWords.MatchString(u) {
		return ""
	}

	// Only check for digits if no narration pattern matched
	if !patternMatched && reDigit.MatchString(u) {
		return ""
	}

	return u
}
