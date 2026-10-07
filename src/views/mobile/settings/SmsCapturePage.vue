<template>
    <f7-page @page:afterin="onPageAfterIn" @page:beforeout="onPageBeforeOut">
        <f7-navbar :class="{ 'disabled': loading }" :title="tt('SMS Auto-capture')" :back-link="tt('Back')"></f7-navbar>

        <f7-block-footer class="padding-horizontal margin-top-half">
            {{ tt('Automatically record bank SMS alerts forwarded from your phone as transactions.') }}
        </f7-block-footer>

        <f7-list strong inset dividers class="margin-vertical-half skeleton-text" v-if="loading">
            <f7-list-item title="Status" after="Unknown"></f7-list-item>
            <f7-list-item media-item title="Last Received" text="Unknown"></f7-list-item>
            <f7-list-item title="Last Outcome" after="Unknown"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-vertical-half" v-else-if="!loading">
            <f7-list-item :title="tt('Status')" :after="tt(status && status.configured ? 'Enabled' : 'Not Set Up')"></f7-list-item>
            <f7-list-item media-item :title="tt('Last Received')" :text="lastReceivedDisplay"></f7-list-item>
            <f7-list-item :title="tt('Last Outcome')" :after="lastOutcomeDisplay"></f7-list-item>
        </f7-list>

        <f7-block-title class="margin-top" v-if="!loading && recentUnparsed.length">{{ tt('Couldn\'t read — add these manually') }}</f7-block-title>
        <f7-list strong inset dividers class="margin-vertical-half" v-if="!loading && recentUnparsed.length">
            <f7-list-item media-item :key="idx"
                          :title="item.sender || receivedAtDisplay(item.receivedAt)"
                          :subtitle="item.sender ? receivedAtDisplay(item.receivedAt) : ''"
                          :text="item.text"
                          v-for="(item, idx) in recentUnparsed"></f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-vertical" :class="{ 'disabled': loading }">
            <f7-list-button :class="{ 'disabled': settingUp }" @click="setUp(null)">{{ tt('Set Up') }}</f7-list-button>
            <f7-list-button :class="{ 'disabled': !status || !status.shortcutUrl }" @click="installShortcut">{{ tt('Install Shortcut') }}</f7-list-button>
            <f7-list-button :class="{ 'disabled': !status || !status.configured || testing }" @click="sendTest">{{ tt('Send Test') }}</f7-list-button>
            <f7-list-button color="red" :class="{ 'disabled': !status || !status.configured || revoking }" @click="revoke">{{ tt('Revoke') }}</f7-list-button>
        </f7-list>

        <f7-block-footer class="padding-horizontal margin-top" v-if="!loading && (!status || !status.shortcutUrl)">
            {{ tt('The shortcut link is not available yet.') }}
        </f7-block-footer>

        <f7-block-footer class="padding-horizontal margin-top" v-if="testing">
            {{ tt('Waiting for a test SMS to arrive…') }}
        </f7-block-footer>

        <f7-block strong outline class="margin-vertical">
            <ol class="padding-inline-start no-margin">
                <li>{{ tt('Tap "Install Shortcut" above and add it to your iPhone.') }}</li>
                <li>{{ tt('When prompted, paste your SMS Auto-capture token as the setup code.') }}</li>
                <li>{{ tt('In the Shortcuts app, create two Automations — Message → Message Contains "Rs", and Message → Message Contains "INR" — each set to Run Immediately → Run Shortcut ("Record bank SMS").') }}</li>
                <li>{{ tt('Turn off "Ask Before Running" on both automations so new bank SMS are captured automatically.') }}</li>
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
import { ref, onBeforeUnmount } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';

import { useSmsCapturePageBase } from '@/views/base/settings/SmsCapturePageBase.ts';

const props = defineProps<{
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showToast, showConfirm, openExternalUrl, routeBackOnError } = useI18nUIComponents();

const {
    loading,
    status,
    settingUp,
    revoking,
    testing,
    lastReceivedDisplay,
    lastOutcomeDisplay,
    recentUnparsed,
    receivedAtDisplay,
    loadStatus,
    createToken,
    revokeToken,
    sendTest: sendTestPoll,
    stopTestPoll
} = useSmsCapturePageBase();

const loadingError = ref<unknown | null>(null);
const currentPassword = ref<string>('');
const generatedToken = ref<string>('');
const showPasswordSheet = ref<boolean>(false);
const showTokenSheet = ref<boolean>(false);

function initialLoad(): void {
    loadStatus(true).catch(error => {
        if (!error.processed) {
            loadingError.value = error;
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

    showLoading(() => settingUp.value);

    createToken(password).then(response => {
        currentPassword.value = '';
        hideLoading();

        showPasswordSheet.value = false;
        generatedToken.value = response.token;
        showTokenSheet.value = true;

        loadStatus(false).catch(error => {
            if (!error.processed) {
                showToast(error.message || error);
            }
        });
    }).catch(error => {
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
    sendTestPoll(
        (outcomeText) => showToast(outcomeText),
        () => showToast('No test SMS received within 2 minutes'),
        (error) => {
            const err = error as { processed?: boolean, message?: string };

            if (!err.processed) {
                showToast(err.message || (error as string));
            }
        }
    );
}

function revoke(): void {
    if (!status.value || !status.value.configured || revoking.value) {
        return;
    }

    showConfirm('Are you sure you want to revoke the SMS Auto-capture token?', () => {
        showLoading(() => revoking.value);

        revokeToken().then(() => {
            hideLoading();

            showToast('SMS Auto-capture has been revoked');

            loadStatus(false).catch(error => {
                if (!error.processed) {
                    showToast(error.message || error);
                }
            });
        }).catch(error => {
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

initialLoad();
</script>
