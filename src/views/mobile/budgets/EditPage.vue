<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt(title)"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-input label="Name" placeholder="Budget name"></f7-list-input>
            <f7-list-item class="list-item-with-header-and-title" header="Budget Amount" title="$0.00"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title list-item-no-item-after" header="Currency" title="Currency"></f7-list-item>
        </f7-list>

        <f7-list form strong inset dividers class="margin-vertical" v-else-if="!loading">
            <template #list>
                <f7-list-input
                    type="text"
                    autocomplete="off"
                    clear-button
                    :label="tt('Name')"
                    :placeholder="tt('Name')"
                    v-model:value="budget.name"
                ></f7-list-input>

                <f7-list-item
                    link="#" no-chevron
                    class="list-item-with-header-and-title"
                    :header="tt('Budget Amount')"
                    :title="formatAmount(budget.amount, budget.currency)"
                    @click="showAmountSheet = true"
                >
                    <number-pad-sheet :min-value="0"
                                      :max-value="TRANSACTION_MAX_AMOUNT"
                                      :currency="budget.currency"
                                      v-model:show="showAmountSheet"
                                      v-model="budget.amount"
                    ></number-pad-sheet>
                </f7-list-item>

                <f7-list-item
                    class="list-item-with-header-and-title list-item-no-item-after"
                    link="#"
                    :header="tt('Currency')"
                    @click="showCurrencyPopup = true"
                >
                    <template #title>
                        <div class="no-padding no-margin">
                            <span>{{ getCurrencyName(budget.currency) }}&nbsp;</span>
                            <small class="smaller">{{ budget.currency }}</small>
                        </div>
                    </template>
                    <list-item-selection-popup value-type="item"
                                               key-field="currencyCode" value-field="currencyCode"
                                               title-field="displayName" after-field="currencyCode"
                                               :title="tt('Currency Name')"
                                               :enable-filter="true"
                                               :filter-placeholder="tt('Currency')"
                                               :filter-no-items-text="tt('No results')"
                                               :items="allCurrencies"
                                               v-model:show="showCurrencyPopup"
                                               v-model="budget.currency">
                    </list-item-selection-popup>
                </f7-list-item>

                <f7-list-item
                    link="#" no-chevron
                    class="list-item-with-header-and-title"
                    :header="tt('Period Type')"
                    :title="tt(budget.getPeriodTypeName())"
                    @click="showPeriodTypeSheet = true"
                >
                    <list-item-selection-sheet value-type="item"
                                               key-field="type" value-field="type" title-field="name"
                                               :title-i18n="true"
                                               :items="allPeriodTypes"
                                               v-model:show="showPeriodTypeSheet"
                                               v-model="budget.periodType">
                    </list-item-selection-sheet>
                </f7-list-item>

                <f7-list-item
                    link="#" no-chevron
                    class="list-item-with-header-and-title"
                    :header="tt('Rollover Type')"
                    :title="tt(budget.getRolloverTypeName())"
                    @click="showRolloverTypeSheet = true"
                >
                    <list-item-selection-sheet value-type="item"
                                               key-field="type" value-field="type" title-field="name"
                                               :title-i18n="true"
                                               :items="allRolloverTypes"
                                               v-model:show="showRolloverTypeSheet"
                                               v-model="budget.rolloverType">
                    </list-item-selection-sheet>
                </f7-list-item>

                <list-number-input :label="tt('Warning Threshold (%)')" :placeholder="tt('Warning Threshold (%)')"
                                   :min-value="0" :max-value="100" :max-decimal-count="0"
                                   v-model="budget.warningThreshold"></list-number-input>

                <list-number-input :label="tt('Alert Threshold (%)')" :placeholder="tt('Alert Threshold (%)')"
                                   :min-value="0" :max-value="100" :max-decimal-count="0"
                                   v-model="budget.alertThreshold"></list-number-input>

                <f7-list-item :title="tt('Auto-create next cycle')">
                    <template #after>
                        <f7-toggle :checked="budget.autoCreateNextCycle" @toggle:change="budget.autoCreateNextCycle = $event"></f7-toggle>
                    </template>
                </f7-list-item>

                <f7-list-item :title="tt('Visible')" v-if="editBudgetId">
                    <template #after>
                        <f7-toggle :checked="budgetVisible" @toggle:change="budgetVisible = $event"></f7-toggle>
                    </template>
                </f7-list-item>
            </template>
        </f7-list>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useBudgetEditPageBase } from '@/views/base/budgets/BudgetEditPageBase.ts';

import { useBudgetsStore } from '@/stores/budget.ts';
import { useUserStore } from '@/stores/user.ts';

import type { LocalizedCurrencyInfo } from '@/core/currency.ts';
import { TRANSACTION_MAX_AMOUNT } from '@/consts/transaction.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const {
    tt,
    getAllCurrencies,
    getCurrencyName,
    formatAmountToLocalizedNumeralsWithCurrency
} = useI18n();

const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();

const {
    editBudgetId,
    clientSessionId,
    loading,
    submitting,
    budget,
    title,
    inputEmptyProblemMessage,
    inputIsEmpty,
    allPeriodTypes,
    allRolloverTypes,
    prepareNewBudget,
    setBudget
} = useBudgetEditPageBase();

const budgetsStore = useBudgetsStore();
const userStore = useUserStore();

const loadingError = ref<unknown | null>(null);
const showCurrencyPopup = ref<boolean>(false);
const showAmountSheet = ref<boolean>(false);
const showPeriodTypeSheet = ref<boolean>(false);
const showRolloverTypeSheet = ref<boolean>(false);

const allCurrencies = computed<LocalizedCurrencyInfo[]>(() => getAllCurrencies());

const budgetVisible = computed<boolean>({
    get: () => !budget.value.hidden,
    set: (value: boolean) => {
        budget.value.hidden = !value;
    }
});

function formatAmount(amount: number, currency: string): string {
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), currency);
}

function init(): void {
    const query = props.f7route.query;
    clientSessionId.value = generateRandomUUID();

    if (query['id']) {
        loading.value = true;

        editBudgetId.value = query['id'];

        budgetsStore.getBudget({
            budgetId: editBudgetId.value
        }).then(response => {
            setBudget(response);
            loading.value = false;
        }).catch(error => {
            if (error.processed) {
                loading.value = false;
            } else {
                loadingError.value = error;
                showToast(error.message || error);
            }
        });
    } else {
        // always a fresh budget (startTime = now, endTime = +30 days), never a stale one from a previous open
        prepareNewBudget(userStore.currentUserDefaultCurrency);
        loading.value = false;
    }
}

function save(): void {
    const router = props.f7router;
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        showAlert(problemMessage);
        return;
    }

    submitting.value = true;
    showLoading(() => submitting.value);

    budgetsStore.saveBudget({
        budget: editBudgetId.value ? budget.value.toModifyRequest() : budget.value.toCreateRequest(clientSessionId.value),
        isEdit: !!editBudgetId.value
    }).then(() => {
        submitting.value = false;
        hideLoading();

        if (!editBudgetId.value) {
            showToast('You have added a new budget');
        } else {
            showToast('You have saved this budget');
        }

        router.back();
    }).catch(error => {
        submitting.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
