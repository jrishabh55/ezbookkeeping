package services

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

func TestIngestDuplicateReference(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	sms := "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nOn 05/10/26\nRef 512345678901"
	r1, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", sms, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r1.Outcome)
	r2, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", sms, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "duplicate", r2.Outcome)
	assert.Equal(t, 1, countTransactions(t, ctx, uid))
}

func TestIngestNeverResolvesOtherUsersAccount(t *testing.T) {
	ctx, other := newAlertTestUser(t, "HDFC Bank 1234")
	_, uid := newAlertTestUser(t, "PNB 9999")
	r, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 512345678902", time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r.Outcome)
	assert.Equal(t, 0, countTransactions(t, ctx, other))
	assert.Equal(t, "Unmatched alerts", accountNameOf(t, ctx, uid, r.TransactionId))
}

func TestIngestOwnTransferPair(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234", "Savings 5678")
	debit := "INR 5,000.00 debited from A/c XX1234 on 05-10-2026 IMPS to A/c XXXXXX5678 Ref 512345678903"
	credit := "INR 5,000.00 credited to A/c XX5678 on 05-10-2026 by IMPS from A/c XXXXXX1234"
	r1, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", debit, time.Now())
	r2, _ := Alerts.Ingest(ctx, uid, "XX-YESBNK", credit, time.Now())
	assert.Equal(t, "added", r1.Outcome)
	assert.Equal(t, "duplicate", r2.Outcome)
	assert.Equal(t, 1, countTransactions(t, ctx, uid))
}

func TestIngestIgnoresOtp(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "OTP is 123456 for txn of Rs.250.00 on card ending 1234", time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "ignored", r.Outcome)
	assert.Equal(t, 0, countTransactions(t, ctx, uid))
}

func TestIngestBalanceMismatchTag(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "INR 100.00 debited from A/c XX1234 to JOHN DOE. Avl Bal INR 5,000.00", time.Now())
	assert.Contains(t, tagNamesOf(t, ctx, uid, r.TransactionId), "Balance mismatch") // account balance is -100.00, SMS says 5,000.00
}

// TestIngestDoesNotDedupeDifferentReferencesWithinWindow is the failing-first test for I1: the
// 120s same-account/same-amount/same-direction heuristic (step 5) must not run at all when the
// current alert has its own reference, and must not treat a candidate with a different,
// non-empty reference as the same payment. Without the fix, two real back-to-back ₹10 payments
// with distinct references would have the second dropped as a false "duplicate".
func TestIngestDoesNotDedupeDifferentReferencesWithinWindow(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	sms1 := "Sent Rs.10.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 600000000001"
	sms2 := "Sent Rs.10.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 600000000002"
	r1, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", sms1, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r1.Outcome)
	r2, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", sms2, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r2.Outcome)
	assert.Equal(t, 2, countTransactions(t, ctx, uid))
}

// TestIngestBalanceMismatchTagFailureDoesNotBlockAdded is the failing-first test for I2: the
// alert row must be marked "added" (with its transaction id) as soon as CreateTransaction
// succeeds, and a failure in the best-effort balance-mismatch tagging afterwards must be logged,
// not surfaced as an Ingest error. The "Balance mismatch" tag is pre-created and hidden so that
// tagging it later is rejected, simulating that failure without needing to mock anything.
func TestIngestBalanceMismatchTagFailureDoesNotBlockAdded(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")

	tag := &models.TransactionTag{Uid: uid, Name: tagBalanceMismatch}
	assert.Nil(t, TransactionTags.CreateTags(ctx, uid, []*models.TransactionTag{tag}, true))
	assert.Nil(t, TransactionTags.HideTag(ctx, uid, []int64{tag.TagId}, true))

	r, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "INR 100.00 debited from A/c XX1234 to JOHN DOE. Avl Bal INR 5,000.00", time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r.Outcome)
	assert.Equal(t, 1, countTransactions(t, ctx, uid))

	status, err := Alerts.Status(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, int64(1), status.Counts["added"])
}

