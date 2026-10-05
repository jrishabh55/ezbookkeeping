<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('Budgets') }}</span>
                        <v-btn density="compact" color="default" variant="text" size="24"
                               class="ms-2" :icon="true" :loading="loading" @click="reload">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                        </v-btn>
                        <v-spacer />
                        <v-btn color="primary" variant="tonal" size="small"
                               :disabled="loading" @click="add">
                            <v-icon :icon="mdiPlus" size="20" start />
                            {{ tt('Add') }}
                        </v-btn>
                    </div>
                </template>

                <v-card-text class="pb-0">
                    <v-row>
                        <v-col cols="12">
                            <v-alert type="info" variant="tonal" density="compact"
                                     :text="tt('Budgets help you track spending against limits you set for specific time periods.')" />
                        </v-col>
                    </v-row>
                </v-card-text>

                <v-card-text>
                    <v-row v-if="loading && !allBudgets.length">
                        <v-col cols="12" class="d-flex justify-center">
                            <v-progress-circular indeterminate />
                        </v-col>
                    </v-row>

                    <v-row v-else-if="!allBudgets.length">
                        <v-col cols="12">
                            <div class="text-center text-muted py-8">
                                <v-icon :icon="mdiWalletOutline" size="48" class="mb-4" />
                                <p>{{ tt('No budgets') }}</p>
                                <v-btn color="primary" variant="tonal" size="small" @click="add">
                                    {{ tt('Add Budget') }}
                                </v-btn>
                            </div>
                        </v-col>
                    </v-row>

                    <v-row v-else>
                        <v-col v-for="budget in allBudgets" :key="budget.id" cols="12" md="6" lg="4">
                            <v-card variant="outlined" class="budget-card">
                                <v-card-title class="d-flex align-center">
                                    <span class="text-truncate">{{ budget.name }}</span>
                                    <v-spacer />
                                    <v-chip size="x-small" :color="budget.hidden ? 'grey' : 'primary'" variant="tonal">
                                        {{ budget.hidden ? tt('Hidden') : budget.getPeriodTypeName() }}
                                    </v-chip>
                                </v-card-title>

                                <v-card-text>
                                    <div class="d-flex justify-space-between mb-2">
                                        <span class="text-muted">{{ tt('Budget Amount') }}</span>
                                        <span class="font-weight-bold">
                                            {{ formatAmount(budget.amount, budget.currency) }}
                                        </span>
                                    </div>

                                    <v-progress-linear
                                        :model-value="getProgressPercentage(budget.id)"
                                        :color="getProgressColor(budget.id)"
                                        height="8"
                                        rounded
                                        class="mb-2"
                                    />

                                    <div class="d-flex justify-space-between text-caption">
                                        <span>{{ tt('Spent') }}: {{ formatAmount(getSpentAmount(budget.id), budget.currency) }}</span>
                                        <span>{{ tt('Remaining') }}: {{ formatAmount(getRemainingAmount(budget), budget.currency) }}</span>
                                    </div>
                                </v-card-text>

                                <v-card-actions>
                                    <v-btn color="primary" variant="text" size="small" @click="viewBudget(budget)">
                                        {{ tt('View') }}
                                    </v-btn>
                                    <v-spacer />
                                    <v-btn variant="text" size="small" :icon="true" @click="editBudget(budget)">
                                        <v-icon :icon="mdiPencilOutline" size="18" />
                                        <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                                    </v-btn>
                                    <v-btn variant="text" size="small" :icon="true" @click="confirmDeleteBudget(budget)">
                                        <v-icon :icon="mdiDeleteOutline" size="18" />
                                        <v-tooltip activator="parent">{{ tt('Delete') }}</v-tooltip>
                                    </v-btn>
                                </v-card-actions>
                            </v-card>
                        </v-col>
                    </v-row>
                </v-card-text>
            </v-card>
        </v-col>
    </v-row>

    <budget-edit-dialog v-model:show="showEditDialog"
                        :budget="editingBudget"
                        :is-edit="isEditMode"
                        @saved="onBudgetSaved" />

    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, onMounted, useTemplateRef } from 'vue';
import { useI18n } from 'vue-i18n';

import {
    mdiRefresh,
    mdiPlus,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiWalletOutline
} from '@mdi/js';

import { useBudgetsStore } from '@/stores/budget.ts';

import { Budget } from '@/models/budget.ts';

import BudgetEditDialog from './dialogs/BudgetEditDialog.vue';
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;

const { t: tt } = useI18n();
const budgetsStore = useBudgetsStore();

const loading = ref(false);
const showEditDialog = ref(false);
const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const editingBudget = ref<Budget | null>(null);
const isEditMode = ref(false);
const budgetProgress = ref<Record<string, { spentAmount: number, remainingAmount: number, progressPercentage: number, isWarning: boolean, isAlert: boolean, isOverspent: boolean }>>({});

const allBudgets = computed(() => budgetsStore.allBudgets);

function formatAmount(amount: number, currency: string): string {
    const displayAmount = amount / 100;
    return new Intl.NumberFormat(undefined, {
        style: 'currency',
        currency: currency || 'USD'
    }).format(displayAmount);
}

function reload(): void {
    loading.value = true;
    budgetsStore.loadAllBudgets({ force: true }).then((budgets) => {
        loadBudgetProgress(budgets);
    }).finally(() => {
        loading.value = false;
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
    if (!progress) return 'primary';
    if (progress.isOverspent) return 'error';
    if (progress.isAlert) return 'warning';
    if (progress.isWarning) return 'orange';
    return 'primary';
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
    editingBudget.value = Budget.createNewBudget();
    isEditMode.value = false;
    showEditDialog.value = true;
}

function viewBudget(budget: Budget): void {
    editingBudget.value = budget;
    isEditMode.value = true;
    showEditDialog.value = true;
}

function editBudget(budget: Budget): void {
    editingBudget.value = budget;
    isEditMode.value = true;
    showEditDialog.value = true;
}

function confirmDeleteBudget(budget: Budget): void {
    confirmDialog.value?.open('Are you sure you want to delete this budget?').then(() => {
        loading.value = true;

        budgetsStore.deleteBudget({ budget }).catch(error => {
            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        }).finally(() => {
            loading.value = false;
        });
    });
}

function onBudgetSaved(): void {
    showEditDialog.value = false;
    reload();
}

onMounted(() => {
    reload();
});
</script>

<style scoped>
.budget-card {
    transition: box-shadow 0.2s ease;
}

.budget-card:hover {
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}
</style>
