// @vitest-environment jsdom
import { afterEach, beforeAll, describe, expect, test } from 'vitest';
import { createApp, h, nextTick, reactive, type App } from 'vue';
import { createI18n } from 'vue-i18n';
import { createPinia } from 'pinia';
import type { Editor } from '@tiptap/core';

import InvoiceEmailBodyEditor from '@/components/common/InvoiceEmailBodyEditor.vue';
import InvoiceEmailSubjectEditor from '@/components/common/InvoiceEmailSubjectEditor.vue';

const apps: App[] = [];

// jsdom has no layout engine; ProseMirror uses Range geometry when scrolling a selection.
beforeAll(() => {
    Object.defineProperty(Range.prototype, 'getClientRects', { configurable: true, value: () => [new DOMRect()] });
    Object.defineProperty(Range.prototype, 'getBoundingClientRect', { configurable: true, value: () => new DOMRect() });
});

async function mountEditor(subject = false, disabled = false) {
    const state = reactive({ value: '', disabled });
    const updates: string[] = [];
    const root = document.createElement('div');
    document.body.append(root);
    const app = createApp({
        render: () => h(subject ? InvoiceEmailSubjectEditor : InvoiceEmailBodyEditor, {
            modelValue: state.value,
            disabled: state.disabled,
            variables: [{ key: 'customer_name', label: 'Customer Name' }, { key: 'invoice_total', label: 'Invoice Total' }],
            'onUpdate:modelValue': (value: string) => {
                updates.push(value);
                state.value = value;
            }
        })
    });
    app.use(createI18n({ legacy: false, locale: 'en', messages: { en: {} }, missingWarn: false, fallbackWarn: false }));
    app.use(createPinia());
    app.mount(root);
    apps.push(app);
    await nextTick();
    await nextTick();
    return { root, state, updates };
}

function button(root: HTMLElement, text: string): HTMLButtonElement {
    const found = Array.from(root.querySelectorAll('button')).find(item => item.textContent?.trim() === text);
    expect(found, `button ${text}`).toBeDefined();
    return found!;
}

afterEach(() => {
    apps.splice(0).forEach(app => app.unmount());
    document.body.replaceChildren();
});