// TestIngestHistoryHitIgnoresMismatchedDirection is the failing-first test for I4: a payee
// history hit whose transaction type doesn't match the current alert's direction (here, a past
// expense being matched against a credit / refund) must be skipped, not used as-is (which would
// make CreateTransaction reject the new transaction with ErrTransactionCategoryTypeInvalid).
func TestIngestHistoryHitIgnoresMismatchedDirection(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	accountId := accountIdByName(t, ctx, uid, "HDFC Bank 1234")

	primary := &models.TransactionCategory{Uid: uid, Type: models.CATEGORY_TYPE_EXPENSE, Name: "Food & Drinks", Icon: 1, Color: "000000"}
	secondary := &models.TransactionCategory{Uid: uid, Type: models.CATEGORY_TYPE_EXPENSE, Name: "Food", Icon: 1, Color: "000000"}
	_, err := TransactionCategories.CreateCategories(ctx, uid, map[*models.TransactionCategory][]*models.TransactionCategory{nil: {primary}, primary: {secondary}})
	assert.Nil(t, err)

	pastExpense := &models.Transaction{
		Uid: uid, Type: models.TRANSACTION_DB_TYPE_EXPENSE, CategoryId: secondary.CategoryId, AccountId: accountId,
		Amount: 30000, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(time.Now().Add(-24 * time.Hour).Unix()),
		TimezoneUtcOffset: 330, Comment: "SWIGGY",
	}
	assert.Nil(t, Transactions.CreateTransaction(ctx, pastExpense, nil, nil))

	credit := "INR 300.00 credited to A/c XX1234 from SWIGGY Ref 600000000003"
	r, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", credit, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r.Outcome)
}

// TestIngestOwnTransferCreditThenDebitIsDuplicate is the failing-first test for I6: the
// own-account transfer dedupe check must run for both sides of a transfer, not only when the
// credit alert is processed after the debit. When the credit SMS arrives first and the debit SMS
// arrives second, the debit must be recognised as the other side of the same transfer already
// recorded, not as a brand new transfer.
func TestIngestOwnTransferCreditThenDebitIsDuplicate(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234", "Savings 5678")
	credit := "INR 5,000.00 credited to A/c XX5678 on 05-10-2026 by IMPS from A/c XXXXXX1234"
	debit := "INR 5,000.00 debited from A/c XX1234 on 05-10-2026 IMPS to A/c XXXXXX5678 Ref 600000000004"
	r1, err := Alerts.Ingest(ctx, uid, "XX-YESBNK", credit, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r1.Outcome)
	r2, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", debit, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "duplicate", r2.Outcome)
	assert.Equal(t, 1, countTransactions(t, ctx, uid))
}

// TestIngestAmbiguousLast4ResolvesToUnmatched is the test for M4: when more than one eligible
// account's name ends with the alert's last-4 digits, the account cannot be resolved
// unambiguously, so the alert must fall back to "Unmatched alerts" with review forced, rather
// than guessing one of them.
func TestIngestAmbiguousLast4ResolvesToUnmatched(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234", "ICICI Bank 1234")
	r, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 600000000005", time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r.Outcome)
	assert.Equal(t, "Unmatched alerts", accountNameOf(t, ctx, uid, r.TransactionId))
	assert.Contains(t, tagNamesOf(t, ctx, uid, r.TransactionId), "Needs review")
}

// TestDeleteAllAlertMessagesOnlyDeletesOwnRows verifies that DeleteAllAlertMessages is scoped
// strictly by uid: deleting one user's alert messages must never touch another user's rows.
func TestDeleteAllAlertMessagesOnlyDeletesOwnRows(t *testing.T) {
	ctx, uid1 := newAlertTestUser(t, "HDFC Bank 1234")
	_, uid2 := newAlertTestUser(t, "ICICI Bank 4321")

	otp := "OTP is 123456 for txn of Rs.250.00 on card ending "
	_, err := Alerts.Ingest(ctx, uid1, "XX-HDFCBK", otp+"1234", time.Now())
	assert.Nil(t, err)
	_, err = Alerts.Ingest(ctx, uid2, "XX-ICICI", otp+"4321", time.Now())
	assert.Nil(t, err)

	statusBefore, err := Alerts.Status(ctx, uid1)
	assert.Nil(t, err)
	assert.Equal(t, int64(1), statusBefore.Counts["ignored"])

	err = Alerts.DeleteAllAlertMessages(ctx, uid1)
	assert.Nil(t, err)

	statusAfter1, err := Alerts.Status(ctx, uid1)
	assert.Nil(t, err)
	assert.Equal(t, int64(0), statusAfter1.Counts["ignored"])

	statusAfter2, err := Alerts.Status(ctx, uid2)
	assert.Nil(t, err)
	assert.Equal(t, int64(1), statusAfter2.Counts["ignored"])
}

