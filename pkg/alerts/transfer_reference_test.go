package alerts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransferReference(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"pnb-neft-debit", "Ac XX4215 debited Rs.200000.00 on 30-10-2026 16:36:10 for NEFT thru MBS reqid 793227839 from PUNBY26303389748/0793227839/Ri. Bal Rs.170137.51 CR.Helpline 18001800/18002021. Not u? Fwd this SMS to 9264092640 to block MBS-PNB", "PUNBY26303389748"},
		{"hdfc-neft-credit", "Update! INR 2,00,000.00 deposited in HDFC Bank A/c XX7731 on 30-OCT-26 for NEFT Cr-PUNB0042800-RISHABH JAIN S O HARISH KUMAR-Rishabh jain-PUNBY26303389748.Avl bal INR 2,90,168.45. Cheque deposits in A/C are subject to clearing", "PUNBY26303389748"},
		{"pnb-neft-debit-x", "Ac XX4215 debited Rs.120000.00 on 29-10-2026 15:10:02 for NEFT thru MBS reqid 792945151 from PUNBX26302969498/0792945151/Ri. Bal Rs.370137.51 CR", "PUNBX26302969498"},
		{"rtgs", "INR 5,00,000.00 deposited in HDFC Bank A/c XX7731 on 08-OCT-26 for RTGS Cr-SCBL0036078-ACME LTD-SCBLR12026100800953867.Avl bal INR 6,00,000.00", "SCBLR12026100800953867"},
		{"upi-has-none", "Sent Rs.250.00\nFrom HDFC Bank A/C *1234\nTo FOOD CORNER\nOn 05/10/26\nRef 512345678901", ""},
		{"utr-shape-without-neft-or-rtgs", "Ac XX4215 debited Rs.100.00 ref PUNBY26303389748", ""},
		{"ifsc-is-not-a-utr", "INR 1,000.00 deposited in HDFC Bank A/c XX7731 for NEFT Cr-PUNB0042800-JOHN DOE", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assert.Equal(t, c.want, TransferReference(c.text)) })
	}
}
