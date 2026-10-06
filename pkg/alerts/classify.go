package alerts

import (
	"regexp"
	"strings"
)

type OwnAccount struct {
	ID    int64
	Name  string
	Last4 string
}

type HistoryHit struct {
	IsTransfer     bool
	CategoryId     int64
	OtherAccountId int64
}

type Classification struct {
	IsTransfer     bool
	CategoryId     int64
	OtherAccountId int64
	NeedsReview    bool
}

type KeywordRule struct {
	Pattern    *regexp.Regexp
	Direction  Direction
	CategoryId int64
}

var reAnyLast4 = regexp.MustCompile(`(?:X|\*){2,}\s*(\d{4})\b`)

// Classify decides category or transfer target: own account by last-4, then payee history, then keywords, then review
func Classify(alert ParsedAlert, accountId int64, own []OwnAccount, history func(payeeKey string) (HistoryHit, bool), keywords []KeywordRule, textUpper string) Classification {
	for _, m := range reAnyLast4.FindAllStringSubmatch(textUpper, -1) {
		for _, a := range own {
			if a.Last4 == m[1] && a.ID != accountId {
				return Classification{IsTransfer: true, OtherAccountId: a.ID}
			}
		}
	}

	if key := PayeeKey(alert.Counterparty); key != "" {
		if hit, ok := history(key); ok {
			if hit.IsTransfer && hit.OtherAccountId != accountId {
				return Classification{IsTransfer: true, OtherAccountId: hit.OtherAccountId}
			}
			if !hit.IsTransfer {
				return Classification{CategoryId: hit.CategoryId}
			}
		}
	}

	for _, k := range keywords {
		if k.Direction == alert.Direction && k.Pattern.MatchString(strings.ToUpper(alert.Counterparty+" "+textUpper)) {
			return Classification{CategoryId: k.CategoryId}
		}
	}

	return Classification{NeedsReview: true}
}
