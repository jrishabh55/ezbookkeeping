<template>
    <v-row>
        <v-col cols="12">
            <v-card :class="{ 'disabled': loading }">
                <template #title>
                    <div class="d-flex align-center">
                        <span>{{ tt('SMS Auto-capture') }}</span>
                        <v-btn density="compact" color="default" variant="text" size="24" class="ms-2"
                               :aria-label="tt('Refresh')" :icon="true" :loading="loading" @click="refreshStatus">
                            <template #loader>
                                <v-progress-circular indeterminate size="20"/>
                            </template>
                            <v-icon :icon="mdiRefresh" size="24" />
                            <v-tooltip activator="parent">{{ tt('Refresh') }}</v-tooltip>
                        </v-btn>
                    </div>
                </template>

                <v-card-text class="pt-2">
                    <span class="text-body-large">{{ tt('Automatically record bank SMS alerts forwarded from your phone as transactions.') }}</span>
                </v-card-text>

                <v-divider/>

                <v-card-text class="mt-1">
                    <v-row>
                        <v-col cols="12" sm="4">
                            <div class="text-body-small">{{ tt('Status') }}</div>
                            <div class="text-title-medium">{{ tt(status && status.configured ? 'Enabled' : 'Not Set Up') }}</div>
                        </v-col>
                        <v-col cols="12" sm="4">
                            <div class="text-body-small">{{ tt('Last Received') }}</div>
                            <div class="text-title-medium">{{ lastReceivedDisplay }}</div>
                        </v-col>
                        <v-col cols="12" sm="4">
                            <div class="text-body-small">{{ tt('Last Outcome') }}</div>
                            <div class="text-title-medium">{{ lastOutcomeDisplay }}</div>
                        </v-col>
                    </v-row>
                </v-card-text>

                <v-divider/>

                <v-card-text class="d-flex flex-wrap align-center gap-4">
                    <v-btn :disabled="settingUp" @click="openSetupForm"
                           v-if="!showSetupForm && !generatedToken">
                        {{ tt('Set Up') }}
                    </v-btn>

                    <v-btn color="secondary" variant="tonal"
                           :disabled="!status || !status.shortcutUrl"
                           :href="status && status.shortcutUrl ? status.shortcutUrl : undefined"
                           target="_blank" rel="noopener">
                        {{ tt('Install Shortcut') }}
                    </v-btn>

                    <v-btn color="secondary" variant="tonal"
                           :disabled="!status || !status.configured || testing"
                           :loading="testing" @click="sendTest">
                        {{ tt('Send Test') }}
                    </v-btn>

                    <v-btn color="error" variant="tonal"
                           :disabled="!status || !status.configured || revoking"
                           @click="revoke">
                        {{ tt('Revoke') }}
                    </v-btn>
                </v-card-text>

                <v-card-text class="pt-0" v-if="!loading && (!status || !status.shortcutUrl)">
                    <span class="text-body-small">{{ tt('The shortcut link is not available yet.') }}</span>
                </v-card-text>

                <v-card-text class="pt-0" v-if="testing">
                    <span class="text-body-small">{{ tt('Waiting for a test SMS to arrive…') }}</span>
                </v-card-text>

                <div v-if="showSetupForm && !generatedToken">
                    <v-divider/>
                    <v-form @submit.prevent="setUp">
                        <v-card-text>
                            <v-row>
                                <v-col cols="12" md="6">
                                    <v-text-field
                                        autocomplete="current-password"
                                        ref="currentPasswordInput"
                                        type="password"
                                        persistent-placeholder
                                        :autofocus="true"
                                        :disabled="settingUp"
                                        :label="tt('Current Password')"
                                        :placeholder="tt('Current Password')"
                                        v-model="currentPassword"
                                        @keyup.enter="setUp"
                                    />
                                </v-col>
                            </v-row>
                        </v-card-text>
                        <v-card-text class="d-flex flex-wrap gap-4">
                            <v-btn :disabled="!currentPassword || settingUp" @click="setUp">
                                {{ tt('Generate') }}
                                <v-progress-circular indeterminate size="22" class="ms-2" v-if="settingUp"></v-progress-circular>
                            </v-btn>
                            <v-btn variant="tonal" :disabled="settingUp" @click="cancelSetup">{{ tt('Cancel') }}</v-btn>
                        </v-card-text>
                    </v-form>
                </div>

                <div v-if="generatedToken">
                    <v-divider/>
                    <v-card-text>
                        <v-alert type="warning" variant="tonal" class="mb-4">
                            {{ tt('This token is shown only once. Copy it now and paste it as the setup code when you install the shortcut. It will not be shown again.') }}
                        </v-alert>
                        <v-textarea no-resize readonly class="always-cursor-text" :rows="2" :model-value="generatedToken"/>
                    </v-card-text>
                    <v-card-text>
                        <div class="d-flex flex-wrap gap-4" ref="tokenButtonContainer">
                            <v-btn variant="tonal" @click="copyToken">{{ tt('Copy') }}</v-btn>
                            <v-btn variant="text" @click="closeToken">{{ tt('Close') }}</v-btn>
                        </div>
                    </v-card-text>
                </div>
            </v-card>
        </v-col>
    </v-row>

    <confirm-dialog ref="confirmDialog"/>
    <snack-bar ref="snackbar" />
