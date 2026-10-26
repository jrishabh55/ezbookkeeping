package api

import (
	"encoding/json"
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

func TestPublicInvoiceResponseHasNoIdsOrPayments(t *testing.T) {
	invoice := &models.Invoice{InvoiceId: 1234, Uid: 99, CustomerId: 5678, Number: "CA/2026-27/01", Currency: "INR", Total: 37000000, PublicToken: "secret"}
	lines := []*models.InvoiceLine{{LineId: 1, ItemName: "Consulting", Quantity: 100, Rate: 37000000, Amount: 37000000}}
	profile := &models.InvoiceProfile{Uid: 99, BusinessName: "Rishabh Jain", NumberPrefix: "CA", DefaultNotes: "internal"}

	data, err := json.Marshal(invoice.ToPublicInvoiceResponse(lines, 100, profile))
	assert.Nil(t, err)

	var resp map[string]any
	assert.Nil(t, json.Unmarshal(data, &resp))

	for _, key := range []string{"id", "uid", "customerId", "payments", "publicToken", "kind"} {
		_, has := resp[key]
		assert.False(t, has, key)
	}

	assert.Equal(t, "CA/2026-27/01", resp["number"])
	assert.Equal(t, float64(100), resp["received"])
	assert.Equal(t, float64(100), resp["lines"].([]any)[0].(map[string]any)["quantity"])

	profileResp := resp["profile"].(map[string]any)
	assert.Equal(t, "Rishabh Jain", profileResp["businessName"])

	for _, key := range []string{"uid", "defaultNotes", "numberPrefix", "defaultTermsDays"} {
		_, has := profileResp[key]
		assert.False(t, has, key)
	}

	assert.NotContains(t, string(data), "secret")
}
