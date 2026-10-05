<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Buckets')"></f7-nav-title>
            <f7-nav-right>
                <f7-link icon-f7="plus" @click="add"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-card class="skeleton-text" v-if="loading">
            <f7-card-header>
                <div class="bucket-info">
                    <span class="bucket-name">Bucket Name</span>
                    <span class="bucket-status">In Progress</span>
                </div>
            </f7-card-header>
            <f7-card-content>
                <f7-progressbar :progress="0"></f7-progressbar>
                <div class="bucket-details">
                    <span>Current: $0.00</span>
                    <span>Target: $0.00</span>
                </div>
            </f7-card-content>
        </f7-card>

        <f7-list strong inset dividers class="margin-vertical" v-if="!loading && !allBuckets.length">
            <f7-list-item :title="tt('No buckets')"></f7-list-item>
        </f7-list>

        <template v-if="!loading && allBuckets.length">
            <f7-card v-for="bucket in allBuckets" :key="bucket.id">
                <f7-card-header>
                    <div class="bucket-info">
                        <div class="bucket-name-wrapper">
                            <div class="bucket-icon" :style="{ backgroundColor: '#' + bucket.color }"></div>
                            <span class="bucket-name">{{ bucket.name }}</span>
                        </div>
                        <f7-badge :color="bucket.isCompleted() ? 'green' : (bucket.hidden ? 'gray' : 'blue')">
                            {{ bucket.isCompleted() ? tt('Completed') : (bucket.hidden ? tt('Hidden') : tt('In Progress')) }}
                        </f7-badge>
                    </div>
                </f7-card-header>
                <f7-card-content>
                    <div class="bucket-amount-row">
                        <span class="label">{{ tt('Current Balance') }}</span>
                        <span class="value">{{ formatAmount(bucket.currentBalance, bucket.currency) }}</span>
                    </div>
                    <div class="bucket-amount-row" v-if="bucket.targetAmount > 0">
                        <span class="label">{{ tt('Target') }}</span>
                        <span class="value">{{ formatAmount(bucket.targetAmount, bucket.currency) }}</span>
                    </div>
                    <f7-progressbar
                        v-if="bucket.targetAmount > 0"
                        :progress="bucket.progress"
                        :color="bucket.isCompleted() ? 'green' : 'blue'"
                    ></f7-progressbar>
                    <div class="bucket-comment" v-if="bucket.comment">{{ bucket.comment }}</div>
                </f7-card-content>
                <f7-card-footer>
                    <f7-link color="green" @click="allocate(bucket)">{{ tt('Add Money') }}</f7-link>
                    <f7-link color="orange" @click="withdraw(bucket)">{{ tt('Withdraw') }}</f7-link>
                    <f7-link @click="editBucket(bucket)">{{ tt('Edit') }}</f7-link>
                    <f7-link color="red" @click="confirmDelete(bucket)">{{ tt('Delete') }}</f7-link>
                </f7-card-footer>
            </f7-card>
        </template>

        <f7-sheet class="bucket-edit-sheet" :opened="showEditSheet" @sheet:closed="showEditSheet = false">
            <f7-toolbar>
                <div class="left">
                    <f7-link sheet-close>{{ tt('Cancel') }}</f7-link>
                </div>
                <div class="right">
                    <f7-link @click="saveBucket">{{ tt('Save') }}</f7-link>
                </div>
            </f7-toolbar>
            <f7-page-content>
                <f7-list strong-ios outline-ios dividers-ios>
                    <f7-list-input
                        :label="tt('Name')"
                        type="text"
                        :placeholder="tt('Bucket name')"
                        v-model:value="editForm.name"
                    ></f7-list-input>
                    <f7-list-input
                        :label="tt('Target Amount (optional)')"
                        type="number"
                        :placeholder="tt('Amount')"
                        v-model:value="editForm.targetAmount"
                    ></f7-list-input>
                    <f7-list-input
                        :label="tt('Currency')"
                        type="select"
                        v-model:value="editForm.currency"
                    >
                        <option value="USD">USD</option>
                        <option value="EUR">EUR</option>
                        <option value="GBP">GBP</option>
                        <option value="CNY">CNY</option>
                        <option value="JPY">JPY</option>
                        <option value="INR">INR</option>
                    </f7-list-input>
                    <f7-list-input
                        :label="tt('Comment')"
                        type="textarea"
                        :placeholder="tt('Optional comment')"
                        v-model:value="editForm.comment"
                    ></f7-list-input>
                </f7-list>
            </f7-page-content>
        </f7-sheet>

        <f7-sheet class="transaction-sheet" :opened="showTransactionSheet" @sheet:closed="showTransactionSheet = false">
            <f7-toolbar>
                <div class="left">
                    <f7-link sheet-close>{{ tt('Cancel') }}</f7-link>
                </div>
                <div class="right">
                    <f7-link @click="saveTransaction">
                        {{ transactionType === 'allocate' ? tt('Add') : tt('Withdraw') }}
                    </f7-link>
                </div>
            </f7-toolbar>
            <f7-page-content>
                <f7-block-title>
                    {{ transactionType === 'allocate' ? tt('Add Money to Bucket') : tt('Withdraw from Bucket') }}
                </f7-block-title>
                <f7-list strong-ios outline-ios dividers-ios>
                    <f7-list-input
                        :label="tt('Amount')"
                        type="number"
                        :placeholder="tt('Amount')"
                        v-model:value="transactionForm.amount"
                    ></f7-list-input>
                    <f7-list-input
                        :label="tt('Comment (optional)')"
                        type="textarea"
                        :placeholder="tt('Optional comment')"
                        v-model:value="transactionForm.comment"
                    ></f7-list-input>
                </f7-list>
                <f7-block v-if="transactionBucket">
                    <p>{{ tt('Current Balance') }}: {{ formatAmount(transactionBucket.currentBalance, transactionBucket.currency) }}</p>
                </f7-block>
            </f7-page-content>
        </f7-sheet>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';

