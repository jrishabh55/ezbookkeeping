package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/validators"
)

func invoiceEmailPostContext(uid int64, payload string) *core.WebContext {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/api/v1/invoice_profile/preview_email.json", strings.NewReader(payload))
	context.Request.Header.Set("Content-Type", "application/json")
	web := &core.WebContext{Context: context}
	web.SetTokenClaims(&core.UserTokenClaims{Uid: uid})
	return web
}

// A handler that drops omitted template fields or leaks them into public DTOs breaks this endpoint test.
func TestInvoiceEmailProfileUpdateHandlerPreservesOmittedTemplatesAndPreviewsUnsavedDraft(t *testing.T) {
	config := &settings.Config{RootUrl: "https://books.example/", DatabaseConfig: &settings.DatabaseConfig{DatabaseType: settings.Sqlite3DbType, DatabasePath: filepath.Join(t.TempDir(), "email-api.db"), MaxIdleConnection: 1}}
	settings.SetCurrentConfig(config)
	require.NoError(t, datastore.InitializeDataStore(config))
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.InvoiceProfile), new(models.Invoice), new(models.InvoiceLine), new(models.InvoicePayment), new(models.Transaction), new(models.Account)))
	engine := binding.Validator.Engine().(*validator.Validate)
	require.NoError(t, engine.RegisterValidation("notBlank", validators.NotBlank))
	_, apiErr := Invoices.InvoiceProfileUpdateHandler(invoiceEmailPostContext(123, `{"numberPrefix":"INV","emailSubjectTemplate":"Private {{invoice_number}}","emailBodyTemplate":"<p>Private body</p>"}`))
	require.Nil(t, apiErr)
	result, apiErr := Invoices.InvoiceProfileUpdateHandler(invoiceEmailPostContext(123, `{"numberPrefix":"OLD","businessName":"Older client"}`))
	require.Nil(t, apiErr)
	profile := result.(*models.InvoiceProfileResponse)
	require.Equal(t, "Private {{invoice_number}}", profile.EmailSubjectTemplate)
	require.Equal(t, "<p>Private body</p>", profile.EmailBodyTemplate)
	result, apiErr = Invoices.InvoiceEmailPreviewHandler(invoiceEmailPostContext(123, `{"subject":"Draft subject","bodyHtml":""}`))
	require.Nil(t, apiErr)
	preview := result.(*models.InvoiceEmailPreviewResponse)
	require.Equal(t, "Draft subject", preview.Subject)
	require.Empty(t, preview.BodyHtml)
	require.True(t, preview.Sample)
	result, apiErr = Invoices.InvoiceProfileGetHandler(invoiceEmailPostContext(123, `{}`))
	require.Nil(t, apiErr)
	require.Equal(t, "Private {{invoice_number}}", result.(*models.InvoiceProfileResponse).EmailSubjectTemplate)
	_, apiErr = Invoices.InvoiceEmailPreviewHandler(invoiceEmailPostContext(0, `{}`))
	require.Equal(t, errs.ErrUserIdInvalid, apiErr)
	_, apiErr = Invoices.InvoiceEmailPreviewHandler(invoiceEmailPostContext(123, `{"invoiceId":"999"}`))
	require.Equal(t, errs.ErrInvoiceNotFound, apiErr)
	_, apiErr = Invoices.InvoiceProfileUpdateHandler(invoiceEmailPostContext(123, `{"numberPrefix":"INV","emailSubjectTemplate":""}`))
	require.NotNil(t, apiErr)
	_, apiErr = Invoices.InvoiceEmailPreviewHandler(invoiceEmailPostContext(123, `{"subject":"{{unknown}}"}`))
	require.NotNil(t, apiErr)
	require.Contains(t, apiErr.Error(), "unknown")
}

func TestInvoiceEmailRequestsDistinguishMissingAndEmptyRichBody(t *testing.T) {
	var request models.InvoiceSendRequest
	require.NoError(t, json.Unmarshal([]byte(`{"id":"42","to":"recipient@example.com"}`), &request))
	require.Nil(t, request.BodyHtml)
	require.Nil(t, request.Message)
	require.NoError(t, json.Unmarshal([]byte(`{"bodyHtml":"","message":"legacy"}`), &request))
	require.NotNil(t, request.BodyHtml)
	require.Empty(t, *request.BodyHtml)
	require.Equal(t, "legacy", *request.Message)
	var preview models.InvoiceEmailPreviewRequest
	require.NoError(t, json.Unmarshal([]byte(`{"invoiceId":"42","bodyHtml":""}`), &preview))
	require.Equal(t, int64(42), preview.InvoiceId)
	require.NotNil(t, preview.BodyHtml)
	require.Nil(t, preview.Subject)
}
