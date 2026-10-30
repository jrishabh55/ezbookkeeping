<template>
    <v-dialog width="1200" v-model="showState">
        <one-column-dialog-layout :disabled="loading || updating" :loading="loading"
                                  :title="invoice ? invoice.number : tt('Invoice')" :cancel-button-title="tt('Close')"
                                  @cancel="close">
            <template #after-title>
                <v-chip class="ms-3" size="small" variant="tonal" :color="ESTIMATE_STATUS_COLORS[estimateStatus ?? 'draft']"
                        v-if="invoice && estimateStatus">{{ tt(estimateStatusName) }}</v-chip>
                <v-chip class="ms-3" size="small" variant="tonal" :color="displayStatus ? statusColors[displayStatus] : 'default'"
                        v-else-if="invoice">{{ tt(displayStatusName) }}</v-chip>
                <span class="text-caption text-medium-emphasis ms-2" v-if="sentOnText">{{ tt('Sent on') }} {{ sentOnText }}</span>
                <a class="text-caption ms-2 cursor-pointer" v-if="invoice?.estimateId"
                   @click="openOther(invoice.estimateId)">{{ tt('format.misc.fromEstimate', { number: invoice.estimateNumber }) }}</a>
            </template>

            <template #toolbar>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Edit')" :disabled="!invoice || loading || updating || !canEdit" @click="edit">
                    <v-icon :icon="mdiPencilOutline" />
                    <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Regenerate with Current Customer Details')" :disabled="!invoice || loading || updating" @click="regenerate"
                       v-if="!isEstimate">
                    <v-icon :icon="mdiRefresh" />
                    <v-tooltip activator="parent">{{ tt('Regenerate with Current Customer Details') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Duplicate')" :disabled="!invoice || loading || updating" @click="duplicate">
                    <v-icon :icon="mdiContentCopy" />
                    <v-tooltip activator="parent">{{ tt('Duplicate') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="invoice?.closedManually ? tt('Reopen') : tt('Mark as Paid')"
                       :disabled="!invoice || loading || updating" @click="toggleClosed"
                       v-if="!isEstimate">
                    <v-icon :icon="invoice?.closedManually ? mdiLockOpenVariantOutline : mdiCheckDecagramOutline" />
                    <v-tooltip activator="parent">{{ invoice?.closedManually ? tt('Reopen') : tt('Mark as Paid') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Delete')" :disabled="!invoice || loading || updating" @click="remove">
                    <v-icon :icon="mdiDeleteOutline" />
                    <v-tooltip activator="parent">{{ tt('Delete') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Print')" :disabled="!invoice || !profile || loading" @click="printInvoice">
                    <v-icon :icon="mdiPrinterOutline" />
                    <v-tooltip activator="parent">{{ tt('Print') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Repeat')" :disabled="!invoice || loading || updating" @click="repeatMonthly"
                       v-if="!isEstimate">
                    <v-icon :icon="mdiRepeat" />
                    <v-tooltip activator="parent">{{ tt('Repeat') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Share Link')" :disabled="!invoice || loading || updating" @click="share">
                    <v-icon :icon="mdiShareVariantOutline" />
                    <v-tooltip activator="parent">{{ tt('Share Link') }}</v-tooltip>
                </v-btn>
                <v-btn density="comfortable" color="primary" variant="tonal" class="ms-2"
                       :prepend-icon="mdiDownloadOutline" :loading="downloadingPdf" :disabled="!invoice || loading"
                       @click="downloadPdf">{{ tt('Download PDF') }}</v-btn>
                <v-btn density="comfortable" color="default" variant="text" class="ms-1" :icon="true"
                       :aria-label="tt('Send')" :disabled="!invoice || !profile || loading || updating" @click="openSend">
                    <v-icon :icon="mdiEmailOutline" />
                    <v-tooltip activator="parent">{{ tt('Send') }}</v-tooltip>
                </v-btn>
            </template>

            <template #content>
                <v-row v-if="invoice && profile">
                    <v-col cols="12" lg="8" class="overflow-x-auto">
                        <invoice-document :invoice="invoice" :profile="profile" />
                    </v-col>

                    <v-col cols="12" lg="4">
                        <template v-if="isEstimate">
                            <v-list density="compact">
                                <v-list-item :title="tt('Total')">
                                    <template #append><span class="font-weight-bold">{{ formatAmount(invoice.total) }}</span></template>
                                </v-list-item>
                                <v-list-item :title="tt('Valid Until')">
                                    <template #append><span>{{ formatDate(invoice.dueDate) }}</span></template>
                                </v-list-item>
                            </v-list>

                            <v-divider class="my-2" />
                            <div class="text-title-small px-4 py-2">{{ tt('Status') }}</div>
                            <div class="text-body-medium px-4 pb-2">{{ estimateResponseText || tt('No response yet') }}</div>
                            <div class="text-body-medium px-4 pb-2" style="white-space: pre-line" v-if="invoice.responseNote">{{ tt("Client's note") }}: {{ invoice.responseNote }}</div>
                            <div class="px-4 pb-2" v-if="canMarkResponse || canReopenEstimate">
                                <v-btn class="me-2" color="success" variant="tonal" size="small" :disabled="updating"
                                       v-if="canMarkResponse" @click="markResponse('accepted')">{{ tt('Mark Accepted') }}</v-btn>
                                <v-btn class="me-2" color="error" variant="tonal" size="small" :disabled="updating"
                                       v-if="canMarkResponse" @click="markResponse('declined')">{{ tt('Mark Declined') }}</v-btn>
                                <v-btn color="default" variant="tonal" size="small" :disabled="updating"
                                       v-if="canReopenEstimate" @click="reopen">{{ tt('Reopen') }}</v-btn>
                            </div>

                            <v-divider class="my-2" />
                            <template v-if="invoice.milestones.length">
                                <div class="text-title-small px-4 py-2">{{ tt('Milestones') }}</div>
                                <v-list density="compact">
                                    <v-list-item :key="milestone.id" v-for="(milestone, index) in invoice.milestones"
                                                 :title="milestone.label"
                                                 :subtitle="`${formatMilestonePercent(milestone.percent)} · ${formatAmount(milestoneAmounts[index] ?? 0)}`">
                                        <template #append>
                                            <a class="cursor-pointer text-body-medium" v-if="milestone.invoiceId"
                                               @click="openOther(milestone.invoiceId)">{{ tt('format.misc.invoicedAs', { number: milestone.invoiceNumber }) }}</a>
                                            <v-btn color="primary" variant="tonal" size="small" :disabled="updating || !canConvert"
                                                   v-else @click="convert(milestone.id)">{{ tt('Create Invoice') }}</v-btn>
                                        </template>
                                    </v-list-item>
                                </v-list>
                            </template>
                            <template v-else>
                                <div class="text-title-small px-4 py-2">{{ tt('Invoices') }}</div>
                                <v-list density="compact" v-if="invoice.convertedInvoices.length">
                                    <v-list-item :key="converted.id" v-for="converted in invoice.convertedInvoices" link
                                                 :title="tt('format.misc.invoicedAs', { number: converted.number })"
                                                 @click="openOther(converted.id)"></v-list-item>
                                </v-list>
                                <div class="px-4 pb-2" v-else>
                                    <v-btn color="primary" variant="tonal" :disabled="updating || !canConvert" @click="convert('')">{{ tt('Convert to Invoice') }}</v-btn>
                                </div>
                            </template>
                        </template>
                        <template v-else>
                            <v-list density="compact">
                                <v-list-item :title="tt('Total')">
                                    <template #append><span>{{ formatAmount(invoice.total) }}</span></template>
                                </v-list-item>
                                <v-list-item :title="tt('Received')">
                                    <template #append><span>{{ formatAmount(invoice.received) }}</span></template>
                                </v-list-item>
                                <v-list-item :title="tt(shortfallName)" v-if="shortfall > 0">
                                    <template #append><span>{{ formatAmount(shortfall) }}</span></template>
                                </v-list-item>
                                <v-list-item :title="tt('Balance Due')" v-else>
                                    <template #append><span class="font-weight-bold">{{ formatAmount(balanceDue) }}</span></template>
                                </v-list-item>
                            </v-list>

                            <v-divider class="my-2" />
                            <div class="text-title-small px-4 py-2">{{ tt('Payments') }}</div>
                            <v-list density="compact" v-if="invoice.payments.length">
                                <v-list-item :key="payment.transactionId" v-for="payment in invoice.payments"
                                             :title="formatPaymentAmount(payment)" :subtitle="formatPaymentDescription(payment)">
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
                        </template>

                        <v-divider class="my-2" />
                        <div class="text-title-small px-4 py-2">{{ tt('Emails') }}</div>
                        <v-list density="compact" v-if="invoice.emails.length">
                            <v-list-item :key="email.id" v-for="email in invoice.emails" link
                                         :title="email.subject" :subtitle="`${email.to} · ${formatEmailSentTime(email)}`"
                                         :disabled="loadingEmail" @click="openEmail(email)">
                                <template #append><v-icon :icon="mdiEmailOutline" /></template>
                            </v-list-item>
                        </v-list>
                        <div class="text-body-medium px-4 pb-2" v-else>{{ tt('No emails sent yet.') }}</div>

                        <template v-if="!isEstimate && displayStatus !== 'paid'">
                            <v-divider class="my-2" />
                            <div class="text-title-small px-4 py-2">{{ tt('Suggested Matches') }}</div>
                            <v-list density="compact" v-if="suggestions.length">
                                <v-list-item :key="suggestion.transactionId" v-for="suggestion in suggestions"
                                             :title="formatPaymentAmount(suggestion)" :subtitle="formatPaymentDescription(suggestion)">
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
    <recurring-invoice-edit-dialog ref="recurringInvoiceEditDialog" />
    <confirm-dialog ref="confirmDialog"/>
    <amount-input-dialog ref="amountInputDialog" />
    <snack-bar ref="snackbar" />

    <v-dialog width="800" v-model="showEmailDialog">
        <v-card class="dialog-layout-card" v-if="viewedEmail">
            <v-toolbar color="primary">
                <v-toolbar-title>{{ viewedEmail.subject }}</v-toolbar-title>
            </v-toolbar>
            <v-card-text class="pa-4">
                <div class="text-body-medium mb-3">{{ tt('To') }}: {{ viewedEmail.to }} · {{ formatEmailSentTime(viewedEmail) }}</div>
                <!-- exactly what was sent; sandboxed like the send preview -->
                <iframe class="invoice-sent-email-frame" sandbox="" referrerpolicy="no-referrer"
                        :title="viewedEmail.subject" :srcdoc="viewedEmail.emailHtml"></iframe>
            </v-card-text>
            <v-card-actions class="px-4 pb-4">
                <v-spacer />
                <v-btn color="gray" @click="showEmailDialog = false">{{ tt('Close') }}</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <v-dialog width="640" v-model="showShareDialog">
        <v-card class="dialog-layout-card">
            <v-toolbar color="primary">
                <v-toolbar-title>{{ tt('Share Link') }}</v-toolbar-title>
            </v-toolbar>
            <v-card-text class="pa-4">
                <div class="text-body-medium mb-3">{{ tt(isEstimate ? 'Anyone with this link can view this estimate and accept or decline it.' : 'Anyone with this link can view this invoice.') }}</div>
                <v-text-field readonly density="comfortable" :model-value="publicLinkUrl" @focus="selectLinkField" />
            </v-card-text>
            <v-card-actions class="px-4 pb-4">
                <v-btn color="error" variant="text" :disabled="updating" @click="updatePublicLink(false)">{{ tt('Disable Link') }}</v-btn>
                <v-btn color="default" variant="text" :disabled="updating" @click="updatePublicLink(true)">{{ tt('Reset Link') }}</v-btn>
                <v-spacer />
                <v-btn color="gray" @click="showShareDialog = false">{{ tt('Close') }}</v-btn>
                <v-btn color="primary" @click="copyLink">{{ tt('Copy') }}</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <v-dialog width="1200" :persistent="sending" v-model="showSendDialog">
        <v-card class="dialog-layout-card">
            <v-toolbar color="primary">
                <v-toolbar-title>{{ tt(isEstimate ? 'Send Estimate' : 'Send Invoice') }}</v-toolbar-title>
            </v-toolbar>
            <v-card-text class="pa-4">
                <v-text-field type="email" autocomplete="off" persistent-placeholder class="mb-2"
                              :disabled="sending"
                              :label="tt('To')" :placeholder="tt('Email')"
                              v-model="sendRequest.to" />
                <template v-if="renewalOption">
                    <v-alert class="mb-2" type="warning" variant="tonal" density="compact" v-if="renewalOption.mixedPeriods">
                        {{ tt('These products renew on different periods. Split them into separate invoices to track their renewals.') }}
                    </v-alert>
                    <v-checkbox class="mb-2" density="compact" hide-details :disabled="sending"
                                :label="renewalOptionText" v-model="sendRequest.createRenewal" v-else />
                </template>
                <invoice-email-template-form v-if="showSendDialog" :key="sendRequest.id"
                                             v-model:subject="sendRequest.subject" v-model:body="sendRequest.bodyHtml"
                                             :invoice-id="sendRequest.id" :disabled="sending" split
                                             @validity-change="emailTemplateValid = $event" />
            </v-card-text>
            <v-card-actions class="px-4 pb-4">
                <v-spacer />
                <v-btn color="gray" :disabled="sending" @click="showSendDialog = false">{{ tt('Cancel') }}</v-btn>
                <v-btn color="primary" :disabled="sending || !sendRequest.to || !emailTemplateValid" @click="send">
                    {{ tt('Send') }}
                    <v-progress-circular indeterminate size="22" class="ms-2" v-if="sending"></v-progress-circular>
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref, watch, useTemplateRef } from 'vue';

import {
    mdiPencilOutline,
    mdiRefresh,
    mdiContentCopy,
    mdiCheckDecagramOutline,
    mdiLockOpenVariantOutline,
    mdiDeleteOutline,
    mdiPrinterOutline,
    mdiDownloadOutline,
    mdiLinkOff,
    mdiRepeat,
    mdiShareVariantOutline,
    mdiEmailOutline
} from '@mdi/js';

import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import AmountInputDialog from '@/components/desktop/AmountInputDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import InvoiceDocument from '@/components/common/InvoiceDocument.vue';
import InvoiceEmailTemplateForm from '@/components/common/InvoiceEmailTemplateForm.vue';
import InvoiceEditDialog from './InvoiceEditDialog.vue';
import RecurringInvoiceEditDialog from './RecurringInvoiceEditDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceDetailPageBase } from '@/views/base/invoices/InvoiceDetailPageBase.ts';
import { ESTIMATE_STATUS_COLORS } from '@/views/base/invoices/EstimateListPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';
import { useAccountsStore } from '@/stores/account.ts';

import type { InvoiceEmail, InvoicePayment, InvoiceSendForm } from '@/models/invoice.ts';

import type { InvoiceDisplayStatus } from '@/lib/invoice.ts';
import { formatMilestonePercent } from '@/lib/estimate.ts';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type AmountInputDialogType = InstanceType<typeof AmountInputDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type InvoiceEditDialogType = InstanceType<typeof InvoiceEditDialog>;
type RecurringInvoiceEditDialogType = InstanceType<typeof RecurringInvoiceEditDialog>;

interface InvoiceDetailResponse {
    changed: boolean;
}

const { tt } = useI18n();
const {
    loading,
    updating,
    downloadingPdf,
    invoice,
    profile,
    suggestions,
    displayStatus,
    displayStatusName,
    balanceDue,
    shortfall,
    shortfallName,
    publicLinkUrl,
    sentOnText,
    renewalOption,
    renewalOptionText,
    viewedEmail,
    loadingEmail,
    formatAmount,
    formatDate,
    isEstimate,
    estimateStatus,
    estimateStatusName,
    estimateResponseText,
    canEdit,
    canMarkResponse,
    canReopenEstimate,
    canConvert,
    convertNeedsConfirm,
    milestoneAmounts,
    setEstimateResponse,
    reopenEstimate,
    needsSettledAmount,
    formatPaymentAmount,
    formatPaymentDescription,
    formatEmailSentTime,
    viewEmail,
    printInvoice,
    setPublicLink,
    copyPublicLink,
    downloadInvoicePdf,
    prepareSendRequest,
    sendInvoice
} = useInvoiceDetailPageBase();

function downloadPdf(): void {
    downloadInvoicePdf().catch(error => snackbar.value?.showError(error));
}

function openEmail(email: InvoiceEmail): void {
    viewEmail(email).then(() => {
        showEmailDialog.value = true;
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

const invoicesStore = useInvoicesStore();
const accountsStore = useAccountsStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const amountInputDialog = useTemplateRef<AmountInputDialogType>('amountInputDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const invoiceEditDialog = useTemplateRef<InvoiceEditDialogType>('invoiceEditDialog');
const recurringInvoiceEditDialog = useTemplateRef<RecurringInvoiceEditDialogType>('recurringInvoiceEditDialog');

const statusColors: Record<InvoiceDisplayStatus, string> = { unpaid: 'warning', partial: 'primary', paid: 'success', overdue: 'error' };

let resolveFunc: ((value: InvoiceDetailResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const invoiceId = ref<string>('');
const changed = ref<boolean>(false);
const showShareDialog = ref<boolean>(false);
const showSendDialog = ref<boolean>(false);
const showEmailDialog = ref<boolean>(false);
const sending = ref<boolean>(false);
const emailTemplateValid = ref<boolean>(false);
const sendRequest = ref<InvoiceSendForm & { bodyHtml: string }>({ id: '', to: '', subject: '', bodyHtml: '', message: '' });

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
        invoicesStore.loadInvoiceProfile({ force: true }),
        accountsStore.loadAllAccounts({ force: false })
    ]).then(([loadedInvoice, loadedProfile]) => {
        invoice.value = loadedInvoice;
        profile.value = loadedProfile;
        loading.value = false;

        if (displayStatus.value === 'paid' || loadedInvoice.kind === 'estimate') {
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

// a suggestion in another currency first asks what it settled in the invoice currency; same-currency links stay one click
function link(suggestion: InvoicePayment): void {
    if (!needsSettledAmount(suggestion)) {
        runAndReload(() => invoicesStore.linkInvoicePayment({ invoiceId: invoiceId.value, transactionId: suggestion.transactionId }));
        return;
    }

    amountInputDialog.value?.open({
        title: 'Settled Amount',
        text: 'Please enter the amount this payment settles in the invoice currency',
        inputLabel: 'Settled Amount',
        inputPlaceholder: 'Settled Amount',
        currency: invoice.value?.currency,
        initAmount: balanceDue.value
    }).then(settledAmount => {
        if (settledAmount > 0) {
            runAndReload(() => invoicesStore.linkInvoicePayment({ invoiceId: invoiceId.value, transactionId: suggestion.transactionId, settledAmount }));
        }
    }).catch(() => {
        // cancelled
    });
}

function unlink(payment: InvoicePayment): void {
    runAndReload(() => invoicesStore.unlinkInvoicePayment({ invoiceId: invoiceId.value, transactionId: payment.transactionId }));
}

// saving the invoice unchanged makes the server copy the customer's current details again and recompute the totals
function regenerate(): void {
    const current = invoice.value;

    if (!current) {
        return;
    }

    confirmDialog.value?.open("Regenerate this invoice with the customer's current details? The PDF and public link will show them too.").then(() => {
        runAndReload(() => invoicesStore.saveInvoice({ invoice: current, isEdit: true }));
    });
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
    confirmDialog.value?.open(isEstimate.value ? 'Are you sure you want to delete this estimate?' : 'Are you sure you want to delete this invoice?').then(() => {
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

function repeatMonthly(): void {
    if (!invoice.value) {
        return;
    }

    recurringInvoiceEditDialog.value?.open({ invoice: invoice.value }).then(result => {
        snackbar.value?.showMessage(result.message);
        changed.value = true; // the list page reloads its recurring list on close
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

// first share creates the link; afterwards the dialog just shows it
function share(): void {
    if (!invoice.value) {
        return;
    }

    if (invoice.value.publicToken) {
        showShareDialog.value = true;
    } else {
        updatePublicLink(true);
    }
}

function updatePublicLink(enabled: boolean): void {
    updating.value = true;

    setPublicLink(enabled).then(() => {
        updating.value = false;
        changed.value = true;
        showShareDialog.value = enabled;

        if (!enabled) {
            snackbar.value?.showMessage('Public link has been disabled');
        }
    }).catch(error => {
        updating.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function copyLink(): void {
    copyPublicLink().then(copied => {
        if (copied) {
            snackbar.value?.showMessage('Link copied');
        }
    });
}

// no clipboard (plain http): select the URL so it can be copied by hand
function selectLinkField(event: FocusEvent): void {
    (event.target as HTMLInputElement | null)?.select();
}

// creates the public link when the invoice has none, then opens the prefilled email form
function openSend(): void {
    if (!invoice.value) {
        return;
    }

    updating.value = true;

    prepareSendRequest().then(request => {
        updating.value = false;
        changed.value = true; // the link may have been created
        emailTemplateValid.value = false;
        sendRequest.value = { ...request, bodyHtml: request.bodyHtml ?? '' };
        showSendDialog.value = true;
    }).catch(error => {
        updating.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function send(): void {
    if (sending.value || !sendRequest.value.to || !emailTemplateValid.value) {
        return;
    }

    sending.value = true;

    sendInvoice(sendRequest.value).then(() => {
        sending.value = false;
        showSendDialog.value = false;
        changed.value = true;
        snackbar.value?.showMessage(isEstimate.value ? 'Estimate has been sent' : 'Invoice has been sent');
    }).catch(error => {
        sending.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

// opens another document in this dialog: the estimate an invoice came from, or an invoice made from this estimate
function openOther(id: string): void {
    invoiceId.value = id;
    invoice.value = null;
    suggestions.value = [];
    load();
}

function markResponse(status: 'accepted' | 'declined'): void {
    runAndReload(() => setEstimateResponse(status));
}

function reopen(): void {
    runAndReload(() => reopenEstimate());
}

// opens the editor with the invoice the estimate (milestoneId '') or a milestone turns into; saving it opens the new invoice here
function convert(milestoneId: string): void {
    const current = invoice.value;

    if (!current) {
        return;
    }

    const start = (): void => {
        invoiceEditDialog.value?.open({ convertFrom: { estimateId: current.id, milestoneId } }).then(result => {
            snackbar.value?.showMessage(result.message);
            changed.value = true;
            openOther(result.invoice.id);
        }).catch(error => {
            if (error && !error.processed) {
                snackbar.value?.showError(error);
            }
        });
    };

    if (convertNeedsConfirm.value) {
        confirmDialog.value?.open("This estimate isn't accepted yet. Convert anyway?").then(start);
    } else {
        start();
    }
}

function close(): void {
    showState.value = false;
}

defineExpose({
    open
});
</script>

<style>
.invoice-sent-email-frame {
    display: block;
    width: 100%;
    height: 60vh;
    border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
    border-radius: 4px;
    background: white;
}
</style>
