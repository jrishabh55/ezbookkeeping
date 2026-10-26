<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('Budgets') }}</span>
                        <v-btn class="ms-3" color="default" variant="outlined"
                               :disabled="loading" @click="add">{{ tt('Add') }}</v-btn>
                        <v-btn density="compact" color="default" variant="text" class="ms-2"
                               :aria-label="tt('Refresh')" :icon="true" :loading="loading" @click="reload(true)">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                        </v-btn>
                        <v-spacer/>
                        <v-btn density="comfortable" color="default" variant="text" class="ms-2"
                               :aria-label="tt('More')" :disabled="loading" :icon="true">
                            <v-icon :icon="mdiDotsVertical" />
                            <v-menu activator="parent">
                                <v-list>
                                    <v-list-item :prepend-icon="mdiEyeOutline"
                                                 :title="tt('Show Hidden Budgets')"
                                                 v-if="!showHidden" @click="showHidden = true"></v-list-item>
                                    <v-list-item :prepend-icon="mdiEyeOffOutline"
                                                 :title="tt('Hide Hidden Budgets')"
                                                 v-if="showHidden" @click="showHidden = false"></v-list-item>
                                </v-list>
                            </v-menu>
                        </v-btn>
                    </div>
                </template>

                <v-card-text class="pb-0">
                    <v-row>
                        <v-col cols="12">
                            <v-alert variant="tonal" density="compact"
                                     :text="tt('Budgets help you track spending against limits you set for specific time periods.')" />
                        </v-col>
                    </v-row>
                </v-card-text>

                <v-card-text>
                    <v-row v-if="loading && noAvailableBudget">
                        <v-col v-for="itemIdx in [ 1, 2, 3 ]" :key="itemIdx" cols="12" md="6" lg="4">
                            <v-card variant="outlined" class="budget-card">
                                <v-card-title class="d-flex align-center">
                                    <v-skeleton-loader class="skeleton-no-margin" width="120px" type="text" :loading="true"></v-skeleton-loader>
                                    <v-spacer />
                                    <v-skeleton-loader class="skeleton-no-margin" width="60px" type="text" :loading="true"></v-skeleton-loader>
                                </v-card-title>

                                <v-card-text>
                                    <div class="d-flex justify-space-between align-center mb-2">
                                        <span class="text-medium-emphasis">{{ tt('Budget Amount') }}</span>
                                        <v-skeleton-loader class="skeleton-no-margin" width="80px" type="text" :loading="true"></v-skeleton-loader>
                                    </div>

                                    <v-skeleton-loader class="skeleton-no-margin mb-2" height="8" type="image" :loading="true"></v-skeleton-loader>

                                    <div class="d-flex justify-space-between text-caption">
                                        <v-skeleton-loader class="skeleton-no-margin" width="100px" type="text" :loading="true"></v-skeleton-loader>
                                        <v-skeleton-loader class="skeleton-no-margin" width="100px" type="text" :loading="true"></v-skeleton-loader>
                                    </div>
                                </v-card-text>
                            </v-card>
                        </v-col>
                    </v-row>

                    <v-row v-else-if="!loading && noAvailableBudget">
                        <v-col cols="12">
                            <span class="text-body-medium">{{ tt('No available budget') }}</span>
                        </v-col>
                    </v-row>

                    <v-row v-else>
                        <v-col v-for="budget in displayBudgets" :key="budget.id" cols="12" md="6" lg="4">
                            <v-card variant="outlined" class="budget-card">
                                <v-card-title class="d-flex align-center">
                                    <span class="text-truncate">{{ budget.name }}</span>
                                    <v-spacer />
                                    <v-chip size="x-small" :color="budget.hidden ? 'grey' : 'primary'" variant="tonal">
                                        {{ budget.hidden ? tt('Hidden') : tt(getPeriodDisplayName(budget)) }}
                                    </v-chip>
                                </v-card-title>

                                <v-card-text>
                                    <div class="d-flex justify-space-between mb-2">
                                        <span class="text-medium-emphasis">{{ tt('Budget Amount') }}</span>
                                        <span class="font-weight-bold">
                                            {{ formatAmount(budget.amount, budget.currency) }}
                                        </span>
                                    </div>

                                    <v-progress-linear
                                        :model-value="getProgressPercentage(budget)"
                                        :color="getProgressColor(budget)"
                                        height="8"
                                        rounded
                                        class="mb-2"
                                    />

                                    <div class="d-flex justify-space-between text-caption">
                                        <span>{{ tt('Spent') }}: {{ formatAmount(getSpentAmount(budget), budget.currency) }}</span>
                                        <span>{{ tt('Remaining') }}: {{ formatAmount(getRemainingAmount(budget), budget.currency) }}</span>
                                    </div>
                                </v-card-text>

                                <v-card-actions>
                                    <v-spacer />
                                    <v-btn variant="text" size="small" :icon="true" :disabled="loading" @click="edit(budget)">
                                        <v-icon :icon="mdiPencilOutline" size="18" />
                                        <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                                    </v-btn>
                                    <v-btn variant="text" size="small" :icon="true" :disabled="loading" @click="confirmDeleteBudget(budget)">
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

    <budget-edit-dialog ref="editDialog" />

    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { useTemplateRef } from 'vue';

import {
    mdiRefresh,
    mdiDotsVertical,
    mdiEyeOutline,
    mdiEyeOffOutline,
    mdiPencilOutline,
    mdiDeleteOutline
} from '@mdi/js';

import { useI18n } from '@/locales/helpers.ts';
import { useBudgetListPageBase } from '@/views/base/budgets/BudgetListPageBase.ts';

import { useBudgetsStore } from '@/stores/budget.ts';

import { Budget } from '@/models/budget.ts';

import BudgetEditDialog from './dialogs/BudgetEditDialog.vue';
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type BudgetEditDialogType = InstanceType<typeof BudgetEditDialog>;

const { tt } = useI18n();

const {
    loading,
    showHidden,
    allBudgets,
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

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const editDialog = useTemplateRef<BudgetEditDialogType>('editDialog');

function init(): void {
    budgetsStore.loadAllBudgets({ force: false }).then(() => {
        loading.value = false;

        loadBudgetProgresses(allBudgets.value);
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function reload(force: boolean): void {
    loading.value = true;

    budgetsStore.loadAllBudgets({ force: force }).then(() => {
        loading.value = false;

        if (force) {
            snackbar.value?.showMessage('Budget list has been updated');
        }

        loadBudgetProgresses(allBudgets.value);
    }).catch(error => {
        loading.value = false;

        if (error && error.isUpToDate) {
            loadBudgetProgresses(allBudgets.value);
        }

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function add(): void {
    editDialog.value?.open({}).then(result => {
        if (result && result.message) {
            snackbar.value?.showMessage(result.message);
        }

        loadBudgetProgresses(allBudgets.value);
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function edit(budget: Budget): void {
    editDialog.value?.open({
        id: budget.id,
        currentBudget: budget
    }).then(result => {
        if (result && result.message) {
            snackbar.value?.showMessage(result.message);
        }

        loadBudgetProgresses(allBudgets.value);
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function confirmDeleteBudget(budget: Budget): void {
    confirmDialog.value?.open('Are you sure you want to delete this budget?').then(() => {
        loading.value = true;

        budgetsStore.deleteBudget({ budget }).then(() => {
            loading.value = false;
        }).catch(error => {
            loading.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

init();
</script>

<style scoped>
.budget-card {
    transition: box-shadow 0.2s ease;
}

.budget-card:hover {
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}
</style>
