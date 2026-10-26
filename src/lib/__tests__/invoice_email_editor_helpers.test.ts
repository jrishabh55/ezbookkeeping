import { describe, expect, test } from 'vitest';

import {
    filterInvoiceEmailVariables,
    invoiceEmailSubjectToDocument,
    invoiceEmailDocumentToSubject,
    isSafeInvoiceEmailLink
} from '../invoice_email_editor.ts';

const variables = [
    { key: 'customer_name', label: 'Customer name', value: '<b>{{invoice_total}}</b>' },
    { key: 'invoice_total', label: 'Invoice total', value: '₹1,000' }
];

describe('invoice email subject and variable helpers', () => {
    test('parses template tokens into atomic nodes without interpreting adjacent markup', () => {
        expect(invoiceEmailSubjectToDocument('<b>Hello</b> {{customer_name}}')).toEqual({
            type: 'doc', content: [{ type: 'paragraph', content: [
                { type: 'text', text: '<b>Hello</b> ' },
                { type: 'invoiceVariable', attrs: { key: 'customer_name' } }
            ] }]
        });
    });

    test('serializes subject variables as stable keys and collapses line breaks', () => {
        expect(invoiceEmailDocumentToSubject({ type: 'doc', content: [{ type: 'paragraph', content: [
            { type: 'text', text: 'Hello\r\n' },
            { type: 'invoiceVariable', attrs: { key: 'customer_name' } }
        ] }] })).toBe('Hello {{customer_name}}');
    });

    test('searches labels and stable keys without consulting variable values', () => {
        expect(filterInvoiceEmailVariables(variables, 'CUSTOMER')).toEqual([variables[0]]);
        expect(filterInvoiceEmailVariables(variables, 'invoice t')).toEqual([variables[1]]);
        expect(filterInvoiceEmailVariables(variables, '1,000')).toEqual([]);
    });

    test.each([
        ['https://example.com/path', true],
        ['http://example.com', true],
        ['mailto:hello@example.com', true],
        ['{{invoice_url}}', true],
        ['javascript:alert(1)', false],
        ['data:text/html,hello', false],
        ['//example.com', false],
        ['https://', false]
    ])('validates link %s', (url, allowed) => {
        expect(isSafeInvoiceEmailLink(url)).toBe(allowed);
    });
});
