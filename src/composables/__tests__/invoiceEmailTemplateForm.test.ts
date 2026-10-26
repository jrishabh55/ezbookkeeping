// @vitest-environment jsdom
import { afterEach, expect, test, vi } from 'vitest';
import { createApp, defineComponent, h, nextTick, ref, type App } from 'vue';

import InvoiceEmailTemplateForm from '@/components/common/InvoiceEmailTemplateForm.vue';

const { previewEmail } = vi.hoisted(() => ({ previewEmail: vi.fn() }));
vi.mock('@/stores/invoice.ts', () => ({ useInvoicesStore: () => ({ previewInvoiceEmail: previewEmail }) }));
vi.mock('@/locales/helpers.ts', () => ({ useI18n: () => ({ tt: (text: string) => text }) }));
vi.mock('@/components/common/InvoiceEmailSubjectEditor.vue', () => ({ default: defineComponent({
    props: ['modelValue', 'variables'], emits: ['update:modelValue'],
    setup(props, { emit }) { return () => h('input', { class: 'subject-editor', value: props.modelValue, 'data-variable-count': props.variables.length, onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLInputElement).value) }); }
}) }));
vi.mock('@/components/common/InvoiceEmailBodyEditor.vue', () => ({ default: defineComponent({
    props: ['modelValue'], emits: ['update:modelValue'],
    setup(props, { emit }) { return () => h('textarea', { class: 'body-editor', value: props.modelValue, onInput: (event: Event) => emit('update:modelValue', (event.target as HTMLTextAreaElement).value) }); }
}) }));

let app: App;
let root: HTMLDivElement;
afterEach(() => { app?.unmount(); root?.remove(); vi.useRealTimers(); });

function mount(disabled = false) {
    vi.useFakeTimers();
    const subject = ref('{{invoice_number}}');
    const body = ref('<p>{{customer_name}}</p>');
    const valid = ref(false);
    root = document.createElement('div');
    document.body.append(root);
    app = createApp({ render: () => h(InvoiceEmailTemplateForm, {
        subject: subject.value, body: body.value, disabled, allowRestore: true,
        'onUpdate:subject': (value: string) => { subject.value = value; },
        'onUpdate:body': (value: string) => { body.value = value; },
        onValidityChange: (value: boolean) => { valid.value = value; }
    }) });
    app.mount(root);
    return { subject, body, valid };
}

const rendered = {
    subject: 'Invoice INV-1', bodyHtml: '<p>Alice</p>', emailHtml: '<html><body><h1>Invoice INV-1</h1><p>Alice</p></body></html>',
    variables: [{ key: 'customer_name', label: 'Customer Name', value: 'Alice' }],
    defaultSubjectTemplate: '{{invoice_number}}', defaultBodyTemplate: '<p>Default</p>', sample: true
};

test('renders the complete server email in a restricted iframe without replacing saved template keys', async () => {
    previewEmail.mockResolvedValue(rendered);
    const state = mount();
    expect(Number(root.querySelector('.subject-editor')?.getAttribute('data-variable-count'))).toBeGreaterThan(0);
    expect(state.valid.value).toBe(false);
    await vi.advanceTimersByTimeAsync(250);
    await nextTick();
    const frame = root.querySelector('iframe')!;
    expect(frame.getAttribute('sandbox')).toBe('');
    expect(frame.getAttribute('srcdoc')).toBe(rendered.emailHtml);
    expect(state.valid.value).toBe(true);
    expect(state.subject.value).toBe('{{invoice_number}}');
    expect(state.body.value).toBe('<p>{{customer_name}}</p>');
});

test('guards model writes while disabled, including restore default and child update events', async () => {
    previewEmail.mockResolvedValue(rendered);
    const state = mount(true);
    await vi.advanceTimersByTimeAsync(250);
    await nextTick();
    const input = root.querySelector<HTMLInputElement>('.subject-editor')!;
    input.value = 'Unexpected child edit';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    root.querySelector('button')!.dispatchEvent(new Event('click', { bubbles: true }));
    await nextTick();
    expect(state.subject.value).toBe('{{invoice_number}}');
    expect(state.body.value).toBe('<p>{{customer_name}}</p>');
});

test('a renderer validation error invalidates the current email and exposes its explanation', async () => {
    previewEmail.mockResolvedValue(rendered);
    const state = mount();
    await vi.advanceTimersByTimeAsync(250);
    await nextTick();
    previewEmail.mockRejectedValue({ error: { errorMessage: 'Unknown invoice email variable: customer_naem' } });
    const input = root.querySelector<HTMLInputElement>('.subject-editor')!;
    input.value = '{{customer_naem}}';
    input.dispatchEvent(new Event('input', { bubbles: true }));
    expect(state.valid.value).toBe(false);
    await vi.advanceTimersByTimeAsync(250);
    await nextTick();
    expect(root.querySelector('iframe')).toBeNull();
    expect(root.querySelector('[role="alert"]')?.textContent).toContain('Unknown invoice email variable: customer_naem');
    expect(state.valid.value).toBe(false);
});
