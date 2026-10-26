<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt(title)"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-item class="list-item-with-header-and-title" header="Current Balance" title="$0.00"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title" header="Amount" title="$0.00"></f7-list-item>
        </f7-list>

        <f7-list form strong inset dividers class="margin-vertical" v-else-if="!loading">
            <f7-list-item class="list-item-with-header-and-title list-item-no-item-after"
                          :header="tt('Current Balance')"
                          :title="bucket ? formatAmount(bucket.currentBalance, bucket.currency) : ''"
            ></f7-list-item>

            <f7-list-item
                link="#" no-chevron
                class="list-item-with-header-and-title"
                :header="tt('Amount')"
                :title="bucket ? formatAmount(amount, bucket.currency) : ''"
                @click="showAmountSheet = true"
            >
                <number-pad-sheet :min-value="0"
                                  :max-value="TRANSACTION_MAX_AMOUNT"
                                  :currency="bucket?.currency"
                                  v-model:show="showAmountSheet"
                                  v-model="amount"
                ></number-pad-sheet>
            </f7-list-item>

            <f7-list-input
                type="textarea"
                style="height: auto"
                autocomplete="off"
                :label="tt('Comment')"
                :placeholder="tt('Comment (optional)')"
                v-textarea-auto-size
                v-model:value="comment"
            ></f7-list-input>
        </f7-list>
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useBucketTransactionPageBase } from '@/views/base/buckets/BucketTransactionPageBase.ts';

import { useBucketsStore } from '@/stores/bucket.ts';

import { TRANSACTION_MAX_AMOUNT } from '@/consts/transaction.ts';

import { generateRandomUUID } from '@/lib/misc.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();

const {
    bucket,
    transactionType,
    amount,
    comment,
    clientSessionId,
    submitting,
    title,
    inputEmptyProblemMessage,
    inputIsEmpty,
    buildAllocateRequest,
    buildWithdrawRequest,
    formatAmount
} = useBucketTransactionPageBase();

const bucketsStore = useBucketsStore();

const loading = ref<boolean>(true);
const loadingError = ref<unknown | null>(null);
const showAmountSheet = ref<boolean>(false);

function init(): void {
    const query = props.f7route.query;
    const bucketId = query['id'];

    loading.value = true;
    transactionType.value = query['type'] === 'withdraw' ? 'withdraw' : 'allocate';
    amount.value = 0;
    comment.value = '';
    clientSessionId.value = generateRandomUUID();

    if (!bucketId) {
        loadingError.value = { message: 'An error occurred' };
        showToast('An error occurred');
        return;
    }

    const existingBucket = bucketsStore.allBucketsMap[bucketId];

    if (existingBucket) {
        bucket.value = existingBucket;
        loading.value = false;
        return;
    }

    bucketsStore.getBucket({
        bucketId: bucketId
    }).then(response => {
        bucket.value = response;
        loading.value = false;
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function save(): void {
    const router = props.f7router;
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        showAlert(problemMessage);
        return;
    }

    submitting.value = true;
    showLoading(() => submitting.value);

    const promise = transactionType.value === 'allocate'
        ? bucketsStore.allocateToBucket({ request: buildAllocateRequest() })
        : bucketsStore.withdrawFromBucket({ request: buildWithdrawRequest() });

    promise.then(() => {
        submitting.value = false;
        hideLoading();

        if (transactionType.value === 'allocate') {
            showToast('Money has been added to the bucket');
        } else {
            showToast('Money has been withdrawn from the bucket');
        }

        router.back();
    }).catch(error => {
        submitting.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
