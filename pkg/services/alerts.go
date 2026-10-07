package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mayswind/ezbookkeeping/pkg/alerts"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// Tag names created on demand for alert-ingested transactions
const (
	tagAutoSms            = "Auto (SMS)"
	tagNeedsReview        = "Needs review"
	tagBalanceMismatch    = "Balance mismatch"
	tagCardPaymentPending = "Card payment pending"
	tagStatementVerified  = "Statement verified"
	tagNotInStatement     = "Not in statement"
)

// reconcileDateWindowSeconds is the ±2 days tolerance (§8 of the design doc) between a
// statement row's date and a candidate "Auto (SMS)" transaction's date
const reconcileDateWindowSeconds = 2 * 24 * 60 * 60

// reReferenceRun matches any run of digits in a statement row's narration; a reference is only
// recognised when a run is *exactly* 12 digits long (a 12-digit reference embedded in a longer
// run of digits, e.g. inside a 16-digit card number, must never match)
var reReferenceRun = regexp.MustCompile(`\d+`)

// extractReferences returns every run of exactly 12 digits in text (UPI / IMPS / RRN references)
func extractReferences(text string) []string {
	var references []string

	for _, run := range reReferenceRun.FindAllString(text, -1) {
		if len(run) == 12 {
			references = append(references, run)
		}
	}

	return references
}

// reconcileOwnerId returns the id that carries an SMS transaction's tags and alert row: the
// transfer-out side for a transfer, the transaction itself otherwise
func reconcileOwnerId(transaction *models.Transaction) int64 {
	if transaction.Type == models.TRANSACTION_DB_TYPE_TRANSFER_IN {
		return transaction.RelatedId
	}

	return transaction.TransactionId
}

// reconcileTypeMatches reports whether a statement row type can be the given book transaction:
// a statement only knows money out / money in, while the book may hold either side of a transfer
func reconcileTypeMatches(rowType models.TransactionDbType, transactionType models.TransactionDbType) bool {
	switch rowType {
	case models.TRANSACTION_DB_TYPE_EXPENSE, models.TRANSACTION_DB_TYPE_TRANSFER_OUT:
		return transactionType == models.TRANSACTION_DB_TYPE_EXPENSE || transactionType == models.TRANSACTION_DB_TYPE_TRANSFER_OUT
	case models.TRANSACTION_DB_TYPE_INCOME, models.TRANSACTION_DB_TYPE_TRANSFER_IN:
		return transactionType == models.TRANSACTION_DB_TYPE_INCOME || transactionType == models.TRANSACTION_DB_TYPE_TRANSFER_IN
	}

	return false
}

// unmatchedAccountName is the name of the per-user fallback account used when an alert's
// account cannot be resolved from its last-4 digits
const unmatchedAccountName = "Unmatched alerts"

// errUnmatchedAccountHidden is returned by resolveAccount when the alert's account can't be
// resolved and this user's "Unmatched alerts" account exists but is hidden (so no transaction can
// be created in it); Ingest turns it into an "unparsed" outcome, keeping the row for the user
var errUnmatchedAccountHidden = errors.New("unmatched alerts account is hidden")

// repeatedAlertWindowSeconds is how close in time an identical ignored / unparsed message from the
// same sender must be to be treated as the same SMS posted again (e.g. by both the "Rs" and the
// "INR" automations)
const repeatedAlertWindowSeconds = 120

// alertTextHash returns a short sha256 prefix of an SMS body, stored so a repeat of an ignored
// message can be recognised without keeping its text
func alertTextHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])[:16]
}

// truncateComment truncates a comment to the 255 runes a transaction comment can hold, by rune
// rather than by byte so a multi-byte character is never split
func truncateComment(comment string) string {
	if utf8.RuneCountInString(comment) > 255 {
		return utils.SubString(comment, 0, 255)
	}

	return comment
}

// Fallback category names looked up (and created on first use) when classification does not
// resolve a usable category; the transfer names are tried in priority order
const (
	categoryNameBankTransfer  = "Bank Transfer"
	categoryNameCardRepayment = "Credit Card Repayment"
	categoryNameOtherTransfer = "Other Transfer"
	categoryNameOtherExpense  = "Other Expense"
	categoryNameOtherIncome   = "Other Income"
)

// reAccountNameLast4 extracts the trailing 4 digits of an account name, used both to build the
// "own accounts" list passed to alerts.Classify and to resolve an alert's account by its last-4
// digits; the digits must be preceded by a non-digit or start of string, so "...51234" is never
// mistaken for last-4 "1234"
var reAccountNameLast4 = regexp.MustCompile(`(?:^|\D)(\d{4})$`)

// keywordCategoryRule is a KeywordRule before its category name has been resolved to this
// user's category id
type keywordCategoryRule struct {
	pattern      *regexp.Regexp
	direction    alerts.Direction
	categoryName string
}

// keywordCategoryRules is the small set of merchant keyword rules used for step 8
// classification when neither an own-account transfer nor payee history resolves the alert
var keywordCategoryRules = []keywordCategoryRule{
	{regexp.MustCompile(`(?i)\b(SWIGGY|ZOMATO|UBER ?EATS|DOMINOS?|PIZZA)\b`), alerts.Debit, "Food"},
	{regexp.MustCompile(`(?i)\b(AIRTEL|JIO|VODAFONE|VI|BSNL)\b`), alerts.Debit, "Telephone Bill"},
	{regexp.MustCompile(`(?i)\b(PETROL|FUEL|HPCL|BPCL|INDIAN ?OIL|IOCL|SHELL)\b`), alerts.Debit, "Personal Car Expense"},
	{regexp.MustCompile(`(?i)\b(NETFLIX|SPOTIFY|PRIME ?VIDEO|HOTSTAR|YOUTUBE ?PREMIUM|APPLE ?MUSIC)\b`), alerts.Debit, "Subscriptions"},
	{regexp.MustCompile(`(?i)\b(PVR|INOX|BOOKMYSHOW|CINEPOLIS)\b`), alerts.Debit, "Movies & Shows"},
}

// ponytail: global lock, per-uid locks if throughput matters
var ingestMu sync.Mutex

// AlertService represents alert service
type AlertService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

