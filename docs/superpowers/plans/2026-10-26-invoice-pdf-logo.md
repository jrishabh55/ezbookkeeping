# Invoice PDF and logo implementation plan

**Goal:** Save real invoice PDFs directly, match the supplied Codeation invoice, and control the uploaded logo globally.

**Architecture:** Extend the existing profile and renderer, expose the renderer through the authenticated API, and use a Blob download on desktop and mobile.

**Spec:** `docs/superpowers/specs/2026-10-26-invoice-pdf-logo-design.md`

## Constraints

- `hideLogo` defaults to false; uploaded image survives either setting.
- Hidden logos are absent from public response data and generated PDFs.
- Keep one renderer for authenticated/public PDF downloads and email attachments.
- Preserve all amounts, taxes, currencies and existing invoice records.
- Use the existing PNG/JPEG upload limit. Use embedded serif fonts supporting ₹.
- Work inline in the managed worktree; take one independent final code review.

## Review focus

- Setting true and then false must persist both values.
- Public responses must omit hidden image data without destroying the private profile image.
- Auth failures or HTML/JSON errors must never download as a PDF.
- Long descriptions/notes and many lines must fit across bordered pages.
- The download must use authenticated headers and a safe filename, with no login token in its URL.

## Task 1: Logo visibility

- [ ] Add regression tests in `pkg/services/invoices_test.go`, `pkg/api/invoices_test.go`, `pkg/invoicepdf/invoicepdf_test.go` and `src/models/__tests__/invoice.test.ts`. Verify hide/re-enable preserves the uploaded image; public image is empty when hidden; hidden PDF contains no image object; profile cloning and update payload retain the flag.
- [ ] Run targeted tests and observe failures before implementation.
- [ ] Add `HideLogo bool` (`NOT NULL DEFAULT false`) to the Go profile, update request/response and API mapping. Respect it in the renderer/public response. Add `hideLogo` to the TS profile and Show logo controls to desktop/mobile settings and preview.
- [ ] Run `go test ./pkg/services ./pkg/api ./pkg/invoicepdf -count=1` and `npx vitest run src/models/__tests__/invoice.test.ts`.

## Task 2: Download and reference layout

- [ ] Add download-response regressions: valid `%PDF-` Blob is returned, JSON/HTML/malformed responses are rejected. Verify the authenticated PDF endpoint through a live API request after deployment.
- [ ] Expose `/api/v1/invoices/pdf/:id` using the existing PDF handler and header auth. Fetch Blob data through services/store, validate before saving with `getInvoicePdfFileName`, and surface failures in desktop/mobile. Make Download PDF visible and rename the print action to Print.
- [ ] Update the Go and Vue document layouts to the reference: page border, serif fonts, Codeation-sized logo, decorative title and appropriate whitespace. Keep existing tax/quantity/currency behavior.
- [ ] Generate samples with logo shown/hidden, GST, long text and multiple pages. Render and visually inspect with Poppler.
- [ ] Run Go build/vet and `BUILD_PIPELINE=1 go test ./...`; run vue-tsc, ESLint and the full Vitest suite.
- [ ] Take one independent code review, fix material findings, commit and integrate into the existing deploy branch. Wait for GHCR, back up SQLite, deploy and verify live PDFs/settings; do not repeat the prior quantity migration.
- [ ] Upload the supplied Codeation logo into the live profile, preserving other fields. Remove temporary records and revoke verification sessions. Update existing handoff notes.
