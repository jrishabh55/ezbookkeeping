package alerts

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPayeeKey(t *testing.T) {
	assert.Equal(t, "JOHN DOE", PayeeKey("JOHN DOE"))
	assert.Equal(t, "JOHN DOE", PayeeKey("UPI-JOHN  DOE-JOHN99@OKAXIS-PUNB0000000-512345678901-PAYMENT"))
	assert.Equal(t, "JOHN DOE", PayeeKey("IMPS-512345678901-JOHN DOE-HDFC-XXXXXXXX1234-SALARY"))
	assert.Equal(t, "ACME LTD", PayeeKey("NEFT CR-ICIC0000001-ACME LTD-JOHN DOE-ICICN12345"))
	assert.Equal(t, "JOHN DOE", PayeeKey("50100000000000-TPT-SALARY-JOHN DOE"))
	assert.Equal(t, "", PayeeKey("INTEREST PAID TILL 31-MAR-2026"))
}

func TestClassifyOwnAccountByLast4(t *testing.T) {
	own := []OwnAccount{{ID: 1, Name: "HDFC Bank 1234", Last4: "1234"}, {ID: 2, Name: "Savings 5678", Last4: "5678"}}
	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "SELF"}
	c := Classify(alert, 1, own, noHistory, nil, "IMPS SENT FROM A/C XX1234 TO A/C XXXXXX5678")
	assert.Equal(t, Classification{IsTransfer: true, OtherAccountId: 2}, c)
}

func TestClassifyHistory(t *testing.T) {
	hist := func(k string) (HistoryHit, bool) {
		if k == "JOHN DOE" {
			return HistoryHit{CategoryId: 42}, true
		}
		return HistoryHit{}, false
	}
	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "JOHN DOE"}
	assert.Equal(t, Classification{CategoryId: 42}, Classify(alert, 1, nil, hist, nil, "SENT RS.1 TO JOHN DOE"))
}

func TestClassifyHistoryTransferTarget(t *testing.T) {
	hist := func(k string) (HistoryHit, bool) {
		return HistoryHit{IsTransfer: true, OtherAccountId: 9}, k == "CRED CLUB"
	}
	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "CRED CLUB"}
	assert.Equal(t, Classification{IsTransfer: true, OtherAccountId: 9}, Classify(alert, 1, nil, hist, nil, "SENT TO CRED CLUB"))
}

func TestClassifyKeywordThenFallback(t *testing.T) {
	kw := []KeywordRule{{Pattern: regexp.MustCompile(`ZOMATO|SWIGGY`), Direction: Debit, CategoryId: 7}}
	food := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "ZOMATO LTD"}
	assert.Equal(t, Classification{CategoryId: 7}, Classify(food, 1, nil, noHistory, kw, "SPENT AT ZOMATO LTD"))
	unknown := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "SOMEONE"}
	assert.Equal(t, Classification{NeedsReview: true}, Classify(unknown, 1, nil, noHistory, kw, "SENT TO SOMEONE"))
}

func noHistory(string) (HistoryHit, bool) { return HistoryHit{}, false }

// Fix round 1: Issue 1 - reAnyLast4 context
func TestClassifyPromoLast4NoTransfer(t *testing.T) {
	own := []OwnAccount{{ID: 1, Name: "HDFC Bank 9999", Last4: "9999"}, {ID: 2, Name: "Card 5678", Last4: "5678"}}
	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Credit, Amount: 2000, Last4: "9999", Counterparty: "RAJ TRADERS"}
	c := Classify(alert, 1, own, noHistory, nil, "Rs.2000 credited to A/c XX9999 via NEFT from RAJ TRADERS. Download our app to link Card XX5678 for cashback.")
	assert.Equal(t, Classification{NeedsReview: true}, c, "should not treat promo card mention as transfer")
}

// Fix round 1: Issue 2 - reNotName bank fees
func TestPayeeKeyBankFees(t *testing.T) {
	assert.Equal(t, "", PayeeKey("CHARGES LEVIED"))
	assert.Equal(t, "", PayeeKey("MAB CHARGES"))
	assert.Equal(t, "", PayeeKey("ATM CARD AMC"))
	assert.Equal(t, "", PayeeKey("GST ON CHARGES"))
	assert.Equal(t, "", PayeeKey("ATM WDL CHGS"))
	assert.Equal(t, "", PayeeKey("DEBIT CARD ANNUAL FEE"))
	assert.Equal(t, "", PayeeKey("CASH WITHDRAWAL ATM"))
}

// Fix round 1: Issue 3 - UPI with hyphens in name
func TestPayeeKeyUPIWithHyphens(t *testing.T) {
	assert.Equal(t, "RAVI-KUMAR AND SONS", PayeeKey("UPI-RAVI-KUMAR AND SONS-ravi@okaxis-REF"))
	assert.Equal(t, "JOHN DOE", PayeeKey("UPI-JOHN  DOE-JOHN99@OKAXIS-PUNB0000000-512345678901-PAYMENT"))
}

// Fix round 1: Issue 4 - POS with digits in merchant name
func TestPayeeKeyPOSWithDigits(t *testing.T) {
	key1 := PayeeKey("POS 1234XXXX AMAZON SELLER7 BLR IN")
	key2 := PayeeKey("POS 1234XXXX AMAZON SELLER9 DEL IN")
	assert.NotEqual(t, key1, key2, "different outlet numbers should produce different keys")
	assert.Equal(t, "AMAZON SELLER7 BLR IN", key1)
	assert.Equal(t, "AMAZON SELLER9 DEL IN", key2)
}

// Fix round 1: Issue 5 - Ambiguous last-4 (multiple accounts)
func TestClassifyAmbiguousLast4(t *testing.T) {
	own := []OwnAccount{{ID: 1, Name: "HDFC Bank 1234", Last4: "1234"}, {ID: 2, Name: "Card 5678", Last4: "5678"}, {ID: 3, Name: "Savings 5678", Last4: "5678"}}
	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 100, Last4: "1234", Counterparty: "JOHN DOE"}
	c := Classify(alert, 1, own, noHistory, nil, "SENT TO A/C XX5678")
	assert.Equal(t, Classification{NeedsReview: true}, c, "ambiguous last-4 should not be treated as transfer")
}

// Fix round 1: Issue 6 - Remove redundant ToUpper
// This is implicitly tested by the other tests; the behavior should not change

func TestClassifyIbFundsTransferDrLast4(t *testing.T) {
	own := []OwnAccount{{ID: 1, Name: "HDFC Bank 7731", Last4: "7731"}, {ID: 2, Name: "HDFC Bank 4071", Last4: "4071"}}
	alert := ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 20000000, Last4: "7731"}
	c := Classify(alert, 1, own, noHistory, nil, "UPDATE: INR 2,00,000.00 DEBITED FROM HDFC BANK XX7731 ON 08-OCT-26. INFO: IB FUNDS TRANSFER DR-XXXXXXXXXX4071-RISHABH JAIN. AVL BAL:INR 46,688.05")
	assert.Equal(t, Classification{IsTransfer: true, OtherAccountId: 2}, c)
}
