import { computed, onScopeDispose, ref, shallowRef, watch } from 'vue';

import type { InvoiceEmailPreviewRequest, InvoiceEmailPreviewResponse } from '@/models/invoice.ts';

// Readiness belongs to the exact editor contents, never to the last successful response.
export function useInvoiceEmailPreview(request: () => InvoiceEmailPreviewRequest, fetchPreview: (request: InvoiceEmailPreviewRequest) => Promise<InvoiceEmailPreviewResponse>, delay = 250) {
    const pending = ref(false);
    const error = shallowRef<unknown>(null);
    const preview = shallowRef<InvoiceEmailPreviewResponse | null>(null);
    const current = ref(false);
    const valid = computed(() => current.value && !pending.value && !error.value);
    let generation = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;

    async function load(value: InvoiceEmailPreviewRequest, version: number): Promise<void> {
        try {
            const result = await fetchPreview(value);
            if (version === generation) {
                preview.value = result;
                current.value = true;
            }
        } catch (problem) {
            if (version === generation) {
                error.value = problem;
            }
        } finally {
            if (version === generation) {
                pending.value = false;
            }
        }
    }

    function refresh(): void {
        const version = ++generation;
        const value = { ...request() };
        clearTimeout(timer);
        pending.value = true;
        current.value = false;
        error.value = null;
        if (delay > 0) {
            timer = setTimeout(() => { void load(value, version); }, delay);
        } else {
            void load(value, version);
        }
    }

    watch(request, refresh, { immediate: true, deep: true, flush: 'sync' });
    onScopeDispose(() => {
        generation++;
        clearTimeout(timer);
        pending.value = false;
        current.value = false;
    });

    return { pending, valid, error, preview, refresh };
}
