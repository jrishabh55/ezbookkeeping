<template>
    <div ref="root" class="invoice-email-editor" :class="{ 'invoice-email-subject': singleLine, 'invoice-email-disabled': disabled }">
        <div ref="toolbar" class="invoice-email-toolbar" role="toolbar" :aria-label="tt('Email Formatting')">
            <div class="invoice-email-formatting" v-if="!singleLine">
                <select :aria-label="tt('Text Style')" :disabled="disabled" :value="headingLevel" @change="setHeading">
                    <option value="0">{{ tt('Paragraph') }}</option>
                    <option value="1">{{ tt('Heading 1') }}</option>
                    <option value="2">{{ tt('Heading 2') }}</option>
                    <option value="3">{{ tt('Heading 3') }}</option>
                </select>
                <button type="button" :disabled="disabled" :aria-label="tt('Bold')" :title="tt('Bold')" :aria-pressed="editor?.isActive('bold')" @mousedown.prevent @click="editor?.chain().focus().toggleBold().run()"><strong>B</strong></button>
                <button type="button" :disabled="disabled" :aria-label="tt('Italic')" :title="tt('Italic')" :aria-pressed="editor?.isActive('italic')" @mousedown.prevent @click="editor?.chain().focus().toggleItalic().run()"><em>I</em></button>
                <button type="button" :disabled="disabled" :aria-label="tt('Underline')" :title="tt('Underline')" :aria-pressed="editor?.isActive('underline')" @mousedown.prevent @click="editor?.chain().focus().toggleUnderline().run()"><u>U</u></button>
                <button type="button" :disabled="disabled" :aria-label="tt('Bullet List')" :title="tt('Bullet List')" :aria-pressed="editor?.isActive('bulletList')" @mousedown.prevent @click="editor?.chain().focus().toggleBulletList().run()">•</button>
                <button type="button" :disabled="disabled" :aria-label="tt('Numbered List')" :title="tt('Numbered List')" :aria-pressed="editor?.isActive('orderedList')" @mousedown.prevent @click="editor?.chain().focus().toggleOrderedList().run()">1.</button>
                <select :aria-label="tt('Text Alignment')" :disabled="disabled" :value="alignment" @change="setAlignment">
                    <option value="left">{{ tt('Align Left') }}</option>
                    <option value="center">{{ tt('Align Center') }}</option>
                    <option value="right">{{ tt('Align Right') }}</option>
                    <option value="justify">{{ tt('Justify') }}</option>
                </select>
                <button type="button" :disabled="disabled" :aria-pressed="editor?.isActive('link')" @mousedown.prevent @click="openLinkEditor">{{ tt('Link') }}</button>
                <button type="button" :disabled="disabled" @mousedown.prevent @click="insertInvoiceButton">{{ tt('Insert Invoice Button') }}</button>
            </div>
            <button type="button" :disabled="disabled" :aria-expanded="menuOpen" :aria-controls="menuId" @mousedown.prevent @click="openVariableMenu">{{ tt('Insert Variable') }}</button>
            <button type="button" :disabled="disabled || !editor?.can().undo()" :aria-label="tt('Undo')" :title="tt('Undo')" @mousedown.prevent @click="editor?.chain().focus().undo().run()">↶</button>
            <button type="button" :disabled="disabled || !editor?.can().redo()" :aria-label="tt('Redo')" :title="tt('Redo')" @mousedown.prevent @click="editor?.chain().focus().redo().run()">↷</button>
        </div>

        <div class="invoice-email-edit-controls" v-if="linkEditorOpen && !disabled">
            <label>{{ tt('Link URL') }}<input v-model="linkUrl" type="text" inputmode="url" :placeholder="'https://example.com'" @keydown.enter.prevent="applyLink" @keydown.esc="linkEditorOpen = false" /></label>
            <button type="button" @mousedown.prevent @click="applyLink">{{ tt('Apply Link') }}</button>
            <button type="button" @mousedown.prevent @click="editor?.chain().focus().unsetLink().run(); linkEditorOpen = false">{{ tt('Remove Link') }}</button>
            <button type="button" @click="linkEditorOpen = false">{{ tt('Cancel') }}</button>
            <span class="invoice-email-error" role="alert" v-if="linkError">{{ tt('Enter an HTTP, HTTPS, or mailto link.') }}</span>
        </div>
        <div class="invoice-email-edit-controls" v-if="selectedButton && !disabled">
            <label>{{ tt('Button Text') }}<input data-button-label :value="buttonLabel" @input="setButtonLabel" /></label>
            <button type="button" @mousedown.prevent @click="moveButton(-1)">{{ tt('Move Up') }}</button>
            <button type="button" @mousedown.prevent @click="moveButton(1)">{{ tt('Move Down') }}</button>
            <button type="button" @mousedown.prevent @click="editor?.chain().focus().deleteSelection().run()">{{ tt('Remove Button') }}</button>
        </div>

        <editor-content :editor="editor" />
        <div :id="menuId" class="invoice-email-variables" :style="menuPosition" v-if="menuOpen && !disabled" @keydown="menuKeyDown">
            <input ref="searchInput" v-model="menuQuery" type="search" :aria-label="tt('Search Variables')" :placeholder="tt('Search Variables')" @input="activeIndex = 0" v-if="manualMenu" />
            <div role="listbox" :aria-label="tt('Insert Variable')">
                <button type="button" role="option" :id="`${menuId}-${index}`" :aria-selected="index === activeIndex"
                        :class="{ 'invoice-email-active-option': index === activeIndex }" :key="variable.key"
                        @mousedown.prevent @click="selectVariable(variable)" v-for="(variable, index) in filteredVariables">
                    <span>{{ variable.label }}</span><small>{{ variable.key }}</small>
                </button>
                <span class="invoice-email-no-results" v-if="!filteredVariables.length">{{ tt('No Matching Variables') }}</span>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, useId, useTemplateRef, watch } from 'vue';