// Initialize an alert service singleton instance
var (
	Alerts = &AlertService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// AlertIngestResult represents the result of ingesting one alert SMS
type AlertIngestResult struct {
	Outcome       string
	Summary       string
	TransactionId int64
}

// Ingest turns one bank / card SMS into at most one transaction in this user's book
func (s *AlertService) Ingest(c core.Context, uid int64, sender string, text string, receivedAt time.Time) (*AlertIngestResult, error) {
	ingestMu.Lock()
	defer ingestMu.Unlock()

	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	parsed := alerts.Parse(sender, text)
	textHash := alertTextHash(text)

	if parsed.Outcome == alerts.OutcomeIgnored || parsed.Outcome == alerts.OutcomeUnparsed {
		if isRepeated, err := s.isRepeatedMessage(c, uid, sender, textHash, string(parsed.Outcome), receivedAt); err != nil {
			return nil, err
		} else if isRepeated {
			return &AlertIngestResult{Outcome: string(parsed.Outcome)}, nil
		}
	}

	message, err := s.storeMessage(c, uid, sender, text, textHash, receivedAt, parsed)

	if err != nil {
		return nil, err
	}

	if parsed.Outcome == alerts.OutcomeIgnored {
		return &AlertIngestResult{Outcome: "ignored"}, nil
	}

	if parsed.Outcome == alerts.OutcomeUnparsed {
		return &AlertIngestResult{Outcome: "unparsed"}, nil
	}

	if isDuplicate, err := s.isDuplicateReference(c, uid, parsed); err != nil {
		return nil, s.failAfterStore(c, uid, message.AlertId, err)
	} else if isDuplicate {
		_ = s.updateMessageOutcome(c, uid, message.AlertId, "duplicate", 0)
		return &AlertIngestResult{Outcome: "duplicate"}, nil
	}

	// A reference lets step 4 (above) resolve duplicates precisely; running the amount/direction
	// heuristic as well would risk dropping a second, genuinely distinct payment that happens to
	// match on amount within the same window, so it only runs when there is no reference at all.
	if parsed.Reference == "" {
		if isDuplicate, err := s.isRecentDuplicate(c, uid, parsed, receivedAt); err != nil {
			return nil, s.failAfterStore(c, uid, message.AlertId, err)
		} else if isDuplicate {
			_ = s.updateMessageOutcome(c, uid, message.AlertId, "duplicate", 0)
			return &AlertIngestResult{Outcome: "duplicate"}, nil
		}
	}

	account, forcedReview, err := s.resolveAccount(c, uid, parsed)

	if errors.Is(err, errUnmatchedAccountHidden) {
		log.Warnf(c, "[alerts.Ingest] cannot record alert for user \"uid:%d\", because its account is unknown and the \"%s\" account is hidden", uid, unmatchedAccountName)
		_ = s.updateMessageOutcome(c, uid, message.AlertId, string(alerts.OutcomeUnparsed), 0)
		return &AlertIngestResult{Outcome: string(alerts.OutcomeUnparsed)}, nil
	} else if err != nil {
		return nil, s.failAfterStore(c, uid, message.AlertId, err)
	}

	tagIds, hiddenTagIds, err := s.ensureTags(c, uid)

	if err != nil {
		return nil, s.failAfterStore(c, uid, message.AlertId, err)
	}

	classification, err := s.classify(c, uid, account.AccountId, parsed, strings.ToUpper(text))

	if err != nil {
		return nil, s.failAfterStore(c, uid, message.AlertId, err)
	}

	// A card bill paid through CRED arrives as two SMS that each name only one end: the bank's
	// "to CRED" debit and the card's "payment received" credit. They become one bank → card
	// transfer; whichever arrives first is held as a pending expense / income until its pair does.
	var extraTags []string

	// (a CRED debit is paired even when payee history says otherwise: CRED is never the destination)
	outgoing := alerts.IsFacilitatorPayment(parsed, text)

	if outgoing || (!classification.IsTransfer && alerts.IsCardPaymentReceived(parsed, text) && account.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD) {
		otherAccountId, isDuplicate, err := s.pairCardPayment(c, uid, outgoing, account.AccountId, parsed.Amount, receivedAt, tagIds)

		if err != nil {
			return nil, s.failAfterStore(c, uid, message.AlertId, err)
		} else if isDuplicate {
			_ = s.updateMessageOutcome(c, uid, message.AlertId, "duplicate", 0)
			return &AlertIngestResult{Outcome: "duplicate"}, nil
		} else if otherAccountId > 0 {
			classification = alerts.Classification{IsTransfer: true, OtherAccountId: otherAccountId}
		} else {
			classification = alerts.Classification{NeedsReview: true}
			extraTags = append(extraTags, tagCardPaymentPending)
		}
	}

	if classification.IsTransfer {
		srcAccountId, dstAccountId := account.AccountId, classification.OtherAccountId

		if parsed.Direction == alerts.Credit {
			srcAccountId, dstAccountId = classification.OtherAccountId, account.AccountId
		}

		isDuplicate, err := s.isDuplicateOwnTransfer(c, uid, srcAccountId, dstAccountId, parsed.Amount, receivedAt, tagIds[tagAutoSms])

		if err != nil {
			return nil, s.failAfterStore(c, uid, message.AlertId, err)
		} else if isDuplicate {
			_ = s.updateMessageOutcome(c, uid, message.AlertId, "duplicate", 0)
			return &AlertIngestResult{Outcome: "duplicate"}, nil
		}

		// The other side's SMS may have arrived first without naming this account (e.g. a credit
		// "by IMPS from JOHN DOE") and been recorded as a plain income / expense; replace it
		conflict, err := s.replaceEarlierTransferSide(c, uid, srcAccountId, dstAccountId, parsed.Direction, parsed.Amount, receivedAt, tagIds[tagAutoSms], tagIds[tagNeedsReview])

		if err != nil {
			return nil, s.failAfterStore(c, uid, message.AlertId, err)
		} else if conflict {
			forcedReview = true
		}
	} else if len(extraTags) == 0 {
		// One side of an own-account transfer whose SMS doesn't name the other account (e.g. a
		// credit "from JOHN DOE"): the other side's SMS may already have recorded the transfer.
		isDuplicate, err := s.isDuplicateTransferSide(c, uid, account.AccountId, parsed.Direction, parsed.Amount, receivedAt, tagIds[tagAutoSms])

		if err != nil {
			return nil, s.failAfterStore(c, uid, message.AlertId, err)
		} else if isDuplicate {
			_ = s.updateMessageOutcome(c, uid, message.AlertId, "duplicate", 0)
			return &AlertIngestResult{Outcome: "duplicate"}, nil
		}
	}

	needsReview := forcedReview || classification.NeedsReview

	transaction, summary, err := s.createTransaction(c, uid, account, parsed, classification, needsReview, text, receivedAt, tagIds, hiddenTagIds, extraTags...)

	if err != nil {
		// Server error after parsing: keep the raw message stored, marked as unparsed for review
		return nil, s.failAfterStore(c, uid, message.AlertId, err)
	}

	// The transaction now exists: mark this alert "added" immediately, before the best-effort
	// balance check below, so that a failure in that check can never leave the row behind at its
	// placeholder outcome (which would make a client retry create a second transaction).
	if err := s.updateMessageOutcome(c, uid, message.AlertId, "added", transaction.TransactionId); err != nil {
		return nil, err
	}

	if hiddenTagIds[tagIds[tagBalanceMismatch]] {
		log.Warnf(c, "[alerts.Ingest] skipping balance check for transaction \"id:%d\" of user \"uid:%d\", because the \"%s\" tag is hidden", transaction.TransactionId, uid, tagBalanceMismatch)
	} else if err := s.checkBalance(c, uid, account.AccountId, parsed, transaction, tagIds[tagBalanceMismatch]); err != nil {
		log.Errorf(c, "[alerts.Ingest] failed to tag balance mismatch for transaction \"id:%d\" of user \"uid:%d\", because %s", transaction.TransactionId, uid, err.Error())
	}

	return &AlertIngestResult{Outcome: "added", Summary: summary, TransactionId: transaction.TransactionId}, nil
}

// Status returns this user's alert ingestion counters and the most recent outcome
func (s *AlertService) Status(c core.Context, uid int64) (*models.AlertStatusResponse, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var messages []*models.AlertMessage
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).OrderBy("received_unix_time desc").Find(&messages)

	if err != nil {
		return nil, err
	}

	response := &models.AlertStatusResponse{
		Counts:         make(map[string]int64),
		RecentUnparsed: make([]*models.AlertUnparsedItem, 0),
	}

	for i := 0; i < len(messages); i++ {
		message := messages[i]
		response.Counts[message.Outcome]++

		if message.Outcome == string(alerts.OutcomeUnparsed) && len(response.RecentUnparsed) < 10 {
			response.RecentUnparsed = append(response.RecentUnparsed, &models.AlertUnparsedItem{
				ReceivedAt: message.ReceivedUnixTime,
				Sender:     message.Sender,
				Text:       message.Text,
			})
		}

		if i == 0 {
			response.LastReceivedAt = message.ReceivedUnixTime
			response.LastOutcome = message.Outcome
		}
	}

	return response, nil
}

// DeleteAllAlertMessages permanently deletes all alert message rows of this user
func (s *AlertService) DeleteAllAlertMessages(c core.Context, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	_, err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Delete(&models.AlertMessage{})

	return err
}

// storeMessage inserts an AlertMessage row for every ingest request (step 1); its outcome is
// updated in place once the final outcome of this request is known. An ignored message (OTP,
// promo, ...) keeps no text, only enough to count it; a parsed one keeps its payee key so later
// alerts from the same payee can learn from the transaction it becomes (see classify).
func (s *AlertService) storeMessage(c core.Context, uid int64, sender string, text string, textHash string, receivedAt time.Time, parsed alerts.ParsedAlert) (*models.AlertMessage, error) {
	message := &models.AlertMessage{
		AlertId:          s.GenerateUuid(uuid.UUID_TYPE_ALERT_MESSAGE),
		Uid:              uid,
		Sender:           sender,
		Text:             text,
		Reference:        parsed.Reference,
		TextHash:         textHash,
		Outcome:          string(parsed.Outcome),
		ReceivedUnixTime: receivedAt.Unix(),
		CreatedUnixTime:  time.Now().Unix(),
	}

	if parsed.Outcome == alerts.OutcomeIgnored {
		message.Text = ""
	}

	if parsed.Outcome == alerts.OutcomeParsed {
		message.PayeeKey = utils.SubString(alerts.PayeeKey(parsed.Counterparty), 0, 64)
	}

	if message.AlertId < 1 {
		return nil, errs.ErrSystemIsBusy
	}

	_, err := s.UserDataDB(uid).NewSession(c).Insert(message)

	if err != nil {
		return nil, err
	}

	return message, nil
}

