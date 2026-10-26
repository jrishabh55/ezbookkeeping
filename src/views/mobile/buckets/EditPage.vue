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
            <f7-list-input label="Name" placeholder="Bucket name"></f7-list-input>
            <f7-list-item class="list-item-with-header-and-title" header="Target Amount" title="$0.00"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title list-item-no-item-after" header="Currency" title="Currency"></f7-list-item>
        </f7-list>

        <f7-list form strong inset dividers class="margin-vertical" v-else-if="!loading">
            <f7-list-input
                type="text"
                autocomplete="off"
                clear-button
                :label="tt('Name')"
                :placeholder="tt('Name')"
                v-model:value="bucket.name"
            ></f7-list-input>

            <f7-list-item
                link="#" no-chevron
                class="list-item-with-header-and-title"
                :header="tt('Target Amount')"
                :title="formatAmount(bucket.targetAmount, bucket.currency)"
                @click="showAmountSheet = true"
            >
                <number-pad-sheet :min-value="0"
                                  :max-value="TRANSACTION_MAX_AMOUNT"
                                  :currency="bucket.currency"
                                  v-model:show="showAmountSheet"
                                  v-model="bucket.targetAmount"
                ></number-pad-sheet>
            </f7-list-item>

            <f7-list-item
                class="list-item-with-header-and-title list-item-no-item-after"
                link="#"
                :header="tt('Currency')"
                @click="showCurrencyPopup = true"
            >
                <template #title>
                    <div class="no-padding no-margin">
                        <span>{{ getCurrencyName(bucket.currency) }}&nbsp;</span>
                        <small class="smaller">{{ bucket.currency }}</small>
                    </div>
                </template>
                <list-item-selection-popup value-type="item"
                                           key-field="currencyCode" value-field="currencyCode"
                                           title-field="displayName" after-field="currencyCode"
                                           :title="tt('Currency Name')"
                                           :enable-filter="true"
                                           :filter-placeholder="tt('Currency')"
                                           :filter-no-items-text="tt('No results')"
                                           :items="allCurrencies"
                                           v-model:show="showCurrencyPopup"
                                           v-model="bucket.currency">
                </list-item-selection-popup>
            </f7-list-item>

            <f7-list-item
                link="#" no-chevron
                class="list-item-with-header-and-title"
                :header="tt('Color')"
                @click="showColorSelectionSheet = true"
            >
                <template #title>
                    <div class="list-item-custom-title no-padding">
                        <ItemIcon icon-type="fixed-f7" icon-id="app_fill" :color="bucket.color"></ItemIcon>
                    </div>
                </template>
                <color-selection-sheet :all-system-color-infos="ALL_ACCOUNT_COLORS"
                                       v-model:show="showColorSelectionSheet"
                                       v-model="bucket.color">
                </color-selection-sheet>
            </f7-list-item>

            <f7-list-item
                link="#" no-chevron
                class="list-item-with-header-and-title"
                :header="tt('Status')"
                :title="tt(bucket.getStatusName())"
                v-if="editBucketId"
                @click="showStatusSheet = true"
            >
                <list-item-selection-sheet value-type="item"
                                           key-field="type" value-field="type" title-field="name"
                                           :title-i18n="true"
                                           :items="allStatuses"
                                           v-model:show="showStatusSheet"
                                           v-model="bucket.status">
                </list-item-selection-sheet>
            </f7-list-item>

            <f7-list-item :title="tt('Visible')" v-if="editBucketId">
                <template #after>
                    <f7-toggle :checked="bucketVisible" @toggle:change="bucketVisible = $event"></f7-toggle>
                </template>
            </f7-list-item>

            <f7-list-input
                type="textarea"
                style="height: auto"
                autocomplete="off"
                :label="tt('Comment')"
                :placeholder="tt('Comment (optional)')"
                v-textarea-auto-size
                v-model:value="bucket.comment"
            ></f7-list-input>
        </f7-list>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useBucketEditPageBase } from '@/views/base/buckets/BucketEditPageBase.ts';

import { useBucketsStore } from '@/stores/bucket.ts';
import { useUserStore } from '@/stores/user.ts';

import type { LocalizedCurrencyInfo } from '@/core/currency.ts';
import { ALL_ACCOUNT_COLORS } from '@/consts/color.ts';
import { TRANSACTION_MAX_AMOUNT } from '@/consts/transaction.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const {
    tt,
    getAllCurrencies,
    getCurrencyName,
    formatAmountToLocalizedNumeralsWithCurrency
} = useI18n();

const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();

const {
    editBucketId,
    clientSessionId,
    loading,
    submitting,
    bucket,
    title,
    inputEmptyProblemMessage,
    inputIsEmpty,
    allStatuses,
    prepareNewBucket,
    setBucket
} = useBucketEditPageBase();

const bucketsStore = useBucketsStore();
const userStore = useUserStore();

const loadingError = ref<unknown | null>(null);
const showCurrencyPopup = ref<boolean>(false);
const showAmountSheet = ref<boolean>(false);
const showColorSelectionSheet = ref<boolean>(false);
const showStatusSheet = ref<boolean>(false);

const allCurrencies = computed<LocalizedCurrencyInfo[]>(() => getAllCurrencies());

const bucketVisible = computed<boolean>({
    get: () => !bucket.value.hidden,
    set: (value: boolean) => {
        bucket.value.hidden = !value;
    }
});

function formatAmount(amount: number, currency: string): string {
    return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), currency);
}

function init(): void {
    const query = props.f7route.query;
    clientSessionId.value = generateRandomUUID();

    if (query['id']) {
        loading.value = true;

        editBucketId.value = query['id'];

        bucketsStore.getBucket({
            bucketId: editBucketId.value
        }).then(response => {
            setBucket(response);
            loading.value = false;
        }).catch(error => {
            if (error.processed) {
                loading.value = false;
            } else {
                loadingError.value = error;
                showToast(error.message || error);
            }
        });
    } else {
        // always a fresh bucket (default colour), never a stale one from a previous open
        prepareNewBucket(userStore.currentUserDefaultCurrency);
        loading.value = false;
    }
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

    bucketsStore.saveBucket({
        bucket: editBucketId.value ? bucket.value.toModifyRequest() : bucket.value.toCreateRequest(clientSessionId.value),
        isEdit: !!editBucketId.value
    }).then(() => {
        submitting.value = false;
        hideLoading();

        if (!editBucketId.value) {
            showToast('You have added a new bucket');
        } else {
            showToast('You have saved this bucket');
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