import { EditorContent, useEditor } from '@tiptap/vue-3';
import type { Editor } from '@tiptap/core';
import { NodeSelection } from '@tiptap/pm/state';
import { exitSuggestion, type SuggestionProps } from '@tiptap/suggestion';

import { useI18n } from '@/locales/helpers.ts';
import {
    createInvoiceEmailExtensions,
    filterInvoiceEmailVariables,
    invoiceEmailDocumentToSubject,
    invoiceEmailSubjectToDocument,
    insertInvoiceEmailButton,
    INVOICE_EMAIL_EDITOR_VARIABLES,
    isSafeInvoiceEmailLink,
    moveInvoiceEmailButton,
    type InvoiceEmailEditorVariable
} from '@/lib/invoice_email_editor.ts';

type VariableSuggestion = SuggestionProps<InvoiceEmailEditorVariable, InvoiceEmailEditorVariable>;

const props = withDefaults(defineProps<{
    modelValue: string;
    variables?: readonly InvoiceEmailEditorVariable[];
    disabled?: boolean;
    ariaLabel?: string;
    singleLine?: boolean;
}>(), {
    variables: () => INVOICE_EMAIL_EDITOR_VARIABLES,
    disabled: false,
    singleLine: false
});
const emit = defineEmits<{ (e: 'update:modelValue', value: string): void }>();
const { tt } = useI18n();
const root = useTemplateRef<HTMLDivElement>('root');
const toolbar = useTemplateRef<HTMLDivElement>('toolbar');
const searchInput = useTemplateRef<HTMLInputElement>('searchInput');
const menuId = `invoice-variables-${useId()}`;
const menuOpen = ref(false);
const manualMenu = ref(false);
const menuQuery = ref('');
const activeIndex = ref(0);
const menuPosition = ref({ top: '0px', left: '8px' });
let suggestion: VariableSuggestion | undefined;
const linkEditorOpen = ref(false);
const linkUrl = ref('');
const linkError = ref(false);

const editorVariables = computed(() => props.variables === INVOICE_EMAIL_EDITOR_VARIABLES
    ? props.variables.map(variable => ({ key: variable.key, label: tt(variable.label) }))
    : props.variables);