// updateMessageOutcome updates the outcome and transaction id of an already-stored alert
// message row (step 11, and the duplicate / unparsed-after-create paths)
func (s *AlertService) updateMessageOutcome(c core.Context, uid int64, alertId int64, outcome string, transactionId int64) error {
	update := &models.AlertMessage{
		Outcome:       outcome,
		TransactionId: transactionId,
	}

	_, err := s.UserDataDB(uid).NewSession(c).ID(alertId).Where("uid=?", uid).Cols("outcome", "transaction_id").Update(update)

	return err
}

// failAfterStore marks an already-stored alert message row "unparsed" (best-effort; its own
// error is discarded) before returning the original error, so that a client retry after an
// internal error is recognised as a fresh message rather than finding the row stuck at its
// initial "parsed" placeholder forever
func (s *AlertService) failAfterStore(c core.Context, uid int64, alertId int64, err error) error {
	_ = s.updateMessageOutcome(c, uid, alertId, string(alerts.OutcomeUnparsed), 0)
	return err
}

// isRepeatedMessage reports whether this user already has an alert row with the same sender, text
// hash and outcome received within 120 seconds: the same ignored / unparsed SMS posted twice (e.g.
// a message containing both "Rs" and "INR" triggers both automations) is not stored twice
func (s *AlertService) isRepeatedMessage(c core.Context, uid int64, sender string, textHash string, outcome string, receivedAt time.Time) (bool, error) {
	return s.UserDataDB(uid).NewSession(c).Where("uid=? AND sender=? AND text_hash=? AND outcome=? AND received_unix_time>=? AND received_unix_time<=?",
		uid, sender, textHash, outcome, receivedAt.Unix()-repeatedAlertWindowSeconds, receivedAt.Unix()+repeatedAlertWindowSeconds).Exist(&models.AlertMessage{})
}

// isDuplicateReference reports whether this user already has an added alert with the same
// reference (step 4)
func (s *AlertService) isDuplicateReference(c core.Context, uid int64, parsed alerts.ParsedAlert) (bool, error) {
	if parsed.Reference == "" {
		return false, nil
	}

	return s.UserDataDB(uid).NewSession(c).Where("uid=? AND reference=? AND outcome=?", uid, parsed.Reference, "added").Exist(&models.AlertMessage{})
}

// isRecentDuplicate reports whether this user already has an added alert for the same
// account, amount and direction received within 120 seconds (step 5); since AlertMessage does
// not itself store account / amount / direction, candidate rows are re-parsed for comparison.
// Only called when the current alert has no reference at all (see Ingest); a candidate row that
// itself carries a different, non-empty reference is never treated as the same payment.
func (s *AlertService) isRecentDuplicate(c core.Context, uid int64, parsed alerts.ParsedAlert, receivedAt time.Time) (bool, error) {
	minTime := receivedAt.Unix() - 120
	maxTime := receivedAt.Unix() + 120

	var candidates []*models.AlertMessage
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND outcome=? AND received_unix_time>=? AND received_unix_time<=?", uid, "added", minTime, maxTime).Find(&candidates)

	if err != nil {
		return false, err
	}

	for i := 0; i < len(candidates); i++ {
		if candidates[i].Reference != "" && candidates[i].Reference != parsed.Reference {
			continue
		}

		candidate := alerts.Parse(candidates[i].Sender, candidates[i].Text)

		if candidate.Outcome == alerts.OutcomeParsed && candidate.Last4 == parsed.Last4 && candidate.Amount == parsed.Amount && candidate.Direction == parsed.Direction {
			return true, nil
		}
	}

	return false, nil
}

// resolveAccount finds this user's account whose name ends with the alert's last-4 digits,
// falling back to (creating if needed) this user's "Unmatched alerts" account (step 6); the
// bool return forces the transaction into review when the fallback account was used. Hidden
// accounts and multi-sub-account parents (which cannot hold transactions) are never matched,
// and an ambiguous match (more than one eligible account with the same last-4) also falls back
// to "Unmatched alerts" with review forced, rather than guessing.
func (s *AlertService) resolveAccount(c core.Context, uid int64, parsed alerts.ParsedAlert) (*models.Account, bool, error) {
	accounts, err := Accounts.GetAllAccountsByUid(c, uid)

	if err != nil {
		return nil, false, err
	}

	if account := uniqueAccountByLast4(accounts, parsed.Last4); account != nil {
		return account, false, nil
	}

	unmatchedHidden := false

	for i := 0; i < len(accounts); i++ {
		if accounts[i].Name == unmatchedAccountName {
			if accounts[i].Hidden {
				unmatchedHidden = true
				continue
			}

			return accounts[i], true, nil
		}
	}

	if unmatchedHidden {
		return nil, false, errUnmatchedAccountHidden
	}

	account := &models.Account{
		Uid:      uid,
		Category: models.ACCOUNT_CATEGORY_CHECKING_ACCOUNT,
		Type:     models.ACCOUNT_TYPE_SINGLE_ACCOUNT,
		Name:     unmatchedAccountName,
		Icon:     1,
		Color:    "000000",
		Currency: "INR",
		Balance:  0,
	}

	if err := Accounts.CreateAccounts(c, account, 0, nil, nil, time.UTC); err != nil {
		return nil, false, err
	}

	return account, true, nil
}

// uniqueAccountByLast4 returns the one account among accounts whose name ends with last4 (preceded
// by a non-digit or the start of the name), or nil when there is no such account or more than
// one; hidden accounts and multi-sub-account parents are never matched
func uniqueAccountByLast4(accounts []*models.Account, last4 string) *models.Account {
	var matches []*models.Account

	for i := 0; i < len(accounts); i++ {
		if accounts[i].Hidden || accounts[i].Type == models.ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS {
			continue
		}

		if m := reAccountNameLast4.FindStringSubmatch(accounts[i].Name); m != nil && m[1] == last4 {
			matches = append(matches, accounts[i])
		}
	}

	if len(matches) == 1 {
		return matches[0]
	}

	return nil
}

// isDuplicateOwnTransfer reports whether this user already recorded the same own-account
// transfer (same source, same destination, same amount, tagged "Auto (SMS)") from the other
// side's alert, within 3 days either side of this alert's received time. It is checked before
// creating any own-account transfer, regardless of which side (debit or credit) is processed
// first, so that whichever SMS arrives second is recognised as a duplicate rather than
// recording the same physical transfer twice.
func (s *AlertService) isDuplicateOwnTransfer(c core.Context, uid int64, srcAccountId int64, dstAccountId int64, amount int64, receivedAt time.Time, autoSmsTagId int64) (bool, error) {
	transfers, err := s.findAutoSmsTransfers(c, uid, "account_id=? AND related_account_id=?", []any{srcAccountId, dstAccountId}, amount, receivedAt, autoSmsTagId)

	if err != nil {
		return false, err
	}

	return len(transfers) > 0, nil
}

// isDuplicateTransferSide reports whether an alert that is about to become a plain income /
// expense on accountId is in fact the other side of an own-account transfer already recorded from
// the other account's SMS (same amount, tagged "Auto (SMS)", within 3 days either side): a credit
// into accountId matches a transfer *to* it recorded from a debit SMS, and a debit from accountId
// matches a transfer *from* it recorded from a credit SMS. Without this, an SMS that doesn't name
// the other account (e.g. "credited ... by IMPS from JOHN DOE") would count the money twice.
func (s *AlertService) isDuplicateTransferSide(c core.Context, uid int64, accountId int64, direction alerts.Direction, amount int64, receivedAt time.Time, autoSmsTagId int64) (bool, error) {
	condition, originDirection := "related_account_id=?", alerts.Debit

	if direction == alerts.Debit {
		condition, originDirection = "account_id=?", alerts.Credit
	}

	transfers, err := s.findAutoSmsTransfers(c, uid, condition, []any{accountId}, amount, receivedAt, autoSmsTagId)

	if err != nil || len(transfers) == 0 {
		return false, err
	}

	transferIds := make([]int64, len(transfers))

	for i := 0; i < len(transfers); i++ {
		transferIds[i] = transfers[i].TransactionId
	}

	var messages []*models.AlertMessage
	err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND outcome=?", uid, "added").In("transaction_id", transferIds).Find(&messages)

	if err != nil {
		return false, err
	}

	for i := 0; i < len(messages); i++ {
		if alerts.Parse(messages[i].Sender, messages[i].Text).Direction == originDirection {
			return true, nil
		}
	}

	return false, nil
}

