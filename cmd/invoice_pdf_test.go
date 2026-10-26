package cmd

import (
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

func TestInvoicePdfDownloadReturnsPdfAttachmentWithUnicodeFilename(t *testing.T) {
	router := gin.New()
	router.GET("/invoice.pdf", bindPdf(func(c *core.WebContext) ([]byte, string, *errs.Error) {
		return []byte("%PDF-1.7\ninvoice document"), "Invoice_客户_2026-10-26.pdf", nil
	}, &settings.Config{}))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/invoice.pdf", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "application/pdf", response.Header().Get("Content-Type"))
	assert.Equal(t, "%PDF-1.7\ninvoice document", response.Body.String())
	header := response.Header().Get("Content-Disposition")
	disposition, params, err := mime.ParseMediaType(header)
	assert.NoError(t, err)
	assert.Equal(t, "attachment", disposition)
	assert.Equal(t, "Invoice_客户_2026-10-26.pdf", params["filename"])
	assert.False(t, strings.Contains(header, "客户"), "HTTP header must encode non-ASCII filename characters")
}
