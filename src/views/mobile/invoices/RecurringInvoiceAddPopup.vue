<template>
    <f7-popup push swipe-to-close :opened="show" @popup:open="onPopupOpen" @popup:closed="onPopupClosed">
        <f7-page>
            <f7-navbar>
                <f7-nav-left>
                    <f7-link popup-close icon-f7="xmark" :aria-label="tt('Cancel')"></f7-link>
                </f7-nav-left>
                <f7-nav-title :title="tt(title)"></f7-nav-title>
                <f7-nav-right>
                    <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt(saveButtonTitle)" @click="save"></f7-link>
                </f7-nav-right>
            </f7-navbar>

            <f7-list form strong inset dividers class="margin-vertical">
                <!-- list-number-input must go in #list (f7-list cannot scan it in the default slot), so every item goes there to keep the order -->
                <template #list>
                    <list-number-input :label="tt('Day of Month')" :placeholder="tt('Day of Month')"
                                       :min-value="1" :max-value="28" :max-decimal-count="0"
                                       :model-value="recurringForm.dayOfMonth"
                                       @update:model-value="setDayOfMonth"></list-number-input>
                    <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                                  :header="tt('First Invoice Date')"
                                  :title="formatDate(recurringForm.nextDate)"
                                  @click="showNextDateSheet = true">
                        <date-selection-sheet v-model:show="showNextDateSheet"
                                              :model-value="recurringForm.nextDate"
                                              @update:model-value="onNextDateChanged">
                        </date-selection-sheet>
                    </f7-list-item>
                    <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                                  :header="tt('Repeat Every')" :title="selectedPeriodName"
                                  @click="showPeriodSheet = true">
                        <list-item-selection-sheet value-type="item"
                                                   key-field="months" value-field="months" title-field="displayName"
                                                   :items="allPeriods"
                                                   v-model:show="showPeriodSheet"
                                                   v-model="recurringForm.periodMonths">
                        </list-item-selection-sheet>
                    </f7-list-item>
                    <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                                  :header="tt('When Due')" :title="selectedModeName"
                                  @click="showModeSheet = true">
                        <list-item-selection-sheet value-type="item"
                                                   key-field="mode" value-field="mode" title-field="displayName"
                                                   :items="allModes"
                                                   v-model:show="showModeSheet"
                                                   v-model="recurringForm.mode">
                        </list-item-selection-sheet>
                    </f7-list-item>
                </template>
            </f7-list>

            <f7-block class="margin-vertical">
                <small>{{ tt(modeHelpText) }}</small>
            </f7-block>
        </f7-page>
    </f7-popup>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useRecurringInvoiceEditPageBase } from '@/views/base/invoices/RecurringInvoiceEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Invoice, RecurringInvoice } from '@/models/invoice.ts';

const props = defineProps<{
    invoice: Invoice | null;
    recurring?: RecurringInvoice | null;
    show: boolean;
}>();

const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'added', value: RecurringInvoice): void;
}>();

const { tt } = useI18n();
const { showToast } = useI18nUIComponents();
const {
    submitting,
    editRecurringId,
    recurring: recurringForm,
    title,
    saveButtonTitle,
    allModes,
    allPeriods,
    selectedModeName,
    selectedPeriodName,
    inputEmptyProblemMessage,
    inputIsEmpty,
    initFromInvoice,
    initFromRecurring,
    setDayOfMonth,
    formatDate
} = useRecurringInvoiceEditPageBase();

const invoicesStore = useInvoicesStore();

const showNextDateSheet = ref<boolean>(false);
const showPeriodSheet = ref<boolean>(false);
const showModeSheet = ref<boolean>(false);

const modeHelpText = computed(() => recurringForm.value.mode === 'draft_ahead' ? 'Draft ahead: the invoice is created this many days early (set in Invoice Profile) and waits for you to send it.' : recurringForm.value.mode === 'auto_send' ? 'Send automatically: the invoice is emailed to the customer on the date.' : 'Create only: the invoice is created on the date and not sent.');

function onNextDateChanged(date: TextualYearMonthDay | ''): void {
    if (date) {
        recurringForm.value.nextDate = date;
    }
}

function save(): void {
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        showToast(problemMessage);
        return;
    }

    const isEdit = !!editRecurringId.value;
    submitting.value = true;
    showLoading(() => submitting.value);

    const saving = isEdit ? invoicesStore.modifyRecurringInvoice({ recurring: recurringForm.value }) : invoicesStore.addRecurringInvoice({ recurring: recurringForm.value });

    saving.then(saved => {
        submitting.value = false;
        hideLoading();
        showToast(isEdit ? 'You have saved this recurring invoice' : 'You have added a recurring invoice');
        emit('added', saved);
        emit('update:show', false);
    }).catch(error => {
        submitting.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPopupOpen(): void {
    submitting.value = false;

    if (props.recurring) {
        initFromRecurring(props.recurring);
    } else if (props.invoice) {
        initFromInvoice(props.invoice);
    }
}

function onPopupClosed(): void {
    emit('update:show', false);
}
</script>
