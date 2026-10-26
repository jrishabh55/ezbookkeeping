<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('Buckets') }}</span>
                        <v-btn class="ms-3" color="default" variant="outlined"
                               :disabled="loading" @click="add">{{ tt('Add') }}</v-btn>
                        <v-btn density="compact" color="default" variant="text" class="ms-2"
                               :aria-label="tt('Refresh')" :icon="true" :loading="loading" @click="reload(true)">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                        </v-btn>
                        <v-spacer/>
                        <v-btn density="comfortable" color="default" variant="text" class="ms-2"
                               :aria-label="tt('More')" :disabled="loading" :icon="true">
                            <v-icon :icon="mdiDotsVertical" />
                            <v-menu activator="parent">
                                <v-list>
                                    <v-list-item :prepend-icon="mdiEyeOutline"
                                                 :title="tt('Show Hidden Buckets')"
                                                 v-if="!showHidden" @click="showHidden = true"></v-list-item>
                                    <v-list-item :prepend-icon="mdiEyeOffOutline"
                                                 :title="tt('Hide Hidden Buckets')"
                                                 v-if="showHidden" @click="showHidden = false"></v-list-item>
                                </v-list>
                            </v-menu>
                        </v-btn>
                    </div>
                </template>

                <v-card-text class="pb-0">
                    <v-row>
                        <v-col cols="12">
                            <v-alert variant="tonal" density="compact"
                                     :text="tt('Buckets help you save money for specific goals like vacations, emergency funds, or large purchases.')" />
                        </v-col>
                    </v-row>
                </v-card-text>

                <v-card-text>
                    <v-row v-if="loading && noAvailableBucket">
                        <v-col v-for="itemIdx in [ 1, 2, 3 ]" :key="itemIdx" cols="12" md="6" lg="4">
                            <v-card variant="outlined" class="bucket-card">
                                <v-card-title class="d-flex align-center">
                                    <v-skeleton-loader class="skeleton-no-margin" type="avatar" :loading="true"></v-skeleton-loader>
                                    <v-skeleton-loader class="skeleton-no-margin ms-2" width="120px" type="text" :loading="true"></v-skeleton-loader>
                                    <v-spacer />
                                    <v-skeleton-loader class="skeleton-no-margin" width="60px" type="text" :loading="true"></v-skeleton-loader>
                                </v-card-title>

                                <v-card-text>
                                    <div class="d-flex justify-space-between align-center mb-2">
                                        <span class="text-medium-emphasis">{{ tt('Current Balance') }}</span>
                                        <v-skeleton-loader class="skeleton-no-margin" width="80px" type="text" :loading="true"></v-skeleton-loader>
                                    </div>

                                    <v-skeleton-loader class="skeleton-no-margin mb-2" height="8" type="image" :loading="true"></v-skeleton-loader>
                                </v-card-text>
                            </v-card>
                        </v-col>
                    </v-row>

                    <v-row v-else-if="!loading && noAvailableBucket">
                        <v-col cols="12">
                            <span class="text-body-medium">{{ tt('No available bucket') }}</span>
                        </v-col>
                    </v-row>

                    <v-row v-else>
                        <v-col v-for="bucket in displayBuckets" :key="bucket.id" cols="12" md="6" lg="4">
                            <v-card variant="outlined" class="bucket-card">
                                <v-card-title class="d-flex align-center">
                                    <v-avatar :color="'#' + bucket.color" size="32" class="me-2">
                                        <v-icon :icon="mdiPiggyBankOutline" size="18" color="white" />
                                    </v-avatar>
                                    <span class="text-truncate">{{ bucket.name }}</span>
                                    <v-spacer />
                                    <v-chip size="x-small" :color="bucket.isCompleted() ? 'success' : (bucket.hidden ? 'grey' : 'primary')" variant="tonal">
                                        {{ bucket.isCompleted() ? tt('Completed') : (bucket.hidden ? tt('Hidden') : tt('In Progress')) }}
                                    </v-chip>
                                </v-card-title>

                                <v-card-text>
                                    <div class="d-flex justify-space-between mb-2">
                                        <span class="text-medium-emphasis">{{ tt('Current Balance') }}</span>
                                        <span class="font-weight-bold">
                                            {{ formatAmount(bucket.currentBalance, bucket.currency) }}
                                        </span>
                                    </div>

                                    <div class="d-flex justify-space-between mb-2" v-if="bucket.targetAmount > 0">
                                        <span class="text-medium-emphasis">{{ tt('Target') }}</span>
                                        <span>{{ formatAmount(bucket.targetAmount, bucket.currency) }}</span>
                                    </div>

                                    <v-progress-linear
                                        v-if="bucket.targetAmount > 0"
                                        :model-value="getProgressPercentage(bucket)"
                                        :color="bucket.isCompleted() ? 'success' : 'primary'"
                                        height="8"
                                        rounded
                                        class="mb-2"
                                    />

                                    <div class="text-caption text-medium-emphasis" v-if="bucket.comment">
                                        {{ bucket.comment }}
                                    </div>
                                </v-card-text>

                                <v-card-actions>
                                    <v-btn color="success" variant="text" size="small" :disabled="loading" @click="allocate(bucket)">
                                        {{ tt('Add Money') }}
                                    </v-btn>
                                    <v-btn color="warning" variant="text" size="small" :disabled="loading" @click="withdraw(bucket)">
                                        {{ tt('Withdraw') }}
                                    </v-btn>
                                    <v-spacer />
                                    <v-btn variant="text" size="small" :icon="true" :disabled="loading" @click="edit(bucket)">
                                        <v-icon :icon="mdiPencilOutline" size="18" />
                                        <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                                    </v-btn>
                                    <v-btn variant="text" size="small" :icon="true" :disabled="loading" @click="confirmDeleteBucket(bucket)">
                                        <v-icon :icon="mdiDeleteOutline" size="18" />
                                        <v-tooltip activator="parent">{{ tt('Delete') }}</v-tooltip>
                                    </v-btn>
                                </v-card-actions>
                            </v-card>
                        </v-col>
                    </v-row>
                </v-card-text>
            </v-card>
        </v-col>
    </v-row>

    <bucket-edit-dialog ref="editDialog" />
    <bucket-transaction-dialog ref="transactionDialog" />

    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { useTemplateRef } from 'vue';

