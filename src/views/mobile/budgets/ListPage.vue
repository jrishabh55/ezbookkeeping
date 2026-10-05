<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Budgets')"></f7-nav-title>
            <f7-nav-right>
                <f7-link icon-f7="plus" @click="add"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-card class="skeleton-text" v-if="loading">
            <f7-card-header>
                <div class="budget-info">
                    <span class="budget-name">Budget Name</span>
                    <span class="budget-amount">$0.00</span>
                </div>
            </f7-card-header>
            <f7-card-content>
                <f7-progressbar :progress="0"></f7-progressbar>
                <div class="budget-details">
                    <span>Spent: $0.00</span>
                    <span>Remaining: $0.00</span>
                </div>
            </f7-card-content>
        </f7-card>

        <f7-list strong inset dividers class="margin-vertical" v-if="!loading && !allBudgets.length">
            <f7-list-item :title="tt('No budgets')"></f7-list-item>
        </f7-list>

        <template v-if="!loading && allBudgets.length">
            <f7-card v-for="budget in allBudgets" :key="budget.id">
            <f7-card-header>
                <div class="budget-info">
                    <span class="budget-name">{{ budget.name }}</span>
                    <f7-badge :color="budget.hidden ? 'gray' : 'blue'">
                        {{ budget.hidden ? tt('Hidden') : budget.getPeriodTypeName() }}
                    </f7-badge>
                </div>
            </f7-card-header>
            <f7-card-content>
                <div class="budget-amount-row">
                    <span class="label">{{ tt('Budget Amount') }}</span>
                    <span class="value">{{ formatAmount(budget.amount, budget.currency) }}</span>
                </div>
                <f7-progressbar :progress="getProgressPercentage(budget.id)" :color="getProgressColor(budget.id)"></f7-progressbar>
                <div class="budget-details">
                    <span>{{ tt('Spent') }}: {{ formatAmount(getSpentAmount(budget.id), budget.currency) }}</span>
                    <span>{{ tt('Remaining') }}: {{ formatAmount(getRemainingAmount(budget), budget.currency) }}</span>
                </div>
            </f7-card-content>
            <f7-card-footer>
                <f7-link @click="editBudget(budget)">{{ tt('Edit') }}</f7-link>
                <f7-link color="red" @click="confirmDelete(budget)">{{ tt('Delete') }}</f7-link>
            </f7-card-footer>
            </f7-card>
        </template>

        <f7-sheet class="budget-edit-sheet" :opened="showEditSheet" @sheet:closed="showEditSheet = false">
            <f7-toolbar>
                <div class="left">
                    <f7-link sheet-close>{{ tt('Cancel') }}</f7-link>
                </div>
                <div class="right">
                    <f7-link @click="saveBudget">{{ tt('Save') }}</f7-link>
                </div>
            </f7-toolbar>
            <f7-page-content>
                <f7-list strong-ios outline-ios dividers-ios>
                    <f7-list-input
                        :label="tt('Name')"
                        type="text"
                        :placeholder="tt('Budget name')"
                        v-model:value="editForm.name"
                    ></f7-list-input>
                    <f7-list-input
                        :label="tt('Budget Amount')"
                        type="number"
                        :placeholder="tt('Amount')"
                        v-model:value="editForm.amount"
                    ></f7-list-input>
                    <f7-list-input
                        :label="tt('Currency')"
                        type="select"
                        v-model:value="editForm.currency"
                    >
                        <option v-for="currency in allCurrencies" :key="currency.currencyCode" :value="currency.currencyCode">
                            {{ currency.displayName }}
                        </option>
                    </f7-list-input>
                    <f7-list-input
                        :label="tt('Period Type')"
                        type="select"
                        v-model:value="editForm.periodType"
                    >
                        <option :value="1">{{ tt('Custom') }}</option>
                        <option :value="2">{{ tt('Weekly') }}</option>
                        <option :value="3">{{ tt('Monthly') }}</option>
                        <option :value="4">{{ tt('Yearly') }}</option>
                    </f7-list-input>
                </f7-list>
            </f7-page-content>
        </f7-sheet>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import type { LocalizedCurrencyInfo } from '@/core/currency.ts';

import { useBudgetsStore } from '@/stores/budget.ts';
import { useUserStore } from '@/stores/user.ts';

import { Budget, BudgetPeriodType } from '@/models/budget.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

const { tt, getAllCurrencies } = useI18n();
const { showConfirm, showToast } = useI18nUIComponents();

const allCurrencies = computed<LocalizedCurrencyInfo[]>(() => getAllCurrencies());
const budgetsStore = useBudgetsStore();
const userStore = useUserStore();

const loading = ref(false);
const showEditSheet = ref(false);
const isEditMode = ref(false);
const editingBudget = ref<Budget | null>(null);

const editForm = ref<{
    name: string;
    amount: number;
    currency: string;
    periodType: number;
}>({
    name: '',
    amount: 0,
    currency: 'USD',
    periodType: BudgetPeriodType.Monthly.type
});

const allBudgets = computed(() => budgetsStore.allBudgets);
const budgetProgress = ref<Record<string, { spentAmount: number, remainingAmount: number, progressPercentage: number, isWarning: boolean, isAlert: boolean, isOverspent: boolean }>>({});

