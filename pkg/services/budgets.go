package services

import (
	"time"

	"xorm.io/builder"
	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

// BudgetService represents budget service
type BudgetService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

// Initialize a budget service singleton instance
var (
	Budgets = &BudgetService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
		ServiceUsingUuid: ServiceUsingUuid{
			container: uuid.Container,
		},
	}
)

// GetTotalBudgetCountByUid returns total budget count of user
func (s *BudgetService) GetTotalBudgetCountByUid(c core.Context, uid int64) (int64, error) {
	if uid <= 0 {
		return 0, errs.ErrUserIdInvalid
	}

	count, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).Count(&models.Budget{})

	return count, err
}

// GetAllBudgetsByUid returns all budget models of user
func (s *BudgetService) GetAllBudgetsByUid(c core.Context, uid int64) ([]*models.Budget, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var budgets []*models.Budget
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=?", uid, false).OrderBy("display_order asc").Find(&budgets)

	return budgets, err
}

// GetBudgetByBudgetId returns budget model according to budget id
func (s *BudgetService) GetBudgetByBudgetId(c core.Context, uid int64, budgetId int64) (*models.Budget, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if budgetId <= 0 {
		return nil, errs.ErrBudgetIdInvalid
	}

	budget := &models.Budget{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND deleted=? AND budget_id=?", uid, false, budgetId).Get(budget)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, errs.ErrBudgetNotFound
	}

	return budget, nil
}

// GetBudgetScopesByBudgetId returns all budget scopes for a budget
func (s *BudgetService) GetBudgetScopesByBudgetId(c core.Context, uid int64, budgetId int64) ([]*models.BudgetScope, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if budgetId <= 0 {
		return nil, errs.ErrBudgetIdInvalid
	}

	var scopes []*models.BudgetScope
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND budget_id=?", uid, budgetId).Find(&scopes)

	return scopes, err
}

// GetCurrentBudgetCycle returns the current active cycle for a budget
func (s *BudgetService) GetCurrentBudgetCycle(c core.Context, uid int64, budgetId int64) (*models.BudgetCycle, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if budgetId <= 0 {
		return nil, errs.ErrBudgetIdInvalid
	}

	now := time.Now().Unix()
	cycle := &models.BudgetCycle{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND budget_id=? AND start_time<=? AND end_time>=?", uid, budgetId, now, now).Get(cycle)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}

	return cycle, nil
}

// GetBudgetCycleByTimeRange returns the budget cycle for a specific time range
func (s *BudgetService) GetBudgetCycleByTimeRange(c core.Context, uid int64, budgetId int64, startTime int64, endTime int64) (*models.BudgetCycle, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if budgetId <= 0 {
		return nil, errs.ErrBudgetIdInvalid
	}

	cycle := &models.BudgetCycle{}
	has, err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND budget_id=? AND start_time=? AND end_time=?", uid, budgetId, startTime, endTime).Get(cycle)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}

	return cycle, nil
}

// GetOrCreateCurrentBudgetCycle returns the latest cycle of a budget, creating the cycles that have elapsed since then when the budget repeats
func (s *BudgetService) GetOrCreateCurrentBudgetCycle(c core.Context, budget *models.Budget, scopes []*models.BudgetScope) (*models.BudgetCycle, error) {
	if budget.Uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	cycle := &models.BudgetCycle{}
	has, err := s.UserDataDB(budget.Uid).NewSession(c).Where("uid=? AND budget_id=?", budget.Uid, budget.BudgetId).OrderBy("start_time desc").Limit(1).Get(cycle)

	if err != nil {
		return nil, err
	} else if !has {
		cycle = s.newBudgetCycle(budget, budget.StartTime, budget.EndTime, 0)

		if _, err = s.UserDataDB(budget.Uid).NewSession(c).Insert(cycle); err != nil {
			return nil, err
		}
	}

	if !budget.AutoCreateNextCycle {
		return cycle, nil
	}

	now := time.Now().Unix()

	// ponytail: cycles are created lazily when progress is requested, two concurrent requests may both create the next cycle; move to a cron job with a unique index if that matters
	for i := 0; i < 1000 && cycle.EndTime < now; i++ {
		spentAmount, err := s.GetBudgetSpentAmount(c, budget, scopes, cycle.StartTime, cycle.EndTime)

		if err != nil {
			return nil, err
		}

		cycle, err = s.CreateNextBudgetCycle(c, budget, cycle, budget.Amount+cycle.CarryoverAmount-spentAmount)

		if err != nil {
			return nil, err
		}
	}

	return cycle, nil
}

// CreateBudget saves a new budget to database
func (s *BudgetService) CreateBudget(c core.Context, budget *models.Budget, scopes []*models.BudgetScope) error {
	if budget.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	budget.BudgetId = s.GenerateUuid(uuid.UUID_TYPE_BUDGET)

	if budget.BudgetId < 1 {
		return errs.ErrSystemIsBusy
	}

	budget.Deleted = false
	budget.CreatedUnixTime = time.Now().Unix()
	budget.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(budget.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Insert(budget)

		if err != nil {
			return err
		}

		if len(scopes) > 0 {
			for i := 0; i < len(scopes); i++ {
				scopes[i].ScopeId = s.GenerateUuid(uuid.UUID_TYPE_BUDGET)
				scopes[i].BudgetId = budget.BudgetId
				scopes[i].Uid = budget.Uid
				scopes[i].CreatedUnixTime = time.Now().Unix()
			}

			_, err = sess.Insert(scopes)

			if err != nil {
				return err
			}
		}

		// Create initial budget cycle
		_, err = sess.Insert(s.newBudgetCycle(budget, budget.StartTime, budget.EndTime, 0))

		return err
	})
}

