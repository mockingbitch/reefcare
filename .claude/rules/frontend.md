---
paths:
  - "web/**"
---

# Frontend rules (SvelteKit + TypeScript)

- The UI is a purpose-built reef tool, not a generic admin dashboard. Mobile-first, responsive.
- The Go backend owns business rules; the frontend validates for UX only.
- Every data view has loading, empty, error, and success states.
- Reusable components for forms, tables, charts, dialogs, navigation.
- Accessibility (WCAG 2.2 AA where practical): semantic HTML, keyboard navigation, focus
  management in dialogs, visible focus, sufficient contrast, labels for all inputs.
- Never communicate status by color alone — include text/icon and accessible labels
  (e.g. "Above target", "Stale: 9 days").
- Charts include a text summary/table alternative, target-range bands, freshness indicators, and
  event annotations (dosing, water changes, equipment changes).
- Show timestamps in the user's timezone; send/receive UTC from the API.
- i18n from the start: Vietnamese first, English ready; no hard-coded user-facing strings.
- Any future hardware-affecting action (dosing pumps) requires explicit confirmation and warnings.
- Run `npm run check`, lint, and tests before reporting frontend work as done.
