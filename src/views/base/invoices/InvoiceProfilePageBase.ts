import { ref, computed } from 'vue';

import { InvoiceProfile } from '@/models/invoice.ts';

import { isEquals } from '@/lib/common.ts';

// matches the backend's 350,000-character limit on logoDataUrl (base64 grows a file by about a third)
const MAX_INVOICE_LOGO_FILE_SIZE = 250 * 1024;

export function useInvoiceProfilePageBase() {
    const loading = ref<boolean>(true);
    const saving = ref<boolean>(false);
    const newProfile = ref<InvoiceProfile>(InvoiceProfile.createNewProfile());
    const oldProfile = ref<InvoiceProfile>(InvoiceProfile.createNewProfile());

    const inputIsNotChanged = computed<boolean>(() => isEquals(newProfile.value.toUpdateRequest(), oldProfile.value.toUpdateRequest()));

    const inputInvalidProblemMessage = computed<string | null>(() => {
        if (!newProfile.value.numberPrefix.trim()) {
            return 'Invoice number prefix cannot be blank';
        } else if (newProfile.value.defaultTermsDays < 0 || newProfile.value.defaultTermsDays > 365) {
            return 'Payment terms must be between 0 and 365 days';
        }

        return null;
    });

    const inputIsInvalid = computed<boolean>(() => !!inputInvalidProblemMessage.value);

    function setProfile(profile: InvoiceProfile): void {
        newProfile.value = profile.clone();
        oldProfile.value = profile.clone();
    }

    function reset(): void {
        newProfile.value = oldProfile.value.clone();
    }

    // reads a png / jpeg file into a data url; rejects with an English message key
    function readLogoFile(file: File): Promise<string> {
        return new Promise((resolve, reject) => {
            if (file.type !== 'image/png' && file.type !== 'image/jpeg') {
                reject({ message: 'The logo must be a PNG or JPEG image' });
                return;
            }

            if (file.size > MAX_INVOICE_LOGO_FILE_SIZE) {
                reject({ message: 'Logo must be smaller than 250 KB' });
                return;
            }

            const reader = new FileReader();
            reader.onload = () => resolve(reader.result as string);
            reader.onerror = () => reject({ message: 'Unable to read this file' });
            reader.readAsDataURL(file);
        });
    }

    return {
        // states
        loading,
        saving,
        newProfile,
        oldProfile,
        // computed states
        inputIsNotChanged,
        inputInvalidProblemMessage,
        inputIsInvalid,
        // functions
        setProfile,
        reset,
        readLogoFile
    };
}
