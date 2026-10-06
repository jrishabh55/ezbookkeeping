package alerts

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name, sender, text string
		want               ParsedAlert
	}{
		{"upi-debit", "XX-HDFCBK", "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nOn 05/10/26\nRef 512345678901\nNot You?\nCall 18002586161/SMS BLOCK UPI to 7308080808",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 25000, Last4: "1234", Counterparty: "FOOD CORNER", Reference: "512345678901"}},
		{"credit-with-balance", "XX-HDFCBK", "Update! INR 1,23,456.78 deposited in HDFC Bank A/c XX1234 on 05-OCT-26 for NEFT Cr-ACME LTD. Avl bal INR 2,00,000.00. Cheque deposits in A/C are subject to clearing",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Credit, Amount: 12345678, Last4: "1234", Counterparty: "ACME LTD", Balance: 20000000, HasBalance: true}},
		{"card-spend", "XX-HDFCBK", "Spent Rs.1,234 On HDFC Bank Card 5678 At CITY FUELS On 2026-10-05:12:34:56.Not You? To Block+Reissue Call 18002323232/SMS BLOCK CC 5678 to 7308080808",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 123400, Last4: "5678", Counterparty: "CITY FUELS"}},
		{"lakh-format", "XX-YESBNK", "INR 1,00,000.00 debited from A/c XX1234 on 05-10-2026 to JOHN DOE. Avl Bal INR 4.00",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 10000000, Last4: "1234", Counterparty: "JOHN DOE", Balance: 400, HasBalance: true}},
		{"no-decimals", "XX-PNBSMS", "Ac XX1234 debited Rs 500 dt 05-10-26 thru UPI ref no 512345678901. Bal Rs 137.51",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 50000, Last4: "1234", Reference: "512345678901", Balance: 13751, HasBalance: true}},
		{"one-decimal", "XX-PNBSMS", "Ac XX1234 credited INR500.5 dt 05-10-26. Bal INR 638.01",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Credit, Amount: 50050, Last4: "1234", Balance: 63801, HasBalance: true}},
		{"otp", "XX-HDFCBK", "OTP is 123456 for txn of Rs.250.00 at FOOD CORNER on HDFC Bank card ending 5678. Valid till 12:34. Do not share OTP",
			ParsedAlert{Outcome: OutcomeIgnored}},
		{"promo", "XX-HDFCBK", "Get Rs.5,000 cashback on your next EMI purchase! Apply now: hdfc.bank.in/x",
			ParsedAlert{Outcome: OutcomeIgnored}},
		{"due-reminder", "XX-HDFCBK", "Payment of Rs.17,512.00 for loan A/c XX1234 is due on 05-10-26. Please maintain sufficient balance",
			ParsedAlert{Outcome: OutcomeIgnored}},
		{"mandate", "XX-HDFCBK", "Your AutoPay mandate of Rs.500.00 for NETFLIX has been set up successfully",
			ParsedAlert{Outcome: OutcomeIgnored}},
		{"financial-but-unclear", "XX-HDFCBK", "Rs.500.00 transaction on A/c XX1234 could not be processed",
			ParsedAlert{Outcome: OutcomeUnparsed}},
		{"cashback-credit-not-ignored", "XX-HDFCBK", "Rs.50 cashback credited to your A/c XX1234 for txn dated 05-10-26. Avl Bal Rs.5000.00",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Credit, Amount: 5000, Last4: "1234", Balance: 500000, HasBalance: true}},
		{"offer-word-in-merchant-name", "XX-HDFCBK", "Spent Rs.500 On HDFC Bank Card 5678 At BIGBAZAAR OFFER STORE On 05-10-26",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 50000, Last4: "5678", Counterparty: "BIGBAZAAR OFFER STORE"}},
		{"collect-request-not-debit", "XX-HDFCBK", "Payment request of Rs.500 sent to shopkeeper@upi. Approve in BHIM app",
			ParsedAlert{Outcome: OutcomeIgnored}},
		{"acc-abbreviation", "XX-YESBNK", "INR 100.00 debited from Acc XX1234 on 05-10-26",
			ParsedAlert{Outcome: OutcomeParsed, Direction: Debit, Amount: 10000, Last4: "1234"}},
		{"dual-verb-with-promo-word", "XX-HDFCBK", "Rs.500 cashback debited and credited to A/c XX1234 on 05-10-26",
			ParsedAlert{Outcome: OutcomeUnparsed}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assert.Equal(t, c.want, Parse(c.sender, c.text)) })
	}
}

func TestParseFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/sms_fixtures.json")
	assert.Nil(t, err)

	var fixtures []struct {
		ID       string       `json:"id"`
		Sender   string       `json:"sender"`
		Text     string       `json:"text"`
		Expected *ParsedAlert `json:"expected"`
	}
	assert.Nil(t, json.Unmarshal(data, &fixtures))

	for _, f := range fixtures {
		if assert.NotNil(t, f.Expected, "fixture %s has no expected result yet", f.ID) {
			assert.Equal(t, *f.Expected, Parse(f.Sender, f.Text), "fixture %s: %s", f.ID, f.Text)
		}
	}
}
