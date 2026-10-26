import { Node, mergeAttributes, type Editor, type Extensions, type JSONContent } from '@tiptap/core';
import StarterKit from '@tiptap/starter-kit';
import TextAlign from '@tiptap/extension-text-align';
import Suggestion, { type SuggestionOptions } from '@tiptap/suggestion';
import { NodeSelection } from '@tiptap/pm/state';

export interface InvoiceEmailEditorVariable {
    readonly key: string;
    readonly label: string;
}

export interface InvoiceEmailEditorOptions {
    readonly variables: () => readonly InvoiceEmailEditorVariable[];
    readonly subject?: boolean;
    readonly suggestion?: SuggestionOptions<InvoiceEmailEditorVariable, InvoiceEmailEditorVariable>['render'];
    readonly translateLabel?: (label: string) => string;
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
    { key: 'business_website', label: 'Business Website' }
];

export function createInvoiceEmailExtensions(options: InvoiceEmailEditorOptions): Extensions {
    const variable = Node.create({
        name: 'invoiceVariable',
        group: 'inline',
        inline: true,
        atom: true,
        selectable: false,
        addAttributes() {
            return { key: { default: '', rendered: false } };
        },
        parseHTML() {
            return [{
                tag: 'span[data-invoice-variable]',
                getAttrs: element => ({ key: element.getAttribute('data-invoice-variable') || '' })
            }];
        },
        renderHTML({ node }) {
            const key = String(node.attrs['key']);
            return ['span', { 'data-invoice-variable': key }, `{{${key}}}`];
        },
        renderText({ node }) {
            return `{{${String(node.attrs['key'])}}}`;
        },
        addNodeView() {
            return ({ node }) => {
                const dom = document.createElement('span');
                const key = String(node.attrs['key']);
                const label = options.variables().find(item => item.key === key)?.label || key;
                dom.className = 'invoice-email-variable';
                dom.setAttribute('data-invoice-variable', key);
                dom.contentEditable = 'false';
                dom.textContent = options.translateLabel?.(label) || label;
                return { dom };
            };
        },
        addProseMirrorPlugins() {
            return [Suggestion<InvoiceEmailEditorVariable, InvoiceEmailEditorVariable>({
                editor: this.editor,
                char: '@',
                allowSpaces: false,
                items: ({ query }) => [...filterInvoiceEmailVariables(options.variables(), query)],
                command: ({ editor, range, props }) => {
                    editor.chain().focus().insertContentAt(range, [
                        { type: 'invoiceVariable', attrs: { key: props.key } },
                        { type: 'text', text: ' ' }
                    ]).run();
                },
                render: options.suggestion
            })];
        }
    });

    const extensions: Extensions = [StarterKit.configure({
        blockquote: false,
        code: false,
        codeBlock: false,
        strike: false,
        horizontalRule: false,
        trailingNode: false,
        heading: options.subject ? false : { levels: [1, 2, 3] },
        bold: options.subject ? false : {},
        italic: options.subject ? false : {},
        underline: options.subject ? false : {},
        bulletList: options.subject ? false : {},
        orderedList: options.subject ? false : {},
        listItem: options.subject ? false : {},
        listKeymap: options.subject ? false : {},
        hardBreak: options.subject ? false : {},
        link: options.subject ? false : {
            openOnClick: false,
            autolink: false,
            linkOnPaste: false,
            HTMLAttributes: { target: null, rel: null },
            isAllowedUri: isSafeInvoiceEmailLink
        }
    }), variable];

    if (!options.subject) {
        extensions.push(Node.create({
            name: 'invoiceButton',
            group: 'block',
            atom: true,
            draggable: true,
            addAttributes() {
                return { label: { default: 'View Invoice', rendered: false } };
            },
            parseHTML() {
                return [{ tag: 'div[data-invoice-button="true"]', getAttrs: element => ({ label: element.textContent || '' }) }];
            },
            renderHTML({ node, HTMLAttributes }) {
                return ['div', mergeAttributes({ 'data-invoice-button': 'true' }, HTMLAttributes), String(node.attrs['label'])];
            },
            renderText({ node }) {
                return String(node.attrs['label']);
            }
        }), TextAlign.configure({
            types: ['heading', 'paragraph', 'invoiceButton'],
            alignments: ['left', 'center', 'right', 'justify'],
            defaultAlignment: null
        }));
    }

    return extensions;
}

