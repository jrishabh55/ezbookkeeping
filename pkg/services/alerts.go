package services

import (
	"fmt"
	"regexp"
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
	tagAutoSms         = "Auto (SMS)"
	tagNeedsReview     = "Needs review"
	tagBalanceMismatch = "Balance mismatch"
)

// unmatchedAccountName is the name of the per-user fallback account used when an alert's
// account cannot be resolved from its last-4 digits
const unmatchedAccountName = "Unmatched alerts"

// Fallback category names looked up (and created on first use) when classification does not
// resolve a usable category; the transfer names are tried in priority order
const (
	categoryNameBankTransfer  = "Bank Transfer"
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

	message, err := s.storeMessage(c, uid, sender, text, receivedAt, parsed)

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

	if err != nil {
		return nil, s.failAfterStore(c, uid, message.AlertId, err)
	}

	tagIds, err := s.ensureTags(c, uid)

	if err != nil {
		return nil, s.failAfterStore(c, uid, message.AlertId, err)
	}

	classification, err := s.classify(c, uid, account.AccountId, parsed, strings.ToUpper(text))

	if err != nil {
		return nil, s.failAfterStore(c, uid, message.AlertId, err)
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
	}

	needsReview := forcedReview || classification.NeedsReview

	transaction, summary, err := s.createTransaction(c, uid, account, parsed, classification, needsReview, text, receivedAt, tagIds)

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

	if err := s.checkBalance(c, uid, account.AccountId, parsed, transaction, tagIds[tagBalanceMismatch]); err != nil {
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
		Counts: make(map[string]int64),
	}

	for i := 0; i < len(messages); i++ {
		message := messages[i]
		response.Counts[message.Outcome]++

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
// updated in place once the final outcome of this request is known
func (s *AlertService) storeMessage(c core.Context, uid int64, sender string, text string, receivedAt time.Time, parsed alerts.ParsedAlert) (*models.AlertMessage, error) {
	message := &models.AlertMessage{
		AlertId:          s.GenerateUuid(uuid.UUID_TYPE_ALERT_MESSAGE),
		Uid:              uid,
		Sender:           sender,
		Text:             text,
		Reference:        parsed.Reference,
		Outcome:          string(parsed.Outcome),
		ReceivedUnixTime: receivedAt.Unix(),
		CreatedUnixTime:  time.Now().Unix(),
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

	var matches []*models.Account

	for i := 0; i < len(accounts); i++ {
		if accounts[i].Hidden || accounts[i].Type == models.ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS {
			continue
		}

		if m := reAccountNameLast4.FindStringSubmatch(accounts[i].Name); m != nil && m[1] == parsed.Last4 {
			matches = append(matches, accounts[i])
		}
	}

	if len(matches) == 1 {
		return matches[0], false, nil
	}

	for i := 0; i < len(accounts); i++ {
		if accounts[i].Name == unmatchedAccountName {
			return accounts[i], true, nil
		}
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

// isDuplicateOwnTransfer reports whether this user already recorded the same own-account
// transfer (same source, same destination, same amount, tagged "Auto (SMS)") from the other
// side's alert, within 3 days either side of this alert's received time. It is checked before
// creating any own-account transfer, regardless of which side (debit or credit) is processed
// first, so that whichever SMS arrives second is recognised as a duplicate rather than
// recording the same physical transfer twice.
func (s *AlertService) isDuplicateOwnTransfer(c core.Context, uid int64, srcAccountId int64, dstAccountId int64, amount int64, receivedAt time.Time, autoSmsTagId int64) (bool, error) {
	minTime := utils.GetMinTransactionTimeFromUnixTime(receivedAt.Add(-3 * 24 * time.Hour).Unix())
	maxTime := utils.GetMaxTransactionTimeFromUnixTime(receivedAt.Add(3 * 24 * time.Hour).Unix())

	var candidates []*models.Transaction
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND type=? AND account_id=? AND related_account_id=? AND amount=? AND transaction_time>=? AND transaction_time<=?",
		uid, false, models.TRANSACTION_DB_TYPE_TRANSFER_OUT, srcAccountId, dstAccountId, amount, minTime, maxTime).Find(&candidates)

	if err != nil {
		return false, err
	}

	if len(candidates) == 0 {
		return false, nil
	}

	transactionIds := make([]int64, len(candidates))

	for i := 0; i < len(candidates); i++ {
		transactionIds[i] = candidates[i].TransactionId
	}

	tagIdsByTransaction, err := TransactionTags.GetAllTagIdsOfTransactions(c, uid, transactionIds)

	if err != nil {
		return false, err
	}

	for i := 0; i < len(candidates); i++ {
		tagIds := tagIdsByTransaction[candidates[i].TransactionId]

		for j := 0; j < len(tagIds); j++ {
			if tagIds[j] == autoSmsTagId {
				return true, nil
			}
		}
	}

	return false, nil
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

	history := func(payeeKey string) (alerts.HistoryHit, bool) {
		transactions, err := Transactions.GetTransactionsByMaxTime(c, uid, 0, 0, 0, nil, nil, nil, false, "", payeeKey, core.MATCH_MODE_IGNORE_CASE, false, 1, 20, false, true)

		if err != nil {
			return alerts.HistoryHit{}, false
		}

		for i := 0; i < len(transactions); i++ {
			transaction := transactions[i]

			if alerts.PayeeKey(transaction.Comment) != payeeKey {
				continue
			}

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
				continue
			}

			if parsed.Direction == alerts.Credit && transaction.Type != models.TRANSACTION_DB_TYPE_INCOME {
				continue
			}

			category := categoryById[transaction.CategoryId]

			if category == nil || category.Hidden {
				continue
			}

			return alerts.HistoryHit{CategoryId: transaction.CategoryId}, true
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

// createTransaction builds and saves the transaction for a parsed alert (step 9); a credit
// into one of this user's own accounts is recorded as a transfer whose source is the other
// account, so the resolved account ends up on the transfer's destination side
func (s *AlertService) createTransaction(c core.Context, uid int64, account *models.Account, parsed alerts.ParsedAlert, classification alerts.Classification, needsReview bool, text string, receivedAt time.Time, tagIds map[string]int64) (*models.Transaction, string, error) {
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
		categoryId, err := s.ensureCategory(c, uid, models.CATEGORY_TYPE_TRANSFER, categoryNameBankTransfer, categoryNameOtherTransfer)

		if err != nil {
			return nil, "", err
		}

		otherAccount, err := Accounts.GetAccountByAccountId(c, uid, classification.OtherAccountId)

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

	transactionTagIds := []int64{tagIds[tagAutoSms]}

	if needsReview {
		transactionTagIds = append(transactionTagIds, tagIds[tagNeedsReview])
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
// demand, skipping any that already exist, and returns their ids by name
func (s *AlertService) ensureTags(c core.Context, uid int64) (map[string]int64, error) {
	tags := []*models.TransactionTag{
		{Uid: uid, Name: tagAutoSms},
		{Uid: uid, Name: tagNeedsReview},
		{Uid: uid, Name: tagBalanceMismatch},
	}

	if err := TransactionTags.CreateTags(c, uid, tags, true); err != nil {
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
// surfaced to the caller as a log, not as an Ingest error.
func (s *AlertService) checkBalance(c core.Context, uid int64, accountId int64, parsed alerts.ParsedAlert, transaction *models.Transaction, balanceMismatchTagId int64) error {
	if !parsed.HasBalance {
		return nil
	}

	account, err := Accounts.GetAccountByAccountId(c, uid, accountId)

	if err != nil {
		return err
	}

	if account.Balance == parsed.Balance {
		return nil
	}

	return Transactions.BatchAddTagsToTransactions(c, uid, []*models.Transaction{transaction}, map[int64][]int64{transaction.TransactionId: {balanceMismatchTagId}})
}

// formatRupees renders an amount in paise as a rupee decimal string ("250.00")
func formatRupees(paise int64) string {
	return fmt.Sprintf("%d.%02d", paise/100, paise%100)
}
