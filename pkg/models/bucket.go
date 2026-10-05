package models

// BucketStatus represents bucket goal status
type BucketStatus byte

// Bucket statuses
const (
	BUCKET_STATUS_IN_PROGRESS BucketStatus = 1
	BUCKET_STATUS_COMPLETED   BucketStatus = 2
)

// BucketTransactionType represents bucket transaction type
type BucketTransactionType byte

// Bucket transaction types
const (
	BUCKET_TRANSACTION_TYPE_ALLOCATE BucketTransactionType = 1
	BUCKET_TRANSACTION_TYPE_WITHDRAW BucketTransactionType = 2
)

// Bucket represents bucket (envelope) data stored in database
type Bucket struct {
	BucketId        int64        `xorm:"PK"`
	Uid             int64        `xorm:"INDEX(IDX_bucket_uid_deleted) NOT NULL"`
	Deleted         bool         `xorm:"INDEX(IDX_bucket_uid_deleted) NOT NULL"`
	Name            string       `xorm:"VARCHAR(64) NOT NULL"`
	TargetAmount    int64        `xorm:"NOT NULL DEFAULT 0"`
	CurrentBalance  int64        `xorm:"NOT NULL DEFAULT 0"`
	Currency        string       `xorm:"VARCHAR(3) NOT NULL"`
	Status          BucketStatus `xorm:"NOT NULL DEFAULT 1"`
	Icon            int64        `xorm:"NOT NULL"`
	Color           string       `xorm:"VARCHAR(6) NOT NULL"`
	Comment         string       `xorm:"VARCHAR(255) NOT NULL"`
	Visible         bool         `xorm:"NOT NULL"`
	DisplayOrder    int32        `xorm:"NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
	DeletedUnixTime int64
}

// BucketAccount represents linked accounts for a bucket stored in database
type BucketAccount struct {
	Id              int64 `xorm:"PK"`
	BucketId        int64 `xorm:"INDEX(IDX_bucket_account_bucket_id) NOT NULL"`
	Uid             int64 `xorm:"NOT NULL"`
	AccountId       int64 `xorm:"NOT NULL"`
	CreatedUnixTime int64
}

// BucketTransaction represents bucket allocation/withdrawal transaction stored in database
type BucketTransaction struct {
	BucketTransactionId int64                 `xorm:"PK"`
	BucketId            int64                 `xorm:"INDEX(IDX_bucket_transaction_bucket_id_time) NOT NULL"`
	Uid                 int64                 `xorm:"INDEX(IDX_bucket_transaction_uid) NOT NULL"`
	Type                BucketTransactionType `xorm:"NOT NULL"`
	Amount              int64                 `xorm:"NOT NULL"`
	TransactionId       int64                 `xorm:"NOT NULL DEFAULT 0"`
	TransactionTime     int64                 `xorm:"INDEX(IDX_bucket_transaction_bucket_id_time) NOT NULL"`
	TimezoneUtcOffset   int16                 `xorm:"NOT NULL"`
	Comment             string                `xorm:"VARCHAR(255) NOT NULL"`
	CreatedUnixTime     int64
	UpdatedUnixTime     int64
}

// BucketCreateRequest represents all parameters of bucket creation request
type BucketCreateRequest struct {
	Name            string   `json:"name" binding:"required,notBlank,max=64"`
	TargetAmount    int64    `json:"targetAmount" binding:"min=0"`
	Currency        string   `json:"currency" binding:"required,len=3,validCurrency"`
	Icon            int64    `json:"icon,string" binding:"required,min=1"`
	Color           string   `json:"color" binding:"required,len=6,validHexRGBColor"`
	Comment         string   `json:"comment" binding:"max=255"`
	AccountIds      []string `json:"accountIds"`
	ClientSessionId string   `json:"clientSessionId"`
}

// BucketModifyRequest represents all parameters of bucket modification request
type BucketModifyRequest struct {
	Id           int64        `json:"id,string" binding:"required,min=1"`
	Name         string       `json:"name" binding:"required,notBlank,max=64"`
	TargetAmount int64        `json:"targetAmount" binding:"min=0"`
	Currency     string       `json:"currency" binding:"required,len=3,validCurrency"`
	Icon         int64        `json:"icon,string" binding:"required,min=1"`
	Color        string       `json:"color" binding:"required,len=6,validHexRGBColor"`
	Comment      string       `json:"comment" binding:"max=255"`
	Status       BucketStatus `json:"status" binding:"min=1,max=2"`
	AccountIds   []string     `json:"accountIds"`
	Visible      bool         `json:"visible"`
}

// BucketListRequest represents all parameters of bucket listing request
type BucketListRequest struct {
	VisibleOnly bool `form:"visible_only"`
}

// BucketGetRequest represents all parameters of bucket getting request
type BucketGetRequest struct {
	Id int64 `form:"id,string" binding:"required,min=1"`
}

// BucketHideRequest represents all parameters of bucket hiding request
type BucketHideRequest struct {
	Id     int64 `json:"id,string" binding:"required,min=1"`
	Hidden bool  `json:"hidden"`
}

// BucketMoveRequest represents all parameters of bucket moving request
type BucketMoveRequest struct {
	NewDisplayOrders []*BucketNewDisplayOrderRequest `json:"newDisplayOrders" binding:"required,min=1"`
}

// BucketNewDisplayOrderRequest represents a data pair of id and display order
type BucketNewDisplayOrderRequest struct {
	Id           int64 `json:"id,string" binding:"required,min=1"`
	DisplayOrder int32 `json:"displayOrder"`
}

// BucketDeleteRequest represents all parameters of bucket deleting request
type BucketDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// BucketAllocateRequest represents all parameters of bucket allocation request
type BucketAllocateRequest struct {
	BucketId        int64  `json:"bucketId,string" binding:"required,min=1"`
	Amount          int64  `json:"amount" binding:"required,min=1"`
	TransactionId   int64  `json:"transactionId,string"`
	TransactionTime int64  `json:"transactionTime" binding:"required,min=1"`
	UtcOffset       int16  `json:"utcOffset" binding:"min=-720,max=840"`
	Comment         string `json:"comment" binding:"max=255"`
	ClientSessionId string `json:"clientSessionId"`
}

// BucketWithdrawRequest represents all parameters of bucket withdrawal request
type BucketWithdrawRequest struct {
	BucketId        int64  `json:"bucketId,string" binding:"required,min=1"`
	Amount          int64  `json:"amount" binding:"required,min=1"`
	TransactionId   int64  `json:"transactionId,string"`
	TransactionTime int64  `json:"transactionTime" binding:"required,min=1"`
	UtcOffset       int16  `json:"utcOffset" binding:"min=-720,max=840"`
	Comment         string `json:"comment" binding:"max=255"`
	ClientSessionId string `json:"clientSessionId"`
}

// BucketTransactionListRequest represents all parameters of bucket transaction listing request
type BucketTransactionListRequest struct {
	BucketId int64 `form:"bucketId,string" binding:"required,min=1"`
	MaxTime  int64 `form:"maxTime"`
	MinTime  int64 `form:"minTime"`
	Count    int32 `form:"count" binding:"omitempty,min=1,max=50"`
}

// BucketTransactionDeleteRequest represents all parameters of bucket transaction delete request
type BucketTransactionDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// BucketInfoResponse represents a view-object of bucket
type BucketInfoResponse struct {
	Id             int64        `json:"id,string"`
	Name           string       `json:"name"`
	TargetAmount   int64        `json:"targetAmount"`
	CurrentBalance int64        `json:"currentBalance"`
	Currency       string       `json:"currency"`
	Status         BucketStatus `json:"status"`
	Icon           int64        `json:"icon,string"`
	Color          string       `json:"color"`
	Comment        string       `json:"comment"`
	DisplayOrder   int32        `json:"displayOrder"`
	Hidden         bool         `json:"hidden"`
	AccountIds     []string     `json:"accountIds,omitempty"`
	Progress       float64      `json:"progress"`
}

// BucketTransactionInfoResponse represents a view-object of bucket transaction
type BucketTransactionInfoResponse struct {
	Id                int64                 `json:"id,string"`
	BucketId          int64                 `json:"bucketId,string"`
	Type              BucketTransactionType `json:"type"`
	Amount            int64                 `json:"amount"`
	TransactionId     int64                 `json:"transactionId,string,omitempty"`
	TransactionTime   int64                 `json:"transactionTime"`
	TimezoneUtcOffset int16                 `json:"utcOffset"`
	Comment           string                `json:"comment"`
}

// ToBucketInfoResponse returns a view-object according to database model
func (b *Bucket) ToBucketInfoResponse() *BucketInfoResponse {
	progress := float64(0)
	if b.TargetAmount > 0 {
		progress = float64(b.CurrentBalance) / float64(b.TargetAmount) * 100
	}

	return &BucketInfoResponse{
		Id:             b.BucketId,
		Name:           b.Name,
		TargetAmount:   b.TargetAmount,
		CurrentBalance: b.CurrentBalance,
		Currency:       b.Currency,
		Status:         b.Status,
		Icon:           b.Icon,
		Color:          b.Color,
		Comment:        b.Comment,
		DisplayOrder:   b.DisplayOrder,
		Hidden:         !b.Visible,
		Progress:       progress,
	}
}

// ToBucketTransactionInfoResponse returns a view-object according to database model
func (bt *BucketTransaction) ToBucketTransactionInfoResponse() *BucketTransactionInfoResponse {
	return &BucketTransactionInfoResponse{
		Id:                bt.BucketTransactionId,
		BucketId:          bt.BucketId,
		Type:              bt.Type,
		Amount:            bt.Amount,
		TransactionId:     bt.TransactionId,
		TransactionTime:   bt.TransactionTime,
		TimezoneUtcOffset: bt.TimezoneUtcOffset,
		Comment:           bt.Comment,
	}
}

// BucketInfoResponseSlice represents the slice data structure of BucketInfoResponse
type BucketInfoResponseSlice []*BucketInfoResponse

// Len returns the count of items
func (b BucketInfoResponseSlice) Len() int {
	return len(b)
}

// Swap swaps two items
func (b BucketInfoResponseSlice) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// Less reports whether the first item is less than the second one
func (b BucketInfoResponseSlice) Less(i, j int) bool {
	return b[i].DisplayOrder < b[j].DisplayOrder
}

// BucketTransactionInfoResponseSlice represents the slice data structure of BucketTransactionInfoResponse
type BucketTransactionInfoResponseSlice []*BucketTransactionInfoResponse

// Len returns the count of items
func (b BucketTransactionInfoResponseSlice) Len() int {
	return len(b)
}

// Swap swaps two items
func (b BucketTransactionInfoResponseSlice) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// Less reports whether the first item is less than the second one (by transaction time descending)
func (b BucketTransactionInfoResponseSlice) Less(i, j int) bool {
	return b[i].TransactionTime > b[j].TransactionTime
}
