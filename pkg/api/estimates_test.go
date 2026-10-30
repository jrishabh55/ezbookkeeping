package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// newTestJsonWebContext wraps a POST request with a JSON body in a web context, like bindApi does
func newTestJsonWebContext(path string, body string) *core.WebContext {
	ginContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	ginContext.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	ginContext.Request.Header.Set("Content-Type", "application/json")

	return &core.WebContext{Context: ginContext}
}

func TestEstimatePublicAnswerHandlersRefuseBadRequestsWithoutTheDatabase(t *testing.T) {
	// a token of the wrong length never reaches the database, so this needs no store
	result, err := Invoices.EstimatePublicAcceptHandler(newTestJsonWebContext("/api/public/estimates/accept.json", `{"token":"unknown"}`))
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	// the request model only guards against an outsized body (max=4000); the real 500-rune limit is the service's, after trimming
	_, err = Invoices.EstimatePublicDeclineHandler(newTestJsonWebContext("/api/public/estimates/decline.json", `{"token":"unknown","note":"`+strings.Repeat("x", 501)+`"}`))
	assert.Equal(t, errs.ErrEstimateResponseNoteTooLong, err)

	// a 500-rune note padded with whitespace is accepted (trimmed first): it reaches the token check, not a note error
	paddedNote := "  " + strings.Repeat("x", 500) + "  "
	result, err = Invoices.EstimatePublicDeclineHandler(newTestJsonWebContext("/api/public/estimates/decline.json", `{"token":"unknown","note":"`+paddedNote+`"}`))
	assert.Nil(t, result)
	assert.Equal(t, errs.ErrInvoiceNotFound, err)

	_, err = Invoices.EstimatePublicDeclineHandler(newTestJsonWebContext("/api/public/estimates/decline.json", `{}`))
	assert.NotNil(t, err)
}

func TestPublicEstimateResponseCarriesStatusAndPaymentTerms(t *testing.T) {
	estimate := &models.Invoice{InvoiceId: 1234, Kind: models.INVOICE_KIND_ESTIMATE, Number: "EST/2026-27/01", Currency: "INR", Total: 235999,
		EstimateStatus: models.ESTIMATE_STATUS_SENT, ResponseNote: "private"}
	resp := estimate.ToPublicInvoiceResponse(nil, 0, &models.InvoiceProfile{})
	resp.Milestones = models.ToPublicEstimateMilestoneResponses([]*models.EstimateMilestone{{MilestoneId: 7, EstimateId: 1234, Label: "Advance", Percent: 3000, InvoiceId: 9}})

	data, err := json.Marshal(resp)
	assert.Nil(t, err)

	var body map[string]any
	assert.Nil(t, json.Unmarshal(data, &body))
	assert.Equal(t, "estimate", body["kind"])
	assert.Equal(t, "sent", body["estimateStatus"])
	assert.Equal(t, map[string]any{"label": "Advance", "percent": float64(3000)}, body["milestones"].([]any)[0])
	assert.NotContains(t, string(data), "1234")
	assert.NotContains(t, string(data), "private")
}
