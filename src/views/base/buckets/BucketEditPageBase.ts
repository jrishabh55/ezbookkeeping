import { ref, computed } from 'vue';

import { useUserStore } from '@/stores/user.ts';

import type { ColorValue } from '@/core/color.ts';
import { Bucket, BucketStatus } from '@/models/bucket.ts';

import { isValidBucketColor } from '@/lib/bucket.ts';

// matches the color the current (pre-refactor) bucket dialogs default a new bucket to;
// intentionally not the model's own createNewBucket() default ('000000' / DEFAULT_ICON_COLOR),
// which other code may still rely on
const DEFAULT_NEW_BUCKET_COLOR: ColorValue = '4CAF50';

function createNewBucket(currency: string): Bucket {
    const newBucket = Bucket.createNewBucket(currency);
    newBucket.color = DEFAULT_NEW_BUCKET_COLOR;
    return newBucket;
}

export function useBucketEditPageBase() {
    const userStore = useUserStore();

    const editBucketId = ref<string | null>(null);
    const clientSessionId = ref<string>('');
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const bucket = ref<Bucket>(createNewBucket(userStore.currentUserDefaultCurrency));

    // resets to a brand new bucket (add mode); always gives it the default colour, never a stale one from a previous open
    function prepareNewBucket(currency: string): void {
        bucket.value = createNewBucket(currency);
    }

    const title = computed<string>(() => editBucketId.value ? 'Edit Bucket' : 'Add Bucket');
    const saveButtonTitle = computed<string>(() => editBucketId.value ? 'Save' : 'Add');

    const inputEmptyProblemMessage = computed<string | null>(() => {
        if (!bucket.value.name) {
            return 'Bucket name cannot be blank';
        } else if (!bucket.value.currency) {
            return 'Currency cannot be blank';
        } else if (bucket.value.targetAmount < 0) {
            return 'Target amount cannot be negative';
        } else if (!isValidBucketColor(bucket.value.color)) {
            return 'Bucket color is invalid';
        } else {
            return null;
        }
    });

    const inputIsEmpty = computed<boolean>(() => !!inputEmptyProblemMessage.value);

    const allStatuses: { type: number, name: string }[] = [
        BucketStatus.InProgress,
        BucketStatus.Completed
    ];

    function setBucket(source: Bucket): void {
        bucket.value = source.clone();
    }

    return {
        // states
        editBucketId,
        clientSessionId,
        loading,
        submitting,
        bucket,
        // computed states
        title,
        saveButtonTitle,
        inputEmptyProblemMessage,
        inputIsEmpty,
        // constants
        allStatuses,
        // functions
        prepareNewBucket,
        setBucket
    };
}
