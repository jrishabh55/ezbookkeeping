package api

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestIsAllowedInvoiceLogo(t *testing.T) {
	assert.True(t, isAllowedInvoiceLogo(""))
	assert.True(t, isAllowedInvoiceLogo("data:image/png;base64,iVBORw0KGgo="))
	assert.True(t, isAllowedInvoiceLogo("data:image/jpeg;base64,/9j/4AAQ"))
	assert.False(t, isAllowedInvoiceLogo("data:image/svg+xml;base64,PHN2Zz4="))
	assert.False(t, isAllowedInvoiceLogo("https://example.com/logo.png"))
}

func TestValidateInvoiceTax(t *testing.T) {
	taxType, err := validateInvoiceTax("", 0)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_TAX_TYPE_NONE, taxType)

	taxType, err = validateInvoiceTax(models.INVOICE_TAX_TYPE_NONE, 1800)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_TAX_TYPE_NONE, taxType)

	taxType, err = validateInvoiceTax(models.INVOICE_TAX_TYPE_IGST, 1800)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_TAX_TYPE_IGST, taxType)

	taxType, err = validateInvoiceTax(models.INVOICE_TAX_TYPE_CGST_SGST, 10000)
	assert.Nil(t, err)
	assert.Equal(t, models.INVOICE_TAX_TYPE_CGST_SGST, taxType)

	_, err = validateInvoiceTax("vat", 1800)
	assert.Equal(t, errs.ErrInvoiceTaxTypeInvalid, err)

	_, err = validateInvoiceTax(models.INVOICE_TAX_TYPE_IGST, 0)
	assert.Equal(t, errs.ErrInvoiceTaxRateInvalid, err)

	_, err = validateInvoiceTax(models.INVOICE_TAX_TYPE_IGST, 10001)
	assert.Equal(t, errs.ErrInvoiceTaxRateInvalid, err)

	_, err = validateInvoiceTax(models.INVOICE_TAX_TYPE_NONE, -1)
	assert.Equal(t, errs.ErrInvoiceTaxRateInvalid, err)
}
