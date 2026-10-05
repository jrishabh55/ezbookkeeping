package models

import "time"

// BudgetPeriodType represents budget period type
type BudgetPeriodType byte

// Budget period types
const (
	BUDGET_PERIOD_TYPE_CUSTOM  BudgetPeriodType = 1
	BUDGET_PERIOD_TYPE_WEEKLY  BudgetPeriodType = 2
	BUDGET_PERIOD_TYPE_MONTHLY BudgetPeriodType = 3
	BUDGET_PERIOD_TYPE_YEARLY  BudgetPeriodType = 4
)

// BudgetRolloverType represents budget rollover behavior
type BudgetRolloverType byte

// Budget rollover types
const (
	BUDGET_ROLLOVER_TYPE_RESET    BudgetRolloverType = 1
	BUDGET_ROLLOVER_TYPE_ROLLOVER BudgetRolloverType = 2
)

// BudgetScopeType represents budget scope type
type BudgetScopeType byte

// Budget scope types
const (
	BUDGET_SCOPE_TYPE_CATEGORY BudgetScopeType = 1
	BUDGET_SCOPE_TYPE_ACCOUNT  BudgetScopeType = 2
	BUDGET_SCOPE_TYPE_TAG      BudgetScopeType = 3
)

// Budget represents budget data stored in database
type Budget struct {
	BudgetId            int64              `xorm:"PK"`
	Uid                 int64              `xorm:"INDEX(IDX_budget_uid_deleted) NOT NULL"`
	Deleted             bool               `xorm:"INDEX(IDX_budget_uid_deleted) NOT NULL"`
	Name                string             `xorm:"VARCHAR(64) NOT NULL"`
	Amount              int64              `xorm:"NOT NULL"`
	Currency            string             `xorm:"VARCHAR(3) NOT NULL"`
	PeriodType          BudgetPeriodType   `xorm:"NOT NULL"`
	StartTime           int64              `xorm:"NOT NULL"`
	EndTime             int64              `xorm:"NOT NULL"`
	RolloverType        BudgetRolloverType `xorm:"NOT NULL"`
	AutoCreateNextCycle bool               `xorm:"NOT NULL"`
	WarningThreshold    int                `xorm:"NOT NULL DEFAULT 75"`
	AlertThreshold      int                `xorm:"NOT NULL DEFAULT 90"`
	OverspentThreshold  int                `xorm:"NOT NULL DEFAULT 100"`
	Visible             bool               `xorm:"NOT NULL"`
	DisplayOrder        int32              `xorm:"NOT NULL"`
	ParentBudgetId      int64              `xorm:"NOT NULL DEFAULT 0"`
	CreatedUnixTime     int64
	UpdatedUnixTime     int64
	DeletedUnixTime     int64
}

// BudgetScope represents budget scope rule stored in database
type BudgetScope struct {
	ScopeId         int64           `xorm:"PK"`
	BudgetId        int64           `xorm:"INDEX(IDX_budget_scope_budget_id) NOT NULL"`
	Uid             int64           `xorm:"NOT NULL"`
	ScopeType       BudgetScopeType `xorm:"NOT NULL"`
	ScopeItemId     int64           `xorm:"NOT NULL"`
	CreatedUnixTime int64
}

// BudgetCycle represents a budget cycle period stored in database
type BudgetCycle struct {
	CycleId           int64 `xorm:"PK"`
	BudgetId          int64 `xorm:"INDEX(IDX_budget_cycle_budget_id_start_time) NOT NULL"`
	Uid               int64 `xorm:"NOT NULL"`
	StartTime         int64 `xorm:"INDEX(IDX_budget_cycle_budget_id_start_time) NOT NULL"`
	EndTime           int64 `xorm:"NOT NULL"`
	CarryoverAmount   int64 `xorm:"NOT NULL DEFAULT 0"`
	NotifiedWarning   bool  `xorm:"NOT NULL DEFAULT false"`
	NotifiedAlert     bool  `xorm:"NOT NULL DEFAULT false"`
	NotifiedOverspent bool  `xorm:"NOT NULL DEFAULT false"`
	CreatedUnixTime   int64
	UpdatedUnixTime   int64
}

