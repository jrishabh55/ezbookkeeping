package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