describe('invoice email editor controls', () => {
    test('searches the insertion menu and inserts a stable variable marker', async () => {
        const { root, state } = await mountEditor();
        button(root, 'Insert Variable').click();
        await nextTick();
        const search = root.querySelector<HTMLInputElement>('input[type="search"]')!;
        expect(search).not.toBeNull();
        search.value = 'invoice';
        search.dispatchEvent(new Event('input', { bubbles: true }));
        await nextTick();
        expect(root.querySelectorAll('[role="option"]')).toHaveLength(1);
        root.querySelector<HTMLElement>('[role="option"]')!.click();
        await nextTick();
        expect(state.value).toBe('<p><span data-invoice-variable="invoice_total">{{invoice_total}}</span> </p>');
        expect(root.querySelector('.invoice-email-variable')?.textContent).toBe('Invoice Total');
    });

    test('subject inserts plain tokens and leaves external reset values editable', async () => {
        const { root, state } = await mountEditor(true);
        button(root, 'Insert Variable').click();
        await nextTick();
        root.querySelector<HTMLElement>('[role="option"]')!.click();
        await nextTick();
        expect(state.value).toBe('{{customer_name}} ');
        state.value = 'Reset {{invoice_total}}';
        await nextTick();
        expect(root.querySelector('[contenteditable="true"]')?.textContent).toBe('Reset Invoice Total');
        expect(root.querySelector('.invoice-email-formatting')).toBeNull();
    });

    test('variable menu supports arrow selection, Enter, and Escape', async () => {
        const { root, state } = await mountEditor(true);
        button(root, 'Insert Variable').click();
        await nextTick();
        const search = root.querySelector<HTMLInputElement>('input[type="search"]')!;
        search.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
        search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        await nextTick();
        expect(state.value).toBe('{{invoice_total}} ');
        button(root, 'Insert Variable').click();
        await nextTick();
        root.querySelector('input')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        await nextTick();
        expect(root.querySelector('[role="listbox"]')).toBeNull();
        expect(state.value).toBe('{{invoice_total}} ');
    });

    test('Escape restores editor focus after dismissing the insertion search', async () => {
        const { root } = await mountEditor(true);
        button(root, 'Insert Variable').click();
        await nextTick();
        const search = root.querySelector<HTMLInputElement>('input[type="search"]')!;
        expect(document.activeElement).toBe(search);
        search.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
        await nextTick();
        await expect.poll(() => document.activeElement).toBe(root.querySelector('.tiptap'));
    });

    test('typing an @ query opens variables and Enter replaces it with an atomic chip', async () => {
        const { root, state } = await mountEditor(true);
        const paragraph = root.querySelector('.tiptap p')!;
        paragraph.textContent = '@invoice';
        const selection = window.getSelection()!;
        selection.collapse(paragraph.firstChild!, 8);
        paragraph.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: '@invoice' }));
        await expect.poll(() => root.querySelectorAll('[role="option"]').length).toBe(1);
        root.querySelector('.tiptap')!.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
        await nextTick();
        expect(state.value).toBe('{{invoice_total}} ');
        expect(root.querySelector('.invoice-email-variable')?.textContent).toBe('Invoice Total');
    });

    test('inserts, labels, moves, and deletes an invoice button', async () => {
        const { root, state } = await mountEditor();
        state.value = '<p>Introduction</p>';
        await nextTick();
        button(root, 'Insert Invoice Button').click();
        await nextTick();
        await expect.poll(() => root.querySelector('input[data-button-label]')).not.toBeNull();
        const label = root.querySelector<HTMLInputElement>('input[data-button-label]')!;
        expect(label).not.toBeNull();
        label.value = 'Pay now';
        label.dispatchEvent(new Event('input', { bubbles: true }));
        await nextTick();
        expect(state.value).toContain('>Pay now</div>');
        button(root, 'Move Down').click();
        await nextTick();
        expect(state.value).toBe('<p>Introduction</p><div data-invoice-button="true" style="text-align: center;">Pay now</div>');
        button(root, 'Remove Button').click();
        await nextTick();
        expect(state.value).toBe('<p>Introduction</p>');
    });

    test('retains each character while replacing a button label through parent v-model before animation frames', async () => {
        const { root, state } = await mountEditor();
        button(root, 'Insert Invoice Button').click();
        await expect.poll(() => root.querySelector('input[data-button-label]')).not.toBeNull();
        const label = root.querySelector<HTMLInputElement>('input[data-button-label]')!;
        label.focus();

        for (const replacement of ['View Online', 'Open']) {
            label.select();
            for (const [index, character] of [...replacement].entries()) {
                label.setRangeText(character, label.selectionStart!, label.selectionEnd!, 'end');
                label.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: character }));
                await nextTick();
                expect(label.value).toBe(replacement.slice(0, index + 1));
                expect(state.value).toContain(`>${replacement.slice(0, index + 1)}</div>`);
            }
        }
        await new Promise<void>(resolve => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
        expect(label.value).toBe('Open');
        expect(state.value).toContain('>Open</div>');
    });

    test('refreshes the label immediately when selecting a different invoice button', async () => {
        const { root, state } = await mountEditor();
        state.value = '<div data-invoice-button="true">First</div><div data-invoice-button="true">Second</div>';
        await nextTick();
        const current = (root.querySelector('.tiptap') as HTMLElement & { editor: Editor }).editor;
        current.commands.setNodeSelection(0);
        await expect.poll(() => root.querySelector<HTMLInputElement>('input[data-button-label]')?.value).toBe('First');
        current.commands.setNodeSelection(1);
        await nextTick();
        expect(root.querySelector<HTMLInputElement>('input[data-button-label]')?.value).toBe('Second');
    });

    test('parent v-model echoes preserve sequential body text transactions before animation frames', async () => {
        const { root, state } = await mountEditor();
        const current = (root.querySelector('.tiptap') as HTMLElement & { editor: Editor }).editor;
        for (const [index, character] of [...'Fast body text'].entries()) {
            current.commands.insertContent(character);
            await nextTick();
            const text = 'Fast body text'.slice(0, index + 1);
            expect(state.value).toBe(`<p>${text}</p>`);
            expect(root.querySelector('.tiptap')?.textContent).toBe(text);
        }
    });

    test('disabled state closes insertion controls and prevents editing', async () => {
        const { root, state } = await mountEditor();
        button(root, 'Insert Variable').click();
        await nextTick();
        state.disabled = true;
        await nextTick();
        expect(root.querySelector('[role="listbox"]')).toBeNull();
        expect(root.querySelector('[contenteditable="false"]')).not.toBeNull();
        expect(Array.from(root.querySelectorAll('button')).every(item => item.disabled)).toBe(true);
        expect(state.value).toBe('');
    });

    test.each([
        { name: 'server HTML', subject: false, value: '<p>Hello<br/>World</p>' },
        { name: 'empty body', subject: false, value: '' },
        { name: 'subject variables', subject: true, value: 'Invoice for {{customer_name}}' },
        { name: 'empty subject', subject: true, value: '' }
    ])('toggling disabled preserves $name without emitting a content change', async ({ subject, value }) => {
        const { root, state, updates } = await mountEditor(subject);
        state.value = value;
        await nextTick();
        expect(updates).toEqual([]);

        state.disabled = true;
        await nextTick();
        expect(root.querySelector('.tiptap')?.getAttribute('contenteditable')).toBe('false');
        expect(updates).toEqual([]);
        state.disabled = false;
        await nextTick();

        expect(root.querySelector('.tiptap')?.getAttribute('contenteditable')).toBe('true');
        expect(state.value).toBe(value);
        expect(updates).toEqual([]);
    });
});