// replaceEarlierTransferSide handles an own-account transfer whose other side was recorded first
// as a plain transaction: a debit SMS on src looks for an income on dst, a credit SMS on dst for an
// expense on src (same amount, tagged "Auto (SMS)", within 3 days either side). Such a transaction
// still tagged "Needs review" is deleted (its alert becomes "duplicate") so the transfer is the only
// record; one the user has already categorised is kept and conflict is returned so the new transfer
// is flagged for review instead of silently counting the money twice.
func (s *AlertService) replaceEarlierTransferSide(c core.Context, uid int64, srcAccountId int64, dstAccountId int64, direction alerts.Direction, amount int64, receivedAt time.Time, autoSmsTagId int64, needsReviewTagId int64) (bool, error) {
	transactionType, accountId := models.TRANSACTION_DB_TYPE_INCOME, dstAccountId

	if direction == alerts.Credit {
		transactionType, accountId = models.TRANSACTION_DB_TYPE_EXPENSE, srcAccountId
	}

	minTime := utils.GetMinTransactionTimeFromUnixTime(receivedAt.Add(-3 * 24 * time.Hour).Unix())
	maxTime := utils.GetMaxTransactionTimeFromUnixTime(receivedAt.Add(3 * 24 * time.Hour).Unix())

	var candidates []*models.Transaction
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND type=? AND account_id=? AND amount=? AND transaction_time>=? AND transaction_time<=?",
		uid, false, transactionType, accountId, amount, minTime, maxTime).OrderBy("transaction_time desc").Find(&candidates)

	if err != nil || len(candidates) == 0 {
		return false, err
	}

	transactionIds := make([]int64, len(candidates))

	for i := 0; i < len(candidates); i++ {
		transactionIds[i] = candidates[i].TransactionId
	}

	tagIdsByTransaction, err := TransactionTags.GetAllTagIdsOfTransactions(c, uid, transactionIds)

	if err != nil {
		return false, err
	}

	conflict := false

	for i := 0; i < len(candidates); i++ {
		hasAutoSms, needsReview := false, false

		for _, tagId := range tagIdsByTransaction[candidates[i].TransactionId] {
			if tagId == autoSmsTagId {
				hasAutoSms = true
			} else if tagId == needsReviewTagId {
				needsReview = true
			}
		}

		if !hasAutoSms {
			continue
		}

		if !needsReview {
			conflict = true
			continue
		}

		if err := Transactions.DeleteTransaction(c, uid, candidates[i].TransactionId); err != nil {
			return false, err
		}

		_, err = s.UserDataDB(uid).NewSession(c).Cols("outcome", "transaction_id").Where("uid=? AND transaction_id=?", uid, candidates[i].TransactionId).Update(&models.AlertMessage{Outcome: "duplicate", TransactionId: 0})

		return false, err
	}

	return conflict, nil
}

// pairCardPayment matches one half of a card bill paid through CRED with the other half recorded
// earlier (same amount, within 3 days either side, tagged "Card payment pending"): outgoing is the
// bank's debit to CRED, otherwise accountId is the card that confirmed the payment. It returns the
// other end's account when the pending half was found (and deleted, its alert marked duplicate),
// or duplicate when this payment is already recorded: the card already received a transfer of
// this amount, or the same half is already pending (the issuer's and CRED's copies of one SMS).
func (s *AlertService) pairCardPayment(c core.Context, uid int64, outgoing bool, accountId int64, amount int64, receivedAt time.Time, tagIds map[string]int64) (int64, bool, error) {
	if !outgoing {
		transfers, err := s.findAutoSmsTransfers(c, uid, "related_account_id=?", []any{accountId}, amount, receivedAt, tagIds[tagAutoSms])

		if err != nil || len(transfers) > 0 {
			return 0, len(transfers) > 0, err
		}
	}

	otherType, sameType := models.TRANSACTION_DB_TYPE_EXPENSE, models.TRANSACTION_DB_TYPE_INCOME

	if outgoing {
		otherType, sameType = models.TRANSACTION_DB_TYPE_INCOME, models.TRANSACTION_DB_TYPE_EXPENSE
	}

	pending, err := s.findPendingCardPayments(c, uid, otherType, 0, amount, receivedAt, tagIds[tagCardPaymentPending])

	if err != nil {
		return 0, false, err
	}

	if len(pending) > 0 {
		if err := Transactions.DeleteTransaction(c, uid, pending[0].TransactionId); err != nil {
			return 0, false, err
		}

		_, err = s.UserDataDB(uid).NewSession(c).Cols("outcome", "transaction_id").Where("uid=? AND transaction_id=?", uid, pending[0].TransactionId).Update(&models.AlertMessage{Outcome: "duplicate", TransactionId: 0})

		return pending[0].AccountId, false, err
	}

	same, err := s.findPendingCardPayments(c, uid, sameType, accountId, amount, receivedAt, tagIds[tagCardPaymentPending])

	return 0, len(same) > 0, err
}

// findPendingCardPayments returns this user's transactions of the given type and amount tagged
// "Card payment pending" within 3 days either side of receivedAt (on accountId when it is set),
// oldest first
func (s *AlertService) findPendingCardPayments(c core.Context, uid int64, transactionType models.TransactionDbType, accountId int64, amount int64, receivedAt time.Time, pendingTagId int64) ([]*models.Transaction, error) {
	minTime := utils.GetMinTransactionTimeFromUnixTime(receivedAt.Add(-3 * 24 * time.Hour).Unix())
	maxTime := utils.GetMaxTransactionTimeFromUnixTime(receivedAt.Add(3 * 24 * time.Hour).Unix())

	session := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND type=? AND amount=? AND transaction_time>=? AND transaction_time<=?", uid, false, transactionType, amount, minTime, maxTime)

	if accountId > 0 {
		session = session.And("account_id=?", accountId)
	}

	var candidates []*models.Transaction

	if err := session.OrderBy("transaction_time asc").Find(&candidates); err != nil || len(candidates) == 0 {
		return nil, err
	}

	transactionIds := make([]int64, len(candidates))

	for i := 0; i < len(candidates); i++ {
		transactionIds[i] = candidates[i].TransactionId
	}

	tagIdsByTransaction, err := TransactionTags.GetAllTagIdsOfTransactions(c, uid, transactionIds)

	if err != nil {
		return nil, err
	}

	var pending []*models.Transaction

	for i := 0; i < len(candidates); i++ {
		for _, tagId := range tagIdsByTransaction[candidates[i].TransactionId] {
			if tagId == pendingTagId {
				pending = append(pending, candidates[i])
				break
			}
		}
	}

	return pending, nil
}

// findAutoSmsTransfers returns this user's transfer-out transactions tagged "Auto (SMS)" with the
// given amount, within 3 days either side of receivedAt, that also satisfy condition (a where
// clause over account_id / related_account_id, with its args)
func (s *AlertService) findAutoSmsTransfers(c core.Context, uid int64, condition string, args []any, amount int64, receivedAt time.Time, autoSmsTagId int64) ([]*models.Transaction, error) {
	minTime := utils.GetMinTransactionTimeFromUnixTime(receivedAt.Add(-3 * 24 * time.Hour).Unix())
	maxTime := utils.GetMaxTransactionTimeFromUnixTime(receivedAt.Add(3 * 24 * time.Hour).Unix())

	var candidates []*models.Transaction
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND type=? AND amount=? AND transaction_time>=? AND transaction_time<=? AND "+condition,
		append([]any{uid, false, models.TRANSACTION_DB_TYPE_TRANSFER_OUT, amount, minTime, maxTime}, args...)...).Find(&candidates)

	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	transactionIds := make([]int64, len(candidates))

	for i := 0; i < len(candidates); i++ {
		transactionIds[i] = candidates[i].TransactionId
	}

	tagIdsByTransaction, err := TransactionTags.GetAllTagIdsOfTransactions(c, uid, transactionIds)

	if err != nil {
		return nil, err
	}

	var transfers []*models.Transaction

	for i := 0; i < len(candidates); i++ {
		tagIds := tagIdsByTransaction[candidates[i].TransactionId]

		for j := 0; j < len(tagIds); j++ {
			if tagIds[j] == autoSmsTagId {
				transfers = append(transfers, candidates[i])
				break
			}
		}
	}

	return transfers, nil
}

