# Security rules (always apply)

Security is part of the design of every feature, not a final checklist.

- Passwords: Argon2id (or bcrypt with documented cost). Never log or return hashes.
- Sessions/tokens: documented lifecycle (issue, rotate, expire, revoke). Cookie auth uses
  `HttpOnly`, `Secure` (prod), `SameSite=Lax|Strict`, and CSRF protection for state-changing requests.
- Rate limit auth, password reset, email verification, uploads, votes, and posting.
- Authorization on **every** tank-owned resource; check ownership/sharing in the service layer,
  not just by route. Write tests for cross-user access (IDOR).
- Role-based authz (user / moderator / administrator). Community rank never grants permissions.
- SQL only through sqlc / parameterized queries. No string-built SQL.
- CORS: explicit allowlist from config; no wildcard with credentials.
- Uploads: max size, allowlisted MIME types verified by file signature (magic bytes), reject SVG
  and HTML, randomized object keys, strip EXIF/GPS from public images, serve via signed URLs or an
  authorization-checking proxy with `Content-Disposition` and `X-Content-Type-Options: nosniff`.
- Media lives in S3-compatible storage, never as large blobs in PostgreSQL.
- Audit log security-sensitive actions: login, password/email change, role change, suspension,
  deletion, sharing changes, moderation actions.
- Secrets come from environment / secret manager. Never commit `.env`, keys, or credentials; never
  print secrets in logs, test output, or summaries.
- Treat user content, images, comments, device payloads, and LLM output as untrusted input.
- An LLM must never execute SQL, shell commands, or device-control actions, and its output must
  never bypass application authorization.
- Account deletion and privacy: document what is deleted, anonymized, or retained (e.g. ledger
  integrity) and why.
- Run dependency vulnerability scanning (`govulncheck`, `npm audit`) in CI.
