package api

import (
	"fmt"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/duplicatechecker"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

// alertIngestMaxTextLength is the maximum accepted length of an ingested SMS body; it mirrors
// the "max=2048" binding tag on models.AlertIngestRequest.Text so that a request is rejected the
// same way regardless of whether the mismatch comes from the validator or from this extra guard
const alertIngestMaxTextLength = 2048

// alertIngestMaxRequestsPerMinute is the ceiling used by the per-uid fixed-window rate limit on
// the ingest endpoint
const alertIngestMaxRequestsPerMinute = 30

// alertIngestRateLimitKeyPrefix namespaces the rate limit counter's key within the shared
// duplicate checker's failure-count cache, which is also used (with different key prefixes) for
// login brute-force protection; it is per-process and resets on restart, and under a
// multi-instance deployment each instance enforces its own ceiling rather than a shared one
// (ponytail: the smallest correct thing, not a distributed rate limiter)
const alertIngestRateLimitKeyPrefix = "alert-ingest:"

// AlertsApi represents alerts api
type AlertsApi struct {
	ApiUsingConfig
	alerts *services.AlertService
	tokens *services.TokenService
	users  *services.UserService
}

// Initialize an alerts api singleton instance
var (
	Alerts = &AlertsApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		alerts: services.Alerts,
		tokens: services.Tokens,
		users:  services.Users,
	}
)

// IngestHandler ingests one bank / card alert SMS forwarded by the iPhone Shortcut and returns
// only the outcome and a human summary; the uid always comes from the ingest token's claims, and
// the response never carries balances or other transactions
func (a *AlertsApi) IngestHandler(c *core.WebContext) (any, *errs.Error) {
	var alertIngestReq models.AlertIngestRequest
	err := c.ShouldBindJSON(&alertIngestReq)

	if err != nil {
		log.Warnf(c, "[alerts.IngestHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if len(alertIngestReq.Text) > alertIngestMaxTextLength {
		log.Warnf(c, "[alerts.IngestHandler] text length %d exceeds the maximum allowed length %d", len(alertIngestReq.Text), alertIngestMaxTextLength)
		return nil, errs.ErrFormatInvalid
	}

	uid := c.GetCurrentUid()
	rateLimitKey := fmt.Sprintf("%s%d", alertIngestRateLimitKeyPrefix, uid)
	requestCount := duplicatechecker.Container.IncreaseFailureCount(rateLimitKey)

	if requestCount > alertIngestMaxRequestsPerMinute {
		log.Warnf(c, "[alerts.IngestHandler] user \"uid:%d\" exceeded the alert ingest rate limit (%d requests within the current window)", uid, requestCount)
		return nil, errs.ErrTooManyRequests
	}

	receivedAt := time.Now()

	if alertIngestReq.ReceivedAt > 0 {
		receivedAt = time.Unix(alertIngestReq.ReceivedAt, 0)
	}

	result, err := a.alerts.Ingest(c, uid, alertIngestReq.Sender, alertIngestReq.Text, receivedAt)

	if err != nil {
		log.Errorf(c, "[alerts.IngestHandler] failed to ingest alert for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	alertIngestResp := &models.AlertIngestResponse{
		Result:  result.Outcome,
		Summary: result.Summary,
	}

	return alertIngestResp, nil
}

// TokenCreateHandler creates (replacing any existing one) this user's SMS alert ingest token; it
// requires a normal session token and this user's password, mirroring TokenGenerateMCPHandler
func (a *AlertsApi) TokenCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var alertTokenCreateReq models.AlertTokenCreateRequest
	err := c.ShouldBindJSON(&alertTokenCreateReq)

	if err != nil {
		log.Warnf(c, "[alerts.TokenCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	claims := c.GetTokenClaims()

	if claims == nil {
		log.Warnf(c, "[alerts.TokenCreateHandler] current token is null")
		return nil, errs.ErrInvalidToken
	} else if claims.Type != core.USER_TOKEN_TYPE_NORMAL {
		log.Warnf(c, "[alerts.TokenCreateHandler] token type \"%d\" is not allowed to create alert ingest tokens", claims.Type)
		return nil, errs.ErrInvalidToken
	}

	user, err := a.users.GetUserById(c, uid)

	if err != nil {
		log.Warnf(c, "[alerts.TokenCreateHandler] failed to get user \"uid:%d\" info, because %s", uid, err.Error())
		return nil, errs.ErrUserNotFound
	}

	if !a.users.IsPasswordEqualsUserPassword(alertTokenCreateReq.Password, user) {
		return nil, errs.ErrUserPasswordWrong
	}

	token, _, err := a.tokens.CreateAlertIngestToken(c, user, 0)

	if err != nil {
		log.Errorf(c, "[alerts.TokenCreateHandler] failed to create alert ingest token for user \"uid:%d\", because %s", user.Uid, err.Error())
		return nil, errs.Or(err, errs.ErrTokenGenerating)
	}

	log.Infof(c, "[alerts.TokenCreateHandler] user \"uid:%d\" has generated a new alert ingest token", user.Uid)

	alertTokenCreateResp := &models.AlertTokenCreateResponse{
		Token:       token,
		ShortcutUrl: a.CurrentConfig().AlertsShortcutUrl,
	}

	return alertTokenCreateResp, nil
}

// TokenRevokeHandler deletes this user's SMS alert ingest token record(s); it requires a normal
// session token
func (a *AlertsApi) TokenRevokeHandler(c *core.WebContext) (any, *errs.Error) {
	claims := c.GetTokenClaims()

	if claims == nil {
		log.Warnf(c, "[alerts.TokenRevokeHandler] current token is null")
		return nil, errs.ErrInvalidToken
	} else if claims.Type != core.USER_TOKEN_TYPE_NORMAL {
		log.Warnf(c, "[alerts.TokenRevokeHandler] token type \"%d\" is not allowed to revoke the alert ingest token", claims.Type)
		return nil, errs.ErrInvalidToken
	}

	uid := c.GetCurrentUid()
	err := a.tokens.DeleteTokensByType(c, uid, core.USER_TOKEN_TYPE_ALERT_INGEST)

	if err != nil {
		log.Errorf(c, "[alerts.TokenRevokeHandler] failed to revoke alert ingest token for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[alerts.TokenRevokeHandler] user \"uid:%d\" has revoked the alert ingest token", uid)
	return true, nil
}

// StatusHandler returns this user's alert ingestion status, counters and shortcut url; it
// requires a normal session token
func (a *AlertsApi) StatusHandler(c *core.WebContext) (any, *errs.Error) {
	claims := c.GetTokenClaims()

	if claims == nil {
		log.Warnf(c, "[alerts.StatusHandler] current token is null")
		return nil, errs.ErrInvalidToken
	} else if claims.Type != core.USER_TOKEN_TYPE_NORMAL {
		log.Warnf(c, "[alerts.StatusHandler] token type \"%d\" is not allowed to query alert status", claims.Type)
		return nil, errs.ErrInvalidToken
	}

	uid := c.GetCurrentUid()
	status, err := a.alerts.Status(c, uid)

	if err != nil {
		log.Errorf(c, "[alerts.StatusHandler] failed to get alert status for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	configured, err := a.tokens.ExistsValidTokenByType(c, uid, core.USER_TOKEN_TYPE_ALERT_INGEST)

	if err != nil {
		log.Errorf(c, "[alerts.StatusHandler] failed to check alert ingest token existence for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	status.Configured = configured
	status.ShortcutUrl = a.CurrentConfig().AlertsShortcutUrl

	return status, nil
}
