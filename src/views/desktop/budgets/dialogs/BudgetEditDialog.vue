<template>
    <v-dialog v-model="showDialog" max-width="600" persistent>
        <v-card>
            <v-card-title>
                {{ isEdit ? tt('Edit Budget') : tt('Add Budget') }}
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
                                v-model.number="formData.amount"
                                :label="tt('Budget Amount')"
                                :rules="[rules.required, rules.positive]"
                                type="number"
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
                            <v-select
                                v-model="formData.periodType"
                                :label="tt('Period Type')"
                                :items="periodTypes"
                                item-title="name"
                                item-value="type"
                                :rules="[rules.required]"
                                variant="outlined"
                                density="compact"
                            />
                        </v-col>

                        <v-col cols="12" md="6">
                            <v-select
                                v-model="formData.rolloverType"
                                :label="tt('Rollover Type')"
                                :items="rolloverTypes"
                                item-title="name"
                                item-value="type"
                                :rules="[rules.required]"
                                variant="outlined"
                                density="compact"
                            />
                        </v-col>

                        <v-col cols="12" md="6">
                            <v-text-field
                                v-model.number="formData.warningThreshold"
                                :label="tt('Warning Threshold (%)')"
                                type="number"
                                :rules="[rules.percentage]"
                                variant="outlined"
                                density="compact"
                            />
                        </v-col>

                        <v-col cols="12" md="6">
                            <v-text-field
                                v-model.number="formData.alertThreshold"
                                :label="tt('Alert Threshold (%)')"
                                type="number"
                                :rules="[rules.percentage]"
                                variant="outlined"
                                density="compact"
                            />
                        </v-col>

                        <v-col cols="12">
                            <v-switch
                                v-model="formData.autoCreateNextCycle"
                                :label="tt('Auto-create next cycle')"
                                color="primary"
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

import { useBudgetsStore } from '@/stores/budget.ts';
import { useUserStore } from '@/stores/user.ts';

import { Budget, BudgetPeriodType, BudgetRolloverType } from '@/models/budget.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

import SnackBar from '@/components/desktop/SnackBar.vue';

type SnackBarType = InstanceType<typeof SnackBar>;

const { t: tt } = useI18n();
const budgetsStore = useBudgetsStore();
const userStore = useUserStore();

const props = defineProps<{
    show: boolean;
    budget: Budget | null;
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
    amount: number;
    currency: string;
    periodType: number;
    rolloverType: number;
    warningThreshold: number;
    alertThreshold: number;
    autoCreateNextCycle: boolean;
    visible: boolean;
}>({
    name: '',
    amount: 0,
    currency: 'USD',
    periodType: BudgetPeriodType.Monthly.type,
    rolloverType: BudgetRolloverType.Reset.type,
    warningThreshold: 75,
    alertThreshold: 90,
    autoCreateNextCycle: true,
    visible: true
});

const periodTypes = [
    BudgetPeriodType.Weekly,
    BudgetPeriodType.Monthly,
    BudgetPeriodType.Yearly,
    BudgetPeriodType.Custom
];

const rolloverTypes = [
    BudgetRolloverType.Reset,
    BudgetRolloverType.Rollover
];

const currencies = computed(() => {
    return ['USD', 'EUR', 'GBP', 'CNY', 'JPY', 'INR', 'CAD', 'AUD'];
});

const rules = {
    required: (v: string | number) => !!v || tt('This field is required'),
    positive: (v: number) => v > 0 || tt('Must be a positive number'),
    percentage: (v: number) => (v >= 0 && v <= 100) || tt('Must be between 0 and 100')
};

watch(() => props.show, (newVal) => {
    if (newVal && props.budget) {
        formData.value = {
            name: props.budget.name,
            amount: props.budget.amount / 100,
            currency: props.budget.currency || userStore.currentUserDefaultCurrency || 'USD',
            periodType: props.budget.periodType,
            rolloverType: props.budget.rolloverType,
            warningThreshold: props.budget.warningThreshold,
            alertThreshold: props.budget.alertThreshold,
            autoCreateNextCycle: props.budget.autoCreateNextCycle,
            visible: !props.budget.hidden
        };
    } else if (newVal) {
        formData.value = {
            name: '',
            amount: 0,
            currency: userStore.currentUserDefaultCurrency || 'USD',
            periodType: BudgetPeriodType.Monthly.type,
            rolloverType: BudgetRolloverType.Reset.type,
            warningThreshold: 75,
            alertThreshold: 90,
            autoCreateNextCycle: true,
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

    const now = Date.now();
    const startTime = Math.floor(now / 1000);
    const endTime = startTime + 30 * 24 * 60 * 60;

    const request = {
        id: props.isEdit && props.budget ? props.budget.id : undefined,
        name: formData.value.name,
        amount: Math.round(formData.value.amount * 100),
        currency: formData.value.currency,
        periodType: formData.value.periodType,
        startTime: props.isEdit && props.budget ? props.budget.startTime : startTime,
        endTime: props.isEdit && props.budget ? props.budget.endTime : endTime,
        rolloverType: formData.value.rolloverType,
        autoCreateNextCycle: formData.value.autoCreateNextCycle,
        warningThreshold: formData.value.warningThreshold,
        alertThreshold: formData.value.alertThreshold,
        overspentThreshold: props.isEdit && props.budget ? props.budget.overspentThreshold : 100,
        visible: formData.value.visible,
        clientSessionId: generateRandomUUID()
    };

    budgetsStore.saveBudget({ budget: request, isEdit: props.isEdit })
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
