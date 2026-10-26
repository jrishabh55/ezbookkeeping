import { ref, computed } from 'vue';

import { useUserStore } from '@/stores/user.ts';

import { Budget, BudgetPeriodType, BudgetRolloverType } from '@/models/budget.ts';

import { isValidBudgetThreshold } from '@/lib/budget.ts';
import { getCurrentUnixTime } from '@/lib/datetime.ts';

// a new budget needs a concrete cycle before it can be saved: the backend rejects Custom-period budgets
// whose startTime >= endTime, and anchors other period types' first cycle at startTime (otherwise it would
// default to 0, i.e. 1970), so every freshly created budget starts now and runs for 30 days
const NEW_BUDGET_DEFAULT_CYCLE_SECONDS = 30 * 24 * 60 * 60;

export function useBudgetEditPageBase() {
    const userStore = useUserStore();

    const editBudgetId = ref<string | null>(null);
    const clientSessionId = ref<string>('');
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const budget = ref<Budget>(createNewBudgetWithDefaultCycle(userStore.currentUserDefaultCurrency));

    const title = computed<string>(() => editBudgetId.value ? 'Edit Budget' : 'Add Budget');
    const saveButtonTitle = computed<string>(() => editBudgetId.value ? 'Save' : 'Add');

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!budget.value.name) {
            return 'Budget name cannot be blank';
        } else if (!(budget.value.amount > 0)) {
            return 'Budget amount must be greater than zero';
        } else if (!budget.value.currency) {
            return 'Currency cannot be blank';
        } else if (!isValidBudgetThreshold(budget.value.warningThreshold) || !isValidBudgetThreshold(budget.value.alertThreshold)) {
            return 'Thresholds must be between 0 and 100';
        } else {
            return null;
        }
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    const allPeriodTypes: { type: number, name: string }[] = [
        BudgetPeriodType.Weekly,
        BudgetPeriodType.Monthly,
        BudgetPeriodType.Yearly,
        BudgetPeriodType.Custom
    ];

    const allRolloverTypes: { type: number, name: string }[] = [
        BudgetRolloverType.Reset,
        BudgetRolloverType.Rollover
    ];

    function createNewBudgetWithDefaultCycle(currency: string): Budget {
        const newBudget = Budget.createNewBudget(currency);
        newBudget.startTime = getCurrentUnixTime();
        newBudget.endTime = newBudget.startTime + NEW_BUDGET_DEFAULT_CYCLE_SECONDS;
        return newBudget;
    }

    // resets to a brand new budget (add mode); always gives it a fresh startTime, never a stale one from a previous open
    function prepareNewBudget(currency: string): void {
        budget.value = createNewBudgetWithDefaultCycle(currency);
    }

    function setBudget(source: Budget): void {
        budget.value = source.clone();
    }

    return {
        // states
        editBudgetId,
        clientSessionId,
        loading,
        submitting,
        budget,
        // computed states
        title,
        saveButtonTitle,
        inputEmptyProblemMessage,
        inputIsEmpty,
        // constants
        allPeriodTypes,
        allRolloverTypes,
        // functions
        prepareNewBudget,
        setBudget
    };
}
