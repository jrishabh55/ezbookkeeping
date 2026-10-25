<template>
    <f7-page @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt('Invoice Profile')"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsNotChanged || inputIsInvalid || saving }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-vertical skeleton-text" v-if="loading">
            <f7-list-item class="list-item-with-header-and-title" header="Business Name" title="Business Name"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title" header="Address" title="Address"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title" header="Invoice Number Prefix" title="INV"></f7-list-item>
        </f7-list>

        <f7-list form strong inset dividers class="margin-vertical" v-else>
            <f7-list-item :header="tt('Logo')">
                <template #footer>
                    <div class="display-flex align-items-center padding-top-half">
                        <image-box style="height: 48px; width: auto" alt="logo" :src="newProfile.logoDataUrl" v-if="newProfile.logoDataUrl"></image-box>
                        <f7-button class="margin-left-half" :text="tt('Update Logo')" @click="logoInput?.click()"></f7-button>
                        <f7-button class="margin-left-half" color="red" :text="tt('Remove Logo')" @click="newProfile.logoDataUrl = ''" v-if="newProfile.logoDataUrl"></f7-button>
                    </div>
                </template>
            </f7-list-item>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Business Name')" :placeholder="tt('Business Name')"
                           v-model:value="newProfile.businessName"></f7-list-input>
            <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                           :label="tt('Address')" :placeholder="tt('Address')"
                           v-textarea-auto-size v-model:value="newProfile.addressLines"></f7-list-input>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Phone')" :placeholder="tt('Phone')"
                           v-model:value="newProfile.phone"></f7-list-input>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Website')" :placeholder="tt('Website')"
                           v-model:value="newProfile.website"></f7-list-input>
            <f7-list-input type="text" autocomplete="off" clear-button
                           :label="tt('Invoice Number Prefix')" :placeholder="tt('Invoice Number Prefix')"
                           v-model:value="newProfile.numberPrefix"></f7-list-input>
            <list-number-input :label="tt('Payment Terms (days)')" :placeholder="tt('Payment Terms (days)')"
                               :min-value="0" :max-value="365" :max-decimal-count="0"
                               v-model="newProfile.defaultTermsDays"></list-number-input>
            <f7-list-input type="textarea" style="height: auto" autocomplete="off"
                           :label="tt('Default Notes / Terms')" :placeholder="tt('Default Notes / Terms')"
                           v-textarea-auto-size v-model:value="newProfile.defaultNotes"></f7-list-input>
        </f7-list>

        <input ref="logoInput" type="file" style="display: none" accept="image/png,image/jpeg" @change="onLogoChosen($event)" />
    </f7-page>
</template>

<script setup lang="ts">
import { ref, useTemplateRef } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useInvoiceProfilePageBase } from '@/views/base/invoices/InvoiceProfilePageBase.ts';

import { useInvoicesStore } from '@/stores/invoice.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    loading,
    saving,
    newProfile,
    inputIsNotChanged,
    inputInvalidProblemMessage,
    inputIsInvalid,
    setProfile,
    readLogoFile
} = useInvoiceProfilePageBase();

const invoicesStore = useInvoicesStore();

const logoInput = useTemplateRef<HTMLInputElement>('logoInput');
const loadingError = ref<unknown | null>(null);

function init(): void {
    loading.value = true;

    invoicesStore.loadInvoiceProfile({ force: true }).then(profile => {
        setProfile(profile);
        loading.value = false;
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
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
        showAlert(error.message);
    });
}

function save(): void {
    const router = props.f7router;
    const problemMessage = inputInvalidProblemMessage.value;

    if (problemMessage) {
        showAlert(problemMessage);
        return;
    }

    saving.value = true;
    showLoading(() => saving.value);

    invoicesStore.updateInvoiceProfile({ profile: newProfile.value }).then(profile => {
        saving.value = false;
        hideLoading();
        setProfile(profile);
        showToast('Invoice profile has been saved');
        router.back();
    }).catch(error => {
        saving.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
