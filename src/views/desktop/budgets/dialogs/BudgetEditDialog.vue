<template>
    <v-dialog width="800" :persistent="isBudgetModified" v-model="showState">
        <one-column-dialog-layout :disabled="loading || submitting" :loading="loading"
                                  :title="tt(title)" :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content>
                <v-form class="mt-2">
                    <v-row>
                        <v-col cols="12" md="12">
                            <v-text-field
                                type="text"
                                autocomplete="off"
                                persistent-placeholder
                                :disabled="loading || submitting"
                                :label="tt('Name')"
                                :placeholder="tt('Name')"
                                v-model="budget.name"
                            />
                        </v-col>
                        <v-col cols="12" md="6">
                            <amount-input
                                persistent-placeholder
                                :disabled="loading || submitting"
                                :currency="budget.currency"
                                :show-currency="true"
                                :label="tt('Budget Amount')"
                                :placeholder="tt('Budget Amount')"
                                v-model="budget.amount"
                            />
                        </v-col>
                        <v-col cols="12" md="6">
                            <currency-select
                                :disabled="loading || submitting"
                                :label="tt('Currency')"
                                :placeholder="tt('Currency')"
                                v-model="budget.currency"
                            />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-select
                                item-title="name"
                                item-value="type"
                                persistent-placeholder
                                :disabled="loading || submitting"
                                :label="tt('Period Type')"
                                :placeholder="tt('Period Type')"
                                :items="localizedPeriodTypes"
                                v-model="budget.periodType"
                            />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-select
                                item-title="name"
                                item-value="type"
                                persistent-placeholder
                                :disabled="loading || submitting"
                                :label="tt('Rollover Type')"
                                :placeholder="tt('Rollover Type')"
                                :items="localizedRolloverTypes"
                                v-model="budget.rolloverType"
                            />
                        </v-col>
                        <v-col cols="12" md="6">
                            <number-input
                                :persistent-placeholder="true"
                                :disabled="loading || submitting"
                                :min-value="0" :max-value="100" :max-decimal-count="0"
                                :label="tt('Warning Threshold (%)')"
                                :placeholder="tt('Warning Threshold (%)')"
                                v-model="budget.warningThreshold"
                            />
                        </v-col>
                        <v-col cols="12" md="6">
                            <number-input
                                :persistent-placeholder="true"
                                :disabled="loading || submitting"
                                :min-value="0" :max-value="100" :max-decimal-count="0"
                                :label="tt('Alert Threshold (%)')"
                                :placeholder="tt('Alert Threshold (%)')"
                                v-model="budget.alertThreshold"
                            />
                        </v-col>
                        <v-col class="py-0 my-n2" cols="12" md="12">
                            <v-switch :disabled="loading || submitting"
                                      :label="tt('Auto-create next cycle')" v-model="budget.autoCreateNextCycle"/>
                        </v-col>
                        <v-col class="py-0 my-n2" cols="12" md="12" v-if="editBudgetId">
                            <v-switch :disabled="loading || submitting"
                                      :label="tt('Visible')" v-model="budgetVisible"/>
                        </v-col>
                    </v-row>
                </v-form>
            </template>

            <template #footer>
                <v-spacer/>
                <v-tooltip :disabled="!inputIsEmpty" :text="inputEmptyProblemMessage ? tt(inputEmptyProblemMessage) : ''">
                    <template v-slot:activator="{ props }">
                        <div v-bind="props" class="d-inline-block">
                            <v-btn :disabled="inputIsEmpty || loading || submitting" @click="save">
                                {{ tt(saveButtonTitle) }}
                                <v-progress-circular indeterminate size="22" class="ms-2" v-if="submitting"></v-progress-circular>
                            </v-btn>
                        </div>
                    </template>
                </v-tooltip>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useBudgetEditPageBase } from '@/views/base/budgets/BudgetEditPageBase.ts';

import { useBudgetsStore } from '@/stores/budget.ts';
import { useUserStore } from '@/stores/user.ts';

import { Budget } from '@/models/budget.ts';

import { isEquals } from '@/lib/common.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

interface BudgetEditResponse {
    message: string;
}

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const {
    editBudgetId,
    clientSessionId,
    loading,
    submitting,
    budget,
    title,
    saveButtonTitle,
    inputEmptyProblemMessage,
    inputIsEmpty,
    allPeriodTypes,
    allRolloverTypes,
    prepareNewBudget,
    setBudget
} = useBudgetEditPageBase();

const budgetsStore = useBudgetsStore();
const userStore = useUserStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

let resolveFunc: ((value: BudgetEditResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const initBudget = ref<Budget | null>(null);

const isBudgetModified = computed<boolean>(() => {
    if (!editBudgetId.value) { // Add
        return !!initBudget.value && !isEquals(budget.value.toCreateRequest(clientSessionId.value), initBudget.value.toCreateRequest(clientSessionId.value));
    } else { // Edit
        return !!initBudget.value && !isEquals(budget.value.toModifyRequest(), initBudget.value.toModifyRequest());
    }
});

const budgetVisible = computed<boolean>({
    get: () => !budget.value.hidden,
    set: (value: boolean) => {
        budget.value.hidden = !value;
    }
});

const localizedPeriodTypes = computed<{ type: number, name: string }[]>(() => allPeriodTypes.map(item => ({ type: item.type, name: tt(item.name) })));
const localizedRolloverTypes = computed<{ type: number, name: string }[]>(() => allRolloverTypes.map(item => ({ type: item.type, name: tt(item.name) })));

function open(options: { id?: string; currentBudget?: Budget }): Promise<BudgetEditResponse> {
    showState.value = true;
    loading.value = true;
    submitting.value = false;
    editBudgetId.value = options.id ?? null;

    if (options.id) {
        initBudget.value = Budget.createNewBudget(userStore.currentUserDefaultCurrency);
        setBudget(initBudget.value);

        if (options.currentBudget) {
            setBudget(options.currentBudget);
        }

        budgetsStore.getBudget({
            budgetId: options.id
        }).then(response => {
            setBudget(response);
            initBudget.value = response.clone();
            loading.value = false;
        }).catch(error => {
            loading.value = false;
            showState.value = false;

            if (!error.processed) {
                rejectFunc?.(error);
            }
        });
    } else {
        // always a fresh budget (startTime = now, endTime = +30 days), never a stale one from a previous open
        prepareNewBudget(userStore.currentUserDefaultCurrency);
        initBudget.value = budget.value.clone();
        clientSessionId.value = generateRandomUUID();
        loading.value = false;
    }

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function save(): void {
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        snackbar.value?.showMessage(problemMessage);
        return;
    }

    submitting.value = true;

    budgetsStore.saveBudget({
        budget: editBudgetId.value ? budget.value.toModifyRequest() : budget.value.toCreateRequest(clientSessionId.value),
        isEdit: !!editBudgetId.value
    }).then(() => {
        submitting.value = false;

        const message = editBudgetId.value ? 'You have saved this budget' : 'You have added a new budget';

        resolveFunc?.({ message });
        showState.value = false;
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function cancel(): void {
    rejectFunc?.();
    showState.value = false;
}

defineExpose({
    open
});
</script>
