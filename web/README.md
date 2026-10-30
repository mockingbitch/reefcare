# ReefCare web

SvelteKit 3 + Svelte 5 + TypeScript single-page app (design: [../docs/frontend.md](../docs/frontend.md)).
Vietnamese first, English second (paraglide-js). The Go API owns every business rule; the UI
only validates for convenience.

## Develop

```sh
make up            # from the repo root: PostgreSQL (:4002), Mailpit (:4003/:4004), API (:4000)
make web-install   # npm ci
make web-dev       # http://localhost:4001 — proxies /api to :4000 (same origin for cookies)
```

Emails (verification, password reset) are in Mailpit at http://localhost:4003; their links point
at :4001 because `compose.yaml` sets `PUBLIC_BASE_URL=http://localhost:4001`.

## Checks

| Command            | What                                                                                                                                                                                 |
| ------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `npm run check`    | compiles messages, `svelte-check` (fails on warnings)                                                                                                                                |
| `npm run lint`     | prettier + eslint                                                                                                                                                                    |
| `npm test`         | unit tests (node) and component tests (headless Chromium; `npx playwright install chromium` once)                                                                                    |
| `npm run build`    | static build into `build/` with a `200.html` SPA fallback and a hash-based CSP                                                                                                       |
| `npm run test:e2e` | Playwright: starts the real API on :4010 (`go run`, migrations applied to the dev database) and the built app on :4011, reads emails from Mailpit; includes axe accessibility checks |
| `npm run gen:api`  | regenerates `src/lib/api/schema.d.ts` from `../api/openapi.yaml` (CI fails when it is stale)                                                                                         |

E2E needs `docker compose up -d db mailpit`. Ports follow the 4000 range in `compose.yaml`;
override with `E2E_API_PORT`, `E2E_WEB_PORT`, `E2E_DATABASE_URL`, `MAILPIT_URL`, `MAILPIT_SMTP_PORT`.

Without Go installed, run the API in Docker and point the tests at it with `E2E_API_URL` (the
API needs `PUBLIC_BASE_URL=http://localhost:4011` so email links reach the test web server):

```sh
docker compose build api && make migrate-up
docker run -d --name reefcare-e2e-api --network reefcare_default -p 127.0.0.1:4010:4010 \
  -e HTTP_ADDR=:4010 -e PUBLIC_BASE_URL=http://localhost:4011 \
  -e DATABASE_URL='postgres://reefcare:reefcare@db:5432/reefcare?sslmode=disable' \
  -e SMTP_HOST=mailpit -e SMTP_PORT=1025 reefcare-api:latest serve
E2E_API_URL=http://localhost:4010 npm run test:e2e
```

E2E accounts use unique `@e2e.reefcare.test` addresses, so runs can share the dev database.

## Layout

```text
messages/{vi,en}.json      every user-facing string (no hard-coded text in components)
src/lib/api/               typed fetch client (CSRF header, error envelope), generated schema types
src/lib/components/        Field, SelectField, Alert, ConfirmDialog (<dialog>), TankForm
src/lib/session.svelte.ts  signed-in user (GET /me), follows the profile language
src/routes/(auth)/         login, register, verify-email, forgot-password, reset-password
src/routes/(app)/          tanks, tank detail/edit/sharing, settings/profile, settings/security
e2e/                       Playwright flows
```

Imports use the `#lib/…` alias with explicit extensions (`#lib/errors.js`, `#lib/session.svelte.js`).
`resolve()` takes route IDs including groups, e.g. `resolve('/(app)/tanks/[id]', { id })`.

## Security notes

- Email-link tokens are read from the `#token=` fragment and removed from history at once; the
  pages set `referrer` to `no-referrer`.
- `X-CSRF-Token` is read from the `__Host-rc_csrf` / `rc_csrf` cookie on every non-GET request.
- Post-login `?next=` accepts same-origin paths only.
- User text is always rendered as text, never `{@html}`. No inline styles (the CSP has no
  `'unsafe-inline'`).
