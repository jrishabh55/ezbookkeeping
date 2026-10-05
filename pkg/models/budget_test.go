package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetBudgetPeriodEndTime(t *testing.T) {
	start := time.Date(2026, 1, 31, 0, 0, 0, 0, time.Local).Unix()

	assert.Equal(t, time.Date(2026, 2, 7, 0, 0, 0, 0, time.Local).Unix()-1, GetBudgetPeriodEndTime(BUDGET_PERIOD_TYPE_WEEKLY, start, 0))
	assert.Equal(t, time.Date(2026, 3, 3, 0, 0, 0, 0, time.Local).Unix()-1, GetBudgetPeriodEndTime(BUDGET_PERIOD_TYPE_MONTHLY, start, 0)) // Go normalizes Feb 31 to Mar 3
	assert.Equal(t, time.Date(2027, 1, 31, 0, 0, 0, 0, time.Local).Unix()-1, GetBudgetPeriodEndTime(BUDGET_PERIOD_TYPE_YEARLY, start, 0))
	assert.Equal(t, start+100, GetBudgetPeriodEndTime(BUDGET_PERIOD_TYPE_CUSTOM, start, 100))
}

func TestSplitBudgetScopes(t *testing.T) {
	categoryIds, accountIds, tagIds := SplitBudgetScopes([]*BudgetScope{
		{ScopeType: BUDGET_SCOPE_TYPE_CATEGORY, ScopeItemId: 1},
		{ScopeType: BUDGET_SCOPE_TYPE_ACCOUNT, ScopeItemId: 2},
		{ScopeType: BUDGET_SCOPE_TYPE_TAG, ScopeItemId: 3},
		{ScopeType: BUDGET_SCOPE_TYPE_TAG, ScopeItemId: 4},
	})

	assert.Equal(t, []int64{1}, categoryIds)
	assert.Equal(t, []int64{2}, accountIds)
	assert.Equal(t, []int64{3, 4}, tagIds)
}
