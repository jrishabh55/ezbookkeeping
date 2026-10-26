<template>
    <f7-popup push swipe-to-close :opened="show" @popup:open="onPopupOpen" @popup:closed="onPopupClosed">
        <f7-page>
            <f7-navbar>
                <f7-nav-left>
                    <f7-link popup-close icon-f7="xmark" :aria-label="tt('Cancel')"></f7-link>
                </f7-nav-left>
                <f7-nav-title :title="tt('Repeat Monthly')"></f7-nav-title>
                <f7-nav-right>
                    <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt('Add')" @click="save"></f7-link>
                </f7-nav-right>
            </f7-navbar>

            <f7-list form strong inset dividers class="margin-vertical">
                <!-- list-number-input must go in #list (f7-list cannot scan it in the default slot), so every item goes there to keep the order -->
                <template #list>
                    <list-number-input :label="tt('Day of Month')" :placeholder="tt('Day of Month')"
                                       :min-value="1" :max-value="28" :max-decimal-count="0"
                                       :model-value="recurring.dayOfMonth"
                                       @update:model-value="setDayOfMonth"></list-number-input>
                    <f7-list-item class="list-item-with-header-and-title" link="#" no-chevron
                                  :header="tt('First Invoice Date')"
                                  :title="formatDate(recurring.nextDate)"
                                  @click="showNextDateSheet = true">
                        <date-selection-sheet v-model:show="showNextDateSheet"
                                              :model-value="recurring.nextDate"
                                              @update:model-value="onNextDateChanged">
                        </date-selection-sheet>
                    </f7-list-item>
                </template>
            </f7-list>

            <f7-block class="margin-vertical">
                <small>{{ tt('A new unpaid invoice is created on this day every month, starting from the first date.') }}</small>
            </f7-block>
        </f7-page>
    </f7-popup>
</template>

<script setup lang="ts">
import { ref } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useRecurringInvoiceEditPageBase } from '@/views/base/invoices/RecurringInvoiceEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { TextualYearMonthDay } from '@/core/datetime.ts';
import type { Invoice, RecurringInvoice } from '@/models/invoice.ts';

const props = defineProps<{
    invoice: Invoice | null;
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
    recurring,
    inputEmptyProblemMessage,
    inputIsEmpty,
    initFromInvoice,
    setDayOfMonth,
    formatDate
} = useRecurringInvoiceEditPageBase();

const invoicesStore = useInvoicesStore();

const showNextDateSheet = ref<boolean>(false);

function onNextDateChanged(date: TextualYearMonthDay | ''): void {
    if (date) {
        recurring.value.nextDate = date;
    }
}

function save(): void {
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        showToast(problemMessage);
        return;
    }

    submitting.value = true;
    showLoading(() => submitting.value);

    invoicesStore.addRecurringInvoice({ recurring: recurring.value }).then(saved => {
        submitting.value = false;
        hideLoading();
        showToast('You have added a recurring invoice');
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

    if (props.invoice) {
        initFromInvoice(props.invoice);
    }
}

function onPopupClosed(): void {
    emit('update:show', false);
}
</script>
