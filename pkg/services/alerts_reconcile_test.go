package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestReconcileMatchesByReferenceAndAmount(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 512345678910", time.Now())
	rows := []*models.ImportTransaction{importRow(uid, "HDFC Bank 1234", models.TRANSACTION_DB_TYPE_EXPENSE, 25000, time.Now(), "UPI-FOOD CORNER-FOOD@OKAXIS-HDFC0000001-512345678910-UPI")}
	m, err := Alerts.MatchStatementRows(ctx, uid, rows)
	assert.Nil(t, err)
	assert.Equal(t, map[int]int64{0: r.TransactionId}, m)
}

func TestReconcileDoesNotMatchDifferentAmount(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nRef 512345678911", time.Now())
	rows := []*models.ImportTransaction{importRow(uid, "HDFC Bank 1234", models.TRANSACTION_DB_TYPE_EXPENSE, 25100, time.Now(), "UPI-FOOD CORNER-512345678911")}
	m, _ := Alerts.MatchStatementRows(ctx, uid, rows)
	assert.Empty(t, m)
}

func TestReconcileMarksNotInStatement(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.99.00\nFrom HDFC Bank A/C *1234\nTo JOHN DOE\nRef 512345678912", time.Now())
	acc := accountIdByName(t, ctx, uid, "HDFC Bank 1234")
	assert.Nil(t, Alerts.MarkNotInStatement(ctx, uid, acc, time.Now().Add(-time.Hour).Unix(), time.Now().Add(time.Hour).Unix()))
	assert.Contains(t, tagNamesOf(t, ctx, uid, r.TransactionId), "Not in statement")
}

// TestReconcileMarkStatementVerifiedIgnoresOtherUsersTransaction is the extra test required by
// ruling 7: a client-supplied id belonging to another user's transaction must be silently
// skipped by MarkStatementVerified rather than acted on, however "matched" the client claims it
// is.
func TestReconcileMarkStatementVerifiedIgnoresOtherUsersTransaction(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.150.00\nFrom HDFC Bank A/C *1234\nTo SHOP\nRef 512345678920", time.Now())

	_, otherUid := newAlertTestUser(t, "HDFC Bank 1234")
	otherR, _ := Alerts.Ingest(ctx, otherUid, "XX-HDFCBK", "Sent Rs.150.00\nFrom HDFC Bank A/C *1234\nTo SHOP\nRef 512345678921", time.Now())

	row := importRow(uid, "HDFC Bank 1234", models.TRANSACTION_DB_TYPE_EXPENSE, 15000, time.Now(), "UPI-SHOP-512345678920")

	// uid asks to verify otherR.TransactionId (not its own transaction): this must be skipped
	// rather than modifying another user's transaction.
	matches := map[int64]*models.ImportTransaction{otherR.TransactionId: row}
	assert.Nil(t, Alerts.MarkStatementVerified(ctx, uid, matches))

	assert.NotContains(t, tagNamesOf(t, ctx, uid, r.TransactionId), "Statement verified")
	assert.Contains(t, tagNamesOf(t, ctx, otherUid, otherR.TransactionId), "Auto (SMS)")
	assert.NotContains(t, tagNamesOf(t, ctx, otherUid, otherR.TransactionId), "Statement verified")
}

// TestReconcileOneSmsTransactionNotMatchedTwice is the extra test required by ruling 7: one SMS
// transaction must not be matched to two identical statement rows (ruling 2 - each side matches
// at most once).
func TestReconcileOneSmsTransactionNotMatchedTwice(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "Sent Rs.300.00\nFrom HDFC Bank A/C *1234\nTo CAFE\nRef 512345678930", time.Now())

	narration := "UPI-CAFE-512345678930"
	rows := []*models.ImportTransaction{
		importRow(uid, "HDFC Bank 1234", models.TRANSACTION_DB_TYPE_EXPENSE, 30000, time.Now(), narration),
		importRow(uid, "HDFC Bank 1234", models.TRANSACTION_DB_TYPE_EXPENSE, 30000, time.Now(), narration),
	}

	m, err := Alerts.MatchStatementRows(ctx, uid, rows)
	assert.Nil(t, err)
	assert.Len(t, m, 1)

	matchedToR := 0
	for _, txnId := range m {
		if txnId == r.TransactionId {
			matchedToR++
		}
	}
	assert.Equal(t, 1, matchedToR)
}

func TestReconcileMatchesSmsWithoutReference(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234")
	r, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "INR 100.00 debited from A/c XX1234 to JOHN DOE.", time.Now())
	rows := []*models.ImportTransaction{importRow(uid, "HDFC Bank 1234", models.TRANSACTION_DB_TYPE_EXPENSE, 10000, time.Now(), "UPI-JOHN DOE-JOHN@OKAXIS-512345678940-UPI")}
	m, err := Alerts.MatchStatementRows(ctx, uid, rows)
	assert.Nil(t, err)
	assert.Equal(t, map[int]int64{0: r.TransactionId}, m)
}

func TestReconcileOwnTransferMatchesBothStatements(t *testing.T) {
	ctx, uid := newAlertTestUser(t, "HDFC Bank 1234", "Savings 5678")
	r, _ := Alerts.Ingest(ctx, uid, "XX-HDFCBK", "INR 5,000.00 debited from A/c XX1234 on 05-10-2026 IMPS to A/c XXXXXX5678 Ref 512345678950", time.Now())
	out := importRow(uid, "HDFC Bank 1234", models.TRANSACTION_DB_TYPE_EXPENSE, 500000, time.Now(), "IMPS-512345678950-SELF")
	in := importRow(uid, "Savings 5678", models.TRANSACTION_DB_TYPE_INCOME, 500000, time.Now(), "IMPS-512345678950-SELF")

	m, err := Alerts.MatchStatementRows(ctx, uid, []*models.ImportTransaction{out})
	assert.Nil(t, err)
	assert.Equal(t, map[int]int64{0: r.TransactionId}, m)

	assert.Nil(t, Alerts.MarkStatementVerified(ctx, uid, map[int64]*models.ImportTransaction{r.TransactionId: out}))
	acc := accountIdByName(t, ctx, uid, "HDFC Bank 1234")
	assert.Nil(t, Alerts.MarkNotInStatement(ctx, uid, acc, time.Now().Add(-time.Hour).Unix(), time.Now().Add(time.Hour).Unix()))
	tags := tagNamesOf(t, ctx, uid, r.TransactionId)
	assert.Contains(t, tags, "Statement verified")
	assert.NotContains(t, tags, "Not in statement")

	// the destination account's statement still recognises the (now verified) transfer
	m, err = Alerts.MatchStatementRows(ctx, uid, []*models.ImportTransaction{in})
	assert.Nil(t, err)
	assert.Equal(t, map[int]int64{0: r.TransactionId}, m)
}