// BudgetCreateRequest represents all parameters of budget creation request
type BudgetCreateRequest struct {
	Name                string             `json:"name" binding:"required,notBlank,max=64"`
	Amount              int64              `json:"amount" binding:"required,min=1"`
	Currency            string             `json:"currency" binding:"required,len=3,validCurrency"`
	PeriodType          BudgetPeriodType   `json:"periodType" binding:"required,min=1,max=4"`
	StartTime           int64              `json:"startTime" binding:"required,min=1"`
	EndTime             int64              `json:"endTime" binding:"required,min=1"`
	RolloverType        BudgetRolloverType `json:"rolloverType" binding:"required,min=1,max=2"`
	AutoCreateNextCycle bool               `json:"autoCreateNextCycle"`
	WarningThreshold    int                `json:"warningThreshold" binding:"min=0,max=100"`
	AlertThreshold      int                `json:"alertThreshold" binding:"min=0,max=100"`
	OverspentThreshold  int                `json:"overspentThreshold" binding:"min=0,max=200"`
	CategoryIds         []string           `json:"categoryIds"`
	AccountIds          []string           `json:"accountIds"`
	TagIds              []string           `json:"tagIds"`
	ClientSessionId     string             `json:"clientSessionId"`
}

// BudgetModifyRequest represents all parameters of budget modification request
type BudgetModifyRequest struct {
	Id                  int64              `json:"id,string" binding:"required,min=1"`
	Name                string             `json:"name" binding:"required,notBlank,max=64"`
	Amount              int64              `json:"amount" binding:"required,min=1"`
	Currency            string             `json:"currency" binding:"required,len=3,validCurrency"`
	PeriodType          BudgetPeriodType   `json:"periodType" binding:"required,min=1,max=4"`
	StartTime           int64              `json:"startTime" binding:"required,min=1"`
	EndTime             int64              `json:"endTime" binding:"required,min=1"`
	RolloverType        BudgetRolloverType `json:"rolloverType" binding:"required,min=1,max=2"`
	AutoCreateNextCycle bool               `json:"autoCreateNextCycle"`
	WarningThreshold    int                `json:"warningThreshold" binding:"min=0,max=100"`
	AlertThreshold      int                `json:"alertThreshold" binding:"min=0,max=100"`
	OverspentThreshold  int                `json:"overspentThreshold" binding:"min=0,max=200"`
	CategoryIds         []string           `json:"categoryIds"`
	AccountIds          []string           `json:"accountIds"`
	TagIds              []string           `json:"tagIds"`
	Visible             bool               `json:"visible"`
}

// BudgetListRequest represents all parameters of budget listing request
type BudgetListRequest struct {
	VisibleOnly bool `form:"visible_only"`
}

// BudgetGetRequest represents all parameters of budget getting request
type BudgetGetRequest struct {
	Id int64 `form:"id,string" binding:"required,min=1"`
}

// BudgetHideRequest represents all parameters of budget hiding request
type BudgetHideRequest struct {
	Id     int64 `json:"id,string" binding:"required,min=1"`
	Hidden bool  `json:"hidden"`
}

// BudgetMoveRequest represents all parameters of budget moving request
type BudgetMoveRequest struct {
	NewDisplayOrders []*BudgetNewDisplayOrderRequest `json:"newDisplayOrders" binding:"required,min=1"`
}

// BudgetNewDisplayOrderRequest represents a data pair of id and display order
type BudgetNewDisplayOrderRequest struct {
	Id           int64 `json:"id,string" binding:"required,min=1"`
	DisplayOrder int32 `json:"displayOrder"`
}

// BudgetDeleteRequest represents all parameters of budget deleting request
type BudgetDeleteRequest struct {
	Id int64 `json:"id,string" binding:"required,min=1"`
}

// BudgetProgressRequest represents all parameters of budget progress request
type BudgetProgressRequest struct {
	Id        int64 `form:"id,string" binding:"required,min=1"`
	StartTime int64 `form:"startTime"`
	EndTime   int64 `form:"endTime"`
}

// BudgetInfoResponse represents a view-object of budget
type BudgetInfoResponse struct {
	Id                  int64                    `json:"id,string"`
	Name                string                   `json:"name"`
	Amount              int64                    `json:"amount"`
	Currency            string                   `json:"currency"`
	PeriodType          BudgetPeriodType         `json:"periodType"`
	StartTime           int64                    `json:"startTime"`
	EndTime             int64                    `json:"endTime"`
	RolloverType        BudgetRolloverType       `json:"rolloverType"`
	AutoCreateNextCycle bool                     `json:"autoCreateNextCycle"`
	WarningThreshold    int                      `json:"warningThreshold"`
	AlertThreshold      int                      `json:"alertThreshold"`
	OverspentThreshold  int                      `json:"overspentThreshold"`
	DisplayOrder        int32                    `json:"displayOrder"`
	Hidden              bool                     `json:"hidden"`
	CategoryIds         []string                 `json:"categoryIds,omitempty"`
	AccountIds          []string                 `json:"accountIds,omitempty"`
	TagIds              []string                 `json:"tagIds,omitempty"`
	CurrentCycle        *BudgetCycleInfoResponse `json:"currentCycle,omitempty"`
}

