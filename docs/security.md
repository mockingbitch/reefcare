# ReefCare — Security Design

Status: **Draft for review** (Phase 0)

## 1. Threat model (summary)

| Asset | Threats | Primary controls |
|---|---|---|
| Accounts | credential stuffing, brute force, session theft, account enumeration | Argon2id, rate limits, uniform responses, HttpOnly cookies, session revocation |
| Private tank data | IDOR, over-sharing in public views/cases | Service-layer authorization, 404 on no access, explicit share/snapshot selection |
| Community integrity | vote rings, spam, reputation farming | DB uniqueness, ledger idempotency, rate limits, moderation |
| Media | malicious files, XSS via SVG/HTML, privacy leaks via EXIF | Signature validation, re-encoding, allowlist, EXIF stripping, private-by-default |
| Intelligence | prompt injection, data leakage, unsafe advice | Authorized snapshot only, typed output validation, safety rules, no actions |
| Infrastructure | secret leakage, SQL injection, dependency vulns | Env secrets, sqlc params, govulncheck/npm audit, least-privilege DB role |

## 2. Authentication

- Passwords: Argon2id, params `m=64 MiB, t=3, p=2` (benchmark on prod hardware; target 100–250 ms),
  16-byte salt, PHC string format so params can be upgraded on next login (conditional rehash that
  can never overwrite a concurrent password change). Concurrent derivations are bounded by
  `ARGON2_MAX_CONCURRENCY` (default GOMAXPROCS); a request that waits > 3 s for a slot gets 503
  `server_busy`, so a login flood cannot exhaust memory.
- Password policy: ≥ 10 chars, max 128, not equal to the email or its local part, and not on a
  blocklist; no composition rules. **Gap:** the embedded blocklist is a small seed list; a top-100k
  breached-password list must be vendored from a reviewed source before launch.
- Sessions: 32 random bytes → base64url token in cookie; DB stores SHA-256(token). Idle timeout
  14 days, absolute 60 days, sliding `last_seen_at` update at most every 5 min. A new session is
  created on every login (the previous one is revoked). Password reset and change bump
  `users.auth_epoch`, which atomically invalidates all sessions — including logins racing the
  change; change-password re-issues the current session's token and CSRF secret and emails a
  notification.
- Email verification and reset tokens: 32 random bytes, stored hashed, single-use, 24 h / 1 h, at
  most one live token per purpose. Links carry the token in the URL fragment (`#token=`), which is
  never sent to servers or in `Referer`. A completed reset also marks the email verified.
- Login/registration/reset responses identical for existing and non-existing emails; constant-time
  comparison; dummy hash computed when the user does not exist; work that only happens for real
  accounts (failed-login audit, reset-token issue) runs in the background so timing matches.
- Emails never include user-chosen text (display names), and display names reject control and
  format characters.
- Rate-limit keys: full IPv4 address, IPv6 /64, plus a larger per-/48 tier for IPv6. Keys are
  stored as fixed-size digests and validated (e.g. email syntax) before use; a full key table evicts
  approximately-least-recently-used keys rather than locking everyone out.
- MFA: not in MVP; schema leaves room (decision D10).

## 3. Authorization

- Roles: `user` (implicit), `moderator`, `administrator` in `user_roles`. Rank never authorizes.
- `tanks.Authorizer` used by every service method on tank-owned data (see
  [architecture.md](architecture.md) §Authorization). Tests include cross-user access for every
  endpoint (table-driven "other user gets 404" suite).
- Admin access to private tank data only via an explicit, audited support path (not in MVP).
- Suspended users: sessions revoked, all writes 403 `account_suspended`.

## 4. Web protections

- HTTPS only in production (Caddy); HSTS 1 year.
- Cookies: `__Host-rc_session` (`HttpOnly`) and `__Host-rc_csrf` (readable by the frontend), both
  `Secure`, `SameSite=Lax`, `Path=/`, no `Domain`. Without https (local dev only) the `__Host-`
  prefix and `Secure` are dropped.
- CSRF: per-session random synchronizer token, header `X-CSRF-Token` on all authenticated unsafe
  methods (constant-time compare); plus an `Origin` / `Sec-Fetch-Site` check on every unsafe
  `/api/v1` request against `PUBLIC_BASE_URL` and `ALLOWED_ORIGINS` (https-only in production).
  JSON-only bodies mean cross-site forms cannot reach the API without a CORS preflight.
- CORS: no CORS headers are emitted (same origin). If ever needed, use an explicit allowlist; never
  `*` with credentials.
