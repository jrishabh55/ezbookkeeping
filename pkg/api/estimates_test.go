package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
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

	_, err = Invoices.EstimatePublicDeclineHandler(newTestJsonWebContext("/api/public/estimates/decline.json", `{"token":"unknown","note":"`+strings.Repeat("x", 501)+`"}`))
	assert.NotNil(t, err) // the 500-character limit is checked when the body is bound

	_, err = Invoices.EstimatePublicDeclineHandler(newTestJsonWebContext("/api/public/estimates/decline.json", `{}`))
	assert.NotNil(t, err)
}
