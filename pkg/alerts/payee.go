package alerts

import (
	"regexp"
	"strings"
)

var narrationPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^UPI-([^-]+)-`),
	regexp.MustCompile(`^IMPS-\d+-([^-]+)-`),
	regexp.MustCompile(`^(?:NEFT|RTGS) (?:CR|DR)-[A-Z0-9]+-([^-]+)`),
	regexp.MustCompile(`^\d+-TPT-[^-]*-([^-]+)$`),
	regexp.MustCompile(`^ACH [CD]- ?([^-]+)`),
	regexp.MustCompile(`^(?:POS|ME DC SI) [\dX]+ ([A-Z .&*]+)`),
}

var reNotName = regexp.MustCompile(`\d|^(INTEREST|SWEEP|INT\.|MONTHLY|EMI|CC |ATW|NWD|SMS )`)

// PayeeKey normalises a counterparty so that the SMS form ("JOHN DOE") and the statement narration
// form ("UPI-JOHN  DOE-...") of the same payee produce the same key
func PayeeKey(text string) string {
	u := strings.ToUpper(strings.TrimSpace(text))

	for _, p := range narrationPatterns {
		if m := p.FindStringSubmatch(u); m != nil {
			u = m[1]
			break
		}
	}

	u = strings.Join(strings.Fields(u), " ")

	if u == "" || reNotName.MatchString(u) {
		return ""
	}

	return u
}
