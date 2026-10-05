<template>
    <v-row class="match-height">
        <v-col cols="12">
            <v-card>
                <template #title>
                    <div class="title-and-toolbar d-flex align-center">
                        <span>{{ tt('Buckets') }}</span>
                        <v-btn density="compact" color="default" variant="text" size="24"
                               class="ms-2" :icon="true" :loading="loading" @click="reload">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                        </v-btn>
                        <v-spacer />
                        <v-btn color="primary" variant="tonal" size="small"
                               :disabled="loading" @click="add">
                            <v-icon :icon="mdiPlus" size="20" start />
                            {{ tt('Add') }}
                        </v-btn>
                    </div>
                </template>

                <v-card-text class="pb-0">
                    <v-row>
                        <v-col cols="12">
                            <v-alert type="info" variant="tonal" density="compact"
                                     :text="tt('Buckets help you save money for specific goals like vacations, emergency funds, or large purchases.')" />
                        </v-col>
                    </v-row>
                </v-card-text>

                <v-card-text>
                    <v-row v-if="loading && !allBuckets.length">
                        <v-col cols="12" class="d-flex justify-center">
                            <v-progress-circular indeterminate />
                        </v-col>
                    </v-row>

                    <v-row v-else-if="!allBuckets.length">
                        <v-col cols="12">
                            <div class="text-center text-muted py-8">
                                <v-icon :icon="mdiPiggyBankOutline" size="48" class="mb-4" />
                                <p>{{ tt('No buckets') }}</p>
                                <v-btn color="primary" variant="tonal" size="small" @click="add">
                                    {{ tt('Add Bucket') }}
                                </v-btn>
                            </div>
                        </v-col>
                    </v-row>

                    <v-row v-else>
                        <v-col v-for="bucket in allBuckets" :key="bucket.id" cols="12" md="6" lg="4">
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
                                        <span class="text-muted">{{ tt('Current Balance') }}</span>
                                        <span class="font-weight-bold">
                                            {{ formatAmount(bucket.currentBalance, bucket.currency) }}
                                        </span>
                                    </div>

                                    <div class="d-flex justify-space-between mb-2" v-if="bucket.targetAmount > 0">
                                        <span class="text-muted">{{ tt('Target') }}</span>
                                        <span>{{ formatAmount(bucket.targetAmount, bucket.currency) }}</span>
                                    </div>

                                    <v-progress-linear
                                        v-if="bucket.targetAmount > 0"
                                        :model-value="bucket.progress"
                                        :color="bucket.isCompleted() ? 'success' : 'primary'"
                                        height="8"
                                        rounded
                                        class="mb-2"
                                    />

                                    <div class="text-caption text-muted" v-if="bucket.comment">
                                        {{ bucket.comment }}
                                    </div>
                                </v-card-text>

                                <v-card-actions>
                                    <v-btn color="success" variant="text" size="small" @click="allocateToBucket(bucket)">
                                        {{ tt('Add Money') }}
                                    </v-btn>
                                    <v-btn color="warning" variant="text" size="small" @click="withdrawFromBucket(bucket)">
                                        {{ tt('Withdraw') }}
                                    </v-btn>
                                    <v-spacer />
                                    <v-btn variant="text" size="small" :icon="true" @click="editBucket(bucket)">
                                        <v-icon :icon="mdiPencilOutline" size="18" />
                                        <v-tooltip activator="parent">{{ tt('Edit') }}</v-tooltip>
                                    </v-btn>
                                    <v-btn variant="text" size="small" :icon="true" @click="confirmDeleteBucket(bucket)">
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

    <bucket-edit-dialog v-model:show="showEditDialog"
                        :bucket="editingBucket"
                        :is-edit="isEditMode"
                        @saved="onBucketSaved" />

    <bucket-transaction-dialog v-model:show="showTransactionDialog"
                               :bucket="transactionBucket"
                               :transaction-type="transactionType"
                               @saved="onTransactionSaved" />

    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, onMounted, useTemplateRef } from 'vue';
import { useI18n } from 'vue-i18n';

import {
    mdiRefresh,
    mdiPlus,
    mdiPencilOutline,
    mdiDeleteOutline,
    mdiPiggyBankOutline
} from '@mdi/js';

import { useBucketsStore } from '@/stores/bucket.ts';

import { Bucket } from '@/models/bucket.ts';

import BucketEditDialog from './dialogs/BucketEditDialog.vue';
import BucketTransactionDialog from './dialogs/BucketTransactionDialog.vue';
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;

const { t: tt } = useI18n();
const bucketsStore = useBucketsStore();

const loading = ref(false);
const showEditDialog = ref(false);
const showTransactionDialog = ref(false);
const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const editingBucket = ref<Bucket | null>(null);
const transactionBucket = ref<Bucket | null>(null);
const transactionType = ref<'allocate' | 'withdraw'>('allocate');
const isEditMode = ref(false);

const allBuckets = computed(() => bucketsStore.allBuckets);

function formatAmount(amount: number, currency: string): string {
    const displayAmount = amount / 100;
    return new Intl.NumberFormat(undefined, {
        style: 'currency',
        currency: currency || 'USD'
    }).format(displayAmount);
}

function reload(): void {
    loading.value = true;
    bucketsStore.loadAllBuckets({ force: true }).finally(() => {
        loading.value = false;
    });
}

function add(): void {
    editingBucket.value = Bucket.createNewBucket();
    isEditMode.value = false;
    showEditDialog.value = true;
}

function editBucket(bucket: Bucket): void {
    editingBucket.value = bucket;
    isEditMode.value = true;
    showEditDialog.value = true;
}

function allocateToBucket(bucket: Bucket): void {
    transactionBucket.value = bucket;
    transactionType.value = 'allocate';
    showTransactionDialog.value = true;
}

function withdrawFromBucket(bucket: Bucket): void {
    transactionBucket.value = bucket;
    transactionType.value = 'withdraw';
    showTransactionDialog.value = true;
}

function confirmDeleteBucket(bucket: Bucket): void {
    confirmDialog.value?.open('Are you sure you want to delete this bucket?').then(() => {
        loading.value = true;

        bucketsStore.deleteBucket({ bucket }).catch(error => {
            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        }).finally(() => {
            loading.value = false;
        });
    });
}

function onBucketSaved(): void {
    showEditDialog.value = false;
    reload();
}

function onTransactionSaved(): void {
    showTransactionDialog.value = false;
    reload();
}

onMounted(() => {
    reload();
});
</script>

<style scoped>
.bucket-card {
    transition: box-shadow 0.2s ease;
}

.bucket-card:hover {
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}
</style>
