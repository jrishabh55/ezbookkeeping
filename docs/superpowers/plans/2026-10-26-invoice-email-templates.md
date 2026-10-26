# Invoice Email Templates Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Let users create a complete rich text invoice email template with variables and preview in invoice settings.

**Architecture:** Store one private subject/body template on the invoice profile. A single Go renderer sanitizes and expands variables for both previews and sends. Shared Vue editors serve desktop/mobile settings and compose screens.

**Tech Stack:** Go/xorm, existing golang.org/x/net/html, Vue 3/TypeScript, Tiptap 3.32.0, Vitest.

**Spec:** `docs/superpowers/specs/2026-10-26-invoice-email-template-design.md`

## Global Constraints

- One saved default per user; the complete body, heading, footer, and button are editable/removable.
- Subject templates are at most 256 characters; serialized body templates at most 32,000 characters.
- The server generates the preview and outgoing email with the same renderer; previews have no invoice/public-link side effects.
- Private templates never enter public profile responses; omitted profile-update fields preserve existing templates.
- No live email, deployment, or production settings changes. Preserve PDF/accounting behavior and legacy plain text sends.
- User requested multiple subagents for speed. Run independent file-owned tasks in parallel, with parent coordinating shared interfaces and commits.

## Review Focus

- Editing a profile from an older client cannot erase a saved template (Task 1 regression).
- Empty rich body is intentional, whereas absent rich body permits legacy plain text/default (Task 1 regression).
- Customer values containing HTML or placeholder syntax remain inert text (Task 1 regression).
- Save/reset/reload cannot replace variable keys with sample values (Tasks 2/3 regression).
- Stale preview responses cannot overwrite newer edits or enable sending invalid content (Task 3 regression/manual check).

## Shared Interfaces

- Body variable: `<span data-invoice-variable="customer_name">{{customer_name}}</span>`; key authoritative, fallback canonical token. Subject uses `{{customer_name}}` text.
- Invoice button: standalone `<div data-invoice-button="true" style="text-align: center">View Invoice</div>`; plain text label, server-supplied trusted URL.
- `InvoiceEmailVariable { key: string; label: string; value: string }`.
- POST `invoice_profile/preview_email.json`: `{invoiceId?: string, subject?: string, bodyHtml?: string}`. Omitted overrides use saved/default templates; present empty body means empty.
- Preview result: `{subject: string, bodyHtml: string, emailHtml: string, variables: InvoiceEmailVariable[], defaultSubjectTemplate: string, defaultBodyTemplate: string, sample: boolean}`. `bodyHtml` is the resolved fragment; `emailHtml` is the full email.
- Private profile response/update adds `emailSubjectTemplate`, `emailBodyTemplate`; GET returns effective defaults when unset. Go update fields are presence-aware.
- Send adds optional `bodyHtml`; present wins over legacy `message`, even empty. Neither present uses profile template. New UI always sends rich body; subject and fragment variables resolve on server.
- Editor components accept `v-model` string, `variables?: readonly {key:string,label:string}[]`, `disabled?: boolean`, `ariaLabel?: string`. Subject editor serializes plain text; body editor serializes HTML. Preview is parent-owned, sandboxed iframe `srcdoc` with scripts disabled.

### Task 1: Backend templates, renderer, preview, and send (backend agent)

**Files:** Modify `pkg/models/invoice.go`, `pkg/api/invoices.go`, `pkg/services/invoices.go`, `cmd/webserver.go`, `templates/email/invoice.tmpl`, relevant Go tests; create focused renderer source/tests under `pkg/invoiceemail/`. Add mailer test seam only if needed.

**Interfaces:** Produce Shared Interfaces exactly. Use existing invoicepdf amount/date/balance helpers and invoice snapshots; catalog keys are those in the spec. Expose default preset and variable metadata through preview/profile responses.

- [x] Write renderer regressions: all catalog values, current balance/currency, optional empty values, non-recursive expansion, hostile markup/URLs, unknown variable keys, final subject length/newline validation, button label/destination, editable absence of all summary/footer content. Run before implementation and record expected failure.
- [x] Implement strict fragment parser/sanitizer and variable renderer. Permit supported tags/alignment only. Subject variables expand as text; ordinary links HTTP(S)/mailto; recognized invoice URL variable only in supported hrefs. Strip scripts/handlers/embeds/styles outside allowed alignment. Resolve invoice button server-side and render email-compatible inline styles.
- [x] Add profile persistence, effective defaults, explicit private DTO mappings, omission-preserving updates and server validation. Add save/reload/default/empty/old-client/public-exclusion tests.
- [x] Add authenticated preview route/API/service. Deterministic samples when no invoice ID; owned actual data otherwise. Test cross-user denial, no public-token/time mutation, and same rendering as send.
- [x] Extend send without changing old plain text semantics. Use fake mailer to inspect subject/body/PDF and failed-send timestamp behavior; no real mail.
- [x] Run `/Users/rishabhjain/.local/share/mise/installs/go/1.27.3/bin/go test ./pkg/invoiceemail ./pkg/models ./pkg/services ./pkg/api ./pkg/templates ./pkg/mail -count=1`; report exact results and changed files. Parent commits after review.

