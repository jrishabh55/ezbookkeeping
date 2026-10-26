# Dedicated Invoice Email Template Page

**Goal:** Move the template editor out of the bottom of Invoice Profile and make it a separate, discoverable feature with its own save/reset flow.

**Architecture:** Dedicated desktop and mobile editor routes reuse the existing rich form and server preview. A narrow template-update API saves only email fields; business-profile saves omit template fields. Shared draft state validates templates independently of business details.

**User clarification:** 2026-10-26: template editing should open as its own modal/page, not remain inline below unrelated profile fields. Use a dedicated page because the complete editor and preview need room. Proceed within the user's requested revision and prior parallel-agent authorization.

## Contracts and ownership

- Desktop route `/settings/invoice_email_template`; mobile route `/invoice/email_template`.
- New state `useInvoiceEmailTemplatePageBase()` exports `loading`, `saving`, `newTemplate` (`{subject:string,bodyHtml:string}`), `emailTemplateValid`, `inputIsNotChanged`, `inputInvalidProblemMessage`, `inputIsInvalid`, `setProfile(profile:InvoiceProfile)`, `reset()`.
- Pages load `invoicesStore.loadInvoiceProfile({force:true})`, call `setProfile`, and save via `invoicesStore.updateInvoiceEmailTemplate(newTemplate.value)`, receiving the updated private InvoiceProfile.
- Authenticated POST `invoice_profile/email_template/update.json` accepts `{subject:string,bodyHtml:string}` (explicit empty body allowed) and returns private InvoiceProfile response. Preserve every business/profile field and user ownership; apply existing template sanitization/validation.
- Existing business-profile saves explicitly omit `emailSubjectTemplate`/`emailBodyTemplate`, preserving templates changed through the dedicated editor even if a cached business form is later saved.
- Desktop gets a wider editor/preview split layout through optional `split` prop on InvoiceEmailTemplateForm; compose retains its current stacked layout. Mobile stacks editor and preview on its own page.
- Save/reset are page-owned, pending/invalid preview blocks template save. Business-profile save has no preview dependency. Keep placeholders unresolved in drafts; rendering, send behavior, HTML validation, and PDF attachments are unchanged.
- Provide clear entry points in Settings navigation, Invoices menu, and a compact Edit Email Template action near the Invoice Profile heading/top. Remove the embedded editor from both profile pages.
- No live email/deployment. No new dependencies. Existing managed worktree/branch reused; base6d299475.

## Task 1 — Backend template-only save (backend agent)

- [x] Add failing service/API regressions: template-only save preserves all profile details, explicit empty body, new-profile defaults, invalid templates rejected without mutation, ownership scoped to current user.
- [x] Implement request/handler/service and route in existing invoice files; use a transaction and update only template columns on existing profiles. Reuse the renderer's validation/canonicalization. Do not use an all-column update of a stale profile snapshot.
- [x] Run focused Go services/API/model/renderer tests; report RED/GREEN. Own Go/cmd only, no TypeScript files or commits.

## Task 2 — Desktop dedicated editor (desktop agent)

- [x] Add InvoiceEmailTemplateSettingPage.vue, authenticated route, Settings nav and Invoices menu links, and compact top-profile Edit Email Template action. Remove inline editor/import/preview state from InvoiceProfileSettingPage.vue.
- [x] Dedicated card title/actions provide Back, Save Changes, Reset; load/save/error states use shared base/store contracts. Use the shared form with `split` and `allow-restore`. Saving stays on this page and marks the saved draft clean.
- [x] Use repo UI conventions and local i18n keys; report new strings to parent. Own desktop pages/router only, no common form/base/store/model or mobile edits.
- [x] Run owned-file ESLint; parent runs integrated typecheck/tests/build/browser checks.

## Task 3 — Mobile dedicated editor (mobile agent)

- [x] Add EmailTemplatePage.vue, authenticated route, Settings and Invoices menu entries, compact top-profile Edit Email Template link. Remove bottom inline editor/import/preview state from ProfilePage.vue.
- [x] Navbar Back/Save and standalone shared form with Restore Default/Reset. Use shared base/store contracts, errors/loading states, and remain on editor after save.
- [x] Own mobile page/router/settings/list/profile files only. Report i18n keys. No dependencies, shared files, or commits.
- [x] Run owned-file ESLint; parent runs integrated checks/browser.

## Task 4 — Shared contracts, draft state, layout, verification (parent)

- [x] Add failing regressions that business-profile save/validation is independent of preview; dedicated draft validates preview/length, reset, unresolved variables, intentional empty body. Retarget existing template validation tests to dedicated state.
- [x] Add template-update request/service/store; ensure business saves omit template fields. Implement independent shared template draft state and remove email checks from business profile base.
- [x] Add responsive split option to shared form; default compose layout unchanged. Test actual mounted structure/preview behavior where meaningful. Update English locale and spec clarification.
- [x] Run full frontend suite, typecheck, changed-file ESLint, production build and focused Go suite. Independently review revision. Verify production desktop/mobile navigation, profile has no embedded editor, dedicated page previews/saves/reloads/reset, and unrelated settings remain independently saveable.
- [x] Commit revision, keep branch/worktree for review, clean temporary QA server. No merge/push/deploy without user request.
