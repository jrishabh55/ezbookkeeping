import { ref, computed } from 'vue';
import { defineStore } from 'pinia';

import {
    type BucketCreateRequest,
    type BucketModifyRequest,
    type BucketNewDisplayOrderRequest,
    type BucketAllocateRequest,
    type BucketWithdrawRequest,
    Bucket,
    BucketTransaction
} from '@/models/bucket.ts';

import services from '@/lib/services.ts';
import logger from '@/lib/logger.ts';

export const useBucketsStore = defineStore('buckets', () => {
    const allBuckets = ref<Bucket[]>([]);
    const allBucketsMap = ref<Record<string, Bucket>>({});
    const bucketListStateInvalid = ref<boolean>(true);

    const allVisibleBuckets = computed<Bucket[]>(() => {
        return allBuckets.value.filter(bucket => !bucket.hidden);
    });

    const allAvailableBucketsCount = computed<number>(() => {
        return allBuckets.value.length;
    });

    const allVisibleBucketsCount = computed<number>(() => {
        return allVisibleBuckets.value.length;
    });

    function loadBucketList(buckets: Bucket[]): void {
        allBuckets.value = buckets;
        allBucketsMap.value = {};

        for (const bucket of buckets) {
            allBucketsMap.value[bucket.id] = bucket;
        }
    }

    function addBucketToBucketList(bucket: Bucket): void {
        allBuckets.value.push(bucket);
        allBucketsMap.value[bucket.id] = bucket;
    }

    function updateBucketInBucketList(bucket: Bucket): void {
        for (let i = 0; i < allBuckets.value.length; i++) {
            if (allBuckets.value[i]?.id === bucket.id) {
                allBuckets.value[i] = bucket;
                break;
            }
        }
        allBucketsMap.value[bucket.id] = bucket;
    }

    function removeBucketFromBucketList(bucketId: string): void {
        for (let i = 0; i < allBuckets.value.length; i++) {
            if (allBuckets.value[i]?.id === bucketId) {
                allBuckets.value.splice(i, 1);
                break;
            }
        }
        delete allBucketsMap.value[bucketId];
    }

    function updateBucketDisplayOrderInBucketList(buckets: Bucket[]): void {
        for (const bucket of buckets) {
            const existingBucket = allBucketsMap.value[bucket.id];
            if (existingBucket) {
                existingBucket.displayOrder = bucket.displayOrder;
            }
        }

        allBuckets.value.sort((a, b) => a.displayOrder - b.displayOrder);
    }

    function updateBucketVisibilityInBucketList(bucketId: string, hidden: boolean): void {
        const bucket = allBucketsMap.value[bucketId];
        if (bucket) {
            bucket.hidden = hidden;
        }
    }

    function resetBuckets(): void {
        allBuckets.value = [];
        allBucketsMap.value = {};
        bucketListStateInvalid.value = true;
    }

    function loadAllBuckets({ force }: { force: boolean }): Promise<Bucket[]> {
        return new Promise((resolve, reject) => {
            if (!force && !bucketListStateInvalid.value) {
                resolve(allBuckets.value);
                return;
            }

            services.getAllBuckets().then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve bucket list' });
                    return;
                }

                const buckets: Bucket[] = [];

                for (const bucketResp of data.result) {
                    buckets.push(Bucket.fromResponse(bucketResp));
                }

                loadBucketList(buckets);
                bucketListStateInvalid.value = false;

                resolve(buckets);
            }).catch(error => {
                logger.error('failed to load all buckets', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve bucket list' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function getBucket({ bucketId }: { bucketId: string }): Promise<Bucket> {
        return new Promise((resolve, reject) => {
            services.getBucket({ id: bucketId }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve bucket' });
                    return;
                }

                const bucket = Bucket.fromResponse(data.result);
                resolve(bucket);
            }).catch(error => {
                logger.error('failed to get bucket', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve bucket' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function saveBucket({ bucket, isEdit }: { bucket: BucketCreateRequest | BucketModifyRequest, isEdit: boolean }): Promise<Bucket> {
        return new Promise((resolve, reject) => {
            let promise;

            if (!isEdit) {
                promise = services.addBucket(bucket as BucketCreateRequest);
            } else {
                promise = services.modifyBucket(bucket as BucketModifyRequest);
            }

            promise.then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to save bucket' });
                    return;
                }

                const savedBucket = Bucket.fromResponse(data.result);

                if (!isEdit) {
                    addBucketToBucketList(savedBucket);
                } else {
                    updateBucketInBucketList(savedBucket);
                }

                resolve(savedBucket);
            }).catch(error => {
                logger.error('failed to save bucket', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to save bucket' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function changeBucketDisplayOrder({ newDisplayOrders }: { newDisplayOrders: BucketNewDisplayOrderRequest[] }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.moveBucket({ newDisplayOrders }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to move bucket' });
                    return;
                }

                const buckets: Bucket[] = [];

                for (const order of newDisplayOrders) {
                    buckets.push(new Bucket(
                        order.id,
                        '',
                        0,
                        0,
                        '',
                        1,
                        '1',
                        '000000',
                        '',
                        order.displayOrder,
                        false,
                        0
                    ));
                }

                updateBucketDisplayOrderInBucketList(buckets);

                resolve(true);
            }).catch(error => {
                logger.error('failed to move bucket', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to move bucket' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function hideBucket({ bucket, hidden }: { bucket: Bucket, hidden: boolean }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.hideBucket({ id: bucket.id, hidden }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to hide bucket' });
                    return;
                }

                updateBucketVisibilityInBucketList(bucket.id, hidden);

                resolve(true);
            }).catch(error => {
                logger.error('failed to hide bucket', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to hide bucket' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function deleteBucket({ bucket }: { bucket: Bucket }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.deleteBucket({ id: bucket.id }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to delete bucket' });
                    return;
                }

                removeBucketFromBucketList(bucket.id);

                resolve(true);
            }).catch(error => {
                logger.error('failed to delete bucket', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to delete bucket' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function allocateToBucket({ request }: { request: BucketAllocateRequest }): Promise<BucketTransaction> {
        return new Promise((resolve, reject) => {
            services.allocateToBucket(request).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to allocate to bucket' });
                    return;
                }

                const transaction = BucketTransaction.fromResponse(data.result);

                // Update bucket balance in list
                const bucket = allBucketsMap.value[request.bucketId];
                if (bucket) {
                    bucket.currentBalance += request.amount;
                    if (bucket.targetAmount > 0) {
                        bucket.progress = (bucket.currentBalance / bucket.targetAmount) * 100;
                    }
                }

                resolve(transaction);
            }).catch(error => {
                logger.error('failed to allocate to bucket', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to allocate to bucket' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function withdrawFromBucket({ request }: { request: BucketWithdrawRequest }): Promise<BucketTransaction> {
        return new Promise((resolve, reject) => {
            services.withdrawFromBucket(request).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to withdraw from bucket' });
                    return;
                }

                const transaction = BucketTransaction.fromResponse(data.result);

                // Update bucket balance in list
                const bucket = allBucketsMap.value[request.bucketId];
                if (bucket) {
                    bucket.currentBalance -= request.amount;
                    if (bucket.targetAmount > 0) {
                        bucket.progress = (bucket.currentBalance / bucket.targetAmount) * 100;
                    }
                }

                resolve(transaction);
            }).catch(error => {
                logger.error('failed to withdraw from bucket', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to withdraw from bucket' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function getBucketTransactions({ bucketId, maxTime, minTime, count }: { bucketId: string, maxTime?: number, minTime?: number, count?: number }): Promise<BucketTransaction[]> {
        return new Promise((resolve, reject) => {
            services.getBucketTransactions({ bucketId, maxTime, minTime, count }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to retrieve bucket transactions' });
                    return;
                }

                const transactions: BucketTransaction[] = [];

                for (const transactionResp of data.result) {
                    transactions.push(BucketTransaction.fromResponse(transactionResp));
                }

                resolve(transactions);
            }).catch(error => {
                logger.error('failed to get bucket transactions', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to retrieve bucket transactions' });
                } else {
                    reject(error);
                }
            });
        });
    }

    function deleteBucketTransaction({ transactionId }: { transactionId: string }): Promise<boolean> {
        return new Promise((resolve, reject) => {
            services.deleteBucketTransaction({ id: transactionId }).then(response => {
                const data = response.data;

                if (!data || !data.success || !data.result) {
                    reject({ message: 'Unable to delete bucket transaction' });
                    return;
                }

                resolve(true);
            }).catch(error => {
                logger.error('failed to delete bucket transaction', error);

                if (error.response && error.response.data && error.response.data.errorMessage) {
                    reject({ error: error.response.data });
                } else if (!error.processed) {
                    reject({ message: 'Unable to delete bucket transaction' });
                } else {
                    reject(error);
                }
            });
        });
    }

    return {
        allBuckets,
        allBucketsMap,
        bucketListStateInvalid,
        allVisibleBuckets,
        allAvailableBucketsCount,
        allVisibleBucketsCount,
        resetBuckets,
        loadAllBuckets,
        getBucket,
        saveBucket,
        changeBucketDisplayOrder,
        hideBucket,
        deleteBucket,
        allocateToBucket,
        withdrawFromBucket,
        getBucketTransactions,
        deleteBucketTransaction
    };
});
