<template>
    <v-dialog v-model="showDialog" max-width="600" persistent>
        <v-card>
            <v-card-title>
                {{ isEdit ? tt('Edit Bucket') : tt('Add Bucket') }}
            </v-card-title>

            <v-card-text>
                <v-form ref="form" v-model="formValid">
                    <v-row>
                        <v-col cols="12">
                            <v-text-field
                                v-model="formData.name"
                                :label="tt('Name')"
                                :rules="[rules.required]"
                                variant="outlined"
                                density="compact"
                            />
                        </v-col>

                        <v-col cols="12" md="6">
                            <v-text-field
                                v-model.number="formData.targetAmount"
                                :label="tt('Target Amount (optional)')"
                                type="number"
                                :rules="[rules.nonNegative]"
                                variant="outlined"
                                density="compact"
                            />
                        </v-col>

                        <v-col cols="12" md="6">
                            <v-select
                                v-model="formData.currency"
                                :label="tt('Currency')"
                                :items="currencies"
                                :rules="[rules.required]"
                                variant="outlined"
                                density="compact"
                            />
                        </v-col>

                        <v-col cols="12" md="6">
                            <v-text-field
                                v-model="formData.color"
                                :label="tt('Color')"
                                :rules="[rules.required]"
                                variant="outlined"
                                density="compact"
                            >
                                <template #prepend-inner>
                                    <div
                                        :style="{ backgroundColor: '#' + formData.color, width: '24px', height: '24px', borderRadius: '4px' }"
                                    />
                                </template>
                            </v-text-field>
                        </v-col>

                        <v-col cols="12" md="6" v-if="isEdit">
                            <v-select
                                v-model="formData.status"
                                :label="tt('Status')"
                                :items="statuses"
                                item-title="name"
                                item-value="type"
                                variant="outlined"
                                density="compact"
                            />
                        </v-col>

                        <v-col cols="12">
                            <v-textarea
                                v-model="formData.comment"
                                :label="tt('Comment')"
                                rows="2"
                                variant="outlined"
                                density="compact"
                            />
                        </v-col>

                        <v-col cols="12" v-if="isEdit">
                            <v-switch
                                v-model="formData.visible"
                                :label="tt('Visible')"
                                color="primary"
                                density="compact"
                            />
                        </v-col>
                    </v-row>
                </v-form>
            </v-card-text>

            <v-card-actions>
                <v-spacer />
                <v-btn variant="text" @click="close">{{ tt('Cancel') }}</v-btn>
                <v-btn color="primary" variant="tonal" :loading="saving" :disabled="!formValid" @click="save">
                    {{ tt('Save') }}
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
import { useUserStore } from '@/stores/user.ts';

import { Bucket, BucketStatus } from '@/models/bucket.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

import SnackBar from '@/components/desktop/SnackBar.vue';

type SnackBarType = InstanceType<typeof SnackBar>;

const { t: tt } = useI18n();
const bucketsStore = useBucketsStore();
const userStore = useUserStore();

const props = defineProps<{
    show: boolean;
    bucket: Bucket | null;
    isEdit: boolean;
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

const formData = ref<{
    name: string;
    targetAmount: number;
    currency: string;
    color: string;
    comment: string;
    status: number;
    visible: boolean;
}>({
    name: '',
    targetAmount: 0,
    currency: 'USD',
    color: '4CAF50',
    comment: '',
    status: BucketStatus.InProgress.type,
    visible: true
});

const statuses = [
    BucketStatus.InProgress,
    BucketStatus.Completed
];

const currencies = computed(() => {
    return ['USD', 'EUR', 'GBP', 'CNY', 'JPY', 'INR', 'CAD', 'AUD'];
});

const rules = {
    required: (v: string | number) => !!v || tt('This field is required'),
    nonNegative: (v: number) => v >= 0 || tt('Must be zero or positive')
};

watch(() => props.show, (newVal) => {
    if (newVal && props.bucket) {
        formData.value = {
            name: props.bucket.name,
            targetAmount: props.bucket.targetAmount / 100,
            currency: props.bucket.currency || userStore.currentUserDefaultCurrency || 'USD',
            color: props.bucket.color || '4CAF50',
            comment: props.bucket.comment,
            status: props.bucket.status,
            visible: !props.bucket.hidden
        };
    } else if (newVal) {
        formData.value = {
            name: '',
            targetAmount: 0,
            currency: userStore.currentUserDefaultCurrency || 'USD',
            color: '4CAF50',
            comment: '',
            status: BucketStatus.InProgress.type,
            visible: true
        };
    }
});

function close(): void {
    showDialog.value = false;
}

function save(): void {
    if (!formValid.value) return;

    saving.value = true;

    const request = {
        id: props.isEdit && props.bucket ? props.bucket.id : undefined,
        name: formData.value.name,
        targetAmount: Math.round(formData.value.targetAmount * 100),
        currency: formData.value.currency,
        icon: props.isEdit && props.bucket ? props.bucket.icon : '1',
        color: formData.value.color,
        comment: formData.value.comment,
        status: formData.value.status,
        visible: formData.value.visible,
        clientSessionId: generateRandomUUID()
    };

    bucketsStore.saveBucket({ bucket: request, isEdit: props.isEdit })
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