// ModifyBudget saves an existing budget to database
func (s *BudgetService) ModifyBudget(c core.Context, budget *models.Budget, scopes []*models.BudgetScope) error {
	if budget.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if budget.BudgetId <= 0 {
		return errs.ErrBudgetIdInvalid
	}

	budget.UpdatedUnixTime = time.Now().Unix()

	return s.UserDataDB(budget.Uid).DoTransaction(c, func(sess *xorm.Session) error {
		existingBudget := &models.Budget{}
		has, err := sess.Where("uid=? AND deleted=? AND budget_id=?", budget.Uid, false, budget.BudgetId).Get(existingBudget)

		if err != nil {
			return err
		} else if !has {
			return errs.ErrBudgetNotFound
		}

		updatedRows, err := sess.ID(budget.BudgetId).Where("uid=? AND deleted=?", budget.Uid, false).Cols("name", "amount", "currency", "period_type", "start_time", "end_time", "rollover_type", "auto_create_next_cycle", "warning_threshold", "alert_threshold", "overspent_threshold", "visible", "updated_unix_time").Update(budget)

		if err != nil {
			return err
		} else if updatedRows < 1 {
			return errs.ErrBudgetNotFound
		}

		// The budget window changed, so the existing cycles no longer match it, restart from the new window
		if existingBudget.StartTime != budget.StartTime || existingBudget.EndTime != budget.EndTime || existingBudget.PeriodType != budget.PeriodType {
			if _, err = sess.Where("uid=? AND budget_id=?", budget.Uid, budget.BudgetId).Delete(&models.BudgetCycle{}); err != nil {
				return err
			}

			if _, err = sess.Insert(s.newBudgetCycle(budget, budget.StartTime, budget.EndTime, 0)); err != nil {
				return err
			}
		}

		// nil means the client did not send scopes, keep the existing ones
		if scopes == nil {
			return nil
		}

		// Delete existing scopes and recreate
		_, err = sess.Where("uid=? AND budget_id=?", budget.Uid, budget.BudgetId).Delete(&models.BudgetScope{})

		if err != nil {
			return err
		}

		if len(scopes) > 0 {
			for i := 0; i < len(scopes); i++ {
				scopes[i].ScopeId = s.GenerateUuid(uuid.UUID_TYPE_BUDGET)
				scopes[i].BudgetId = budget.BudgetId
				scopes[i].Uid = budget.Uid
				scopes[i].CreatedUnixTime = time.Now().Unix()
			}

			_, err = sess.Insert(scopes)

			if err != nil {
				return err
			}
		}

		return nil
	})
}

// HideBudget updates visibility of a budget
func (s *BudgetService) HideBudget(c core.Context, uid int64, budgetId int64, hidden bool) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if budgetId <= 0 {
		return errs.ErrBudgetIdInvalid
	}

	now := time.Now().Unix()

	updateModel := &models.Budget{
		Visible:         !hidden,
		UpdatedUnixTime: now,
	}

	updatedRows, err := s.UserDataDB(uid).NewSession(c).ID(budgetId).Where("uid=? AND deleted=?", uid, false).Cols("visible", "updated_unix_time").Update(updateModel)

	if err != nil {
		return err
	} else if updatedRows < 1 {
		return errs.ErrBudgetNotFound
	}

	return nil
}

// ModifyBudgetDisplayOrders updates display orders of budgets
func (s *BudgetService) ModifyBudgetDisplayOrders(c core.Context, uid int64, budgets []*models.Budget) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	for i := 0; i < len(budgets); i++ {
		budgets[i].UpdatedUnixTime = time.Now().Unix()
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		for i := 0; i < len(budgets); i++ {
			budget := budgets[i]
			updatedRows, err := sess.ID(budget.BudgetId).Where("uid=? AND deleted=?", uid, false).Cols("display_order", "updated_unix_time").Update(budget)

			if err != nil {
				return err
			} else if updatedRows < 1 {
				return errs.ErrBudgetNotFound
			}
		}

		return nil
	})
}

