package api

import (
	"sort"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/duplicatechecker"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// BudgetsApi represents budget api
type BudgetsApi struct {
	ApiUsingConfig
	ApiUsingDuplicateChecker
	budgets      *services.BudgetService
	accounts     *services.AccountService
	categories   *services.TransactionCategoryService
	tags         *services.TransactionTagService
	transactions *services.TransactionService
}

// Initialize a budget api singleton instance
var (
	Budgets = &BudgetsApi{
		ApiUsingConfig: ApiUsingConfig{
			container: settings.Container,
		},
		ApiUsingDuplicateChecker: ApiUsingDuplicateChecker{
			ApiUsingConfig: ApiUsingConfig{
				container: settings.Container,
			},
			container: duplicatechecker.Container,
		},
		budgets:      services.Budgets,
		accounts:     services.Accounts,
		categories:   services.TransactionCategories,
		tags:         services.TransactionTags,
		transactions: services.Transactions,
	}
)

// BudgetListHandler returns budgets list of current user
func (a *BudgetsApi) BudgetListHandler(c *core.WebContext) (any, *errs.Error) {
	var budgetListReq models.BudgetListRequest
	err := c.ShouldBindQuery(&budgetListReq)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetListHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	budgets, err := a.budgets.GetAllBudgetsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[budgets.BudgetListHandler] failed to get all budgets for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	budgetResps := make(models.BudgetInfoResponseSlice, 0, len(budgets))

	for i := 0; i < len(budgets); i++ {
		budget := budgets[i]

		if budgetListReq.VisibleOnly && !budget.Visible {
			continue
		}

		budgetResp := budget.ToBudgetInfoResponse()

		// Get scopes
		scopes, err := a.budgets.GetBudgetScopesByBudgetId(c, uid, budget.BudgetId)
		if err == nil {
			fillBudgetScopes(budgetResp, scopes)
		}

		// Get current cycle
		cycle, err := a.budgets.GetCurrentBudgetCycle(c, uid, budget.BudgetId)
		if err == nil && cycle != nil {
			budgetResp.CurrentCycle = cycle.ToBudgetCycleInfoResponse()
		}

		budgetResps = append(budgetResps, budgetResp)
	}

	sort.Sort(budgetResps)

	return budgetResps, nil
}

// BudgetGetHandler returns one specific budget of current user
func (a *BudgetsApi) BudgetGetHandler(c *core.WebContext) (any, *errs.Error) {
	var budgetGetReq models.BudgetGetRequest
	err := c.ShouldBindQuery(&budgetGetReq)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetGetHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	budget, err := a.budgets.GetBudgetByBudgetId(c, uid, budgetGetReq.Id)

	if err != nil {
		log.Errorf(c, "[budgets.BudgetGetHandler] failed to get budget \"id:%d\" for user \"uid:%d\", because %s", budgetGetReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	budgetResp := budget.ToBudgetInfoResponse()

	// Get scopes
	scopes, err := a.budgets.GetBudgetScopesByBudgetId(c, uid, budget.BudgetId)
	if err == nil {
		fillBudgetScopes(budgetResp, scopes)
	}

	// Get current cycle
	cycle, err := a.budgets.GetCurrentBudgetCycle(c, uid, budget.BudgetId)
	if err == nil && cycle != nil {
		budgetResp.CurrentCycle = cycle.ToBudgetCycleInfoResponse()
	}

	return budgetResp, nil
}

// BudgetCreateHandler saves a new budget by request parameters for current user
func (a *BudgetsApi) BudgetCreateHandler(c *core.WebContext) (any, *errs.Error) {
	var budgetCreateReq models.BudgetCreateRequest
	err := c.ShouldBindJSON(&budgetCreateReq)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetCreateHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if budgetCreateReq.PeriodType != models.BUDGET_PERIOD_TYPE_CUSTOM {
		budgetCreateReq.EndTime = models.GetBudgetPeriodEndTime(budgetCreateReq.PeriodType, budgetCreateReq.StartTime, 0)
	}

	scopes, err := buildBudgetScopes(budgetCreateReq.CategoryIds, budgetCreateReq.AccountIds, budgetCreateReq.TagIds)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetCreateHandler] parse scope ids failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if budgetCreateReq.StartTime >= budgetCreateReq.EndTime {
		log.Warnf(c, "[budgets.BudgetCreateHandler] start time must be before end time")
		return nil, errs.ErrBudgetStartTimeAfterEndTime
	}

	uid := c.GetCurrentUid()

	// Check for duplicate submission
	if budgetCreateReq.ClientSessionId != "" {
		found, _ := a.GetSubmissionRemark(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_BUDGET, uid, budgetCreateReq.ClientSessionId)
		if found {
			log.Infof(c, "[budgets.BudgetCreateHandler] budget \"name:%s\" for user \"uid:%d\" has been created, skip duplicate request", budgetCreateReq.Name, uid)
			return nil, errs.ErrDuplicatedSubmission
		}
	}

	// Set default thresholds if not provided
	warningThreshold := budgetCreateReq.WarningThreshold
	if warningThreshold <= 0 {
		warningThreshold = 75
	}
	alertThreshold := budgetCreateReq.AlertThreshold
	if alertThreshold <= 0 {
		alertThreshold = 90
	}
	overspentThreshold := budgetCreateReq.OverspentThreshold
	if overspentThreshold <= 0 {
		overspentThreshold = 100
	}

	maxDisplayOrder, err := a.budgets.GetTotalBudgetCountByUid(c, uid)
	if err != nil {
		log.Errorf(c, "[budgets.BudgetCreateHandler] failed to get total budget count for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	budget := &models.Budget{
		Uid:                 uid,
		Name:                budgetCreateReq.Name,
		Amount:              budgetCreateReq.Amount,
		Currency:            budgetCreateReq.Currency,
		PeriodType:          budgetCreateReq.PeriodType,
		StartTime:           budgetCreateReq.StartTime,
		EndTime:             budgetCreateReq.EndTime,
		RolloverType:        budgetCreateReq.RolloverType,
		AutoCreateNextCycle: budgetCreateReq.AutoCreateNextCycle,
		WarningThreshold:    warningThreshold,
		AlertThreshold:      alertThreshold,
		OverspentThreshold:  overspentThreshold,
		Visible:             true,
		DisplayOrder:        int32(maxDisplayOrder) + 1,
	}

	err = a.budgets.CreateBudget(c, budget, scopes)

	if err != nil {
		log.Errorf(c, "[budgets.BudgetCreateHandler] failed to create budget \"name:%s\" for user \"uid:%d\", because %s", budgetCreateReq.Name, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budgets.BudgetCreateHandler] user \"uid:%d\" has created a new budget \"id:%d\"", uid, budget.BudgetId)

	if budgetCreateReq.ClientSessionId != "" {
		a.SetSubmissionRemarkIfEnable(duplicatechecker.DUPLICATE_CHECKER_TYPE_NEW_BUDGET, uid, budgetCreateReq.ClientSessionId, "")
	}

	budgetResp := budget.ToBudgetInfoResponse()

	return budgetResp, nil
}

// BudgetModifyHandler saves an existing budget by request parameters for current user
func (a *BudgetsApi) BudgetModifyHandler(c *core.WebContext) (any, *errs.Error) {
	var budgetModifyReq models.BudgetModifyRequest
	err := c.ShouldBindJSON(&budgetModifyReq)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetModifyHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if budgetModifyReq.PeriodType != models.BUDGET_PERIOD_TYPE_CUSTOM {
		budgetModifyReq.EndTime = models.GetBudgetPeriodEndTime(budgetModifyReq.PeriodType, budgetModifyReq.StartTime, 0)
	}

	scopes, err := buildBudgetScopes(budgetModifyReq.CategoryIds, budgetModifyReq.AccountIds, budgetModifyReq.TagIds)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetModifyHandler] parse scope ids failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	if budgetModifyReq.StartTime >= budgetModifyReq.EndTime {
		log.Warnf(c, "[budgets.BudgetModifyHandler] start time must be before end time")
		return nil, errs.ErrBudgetStartTimeAfterEndTime
	}

	uid := c.GetCurrentUid()

	budget := &models.Budget{
		BudgetId:            budgetModifyReq.Id,
		Uid:                 uid,
		Name:                budgetModifyReq.Name,
		Amount:              budgetModifyReq.Amount,
		Currency:            budgetModifyReq.Currency,
		PeriodType:          budgetModifyReq.PeriodType,
		StartTime:           budgetModifyReq.StartTime,
		EndTime:             budgetModifyReq.EndTime,
		RolloverType:        budgetModifyReq.RolloverType,
		AutoCreateNextCycle: budgetModifyReq.AutoCreateNextCycle,
		WarningThreshold:    budgetModifyReq.WarningThreshold,
		AlertThreshold:      budgetModifyReq.AlertThreshold,
		OverspentThreshold:  budgetModifyReq.OverspentThreshold,
		Visible:             budgetModifyReq.Visible,
	}

	err = a.budgets.ModifyBudget(c, budget, scopes)

	if err != nil {
		log.Errorf(c, "[budgets.BudgetModifyHandler] failed to update budget \"id:%d\" for user \"uid:%d\", because %s", budgetModifyReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budgets.BudgetModifyHandler] user \"uid:%d\" has updated budget \"id:%d\"", uid, budgetModifyReq.Id)

	budgetResp := budget.ToBudgetInfoResponse()

	return budgetResp, nil
}

// BudgetHideHandler hides a budget by request parameters for current user
func (a *BudgetsApi) BudgetHideHandler(c *core.WebContext) (any, *errs.Error) {
	var budgetHideReq models.BudgetHideRequest
	err := c.ShouldBindJSON(&budgetHideReq)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetHideHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.budgets.HideBudget(c, uid, budgetHideReq.Id, budgetHideReq.Hidden)

	if err != nil {
		log.Errorf(c, "[budgets.BudgetHideHandler] failed to hide budget \"id:%d\" for user \"uid:%d\", because %s", budgetHideReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budgets.BudgetHideHandler] user \"uid:%d\" has hidden budget \"id:%d\"", uid, budgetHideReq.Id)

	return true, nil
}

// BudgetMoveHandler moves display order of budgets by request parameters for current user
func (a *BudgetsApi) BudgetMoveHandler(c *core.WebContext) (any, *errs.Error) {
	var budgetMoveReq models.BudgetMoveRequest
	err := c.ShouldBindJSON(&budgetMoveReq)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetMoveHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	budgets := make([]*models.Budget, len(budgetMoveReq.NewDisplayOrders))

	for i := 0; i < len(budgetMoveReq.NewDisplayOrders); i++ {
		budgets[i] = &models.Budget{
			BudgetId:     budgetMoveReq.NewDisplayOrders[i].Id,
			DisplayOrder: budgetMoveReq.NewDisplayOrders[i].DisplayOrder,
		}
	}

	err = a.budgets.ModifyBudgetDisplayOrders(c, uid, budgets)

	if err != nil {
		log.Errorf(c, "[budgets.BudgetMoveHandler] failed to move budgets for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budgets.BudgetMoveHandler] user \"uid:%d\" has moved budgets", uid)

	return true, nil
}

// BudgetDeleteHandler deletes a budget by request parameters for current user
func (a *BudgetsApi) BudgetDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var budgetDeleteReq models.BudgetDeleteRequest
	err := c.ShouldBindJSON(&budgetDeleteReq)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	err = a.budgets.DeleteBudget(c, uid, budgetDeleteReq.Id)

	if err != nil {
		log.Errorf(c, "[budgets.BudgetDeleteHandler] failed to delete budget \"id:%d\" for user \"uid:%d\", because %s", budgetDeleteReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[budgets.BudgetDeleteHandler] user \"uid:%d\" has deleted budget \"id:%d\"", uid, budgetDeleteReq.Id)

	return true, nil
}

// BudgetProgressHandler returns budget progress for current user
func (a *BudgetsApi) BudgetProgressHandler(c *core.WebContext) (any, *errs.Error) {
	var budgetProgressReq models.BudgetProgressRequest
	err := c.ShouldBindQuery(&budgetProgressReq)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetProgressHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	budget, err := a.budgets.GetBudgetByBudgetId(c, uid, budgetProgressReq.Id)

	if err != nil {
		log.Errorf(c, "[budgets.BudgetProgressHandler] failed to get budget \"id:%d\" for user \"uid:%d\", because %s", budgetProgressReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	scopes, err := a.budgets.GetBudgetScopesByBudgetId(c, uid, budget.BudgetId)

	if err != nil {
		log.Errorf(c, "[budgets.BudgetProgressHandler] failed to get scopes for budget \"id:%d\" for user \"uid:%d\", because %s", budgetProgressReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	// Get current cycle or specified cycle
	var cycle *models.BudgetCycle
	if budgetProgressReq.StartTime > 0 && budgetProgressReq.EndTime > 0 {
		cycle, err = a.budgets.GetBudgetCycleByTimeRange(c, uid, budget.BudgetId, budgetProgressReq.StartTime, budgetProgressReq.EndTime)
	} else {
		cycle, err = a.budgets.GetOrCreateCurrentBudgetCycle(c, budget, scopes)
	}

	if err != nil {
		log.Errorf(c, "[budgets.BudgetProgressHandler] failed to get budget cycle for budget \"id:%d\" for user \"uid:%d\", because %s", budgetProgressReq.Id, uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	startTime := budget.StartTime
	endTime := budget.EndTime
	carryoverAmount := int64(0)

	if cycle != nil {
		startTime = cycle.StartTime
		endTime = cycle.EndTime
		carryoverAmount = cycle.CarryoverAmount
	}

	// Calculate spent amount from transactions
	spentAmount, err := a.budgets.GetBudgetSpentAmount(c, budget, scopes, startTime, endTime)

	if err != nil {
		log.Warnf(c, "[budgets.BudgetProgressHandler] failed to get spent amount for budget \"id:%d\" for user \"uid:%d\", because %s", budgetProgressReq.Id, uid, err.Error())
		spentAmount = 0
	}

	totalBudgetAmount := budget.Amount + carryoverAmount
	remainingAmount := totalBudgetAmount - spentAmount

	progressPercentage := float64(0)
	if totalBudgetAmount > 0 {
		progressPercentage = float64(spentAmount) / float64(totalBudgetAmount) * 100
	}

	isWarning := progressPercentage >= float64(budget.WarningThreshold) && progressPercentage < float64(budget.AlertThreshold)
	isAlert := progressPercentage >= float64(budget.AlertThreshold) && progressPercentage < float64(budget.OverspentThreshold)
	isOverspent := progressPercentage >= float64(budget.OverspentThreshold)

	progressResp := &models.BudgetProgressResponse{
		BudgetId:           budget.BudgetId,
		BudgetName:         budget.Name,
		BudgetAmount:       budget.Amount,
		Currency:           budget.Currency,
		SpentAmount:        spentAmount,
		RemainingAmount:    remainingAmount,
		CarryoverAmount:    carryoverAmount,
		TotalBudgetAmount:  totalBudgetAmount,
		ProgressPercentage: progressPercentage,
		StartTime:          startTime,
		EndTime:            endTime,
		WarningThreshold:   budget.WarningThreshold,
		AlertThreshold:     budget.AlertThreshold,
		OverspentThreshold: budget.OverspentThreshold,
		IsWarning:          isWarning,
		IsAlert:            isAlert,
		IsOverspent:        isOverspent,
	}

	return progressResp, nil
}

func fillBudgetScopes(budgetResp *models.BudgetInfoResponse, scopes []*models.BudgetScope) {
	categoryIds, accountIds, tagIds := models.SplitBudgetScopes(scopes)

	if len(categoryIds) > 0 {
		budgetResp.CategoryIds = utils.Int64ArrayToStringArray(categoryIds)
	}

	if len(accountIds) > 0 {
		budgetResp.AccountIds = utils.Int64ArrayToStringArray(accountIds)
	}

	if len(tagIds) > 0 {
		budgetResp.TagIds = utils.Int64ArrayToStringArray(tagIds)
	}
}

// buildBudgetScopes returns nil when the client sent no scope lists at all, so modifying keeps the existing scopes
func buildBudgetScopes(categoryIds []string, accountIds []string, tagIds []string) ([]*models.BudgetScope, error) {
	if categoryIds == nil && accountIds == nil && tagIds == nil {
		return nil, nil
	}

	scopes := make([]*models.BudgetScope, 0)

	for _, item := range []struct {
		scopeType models.BudgetScopeType
		ids       []string
	}{
		{models.BUDGET_SCOPE_TYPE_CATEGORY, categoryIds},
		{models.BUDGET_SCOPE_TYPE_ACCOUNT, accountIds},
		{models.BUDGET_SCOPE_TYPE_TAG, tagIds},
	} {
		ids, err := utils.StringArrayToInt64Array(item.ids)

		if err != nil {
			return nil, err
		}

		for _, id := range ids {
			scopes = append(scopes, &models.BudgetScope{
				ScopeType:   item.scopeType,
				ScopeItemId: id,
			})
		}
	}

	return scopes, nil
}
