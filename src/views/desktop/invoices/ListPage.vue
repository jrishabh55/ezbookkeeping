<template>
    <main-page-layout nav-items-class="my-3">
        <template #nav-items>
            <div class="mx-2 mb-2">
                <span class="text-body-medium">{{ tt('Outstanding') }}</span>
                <div class="text-body-large text-truncate mt-1 mb-2">
                    <template v-if="!loading || allInvoices.length > 0">
                        <div :key="total.currency" v-for="total in outstandingTotals">{{ formatAmount(total.amount, total.currency) }}</div>
                    </template>
                    <v-skeleton-loader class="skeleton-no-margin mt-1 mb-2" type="text" :loading="true" v-else></v-skeleton-loader>
                </div>
                <span class="text-body-medium">{{ tt('Overdue') }}</span>
                <div class="text-body-large text-expense text-truncate mt-1">
                    <template v-if="!loading || allInvoices.length > 0">
                        <div :key="total.currency" v-for="total in overdueTotals">{{ formatAmount(total.amount, total.currency) }}</div>
                    </template>
                    <v-skeleton-loader class="skeleton-no-margin mt-1 mb-2" type="text" :loading="true" v-else></v-skeleton-loader>
                </div>
            </div>

            <v-divider class="my-2" />

            <li class="nav-link">
                <a class="d-flex align-center cursor-pointer"
                   :class="{ 'router-link-active router-link-exact-active': activeView === 'invoices', 'disabled': loading }"
                   @click="activeView = 'invoices'">
                    <v-icon class="nav-item-icon" :icon="mdiFileDocumentOutline"/>
                    <span class="nav-item-title">{{ tt('Invoices') }}</span>
                </a>
            </li>
            <li class="nav-link">
                <a class="d-flex align-center cursor-pointer"
                   :class="{ 'router-link-active router-link-exact-active': activeView === 'recurring', 'disabled': loading }"
                   @click="activeView = 'recurring'">
                    <v-icon class="nav-item-icon" :icon="mdiRepeat"/>
                    <span class="nav-item-title">{{ tt('Recurring') }}</span>
                </a>
            </li>
            <li class="nav-link">
                <a class="d-flex align-center cursor-pointer"
                   :class="{ 'router-link-active router-link-exact-active': activeView === 'estimates', 'disabled': loading }"
                   @click="activeView = 'estimates'">
                    <v-icon class="nav-item-icon" :icon="mdiFileDocumentCheckOutline"/>
                    <span class="nav-item-title">{{ tt('Estimates') }}</span>
                </a>
            </li>
        </template>

        <template #content>
            <v-row class="match-height">
                <v-col cols="12">
                    <v-card>
                        <template #title>
                            <div class="title-and-toolbar d-flex align-center">
                                <span>{{ tt(activeViewTitle) }}</span>
                                <v-btn class="ms-3" color="default" variant="outlined"
                                       :disabled="loading || updating" @click="add" v-if="activeView !== 'recurring'">{{ tt('Add') }}</v-btn>
                                <v-btn density="compact" color="default" variant="text" class="ms-2"
                                       :aria-label="tt('Refresh')" :icon="true" :disabled="loading || updating"
                                       :loading="loading" @click="reload(true)">
                                    <template #loader>
                                        <v-progress-circular indeterminate size="20"/>
                                    </template>
                                    <v-icon :icon="mdiRefresh" size="24" />
                                    <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                                </v-btn>
                                <v-spacer/>
                                <v-btn density="comfortable" color="default" variant="text" class="ms-2"
                                       :aria-label="tt('More')" :disabled="loading || updating" :icon="true">
                                    <v-icon :icon="mdiDotsVertical" />
                                    <v-menu activator="parent">
                                        <v-list>
                                            <v-list-item :prepend-icon="mdiEyeOutline" :title="tt('Show Paid Invoices')"
                                                         v-if="activeView === 'invoices' && !showPaid" @click="showPaid = true"></v-list-item>
                                            <v-list-item :prepend-icon="mdiEyeOffOutline" :title="tt('Hide Paid Invoices')"
                                                         v-if="activeView === 'invoices' && showPaid" @click="showPaid = false"></v-list-item>
                                            <v-list-item :prepend-icon="mdiEyeOutline" :title="tt('Show Closed Estimates')"
                                                         v-if="activeView === 'estimates' && !showClosed" @click="showClosed = true"></v-list-item>
                                            <v-list-item :prepend-icon="mdiEyeOffOutline" :title="tt('Hide Closed Estimates')"
                                                         v-if="activeView === 'estimates' && showClosed" @click="showClosed = false"></v-list-item>
                                            <v-list-item :prepend-icon="mdiFileDocumentEditOutline" :title="tt('Invoice Profile')"
                                                         to="/settings/invoice_profile"></v-list-item>
                                            <v-list-item :prepend-icon="mdiEmailEditOutline" :title="tt('Invoice Email Template')"
                                                         to="/settings/invoice_email_template"></v-list-item>
                                        </v-list>
                                    </v-menu>
                                </v-btn>
                            </div>
                        </template>

                        <v-table class="table-striped" :hover="!loading" v-if="activeView === 'invoices'">
                            <thead>
                            <tr>
                                <th>{{ tt('Invoice Number') }}</th>
                                <th>{{ tt('Customer') }}</th>
                                <th>{{ tt('Invoice Date') }}</th>
                                <th>{{ tt('Payment Due') }}</th>
                                <th class="text-right">{{ tt('Total') }}</th>
                                <th class="text-right">{{ tt('Balance Due') }}</th>
                                <th>{{ tt('Status') }}</th>
                            </tr>
                            </thead>

                            <tbody v-if="loading && noAvailableInvoice">
                            <tr :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]">
                                <td class="px-0" colspan="7"><v-skeleton-loader type="text" :loading="true"></v-skeleton-loader></td>
                            </tr>
                            </tbody>

                            <tbody v-if="!loading && noAvailableInvoice">
                            <tr><td colspan="7">{{ tt('No available invoice') }}</td></tr>
                            </tbody>

                            <tbody v-if="!noAvailableInvoice">
                            <tr class="cursor-pointer" :key="invoice.id" v-for="invoice in displayInvoices" @click="view(invoice)">
                                <td>{{ invoice.number }}</td>
                                <td>{{ invoice.billToName }}</td>
                                <td>{{ formatDate(invoice.invoiceDate) }}</td>
                                <td>{{ formatDate(invoice.dueDate) }}</td>
                                <td class="text-right">{{ formatAmount(invoice.total, invoice.currency) }}</td>
                                <td class="text-right">{{ formatAmount(getInvoiceDisplayBalanceDue(invoice), invoice.currency) }}</td>
                                <td>
                                    <v-chip size="small" variant="tonal" :color="statusColors[getDisplayStatus(invoice)]">{{ tt(getDisplayStatusName(invoice)) }}</v-chip>
                                </td>
                            </tr>
                            </tbody>
                        </v-table>

                        <v-table class="table-striped" :hover="!loading" v-if="activeView === 'recurring'">
                            <thead>
                            <tr>
                                <th>{{ tt('Customer') }}</th>
                                <th>{{ tt('Source Invoice') }}</th>
                                <th>{{ tt('Schedule') }}</th>
                                <th>{{ tt('Next Date') }}</th>
                                <th>{{ tt('Enabled') }}</th>
                                <th class="text-right">{{ tt('Operation') }}</th>
                            </tr>
                            </thead>

                            <tbody v-if="!loading && noAvailableRecurringInvoice">
                            <tr><td colspan="6">{{ tt('No available recurring invoice') }}</td></tr>
                            </tbody>

                            <tbody>
                            <tr :key="recurring.id" v-for="recurring in allRecurringInvoices"
                                @mouseover="hoveredRecurringId = recurring.id" @mouseleave="hoveredRecurringId = ''">
                                <td>{{ recurring.customerName }}</td>
                                <td>
                                    <a class="cursor-pointer" @click="viewRecurringSource(recurring)" v-if="recurring.sourceInvoiceNumber">{{ recurring.sourceInvoiceNumber }}</a>
                                    <span class="text-medium-emphasis" v-else>{{ getSourceNumber(recurring) }}</span>
                                </td>
                                <td>{{ getScheduleText(recurring) }}</td>
                                <td>{{ formatDate(recurring.nextDate) }}</td>
                                <td>
                                    <v-switch density="compact" hide-details color="primary"
                                              :disabled="loading || updating" :model-value="recurring.enabled"
                                              @update:model-value="setRecurringEnabled(recurring, !!$event)" />
                                </td>
                                <td class="text-right">
                                    <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                           :prepend-icon="mdiPencilOutline" :disabled="loading || updating"
                                           v-if="hoveredRecurringId === recurring.id && !loading"
                                           @click="editRecurring(recurring)">{{ tt('Edit') }}</v-btn>
                                    <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                           :prepend-icon="mdiDeleteOutline" :disabled="loading || updating"
                                           v-if="hoveredRecurringId === recurring.id && !loading"
                                           @click="removeRecurring(recurring)">{{ tt('Delete') }}</v-btn>
                                </td>
                            </tr>
                            </tbody>
                        </v-table>

                        <v-table class="table-striped" :hover="!loading" v-if="activeView === 'estimates'">
                            <thead>
                            <tr>
                                <th>{{ tt('Estimate Number') }}</th>
                                <th>{{ tt('Customer') }}</th>
                                <th>{{ tt('Estimate Date') }}</th>
                                <th>{{ tt('Valid Until') }}</th>
                                <th class="text-right">{{ tt('Total') }}</th>
                                <th>{{ tt('Status') }}</th>
                            </tr>
                            </thead>

                            <tbody v-if="loading && noAvailableEstimate">
                            <tr :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]">
                                <td class="px-0" colspan="6"><v-skeleton-loader type="text" :loading="true"></v-skeleton-loader></td>
                            </tr>
                            </tbody>

                            <tbody v-if="!loading && noAvailableEstimate">
                            <tr><td colspan="6">{{ tt('No available estimate') }}</td></tr>
                            </tbody>

                            <tbody v-if="!noAvailableEstimate">
                            <tr class="cursor-pointer" :key="estimate.id" v-for="estimate in displayEstimates" @click="view(estimate)">
                                <td>{{ estimate.number }}</td>
                                <td>{{ estimate.billToName }}</td>
                                <td>{{ formatDate(estimate.invoiceDate) }}</td>
                                <td>{{ formatDate(estimate.dueDate) }}</td>
                                <td class="text-right">{{ formatAmount(estimate.total, estimate.currency) }}</td>
                                <td>
                                    <v-chip size="small" variant="tonal" :color="ESTIMATE_STATUS_COLORS[getEstimateStatus(estimate)]">{{ tt(getEstimateStatusName(estimate)) }}</v-chip>
                                </td>
                            </tr>
                            </tbody>
                        </v-table>
                    </v-card>
                </v-col>
            </v-row>
        </template>
    </main-page-layout>

    <invoice-edit-dialog ref="invoiceEditDialog" />
    <invoice-detail-dialog ref="invoiceDetailDialog" />
    <recurring-invoice-edit-dialog ref="recurringInvoiceEditDialog" />
    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, useTemplateRef, onMounted } from 'vue';
