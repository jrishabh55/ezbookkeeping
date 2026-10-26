<template>
    <f7-page ptr @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Buckets')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="ellipsis" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
                <f7-link icon-f7="plus" :aria-label="tt('Add')" href="/bucket/add"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="Bucket Name" after="$0.00" :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]">
                <template #footer>
                    <div>Target $0.00 · In Progress</div>
                    <f7-progressbar :progress="0"></f7-progressbar>
                </template>
            </f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && noAvailableBucket">
            <f7-list-item :title="tt('No available bucket')"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half bucket-list" v-if="!loading && !noAvailableBucket">
            <f7-list-item swipeout
                          :id="getBucketDomId(bucket)"
                          :title="bucket.name"
                          :after="formatAmount(bucket.currentBalance, bucket.currency)"
                          :key="bucket.id"
                          v-for="bucket in displayBuckets">
                <template #footer>
                    <div><span v-if="bucket.targetAmount > 0">{{ tt('Target') }}: {{ formatAmount(bucket.targetAmount, bucket.currency) }} · </span>{{ tt(getStatusDisplayName(bucket)) }}<span v-if="bucket.hidden"> · {{ tt('Hidden') }}</span></div>
                    <f7-progressbar :progress="getProgressPercentage(bucket)" :color="bucket.isCompleted() ? 'green' : undefined" v-if="bucket.targetAmount > 0"></f7-progressbar>
                </template>
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR">
                    <f7-swipeout-button color="orange" close :text="tt('Edit')" @click="edit(bucket)"></f7-swipeout-button>
                    <f7-swipeout-button class="padding-horizontal" close :aria-label="tt('More')" @click="openBucketActions(bucket)">
                        <f7-icon f7="ellipsis"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button v-if="!showHidden" @click="showHidden = true">{{ tt('Show Hidden Buckets') }}</f7-actions-button>
                <f7-actions-button v-if="showHidden" @click="showHidden = false">{{ tt('Hide Hidden Buckets') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showBucketActionsSheet" @actions:closed="showBucketActionsSheet = false">
            <f7-actions-group>
                <f7-actions-button @click="allocate(bucketForActions)">{{ tt('Add Money') }}</f7-actions-button>
                <f7-actions-button @click="withdraw(bucketForActions)">{{ tt('Withdraw') }}</f7-actions-button>
                <f7-actions-button color="red" @click="remove(bucketForActions, false)">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this bucket?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(bucketToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
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

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading, onSwipeoutDeleted } from '@/lib/ui/mobile.ts';
import { useBucketListPageBase } from '@/views/base/buckets/BucketListPageBase.ts';

import { useBucketsStore } from '@/stores/bucket.ts';

import { TextDirection } from '@/core/text.ts';
import { Bucket } from '@/models/bucket.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    showHidden,
    displayBuckets,
    noAvailableBucket,
    formatAmount,
    getProgressPercentage,
    getStatusDisplayName
} = useBucketListPageBase();

const bucketsStore = useBucketsStore();

const loadingError = ref<unknown | null>(null);
const bucketToDelete = ref<Bucket | null>(null);
const bucketForActions = ref<Bucket | null>(null);
const showMoreActionSheet = ref<boolean>(false);
const showBucketActionsSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

function getBucketDomId(bucket: Bucket): string {
    return 'bucket_' + bucket.id;
}

function init(): void {
    loading.value = true;

    bucketsStore.loadAllBuckets({ force: false }).then(() => {
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

function reload(done?: () => void): void {
    const force = !!done;

    bucketsStore.loadAllBuckets({ force: force }).then(() => {
        done?.();

        if (force) {
            showToast('Bucket list has been updated');
        }
    }).catch(error => {
        done?.();

        if (error.isUpToDate) {
            showToast('Bucket list is up to date');
        } else if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function edit(bucket: Bucket): void {
    props.f7router.navigate(`/bucket/edit?id=${bucket.id}`);
}

function openBucketActions(bucket: Bucket): void {
    bucketForActions.value = bucket;
    showBucketActionsSheet.value = true;
}

function allocate(bucket: Bucket | null): void {
    if (!bucket) {
        showAlert('An error occurred');
        return;
    }

    props.f7router.navigate(`/bucket/transaction?id=${bucket.id}&type=allocate`);
    showBucketActionsSheet.value = false;
    bucketForActions.value = null;
}

function withdraw(bucket: Bucket | null): void {
    if (!bucket) {
        showAlert('An error occurred');
        return;
    }

    props.f7router.navigate(`/bucket/transaction?id=${bucket.id}&type=withdraw`);
    showBucketActionsSheet.value = false;
    bucketForActions.value = null;
}

function remove(bucket: Bucket | null, confirm: boolean): void {
    if (!bucket) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        bucketToDelete.value = bucket;
        showDeleteActionSheet.value = true;
        showBucketActionsSheet.value = false;
        bucketForActions.value = null;
        return;
    }

    showDeleteActionSheet.value = false;
    bucketToDelete.value = null;
    showLoading();

    bucketsStore.deleteBucket({
        bucket: bucket,
        beforeResolve: (done) => {
            onSwipeoutDeleted(getBucketDomId(bucket), done);
        }
    }).then(() => {
        hideLoading();
    }).catch(error => {
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    if (!loading.value) {
        reload();
    }

    routeBackOnError(props.f7router, loadingError);
}

init();
</script>

<style>
.bucket-list {
    --f7-list-item-footer-font-size: var(--ebk-large-footer-font-size);
}

.bucket-list .item-footer {
    padding-top: 4px;
}
</style>
