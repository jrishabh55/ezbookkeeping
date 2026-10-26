<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Budgets')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
                <f7-link icon-f7="plus" :aria-label="tt('Add')" href="/budget/add"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="Budget Name" after="$0.00" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]">
                <template #footer>
                    <div>Spent $0.00 · Remaining $0.00 · Monthly</div>
                    <f7-progressbar :progress="0"></f7-progressbar>
                </template>
            </f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && noAvailableBudget">
            <f7-list-item :title="tt('No available budget')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half budget-list" v-if="!loading && !noAvailableBudget">
            <f7-list-item swipeout
                          :id="getBudgetDomId(budget)"
                          :title="budget.name"
                          :after="formatAmount(budget.amount, budget.currency)"
                          :key="budget.id"
                          v-for="budget in displayBudgets">
                <template #footer>
                    <div>{{ tt('Spent') }}: {{ formatAmount(getSpentAmount(budget), budget.currency) }} · {{ tt('Remaining') }}: {{ formatAmount(getRemainingAmount(budget), budget.currency) }} · {{ tt(getPeriodDisplayName(budget)) }}</div>
                    <f7-progressbar :progress="getProgressPercentage(budget)" :color="getF7ProgressColor(budget)"></f7-progressbar>
                </template>
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR">
                    <f7-swipeout-button color="orange" close :text="tt('Edit')" @click="edit(budget)"></f7-swipeout-button>
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(budget, false)">
                        <f7-icon f7="trash"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button v-if="!showHidden" @click="showHidden = true">{{ tt('Show Hidden Budgets') }}</f7-actions-button>
                <f7-actions-button v-if="showHidden" @click="showHidden = false">{{ tt('Hide Hidden Budgets') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this budget?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(budgetToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading, onSwipeoutDeleted } from '@/lib/ui/mobile.ts';
import { useBudgetListPageBase } from '@/views/base/budgets/BudgetListPageBase.ts';

import { useBudgetsStore } from '@/stores/budget.ts';

import { TextDirection } from '@/core/text.ts';
import { Budget } from '@/models/budget.ts';

import type { BudgetProgressColor } from '@/lib/budget.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    showHidden,
    displayBudgets,
    noAvailableBudget,
    loadBudgetProgresses,
    formatAmount,
    getSpentAmount,
    getRemainingAmount,
    getProgressPercentage,
    getProgressColor,
    getPeriodDisplayName
} = useBudgetListPageBase();

const budgetsStore = useBudgetsStore();

const loadingError = ref<unknown | null>(null);
const budgetToDelete = ref<Budget | null>(null);
const showMoreActionSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

// the 4 progress states need to stay visually distinct from each other in Framework7's colour set
function getF7ProgressColor(budget: Budget): string {
    const color: BudgetProgressColor = getProgressColor(budget);

    switch (color) {
        case 'orange':
            return 'orange';
        case 'warning':
            return 'yellow';
        case 'error':
            return 'red';
        default:
            return 'blue';
    }
}

function getBudgetDomId(budget: Budget): string {
    return 'budget_' + budget.id;
}

function init(): void {
    loading.value = true;

    budgetsStore.loadAllBudgets({ force: false }).then(() => {
        loading.value = false;
        loadBudgetProgresses(displayBudgets.value);
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function reload(done?: () => void): void {
    const force = !!done;

    budgetsStore.loadAllBudgets({ force: force }).then(() => {
        done?.();

        if (force) {
            showToast('Budget list has been updated');
        }

        loadBudgetProgresses(displayBudgets.value);
    }).catch(error => {
        done?.();

        if (error.isUpToDate) {
            showToast('Budget list is up to date');
            loadBudgetProgresses(displayBudgets.value);
        } else if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function edit(budget: Budget): void {
    props.f7router.navigate(`/budget/edit?id=${budget.id}`);
}

function remove(budget: Budget | null, confirm: boolean): void {
    if (!budget) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        budgetToDelete.value = budget;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    budgetToDelete.value = null;
    showLoading();

    budgetsStore.deleteBudget({
        budget: budget,
        beforeResolve: (done) => {
            onSwipeoutDeleted(getBudgetDomId(budget), done);
        }
    }).then(() => {
        hideLoading();
    }).catch(error => {
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    if (!loading.value) {
        reload();
    }

    routeBackOnError(props.f7router, loadingError);
}

init();
</script>

<style>
.budget-list {
    --f7-list-item-footer-font-size: var(--ebk-large-footer-font-size);
}

.budget-list .item-footer {
    padding-top: 4px;
}
</style>
