package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/validators"
)

// newTestWebContext wraps a GET request to path (with its query string) in a web context, like bindApi does
func newTestWebContext(path string) *core.WebContext {
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = httptest.NewRequest(http.MethodGet, path, nil)

	return &core.WebContext{Context: ginContext}
}

func TestInvoicePublicPdfHandlerUnknownTokenIsNotFound(t *testing.T) {
	// a token of the wrong length never reaches the database, so this needs no store
	pdf, fileName, err := Invoices.InvoicePublicPdfHandler(newTestWebContext("/api/public/invoices/pdf?token=unknown"))
	assert.Nil(t, pdf)
	assert.Equal(t, "", fileName)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	_, _, err = Invoices.InvoicePublicPdfHandler(newTestWebContext("/api/public/invoices/pdf"))
	assert.Equal(t, errs.ErrInvoiceNotFound, err)
}

func TestIsAllowedInvoiceLogo(t *testing.T) {
	assert.True(t, isAllowedInvoiceLogo(""))
	assert.True(t, isAllowedInvoiceLogo("data:image/png;base64,iVBORw0KGgo="))
	assert.True(t, isAllowedInvoiceLogo("data:image/jpeg;base64,/9j/4AAQ"))
	assert.False(t, isAllowedInvoiceLogo("data:image/svg+xml;base64,PHN2Zz4="))
	assert.False(t, isAllowedInvoiceLogo("https://example.com/logo.png"))
}

func TestPublicInvoiceResponseOmitsHiddenLogoWithoutRemovingUpload(t *testing.T) {
	profile := &models.InvoiceProfile{LogoDataUrl: "data:image/png;base64,stored-upload"}
	invoice := &models.Invoice{}
	assert.Equal(t, profile.LogoDataUrl, invoice.ToPublicInvoiceResponse(nil, 0, profile).Profile.LogoDataUrl)
	assert.NoError(t, json.Unmarshal([]byte(`{"HideLogo":true}`), profile))
	assert.Empty(t, invoice.ToPublicInvoiceResponse(nil, 0, profile).Profile.LogoDataUrl)
	assert.Equal(t, "data:image/png;base64,stored-upload", profile.ToInvoiceProfileResponse().LogoDataUrl)
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

func TestInvoiceCreateHandlerRefusesAMilestoneWithoutItsEstimate(t *testing.T) {
	// the web server registers this at startup; the request below must bind so the handler's own check is what refuses it
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		_ = v.RegisterValidation("notBlank", validators.NotBlank)
	}

	body := `{"number":"INV/2026-27/01","customerId":"1","invoiceDate":"2026-10-30","dueDate":"2026-11-06","milestoneId":"5",` +
		`"lines":[{"itemName":"Advance","quantity":100,"rate":66660}]}`
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = httptest.NewRequest(http.MethodPost, "/api/v1/invoices/add.json", strings.NewReader(body))
	ginContext.Request.Header.Set("Content-Type", "application/json")

	_, err := Invoices.InvoiceCreateHandler(&core.WebContext{Context: ginContext})
	assert.NotNil(t, err)
	assert.Equal(t, errs.ErrIncompleteOrIncorrectSubmission.Code(), err.Code())
	assert.Equal(t, http.StatusBadRequest, err.HttpStatusCode)
}
