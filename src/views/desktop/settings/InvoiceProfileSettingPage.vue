<template>
    <v-row>
        <v-col cols="12">
            <v-card class="setting-items" :class="{ 'disabled': loading || saving }">
                <template #title>
                    <div class="d-flex flex-wrap align-center ga-3">
                        <span>{{ tt('Invoice Profile') }}</span>
                        <v-progress-circular indeterminate size="20" v-if="loading"></v-progress-circular>
                        <v-spacer />
                        <v-btn size="small" color="default" variant="tonal" :disabled="saving" to="/settings/invoice_email_template">{{ tt('Edit Email Template') }}</v-btn>
                    </div>
                </template>

                <v-card-text class="d-flex mt-2">
                    <v-avatar rounded="lg" variant="tonal" size="100" class="me-4 cursor-pointer">
                        <v-img :src="newProfile.logoDataUrl" v-if="newProfile.logoDataUrl"></v-img>
                        <v-icon size="48" :icon="mdiImageOutline" v-else />

                    </v-avatar>
                    <div class="d-flex flex-column justify-center">
                        <span class="text-body-large">{{ tt('These details are printed on every invoice.') }}</span>
                        <div class="d-flex ga-2 mt-2">
                            <v-btn size="small" variant="tonal" :disabled="saving" @click="logoInput?.click()">{{ tt(newProfile.logoDataUrl ? 'Update Logo' : 'Upload Logo') }}</v-btn>
                            <v-btn size="small" variant="text" :disabled="saving" v-if="newProfile.logoDataUrl" @click="newProfile.logoDataUrl = ''">{{ tt('Remove Logo') }}</v-btn>
                        </div>
                        <v-switch :label="tt('Show logo on invoices')" :disabled="saving || !newProfile.logoDataUrl"
                                  :model-value="!newProfile.hideLogo" @update:model-value="newProfile.hideLogo = !$event" />
                    </div>
                </v-card-text>

                <v-divider />

                <v-form>
                    <v-card-text class="my-1 pa-0 text-body-medium">
                        <div class="setting-item">
                            <span>{{ tt('Business Name') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('Business Name')"
                                          v-model="newProfile.businessName" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Address') }}</span>
                            <v-spacer/>
                            <v-textarea class="ms-4" density="compact" rows="4" autocomplete="off"
                                        persistent-placeholder max-width="400px"
                                        :disabled="loading || saving" :placeholder="tt('Address')"
                                        v-model="newProfile.addressLines" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Phone') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('Phone')"
                                          v-model="newProfile.phone" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Website') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('Website')"
                                          v-model="newProfile.website" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('GSTIN') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('GSTIN')"
                                          v-model="newProfile.gstin" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Invoice Number Prefix') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('Invoice Number Prefix')"
                                          v-model="newProfile.numberPrefix" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Payment Terms (days)') }}</span>
                            <v-spacer/>
                            <number-input class="ms-4" density="compact" max-width="400px"
                                          :disabled="loading || saving"
                                          :min-value="0" :max-value="365" :max-decimal-count="0"
                                          v-model="newProfile.defaultTermsDays" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Renewal Drafts (days before)') }}</span>
                            <v-spacer/>
                            <number-input class="ms-4" density="compact" max-width="400px"
                                          :disabled="loading || saving"
                                          :min-value="0" :max-value="90" :max-decimal-count="0"
                                          v-model="newProfile.renewalDraftDays" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Default Currency') }}</span>
                            <v-spacer/>
                            <currency-select class="ms-4" density="compact" max-width="400px"
                                             :disabled="loading || saving" :placeholder="tt('Default Currency')"
                                             v-model="newProfile.defaultCurrency" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Default SAC') }}</span>
                            <v-spacer/>
                            <v-text-field class="ms-4" density="compact" type="text" autocomplete="off"
                                          persistent-placeholder max-width="400px"
                                          :disabled="loading || saving" :placeholder="tt('Default SAC')"
                                          v-model="newProfile.defaultSac" />
                        </div>
                        <div class="setting-item">
                            <span>{{ tt('Default Notes / Terms') }}</span>
                            <v-spacer/>
                            <v-textarea class="ms-4" density="compact" rows="7" autocomplete="off"
                                        persistent-placeholder max-width="400px"
                                        :disabled="loading || saving" :placeholder="tt('Default Notes / Terms')"
                                        v-model="newProfile.defaultNotes" />
                        </div>
                    </v-card-text>

                    <v-card-text class="d-flex flex-wrap gap-4">
                        <v-tooltip :disabled="!inputIsInvalid" :text="inputInvalidProblemMessage ? tt(inputInvalidProblemMessage) : ''">
                            <template v-slot:activator="{ props }">
                                <div v-bind="props" class="d-inline-block">
                                    <v-btn :disabled="inputIsNotChanged || inputIsInvalid || saving" @click="save">
                                        {{ tt('Save Changes') }}
                                        <v-progress-circular indeterminate size="22" class="ms-2" v-if="saving"></v-progress-circular>
                                    </v-btn>
                                </div>
                            </template>
                        </v-tooltip>
                        <v-btn color="default" variant="tonal" :disabled="inputIsNotChanged || saving" @click="reset">{{ tt('Reset') }}</v-btn>
                    </v-card-text>
                </v-form>
            </v-card>
        </v-col>
    </v-row>

    <snack-bar ref="snackbar" />
    <input ref="logoInput" type="file" style="display: none" accept="image/png,image/jpeg" @change="onLogoChosen($event)" />
</template>

<script setup lang="ts">
import { useTemplateRef } from 'vue';

import { mdiImageOutline } from '@mdi/js';

import SnackBar from '@/components/desktop/SnackBar.vue';

import { useI18n } from '@/locales/helpers.ts';
import { useInvoiceProfilePageBase } from '@/views/base/invoices/InvoiceProfilePageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

type SnackBarType = InstanceType<typeof SnackBar>;

const { tt } = useI18n();
const {
    loading,
    saving,
    newProfile,
    inputIsNotChanged,
    inputInvalidProblemMessage,
    inputIsInvalid,
    setProfile,
    reset,
    readLogoFile
} = useInvoiceProfilePageBase();

const invoicesStore = useInvoicesStore();

const snackbar = useTemplateRef<SnackBarType>('snackbar');
const logoInput = useTemplateRef<HTMLInputElement>('logoInput');

function init(): void {
    loading.value = true;

    invoicesStore.loadInvoiceProfile({ force: true }).then(profile => {
        setProfile(profile);
        loading.value = false;
    }).catch(error => {
        loading.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

function onLogoChosen(event: Event): void {
    const el = event.target as HTMLInputElement;

    if (!el.files || !el.files.length || !el.files[0]) {
        return;
    }

    const logoFile = el.files[0] as File;
    el.value = '';

    readLogoFile(logoFile).then(dataUrl => {
        newProfile.value.logoDataUrl = dataUrl;
    }).catch(error => {
        snackbar.value?.showError(error);
    });
}

function save(): void {
    if (saving.value || inputIsInvalid.value) {
        return;
    }

    saving.value = true;

    invoicesStore.updateInvoiceProfile({ profile: newProfile.value }).then(profile => {
        saving.value = false;
        setProfile(profile);
        snackbar.value?.showMessage('Invoice profile has been saved');
    }).catch(error => {
        saving.value = false;

        if (!error.processed) {
            snackbar.value?.showError(error);
        }
    });
}

init();
</script>
