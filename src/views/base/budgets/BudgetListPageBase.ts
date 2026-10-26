import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useBudgetsStore } from '@/stores/budget.ts';

import { type BudgetProgressResponse, Budget } from '@/models/budget.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { type BudgetProgressColor, clampBudgetProgressPercentage, getBudgetProgressColor } from '@/lib/budget.ts';

export function useBudgetListPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency } = useI18n();

    const budgetsStore = useBudgetsStore();

    const loading = ref<boolean>(true);
    const showHidden = ref<boolean>(false);
    const budgetProgresses = ref<Record<string, BudgetProgressResponse>>({});

    const allBudgets = computed<Budget[]>(() => budgetsStore.allBudgets);
    const displayBudgets = computed<Budget[]>(() => showHidden.value ? allBudgets.value : allBudgets.value.filter(budget => !budget.hidden));
    const noAvailableBudget = computed<boolean>(() => displayBudgets.value.length < 1);

    // never throws; a budget whose progress fails to load just keeps showing its budgeted amount with no spend
    function loadBudgetProgresses(budgets: Budget[]): Promise<void> {
        return Promise.all(budgets.map(budget =>
            budgetsStore.getBudgetProgress({ budgetId: budget.id }).then(progress => {
                budgetProgresses.value[budget.id] = progress;
            }).catch(() => {
                // keep whatever progress (if any) was already loaded for this budget
            })
        )).then(() => undefined);
    }

    function formatAmount(amount: number, currency: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), currency);
    }

    function getSpentAmount(budget: Budget): number {
        return budgetProgresses.value[budget.id]?.spentAmount ?? 0;
    }

    function getRemainingAmount(budget: Budget): number {
        return budgetProgresses.value[budget.id]?.remainingAmount ?? budget.amount;
    }

    function getProgressPercentage(budget: Budget): number {
        return clampBudgetProgressPercentage(budgetProgresses.value[budget.id]?.progressPercentage ?? 0);
    }

    function getProgressColor(budget: Budget): BudgetProgressColor {
        return getBudgetProgressColor(budgetProgresses.value[budget.id]);
    }

    function getPeriodDisplayName(budget: Budget): string {
        return budget.getPeriodTypeName();
    }

    return {
        // states
        loading,
        showHidden,
        budgetProgresses,
        // computed states
        allBudgets,
        displayBudgets,
        noAvailableBudget,
        // functions
        loadBudgetProgresses,
        formatAmount,
        getSpentAmount,
        getRemainingAmount,
        getProgressPercentage,
        getProgressColor,
        getPeriodDisplayName
    };
}