// classify builds the inputs alerts.Classify needs from this user's own data and delegates to
// it (step 8). Hidden accounts and multi-sub-account parents are excluded from the "own"
// transfer-matching list, and hidden categories are excluded from the keyword mapping and from
// payee history hits, since none of them can legally be used on a new transaction.
func (s *AlertService) classify(c core.Context, uid int64, accountId int64, parsed alerts.ParsedAlert, textUpper string) (alerts.Classification, error) {
	accounts, err := Accounts.GetAllAccountsByUid(c, uid)

	if err != nil {
		return alerts.Classification{}, err
	}

	own := make([]alerts.OwnAccount, 0, len(accounts))

	for i := 0; i < len(accounts); i++ {
		if accounts[i].Hidden || accounts[i].Type == models.ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS {
			continue
		}

		if m := reAccountNameLast4.FindStringSubmatch(accounts[i].Name); m != nil {
			own = append(own, alerts.OwnAccount{ID: accounts[i].AccountId, Name: accounts[i].Name, Last4: m[1]})
		}
	}

	categories, err := TransactionCategories.GetAllCategoriesByUid(c, uid, 0, -1)

	if err != nil {
		return alerts.Classification{}, err
	}

	categoryById := make(map[int64]*models.TransactionCategory, len(categories))
	categoryIdByName := make(map[string]int64, len(categories))

	for i := 0; i < len(categories); i++ {
		categoryById[categories[i].CategoryId] = categories[i]

		if !categories[i].Hidden && categories[i].ParentCategoryId != models.LevelOneTransactionCategoryParentId {
			categoryIdByName[categories[i].Name] = categories[i].CategoryId
		}
	}

	// usable reports whether a past transaction of the same payee can classify this alert, and how
	usable := func(transaction *models.Transaction) (alerts.HistoryHit, bool) {
		isTransfer := transaction.Type == models.TRANSACTION_DB_TYPE_TRANSFER_OUT || transaction.Type == models.TRANSACTION_DB_TYPE_TRANSFER_IN

		if isTransfer {
			// This past transaction may have been recorded from either side of the
			// transfer; the "other" account is whichever side isn't the resolved account.
			otherAccountId := transaction.RelatedAccountId

			if otherAccountId == accountId {
				otherAccountId = transaction.AccountId
			}

			return alerts.HistoryHit{IsTransfer: true, OtherAccountId: otherAccountId}, true
		}

		// A non-transfer hit is only usable when its direction matches this alert's: an
		// expense category can't classify a credit, and vice versa (e.g. a refund from the
		// same payee that was previously an expense).
		if parsed.Direction == alerts.Debit && transaction.Type != models.TRANSACTION_DB_TYPE_EXPENSE {
			return alerts.HistoryHit{}, false
		}

		if parsed.Direction == alerts.Credit && transaction.Type != models.TRANSACTION_DB_TYPE_INCOME {
			return alerts.HistoryHit{}, false
		}

		category := categoryById[transaction.CategoryId]

		if category == nil || category.Hidden {
			return alerts.HistoryHit{}, false
		}

		return alerts.HistoryHit{CategoryId: transaction.CategoryId}, true
	}

	history := func(payeeKey string) (alerts.HistoryHit, bool) {
		// SMS-captured transactions first: their comment is the raw SMS, so the comment search
		// below can never find them; the alert row keeps the payee key, and the transaction's
		// *current* category / transfer target is used, so the user's corrections are learnt.
		if hit, ok := s.smsPayeeHistory(c, uid, payeeKey, categoryById, usable); ok {
			return hit, true
		}

		transactions, err := Transactions.GetTransactionsByMaxTime(c, uid, 0, 0, 0, nil, nil, nil, false, "", payeeKey, core.MATCH_MODE_IGNORE_CASE, false, 1, 20, false, true)

		if err != nil {
			return alerts.HistoryHit{}, false
		}

		for i := 0; i < len(transactions); i++ {
			if alerts.PayeeKey(transactions[i].Comment) != payeeKey {
				continue
			}

			if hit, ok := usable(transactions[i]); ok {
				return hit, true
			}
		}

		return alerts.HistoryHit{}, false
	}

	keywords := make([]alerts.KeywordRule, 0, len(keywordCategoryRules))

	for i := 0; i < len(keywordCategoryRules); i++ {
		rule := keywordCategoryRules[i]

		if categoryId, ok := categoryIdByName[rule.categoryName]; ok {
			keywords = append(keywords, alerts.KeywordRule{Pattern: rule.pattern, Direction: rule.direction, CategoryId: categoryId})
		}
	}

	return alerts.Classify(parsed, accountId, own, history, keywords, textUpper), nil
}

// smsPayeeHistory looks up this user's newest SMS-captured transactions of the same payee (by the
// payee key stored on their alert rows) and returns the first usable one's classification. A
// transaction still sitting in a fallback category ("Other Expense" / "Other Income") taught
// nothing, so it is skipped rather than turning the next alert's review into a silent guess.
func (s *AlertService) smsPayeeHistory(c core.Context, uid int64, payeeKey string, categoryById map[int64]*models.TransactionCategory, usable func(*models.Transaction) (alerts.HistoryHit, bool)) (alerts.HistoryHit, bool) {
	var messages []*models.AlertMessage
	payeeKey = utils.SubString(payeeKey, 0, 64) // stored truncated to the column width
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND payee_key=? AND outcome=? AND transaction_id>?", uid, payeeKey, "added", 0).OrderBy("received_unix_time desc").Limit(20).Find(&messages)

	if err != nil || len(messages) == 0 {
		return alerts.HistoryHit{}, false
	}

	transactionIds := make([]int64, len(messages))

	for i := 0; i < len(messages); i++ {
		transactionIds[i] = messages[i].TransactionId
	}

	transactions, err := Transactions.GetTransactionsByTransactionIds(c, uid, transactionIds)

	if err != nil {
		return alerts.HistoryHit{}, false
	}

	transactionById := make(map[int64]*models.Transaction, len(transactions))

	for i := 0; i < len(transactions); i++ {
		transactionById[transactions[i].TransactionId] = transactions[i]
	}

	for i := 0; i < len(messages); i++ {
		transaction := transactionById[messages[i].TransactionId]

		if transaction == nil {
			continue
		}

		if category := categoryById[transaction.CategoryId]; category != nil && (category.Name == categoryNameOtherExpense || category.Name == categoryNameOtherIncome) {
			continue
		}

		if hit, ok := usable(transaction); ok {
			return hit, true
		}
	}

	return alerts.HistoryHit{}, false
}

