package errs

import "net/http"

// Error codes related to budgets
var (
	ErrBudgetIdInvalid             = NewNormalError(NormalSubcategoryBudget, 0, http.StatusBadRequest, "budget id is invalid")
	ErrBudgetNotFound              = NewNormalError(NormalSubcategoryBudget, 1, http.StatusBadRequest, "budget not found")
	ErrBudgetPeriodTypeInvalid     = NewNormalError(NormalSubcategoryBudget, 2, http.StatusBadRequest, "budget period type is invalid")
	ErrBudgetRolloverTypeInvalid   = NewNormalError(NormalSubcategoryBudget, 3, http.StatusBadRequest, "budget rollover type is invalid")
	ErrBudgetStartTimeAfterEndTime = NewNormalError(NormalSubcategoryBudget, 4, http.StatusBadRequest, "budget start time must be before end time")
	ErrBudgetAmountInvalid         = NewNormalError(NormalSubcategoryBudget, 5, http.StatusBadRequest, "budget amount is invalid")
	ErrBudgetCurrencyInvalid       = NewNormalError(NormalSubcategoryBudget, 6, http.StatusBadRequest, "budget currency is invalid")
	ErrBudgetScopeInvalid          = NewNormalError(NormalSubcategoryBudget, 7, http.StatusBadRequest, "budget scope is invalid")
	ErrBudgetCycleNotFound         = NewNormalError(NormalSubcategoryBudget, 8, http.StatusBadRequest, "budget cycle not found")
	ErrBudgetThresholdInvalid      = NewNormalError(NormalSubcategoryBudget, 9, http.StatusBadRequest, "budget threshold is invalid")
	ErrBudgetCategoryNotFound      = NewNormalError(NormalSubcategoryBudget, 10, http.StatusBadRequest, "budget category not found")
	ErrBudgetAccountNotFound       = NewNormalError(NormalSubcategoryBudget, 11, http.StatusBadRequest, "budget account not found")
	ErrBudgetTagNotFound           = NewNormalError(NormalSubcategoryBudget, 12, http.StatusBadRequest, "budget tag not found")
)