const filteredVariables = computed(() => filterInvoiceEmailVariables(editorVariables.value, menuQuery.value));
const headingLevel = computed(() => editor.value?.isActive('heading') ? editor.value.getAttributes('heading')['level'] : 0);
const alignment = computed(() => {
    if (!editor.value) {
        return 'left';
    }
    const type = editor.value.isActive('invoiceButton') ? 'invoiceButton' : editor.value.isActive('heading') ? 'heading' : 'paragraph';
    return editor.value.getAttributes(type)['textAlign'] || 'left';
});
const selectedButton = ref(false);
const buttonLabel = ref('');

const editor = useEditor({
    extensions: createInvoiceEmailExtensions({
        variables: () => editorVariables.value,
        subject: props.singleLine,
        suggestion: () => ({
            onStart: updateSuggestion,
            onUpdate: updateSuggestion,
            onExit: () => { if (!manualMenu.value) { closeMenu(); } },
            onKeyDown: ({ event }) => menuKeyDown(event)
        })
    }),
    content: props.singleLine ? invoiceEmailSubjectToDocument(props.modelValue) : props.modelValue,
    editable: !props.disabled,
    onCreate: ({ editor: current }) => syncButtonSelection(current),
    onTransaction: ({ editor: current }) => syncButtonSelection(current),
    editorProps: {
        attributes: {
            role: 'textbox',
            'aria-label': props.ariaLabel || tt(props.singleLine ? 'Email Subject' : 'Email Body'),
            'aria-multiline': String(!props.singleLine)
        },
        handleKeyDown: (_view, event) => {
            if (menuOpen.value && menuKeyDown(event)) {
                return true;
            }
            return props.singleLine && event.key === 'Enter';
        },
        handlePaste: (_view, event) => {
            if (!props.singleLine || !event.clipboardData || props.disabled) {
                return false;
            }
            const text = event.clipboardData.getData('text/plain');
            editor.value?.commands.insertContent(invoiceEmailSubjectToDocument(text).content?.[0]?.content || []);
            return true;
        }
    },
    onUpdate: ({ editor: current }) => {
        if (!props.disabled) {
            emit('update:modelValue', props.singleLine ? invoiceEmailDocumentToSubject(current.getJSON()) : current.isEmpty ? '' : current.getHTML());
        }
    }
});

function closeMenu(): void {
    menuOpen.value = false;
    manualMenu.value = false;
    suggestion = undefined;
}

function positionMenu(clientRect?: (() => DOMRect | null) | null): void {
    const bounds = root.value?.getBoundingClientRect();
    const caret = clientRect?.();
    const left = bounds && caret ? Math.max(8, Math.min(caret.left - bounds.left, bounds.width - 300)) : 8;
    const top = bounds && caret ? caret.bottom - bounds.top + 5 : (toolbar.value?.offsetHeight || 0) + 4;
    menuPosition.value = { top: `${top}px`, left: `${left}px` };
}

function updateSuggestion(value: VariableSuggestion): void {
    if (props.disabled || manualMenu.value) {
        return;
    }
    suggestion = value;
    menuOpen.value = true;
    menuQuery.value = value.query;
    activeIndex.value = 0;
    positionMenu(value.clientRect);
}

function openVariableMenu(): void {
    if (props.disabled || !editor.value) {
        return;
    }
    exitSuggestion(editor.value.view);
    manualMenu.value = true;
    menuOpen.value = true;
    menuQuery.value = '';
    activeIndex.value = 0;
    positionMenu();
    nextTick(() => searchInput.value?.focus());
}

function selectVariable(variable: InvoiceEmailEditorVariable): void {
    if (props.disabled || !editor.value) {
        return;
    }
    if (suggestion && !manualMenu.value) {
        suggestion.command(variable);
    } else {
        editor.value.chain().focus().insertContent([
            { type: 'invoiceVariable', attrs: { key: variable.key } },
            { type: 'text', text: ' ' }
        ]).run();
    }
    closeMenu();
}

