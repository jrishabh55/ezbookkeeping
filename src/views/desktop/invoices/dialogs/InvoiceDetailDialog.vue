<template>
    <v-dialog width="1200" v-model="showState">
        <one-column-dialog-layout :disabled="loading || updating" :loading="loading"
                                  :title="invoice ? invoice.number : tt('Invoice')" :cancel-button-title="tt('Close')"
                                  @cancel="close">
            <template #after-title>
                <v-chip class="ms-3" size="small" variant="tonal" :color="displayStatus ? statusColors[displayStatus] : 'default'"
                        v-if="invoice">{{ tt(displayStatusName) }}</v-chip>
            </template>

            <template #toolbar>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Edit')" :disabled="!invoice || loading || updating" @click="edit">
                    <v-icon :icon="mdiPencilOutline" />
                    <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Duplicate')" :disabled="!invoice || loading || updating" @click="duplicate">
                    <v-icon :icon="mdiContentCopy" />
                    <v-tooltip activator="parent">{{ tt('Duplicate') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="invoice?.closedManually ? tt('Reopen') : tt('Mark as Paid')"
                       :disabled="!invoice || loading || updating" @click="toggleClosed">
                    <v-icon :icon="invoice?.closedManually ? mdiLockOpenVariantOutline : mdiCheckDecagramOutline" />
                    <v-tooltip activator="parent">{{ invoice?.closedManually ? tt('Reopen') : tt('Mark as Paid') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Delete')" :disabled="!invoice || loading || updating" @click="remove">
                    <v-icon :icon="mdiDeleteOutline" />
                    <v-tooltip activator="parent">{{ tt('Delete') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Print / Save PDF')" :disabled="!invoice || !profile || loading" @click="printInvoice">
                    <v-icon :icon="mdiPrinterOutline" />
                    <v-tooltip activator="parent">{{ tt('Print / Save PDF') }}</v-tooltip>
                </v-btn>
            </template>

            <template #content>
                <v-row v-if="invoice && profile">
                    <v-col cols="12" lg="8" class="overflow-x-auto">
                        <invoice-document :invoice="invoice" :profile="profile" />
                    </v-col>

                    <v-col cols="12" lg="4">
                        <v-list density="compact">
                            <v-list-item :title="tt('Total')">
                                <template #append><span>{{ formatAmount(invoice.total) }}</span></template>
                            </v-list-item>
                            <v-list-item :title="tt('Received')">
                                <template #append><span>{{ formatAmount(invoice.received) }}</span></template>
                            </v-list-item>
                            <v-list-item :title="tt('Bank Charges')" v-if="bankCharges > 0">
                                <template #append><span>{{ formatAmount(bankCharges) }}</span></template>
                            </v-list-item>
                            <v-list-item :title="tt('Balance Due')" v-else>
                                <template #append><span class="font-weight-bold">{{ formatAmount(balanceDue) }}</span></template>
                            </v-list-item>
                        </v-list>

                        <v-divider class="my-2" />
                        <div class="text-title-small px-4 py-2">{{ tt('Payments') }}</div>
                        <v-list density="compact" v-if="invoice.payments.length">
                            <v-list-item :key="payment.transactionId" v-for="payment in invoice.payments"
                                         :title="formatAmount(payment.amount)" :subtitle="formatPaymentDescription(payment)">
                                <template #append>
                                    <v-btn density="comfortable" color="default" variant="text" :icon="true"
                                           :aria-label="tt('Unlink')" :disabled="updating" @click="unlink(payment)">
                                        <v-icon :icon="mdiLinkOff" />
                                        <v-tooltip activator="parent">{{ tt('Unlink') }}</v-tooltip>
                                    </v-btn>
                                </template>
                            </v-list-item>
                        </v-list>
                        <div class="text-body-medium px-4 pb-2" v-else>{{ tt('No payments linked yet.') }}</div>

                        <template v-if="displayStatus !== 'paid'">
                            <v-divider class="my-2" />
                            <div class="text-title-small px-4 py-2">{{ tt('Suggested Matches') }}</div>
                            <v-list density="compact" v-if="suggestions.length">
                                <v-list-item :key="suggestion.transactionId" v-for="suggestion in suggestions"
                                             :title="formatAmount(suggestion.amount)" :subtitle="formatPaymentDescription(suggestion)">
                                    <template #append>
                                        <v-btn color="primary" variant="tonal" size="small" :disabled="updating" @click="link(suggestion)">{{ tt('Link') }}</v-btn>
                                    </template>
                                </v-list-item>
                            </v-list>
                            <div class="text-body-medium px-4 pb-2" v-else>{{ tt('No matching income transactions found yet.') }}</div>
                        </template>
                    </v-col>
                </v-row>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <invoice-edit-dialog ref="invoiceEditDialog" />
    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, watch, useTemplateRef } from 'vue';

import {
    mdiPencilOutline,
    mdiContentCopy,
    mdiCheckDecagramOutline,
    mdiLockOpenVariantOutline,
    mdiDeleteOutline,
    mdiPrinterOutline,
    mdiLinkOff
} from '@mdi/js';

import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import InvoiceDocument from '@/components/common/InvoiceDocument.vue';
import InvoiceEditDialog from './InvoiceEditDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceDetailPageBase } from '@/views/base/invoices/InvoiceDetailPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';
import { useAccountsStore } from '@/stores/account.ts';

import type { InvoicePayment } from '@/models/invoice.ts';

import type { InvoiceDisplayStatus } from '@/lib/invoice.ts';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type InvoiceEditDialogType = InstanceType<typeof InvoiceEditDialog>;

interface InvoiceDetailResponse {
    changed: boolean;
}

const { tt } = useI18n();
const {
    loading,
    updating,
    invoice,
    profile,
    suggestions,
    displayStatus,
    displayStatusName,
    balanceDue,
    bankCharges,
    formatAmount,
    formatPaymentDescription,
    printInvoice
} = useInvoiceDetailPageBase();

const invoicesStore = useInvoicesStore();
const accountsStore = useAccountsStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const invoiceEditDialog = useTemplateRef<InvoiceEditDialogType>('invoiceEditDialog');

const statusColors: Record<InvoiceDisplayStatus, string> = { unpaid: 'warning', partial: 'info', paid: 'success', overdue: 'error' };

let resolveFunc: ((value: InvoiceDetailResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const invoiceId = ref<string>('');
const changed = ref<boolean>(false);

// resolves open()'s promise on every dismissal (X/Close, Escape, backdrop click), not just the explicit close() call
watch(showState, value => {
    if (!value) {
        resolveFunc?.({ changed: changed.value });
        resolveFunc = null;
    }
});

function load(): void {
    loading.value = true;

    Promise.all([
        invoicesStore.getInvoice({ invoiceId: invoiceId.value }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        accountsStore.loadAllAccounts({ force: false })
    ]).then(([loadedInvoice, loadedProfile]) => {
        invoice.value = loadedInvoice;
        profile.value = loadedProfile;
        loading.value = false;

        if (displayStatus.value === 'paid') {
            suggestions.value = [];
            return;
        }

        invoicesStore.getInvoicePaymentSuggestions({ invoiceId: loadedInvoice.id }).then(result => {
            suggestions.value = result;
        }).catch(error => {
            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    }).catch(error => {
        loading.value = false;
        showState.value = false;

        if (!error.processed) {
            rejectFunc?.(error);
        }
    });
}

function open(options: { id: string }): Promise<InvoiceDetailResponse> {
    showState.value = true;
    invoiceId.value = options.id;
    invoice.value = null;
    suggestions.value = [];
    changed.value = false;
    load();

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function runAndReload(action: () => Promise<unknown>): void {
    updating.value = true;

    action().then(() => {
        updating.value = false;
        changed.value = true;
        load();
    }).catch(error => {
        updating.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function link(suggestion: InvoicePayment): void {
    runAndReload(() => invoicesStore.linkInvoicePayment({ invoiceId: invoiceId.value, transactionId: suggestion.transactionId }));
}

function unlink(payment: InvoicePayment): void {
    runAndReload(() => invoicesStore.unlinkInvoicePayment({ invoiceId: invoiceId.value, transactionId: payment.transactionId }));
}

function toggleClosed(): void {
    if (invoice.value) {
        const current = invoice.value;
        runAndReload(() => invoicesStore.setInvoiceClosed({ invoice: current, closed: !current.closedManually }));
    }
}

function edit(): void {
    invoiceEditDialog.value?.open({ id: invoiceId.value, currentInvoice: invoice.value ?? undefined }).then(result => {
        snackbar.value?.showMessage(result.message);
        changed.value = true;
        load();
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function duplicate(): void {
    invoiceEditDialog.value?.open({ duplicateFrom: invoice.value ?? undefined }).then(result => {
        snackbar.value?.showMessage(result.message);
        changed.value = true;
        invoiceId.value = result.invoice.id;
        load();
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function remove(): void {
    confirmDialog.value?.open('Are you sure you want to delete this invoice?').then(() => {
        if (!invoice.value) {
            return;
        }

        updating.value = true;

        invoicesStore.deleteInvoice({ invoice: invoice.value }).then(() => {
            updating.value = false;
            changed.value = true;
            close();
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

function close(): void {
    showState.value = false;
}

defineExpose({
    open
});
</script>
