import { ref, computed, onBeforeUnmount } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useAlertsStore } from '@/stores/alert.ts';
import type { AlertTokenCreateResponse, AlertStatusResponse, AlertUnparsedItem } from '@/models/alert.ts';

import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

const TEST_POLL_INTERVAL_MILLS = 3000;
const TEST_POLL_MAX_DURATION_MILLS = 2 * 60 * 1000;

function totalOutcomeCount(counts: Record<string, number> | undefined | null): number {
    if (!counts) {
        return 0;
    }

    let total = 0;

    for (const key of Object.keys(counts)) {
        total += counts[key] || 0;
    }

    return total;
}

export function useSmsCapturePageBase() {
    const { tt, formatDateTimeToLongDateTime } = useI18n();

    const alertsStore = useAlertsStore();

    const loading = ref<boolean>(true);
    const status = ref<AlertStatusResponse | null>(null);
    const settingUp = ref<boolean>(false);
    const revoking = ref<boolean>(false);
    const testing = ref<boolean>(false);

    let testPollTimer: ReturnType<typeof setInterval> | null = null;

    const lastReceivedDisplay = computed<string>(() => {
        if (!status.value || !status.value.lastReceivedAt) {
            return tt('Never');
        }

        return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(status.value.lastReceivedAt));
    });

    const lastOutcomeDisplay = computed<string>(() => {
        if (!status.value || !status.value.lastOutcome) {
            return tt('Never');
        }

        return outcomeDisplayName(status.value.lastOutcome);
    });

    const recentUnparsed = computed<AlertUnparsedItem[]>(() => status.value?.recentUnparsed || []);

    function receivedAtDisplay(receivedAt: number): string {
        return formatDateTimeToLongDateTime(parseDateTimeFromUnixTime(receivedAt));
    }

    function outcomeDisplayName(outcome: string): string {
        if (outcome === 'added') {
            return tt('Added');
        } else if (outcome === 'duplicate') {
            return tt('Duplicate');
        } else if (outcome === 'ignored') {
            return tt('Ignored');
        } else if (outcome === 'unparsed') {
            return tt('Unparsed');
        }

        return outcome;
    }

    function loadStatus(updateLoadingState: boolean): Promise<AlertStatusResponse> {
        return new Promise((resolve, reject) => {
            if (updateLoadingState) {
                loading.value = true;
            }

            alertsStore.getAlertStatus().then(response => {
                status.value = response;

                if (updateLoadingState) {
                    loading.value = false;
                }

                resolve(response);
            }).catch(error => {
                if (updateLoadingState) {
                    loading.value = false;
                }

                reject(error);
            });
        });
    }

    // Callers are expected to follow a successful create/revoke with their own loadStatus(false)
    // call (and their own error handling for that follow-up refresh), the same way platform pages
    // already re-fetch after other mutations - this keeps this composable's promises resolving
    // with exactly one outcome instead of racing a second, internal request.
    function createToken(password: string): Promise<AlertTokenCreateResponse> {
        settingUp.value = true;

        return new Promise((resolve, reject) => {
            alertsStore.createAlertToken({ password }).then(response => {
                settingUp.value = false;
                resolve(response);
            }).catch(error => {
                settingUp.value = false;
                reject(error);
            });
        });
    }

    function revokeToken(): Promise<boolean> {
        revoking.value = true;

        return new Promise((resolve, reject) => {
            alertsStore.revokeAlertToken().then(result => {
                revoking.value = false;
                resolve(result);
            }).catch(error => {
                revoking.value = false;
                reject(error);
            });
        });
    }

    function stopTestPoll(): void {
        if (testPollTimer !== null) {
            clearInterval(testPollTimer);
            testPollTimer = null;
        }
    }

    // Polls getAlertStatus every 3s for up to 2 minutes. Stops as soon as either the newest
    // lastReceivedAt is newer than the value captured when polling started, or the total count
    // of outcomes has gone up (covers the case where a test message arrives within the same
    // second as the value captured at start, which lastReceivedAt alone cannot distinguish).
    function sendTest(onResult: (outcomeText: string) => void, onTimeout: () => void, onPollError?: (error: unknown) => void): void {
        if (!status.value || !status.value.configured || testing.value) {
            return;
        }

        const startLastReceivedAt = status.value.lastReceivedAt;
        const startTotalCount = totalOutcomeCount(status.value.counts);

        testing.value = true;
        stopTestPoll();

        const startTime = Date.now();

        testPollTimer = setInterval(() => {
            if (Date.now() - startTime >= TEST_POLL_MAX_DURATION_MILLS) {
                stopTestPoll();
                testing.value = false;
                onTimeout();
                return;
            }

            alertsStore.getAlertStatus().then(response => {
                status.value = response;

                const newTotalCount = totalOutcomeCount(response.counts);

                if (response.lastReceivedAt > startLastReceivedAt || newTotalCount > startTotalCount) {
                    stopTestPoll();
                    testing.value = false;
                    onResult(outcomeDisplayName(response.lastOutcome));
                }
            }).catch(error => {
                if (onPollError) {
                    onPollError(error);
                }
            });
        }, TEST_POLL_INTERVAL_MILLS);
    }

    onBeforeUnmount(() => {
        stopTestPoll();
    });

    return {
        // states
        loading,
        status,
        settingUp,
        revoking,
        testing,
        // computed states
        lastReceivedDisplay,
        lastOutcomeDisplay,
        recentUnparsed,
        // functions
        outcomeDisplayName,
        receivedAtDisplay,
        loadStatus,
        createToken,
        revokeToken,
        sendTest,
        stopTestPoll
    };
}