function menuKeyDown(event: KeyboardEvent): boolean {
    if (!menuOpen.value) {
        return false;
    }
    if (event.key === 'Escape') {
        closeMenu();
        if (editor.value) {
            exitSuggestion(editor.value.view);
            editor.value.commands.focus();
        }
    } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
        const count = filteredVariables.value.length;
        activeIndex.value = count ? (activeIndex.value + (event.key === 'ArrowDown' ? 1 : -1) + count) % count : 0;
        nextTick(() => root.value?.querySelector('.invoice-email-active-option')?.scrollIntoView?.({ block: 'nearest' }));
    } else if (event.key === 'Enter') {
        const variable = filteredVariables.value[activeIndex.value];
        if (variable) {
            selectVariable(variable);
        }
    } else {
        return false;
    }
    event.preventDefault();
    return true;
}

function setHeading(event: Event): void {
    const level = Number((event.target as HTMLSelectElement).value);
    if (level === 1 || level === 2 || level === 3) {
        editor.value?.chain().focus().setHeading({ level }).run();
    } else {
        editor.value?.chain().focus().setParagraph().run();
    }
}

function setAlignment(event: Event): void {
    editor.value?.chain().focus().setTextAlign((event.target as HTMLSelectElement).value).run();
}

function openLinkEditor(): void {
    linkUrl.value = String(editor.value?.getAttributes('link')['href'] || '');
    linkError.value = false;
    linkEditorOpen.value = !linkEditorOpen.value;
}

function applyLink(): void {
    const url = linkUrl.value.trim();
    if (!isSafeInvoiceEmailLink(url)) {
        linkError.value = true;
        return;
    }
    editor.value?.chain().focus().extendMarkRange('link').setLink({ href: url }).run();
    linkEditorOpen.value = false;
}

function insertInvoiceButton(): void {
    if (editor.value) {
        insertInvoiceEmailButton(editor.value, tt('View Invoice'));
    }
}

function setButtonLabel(event: Event): void {
    buttonLabel.value = (event.target as HTMLInputElement).value;
    editor.value?.commands.updateAttributes('invoiceButton', { label: buttonLabel.value });
}

function syncButtonSelection(current: Editor): void {
    const selection = current.view.state.selection;
    selectedButton.value = !props.singleLine && selection instanceof NodeSelection && selection.node.type.name === 'invoiceButton';
    // Tiptap's Vue state notifications wait two animation frames. Input drafts must
    // follow the committed transaction so a parent v-model render cannot restore an old label.
    buttonLabel.value = selectedButton.value && selection instanceof NodeSelection ? String(selection.node.attrs['label'] || '') : '';
}

function moveButton(direction: -1 | 1): void {
    if (editor.value) {
        moveInvoiceEmailButton(editor.value, direction);
    }
}

watch(() => props.modelValue, value => {
    const current = editor.value;
    if (!current) {
        return;
    }
    const serialized = props.singleLine ? invoiceEmailDocumentToSubject(current.getJSON()) : current.isEmpty ? '' : current.getHTML();
    if (value !== serialized) {
        current.commands.setContent(props.singleLine ? invoiceEmailSubjectToDocument(value) : value, { emitUpdate: false });
    }
});

watch(() => props.disabled, disabled => {
    editor.value?.setEditable(!disabled);
    if (disabled) {
        closeMenu();
        linkEditorOpen.value = false;
    }
});

watch(() => props.ariaLabel, label => {
    editor.value?.setOptions({ editorProps: { attributes: {
        role: 'textbox',
        'aria-label': label || tt(props.singleLine ? 'Email Subject' : 'Email Body'),
        'aria-multiline': String(!props.singleLine)
    } } });
});
</script>

