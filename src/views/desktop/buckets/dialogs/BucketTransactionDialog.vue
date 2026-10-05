<template>
    <v-dialog v-model="showDialog" max-width="500" persistent>
        <v-card>
            <v-card-title>
                {{ transactionType === 'allocate' ? tt('Add Money to Bucket') : tt('Withdraw from Bucket') }}
            </v-card-title>

            <v-card-text>
                <v-form ref="form" v-model="formValid">
                    <v-row>
                        <v-col cols="12">
                            <v-text-field
                                v-model.number="formData.amount"
                                :label="tt('Amount')"
                                :rules="[rules.required, rules.positive]"
                                type="number"
                                variant="outlined"
                                density="compact"
                                autofocus
                            />
                        </v-col>

                        <v-col cols="12">
                            <v-textarea
                                v-model="formData.comment"
                                :label="tt('Comment (optional)')"
                                rows="2"
                                variant="outlined"
                                density="compact"
                            />
                        </v-col>
                    </v-row>
                </v-form>

                <v-alert v-if="bucket" type="info" variant="tonal" density="compact" class="mt-2">
                    {{ tt('Current Balance') }}: {{ formatAmount(bucket.currentBalance, bucket.currency) }}
                </v-alert>
            </v-card-text>

            <v-card-actions>
                <v-spacer />
                <v-btn variant="text" @click="close">{{ tt('Cancel') }}</v-btn>
                <v-btn
                    :color="transactionType === 'allocate' ? 'success' : 'warning'"
                    variant="tonal"
                    :loading="saving"
                    :disabled="!formValid"
                    @click="save"
                >
                    {{ transactionType === 'allocate' ? tt('Add') : tt('Withdraw') }}
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import { ref, computed, watch, useTemplateRef } from 'vue';
import { useI18n } from 'vue-i18n';

import { useBucketsStore } from '@/stores/bucket.ts';

import { Bucket } from '@/models/bucket.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

import SnackBar from '@/components/desktop/SnackBar.vue';

type SnackBarType = InstanceType<typeof SnackBar>;

const { t: tt } = useI18n();
const bucketsStore = useBucketsStore();

const props = defineProps<{
    show: boolean;
    bucket: Bucket | null;
    transactionType: 'allocate' | 'withdraw';
}>();

const emit = defineEmits<{
    (e: 'update:show', value: boolean): void;
    (e: 'saved'): void;
}>();

const showDialog = computed({
    get: () => props.show,
    set: (value) => emit('update:show', value)
});

const formValid = ref(false);
const saving = ref(false);
const snackbar = useTemplateRef<SnackBarType>('snackbar');

const formData = ref({
    amount: 0,
    comment: ''
});

const rules = {
    required: (v: number) => !!v || tt('This field is required'),
    positive: (v: number) => v > 0 || tt('Must be a positive number')
};

function formatAmount(amount: number, currency: string): string {
    const displayAmount = amount / 100;
    return new Intl.NumberFormat(undefined, {
        style: 'currency',
        currency: currency || 'USD'
    }).format(displayAmount);
}

watch(() => props.show, (newVal) => {
    if (newVal) {
        formData.value = {
            amount: 0,
            comment: ''
        };
    }
});

function close(): void {
    showDialog.value = false;
}

function save(): void {
    if (!formValid.value || !props.bucket) return;

    saving.value = true;

    const request = {
        bucketId: props.bucket.id,
        amount: Math.round(formData.value.amount * 100),
        transactionTime: Math.floor(Date.now() / 1000),
        utcOffset: new Date().getTimezoneOffset() * -1,
        comment: formData.value.comment,
        clientSessionId: generateRandomUUID()
    };

    const promise = props.transactionType === 'allocate'
        ? bucketsStore.allocateToBucket({ request })
        : bucketsStore.withdrawFromBucket({ request });

    promise
        .then(() => {
            emit('saved');
            close();
        })
        .catch(error => {
            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        })
        .finally(() => {
            saving.value = false;
        });
}
</script>
