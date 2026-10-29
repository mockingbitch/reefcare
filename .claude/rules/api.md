---
paths:
  - "api/**"
  - "internal/**/http*.go"
  - "internal/**/handler*.go"
  - "internal/platform/httpserver/**"
  - "docs/api-guidelines.md"
---

# REST API rules

- Prefix: `/api/v1`. JSON request/response, `snake_case` field names, RFC 3339 UTC timestamps.
- Define request/response schemas and authorization rules in `api/openapi.yaml` **before or with**
  the handler. The OpenAPI document must match the implementation.
- Consistent error body, e.g.
  `{"error": {"code": "validation_failed", "message": "...", "details": [...], "request_id": "..."}}`
  with stable machine-readable `code` values.
- Status codes: 200/201/204 success; 400 malformed; 401 unauthenticated; 403 authenticated but not
  allowed; 404 for resources the caller may not know exist (prefer 404 over 403 for private tanks to
  avoid IDOR enumeration); 409 conflict/version mismatch; 422 domain validation; 429 rate limited.
- Collections: cursor pagination (`limit`, `cursor`, response `next_cursor`), bounded `limit`
  (default 50, max 200), explicit allowed sort/filter fields.
- Every route under a tank (`/tanks/{tankID}/...`) and every resource reached by its own ID
  (`/dosing-plans/{planID}`) resolves the owning tank and checks the caller's permission.
- Public endpoints return only fields explicitly marked shareable; never reuse private DTOs.
- `Idempotency-Key` header is supported for retried creates (dosing-event ingestion, water-change
  completion, reward processing) and is persisted with a uniqueness constraint.
- Optimistic concurrency via `version` field or `If-Match` on PATCH of contested resources.
- Every request gets a correlation ID (`X-Request-ID`, generated if absent) returned in responses
  and logs.
- Validate all inputs server-side: types, ranges, enums, lengths, physical plausibility of reef
  parameters. Reject unknown units rather than guessing.
- Health: `/healthz` (liveness) and `/readyz` (DB reachable, migrations applied).