// createTransaction builds and saves the transaction for a parsed alert (step 9); a credit
// into one of this user's own accounts is recorded as a transfer whose source is the other
// account, so the resolved account ends up on the transfer's destination side
func (s *AlertService) createTransaction(c core.Context, uid int64, account *models.Account, parsed alerts.ParsedAlert, classification alerts.Classification, needsReview bool, text string, receivedAt time.Time, tagIds map[string]int64, hiddenTagIds map[int64]bool, extraTagNames ...string) (*models.Transaction, string, error) {
	comment := text

	// Truncated by rune, not by byte, so a multi-byte character (e.g. "₹") right at the boundary
	// is never split into invalid UTF-8; this matches how the "max=255" binding tag on
	// models.TransactionCreateRequest.Comment measures length (utf8.RuneCountInString), and the
	// database column's VARCHAR(255) limit is likewise interpreted as 255 characters, not bytes.
	if utf8.RuneCountInString(comment) > 255 {
		comment = utils.SubString(comment, 0, 255)
	}

	transaction := &models.Transaction{
		Uid:               uid,
		Amount:            parsed.Amount,
		TransactionTime:   utils.GetMinTransactionTimeFromUnixTime(receivedAt.Unix()),
		TimezoneUtcOffset: 330,
		Comment:           comment,
	}

	var categoryLabel string

	if classification.IsTransfer {
		otherAccount, err := Accounts.GetAccountByAccountId(c, uid, classification.OtherAccountId)

		if err != nil {
			return nil, "", err
		}

		categoryNames := []string{categoryNameBankTransfer, categoryNameOtherTransfer}
		destination := otherAccount

		if parsed.Direction == alerts.Credit {
			destination = account
		}

		if destination.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD {
			categoryNames = append([]string{categoryNameCardRepayment}, categoryNames...)
		}

		categoryId, err := s.ensureCategory(c, uid, models.CATEGORY_TYPE_TRANSFER, categoryNames...)

		if err != nil {
			return nil, "", err
		}

		transaction.Type = models.TRANSACTION_DB_TYPE_TRANSFER_OUT
		transaction.CategoryId = categoryId
		transaction.RelatedAccountAmount = parsed.Amount

		if parsed.Direction == alerts.Debit {
			transaction.AccountId = account.AccountId
			transaction.RelatedAccountId = classification.OtherAccountId
			categoryLabel = fmt.Sprintf("Transfer to %s", otherAccount.Name)
		} else {
			transaction.AccountId = classification.OtherAccountId
			transaction.RelatedAccountId = account.AccountId
			categoryLabel = fmt.Sprintf("Transfer from %s", otherAccount.Name)
		}
	} else {
		categoryId := classification.CategoryId

		if categoryId == 0 {
			fallbackType := models.CATEGORY_TYPE_EXPENSE
			fallbackName := categoryNameOtherExpense

			if parsed.Direction == alerts.Credit {
				fallbackType = models.CATEGORY_TYPE_INCOME
				fallbackName = categoryNameOtherIncome
			}

			var err error
			categoryId, err = s.ensureCategory(c, uid, fallbackType, fallbackName)

			if err != nil {
				return nil, "", err
			}
		}

		category, err := TransactionCategories.GetCategoryByCategoryId(c, uid, categoryId)

		if err != nil {
			return nil, "", err
		}

		transaction.Type = models.TRANSACTION_DB_TYPE_EXPENSE

		if parsed.Direction == alerts.Credit {
			transaction.Type = models.TRANSACTION_DB_TYPE_INCOME
		}

		transaction.AccountId = account.AccountId
		transaction.CategoryId = categoryId
		categoryLabel = category.Name
	}

	tagNames := append([]string{tagAutoSms}, extraTagNames...)

	if needsReview {
		tagNames = append(tagNames, tagNeedsReview)
	}

	transactionTagIds := make([]int64, 0, len(tagNames))

	// A tag the user has hidden can't be put on a new transaction; it's skipped (and logged)
	// rather than failing the whole alert
	for _, tagName := range tagNames {
		if hiddenTagIds[tagIds[tagName]] {
			log.Warnf(c, "[alerts.createTransaction] not tagging new transaction of user \"uid:%d\" \"%s\", because that tag is hidden", uid, tagName)
			continue
		}

		transactionTagIds = append(transactionTagIds, tagIds[tagName])
	}

	if err := Transactions.CreateTransaction(c, transaction, transactionTagIds, nil); err != nil {
		return nil, "", err
	}

	summary := fmt.Sprintf("₹%s · %s · %s", formatRupees(parsed.Amount), categoryLabel, account.Name)

	return transaction, summary, nil
}

// ensureCategory finds this user's visible sub-category matching one of the given names (tried
// in order) for the given type, creating a primary category and sub-category named after the
// first name on first use; it is needed because every transaction (transfer or not) must
// reference a real, visible sub-category, which classification alone does not always provide.
// A hidden category of the same name is never reused, since it cannot be used on a transaction.
func (s *AlertService) ensureCategory(c core.Context, uid int64, categoryType models.TransactionCategoryType, names ...string) (int64, error) {
	categories, err := TransactionCategories.GetAllCategoriesByUid(c, uid, categoryType, -1)

	if err != nil {
		return 0, err
	}

	visibleIdByName := make(map[string]int64, len(categories))

	for i := 0; i < len(categories); i++ {
		if !categories[i].Hidden && categories[i].ParentCategoryId != models.LevelOneTransactionCategoryParentId {
			visibleIdByName[categories[i].Name] = categories[i].CategoryId
		}
	}

	for _, name := range names {
		if categoryId, ok := visibleIdByName[name]; ok {
			return categoryId, nil
		}
	}

	name := names[0]
	primary := &models.TransactionCategory{Uid: uid, Type: categoryType, Name: name, Icon: 1, Color: "000000"}
	secondary := &models.TransactionCategory{Uid: uid, Type: categoryType, Name: name, Icon: 1, Color: "000000"}

	categoriesToCreate := map[*models.TransactionCategory][]*models.TransactionCategory{
		nil:     {primary},
		primary: {secondary},
	}

	if _, err := TransactionCategories.CreateCategories(c, uid, categoriesToCreate); err != nil {
		return 0, err
	}

	return secondary.CategoryId, nil
}

// ensureTags creates this user's "Auto (SMS)", "Needs review" and "Balance mismatch" tags on
// demand, skipping any that already exist, and returns their ids by name along with the set of
// those ids the user has hidden (which can't be put on a transaction)
func (s *AlertService) ensureTags(c core.Context, uid int64) (map[string]int64, map[int64]bool, error) {
	tags := []*models.TransactionTag{
		{Uid: uid, Name: tagAutoSms},
		{Uid: uid, Name: tagNeedsReview},
		{Uid: uid, Name: tagBalanceMismatch},
		{Uid: uid, Name: tagCardPaymentPending},
	}

	if err := TransactionTags.CreateTags(c, uid, tags, true); err != nil {
		return nil, nil, err
	}

	tagIds := make(map[string]int64, len(tags))
	hiddenTagIds := make(map[int64]bool)

	for i := 0; i < len(tags); i++ {
		tagIds[tags[i].Name] = tags[i].TagId

		if tags[i].Hidden {
			hiddenTagIds[tags[i].TagId] = true
		}
	}

	return tagIds, hiddenTagIds, nil
}

// ensureTag creates one tag of this user on demand (skipping it if it already exists) and returns
// its id; used by statement reconciliation only once there is something to tag
func (s *AlertService) ensureTag(c core.Context, uid int64, name string) (int64, error) {
	tags := []*models.TransactionTag{{Uid: uid, Name: name}}

	if err := TransactionTags.CreateTags(c, uid, tags, true); err != nil {
		return 0, err
	}

	return tags[0].TagId, nil
}

// lookupTagIds returns the ids of this user's existing tags with the given names, by name,
// without creating any; a missing tag has no entry
func (s *AlertService) lookupTagIds(c core.Context, uid int64, names ...string) (map[string]int64, error) {
	var tags []*models.TransactionTag
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).In("name", names).Find(&tags)

	if err != nil {
		return nil, err
	}

	tagIds := make(map[string]int64, len(tags))

	for i := 0; i < len(tags); i++ {
		tagIds[tags[i].Name] = tags[i].TagId
	}

	return tagIds, nil
}

// checkBalance tags the transaction "Balance mismatch" when the alert carried a balance that
// does not match this account's balance after the transaction was inserted (step 10). Called
// only after the alert has already been marked "added" (see Ingest), so any failure here is
// surfaced to the caller as a log, not as an Ingest error. A credit card's SMS "balance" is its
// available limit, and an overdrawn account's book balance (negative before this transaction)
// can't be compared with the bank's available balance either, so both are skipped.
func (s *AlertService) checkBalance(c core.Context, uid int64, accountId int64, parsed alerts.ParsedAlert, transaction *models.Transaction, balanceMismatchTagId int64) error {
	if !parsed.HasBalance {
		return nil
	}

	account, err := Accounts.GetAccountByAccountId(c, uid, accountId)

	if err != nil {
		return err
	}

	if account.Category == models.ACCOUNT_CATEGORY_CREDIT_CARD {
		return nil
	}

	balanceBefore := account.Balance + parsed.Amount

	if parsed.Direction == alerts.Credit {
		balanceBefore = account.Balance - parsed.Amount
	}

	if balanceBefore < 0 || account.Balance == parsed.Balance {
		return nil
	}

	return Transactions.BatchAddTagsToTransactions(c, uid, []*models.Transaction{transaction}, map[int64][]int64{transaction.TransactionId: {balanceMismatchTagId}})
}

// formatRupees renders an amount in paise as a rupee decimal string ("250.00")
func formatRupees(paise int64) string {
	return fmt.Sprintf("%d.%02d", paise/100, paise%100)
}

// matchCandidate is one (statement row, candidate SMS transaction) pair that satisfies every
// hard constraint (§8: same account, same db type/direction, same amount, within ±2 days,
// reference equal when the statement row has one); MatchStatementRows ranks these and assigns
// greedily so that each side is used at most once (ruling 2)
type matchCandidate struct {
	rowIndex int
	txnId    int64
	refMatch bool
	timeDiff int64
}

