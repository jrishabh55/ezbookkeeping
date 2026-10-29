<template>
    <v-dialog width="480" v-model="showState">
        <one-column-dialog-layout :disabled="submitting" :loading="false"
                                  :title="tt(title)" :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content>
                <v-form class="mt-2">
                    <v-row>
                        <v-col cols="12" md="6">
                            <number-input :disabled="submitting" persistent-placeholder
                                          :label="tt('Day of Month')" :placeholder="tt('Day of Month')"
                                          :min-value="1" :max-value="28" :max-decimal-count="0"
                                          :model-value="recurring.dayOfMonth"
                                          @update:model-value="setDayOfMonth" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <date-select :disabled="submitting"
                                         :label="tt('First Invoice Date')"
                                         v-model="recurring.nextDate" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-combobox persistent-placeholder :disabled="submitting"
                                        :label="tt('Repeat Every')"
                                        :items="allPeriods" item-title="displayName" item-value="months" :return-object="false"
                                        :model-value="recurring.periodMonths"
                                        @update:model-value="recurring.periodMonths = Number($event) || 0" />
                        </v-col>
                        <v-col cols="12" md="6">
                            <v-select persistent-placeholder :disabled="submitting"
                                      :label="tt('When Due')"
                                      :items="allModes" item-title="displayName" item-value="mode"
                                      v-model="recurring.mode" />
                        </v-col>
                        <v-col cols="12" md="12">
                            <span class="text-body-medium text-medium-emphasis">{{ tt(modeHelpText) }}</span>
                        </v-col>
                    </v-row>
                </v-form>
            </template>

            <template #footer>
                <v-spacer/>
                <v-tooltip :disabled="!inputIsEmpty" :text="inputEmptyProblemMessage ? tt(inputEmptyProblemMessage) : ''">
                    <template v-slot:activator="{ props }">
                        <div v-bind="props" class="d-inline-block">
                            <v-btn :disabled="inputIsEmpty || submitting" @click="save">
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
import { ref, computed, useTemplateRef } from 'vue';

import SnackBar from '@/components/desktop/SnackBar.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useRecurringInvoiceEditPageBase } from '@/views/base/invoices/RecurringInvoiceEditPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { Invoice, RecurringInvoice } from '@/models/invoice.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

interface RecurringInvoiceEditResponse {
    message: string;
    recurring: RecurringInvoice;
}

const { tt } = useI18n();
const {
    submitting,
    editRecurringId,
    recurring,
    title,
    saveButtonTitle,
    allModes,
    allPeriods,
    inputEmptyProblemMessage,
    inputIsEmpty,
    initFromInvoice,
    initFromRecurring,
    setDayOfMonth
} = useRecurringInvoiceEditPageBase();

const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

let resolveFunc: ((value: RecurringInvoiceEditResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);

const modeHelpText = computed(() => recurring.value.mode === 'draft_ahead' ? 'Draft ahead: the invoice is created this many days early (set in Invoice Profile) and waits for you to send it.' : recurring.value.mode === 'auto_send' ? 'Send automatically: the invoice is emailed to the customer on the date.' : 'Create only: the invoice is created on the date and not sent.');

function open(options: { invoice?: Invoice, recurring?: RecurringInvoice }): Promise<RecurringInvoiceEditResponse> {
    showState.value = true;
    submitting.value = false;

    if (options.recurring) {
        initFromRecurring(options.recurring);
    } else if (options.invoice) {
        initFromInvoice(options.invoice);
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

    const isEdit = !!editRecurringId.value;
    submitting.value = true;

    const saving = isEdit ? invoicesStore.modifyRecurringInvoice({ recurring: recurring.value }) : invoicesStore.addRecurringInvoice({ recurring: recurring.value });

    saving.then(saved => {
        submitting.value = false;
        resolveFunc?.({ message: isEdit ? 'You have saved this recurring invoice' : 'You have added a recurring invoice', recurring: saved });
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