// DeleteBudget deletes a budget
func (s *BudgetService) DeleteBudget(c core.Context, uid int64, budgetId int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if budgetId <= 0 {
		return errs.ErrBudgetIdInvalid
	}

	now := time.Now().Unix()

	updateModel := &models.Budget{
		Deleted:         true,
		DeletedUnixTime: now,
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		deletedRows, err := sess.ID(budgetId).Where("uid=? AND deleted=?", uid, false).Cols("deleted", "deleted_unix_time").Update(updateModel)

		if err != nil {
			return err
		} else if deletedRows < 1 {
			return errs.ErrBudgetNotFound
		}

		// Delete scopes
		_, err = sess.Where("uid=? AND budget_id=?", uid, budgetId).Delete(&models.BudgetScope{})

		return err
	})
}

// UpdateBudgetCycleNotificationStatus updates notification flags for a budget cycle
func (s *BudgetService) UpdateBudgetCycleNotificationStatus(c core.Context, uid int64, cycleId int64, notifiedWarning bool, notifiedAlert bool, notifiedOverspent bool) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	if cycleId <= 0 {
		return errs.ErrBudgetCycleNotFound
	}

	now := time.Now().Unix()

	updateModel := &models.BudgetCycle{
		NotifiedWarning:   notifiedWarning,
		NotifiedAlert:     notifiedAlert,
		NotifiedOverspent: notifiedOverspent,
		UpdatedUnixTime:   now,
	}

	_, err := s.UserDataDB(uid).NewSession(c).ID(cycleId).Where("uid=?", uid).Cols("notified_warning", "notified_alert", "notified_overspent", "updated_unix_time").Update(updateModel)

	return err
}

// GetBudgetSpentAmount calculates the total expense amount for a budget within a time range, only expenses from accounts in the budget currency are counted
func (s *BudgetService) GetBudgetSpentAmount(c core.Context, budget *models.Budget, scopes []*models.BudgetScope, startTime int64, endTime int64) (int64, error) {
	if budget.Uid <= 0 {
		return 0, errs.ErrUserIdInvalid
	}

	if budget.BudgetId <= 0 {
		return 0, errs.ErrBudgetIdInvalid
	}

	uid := budget.Uid
	categoryIds, accountIds, tagIds := models.SplitBudgetScopes(scopes)

	// ponytail: expenses in other currencies are skipped instead of converted, convert with exchange rates if multi-currency budgets are needed
	sameCurrencyAccounts := builder.Select("account_id").From("account").Where(builder.Eq{"uid": uid, "deleted": false, "currency": budget.Currency})

	sess := s.UserDataDB(uid).NewSession(c).Table("transaction").
		Where("uid=? AND deleted=? AND type=?", uid, false, models.TRANSACTION_DB_TYPE_EXPENSE).
		And("transaction_time>=? AND transaction_time<=?", utils.GetMinTransactionTimeFromUnixTime(startTime), utils.GetMaxTransactionTimeFromUnixTime(endTime)).
		And(builder.In("account_id", sameCurrencyAccounts))

	if len(categoryIds) > 0 {
		sess = sess.In("category_id", categoryIds)
	}

	if len(accountIds) > 0 {
		sess = sess.In("account_id", accountIds)
	}

	if len(tagIds) > 0 {
		taggedTransactions := builder.Select("transaction_id").From("transaction_tag_index").Where(builder.Eq{"uid": uid, "deleted": false}.And(builder.In("tag_id", tagIds)))
		sess = sess.And(builder.In("transaction_id", taggedTransactions))
	}

	return sess.SumInt(&models.Transaction{}, "amount")
}

// CreateNextBudgetCycle creates a new cycle for a repeating budget
// previousCycleRemainingAmount should be the remaining budget from the previous cycle (totalBudget - spentAmount)
func (s *BudgetService) CreateNextBudgetCycle(c core.Context, budget *models.Budget, previousCycle *models.BudgetCycle, previousCycleRemainingAmount int64) (*models.BudgetCycle, error) {
	if budget.Uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	nextStartTime := previousCycle.EndTime + 1
	nextEndTime := models.GetBudgetPeriodEndTime(budget.PeriodType, nextStartTime, previousCycle.EndTime-previousCycle.StartTime)

	carryoverAmount := int64(0)
	if budget.RolloverType == models.BUDGET_ROLLOVER_TYPE_ROLLOVER {
		carryoverAmount = previousCycleRemainingAmount
	}

	cycle := s.newBudgetCycle(budget, nextStartTime, nextEndTime, carryoverAmount)

	_, err := s.UserDataDB(budget.Uid).NewSession(c).Insert(cycle)

	if err != nil {
		return nil, err
	}

	return cycle, nil
}

func (s *BudgetService) newBudgetCycle(budget *models.Budget, startTime int64, endTime int64, carryoverAmount int64) *models.BudgetCycle {
	now := time.Now().Unix()

	return &models.BudgetCycle{
		CycleId:         s.GenerateUuid(uuid.UUID_TYPE_BUDGET),
		BudgetId:        budget.BudgetId,
		Uid:             budget.Uid,
		StartTime:       startTime,
		EndTime:         endTime,
		CarryoverAmount: carryoverAmount,
		CreatedUnixTime: now,
		UpdatedUnixTime: now,
	}
}
