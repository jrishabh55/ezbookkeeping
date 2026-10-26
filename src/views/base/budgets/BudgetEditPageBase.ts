import { ref, computed } from 'vue';

import { useUserStore } from '@/stores/user.ts';

import { Budget, BudgetPeriodType, BudgetRolloverType } from '@/models/budget.ts';

import { isValidBudgetThreshold } from '@/lib/budget.ts';

export function useBudgetEditPageBase() {
    const userStore = useUserStore();

    const editBudgetId = ref<string | null>(null);
    const clientSessionId = ref<string>('');
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const budget = ref<Budget>(Budget.createNewBudget(userStore.currentUserDefaultCurrency));

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
        setBudget
    };
}
