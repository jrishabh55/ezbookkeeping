package api

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// The template endpoint must preserve unrelated profile fields and take ownership from authentication, not JSON.
func TestInvoiceEmailTemplateUpdateHandlerPreservesBusinessDetailsAndUsesAuthenticatedUID(t *testing.T) {
	config := &settings.Config{DatabaseConfig: &settings.DatabaseConfig{DatabaseType: settings.Sqlite3DbType, DatabasePath: filepath.Join(t.TempDir(), "template-update-api.db"), MaxIdleConnection: 1}}
	settings.SetCurrentConfig(config)
	require.NoError(t, datastore.InitializeDataStore(config))
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.InvoiceProfile)))
	ctx := core.NewNullContext()
	original := &models.InvoiceProfile{Uid: 101, BusinessName: "Business", AddressLines: "Address", Phone: "Phone", Website: "https://example.com", LogoDataUrl: "data:image/png;base64,stored", HideLogo: true, DefaultNotes: "Notes", NumberPrefix: "OWN", DefaultTermsDays: 15, Gstin: "07AAAAA0000A1Z5", DefaultSac: "998314", DefaultCurrency: "USD", EmailSubjectTemplate: "Old subject", EmailBodyTemplate: "<p>Old body</p>"}
	other := &models.InvoiceProfile{Uid: 202, BusinessName: "Other Business", NumberPrefix: "OTHER", EmailSubjectTemplate: "Other subject", EmailBodyTemplate: "<p>Other body</p>"}
	require.NoError(t, Invoices.invoices.SaveProfile(ctx, original))
	require.NoError(t, Invoices.invoices.SaveProfile(ctx, other))
	result, apiErr := Invoices.InvoiceEmailTemplateUpdateHandler(invoiceEmailPostContext(101, `{"uid":"202","subject":"Saved {{invoice_number}}","bodyHtml":""}`))
	require.Nil(t, apiErr)
	require.NotNil(t, result)
	expected := original.ToInvoiceProfileResponse()
	expected.EmailSubjectTemplate = "Saved {{invoice_number}}"
	expected.EmailBodyTemplate = ""
	require.Equal(t, expected, result)
	savedOther, err := Invoices.invoices.GetProfile(ctx, 202)
	require.NoError(t, err)
	require.Equal(t, other, savedOther)
	saved, err := Invoices.invoices.GetProfile(ctx, 101)
	require.NoError(t, err)
	for _, payload := range []string{
		`{}`, `{"bodyHtml":""}`, `{"subject":"Missing body"}`, `{"subject":"Null body","bodyHtml":null}`, `{"subject":"","bodyHtml":""}`,
		`{"subject":"Line\nbreak","bodyHtml":""}`, `{"subject":"{{unknown}}","bodyHtml":""}`, `{"subject":"Invoice","bodyHtml":"<span data-invoice-variable=\"unknown\">x</span>"}`,
	} {
		_, apiErr = Invoices.InvoiceEmailTemplateUpdateHandler(invoiceEmailPostContext(101, payload))
		require.NotNil(t, apiErr, payload)
		unchanged, err := Invoices.invoices.GetProfile(ctx, 101)
		require.NoError(t, err)
		require.Equal(t, saved, unchanged)
	}
	_, apiErr = Invoices.InvoiceEmailTemplateUpdateHandler(invoiceEmailPostContext(0, `{"subject":"Invoice","bodyHtml":""}`))
	require.Equal(t, errs.ErrUserIdInvalid, apiErr)
	payload, err := json.Marshal(map[string]string{"subject": "New profile", "bodyHtml": `<p><span data-invoice-variable="invoice_number">Sample</span><img src="x"></p>`})
	require.NoError(t, err)
	result, apiErr = Invoices.InvoiceEmailTemplateUpdateHandler(invoiceEmailPostContext(303, string(payload)))
	require.Nil(t, apiErr)
	require.NotNil(t, result)
	created := result.(*models.InvoiceProfileResponse)
	require.Equal(t, "INV", created.NumberPrefix)
	require.Equal(t, int32(7), created.DefaultTermsDays)
	require.Empty(t, created.BusinessName)
	require.Equal(t, `<p><span data-invoice-variable="invoice_number">{{invoice_number}}</span></p>`, created.EmailBodyTemplate)
	require.Equal(t, "New profile", created.EmailSubjectTemplate)
}
