# ReefCare — Web Frontend Proposal (milestone 1.8)

Status: **Accepted 2026-10-30 and implemented** in `web/` (see [web/README.md](../web/README.md)).
The three decisions in §5 were accepted as proposed.

## 1. Scope of 1.8

Exit criterion of Phase 1: a user can register, verify, log in, create and share a tank in the
browser.

| Area | Pages |
|---|---|
| Auth | `/register`, `/verify-email`, `/login`, `/forgot-password`, `/reset-password` (paths already used in emails) |
| Account | `/settings/profile` (PATCH /me), `/settings/security` (password change, sign out everywhere, delete account) |
| Tanks | `/tanks` (list, include shared/archived), `/tanks/new`, `/tanks/[id]` (detail), `/tanks/[id]/edit`, `/tanks/[id]/sharing` |
| Shell | layout with navigation, language switch (vi/en), signed-in menu, 404/500 pages |

Out of scope: dashboard, charts, water tests and everything else in Phase 2.10.

## 2. Stack

| Concern | Choice | Why |
|---|---|---|
| Framework | SvelteKit 3 + Svelte 5 (runes) + TypeScript (strict) | Stack in CLAUDE.md; SSR-capable, small runtime |
| Rendering | SPA mode (`adapter-static` with `200.html` fallback, `ssr = false`) served by Caddy next to the API | The API is same-origin and cookie-authenticated; no Node server to run or secure; strict CSP is simpler without SSR. Revisit if public community pages need SEO (Phase 4) |
| API client | Thin typed `fetch` wrapper; types generated from `api/openapi.yaml` with `openapi-typescript` | One source of truth; `npm run check` fails when the contract changes |
| i18n | `paraglide-js` (compile-time messages, tree-shaken) | Type-safe keys, no runtime string lookup; `vi` default, `en` second |
| Forms | Native forms + progressive enhancement; server field errors mapped by `details[].field`/`code` to localized messages | Backend owns validation; client checks only for UX |
| Styling | Plain CSS with design tokens (custom properties), no UI kit | Small, accessible components we control; avoids a heavy dependency |
| Tests | Vitest (units), Playwright (auth + tank flows against the compose stack), `@axe-core/playwright` for accessibility | Matches the Definition of Done |
| Lint | `eslint` + `eslint-plugin-svelte`, `prettier`, `svelte-check` | `npm run check`, `npm run lint` in CI |

Dev: `npm run dev` on `:4001` (the dev `PUBLIC_BASE_URL`), Vite proxies `/api` to `:4000` so
cookies stay same-origin.

## 3. Security requirements

- Tokens from email links are read from `location.hash` (`#token=`), immediately removed with
  `history.replaceState`, and the auth pages set `<meta name="referrer" content="no-referrer">`.
- Every non-GET request sends `X-CSRF-Token` from the `rc_csrf` / `__Host-rc_csrf` cookie; on
  401 the app clears state and redirects to `/login?next=` (same-origin paths only).
- Caddy sets the headers from docs/security.md §4: strict CSP (`default-src 'self'`,
  `frame-ancestors 'none'`, no inline scripts — SvelteKit's `kit.csp` hashes), HSTS,
  `Permissions-Policy`, `Referrer-Policy`.
- User-written text is rendered as text, never `{@html}`.

## 4. UX requirements (from .claude/rules/frontend.md)

- Mobile-first; every data view has loading, empty, error and success states.
- Status never by color alone; visible focus; labels on every input; dialogs trap focus.
- Times rendered in the profile timezone with `Intl.DateTimeFormat`; API values stay UTC.
- Decimal inputs are sent as strings (trimmed; a single decimal comma becomes a dot); the UI warns when the
  system volume is below the display volume (allowed by the API, US-TK-1 AC1).

## 5. Decisions (accepted 2026-10-30)

1. SPA (`adapter-static`), not SSR.
2. `paraglide-js`, locale strategy `localStorage → preferredLanguage → baseLocale (vi)`; the
   profile's `locale` wins after sign-in and the language switcher updates the profile.
3. No component library; plain CSS with tokens in `src/app.css`.

## 6. Implementation notes

- Dev and preview servers proxy `/api` to the Go API, so the browser sees one origin; in
  development `PUBLIC_BASE_URL` is the Vite origin (`compose.yaml`).
- CSP is emitted as a `<meta>` tag in `200.html` with a hash for SvelteKit's bootstrap script.
  `frame-ancestors`, HSTS and the other response headers still belong to Caddy (open item L4).
- Decimal inputs accept a single comma as decimal separator (`60,5` → `60.5`) and display with
  the locale's separator; values are never converted to floats.
- Tests: unit (vitest, node), component (vitest browser mode, Chromium), end-to-end
  (Playwright with the real API, PostgreSQL and Mailpit, plus axe WCAG 2.2 AA checks).
- Not done yet: Caddy config for serving `build/` (L4), password-manager friendly
  `autocomplete` audit on every form, dark-mode screenshots in e2e.
