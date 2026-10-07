package alerts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCardPaymentDetection(t *testing.T) {
	debit := "Sent Rs.5000.00\nFrom HDFC Bank A/C *1234\nTo CRED\nOn 05/10/26\nRef 512345678960"
	assert.True(t, IsFacilitatorPayment(Parse("XX-HDFCBK", debit), debit))
	upi := "Rs.5000.00 debited from A/c XX1234 to VPA cred.club@axisb Ref 512345678961"
	assert.True(t, IsFacilitatorPayment(Parse("XX-HDFCBK", upi), upi))
	credited := "INR 5,000.00 credited to A/c XX1234 by IMPS from JOHN DOE"
	assert.False(t, IsFacilitatorPayment(Parse("XX-HDFCBK", credited), credited))

	issuer := "Dear Customer, Payment of INR 945.70 has been received towards your ICICI Bank Credit Card XX5678 on 16-NOV-26 through UPI. Thank you."
	assert.True(t, IsCardPaymentReceived(Parse("XX-ICICIB", issuer), issuer))
	cred := "Payment of INR 5,311 was received for your HDFC Bank credit card XXXX-5678 on 02-Mar-26 and you have earned 100 CRED coins."
	assert.True(t, IsCardPaymentReceived(Parse("XX-CREDIN", cred), cred))
	cashback := "Posted | CashBack of Rs.276 to HDFC Bank Credit Card 5678 on 05/MAR/26 towards JOHN DOE posting Aug'22."
	assert.False(t, IsCardPaymentReceived(Parse("XX-HDFCBK", cashback), cashback))
}
