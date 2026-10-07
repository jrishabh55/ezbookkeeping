package alerts

import "regexp"

// Card-bill apps only move money from a bank to a card; they are never an account of their own
var (
	reFacilitator         = regexp.MustCompile(`(?i)\bCRED\b|CRED\.CLUB`)
	reCardPaymentReceived = regexp.MustCompile(`(?is)\bpayment\b.{0,60}?\b(?:received|credited)\b.{0,60}?\bcard\b`)
)

// IsFacilitatorPayment reports a debit paid to a card-bill app (CRED), which does not name the card
func IsFacilitatorPayment(alert ParsedAlert, text string) bool {
	return alert.Direction == Debit && reFacilitator.MatchString(text)
}

// IsCardPaymentReceived reports a credit that confirms a bill payment into a credit card
// (not cashback or a refund)
func IsCardPaymentReceived(alert ParsedAlert, text string) bool {
	return alert.Direction == Credit && reCardPaymentReceived.MatchString(text)
}
