import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { type BucketAllocateRequest, type BucketWithdrawRequest, Bucket } from '@/models/bucket.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { getCurrentUnixTime, getTimezoneOffsetMinutes } from '@/lib/datetime.ts';

export function useBucketTransactionPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency } = useI18n();

    const bucket = ref<Bucket | null>(null);
    const transactionType = ref<'allocate' | 'withdraw'>('allocate');
    const amount = ref<number>(0);
    const comment = ref<string>('');
    const clientSessionId = ref<string>('');
    const submitting = ref<boolean>(false);

    const title = computed<string>(() => transactionType.value === 'allocate' ? 'Add Money to Bucket' : 'Withdraw from Bucket');
    const saveButtonTitle = computed<string>(() => transactionType.value === 'allocate' ? 'Add' : 'Withdraw');

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!(amount.value > 0)) {
            return 'Amount must be greater than zero';
        } else if (transactionType.value === 'withdraw' && bucket.value && amount.value > bucket.value.currentBalance) {
            return 'Withdrawal amount exceeds bucket balance';
        } else {
            return null;
        }
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    function buildAllocateRequest(): BucketAllocateRequest {
        const transactionTime = getCurrentUnixTime();

        return {
            bucketId: bucket.value?.id ?? '',
            amount: amount.value,
            transactionTime: transactionTime,
            utcOffset: getTimezoneOffsetMinutes(transactionTime),
            comment: comment.value,
            clientSessionId: clientSessionId.value
        };
    }

    function buildWithdrawRequest(): BucketWithdrawRequest {
        const transactionTime = getCurrentUnixTime();

        return {
            bucketId: bucket.value?.id ?? '',
            amount: amount.value,
            transactionTime: transactionTime,
            utcOffset: getTimezoneOffsetMinutes(transactionTime),
            comment: comment.value,
            clientSessionId: clientSessionId.value
        };
    }

    function formatAmount(value: number, currency: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(value), currency);
    }

    return {
        // states
        bucket,
        transactionType,
        amount,
        comment,
        clientSessionId,
        submitting,
        // computed states
        title,
        saveButtonTitle,
        inputEmptyProblemMessage,
        inputIsEmpty,
        // functions
        buildAllocateRequest,
        buildWithdrawRequest,
        formatAmount
    };
}