import { useRoute, useRouter } from 'vue-router';

import {
    mdiRefresh,
    mdiDotsVertical,
    mdiEyeOutline,
    mdiEyeOffOutline,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiFileDocumentOutline,
    mdiFileDocumentEditOutline,
    mdiFileDocumentCheckOutline,
    mdiEmailEditOutline,
    mdiRepeat
} from '@mdi/js';

import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import InvoiceEditDialog from './dialogs/InvoiceEditDialog.vue';
import InvoiceDetailDialog from './dialogs/InvoiceDetailDialog.vue';
import RecurringInvoiceEditDialog from './dialogs/RecurringInvoiceEditDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceListPageBase } from '@/views/base/invoices/InvoiceListPageBase.ts';
import { useRecurringInvoiceListPageBase } from '@/views/base/invoices/RecurringInvoiceListPageBase.ts';
import { useEstimateListPageBase, ESTIMATE_STATUS_COLORS } from '@/views/base/invoices/EstimateListPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { Invoice, RecurringInvoice } from '@/models/invoice.ts';

import { type InvoiceDisplayStatus, getInvoiceDisplayBalanceDue } from '@/lib/invoice.ts';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type InvoiceEditDialogType = InstanceType<typeof InvoiceEditDialog>;
type InvoiceDetailDialogType = InstanceType<typeof InvoiceDetailDialog>;
type RecurringInvoiceEditDialogType = InstanceType<typeof RecurringInvoiceEditDialog>;

