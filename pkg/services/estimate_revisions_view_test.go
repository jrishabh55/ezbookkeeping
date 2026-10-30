package services

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

func TestSupersededPublicViewPointsAtTheLatestSharedVersion(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	token := shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	revision, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)

	// the old link still opens the version the client saw
	old, _, _, _, err := Invoices.GetInvoiceByPublicToken(ctx, token)
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/01", old.Number)
	assert.Equal(t, models.ESTIMATE_STATUS_SUPERSEDED, old.EstimateStatus)

	// an unsent draft is never pointed at, not even when it somehow has a token
	supersededBy, err := Invoices.GetEstimateSupersededBy(ctx, old)
	assert.Nil(t, err)
	assert.Equal(t, "", supersededBy)

	draftToken, err := generateInvoicePublicToken()
	assert.Nil(t, err)
	_, err = datastore.Container.UserDataStore.Choose(uid).NewSession(ctx).Cols("public_token").
		Where("uid=? AND invoice_id=?", uid, revision.InvoiceId).Update(&models.Invoice{PublicToken: draftToken})
	assert.Nil(t, err)
	supersededBy, err = Invoices.GetEstimateSupersededBy(ctx, old)
	assert.Nil(t, err)
	assert.Equal(t, "", supersededBy)

	// once the revision is sent with a link, the old page links to it
	revisionToken := shareTestEstimate(t, ctx, uid, revision.InvoiceId)
	supersededBy, err = Invoices.GetEstimateSupersededBy(ctx, old)
	assert.Nil(t, err)
	assert.Equal(t, revisionToken, supersededBy)

	// the latest version itself is superseded by nothing
	latest, _, _, _, err := Invoices.GetInvoiceByPublicToken(ctx, revisionToken)
	assert.Nil(t, err)
	supersededBy, err = Invoices.GetEstimateSupersededBy(ctx, latest)
	assert.Nil(t, err)
	assert.Equal(t, "", supersededBy)

	// a disabled link on the latest version leaves only the notice
	_, _, err = Invoices.SetInvoicePublicToken(ctx, uid, revision.InvoiceId, false)
	assert.Nil(t, err)
	supersededBy, err = Invoices.GetEstimateSupersededBy(ctx, old)
	assert.Nil(t, err)
	assert.Equal(t, "", supersededBy)
}

func TestEstimateListShowsLatestVersionsUnlessSupersededAreRequested(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	revision, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)
	other := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)

	latest, err := Invoices.GetLatestEstimates(ctx, uid)
	assert.Nil(t, err)
	latestIds := make([]int64, len(latest))

	for idx, row := range latest {
		latestIds[idx] = row.InvoiceId
	}

	assert.ElementsMatch(t, []int64{revision.InvoiceId, other.InvoiceId}, latestIds)

	all, err := Invoices.GetAllEstimates(ctx, uid)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(all))
}

func TestEstimateRevisionsListTheWholeChainNewestFirst(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	first, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)
	shareTestEstimate(t, ctx, uid, first.InvoiceId)
	second, err := Invoices.ReviseEstimate(ctx, uid, first.InvoiceId, "2026-10-31")
	assert.Nil(t, err)
	assert.Equal(t, "EST/2026-27/01-R2", second.Number)

	for _, version := range []*models.Invoice{estimate, first, second} {
		revisions, err := Invoices.GetEstimateRevisions(ctx, uid, version)
		assert.Nil(t, err)
		assert.Equal(t, 3, len(revisions))
		assert.Equal(t, "EST/2026-27/01-R2", revisions[0].Number)
		assert.Equal(t, "EST/2026-27/01-R1", revisions[1].Number)
		assert.Equal(t, "EST/2026-27/01", revisions[2].Number)
	}

	revisions, err := Invoices.GetEstimateRevisions(ctx, uid, estimate)
	assert.Nil(t, err)
	progress, err := Invoices.GetEstimateProgress(ctx, uid, []int64{revisions[0].InvoiceId, revisions[1].InvoiceId, revisions[2].InvoiceId})
	assert.Nil(t, err)

	resp := estimate.ToInvoiceInfoResponse(nil, nil)
	resp.SetEstimateRevisions(revisions, progress)
	assert.Equal(t, utils.Int64ToString(second.InvoiceId), resp.Revisions[0].Id)
	assert.Equal(t, int32(2), resp.Revisions[0].RevisionNumber)
	assert.Equal(t, models.ESTIMATE_STATUS_DRAFT, resp.Revisions[0].EstimateStatus)
	assert.Equal(t, "2026-10-31", resp.Revisions[0].InvoiceDate)
	assert.Equal(t, models.ESTIMATE_STATUS_SUPERSEDED, resp.Revisions[2].EstimateStatus)
	assert.Equal(t, estimate.Total, resp.Revisions[2].Total)
	assert.False(t, resp.Revisions[2].FullyInvoiced)

	// a first version that was never revised lists only itself
	lone := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/02", testEstimateLines(), nil)
	revisions, err = Invoices.GetEstimateRevisions(ctx, uid, lone)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(revisions))
}

func TestCustomerTimelineShowsRevisedEvents(t *testing.T) {
	ctx, uid := newAlertTestUser(t)
	customer := createTestCustomer(t, uid)
	estimate := createTestEstimate(t, uid, customer.CustomerId, "EST/2026-27/01", testEstimateLines(), nil)
	shareTestEstimate(t, ctx, uid, estimate.InvoiceId)
	revision, err := Invoices.ReviseEstimate(ctx, uid, estimate.InvoiceId, "2026-10-30")
	assert.Nil(t, err)

	overview, err := Invoices.GetCustomerOverview(ctx, uid, customer.CustomerId)
	assert.Nil(t, err)

	kinds := make(map[string]int)
	var revised *models.CustomerTimelineItemResponse

	for _, item := range overview.Timeline {
		kinds[item.Kind]++

		if item.Kind == models.CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED {
			revised = item
		}
	}

	assert.Equal(t, 1, kinds[models.CUSTOMER_TIMELINE_KIND_ESTIMATE]) // the first version only: a revision is a revised event
	assert.Equal(t, 1, kinds[models.CUSTOMER_TIMELINE_KIND_ESTIMATE_REVISED])
	assert.NotNil(t, revised)
	assert.Equal(t, "EST/2026-27/01-R1", revised.InvoiceNumber)
	assert.Equal(t, "EST/2026-27/01", revised.EstimateNumber)
	assert.Equal(t, utils.Int64ToString(revision.InvoiceId), revised.InvoiceId)
	assert.Equal(t, revision.CreatedUnixTime, revised.UnixTime)
}