- Headers: `Content-Security-Policy` (strict, nonce-based in SvelteKit), `X-Content-Type-Options:
  nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy` minimal,
  `frame-ancestors 'none'`.
- Request limits: body 1 MiB for JSON, header timeouts, `ReadHeaderTimeout`, `MaxHeaderBytes`.
- User-generated text rendered as text or sanitized Markdown subset (no raw HTML).

## 5. Data access

- Only sqlc-generated, parameterized queries. No dynamic SQL from user input; dynamic sort/filter
  maps to fixed query variants.
- Application DB role: DML only on application tables, no DDL, no superuser. Migrations run with a
  separate migration role.
- `case_snapshots`, `reputation_ledger`, `audit_logs`: UPDATE/DELETE revoked from the app role
  (append-only).

## 6. File uploads (Phase 4 or when photos ship)

1. Client requests an upload → API checks authorization, size (≤ 10 MiB images, video decision
   D8), declared MIME (JPEG, PNG, WebP, HEIC) → returns presigned PUT URL to a *quarantine* prefix
   with random key.
2. Worker validates magic bytes, decodes and **re-encodes** the image (removes EXIF/GPS and
   polyglot payloads), produces variants, moves to final prefix, marks `ready`.
3. SVG, HTML, PDF and unknown types are rejected.
4. Private media served via short-lived presigned GET after authorization; public media via CDN
   path with `Content-Type` from DB and `Content-Disposition: inline; filename="…"`.

## 7. Intelligence and LLM boundary

- The engine receives only the snapshot built after authorization.
- LLM (optional, later) receives a minimized, pseudonymized snapshot; no emails, names, or other
  users' data. Output parsed into the typed model; unknown fields dropped; references validated
  against DB; free text rendered as plain text.
- The LLM has **no tools**: no SQL, shell, HTTP, or device control. Prompt-injection content in user
  notes cannot change authorization because authorization happens before and outside the model.

## 8. Logging and audit

- Never log: passwords, tokens, cookies, `Authorization` headers, reset links, full IPs, email
  bodies. A redaction middleware/handler enforces deny-listed keys.
- User IDs logged as UUIDs; emails never logged.
- Audit events: login success and failure (failures recorded per existing account, written in the background), registration, email verification, password reset request/completion, logout-all, password/email change, role grant/
  revoke, suspension, account deletion, tank share grant/revoke, visibility change, moderation
  actions, admin data access.

## 9. Privacy and account deletion

- Tanks private by default; sharing explicit; public case snapshots show only selected categories.
- Deletion request (`DELETE /api/v1/me`, password required) → account `deactivated`, every
  session and pending email link revoked, confirmation email sent, audited
  (`account.deactivated`), profile hidden and the account's tanks hidden from their viewers
  immediately. The email address stays reserved until erasure (open item S5).
- After 30 days the `identity.erase_account` job (one transaction, idempotent): every module's
  registered eraser deletes the account's private data (today: owned tanks with their timeline
  and shares, and shares granted to the account); sessions, email-link tokens and roles are
  deleted, and queued jobs about the account are removed; the users row is **anonymized, not
  deleted** (email placeholder `erased:<id>` — not an address, so nobody can register it and
  block the erasure — unusable password, "Deleted user", no username/bio/preferences, default
  locale/timezone, `erased_at` set), so
  `audit_logs` (and later `reputation_ledger`) rows keep a valid id that no longer links to
  personal data. The original email address can then be registered again. Events the account
  recorded on other people's tanks stay, attributed to the anonymized row. Later modules add
  their own eraser (`identity.Service.OnErase`); authored public content (Phase 4) will be
  anonymized rather than deleted (decision D12).
- Retained after erasure, on purpose: `audit_logs` rows of the account (action, time, truncated
  `ip_prefix`, `request_id`) — the audit trigger makes them append-only; their retention period
  is open item S12. `users.created_at`/`deactivated_at` stay for integrity checks.
- Signing in to an erased account takes the same path (dummy hash, one no-op insert) as an
  unknown email, so timing does not reveal that an account was erased. Addresses on the
  reserved `.invalid` TLD are rejected at registration.
- Data export (JSON) of own data: Phase 5.

## 10. Secrets and supply chain

- Secrets via environment / platform secret manager; `.env.example` documents names only.
- `govulncheck`, `npm audit`, Dependabot/Renovate (decision D9), pinned container base images,
  minimal distroless runtime image, non-root user.

## 11. Abuse prevention (community)

Self-vote block, unique votes, per-user daily caps on reputation gained from one source user and
one post, delayed reward finalization (e.g. 24 h) to allow reversal, account age/verification gates
for voting, reports queue, moderator actions audited and reversible.
