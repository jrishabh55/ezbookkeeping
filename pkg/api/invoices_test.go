package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsAllowedInvoiceLogo(t *testing.T) {
	assert.True(t, isAllowedInvoiceLogo(""))
	assert.True(t, isAllowedInvoiceLogo("data:image/png;base64,iVBORw0KGgo="))
	assert.True(t, isAllowedInvoiceLogo("data:image/jpeg;base64,/9j/4AAQ"))
	assert.False(t, isAllowedInvoiceLogo("data:image/svg+xml;base64,PHN2Zz4="))
	assert.False(t, isAllowedInvoiceLogo("https://example.com/logo.png"))
}