// TestIngestCommentTruncatesByRuneNotByte is the regression test for M2: the stored transaction
// comment must be truncated at a rune boundary, not a byte boundary, so a multi-byte character
// (e.g. "₹") that straddles the 255th byte is never split into invalid UTF-8.
func TestIngestCommentTruncatesByRuneNotByte(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")

	base := "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 700000000001\n"

	// Pad with plain ASCII so that exactly 254 runes (254 bytes, since they're all 1-byte
	// characters) precede the "₹", making "₹" the 255th rune; under the old `comment[:255]`
	// byte-slice, this would cut off after the "₹" character's first byte (0xE2), producing an
	// invalid UTF-8 tail. A few trailing characters after the "₹" push the total length past 255
	// runes so truncation actually triggers.
	asciiPrefixLen := 254 - utf8.RuneCountInString(base)
	if asciiPrefixLen < 0 {
		t.Fatalf("base SMS is already %d runes, longer than the 254 budget this test assumes", utf8.RuneCountInString(base))
	}

	text := base + strings.Repeat("A", asciiPrefixLen) + "₹" + "XYZ"
	expectedComment := string([]rune(text)[:255])

	r, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", text, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r.Outcome)

	tx, err := Transactions.GetTransactionByTransactionId(ctx, uid, r.TransactionId)
	assert.Nil(t, err)
	assert.True(t, utf8.ValidString(tx.Comment), "comment must be valid UTF-8, not a byte-sliced mid-character cut")
	assert.Equal(t, 255, utf8.RuneCountInString(tx.Comment))
	assert.Equal(t, expectedComment, tx.Comment)
	assert.Contains(t, tx.Comment, "₹")
	assert.NotContains(t, tx.Comment, "XYZ")
}

// TestIngestOwnTransferCreditWithoutSourceIsDuplicate is the failing-first test for I-1 (debit
// first): the debit names the destination, so it becomes a transfer; the credit on the
// destination doesn't name the source ("from JOHN DOE"), and must not become a second, income
// record of the same money.
func TestIngestOwnTransferCreditWithoutSourceIsDuplicate(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234", "Savings 5678")
	debit := "INR 5,000.00 debited from A/c XX1234 on 05-10-2026 IMPS to A/c XXXXXX5678 Ref 512345678903"
	credit := "INR 5,000.00 credited to A/c XX5678 on 05-10-2026 by IMPS from JOHN DOE"
	r1, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", debit, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r1.Outcome)
	r2, err := Alerts.Ingest(ctx, uid, "XX-YESBNK", credit, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "duplicate", r2.Outcome)
	assert.Equal(t, 1, countTransactions(t, ctx, uid))
}

// TestIngestOwnTransferDebitWithoutDestinationIsDuplicate is the failing-first test for I-1
// (credit first): the credit names the source, so it becomes a transfer; the debit on the source
// doesn't name the destination, and must not become a second, expense record of the same money.
func TestIngestOwnTransferDebitWithoutDestinationIsDuplicate(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234", "Savings 5678")
	credit := "INR 5,000.00 credited to A/c XX5678 on 05-10-2026 by IMPS from A/c XXXXXX1234"
	debit := "INR 5,000.00 debited from A/c XX1234 on 05-10-2026 IMPS to JOHN DOE Ref 512345678904"
	r1, err := Alerts.Ingest(ctx, uid, "XX-YESBNK", credit, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r1.Outcome)
	r2, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", debit, time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "duplicate", r2.Outcome)
	assert.Equal(t, 1, countTransactions(t, ctx, uid))
}

