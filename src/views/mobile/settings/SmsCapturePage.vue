<template>
    <f7-page @page:afterin="onPageAfterIn" @page:beforeout="onPageBeforeOut">
        <f7-navbar :class="{ 'disabled': loading }" :title="tt('SMS Auto-capture')" :back-link="tt('Back')"></f7-navbar>

        <f7-block-footer class="padding-horizontal margin-top-half">
            {{ tt('Automatically record bank SMS alerts forwarded from your phone as transactions.') }}
        </f7-block-footer>

        <f7-list strong inset dividers class="margin-vertical-half skeleton-text" v-if="loading">
            <f7-list-item title="Status" after="Unknown"></f7-list-item>
            <f7-list-item title="Last Received" after="Unknown"></f7-list-item>
            <f7-list-item title="Last Outcome" after="Unknown"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-vertical-half" v-else-if="!loading">
            <f7-list-item :title="tt('Status')" :after="tt(status && status.configured ? 'Enabled' : 'Not Set Up')"></f7-list-item>
            <f7-list-item :title="tt('Last Received')" :after="lastReceivedDisplay"></f7-list-item>
            <f7-list-item :title="tt('Last Outcome')" :after="lastOutcomeDisplay"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-vertical" :class="{ 'disabled': loading }">
            <f7-list-button :class="{ 'disabled': settingUp }" @click="setUp(null)">{{ tt('Set Up') }}</f7-list-button>
            <f7-list-button :class="{ 'disabled': !status || !status.shortcutUrl }" @click="installShortcut">{{ tt('Install Shortcut') }}</f7-list-button>
            <f7-list-button :class="{ 'disabled': !status || !status.configured || testing }" @click="sendTest">{{ tt('Send Test') }}</f7-list-button>
            <f7-list-button color="red" :class="{ 'disabled': !status || !status.configured || revoking }" @click="revoke">{{ tt('Revoke') }}</f7-list-button>
        </f7-list>

        <f7-block-footer class="padding-horizontal" v-if="!loading && (!status || !status.shortcutUrl)">
            {{ tt('The shortcut link is not available yet.') }}
        </f7-block-footer>

        <f7-block-footer class="padding-horizontal" v-if="testing">
            {{ tt('Waiting for a test SMS to arrive…') }}
        </f7-block-footer>

        <f7-block strong outline class="margin-vertical">
            <ol class="padding-inline-start no-margin">
                <li>{{ tt('Tap "Install Shortcut" above and add it to your iPhone.') }}</li>
                <li>{{ tt('When prompted, paste your SMS Auto-capture token as the setup code.') }}</li>
                <li>{{ tt('In the Shortcuts app, create an Automation: Message → Message Contains your bank\'s sender → Run Immediately → Run Shortcut ("Record bank SMS").') }}</li>
                <li>{{ tt('Turn off "Ask Before Running" so new bank SMS are captured automatically.') }}</li>
            </ol>
        </f7-block>

        <password-input-sheet :title="tt('Set Up')"
                              :hint="tt('Your current password is required to set up SMS Auto-capture.')"
                              :confirm-disabled="settingUp"
                              :cancel-disabled="settingUp"
                              v-model:show="showPasswordSheet"
                              v-model="currentPassword"
                              @password:confirm="setUp">
        </password-input-sheet>

        <information-sheet class="token-information-sheet"
                           :title="tt('SMS Auto-capture Token')"
                           :hint="tt('This token is shown only once. Copy it now and paste it as the setup code when you install the shortcut. It will not be shown again.')"
                           :information="generatedToken"
                           :row-count="3"
                           :enable-copy="true"
                           v-model:show="showTokenSheet"
                           @info:copied="onTokenCopied">
        </information-sheet>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';

import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';

import { useAlertsStore } from '@/stores/alert.ts';
import type { AlertStatusResponse } from '@/models/alert.ts';

const TEST_POLL_INTERVAL_MILLS = 3000;
const TEST_POLL_MAX_DURATION_MILLS = 2 * 60 * 1000;