</template>

<script setup lang="ts">
import ConfirmDialog from '@/components/desktop/ConfirmDialog.vue';
import SnackBar from '@/components/desktop/SnackBar.vue';
import { VTextField } from 'vuetify/components/VTextField';

import { ref, onBeforeUnmount, useTemplateRef } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { useSmsCapturePageBase } from '@/views/base/settings/SmsCapturePageBase.ts';

import { copyTextToClipboard } from '@/lib/ui/common.ts';

import { mdiRefresh } from '@mdi/js';

type ConfirmDialogType = InstanceType<typeof ConfirmDialog>;
type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();

const {
    loading,
    status,
    settingUp,
    revoking,
    testing,
    lastReceivedDisplay,
    lastOutcomeDisplay,
    loadStatus,
    createToken,
    revokeToken,
    sendTest: sendTestPoll,
    stopTestPoll
} = useSmsCapturePageBase();

const currentPasswordInput = useTemplateRef<VTextField>('currentPasswordInput');
const confirmDialog = useTemplateRef<ConfirmDialogType>('confirmDialog');
const snackbar = useTemplateRef<SnackBarType>('snackbar');
const tokenButtonContainer = useTemplateRef<HTMLElement>('tokenButtonContainer');

const showSetupForm = ref<boolean>(false);
const currentPassword = ref<string>('');
const generatedToken = ref<string>('');

function refreshStatus(): void {
    loadStatus(true).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function openSetupForm(): void {
    currentPassword.value = '';
    showSetupForm.value = true;

    setTimeout(() => currentPasswordInput.value?.focus(), 0);
}

function cancelSetup(): void {
    currentPassword.value = '';
    showSetupForm.value = false;
}

function setUp(): void {
    if (!currentPassword.value || settingUp.value) {
        return;
    }

    createToken(currentPassword.value).then(response => {
        currentPassword.value = '';
        showSetupForm.value = false;
        generatedToken.value = response.token;

        loadStatus(false).catch(error => {
            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    }).catch(error => {
        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function copyToken(): void {
    copyTextToClipboard(generatedToken.value, tokenButtonContainer.value);
    snackbar.value?.showMessage('Token copied');
}

function closeToken(): void {
    generatedToken.value = '';
}

function sendTest(): void {
    sendTestPoll(
        (outcomeText) => snackbar.value?.showMessage(outcomeText),
        () => snackbar.value?.showMessage('No test SMS received within 2 minutes'),
        (error) => {
            const err = error as { processed?: boolean, message?: string };

            if (!err.processed) {
                snackbar.value?.showError(err.message ? { message: err.message } : (error as string));
            }
        }
    );
}

function revoke(): void {
    if (!status.value || !status.value.configured || revoking.value) {
        return;
    }

    confirmDialog.value?.open('Are you sure you want to revoke the SMS Auto-capture token?').then(() => {
        revokeToken().then(() => {
            snackbar.value?.showMessage('SMS Auto-capture has been revoked');

            loadStatus(false).catch(error => {
                if (!error.processed) {
                    snackbar.value?.showError(error);
                }
            });
        }).catch(error => {
            if (!error.processed) {
                snackbar.value?.showError(error);
            }
        });
    });
}

onBeforeUnmount(() => {
    generatedToken.value = '';
    currentPassword.value = '';
    showSetupForm.value = false;
    stopTestPoll();
});

refreshStatus();
</script>
