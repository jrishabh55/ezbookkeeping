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

func TestAccountExtendCardPaymentAccountChanged(t *testing.T) {
	stored := int64(7731)
	extend := &models.AccountExtend{CreditCardPaymentAccountId: &stored}

	assert.False(t, cardPaymentAccountChanged(0, extend))
	assert.False(t, cardPaymentAccountChanged(7731, extend)) // unchanged, even if 7731 was since deleted
	assert.True(t, cardPaymentAccountChanged(9260, extend))
	assert.True(t, cardPaymentAccountChanged(7731, nil))
	assert.True(t, cardPaymentAccountChanged(7731, &models.AccountExtend{}))
}