// MatchStatementRows matches each statement row against this user's "Auto (SMS)" transactions
// (design doc §8): same account, same amount and direction, date within ±2 days of each other,
// and (when the statement row's narration carries a 12-digit reference) an equal reference on
// the SMS side. Each SMS transaction matches at most one row and vice versa (ruling 2): when
// several candidates qualify for the same row or transaction, the reference-matching pair (if
// any) wins, then the pair closest in time.
//
// Statement rows carry account *names*, not ids, at this point (ruling 3): a row's account is
// resolved against this uid's accounts the same way the rest of the import flow resolves them,
// via OriginalSourceAccountName, falling back to AccountId if that's already been resolved by
// the caller. When neither resolves it, the importer's account name (e.g. "HDFC Bank 1234") is
// resolved by its trailing 4 digits the same way an SMS's account is (see resolveAccount), since
// users rarely name the account exactly as the importer does; a matched row resolved this way
// gets that account id, so the matched row the client echoes back on import names its account.
//
// An already "Statement verified" transaction still matches (so the other account of a transfer,
// or the same statement imported again, is recognised); but a verified non-transfer only matches
// a row with its exact narration on its exact (statement) day, so that a later, genuinely new
// statement row of the same amount isn't swallowed by it.
//
// The returned map is row index -> matched transaction id; a row with no entry has no match.
func (s *AlertService) MatchStatementRows(c core.Context, uid int64, rows []*models.ImportTransaction) (map[int]int64, error) {
	result := make(map[int]int64)

	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if len(rows) == 0 {
		return result, nil
	}

	accounts, err := Accounts.GetAllAccountsByUid(c, uid)

	if err != nil {
		return nil, err
	}

	accountByName := Accounts.GetVisibleAccountNameMapByList(accounts)

	tagIds, err := s.lookupTagIds(c, uid, tagAutoSms, tagStatementVerified)

	if err != nil {
		return nil, err
	}

	autoSmsTagId, hasAutoSmsTag := tagIds[tagAutoSms]

	// no "Auto (SMS)" tag means nothing was ever captured from SMS: nothing can match
	if !hasAutoSmsTag {
		return result, nil
	}

	statementVerifiedTagId := tagIds[tagStatementVerified]

	type rowInfo struct {
		index      int
		accountId  int64
		amount     int64
		dbType     models.TransactionDbType
		unixTime   int64
		utcOffset  int16
		comment    string
		references []string
	}

	resolvedByLast4 := make(map[int]int64)

	infos := make([]rowInfo, 0, len(rows))
	accountIdSet := make(map[int64]bool)
	var minUnixTime, maxUnixTime int64

	for i := 0; i < len(rows); i++ {
		row := rows[i]

		if row == nil || row.Transaction == nil {
			continue
		}

		accountId := row.AccountId

		if row.OriginalSourceAccountName != "" {
			if account, ok := accountByName[row.OriginalSourceAccountName]; ok {
				accountId = account.AccountId
			} else if m := reAccountNameLast4.FindStringSubmatch(row.OriginalSourceAccountName); m != nil && accountId <= 0 {
				if account := uniqueAccountByLast4(accounts, m[1]); account != nil {
					accountId = account.AccountId
					resolvedByLast4[i] = accountId
				}
			}
		}

		if accountId <= 0 {
			continue
		}

		unixTime := utils.GetUnixTimeFromTransactionTime(row.TransactionTime)

		infos = append(infos, rowInfo{
			index:      i,
			accountId:  accountId,
			amount:     row.Amount,
			dbType:     row.Type,
			unixTime:   unixTime,
			utcOffset:  row.TimezoneUtcOffset,
			comment:    truncateComment(row.Comment),
			references: extractReferences(row.Comment),
		})

		accountIdSet[accountId] = true

		if len(infos) == 1 || unixTime < minUnixTime {
			minUnixTime = unixTime
		}

		if len(infos) == 1 || unixTime > maxUnixTime {
			maxUnixTime = unixTime
		}
	}

	if len(infos) == 0 {
		return result, nil
	}

	accountIds := make([]int64, 0, len(accountIdSet))

	for accountId := range accountIdSet {
		accountIds = append(accountIds, accountId)
	}

	windowMinTransactionTime := utils.GetMinTransactionTimeFromUnixTime(minUnixTime - reconcileDateWindowSeconds)
	windowMaxTransactionTime := utils.GetMaxTransactionTimeFromUnixTime(maxUnixTime + reconcileDateWindowSeconds)

	var candidates []*models.Transaction
	err = s.UserDataDB(uid).NewSession(c).
		In("account_id", accountIds).
		Where("uid=? AND deleted=? AND transaction_time>=? AND transaction_time<=?", uid, false, windowMinTransactionTime, windowMaxTransactionTime).
		Find(&candidates)

	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return result, nil
	}

	candidateIds := make([]int64, len(candidates))

	for i := 0; i < len(candidates); i++ {
		candidateIds[i] = reconcileOwnerId(candidates[i])
	}

	tagIdsByTransaction, err := TransactionTags.GetAllTagIdsOfTransactions(c, uid, candidateIds)

	if err != nil {
		return nil, err
	}

	var alertMessages []*models.AlertMessage
	err = s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).In("transaction_id", candidateIds).Find(&alertMessages)

	if err != nil {
		return nil, err
	}

	referenceByTransactionId := make(map[int64]string, len(alertMessages))

	for i := 0; i < len(alertMessages); i++ {
		if alertMessages[i].TransactionId > 0 {
			referenceByTransactionId[alertMessages[i].TransactionId] = alertMessages[i].Reference
		}
	}

	var pairs []matchCandidate

	for i := 0; i < len(infos); i++ {
		info := infos[i]

		for j := 0; j < len(candidates); j++ {
			candidate := candidates[j]

			if candidate.AccountId != info.accountId || !reconcileTypeMatches(info.dbType, candidate.Type) || candidate.Amount != info.amount {
				continue
			}

			ownerId := reconcileOwnerId(candidate)
			hasAutoSms, isVerified := false, false

			for _, tagId := range tagIdsByTransaction[ownerId] {
				if tagId == autoSmsTagId {
					hasAutoSms = true
				} else if tagId == statementVerifiedTagId {
					isVerified = true
				}
			}

			// an already verified transaction still matches, so the other account of a transfer
			// (or the same statement imported again) is recognised instead of imported twice;
			// MarkStatementVerified leaves it unchanged
			if !hasAutoSms && !isVerified {
				continue
			}

			candidateUnixTime := utils.GetUnixTimeFromTransactionTime(candidate.TransactionTime)

			// a verified non-transfer already took its statement row's narration and date, so it
			// only matches that same row again, never a different row of the same amount
			if !hasAutoSms && candidate.Type != models.TRANSACTION_DB_TYPE_TRANSFER_OUT && candidate.Type != models.TRANSACTION_DB_TYPE_TRANSFER_IN {
				rowZone := time.FixedZone("", int(info.utcOffset)*60)

				if candidate.Comment != info.comment || time.Unix(candidateUnixTime, 0).In(rowZone).Format("2006-01-02") != time.Unix(info.unixTime, 0).In(rowZone).Format("2006-01-02") {
					continue
				}
			}
			timeDiff := candidateUnixTime - info.unixTime

			if timeDiff < 0 {
				timeDiff = -timeDiff
			}

			if timeDiff > reconcileDateWindowSeconds {
				continue
			}

			candidateReference := referenceByTransactionId[ownerId]
			refMatch := false

			for _, reference := range info.references {
				if reference == candidateReference {
					refMatch = true
				}
			}

			// references must agree only when both sides have one
			if candidateReference != "" && len(info.references) > 0 && !refMatch {
				continue
			}

			pairs = append(pairs, matchCandidate{
				rowIndex: info.index,
				txnId:    ownerId,
				refMatch: refMatch,
				timeDiff: timeDiff,
			})
		}
	}

	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].refMatch != pairs[j].refMatch {
			return pairs[i].refMatch
		}

		return pairs[i].timeDiff < pairs[j].timeDiff
	})

	usedRows := make(map[int]bool, len(pairs))
	usedTxns := make(map[int64]bool, len(pairs))

	for i := 0; i < len(pairs); i++ {
		pair := pairs[i]

		if usedRows[pair.rowIndex] || usedTxns[pair.txnId] {
			continue
		}

		usedRows[pair.rowIndex] = true
		usedTxns[pair.txnId] = true
		result[pair.rowIndex] = pair.txnId

		if accountId, ok := resolvedByLast4[pair.rowIndex]; ok {
			rows[pair.rowIndex].AccountId = accountId
		}
	}

	return result, nil
}

