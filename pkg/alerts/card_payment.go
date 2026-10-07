package alerts

import (
	"regexp"
	"strings"
)

// Card-bill apps only move money from a bank to a card; they are never an account of their own
var (
	// CRED's card-bill route (cred.club / "CRED CLUB", or a payee shown as just "CRED"); its
	// utility, store and Pay routes (cred.utility, cred.store, credpay.*) are ordinary purchases
	reFacilitator         = regexp.MustCompile(`(?i)CRED\.CLUB|\bCRED CLUB\b|^CRED$`)
	reCardPaymentReceived = regexp.MustCompile(`(?is)\bpayment\b.{0,60}?\b(?:received|credited)\b.{0,60}?\bcard\b`)
)

// IsFacilitatorPayment reports a debit paid to a card-bill app (CRED), which does not name the card
func IsFacilitatorPayment(alert ParsedAlert, text string) bool {
	if alert.Direction != Debit {
		return false
	}

	return reFacilitator.MatchString(alert.Counterparty) || reFacilitator.MatchString(strings.ReplaceAll(text, "\n", " "))
}

// IsCardPaymentReceived reports a credit that confirms a bill payment into a credit card
// (not cashback or a refund)
func IsCardPaymentReceived(alert ParsedAlert, text string) bool {
	return alert.Direction == Credit && reCardPaymentReceived.MatchString(text)
}
