<template>
    <v-dialog width="480" v-model="showState">
        <one-column-dialog-layout :disabled="submitting" :loading="false"
                                  :title="tt('Repeat Monthly')" :cancel-button-title="tt('Cancel')"
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
                        <v-col cols="12" md="12">
                            <span class="text-body-medium text-medium-emphasis">{{ tt('A new unpaid invoice is created on this day every month, starting from the first date.') }}</span>
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
                                {{ tt('Add') }}
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
import { ref, useTemplateRef } from 'vue';

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
    recurring,
    inputEmptyProblemMessage,
    inputIsEmpty,
    initFromInvoice,
    setDayOfMonth
} = useRecurringInvoiceEditPageBase();

const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

let resolveFunc: ((value: RecurringInvoiceEditResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);

function open(options: { invoice: Invoice }): Promise<RecurringInvoiceEditResponse> {
    showState.value = true;
    submitting.value = false;
    initFromInvoice(options.invoice);

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

    invoicesStore.addRecurringInvoice({ recurring: recurring.value }).then(saved => {
        submitting.value = false;
        resolveFunc?.({ message: 'You have added a recurring invoice', recurring: saved });
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