// MarkStatementVerified swaps each matched SMS transaction's "Auto (SMS)" tag (and "Not in
// statement", if a previous import left it tagged that way) for "Statement verified", taking
// the statement row's date and narration while keeping its category (design doc §8).
//
// SECURITY (ruling 1): matches is built from client-supplied transaction ids (the matched ids
// returned by a prior parse, echoed back on import); every id is verified server-side to
// belong to uid, not be deleted, and carry the "Auto (SMS)" tag before anything is touched.
// Anything else is silently skipped (and logged): a client can never make the server modify
// another user's transaction, nor a transaction that was never an SMS capture.
//
// A transaction that fails to verify is logged and the rest are still verified; the returned
// set holds the ids that failed (all of them when the error return is non-nil), so the caller
// can keep them out of MarkNotInStatement - they were in the statement after all.
func (s *AlertService) MarkStatementVerified(c core.Context, uid int64, matches map[int64]*models.ImportTransaction) (map[int64]bool, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	failedIds := make(map[int64]bool)

	if len(matches) == 0 {
		return failedIds, nil
	}

	ids := make([]int64, 0, len(matches))

	for id := range matches {
		ids = append(ids, id)
	}

	allFailed := func(err error) (map[int64]bool, error) {
		for _, id := range ids {
			failedIds[id] = true
		}

		return failedIds, err
	}

	transactions, err := Transactions.GetTransactionsByTransactionIds(c, uid, ids)

	if err != nil {
		return allFailed(err)
	}

	transactionById := make(map[int64]*models.Transaction, len(transactions))

	for i := 0; i < len(transactions); i++ {
		transactionById[transactions[i].TransactionId] = transactions[i]
	}

	tagIdsByTransaction, err := TransactionTags.GetAllTagIdsOfTransactions(c, uid, ids)

	if err != nil {
		return allFailed(err)
	}

	tagIds, err := s.lookupTagIds(c, uid, tagAutoSms, tagStatementVerified, tagNotInStatement)

	if err != nil {
		return allFailed(err)
	}

	autoSmsTagId, hasAutoSmsTag := tagIds[tagAutoSms]

	// no "Auto (SMS)" tag means no transaction can qualify below
	if !hasAutoSmsTag {
		return failedIds, nil
	}

	statementVerifiedTagId := tagIds[tagStatementVerified]
	notInStatementTagId, hasNotInStatementTag := tagIds[tagNotInStatement]

	for id, row := range matches {
		// This transaction either does not belong to uid, was deleted, or - since
		// GetTransactionsByTransactionIds found it but the loop below rejects it - never
		// carried "Auto (SMS)" in the first place; either way, a client-supplied id earns no
		// trust and is skipped rather than acted on (ruling 1).
		transaction, exists := transactionById[id]

		if !exists {
			log.Warnf(c, "[alerts.MarkStatementVerified] skipping transaction \"id:%d\" for user \"uid:%d\", because it does not belong to this user or is deleted", id, uid)
			continue
		}

		hasAutoSms, hasNotInStatement := false, false

		for _, tagId := range tagIdsByTransaction[id] {
			if tagId == autoSmsTagId {
				hasAutoSms = true
			} else if hasNotInStatementTag && tagId == notInStatementTagId {
				hasNotInStatement = true
			}
		}

		if !hasAutoSms {
			log.Warnf(c, "[alerts.MarkStatementVerified] skipping transaction \"id:%d\" for user \"uid:%d\", because it is not tagged \"Auto (SMS)\"", id, uid)
			continue
		}

		// "Statement verified" is only created once there is something to tag with it
		if statementVerifiedTagId == 0 {
			statementVerifiedTagId, err = s.ensureTag(c, uid, tagStatementVerified)

			if err != nil {
				return allFailed(err)
			}
		}

		newTransaction := *transaction

		if row != nil && row.Transaction != nil {
			newTransaction.TransactionTime = utils.GetMinTransactionTimeFromUnixTime(utils.GetUnixTimeFromTransactionTime(row.TransactionTime))
			newTransaction.TimezoneUtcOffset = row.TimezoneUtcOffset
			newTransaction.Comment = truncateComment(row.Comment)
		}

		removeTagIds := []int64{autoSmsTagId}

		if hasNotInStatement {
			removeTagIds = append(removeTagIds, notInStatementTagId)
		}

		addTagIds := []int64{statementVerifiedTagId}
		currentTagIdsCount := len(tagIdsByTransaction[id])

		err = Transactions.ModifyTransaction(c, &newTransaction, false, currentTagIdsCount, addTagIds, removeTagIds, nil, nil)

		// The existing service refuses to move a transaction's time before an opening-balance
		// (MODIFY_BALANCE) transaction on the same account (ruling 4); when that happens, the
		// transaction is still verified, just with its original time kept.
		if err == errs.ErrCannotAddTransactionBeforeBalanceModificationTransaction {
			log.Infof(c, "[alerts.MarkStatementVerified] keeping original time for transaction \"id:%d\" of user \"uid:%d\", because %s", id, uid, err.Error())
			newTransaction.TransactionTime = transaction.TransactionTime
			newTransaction.TimezoneUtcOffset = transaction.TimezoneUtcOffset
			err = Transactions.ModifyTransaction(c, &newTransaction, false, currentTagIdsCount, addTagIds, removeTagIds, nil, nil)
		}

		if err != nil {
			log.Errorf(c, "[alerts.MarkStatementVerified] failed to verify transaction \"id:%d\" for user \"uid:%d\", because %s", id, uid, err.Error())
			failedIds[id] = true
		}
	}

	return failedIds, nil
}

// MarkNotInStatement tags "Not in statement" every "Auto (SMS)" transaction of uid+accountId in
// [from, to] that isn't already "Statement verified" (design doc §8). It takes no set of
// matched ids (ruling 5): it is always called after MarkStatementVerified has already swapped
// the matched transactions' tags away from "Auto (SMS)", so those are naturally excluded here,
// and a transaction already tagged "Not in statement" by a previous import is left alone. The
// matched ones MarkStatementVerified failed to verify still carry "Auto (SMS)", so they are
// passed in exclude.
func (s *AlertService) MarkNotInStatement(c core.Context, uid int64, accountId int64, from int64, to int64, exclude map[int64]bool) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if accountId <= 0 {
		return errs.ErrAccountIdInvalid
	}

	tagIds, err := s.lookupTagIds(c, uid, tagAutoSms, tagStatementVerified, tagNotInStatement)

	if err != nil {
		return err
	}

	autoSmsTagId, hasAutoSmsTag := tagIds[tagAutoSms]

	if !hasAutoSmsTag {
		return nil
	}

	statementVerifiedTagId := tagIds[tagStatementVerified]
	notInStatementTagId, hasNotInStatementTag := tagIds[tagNotInStatement]

	minTransactionTime := utils.GetMinTransactionTimeFromUnixTime(from)
	maxTransactionTime := utils.GetMaxTransactionTimeFromUnixTime(to)

	var candidates []*models.Transaction
	err = s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND account_id=? AND transaction_time>=? AND transaction_time<=?", uid, false, accountId, minTransactionTime, maxTransactionTime).Find(&candidates)

	if err != nil {
		return err
	}

	if len(candidates) == 0 {
		return nil
	}

	candidateIds := make([]int64, len(candidates))

	for i := 0; i < len(candidates); i++ {
		candidateIds[i] = candidates[i].TransactionId
	}

	tagIdsByTransaction, err := TransactionTags.GetAllTagIdsOfTransactions(c, uid, candidateIds)

	if err != nil {
		return err
	}

	toTag := make([]*models.Transaction, 0, len(candidates))

	for i := 0; i < len(candidates); i++ {
		candidate := candidates[i]
		hasAutoSms, isVerified, isNotInStatement := false, false, false

		for _, tagId := range tagIdsByTransaction[candidate.TransactionId] {
			if tagId == autoSmsTagId {
				hasAutoSms = true
			} else if tagId == statementVerifiedTagId {
				isVerified = true
			} else if hasNotInStatementTag && tagId == notInStatementTagId {
				isNotInStatement = true
			}
		}

		if exclude[candidate.TransactionId] || exclude[reconcileOwnerId(candidate)] {
			continue
		}

		if hasAutoSms && !isVerified && !isNotInStatement {
			toTag = append(toTag, candidate)
		}
	}

	if len(toTag) == 0 {
		return nil
	}

	if !hasNotInStatementTag {
		notInStatementTagId, err = s.ensureTag(c, uid, tagNotInStatement)

		if err != nil {
			return err
		}
	}

	addTagIds := make(map[int64][]int64, len(toTag))

	for i := 0; i < len(toTag); i++ {
		addTagIds[toTag[i].TransactionId] = []int64{notInStatementTagId}
	}

	return Transactions.BatchAddTagsToTransactions(c, uid, toTag, addTagIds)
}