const { tt } = useI18n();
const {
    loading,
    showPaid,
    allInvoices,
    displayInvoices,
    noAvailableInvoice,
    outstandingTotals,
    overdueTotals,
    formatAmount,
    formatDate,
    getDisplayStatus,
    getDisplayStatusName
} = useInvoiceListPageBase();
const {
    allRecurringInvoices,
    noAvailableRecurringInvoice,
    getSourceNumber,
    getScheduleText
} = useRecurringInvoiceListPageBase();
const {
    showClosed,
    displayEstimates,
    noAvailableEstimate,
    getStatus: getEstimateStatus,
    getStatusName: getEstimateStatusName
} = useEstimateListPageBase();

const invoicesStore = useInvoicesStore();
const route = useRoute();
const router = useRouter();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const invoiceEditDialog = useTemplateRef<InvoiceEditDialogType>('invoiceEditDialog');
const invoiceDetailDialog = useTemplateRef<InvoiceDetailDialogType>('invoiceDetailDialog');
const recurringInvoiceEditDialog = useTemplateRef<RecurringInvoiceEditDialogType>('recurringInvoiceEditDialog');

const statusColors: Record<InvoiceDisplayStatus, string> = { unpaid: 'warning', partial: 'primary', paid: 'success', overdue: 'error' };