const props = defineProps<{
    f7router: Router.Router;
}>();

const { tt, formatDateTimeToLongDateTime } = useI18n();
const { showToast, showConfirm, openExternalUrl, routeBackOnError } = useI18nUIComponents();

const alertsStore = useAlertsStore();

const loading = ref<boolean>(true);
const loadingError = ref<unknown | null>(null);
const status = ref<AlertStatusResponse | null>(null);
const settingUp = ref<boolean>(false);
const revoking = ref<boolean>(false);
const testing = ref<boolean>(false);
const currentPassword = ref<string>('');
const generatedToken = ref<string>('');
const showPasswordSheet = ref<boolean>(false);
const showTokenSheet = ref<boolean>(false);

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

function loadStatus(silent?: boolean): void {
    if (!silent) {
        loading.value = true;
    }

    alertsStore.getAlertStatus().then(response => {
        status.value = response;
        loading.value = false;
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            if (!silent) {
                loadingError.value = error;
            }

            loading.value = false;
            showToast(error.message || error);
        }
    });
}

function setUp(password: string | null): void {
    if (!password) {
        currentPassword.value = '';
        showPasswordSheet.value = true;
        return;
    }

    settingUp.value = true;
    showLoading(() => settingUp.value);

    alertsStore.createAlertToken({ password }).then(response => {
        settingUp.value = false;
        currentPassword.value = '';
        hideLoading();

        showPasswordSheet.value = false;
        generatedToken.value = response.token;
        showTokenSheet.value = true;

        loadStatus(true);
    }).catch(error => {
        settingUp.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onTokenCopied(): void {
    showToast('Token copied');
}

function installShortcut(): void {
    if (!status.value || !status.value.shortcutUrl) {
        return;
    }

    openExternalUrl(status.value.shortcutUrl);
}

function sendTest(): void {
    if (!status.value || !status.value.configured || testing.value) {
        return;
    }

    const lastReceivedAtAtStart = status.value.lastReceivedAt;
    testing.value = true;

    stopTestPoll();

    const startTime = Date.now();

    testPollTimer = setInterval(() => {
        if (Date.now() - startTime >= TEST_POLL_MAX_DURATION_MILLS) {
            stopTestPoll();
            testing.value = false;
            showToast('No test SMS received within 2 minutes');
            return;
        }

        alertsStore.getAlertStatus().then(response => {
            const newStatus = response;
            status.value = newStatus;

            if (newStatus.lastReceivedAt > lastReceivedAtAtStart) {
                stopTestPoll();
                testing.value = false;
                showToast(outcomeDisplayName(newStatus.lastOutcome));
            }
        }).catch(error => {
            if (!error.processed) {
                showToast(error.message || error);
            }
        });
    }, TEST_POLL_INTERVAL_MILLS);
}

function stopTestPoll(): void {
    if (testPollTimer !== null) {
        clearInterval(testPollTimer);
        testPollTimer = null;
    }
}

function revoke(): void {
    if (!status.value || !status.value.configured || revoking.value) {
        return;
    }

    showConfirm('Are you sure you want to revoke the SMS Auto-capture token?', () => {
        revoking.value = true;
        showLoading(() => revoking.value);

        alertsStore.revokeAlertToken().then(() => {
            revoking.value = false;
            hideLoading();

            showToast('SMS Auto-capture has been revoked');
            loadStatus(true);
        }).catch(error => {
            revoking.value = false;
            hideLoading();

            if (!error.processed) {
                showToast(error.message || error);
            }
        });
    });
}

function clearSensitiveState(): void {
    generatedToken.value = '';
    currentPassword.value = '';
    showTokenSheet.value = false;
    showPasswordSheet.value = false;
    stopTestPoll();
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

function onPageBeforeOut(): void {
    clearSensitiveState();
}

onBeforeUnmount(() => {
    clearSensitiveState();
});

loadStatus();
</script>
