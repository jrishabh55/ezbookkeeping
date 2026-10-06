package middlewares

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/core"
)

func TestIsAlertIngestToken(t *testing.T) {
	assert.True(t, IsAlertIngestToken(&core.UserTokenClaims{Type: core.USER_TOKEN_TYPE_ALERT_INGEST}))
	for _, other := range []core.TokenType{core.USER_TOKEN_TYPE_NORMAL, core.USER_TOKEN_TYPE_MCP, core.USER_TOKEN_TYPE_API, core.USER_TOKEN_TYPE_REQUIRE_2FA} {
		assert.False(t, IsAlertIngestToken(&core.UserTokenClaims{Type: other}))
	}
	assert.False(t, IsAlertIngestToken(nil))
}