const activeView = ref<'invoices' | 'recurring' | 'estimates'>('invoices');
const updating = ref<boolean>(false);
const hoveredRecurringId = ref<string>('');

const VIEW_TITLES: Record<'invoices' | 'recurring' | 'estimates', string> = { invoices: 'Invoices', recurring: 'Recurring', estimates: 'Estimates' };
const activeViewTitle = computed<string>(() => VIEW_TITLES[activeView.value]);

function reload(force: boolean): void {
    loading.value = true;

    Promise.all([
        invoicesStore.loadAllInvoices({ force: force }),
        invoicesStore.loadAllRecurringInvoices({ force: force }),
        invoicesStore.loadInvoiceProfile({ force: false }),
        invoicesStore.loadAllEstimates({ force: force })
    ]).then(() => {
        loading.value = false;

        if (force) {
            snackbar.value?.showMessage('Invoice list has been updated');
        }
    }).catch(error => {
        loading.value = false;

        if (error.isUpToDate) {
            snackbar.value?.showMessage('Invoice list is up to date');
        } else if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function add(): void {
    invoiceEditDialog.value?.open({ kind: activeView.value === 'estimates' ? 'estimate' : 'invoice' }).then(result => {
        snackbar.value?.showMessage(result.message);
        reload(false);
        view(result.invoice);
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function view(invoice: Invoice): void {
    viewById(invoice.id);
}

function viewById(invoiceId: string): void {
    invoiceDetailDialog.value?.open({ id: invoiceId }).then(result => {
        if (result.changed) {
            reload(false);
        }
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function viewRecurringSource(recurring: RecurringInvoice): void {
    invoiceDetailDialog.value?.open({ id: recurring.sourceInvoiceId }).then(result => {
        if (result.changed) {
            reload(false);
        }
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function setRecurringEnabled(recurring: RecurringInvoice, enabled: boolean): void {
    if (recurring.enabled === enabled) {
        return;
    }

    const modified = recurring.clone();
    modified.enabled = enabled;
    updating.value = true;

    invoicesStore.modifyRecurringInvoice({ recurring: modified }).then(() => {
        updating.value = false;
    }).catch(error => {
        updating.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function editRecurring(recurring: RecurringInvoice): void {
    recurringInvoiceEditDialog.value?.open({ recurring }).then(result => {
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function removeRecurring(recurring: RecurringInvoice): void {
    confirmDialog.value?.open('Are you sure you want to delete this recurring invoice?').then(() => {
        updating.value = true;

        invoicesStore.deleteRecurringInvoice({ recurring }).then(() => {
            updating.value = false;
        }).catch(error => {
            updating.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

reload(false);

onMounted(() => {
    // opened from the invoice chip on a transaction: show that invoice, then drop the query so a refresh does not reopen it
    const requestedInvoiceId = route.query['invoiceId'];

    if (typeof requestedInvoiceId === 'string' && requestedInvoiceId) {
        router.replace('/invoice/list');
        viewById(requestedInvoiceId);
    }
});
</script>
