<template>
    <v-dialog width="800" :persistent="isBucketModified" v-model="showState">
        <one-column-dialog-layout :disabled="loading || submitting" :loading="loading"
                                  :title="tt(title)" :cancel-button-title="tt('Cancel')"
                                  @cancel="cancel">
            <template #content>
                <v-form class="mt-2">
                    <v-row>
                        <v-col cols="12" md="12">
                            <v-text-field
                                type="text"
                                autocomplete="off"
                                persistent-placeholder
                                :disabled="loading || submitting"
                                :label="tt('Name')"
                                :placeholder="tt('Name')"
                                v-model="bucket.name"
                            />
                        </v-col>
                        <v-col cols="12" md="6">
                            <amount-input
                                persistent-placeholder
                                :disabled="loading || submitting"
                                :currency="bucket.currency"
                                :show-currency="true"
                                :label="tt('Target Amount')"
                                :placeholder="tt('Target Amount')"
                                v-model="bucket.targetAmount"
                            />
                        </v-col>
                        <v-col cols="12" md="6">
                            <currency-select
                                :disabled="loading || submitting"
                                :label="tt('Currency')"
                                :placeholder="tt('Currency')"
                                v-model="bucket.currency"
                            />
                        </v-col>
                        <v-col cols="12" md="6">
                            <color-select :all-system-color-infos="ALL_ACCOUNT_COLORS"
                                          :label="tt('Color')"
                                          :disabled="loading || submitting"
                                          v-model="bucket.color" />
                        </v-col>
                        <v-col cols="12" md="6" v-if="editBucketId">
                            <v-select
                                item-title="name"
                                item-value="type"
                                persistent-placeholder
                                :disabled="loading || submitting"
                                :label="tt('Status')"
                                :placeholder="tt('Status')"
                                :items="localizedStatuses"
                                v-model="bucket.status"
                            />
                        </v-col>
                        <v-col cols="12" md="12">
                            <v-textarea
                                type="text"
                                autocomplete="off"
                                persistent-placeholder
                                rows="3"
                                :disabled="loading || submitting"
                                :label="tt('Comment')"
                                :placeholder="tt('Comment')"
                                v-model="bucket.comment"
                            />
                        </v-col>
                        <v-col class="py-0 my-n2" cols="12" md="12" v-if="editBucketId">
                            <v-switch :disabled="loading || submitting"
                                      :label="tt('Visible')" v-model="bucketVisible"/>
                        </v-col>
                    </v-row>
                </v-form>
            </template>

            <template #footer>
                <v-spacer/>
                <v-tooltip :disabled="!inputIsEmpty" :text="inputEmptyProblemMessage ? tt(inputEmptyProblemMessage) : ''">
                    <template v-slot:activator="{ props }">
                        <div v-bind="props" class="d-inline-block">
                            <v-btn :disabled="inputIsEmpty || loading || submitting" @click="save">
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

import { ref, computed, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';
import { useBucketEditPageBase } from '@/views/base/buckets/BucketEditPageBase.ts';

import { useBucketsStore } from '@/stores/bucket.ts';
import { useUserStore } from '@/stores/user.ts';

import { Bucket } from '@/models/bucket.ts';

import type { ColorValue } from '@/core/color.ts';
import { ALL_ACCOUNT_COLORS } from '@/consts/color.ts';

import { isEquals } from '@/lib/common.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

interface BucketEditResponse {
    message: string;
}

// matches the color a new bucket has always defaulted to in this dialog
// (not the model's own createNewBucket() default, which other code may still rely on)
const DEFAULT_NEW_BUCKET_COLOR: ColorValue = '4CAF50';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const {
    editBucketId,
    clientSessionId,
    loading,
    submitting,
    bucket,
    title,
    saveButtonTitle,
    inputEmptyProblemMessage,
    inputIsEmpty,
    allStatuses,
    setBucket
} = useBucketEditPageBase();

const bucketsStore = useBucketsStore();
const userStore = useUserStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');

let resolveFunc: ((value: BucketEditResponse) => void) | null = null;
let rejectFunc: ((reason?: unknown) => void) | null = null;

const showState = ref<boolean>(false);
const initBucket = ref<Bucket | null>(null);

const isBucketModified = computed<boolean>(() => {
    if (!editBucketId.value) { // Add
        return !!initBucket.value && !isEquals(bucket.value.toCreateRequest(clientSessionId.value), initBucket.value.toCreateRequest(clientSessionId.value));
    } else { // Edit
        return !!initBucket.value && !isEquals(bucket.value.toModifyRequest(), initBucket.value.toModifyRequest());
    }
});

const bucketVisible = computed<boolean>({
    get: () => !bucket.value.hidden,
    set: (value: boolean) => {
        bucket.value.hidden = !value;
    }
});

const localizedStatuses = computed<{ type: number, name: string }[]>(() => allStatuses.map(item => ({ type: item.type, name: tt(item.name) })));

function open(options: { id?: string; currentBucket?: Bucket }): Promise<BucketEditResponse> {
    showState.value = true;
    loading.value = true;
    submitting.value = false;
    editBucketId.value = options.id ?? null;

    initBucket.value = Bucket.createNewBucket(userStore.currentUserDefaultCurrency);
    initBucket.value.color = DEFAULT_NEW_BUCKET_COLOR;
    setBucket(initBucket.value);

    if (options.id) {
        if (options.currentBucket) {
            setBucket(options.currentBucket);
        }

        bucketsStore.getBucket({
            bucketId: options.id
        }).then(response => {
            setBucket(response);
            initBucket.value = response.clone();
            loading.value = false;
        }).catch(error => {
            loading.value = false;
            showState.value = false;

            if (!error.processed) {
                rejectFunc?.(error);
            }
        });
    } else {
        clientSessionId.value = generateRandomUUID();
        loading.value = false;
    }

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

    bucketsStore.saveBucket({
        bucket: editBucketId.value ? bucket.value.toModifyRequest() : bucket.value.toCreateRequest(clientSessionId.value),
        isEdit: !!editBucketId.value
    }).then(() => {
        submitting.value = false;

        const message = editBucketId.value ? 'You have saved this bucket' : 'You have added a new bucket';

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
