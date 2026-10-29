<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="invoice ? invoice.number : tt('Invoice')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading || updating }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-item title="Status" after="Unpaid"></f7-list-item>
            <f7-list-item title="Total" after="₹000,000.00"></f7-list-item>
            <f7-list-item title="Balance Due" after="₹000,000.00"></f7-list-item>
        </f7-list>

        <template v-if="!loading && invoice && profile">
            <f7-block>
                <f7-button fill :disabled="downloadingPdf" @click="downloadPdf">{{ tt('Download PDF') }}</f7-button>
            </f7-block>
            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item :title="tt('Status')" :after="tt(displayStatusName)"></f7-list-item>
                <f7-list-item :title="tt('Sent on')" :after="sentOnText" v-if="sentOnText"></f7-list-item>
                <f7-list-item :title="tt('Customer')" :after="invoice.billToName"></f7-list-item>
                <f7-list-item :title="tt('Payment Due')" :after="formatDate(invoice.dueDate)"></f7-list-item>
                <f7-list-item :title="tt('Total')" :after="formatAmount(invoice.total)"></f7-list-item>
                <f7-list-item :title="tt('Received')" :after="formatAmount(invoice.received)"></f7-list-item>
                <f7-list-item :title="tt(shortfallName)" :after="formatAmount(shortfall)" v-if="shortfall > 0"></f7-list-item>
                <f7-list-item :title="tt('Balance Due')" :after="formatAmount(balanceDue)" v-else></f7-list-item>
            </f7-list>

            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item group-title>
                    <small>{{ tt('Payments') }}</small>
                </f7-list-item>
                <f7-list-item :title="tt('No payments linked yet.')" v-if="!invoice.payments.length"></f7-list-item>
                <f7-list-item swipeout
                              :title="formatPaymentAmount(payment)"
                              :footer="formatPaymentDescription(payment)"
                              :key="payment.transactionId"
                              v-for="payment in invoice.payments">
                    <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                         :right="textDirection === TextDirection.LTR">
                        <f7-swipeout-button color="red" close :text="tt('Unlink')" @click="unlink(payment)"></f7-swipeout-button>
                    </f7-swipeout-actions>
                </f7-list-item>
            </f7-list>

            <f7-list strong inset dividers class="margin-vertical">
                <f7-list-item group-title>
                    <small>{{ tt('Emails') }}</small>
                </f7-list-item>
                <f7-list-item :title="tt('No emails sent yet.')" v-if="!invoice.emails.length"></f7-list-item>
                <f7-list-item link="#" no-chevron
                              :class="{ 'disabled': loadingEmail }"
                              :title="email.subject"
                              :footer="`${email.to} · ${formatEmailSentTime(email)}`"
                              :key="email.id"
                              v-for="email in invoice.emails"
                              @click="openEmail(email)"></f7-list-item>
            </f7-list>

            <f7-list strong inset dividers class="margin-vertical" v-if="displayStatus !== 'paid'">
                <f7-list-item group-title>
                    <small>{{ tt('Suggested Matches') }}</small>
                </f7-list-item>
                <f7-list-item :title="tt('No matching income transactions found yet.')" v-if="!suggestions.length"></f7-list-item>
                <f7-list-item :title="formatPaymentAmount(suggestion)"
                              :footer="formatPaymentDescription(suggestion)"
                              :key="suggestion.transactionId"
                              v-for="suggestion in suggestions">
                    <template #after>
                        <f7-button small fill :class="{ 'disabled': updating }" :text="tt('Link')" @click="link(suggestion)"></f7-button>
                    </template>
                </f7-list-item>
            </f7-list>

            <f7-block class="invoice-document-preview">
                <invoice-document :invoice="invoice" :profile="profile" />
            </f7-block>

            <!-- a suggestion in another currency first asks what it settled in the invoice currency -->
            <number-pad-sheet :min-value="0"
                              :max-value="TRANSACTION_MAX_AMOUNT"
                              :currency="invoice.currency"
                              :hint="tt('Please enter the amount this payment settles in the invoice currency')"
                              v-model:show="showSettledAmountSheet"
                              v-model="settledAmount"
                              @update:model-value="onSettledAmountEntered"
            ></number-pad-sheet>
        </template>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button @click="f7router.navigate(`/invoice/edit?id=${invoiceId}`)">{{ tt('Edit') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate(`/invoice/add?duplicateFrom=${invoiceId}`)">{{ tt('Duplicate') }}</f7-actions-button>
                <f7-actions-button @click="regenerate">{{ tt('Regenerate with Current Customer Details') }}</f7-actions-button>
                <f7-actions-button @click="toggleClosed">{{ invoice?.closedManually ? tt('Reopen') : tt('Mark as Paid') }}</f7-actions-button>
                <f7-actions-button @click="printInvoice">{{ tt('Print') }}</f7-actions-button>
                <f7-actions-button @click="showRepeatMonthlyPopup = true">{{ tt('Repeat') }}</f7-actions-button>
                <f7-actions-button @click="share">{{ tt('Share Link') }}</f7-actions-button>
                <f7-actions-button @click="downloadPdf" :disabled="downloadingPdf">{{ tt('Download PDF') }}</f7-actions-button>
                <f7-actions-button @click="f7router.navigate(`/invoice/send?id=${invoiceId}`)">{{ tt('Send') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button color="red" @click="showDeleteActionSheet = true">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this invoice?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <recurring-invoice-add-popup :invoice="invoice" v-model:show="showRepeatMonthlyPopup" />

        <f7-popup push swipe-to-close :opened="showEmailPopup" @popup:closed="showEmailPopup = false">
            <f7-page>
                <f7-navbar>
                    <f7-nav-left>
                        <f7-link popup-close icon-f7="xmark" :aria-label="tt('Close')"></f7-link>
                    </f7-nav-left>
                    <f7-nav-title :title="viewedEmail?.subject"></f7-nav-title>
                </f7-navbar>
                <template v-if="viewedEmail">
                    <f7-block-title>{{ tt('To') }}: {{ viewedEmail.to }} · {{ formatEmailSentTime(viewedEmail) }}</f7-block-title>
                    <!-- exactly what was sent; sandboxed like the send preview -->
                    <iframe class="invoice-sent-email-frame" sandbox="" referrerpolicy="no-referrer"
                            :title="viewedEmail.subject" :srcdoc="viewedEmail.emailHtml"></iframe>
                </template>
            </f7-page>
        </f7-popup>
        <f7-actions close-by-outside-click close-on-escape :opened="showShareActionSheet" @actions:closed="showShareActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Anyone with this link can view this invoice.') }}</f7-actions-label>
                <f7-actions-label class="invoice-public-link">{{ publicLinkUrl }}</f7-actions-label>
                <f7-actions-button @click="copyLink">{{ tt('Copy') }}</f7-actions-button>
                <f7-actions-button @click="updatePublicLink(true)">{{ tt('Reset Link') }}</f7-actions-button>
                <f7-actions-button color="red" @click="updatePublicLink(false)">{{ tt('Disable Link') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import InvoiceDocument from '@/components/common/InvoiceDocument.vue';
import RecurringInvoiceAddPopup from './RecurringInvoiceAddPopup.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useInvoiceDetailPageBase } from '@/views/base/invoices/InvoiceDetailPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';
import { useAccountsStore } from '@/stores/account.ts';

import { TextDirection } from '@/core/text.ts';
import { TRANSACTION_MAX_AMOUNT } from '@/consts/transaction.ts';
import type { InvoiceEmail, InvoicePayment } from '@/models/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showToast, showPrompt, showConfirm, routeBackOnError } = useI18nUIComponents();
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
    viewedEmail,
    loadingEmail,
    formatAmount,
    formatDate,
    needsSettledAmount,
    formatPaymentAmount,
    formatPaymentDescription,
    formatEmailSentTime,
    viewEmail,
    printInvoice,
    setPublicLink,
    copyPublicLink,
    downloadInvoicePdf
} = useInvoiceDetailPageBase();

function downloadPdf(): void {
    downloadInvoicePdf().catch(error => showToast(error.message || error));
}

function openEmail(email: InvoiceEmail): void {
    viewEmail(email).then(() => {
        showEmailPopup.value = true;
    }).catch(error => {
        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

const invoicesStore = useInvoicesStore();
const accountsStore = useAccountsStore();

const loadingError = ref<unknown | null>(null);
const invoiceId = ref<string>(props.f7route.query['id'] || '');
const showMoreActionSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);
const showRepeatMonthlyPopup = ref<boolean>(false);
const showShareActionSheet = ref<boolean>(false);
const showSettledAmountSheet = ref<boolean>(false);
const showEmailPopup = ref<boolean>(false);
const settledAmount = ref<number>(0);
const settledAmountSuggestion = ref<InvoicePayment | null>(null); // the cross-currency suggestion waiting for its settled amount

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

function load(): void {
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
                showToast(error.message || error);
            }
        });
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function runAndReload(action: () => Promise<unknown>): void {
    updating.value = true;
    showLoading(() => updating.value);

    action().then(() => {
        updating.value = false;
        hideLoading();
        load();
    }).catch(error => {
        updating.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

// a suggestion in another currency first asks what it settled in the invoice currency; same-currency links stay one tap
function link(suggestion: InvoicePayment): void {
    if (needsSettledAmount(suggestion)) {
        settledAmountSuggestion.value = suggestion;
        settledAmount.value = balanceDue.value;
        showSettledAmountSheet.value = true;
        return;
    }

    runAndReload(() => invoicesStore.linkInvoicePayment({ invoiceId: invoiceId.value, transactionId: suggestion.transactionId }));
}

function onSettledAmountEntered(amount: number): void {
    const suggestion = settledAmountSuggestion.value;
    settledAmountSuggestion.value = null;

    if (suggestion && amount > 0) {
        runAndReload(() => invoicesStore.linkInvoicePayment({ invoiceId: invoiceId.value, transactionId: suggestion.transactionId, settledAmount: amount }));
    }
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

    showConfirm("Regenerate this invoice with the customer's current details? The PDF and public link will show them too.", () => {
        runAndReload(() => invoicesStore.saveInvoice({ invoice: current, isEdit: true }));
    });
}

function toggleClosed(): void {
    if (invoice.value) {
        const current = invoice.value;
        runAndReload(() => invoicesStore.setInvoiceClosed({ invoice: current, closed: !current.closedManually }));
    }
}

function remove(): void {
    if (!invoice.value) {
        return;
    }

    const router = props.f7router;
    showDeleteActionSheet.value = false;
    updating.value = true;
    showLoading(() => updating.value);

    invoicesStore.deleteInvoice({ invoice: invoice.value }).then(() => {
        updating.value = false;
        hideLoading();
        router.back();
    }).catch(error => {
        updating.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

// first share creates the link; afterwards the sheet just shows it
function share(): void {
    if (!invoice.value) {
        return;
    }

    if (invoice.value.publicToken) {
        showShareActionSheet.value = true;
    } else {
        updatePublicLink(true);
    }
}

function updatePublicLink(enabled: boolean): void {
    showShareActionSheet.value = false;
    updating.value = true;
    showLoading(() => updating.value);

    setPublicLink(enabled).then(() => {
        updating.value = false;
        hideLoading();

        if (enabled) {
            showShareActionSheet.value = true;
        } else {
            showToast('Public link has been disabled');
        }
    }).catch(error => {
        updating.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function copyLink(): void {
    copyPublicLink().then(copied => {
        if (copied) {
            showToast('Link copied');
        } else {
            showPrompt('Share Link', publicLinkUrl.value); // no clipboard (plain http): a field to copy from by hand
        }
    });
}

function onPageAfterIn(): void {
    if (!loading.value) {
        load(); // back from the edit page
    }

    routeBackOnError(props.f7router, loadingError);
}

load();
</script>

<style>
.invoice-document-preview {
    overflow-x: auto;
}

.invoice-public-link {
    word-break: break-all;
}

.invoice-sent-email-frame {
    display: block;
    box-sizing: border-box;
    width: calc(100% - 32px);
    height: 70vh;
    margin: 0 16px;
    border: 1px solid var(--f7-list-item-border-color, #ddd);
    border-radius: 4px;
    background: white;
}
</style>