<style scoped>
.invoice-email-editor {
    position: relative;
    border: 1px solid rgba(128, 128, 128, 0.4);
    border-radius: 8px;
    font-size: 14px;
    line-height: 1.5;
    min-width: 0;
    color: inherit;
    background: var(--f7-list-bg-color, rgb(var(--v-theme-surface, 255, 255, 255)));
}
.invoice-email-toolbar, .invoice-email-formatting, .invoice-email-edit-controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 4px;
}
.invoice-email-toolbar, .invoice-email-edit-controls {
    padding: 6px;
    border-bottom: 1px solid rgba(128, 128, 128, 0.25);
}
.invoice-email-editor button, .invoice-email-editor select, .invoice-email-editor input {
    font: inherit;
    font-size: 12px;
    color: inherit;
    border: 1px solid rgba(128, 128, 128, 0.3);
    border-radius: 4px;
    background: transparent;
    min-height: 34px;
    padding: 4px 8px;
    max-width: 100%;
    box-sizing: border-box;
}
.invoice-email-editor button { cursor: pointer; }
.invoice-email-editor button:hover:not(:disabled), .invoice-email-editor button[aria-pressed="true"], .invoice-email-editor button.invoice-email-active-option { background: rgba(61, 134, 245, 0.15); }
.invoice-email-editor :disabled { opacity: 0.5; cursor: default; }
.invoice-email-disabled :deep(.tiptap) { opacity: 0.6; }
.invoice-email-editor :focus-visible { outline: 2px solid #3d86f5; outline-offset: 1px; }
.invoice-email-editor :deep(.tiptap) {
    padding: 14px;
    min-height: 240px;
    outline: none;
    overflow-wrap: anywhere;
}
.invoice-email-subject :deep(.tiptap) { min-height: 48px; padding: 10px 14px; }
.invoice-email-editor :deep(.tiptap p) { margin: 0 0 12px; }
.invoice-email-editor :deep(.tiptap > :last-child) { margin-bottom: 0; }
.invoice-email-editor :deep(.tiptap h1) { font-size: 28px; margin: 16px 0 10px; }
.invoice-email-editor :deep(.tiptap h2) { font-size: 22px; margin: 14px 0 8px; }
.invoice-email-editor :deep(.tiptap h3) { font-size: 18px; margin: 12px 0 8px; }
.invoice-email-editor :deep(.tiptap ul), .invoice-email-editor :deep(.tiptap ol) { padding-inline-start: 24px; margin: 0 0 12px; }
.invoice-email-editor :deep(.tiptap ul) { list-style: disc; }
.invoice-email-editor :deep(.tiptap ol) { list-style: decimal; }
.invoice-email-editor :deep(.tiptap a) { color: #3d86f5; text-decoration: underline; }
.invoice-email-editor :deep(.invoice-email-variable) { display: inline; border-radius: 4px; padding: 1px 4px; background: rgba(61, 134, 245, 0.15); box-decoration-break: clone; }
.invoice-email-editor :deep([data-invoice-button]) { color: #fff; background: #3d86f5; border: 2px solid transparent; border-radius: 5px; padding: 10px 18px; margin: 12px 0; cursor: grab; }
.invoice-email-editor :deep([data-invoice-button].ProseMirror-selectednode) { border-color: #143b75; }
.invoice-email-edit-controls label { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.invoice-email-edit-controls input { min-width: 160px; flex: 1; }
.invoice-email-error { width: 100%; color: #c62828; }
.invoice-email-variables {
    position: absolute;
    z-index: 20;
    width: 290px;
    max-width: calc(100% - 16px);
    padding: 6px;
    border: 1px solid rgba(128, 128, 128, 0.4);
    border-radius: 6px;
    background: var(--f7-list-bg-color, rgb(var(--v-theme-surface, 255, 255, 255)));
    box-shadow: 0 4px 14px rgba(0, 0, 0, 0.15);
}
.invoice-email-variables input { width: 100%; margin-bottom: 4px; }
.invoice-email-variables [role="listbox"] { max-height: 230px; overflow-y: auto; }
.invoice-email-variables [role="option"] { display: flex; justify-content: space-between; text-align: start; align-items: center; gap: 8px; border: none; width: 100%; }
.invoice-email-variables small { opacity: 0.6; font-size: 10px; }
.invoice-email-no-results { display: block; padding: 8px; font-size: 12px; }
@media (max-width: 600px) {
    .invoice-email-editor button, .invoice-email-editor select { min-height: 38px; }
    .invoice-email-editor :deep(.tiptap) { min-height: 200px; padding: 12px; }
    .invoice-email-subject :deep(.tiptap) { min-height: 48px; }
}
</style>
