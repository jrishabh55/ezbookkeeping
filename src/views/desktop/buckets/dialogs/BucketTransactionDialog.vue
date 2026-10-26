<template>
    <v-dialog width="500" :persistent="submitting" v-model="showState">
        <one-column-dialog-layout :disabled="submitting" :title="tt(title)" :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content>
                <v-alert variant="tonal" density="compact" class="mb-4" v-if="bucket">
                    {{ tt('Current Balance') }}: {{ formatAmount(bucket.currentBalance, bucket.currency) }}
                </v-alert>

                <v-form>
                    <v-row>
                        <v-col cols="12">
                            <amount-input
                                persistent-placeholder
                                autofocus
                                :disabled="submitting"
                                :currency="bucket?.currency"
                                :show-currency="true"
                                :label="tt('Amount')"
                                :placeholder="tt('Amount')"
                                v-model="amount"
                            />
                        </v-col>
                        <v-col cols="12">
                            <v-textarea
                                type="text"
                                autocomplete="off"
                                persistent-placeholder
                                rows="3"
                                :disabled="submitting"
                                :label="tt('Comment')"
                                :placeholder="tt('Comment')"
                                v-model="comment"
                            />
                        </v-col>
                    </v-row>
                </v-form>
            </template>

            <template #footer>
                <v-spacer/>
                <v-tooltip :disabled="!inputIsEmpty" :text="inputEmptyProblemMessage ? tt(inputEmptyProblemMessage) : ''">
                    <template v-slot:activator="{ props }">
                        <div v-bind="props" class="d-inline-block">
                            <v-btn :disabled="inputIsEmpty || submitting" @click="save">
                                {{ tt(saveButtonTitle) }}
                                <v-progress-circular indeterminate size="22" class="ms-2" v-if="submitting"></v-progress-circular>
                            </v-btn>
                        </div>
                    </template>
                </v-tooltip>
            </template>
        </one-column-dialog-layout>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import SnackBar from '@/components/desktop/SnackBar.vue';

import { ref, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useBucketTransactionPageBase } from '@/views/base/buckets/BucketTransactionPageBase.ts';

import { useBucketsStore } from '@/stores/bucket.ts';

import { Bucket } from '@/models/bucket.ts';

import { generateRandomUUID } from '@/lib/misc.ts';

interface BucketTransactionResponse {
    message: string;
}

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const {
    bucket,
    transactionType,
    amount,
    comment,
    clientSessionId,
    submitting,
    title,
    saveButtonTitle,
    inputEmptyProblemMessage,
    inputIsEmpty,
    buildAllocateRequest,
    buildWithdrawRequest,
    formatAmount
} = useBucketTransactionPageBase();

const bucketsStore = useBucketsStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

let resolveFunc: ((value: BucketTransactionResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);

function open(options: { bucket: Bucket; type: 'allocate' | 'withdraw' }): Promise<BucketTransactionResponse> {
    showState.value = true;
    submitting.value = false;
    bucket.value = options.bucket;
    transactionType.value = options.type;
    amount.value = 0;
    comment.value = '';
    clientSessionId.value = generateRandomUUID();

    return new Promise((resolve, reject) => {
        resolveFunc = resolve;
        rejectFunc = reject;
    });
}

function save(): void {
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        snackbar.value?.showMessage(problemMessage);
        return;
    }

    submitting.value = true;

    const promise = transactionType.value === 'allocate'
        ? bucketsStore.allocateToBucket({ request: buildAllocateRequest() })
        : bucketsStore.withdrawFromBucket({ request: buildWithdrawRequest() });

    promise.then(() => {
        submitting.value = false;

        const message = transactionType.value === 'allocate' ? 'Money has been added to the bucket' : 'Money has been withdrawn from the bucket';

        resolveFunc?.({ message });
        showState.value = false;
    }).catch(error => {
        submitting.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function cancel(): void {
    rejectFunc?.();
    showState.value = false;
}

defineExpose({
    open
});
</script>
