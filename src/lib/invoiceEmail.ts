import type { Invoice, InvoiceProfile, InvoiceSendRequest } from '@/models/invoice.ts';

// Keep the saved template unresolved: the server expands it for this invoice on preview/send.
export function createInvoiceEmailSendRequest(invoice: Pick<Invoice, 'id' | 'billToEmail'>, profile: Pick<InvoiceProfile, 'emailSubjectTemplate' | 'emailBodyTemplate'> | null): InvoiceSendRequest {
    return {
        id: invoice.id,
        to: invoice.billToEmail,
        subject: profile?.emailSubjectTemplate ?? '',
        bodyHtml: profile?.emailBodyTemplate ?? '',
        message: ''
    };
}

export function getInvoiceEmailPreviewErrorMessage(error: unknown): string {
    if (typeof error === 'string' && error) {
        return error;
    }
    if (typeof error === 'object' && error) {
        if ('message' in error && typeof error.message === 'string' && error.message) {
            return error.message;
        }
        if ('error' in error && typeof error.error === 'object' && error.error && 'errorMessage' in error.error && typeof error.error.errorMessage === 'string' && error.error.errorMessage) {
            return error.error.errorMessage;
        }
    }
    return 'Unable to preview invoice email';
}