### Task 2: Shared rich text editors (editor agent)

**Files:** Create `src/components/common/InvoiceEmailBodyEditor.vue`, `src/components/common/InvoiceEmailSubjectEditor.vue`, focused helper/extension source under `src/lib/`, and tests under `src/lib/__tests__/`. Parent owns dependency edits.

**Interfaces:** Consume/produce Shared Interfaces; do not render or expand variables in the frontend. Body editor emits restricted HTML markers; subject emits a single-line string with canonical tokens.

- [x] Add meaningful failing tests for editor HTML/token round-tripping, unknown marker handling, subject serialization, mention filtering and literal variable values. Use jsdom for real editor/parser behavior.
- [x] Build Tiptap custom variable and button nodes, supported StarterKit features, TextAlign, and Suggestion. Pin 3.32.0 for core/pm/vue-3/starter-kit/extension-text-align/suggestion, installed by parent; no extension-mention needed.
- [x] Build reusable body/subject editors with formatting toolbar, searchable @ and Insert Variable, arrow/Enter/Escape keyboard behavior, links, disabled state and labels, editable invoice button text and movement controls usable on mobile.
- [x] Run focused Vitest, vue-tsc and ESLint on owned files. Document RED/GREEN evidence and any integration caveat. Parent commits after review.

### Task 3: Profile and compose integration (UI agent)

**Files:** Modify desktop `InvoiceProfileSettingPage.vue` and `InvoiceDetailDialog.vue`, mobile invoice `ProfilePage.vue` and `SendPage.vue`, shared base `InvoiceDetailPageBase.ts` and `InvoiceProfilePageBase.ts`; create common `InvoiceEmailTemplateForm.vue` and preview helper/composable/tests if needed. Parent owns models/services/store/locales.

**Interfaces:** Use Task 2 components and preview API through `invoicesStore.previewInvoiceEmail(req)`. `InvoiceProfile` has both template fields and dirty-state serialization. Shared form accepts subject/body v-models, optional invoiceId, disabled, preview state/events as needed.

- [x] Test latest-preview-wins behavior (out-of-order responses), error/pending handling, and compose request preservation with intentionally empty rich body; verify expected failures.
- [x] Add complete template section on desktop/mobile profile, subject/body editors, variable metadata, sample preview, Restore Default Template, save/reset/dirty-state. Save must reject invalid/unknown template content and surface useful error.
- [x] Replace desktop/mobile plain text compose message with shared rich editor and actual-invoice preview. Initialize compose from profile templates without persisting resolved values; edits affect that send only. Use existing prepare public-link flow. Disable send while invalid/current preview unavailable, and surface failures.
- [x] Render complete server email in a sandboxed iframe without scripts; handle mobile viewport. Guard stale async responses and no writes while disabled.
- [x] Run focused tests and ESLint, report edited files and manual verification points. Parent commits after review.

### Task 4: Contracts, dependencies, integration, verification (parent)

**Files:** `src/models/invoice.ts`, `src/lib/services.ts`, `src/stores/invoice.ts`, `src/models/__tests__/invoice.test.ts`, `src/locales/en.json`, `package.json`, plan/spec/ledger; integration fixes assigned back to owners where practical.

- [x] Install pinned editor packages and jsdom, recording dependency resolution. Add profile clone/update tests that fail before adding template fields; implement model fields, preview/send types, services/store method matching Shared Interfaces.
- [x] Coordinate file ownership and share contract changes. Run reviews on each task's diff and resolve material findings, then commit complete feature changes on `codex/invoice-email-templates`.
- [x] Run all Go tests with `BUILD_PIPELINE=1`, frontend Vitest, typecheck, changed-file ESLint, production build. Distinguish any pre-existing failure from new failure using baseline; root baseline passes 28 focused invoice tests and focused Go tests. Worktree avoids unrelated nested `.claude/` test copies.
- [x] Inspect local desktop/mobile template settings, mentions, formatting, preview, defaults, save/reload, actual compose and single-send editing. Use isolated temporary DB/fake SMTP as needed; no live mail.
- [x] Perform independent whole-feature code review after tests, fix material findings, preserve worktree and report final evidence/limitations. Do not publish or merge without user request.