import { useBucketsStore } from '@/stores/bucket.ts';
import { useUserStore } from '@/stores/user.ts';

import { Bucket, BucketStatus } from '@/models/bucket.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

const { tt } = useI18n();
const { showConfirm, showToast } = useI18nUIComponents();
const bucketsStore = useBucketsStore();
const userStore = useUserStore();

const loading = ref(false);
const showEditSheet = ref(false);
const showTransactionSheet = ref(false);
const isEditMode = ref(false);
const editingBucket = ref<Bucket | null>(null);
const transactionBucket = ref<Bucket | null>(null);
const transactionType = ref<'allocate' | 'withdraw'>('allocate');

const editForm = ref({
    name: '',
    targetAmount: 0,
    currency: 'USD',
    comment: ''
});

const transactionForm = ref({
    amount: 0,
    comment: ''
});

const allBuckets = computed(() => bucketsStore.allBuckets);

function formatAmount(amount: number, currency: string): string {
    const displayAmount = amount / 100;
    return new Intl.NumberFormat(undefined, {
        style: 'currency',
        currency: currency || 'USD'
    }).format(displayAmount);
}

function onPageAfterIn(): void {
    if (bucketsStore.bucketListStateInvalid) {
        reload(null);
    }
}

function reload(done: (() => void) | null): void {
    loading.value = true;
    bucketsStore.loadAllBuckets({ force: true }).finally(() => {
        loading.value = false;
        if (done) {
            done();
        }
    });
}

function add(): void {
    editingBucket.value = null;
    isEditMode.value = false;
    editForm.value = {
        name: '',
        targetAmount: 0,
        currency: userStore.currentUserDefaultCurrency || 'USD',
        comment: ''
    };
    showEditSheet.value = true;
}