export function invoiceEmailSubjectToDocument(subject: string): JSONContent {
    const text = subject.replace(/[\r\n]+/g, ' ');
    const content: JSONContent[] = [];
    let offset = 0;

    for (const match of text.matchAll(/\{\{([a-z][a-z0-9_]*)\}\}/g)) {
        if (match.index > offset) {
            content.push({ type: 'text', text: text.slice(offset, match.index) });
        }
        content.push({ type: 'invoiceVariable', attrs: { key: match[1] } });
        offset = match.index + match[0].length;
    }

    if (offset < text.length) {
        content.push({ type: 'text', text: text.slice(offset) });
    }

    return { type: 'doc', content: [{ type: 'paragraph', content }] };
}

export function invoiceEmailDocumentToSubject(document: JSONContent): string {
    if (document.type === 'invoiceVariable') {
        return `{{${String(document.attrs?.['key'] || '')}}}`;
    }
    if (document.type === 'text') {
        return (document.text || '').replace(/[\r\n]+/g, ' ');
    }

    return (document.content || []).map(invoiceEmailDocumentToSubject).join(document.type === 'doc' ? ' ' : '');
}

export function filterInvoiceEmailVariables(variables: readonly InvoiceEmailEditorVariable[], query: string): readonly InvoiceEmailEditorVariable[] {
    const search = query.toLocaleLowerCase().replace(/_/g, ' ').trim();
    return variables.filter(variable => `${variable.key.replace(/_/g, ' ')} ${variable.label}`.toLocaleLowerCase().includes(search));
}

export function isSafeInvoiceEmailLink(url: string): boolean {
    if (url === '{{invoice_url}}') {
        return true;
    }
    if (/[\u0000-\u0020\u007f]/.test(url)) {
        return false;
    }

    try {
        const parsed = new URL(url);
        return ((parsed.protocol === 'https:' || parsed.protocol === 'http:') && !!parsed.hostname)
            || (parsed.protocol === 'mailto:' && !!parsed.pathname);
    } catch {
        return false;
    }
}

export function moveInvoiceEmailButton(editor: Editor, direction: -1 | 1): boolean {
    const { state, view } = editor;
    const { selection } = state;
    if (!(selection instanceof NodeSelection) || selection.node.type.name !== 'invoiceButton' || selection.$from.depth !== 0) {
        return false;
    }

    const index = selection.$from.index();
    const adjacent = state.doc.maybeChild(index + direction);
    if (!adjacent) {
        return false;
    }

    const target = direction < 0 ? selection.from - adjacent.nodeSize : selection.to + adjacent.nodeSize;
    const transaction = state.tr.delete(selection.from, selection.to);
    const position = transaction.mapping.map(target);
    transaction.insert(position, selection.node);
    transaction.setSelection(NodeSelection.create(transaction.doc, position));
    view.dispatch(transaction);
    return true;
}

export function insertInvoiceEmailButton(editor: Editor, label: string): void {
    editor.chain().focus().command(({ tr, state }) => {
        const type = state.schema.nodes['invoiceButton'];
        if (!type) {
            return false;
        }
        const from = tr.selection.$from;
        // Buttons remain top-level blocks, including when inserted while editing a list.
        const position = from.depth === 0 ? tr.selection.to
            : from.depth === 1 && from.parentOffset === 0 ? from.before(1) : from.after(1);
        tr.insert(position, type.create({ label, textAlign: 'center' }));
        tr.setSelection(NodeSelection.create(tr.doc, position));
        return true;
    }).run();
}
