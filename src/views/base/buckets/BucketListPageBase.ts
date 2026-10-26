import { ref, computed } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useBucketsStore } from '@/stores/bucket.ts';

import { Bucket } from '@/models/bucket.ts';

import { parseBigDecimal } from '@/lib/numeral.ts';
import { clampBucketProgressPercentage } from '@/lib/bucket.ts';

export function useBucketListPageBase() {
    const { formatAmountToLocalizedNumeralsWithCurrency } = useI18n();

    const bucketsStore = useBucketsStore();

    const loading = ref<boolean>(true);
    const showHidden = ref<boolean>(false);

    const allBuckets = computed<Bucket[]>(() => bucketsStore.allBuckets);
    const displayBuckets = computed<Bucket[]>(() => showHidden.value ? allBuckets.value : allBuckets.value.filter(bucket => !bucket.hidden));
    const noAvailableBucket = computed<boolean>(() => displayBuckets.value.length < 1);

    function formatAmount(amount: number, currency: string): string {
        return formatAmountToLocalizedNumeralsWithCurrency(parseBigDecimal(amount), currency);
    }

    function getProgressPercentage(bucket: Bucket): number {
        return clampBucketProgressPercentage(bucket.progress);
    }

    function getStatusDisplayName(bucket: Bucket): string {
        return bucket.getStatusName();
    }

    return {
        // states
        loading,
        showHidden,
        // computed states
        allBuckets,
        displayBuckets,
        noAvailableBucket,
        // functions
        formatAmount,
        getProgressPercentage,
        getStatusDisplayName
    };
}