function formatAmount(amount: number, currency: string): string {
    const displayAmount = amount / 100;
    return new Intl.NumberFormat(undefined, {
        style: 'currency',
        currency: currency || 'USD'
    }).format(displayAmount);
}

function onPageAfterIn(): void {
    if (budgetsStore.budgetListStateInvalid) {
        reload(null);
    }
}

function reload(done: (() => void) | null): void {
    loading.value = true;
    budgetsStore.loadAllBudgets({ force: true }).then((budgets) => {
        loadBudgetProgress(budgets);
    }).finally(() => {
        loading.value = false;
        if (done) {
            done();
        }
    });
}

function loadBudgetProgress(budgets: Budget[]): void {
    for (const budget of budgets) {
        budgetsStore.getBudgetProgress({ budgetId: budget.id }).then((progress) => {
            budgetProgress.value[budget.id] = {
                spentAmount: progress.spentAmount,
                remainingAmount: progress.remainingAmount,
                progressPercentage: progress.progressPercentage,
                isWarning: progress.isWarning,
                isAlert: progress.isAlert,
                isOverspent: progress.isOverspent
            };
        }).catch(() => {
            budgetProgress.value[budget.id] = {
                spentAmount: 0,
                remainingAmount: budget.amount,
                progressPercentage: 0,
                isWarning: false,
                isAlert: false,
                isOverspent: false
            };
        });
    }
}

function getProgressColor(budgetId: string): string {
    const progress = budgetProgress.value[budgetId];
    if (!progress) return 'blue';
    if (progress.isOverspent) return 'red';
    if (progress.isAlert) return 'orange';
    if (progress.isWarning) return 'yellow';
    return 'blue';
}

function getSpentAmount(budgetId: string): number {
    return budgetProgress.value[budgetId]?.spentAmount || 0;
}

function getRemainingAmount(budget: Budget): number {
    return budgetProgress.value[budget.id]?.remainingAmount ?? budget.amount;
}

function getProgressPercentage(budgetId: string): number {
    return budgetProgress.value[budgetId]?.progressPercentage || 0;
}

function add(): void {
    editingBudget.value = null;
    isEditMode.value = false;
    editForm.value = {
        name: '',
        amount: 0,
        currency: userStore.currentUserDefaultCurrency || 'USD',
        periodType: BudgetPeriodType.Monthly.type
    };
    showEditSheet.value = true;
}

function editBudget(budget: Budget): void {
    editingBudget.value = budget;
    isEditMode.value = true;
    editForm.value = {
        name: budget.name,
        amount: budget.amount / 100,
        currency: budget.currency,
        periodType: budget.periodType
    };
    showEditSheet.value = true;
}

function saveBudget(): void {
    if (!editForm.value.name) {
        showToast('Name is required');
        return;
    }

    const now = Date.now();
    const startTime = Math.floor(now / 1000);
    const endTime = startTime + 30 * 24 * 60 * 60;

    // the mobile form only edits a few fields, keep the rest from the existing budget
    const existing = isEditMode.value ? editingBudget.value : null;

    const request = {
        id: existing ? existing.id : undefined,
        name: editForm.value.name,
        amount: Math.round(editForm.value.amount * 100),
        currency: editForm.value.currency,
        periodType: editForm.value.periodType,
        startTime: existing ? existing.startTime : startTime,
        endTime: existing ? existing.endTime : endTime,
        rolloverType: existing ? existing.rolloverType : 1,
        autoCreateNextCycle: existing ? existing.autoCreateNextCycle : true,
        warningThreshold: existing ? existing.warningThreshold : 75,
        alertThreshold: existing ? existing.alertThreshold : 90,
        overspentThreshold: existing ? existing.overspentThreshold : 100,
        visible: existing ? !existing.hidden : true,
        clientSessionId: generateRandomUUID()
    };

    showLoading();
    budgetsStore.saveBudget({ budget: request, isEdit: isEditMode.value })
        .then(() => {
            showEditSheet.value = false;
            showToast('Budget saved');
        })
        .catch(error => {
            showToast(error.message || 'Unable to save budget');
        })
        .finally(() => {
            hideLoading();
        });
}

function confirmDelete(budget: Budget): void {
    showConfirm('Are you sure you want to delete this budget?', () => deleteBudget(budget));
}

function deleteBudget(budget: Budget): void {
    showLoading();
    budgetsStore.deleteBudget({ budget })
        .then(() => {
            showToast('Budget deleted');
        })
        .catch(error => {
            showToast(error.message || 'Unable to delete budget');
        })
        .finally(() => {
            hideLoading();
        });
}

onMounted(() => {
    reload(null);
});
</script>

<style scoped>
.budget-info {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
}

.budget-name {
    font-weight: 600;
}

.budget-amount-row {
    display: flex;
    justify-content: space-between;
    margin-bottom: 8px;
}

.budget-amount-row .label {
    color: var(--f7-list-item-footer-text-color);
}

.budget-amount-row .value {
    font-weight: 600;
}

.budget-details {
    display: flex;
    justify-content: space-between;
    margin-top: 8px;
    font-size: 12px;
    color: var(--f7-list-item-footer-text-color);
}
</style>
