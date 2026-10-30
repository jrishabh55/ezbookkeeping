<template>
    <main-page-layout no-navbar>
        <template #content>
            <v-row class="match-height">
                <v-col cols="12">
                    <v-card>
                        <template #title>
                            <div class="title-and-toolbar d-flex align-center">
                                <span>{{ tt('Customers') }}</span>
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
                            </div>
                        </template>

                        <v-table class="table-striped" :hover="!loading">
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
                                        <a class="cursor-pointer" @click="viewCustomer(customer)">{{ customer.name }}</a>
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

    <customer-edit-dialog ref="customerEditDialog" />
    <customer-detail-dialog ref="customerDetailDialog" />
    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { computed, ref, useTemplateRef } from 'vue';

import {
    mdiRefresh,
    mdiPencilOutline,
    mdiDeleteOutline
} from '@mdi/js';

import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import CustomerEditDialog from '../invoices/dialogs/CustomerEditDialog.vue';
import CustomerDetailDialog from '../invoices/dialogs/CustomerDetailDialog.vue';

import { useI18n } from '@/locales/helpers.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

import type { Customer } from '@/models/invoice.ts';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type CustomerEditDialogType = InstanceType<typeof CustomerEditDialog>;
type CustomerDetailDialogType = InstanceType<typeof CustomerDetailDialog>;

const { tt } = useI18n();

const invoicesStore = useInvoicesStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const customerEditDialog = useTemplateRef<CustomerEditDialogType>('customerEditDialog');
const customerDetailDialog = useTemplateRef<CustomerDetailDialogType>('customerDetailDialog');

const loading = ref<boolean>(true);
const updating = ref<boolean>(false);
const hoveredCustomerId = ref<string>('');

const allCustomers = computed<Customer[]>(() => invoicesStore.allCustomers);

function reload(force: boolean): void {
    loading.value = true;

    invoicesStore.loadAllCustomers({ force: force }).then(() => {
        loading.value = false;

        if (force) {
            snackbar.value?.showMessage('Customer list has been updated');
        }
    }).catch(error => {
        loading.value = false;

        if (error.isUpToDate) {
            snackbar.value?.showMessage('Customer list is up to date');
        } else if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function add(): void {
    customerEditDialog.value?.open({}).then(result => {
        snackbar.value?.showMessage(result.message);
    }).catch(error => {
        if (error && !error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function viewCustomer(customer: Customer): void {
    customerDetailDialog.value?.open({ id: customer.id });
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
</script>