function editBucket(bucket: Bucket): void {
    editingBucket.value = bucket;
    isEditMode.value = true;
    editForm.value = {
        name: bucket.name,
        targetAmount: bucket.targetAmount / 100,
        currency: bucket.currency,
        comment: bucket.comment
    };
    showEditSheet.value = true;
}

function saveBucket(): void {
    if (!editForm.value.name) {
        showToast('Name is required');
        return;
    }

    // the mobile form only edits a few fields, keep the rest from the existing bucket
    const existing = isEditMode.value ? editingBucket.value : null;

    const request = {
        id: existing ? existing.id : undefined,
        name: editForm.value.name,
        targetAmount: Math.round(editForm.value.targetAmount * 100),
        currency: editForm.value.currency,
        icon: existing ? existing.icon : '1',
        color: existing ? existing.color : '4CAF50',
        comment: editForm.value.comment,
        status: existing ? existing.status : BucketStatus.InProgress.type,
        visible: existing ? !existing.hidden : true,
        clientSessionId: generateRandomUUID()
    };

    showLoading();
    bucketsStore.saveBucket({ bucket: request, isEdit: isEditMode.value })
        .then(() => {
            showEditSheet.value = false;
            showToast('Bucket saved');
        })
        .catch(error => {
            showToast(error.message || 'Unable to save bucket');
        })
        .finally(() => {
            hideLoading();
        });
}

function allocate(bucket: Bucket): void {
    transactionBucket.value = bucket;
    transactionType.value = 'allocate';
    transactionForm.value = { amount: 0, comment: '' };
    showTransactionSheet.value = true;
}

function withdraw(bucket: Bucket): void {
    transactionBucket.value = bucket;
    transactionType.value = 'withdraw';
    transactionForm.value = { amount: 0, comment: '' };
    showTransactionSheet.value = true;
}

function saveTransaction(): void {
    if (!transactionBucket.value || transactionForm.value.amount <= 0) {
        showToast('Amount is required');
        return;
    }

    const request = {
        bucketId: transactionBucket.value.id,
        amount: Math.round(transactionForm.value.amount * 100),
        transactionTime: Math.floor(Date.now() / 1000),
        utcOffset: new Date().getTimezoneOffset() * -1,
        comment: transactionForm.value.comment,
        clientSessionId: generateRandomUUID()
    };

    showLoading();
    const promise = transactionType.value === 'allocate'
        ? bucketsStore.allocateToBucket({ request })
        : bucketsStore.withdrawFromBucket({ request });

    promise
        .then(() => {
            showTransactionSheet.value = false;
            showToast('Transaction saved');
        })
        .catch(error => {
            showToast(error.message || (transactionType.value === 'allocate' ? 'Unable to allocate to bucket' : 'Unable to withdraw from bucket'));
        })
        .finally(() => {
            hideLoading();
        });
}

function confirmDelete(bucket: Bucket): void {
    showConfirm('Are you sure you want to delete this bucket?', () => deleteBucket(bucket));
}

function deleteBucket(bucket: Bucket): void {
    showLoading();
    bucketsStore.deleteBucket({ bucket })
        .then(() => {
            showToast('Bucket deleted');
        })
        .catch(error => {
            showToast(error.message || 'Unable to delete bucket');
        })
        .finally(() => {
            hideLoading();
        });
}

onMounted(() => {
    reload(null);
});
</script>

<style scoped>
.bucket-info {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
}

.bucket-name-wrapper {
    display: flex;
    align-items: center;
    gap: 8px;
}

.bucket-icon {
    width: 24px;
    height: 24px;
    border-radius: 50%;
}

.bucket-name {
    font-weight: 600;
}

.bucket-amount-row {
    display: flex;
    justify-content: space-between;
    margin-bottom: 8px;
}

.bucket-amount-row .label {
    color: var(--f7-list-item-footer-text-color);
}

.bucket-amount-row .value {
    font-weight: 600;
}

.bucket-comment {
    margin-top: 8px;
    font-size: 12px;
    color: var(--f7-list-item-footer-text-color);
}
</style>
