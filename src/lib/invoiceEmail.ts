import type { Invoice, InvoiceProfile, InvoiceSendRequest } from '@/models/invoice.ts';

// Kept out of invoice_email_editor.ts so screens can list variables without loading the editor.
export interface InvoiceEmailEditorVariable {
    readonly key: string;
    readonly label: string;
}

export const INVOICE_EMAIL_EDITOR_VARIABLES: readonly InvoiceEmailEditorVariable[] = [
    { key: 'customer_name', label: 'Customer Name' },
    { key: 'contact_name', label: 'Contact Name' },
    { key: 'customer_email', label: 'Customer Email' },
    { key: 'invoice_number', label: 'Invoice Number' },
    { key: 'invoice_date', label: 'Invoice Date' },
    { key: 'due_date', label: 'Due Date' },
    { key: 'invoice_total', label: 'Invoice Total' },
    { key: 'amount_paid', label: 'Amount Paid' },
    { key: 'amount_due', label: 'Amount Due' },
    { key: 'currency', label: 'Currency' },
    { key: 'invoice_url', label: 'Invoice URL' },
    { key: 'business_name', label: 'Business Name' },
    { key: 'business_address', label: 'Business Address' },
    { key: 'business_phone', label: 'Business Phone' },
    { key: 'business_website', label: 'Business Website' },
    { key: 'document_type', label: 'Document Type' }
];

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
