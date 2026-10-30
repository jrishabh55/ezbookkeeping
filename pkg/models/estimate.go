package models

// Estimate statuses stored in Invoice.EstimateStatus. "expired" is never stored: a draft or sent estimate whose
// valid-until date (DueDate) is before today shows as expired, and extending the date revives it. "superseded" is a version
// a newer version (a revision) replaced: read-only history that keeps its lines, milestones, emails and recorded answer.
const (
	ESTIMATE_STATUS_DRAFT      = "draft"
	ESTIMATE_STATUS_SENT       = "sent"
	ESTIMATE_STATUS_ACCEPTED   = "accepted"
	ESTIMATE_STATUS_DECLINED   = "declined"
	ESTIMATE_STATUS_SUPERSEDED = "superseded"
)

// EstimateNumberPrefixDefault is the estimate number prefix of a profile that has not set one
const EstimateNumberPrefixDefault = "EST"

// EstimateValidDaysDefault is how long a new estimate stays valid unless the profile says otherwise
const EstimateValidDaysDefault = 30

// EstimateMaxMilestones is the most payment terms one estimate may have
const EstimateMaxMilestones = 10

// EstimateMilestonePercentTotal is what the milestone percents (basis points) of an estimate must add up to: 100%
const EstimateMilestonePercentTotal = 10000

// EstimateMilestoneLabelMaxLength is the longest milestone label in characters
const EstimateMilestoneLabelMaxLength = 128

// EstimateResponseNoteMaxLength is the longest note a client may leave when declining, in characters
const EstimateResponseNoteMaxLength = 500

// EstimateMilestone is one payment term of an estimate: a share of its subtotal that is invoiced on its own
type EstimateMilestone struct {
	MilestoneId  int64  `xorm:"PK"`
	Uid          int64  `xorm:"INDEX(IDX_estimate_milestone_uid_estimate_id) NOT NULL"`
	EstimateId   int64  `xorm:"INDEX(IDX_estimate_milestone_uid_estimate_id) NOT NULL"`
	DisplayOrder int32  `xorm:"NOT NULL"`
	Label        string `xorm:"VARCHAR(128) NOT NULL"`
	Percent      int32  `xorm:"NOT NULL"`           // basis points
	InvoiceId    int64  `xorm:"NOT NULL DEFAULT 0"` // the non-deleted invoice made from this milestone; 0 until invoiced
}

// EstimateMilestoneRequest is one milestone of an estimate creation / modification request
type EstimateMilestoneRequest struct {
	Label   string `json:"label" binding:"required,notBlank,max=128"`
	Percent int32  `json:"percent" binding:"required,min=1,max=10000"`
}

// EstimateMilestoneResponse is a milestone with the invoice made from it, if any
type EstimateMilestoneResponse struct {
	Id            string `json:"id"`
	Label         string `json:"label"`
	Percent       int32  `json:"percent"`
	InvoiceId     string `json:"invoiceId,omitempty"`
	InvoiceNumber string `json:"invoiceNumber,omitempty"`
}

// EstimateProgress is what has been invoiced from an estimate: its milestones (with their invoice ids) and every
// non-deleted invoice made from it, ordered by number
type EstimateProgress struct {
	Milestones    []*EstimateMilestone
	Invoices      []*Invoice
	FullyInvoiced bool // every milestone is invoiced or, without milestones, an invoice exists
}

// EstimateConvertedInvoiceResponse is an invoice made from an estimate
type EstimateConvertedInvoiceResponse struct {
	Id     string `json:"id"`
	Number string `json:"number"`
}

// EstimateRespondRequest records your own answer for an estimate (Mark accepted / Mark declined)
type EstimateRespondRequest struct {
	Id     int64  `json:"id,string" binding:"required,min=1"`
	Status string `json:"status" binding:"required,oneof=accepted declined"`
}

// EstimateInvoiceDraftRequest asks for the unsaved invoice an estimate (no milestone) or one of its milestones turns into
type EstimateInvoiceDraftRequest struct {
	Id          int64 `form:"id" binding:"required,min=1"`
	MilestoneId int64 `form:"milestone_id" binding:"min=0"`
}

// EstimatePublicRespondRequest is the client's answer through a public estimate link: the token is the only key
type EstimatePublicRespondRequest struct {
	Token string `json:"token" binding:"required,max=64"`
	Note  string `json:"note" binding:"max=4000"` // decline only; plain text. A generous request-size guard: the service trims then
	// enforces the real 500-rune limit (EstimateResponseNoteMaxLength), so an over-long note gets ErrEstimateResponseNoteTooLong
	// rather than the generic submission error, and a 500-rune note padded with whitespace is still accepted.
}

// PublicEstimateMilestoneResponse is a payment term as the client sees it: no ids, no invoices
type PublicEstimateMilestoneResponse struct {
	Label   string `json:"label"`
	Percent int32  `json:"percent"`
}

// ToPublicEstimateMilestoneResponses returns the milestones served through a public estimate link
func ToPublicEstimateMilestoneResponses(milestones []*EstimateMilestone) []*PublicEstimateMilestoneResponse {
	resps := make([]*PublicEstimateMilestoneResponse, len(milestones))

	for idx, milestone := range milestones {
		resps[idx] = &PublicEstimateMilestoneResponse{Label: milestone.Label, Percent: milestone.Percent}
	}

	return resps
}