import {
    mdiRefresh,
    mdiDotsVertical,
    mdiEyeOutline,
    mdiEyeOffOutline,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiPiggyBankOutline
} from '@mdi/js';

import { useI18n } from '@/locales/helpers.ts';
import { useBucketListPageBase } from '@/views/base/buckets/BucketListPageBase.ts';

import { useBucketsStore } from '@/stores/bucket.ts';

import { Bucket } from '@/models/bucket.ts';

import BucketEditDialog from './dialogs/BucketEditDialog.vue';
import BucketTransactionDialog from './dialogs/BucketTransactionDialog.vue';
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;
type BucketEditDialogType = InstanceType<typeof BucketEditDialog>;
type BucketTransactionDialogType = InstanceType<typeof BucketTransactionDialog>;

const { tt } = useI18n();

const {
    loading,
    showHidden,
    displayBuckets,
    noAvailableBucket,
    formatAmount,
    getProgressPercentage
} = useBucketListPageBase();

const bucketsStore = useBucketsStore();

const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const editDialog = useTemplateRef<BucketEditDialogType>('editDialog');
const transactionDialog = useTemplateRef<BucketTransactionDialogType>('transactionDialog');

function init(): void {
    bucketsStore.loadAllBuckets({ force: false }).then(() => {
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function reload(force: boolean): void {
    loading.value = true;

    bucketsStore.loadAllBuckets({ force: force }).then(() => {
        loading.value = false;

        if (force) {
            snackbar.value?.showMessage('Bucket list has been updated');
        }
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function add(): void {
    editDialog.value?.open({}).then(result => {
        if (result && result.message) {
            snackbar.value?.showMessage(result.message);
        }
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function edit(bucket: Bucket): void {
    editDialog.value?.open({
        id: bucket.id,
        currentBucket: bucket
    }).then(result => {
        if (result && result.message) {
            snackbar.value?.showMessage(result.message);
        }
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function allocate(bucket: Bucket): void {
    transactionDialog.value?.open({
        bucket: bucket,
        type: 'allocate'
    }).then(result => {
        if (result && result.message) {
            snackbar.value?.showMessage(result.message);
        }
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function withdraw(bucket: Bucket): void {
    transactionDialog.value?.open({
        bucket: bucket,
        type: 'withdraw'
    }).then(result => {
        if (result && result.message) {
            snackbar.value?.showMessage(result.message);
        }
    }).catch(error => {
        if (error) {
            snackbar.value?.showError(error);
        }
    });
}

function confirmDeleteBucket(bucket: Bucket): void {
    confirmDialog.value?.open('Are you sure you want to delete this bucket?').then(() => {
        loading.value = true;

        bucketsStore.deleteBucket({ bucket }).then(() => {
            loading.value = false;
        }).catch(error => {
            loading.value = false;

            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

init();
</script>

<style scoped>
.bucket-card {
    transition: box-shadow 0.2s ease;
}

.bucket-card:hover {
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}
</style>
