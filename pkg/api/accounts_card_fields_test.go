package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestAccountExtendCardPaymentFields(t *testing.T) {
	day, from := 3, int64(7731)
	a := &models.Account{Category: models.ACCOUNT_CATEGORY_CREDIT_CARD,
		Extend: &models.AccountExtend{CreditCardPaymentDueDay: &day, CreditCardPaymentAccountId: &from}}
	r := a.ToAccountInfoResponse()
	assert.Equal(t, 3, *r.CreditCardPaymentDueDay)
	assert.Equal(t, int64(7731), *r.CreditCardPaymentAccountId)
}