// BudgetCycleInfoResponse represents a view-object of budget cycle
type BudgetCycleInfoResponse struct {
	Id              int64 `json:"id,string"`
	BudgetId        int64 `json:"budgetId,string"`
	StartTime       int64 `json:"startTime"`
	EndTime         int64 `json:"endTime"`
	CarryoverAmount int64 `json:"carryoverAmount"`
}

// BudgetProgressResponse represents a view-object of budget progress
type BudgetProgressResponse struct {
	BudgetId           int64   `json:"budgetId,string"`
	BudgetName         string  `json:"budgetName"`
	BudgetAmount       int64   `json:"budgetAmount"`
	Currency           string  `json:"currency"`
	SpentAmount        int64   `json:"spentAmount"`
	RemainingAmount    int64   `json:"remainingAmount"`
	CarryoverAmount    int64   `json:"carryoverAmount"`
	TotalBudgetAmount  int64   `json:"totalBudgetAmount"`
	ProgressPercentage float64 `json:"progressPercentage"`
	StartTime          int64   `json:"startTime"`
	EndTime            int64   `json:"endTime"`
	WarningThreshold   int     `json:"warningThreshold"`
	AlertThreshold     int     `json:"alertThreshold"`
	OverspentThreshold int     `json:"overspentThreshold"`
	IsOverspent        bool    `json:"isOverspent"`
	IsWarning          bool    `json:"isWarning"`
	IsAlert            bool    `json:"isAlert"`
}

// ToAccountInfoResponse returns a view-object according to database model
func (b *Budget) ToBudgetInfoResponse() *BudgetInfoResponse {
	return &BudgetInfoResponse{
		Id:                  b.BudgetId,
		Name:                b.Name,
		Amount:              b.Amount,
		Currency:            b.Currency,
		PeriodType:          b.PeriodType,
		StartTime:           b.StartTime,
		EndTime:             b.EndTime,
		RolloverType:        b.RolloverType,
		AutoCreateNextCycle: b.AutoCreateNextCycle,
		WarningThreshold:    b.WarningThreshold,
		AlertThreshold:      b.AlertThreshold,
		OverspentThreshold:  b.OverspentThreshold,
		DisplayOrder:        b.DisplayOrder,
		Hidden:              !b.Visible,
	}
}

// ToBudgetCycleInfoResponse returns a view-object according to database model
func (c *BudgetCycle) ToBudgetCycleInfoResponse() *BudgetCycleInfoResponse {
	return &BudgetCycleInfoResponse{
		Id:              c.CycleId,
		BudgetId:        c.BudgetId,
		StartTime:       c.StartTime,
		EndTime:         c.EndTime,
		CarryoverAmount: c.CarryoverAmount,
	}
}

// BudgetInfoResponseSlice represents the slice data structure of BudgetInfoResponse
type BudgetInfoResponseSlice []*BudgetInfoResponse

// Len returns the count of items
func (b BudgetInfoResponseSlice) Len() int {
	return len(b)
}

// Swap swaps two items
func (b BudgetInfoResponseSlice) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

// Less reports whether the first item is less than the second one
func (b BudgetInfoResponseSlice) Less(i, j int) bool {
	return b[i].DisplayOrder < b[j].DisplayOrder
}

// GetBudgetPeriodEndTime returns the last second of the period starting at startTime, custom periods keep the given duration
func GetBudgetPeriodEndTime(periodType BudgetPeriodType, startTime int64, customDuration int64) int64 {
	start := time.Unix(startTime, 0)

	switch periodType {
	case BUDGET_PERIOD_TYPE_WEEKLY:
		return start.AddDate(0, 0, 7).Unix() - 1
	case BUDGET_PERIOD_TYPE_MONTHLY:
		return start.AddDate(0, 1, 0).Unix() - 1
	case BUDGET_PERIOD_TYPE_YEARLY:
		return start.AddDate(1, 0, 0).Unix() - 1
	default:
		return startTime + customDuration
	}
}

// SplitBudgetScopes returns the category, account and tag ids of the budget scopes
func SplitBudgetScopes(scopes []*BudgetScope) (categoryIds []int64, accountIds []int64, tagIds []int64) {
	for _, scope := range scopes {
		switch scope.ScopeType {
		case BUDGET_SCOPE_TYPE_CATEGORY:
			categoryIds = append(categoryIds, scope.ScopeItemId)
		case BUDGET_SCOPE_TYPE_ACCOUNT:
			accountIds = append(accountIds, scope.ScopeItemId)
		case BUDGET_SCOPE_TYPE_TAG:
			tagIds = append(tagIds, scope.ScopeItemId)
		}
	}

	return categoryIds, accountIds, tagIds
}
