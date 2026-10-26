<template>
    <main-page-layout nav-items-class="my-3">
        <template #nav-items>
            <div class="mx-2 mb-2">
                <span class="text-body-medium">{{ tt('Outstanding') }}</span>
                <div class="text-body-large text-truncate mt-1 mb-2">
                    <span v-if="!loading || allInvoices.length > 0">{{ formatAmount(outstandingTotal) }}</span>
                    <v-skeleton-loader class="skeleton-no-margin mt-1 mb-2" type="text" :loading="true" v-else></v-skeleton-loader>
                </div>
                <span class="text-body-medium">{{ tt('Overdue') }}</span>
                <div class="text-body-large text-expense text-truncate mt-1">
                    <span v-if="!loading || allInvoices.length > 0">{{ formatAmount(overdueTotal) }}</span>
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
                   :class="{ 'router-link-active router-link-exact-active': activeView === 'customers', 'disabled': loading }"
                   @click="activeView = 'customers'">
                    <v-icon class="nav-item-icon" :icon="mdiAccountMultipleOutline"/>
                    <span class="nav-item-title">{{ tt('Customers') }}</span>
                </a>
            </li>
        </template>

        <template #content>
            <v-row class="match-height">
                <v-col cols="12">
                    <v-card>
                        <template #title>
                            <div class="title-and-toolbar d-flex align-center">
                                <span>{{ activeView === 'invoices' ? tt('Invoices') : tt('Customers') }}</span>
                                <v-btn class="ms-3" color="default" variant="outlined"
                                       :disabled="loading || updating" @click="add">{{ tt('Add') }}</v-btn>
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
                                            <v-list-item :prepend-icon="mdiFileDocumentEditOutline" :title="tt('Invoice Profile')"
                                                         to="/settings/invoice_profile"></v-list-item>
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
                                <td class="text-right">{{ formatAmount(invoice.total) }}</td>
                                <td class="text-right">{{ formatAmount(getInvoiceDisplayBalanceDue(invoice)) }}</td>
                                <td>
                                    <v-chip size="small" variant="tonal" :color="statusColors[getDisplayStatus(invoice)]">{{ tt(getDisplayStatusName(invoice)) }}</v-chip>
                                </td>
                            </tr>
                            </tbody>
                        </v-table>

                        <v-table class="table-striped" :hover="!loading" v-if="activeView === 'customers'">
                            <thead>
                            <tr>
                                <th>
                                    <div class="d-flex align-center">
                                        <span>{{ tt('Customer Name') }}</span>
                                        <v-spacer/>
                                        <span>{{ tt('Operation') }}</span>
                                    </div>
                                </th>
                            </tr>
                            </thead>

                            <tbody v-if="!loading && !allCustomers.length">
                            <tr><td>{{ tt('No available customer') }}</td></tr>
                            </tbody>

                            <tbody>
                            <tr :key="customer.id" v-for="customer in allCustomers"
                                @mouseover="hoveredCustomerId = customer.id" @mouseleave="hoveredCustomerId = ''">
                                <td>
                                    <div class="d-flex align-center">
                                        <span>{{ customer.name }}</span>
                                        <small class="text-medium-emphasis ms-2" v-if="customer.contactName">{{ customer.contactName }}</small>
                                        <v-spacer/>
                                        <template v-if="hoveredCustomerId === customer.id && !loading">
                                            <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                                   :prepend-icon="mdiPencilOutline" :disabled="loading || updating"
                                                   @click="editCustomer(customer)">{{ tt('Edit') }}</v-btn>
                                            <v-btn class="px-2" color="default" density="comfortable" variant="text"
                                                   :prepend-icon="mdiDeleteOutline" :disabled="loading || updating"
                                                   @click="removeCustomer(customer)">{{ tt('Delete') }}</v-btn>
                                        </template>
                                    </div>
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
    <customer-edit-dialog ref="customerEditDialog" />
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
    mdiAccountMultipleOutline
} from '@mdi/js';

import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import InvoiceEditDialog from './dialogs/InvoiceEditDialog.vue';
import InvoiceDetailDialog from './dialogs/InvoiceDetailDialog.vue';
import CustomerEditDialog from './dialogs/CustomerEditDialog.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceListPageBase } from '@/views/base/invoices/InvoiceListPageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { Customer, Invoice } from '@/models/invoice.ts';

import { type InvoiceDisplayStatus, getInvoiceDisplayBalanceDue } from '@/lib/invoice.ts';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type InvoiceEditDialogType = InstanceType<typeof InvoiceEditDialog>;
type InvoiceDetailDialogType = InstanceType<typeof InvoiceDetailDialog>;
type CustomerEditDialogType = InstanceType<typeof CustomerEditDialog>;

const { tt } = useI18n();
const {
    loading,
    showPaid,
    allInvoices,
    displayInvoices,
    noAvailableInvoice,
    outstandingTotal,
    overdueTotal,
    formatAmount,
    formatDate,
    getDisplayStatus,
    getDisplayStatusName
} = useInvoiceListPageBase();

const invoicesStore = useInvoicesStore();
const route = useRoute();
const router = useRouter();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const invoiceEditDialog = useTemplateRef<InvoiceEditDialogType>('invoiceEditDialog');
const invoiceDetailDialog = useTemplateRef<InvoiceDetailDialogType>('invoiceDetailDialog');
const customerEditDialog = useTemplateRef<CustomerEditDialogType>('customerEditDialog');

const statusColors: Record<InvoiceDisplayStatus, string> = { unpaid: 'warning', partial: 'primary', paid: 'success', overdue: 'error' };

const activeView = ref<'invoices' | 'customers'>('invoices');
const updating = ref<boolean>(false);
const hoveredCustomerId = ref<string>('');

const allCustomers = computed<Customer[]>(() => invoicesStore.allCustomers);

function reload(force: boolean): void {
    loading.value = true;

    Promise.all([
        invoicesStore.loadAllInvoices({ force: force }),
        invoicesStore.loadAllCustomers({ force: force })
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
    if (activeView.value === 'customers') {
        customerEditDialog.value?.open({}).then(result => {
            snackbar.value?.showMessage(result.message);
        }).catch(error => {
            if (error && !error.processed) {
                snackbar.value?.showError(error);
            }
        });
        return;
    }

    invoiceEditDialog.value?.open({}).then(result => {
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

function editCustomer(customer: Customer): void {
    customerEditDialog.value?.open({ id: customer.id, currentCustomer: customer }).then(result => {
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function removeCustomer(customer: Customer): void {
    confirmDialog.value?.open('Are you sure you want to delete this customer?').then(() => {
        updating.value = true;

        invoicesStore.deleteCustomer({ customer }).then(() => {
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
