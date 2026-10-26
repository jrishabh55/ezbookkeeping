import { ref, computed } from 'vue';

import { useUserStore } from '@/stores/user.ts';

import { Bucket, BucketStatus } from '@/models/bucket.ts';

import { isValidBucketColor } from '@/lib/bucket.ts';

export function useBucketEditPageBase() {
    const userStore = useUserStore();

    const editBucketId = ref<string | null>(null);
    const clientSessionId = ref<string>('');
    const loading = ref<boolean>(false);
    const submitting = ref<boolean>(false);
    const bucket = ref<Bucket>(Bucket.createNewBucket(userStore.currentUserDefaultCurrency));

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
        setBucket
    };
}