// TestIngestLearnsPayeeFromCorrectedSmsTransaction is the failing-first test for I-2: an SMS
// transaction's comment is the raw SMS, so payee history must find it through its alert row; once
// the user corrects its category, the next SMS from the same payee gets that category, unreviewed.
func TestIngestLearnsPayeeFromCorrectedSmsTransaction(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r1, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.450.00\nFrom HDFC Bank A/C *1234\nTo GREEN GROCERS\nRef 600000000010", time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r1.Outcome)
	assert.Contains(t, tagNamesOf(t, ctx, uid, r1.TransactionId), "Needs review")

	primary := &models.TransactionCategory{Uid: uid, Type: models.CATEGORY_TYPE_EXPENSE, Name: "Food & Drinks", Icon: 1, Color: "000000"}
	groceries := &models.TransactionCategory{Uid: uid, Type: models.CATEGORY_TYPE_EXPENSE, Name: "Groceries", Icon: 1, Color: "000000"}
	_, err = TransactionCategories.CreateCategories(ctx, uid, map[*models.TransactionCategory][]*models.TransactionCategory{nil: {primary}, primary: {groceries}})
	assert.Nil(t, err)

	// the user's correction: only the category changes
	tx, err := Transactions.GetTransactionByTransactionId(ctx, uid, r1.TransactionId)
	assert.Nil(t, err)
	corrected := *tx
	corrected.CategoryId = groceries.CategoryId
	assert.Nil(t, Transactions.ModifyTransaction(ctx, &corrected, false, len(tagNamesOf(t, ctx, uid, r1.TransactionId)), nil, nil, nil, nil))

	r2, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.120.00\nFrom HDFC Bank A/C *1234\nTo GREEN GROCERS\nRef 600000000011", time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r2.Outcome)
	tx2, err := Transactions.GetTransactionByTransactionId(ctx, uid, r2.TransactionId)
	assert.Nil(t, err)
	assert.Equal(t, groceries.CategoryId, tx2.CategoryId)
	assert.NotContains(t, tagNamesOf(t, ctx, uid, r2.TransactionId), "Needs review")
}

// TestIngestIgnoredKeepsNoTextAndUnparsedIsListed is the failing-first test for I-5: an ignored
// SMS (e.g. an OTP) is counted but its text isn't kept, and an unparsed SMS is listed in Status so
// the user can add it manually; the same unparsed SMS posted twice (M-10) is stored once.
func TestIngestIgnoredKeepsNoTextAndUnparsedIsListed(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "OTP is 123456 for txn of Rs.250.00 on card ending 1234", time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "ignored", r.Outcome)

	var rows []*models.AlertMessage
	assert.Nil(t, datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Where("uid=?", uid).Find(&rows))
	assert.Len(t, rows, 1)
	assert.Equal(t, "", rows[0].Text)
	assert.Equal(t, "XX-HDFCBK", rows[0].Sender)

	unparsed := "Rs.500.00 transaction on A/c XX1234 could not be processed"
	for i := 0; i < 2; i++ {
		r, err = Alerts.Ingest(ctx, uid, "XX-HDFCBK", unparsed, time.Now())
		assert.Nil(t, err)
		assert.Equal(t, "unparsed", r.Outcome)
	}

	status, err := Alerts.Status(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, int64(1), status.Counts["ignored"])
	assert.Equal(t, int64(1), status.Counts["unparsed"])
	assert.Len(t, status.RecentUnparsed, 1)
	assert.Equal(t, unparsed, status.RecentUnparsed[0].Text)
	assert.Equal(t, "XX-HDFCBK", status.RecentUnparsed[0].Sender)
}

// TestIngestSkipsBalanceCheckOnCreditCard covers M-7: a credit card SMS's "balance" is its
// available limit, never the book balance, so it must not be tagged "Balance mismatch".
func TestIngestSkipsBalanceCheckOnCreditCard(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	card := &models.Account{Uid: uid, Name: "HDFC Card 1234", Category: models.ACCOUNT_CATEGORY_CREDIT_CARD, Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Icon: 1, Color: "000000", Currency: "INR"}
	assert.Nil(t, Accounts.CreateAccounts(ctx, card, 0, nil, nil, time.UTC))
	r, err := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "INR 100.00 debited from A/c XX1234 to JOHN DOE. Avl Bal INR 5,000.00", time.Now())
	assert.Nil(t, err)
	assert.Equal(t, "added", r.Outcome)
	assert.NotContains(t, tagNamesOf(t, ctx, uid, r.TransactionId), "Balance mismatch")
}
