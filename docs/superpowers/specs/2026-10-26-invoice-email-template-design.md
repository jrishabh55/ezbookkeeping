# Fully editable invoice email template

Date: 2026-10-26
Status: approved by user 2026-10-26 ("Build it")

## Intent and agreed direction

The user wants a simple invoice email template creator in invoice settings, with rich text and insertable variables. They supplied a Wave invoice email as a reference and chose **fully editable email body, including the heading and footer**, over a fixed Wave layout with only an editable message.

The exact personal Gmail link was inaccessible through the connected account and an unauthenticated browser. A forwarded Wave invoice in the connected work account established the reference: business-name introduction, prominent amount and due date, View Online button, and business/contact footer. Use this structure as a starting preset, without copying customer information, tracking images, Wave branding, or legal text.

## Approaches

1. Fixed Wave-style layout with an editable subject/message. Smaller editor, but the heading and footer remain fixed. The user declined this approach.
2. One rich text editor for the entire body, with an editable subject, variables, movable invoice button, and preview. The user selected this approach because the heading and footer must also be editable.
3. Multiple templates or a drag-and-drop layout designer. These add template selection and layout machinery beyond the requested simple creator; defer them.

## Scope and interface

- Add an **Email Template** section to the existing Invoice Profile settings, already reachable from the invoice list and Settings. Expose the same capability in the mobile invoice profile.
- Start with one saved default template per user/business profile. This is a builder proposal, not an answer the user gave to the earlier single-versus-multiple question.
- Provide an editable plain text subject and a rich text editor for the complete body: heading, message, amount/due-date text, invoice button, signature, and footer. Every body element can be moved, edited, or removed. Only the outer email document and responsive content width are structural.
- Support headings, paragraphs, bold, italic, underline, bullet/numbered lists, links, text alignment, and undo/redo. No HTML source editor, uploaded images, arbitrary CSS, scripting, or column designer in this version.
- Type `@` to search variables, select with arrow keys/Enter, dismiss with Escape, or click **Insert Variable**. Variables appear as labeled chips and delete atomically; users do not need to write placeholder syntax. Allow variable insertion into the subject too.
- Provide an **Insert Invoice Button** control with editable button text. Its destination is always the current invoice's public URL. The node is movable/removable like the rest of the body.
- Show a rendered email preview with sample invoice data, clearly marked as sample data. Update the preview after edits. Include a **Restore Default Template** action that changes the unsaved draft; the normal Save Changes action persists it.
- Start from a Wave-inspired preset with business introduction, amount due, due date, invoice button, and business footer. No compulsory content is appended outside the editable body.
- The Send Invoice form loads the saved template populated with the actual invoice, shows the same rich text editor/preview, and allows edits for that send without changing the saved default.
- No test email is sent as part of saving, previewing, or verification.

## Variables

Use a fixed documented catalog in this version:

| Variable | Value |
| --- | --- |
| `customer_name` | Invoice's saved bill-to name |
| `contact_name` | Saved bill-to contact name, falling back to customer name |
| `customer_email` | Invoice's saved bill-to email |
| `invoice_number` | Invoice number |
| `invoice_date` | Formatted invoice date |
| `due_date` | Formatted due date |
| `invoice_total` | Formatted invoice total with currency |
| `amount_paid` | Formatted linked payments in invoice currency |
| `amount_due` | Current balance due, using the existing invoice rules |
| `currency` | Invoice currency code |
| `invoice_url` | Current public invoice URL |
| `business_name` | Invoice profile business name |
| `business_address` | Profile address, with preserved line breaks |
| `business_phone` | Profile phone |
| `business_website` | Profile website |

Business variable values remain editable through the existing profile fields; invoice/customer values come from the invoice snapshot and current payments. A contact email can be typed into the footer. Arbitrary custom variable definitions and conditional expressions are deferred. Empty optional fields become empty text. Inserted values are text, never interpreted as templates or markup.

## Storage and API

- Extend `InvoiceProfile` with an email subject template (up to 256 characters) and body template (up to 32,000 characters of serialized HTML). Add columns through the existing xorm Sync2 mechanism; no manual data migration.
- Include both fields in private profile request/response, cloning, dirty-state comparison, and save/reset. Keep them out of public invoice profile responses. Missing fields in an update must preserve previously saved templates, so an older client cannot silently erase them.
- Profiles without saved templates use the built-in preset. Restore Default writes the preset on the next explicit save. Blank email subjects are invalid in the editor; an intentionally empty body is permitted.
- Save rich text as a restricted HTML fragment with explicit variable and invoice-button markers. Tiptap nodes serialize stable variable keys, not sample values or localized labels.
- Add an authenticated preview endpoint under invoice profile. It accepts unsaved subject/body overrides and an optional invoice ID. Without an invoice ID it uses deterministic sample data; with one it loads an owned invoice, payments, and profile. Return resolved subject, rendered body/email, and variable metadata needed by the editor. Preview must not create a public token or change invoice state; compose can use the existing prepare/share flow before requesting the actual preview.
- The same Go renderer handles preview and outgoing emails, including defaults and variable expansion. This prevents preview/sending differences and avoids duplicate formatting logic in TypeScript.
- Extend the send request with an optional rich HTML body field. Presence distinguishes an intentionally empty rich body from legacy plain text `message`; retain the old plain text request behavior for existing callers. Resolve defaults when no content override is supplied. The UI sends its current composed rich body.
- Reuse the existing mailer, PDF attachment, invoice public link, SMTP checks, and successful-send timestamp behavior. Generated PDFs and invoice accounting are unaffected.

