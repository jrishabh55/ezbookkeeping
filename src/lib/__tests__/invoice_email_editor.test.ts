// @vitest-environment jsdom
import { afterEach, describe, expect, test } from 'vitest';
import { Editor } from '@tiptap/core';

import {
    createInvoiceEmailExtensions,
    invoiceEmailSubjectToDocument,
    invoiceEmailDocumentToSubject,
    insertInvoiceEmailButton,
    moveInvoiceEmailButton
} from '../invoice_email_editor.ts';

const variables = [
    { key: 'customer_name', label: 'Customer name', value: '<b>{{invoice_total}}</b>' },
    { key: 'invoice_total', label: 'Invoice total', value: '₹1,000' }
];
const editors: Editor[] = [];

function createEditor(content: string, subject = false): Editor {
    const editor = new Editor({
        element: document.createElement('div'),
        extensions: createInvoiceEmailExtensions({ variables: () => variables, subject }),
        content: subject ? invoiceEmailSubjectToDocument(content) : content
    });
    editors.push(editor);
    return editor;
}

afterEach(() => {
    editors.splice(0).forEach(editor => editor.destroy());
});

describe('invoice email editor serialization', () => {
    test('round trips subject variables without expanding literal variable values', () => {
        const editor = createEditor('Invoice {{invoice_total}} for {{customer_name}}', true);
        expect(editor.getJSON().content?.[0]?.content).toEqual([
            { type: 'text', text: 'Invoice ' },
            { type: 'invoiceVariable', attrs: { key: 'invoice_total' } },
            { type: 'text', text: ' for ' },
            { type: 'invoiceVariable', attrs: { key: 'customer_name' } }
        ]);
        expect(invoiceEmailDocumentToSubject(editor.getJSON())).toBe('Invoice {{invoice_total}} for {{customer_name}}');
        expect(editor.getHTML()).not.toContain('₹1,000');
        expect(editor.getHTML()).not.toContain('&lt;b&gt;');
    });

    test('serializes variable keys instead of pasted labels or sample values', () => {
        const editor = createEditor('<h2 style="text-align: right">Hello <span data-invoice-variable="customer_name">Pasted Sample</span></h2><p><strong>Bold</strong> <em>Italic</em> <u>Underlined</u></p><ol start="3"><li><p>First</p></li></ol>');
        expect(editor.getHTML()).toBe('<h2 style="text-align: right;">Hello <span data-invoice-variable="customer_name">{{customer_name}}</span></h2><p><strong>Bold</strong> <em>Italic</em> <u>Underlined</u></p><ol start="3"><li><p>First</p></li></ol>');
    });

    test('preserves unknown marker keys for useful server validation', () => {
        const editor = createEditor('<p><span data-invoice-variable="unrecognized">Sample</span></p>');
        expect(editor.getHTML()).toBe('<p><span data-invoice-variable="unrecognized">{{unrecognized}}</span></p>');
        expect(invoiceEmailDocumentToSubject(invoiceEmailSubjectToDocument('Hello {{unrecognized}}'))).toBe('Hello {{unrecognized}}');
    });

    test('keeps invoice buttons movable with plain labels and no stored destination', () => {
        const editor = createEditor('<p>Before</p><div data-invoice-button="true" style="text-align: center">Pay &amp; view</div><p>After</p>');
        editor.commands.setNodeSelection(8);
        expect(moveInvoiceEmailButton(editor, -1)).toBe(true);
        expect(editor.getHTML()).toBe('<div data-invoice-button="true" style="text-align: center;">Pay &amp; view</div><p>Before</p><p>After</p>');
        expect(moveInvoiceEmailButton(editor, -1)).toBe(false);
        expect(moveInvoiceEmailButton(editor, 1)).toBe(true);
        editor.commands.updateAttributes('invoiceButton', { label: 'Open invoice' });
        expect(editor.getHTML()).toBe('<p>Before</p><div data-invoice-button="true" style="text-align: center;">Open invoice</div><p>After</p>');
        expect(editor.getHTML()).not.toContain('href');
    });

    test('subject paste remains one line and ignores formatting', () => {
        const editor = createEditor('Hello\r\n{{customer_name}}', true);
        expect(invoiceEmailDocumentToSubject(editor.getJSON())).toBe('Hello {{customer_name}}');
        editor.commands.setContent('<p><strong>First</strong></p><p>Second<br>Third</p>');
        expect(invoiceEmailDocumentToSubject(editor.getJSON())).toBe('First Second Third');
    });

    test('rejects unsafe pasted links while keeping allowed formatting', () => {
        const editor = createEditor('<p><a href="javascript:alert(1)">Unsafe</a> <a href="https://example.com/invoice">Safe</a></p><script>alert(1)</script><img src="x" onerror="alert(1)">');
        expect(editor.getHTML()).toBe('<p>Unsafe <a href="https://example.com/invoice">Safe</a></p>');
    });

    test('discards pasted alignment outside the supported editor values', () => {
        const editor = createEditor('<p style="text-align: start">Hello</p>');
        expect(editor.getHTML()).toBe('<p>Hello</p>');
    });

    test('inserting a button from a list keeps it at the document level so it can move', () => {
        const editor = createEditor('<ul><li><p>List item</p></li></ul><p>After</p>');
        editor.commands.setTextSelection(5);
        insertInvoiceEmailButton(editor, 'Open');
        expect(editor.getHTML()).toBe('<ul><li><p>List item</p></li></ul><div data-invoice-button="true" style="text-align: center;">Open</div><p>After</p>');
        expect(moveInvoiceEmailButton(editor, 1)).toBe(true);
        expect(editor.getHTML()).toBe('<ul><li><p>List item</p></li></ul><p>After</p><div data-invoice-button="true" style="text-align: center;">Open</div>');
    });
});