## Rendering and validation

- Use the existing Vue 3 stack and a maintained Tiptap editor with only the required extensions. This is the builder's implementation choice; the user selected behavior rather than a library. Reference: https://tiptap.dev/docs/editor/getting-started/install/vue3.
- Use the existing Go HTML parser dependency to parse and serialize a strict allowed subset of HTML. Permit only editor-supported elements and narrowly allowed attributes/alignment. Strip scripts, handlers, arbitrary styles, images, embeds, and unsafe URL protocols before content reaches a preview or email.
- Resolve variables only in text nodes and explicitly supported link/button attributes. Escape text and attribute values according to their context. The invoice button resolves solely to the server's invoice public URL. Ordinary links permit safe HTTP(S)/mailto destinations.
- Reject unknown variable keys in templates with a useful error. Do not recursively expand braces appearing inside a customer/business value. Reject subject line breaks and enforce the final expanded subject limit before send.
- The email wrapper uses a responsive, email-compatible table and inline styles. Preview shows the complete generated email rather than relying on app typography. All editable heading/footer content stays inside the body fragment.
- Dates and currency use the existing invoice document formatting, including the existing paid/balance behavior. Never use sample amounts or customer data in a real send.
- Keep the existing per-user authorization boundaries for profile, preview, and send; private templates are never exposed by the public invoice endpoints.

## Verification and completion

- Regression tests for profile persistence, empty/reset/default behavior, old-client updates, and public-response exclusion.
- Renderer tests for every variable, missing optional values, supported formatting, movable/button labels, HTML escaping, unsafe markup/URLs, unknown tokens, non-recursive substitution, and expanded subject validation.
- API/service tests for preview sample versus actual invoice, cross-user denial, no preview side effects, partial payments/multiple currencies, rich send versus legacy plain text, PDF attachment, and unchanged failed-send timestamps. Use a fake mailer; do not send live mail.
- Frontend tests for model serialization, variable catalogs/editor helpers, subject insertion, and request preparation. Type-check, lint changed files, run relevant Go and Vitest suites, and build the frontend.
- Inspect desktop/mobile settings and send UI in a local browser: formatting, keyboard mentions, link/button editing, preview, save/reload, default restoration, and edits for a single send.
- Implementation is a cohesive settings-plus-mail feature. Deployment or changing a live account's template is outside this request unless separately authorized.

## Left to the builder

Choose the small component boundaries, extension versions compatible with the installed Vue/tooling, exact toolbar icons, preview debounce, and renderer/helper package organization. Preserve the repository's UI and localization conventions. If project instructions do not override it, use an isolated managed worktree for implementation and leave unrelated `.claude/` files alone.

## Implementation verification

Implemented on `codex/invoice-email-templates`. Verified a clean locked npm install, 26 frontend test files / 38,785 tests, the full Go suite with `BUILD_PIPELINE=1`, Go vet, Vue type checking, changed-file ESLint, and the production build. Independent source review and scoped fix reviews are approved.

A local isolated account was used to inspect desktop/mobile settings and compose, insert variables by menu and `@`, save/reload without losing keys, restore defaults/reset, preview actual customer/amount/date/link values, and confirm edits for one send leave the saved template unchanged. Native keyboard checks confirm repeated button-label replacement retains all characters. No live email, deployment, or production template change was performed.

## User interface clarification — 2026-10-26

The user clarified that editing must open as its own feature in a modal or separate page, rather than remain embedded at the bottom of Invoice Profile. Use dedicated desktop/mobile template editor pages, reachable from Settings, the Invoices menu, and a compact Edit Email Template action. Give templates their own save/reset/preview state and template-only persistence; unrelated business details must save independently. This supersedes the original inline Email Template section arrangement.

The dedicated-page revision includes template-only persistence and business-profile SQL updates that omit template columns when omitted by the caller. Full checks pass: 38,790 frontend tests, all Go packages, Vue type checking, Go vet, lint and production build. Browser checks verify navigation, side-by-side desktop preview, independent save/reload/default restoration and clean state after saving. Shared editor editability changes do not emit content updates, and mobile editor surfaces/toolbar controls follow Framework7 styling.
