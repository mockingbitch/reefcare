# ReefCare — API Guidelines and Initial Contracts

Status: accepted (Phase 0); implemented endpoints are specified in `api/openapi.yaml`
(OpenAPI 3.1), which must match the implementation (`internal/app/openapi_test.go`).

## 1. Conventions

- Base path `/api/v1`. JSON only (`Content-Type: application/json; charset=utf-8`).
- Field names `snake_case`. IDs are UUID strings. Timestamps RFC 3339 in UTC (`2026-10-29T03:15:00Z`).
  Dates `YYYY-MM-DD`. Decimal quantities are JSON **strings** (`"8.3"`) to preserve precision.
- Single resources are returned unwrapped; collections as
  `{"items": [...], "next_cursor": "opaque" | null}`.
- `PATCH` uses JSON merge semantics on an explicit allowlist of fields; unknown fields → 400.
  Contested resources require `version` in the body; mismatch → 409 `version_conflict`.
- Pagination: `?limit=` (default 50, max 200) and `?cursor=` (opaque, base64 of the keyset).
- Filtering: explicit named query params (`from`, `to`, `category`, `parameter`); sorting only on
  documented fields (`?sort=-measured_at`).
- Retries: `Idempotency-Key` header supported on `POST` endpoints marked *idempotent*. Same key +
  different body → 422 `idempotency_key_reused`.

## 2. Errors

```json
{
  "error": {
    "code": "validation_failed",
    "message": "Some fields are invalid.",
    "details": [{"field": "readings[0].value", "code": "out_of_range", "params": {"min": "0", "max": "30"}}],
    "request_id": "01JB..."
  }
}
```

`message` is a human fallback; clients localize using `code` and `details[].code`.

| Status | Codes |
|---|---|
| 400 | `malformed_json`, `unknown_field`, `invalid_query` |
| 401 | `unauthenticated`, `invalid_credentials`, `session_expired` |
| 403 | `forbidden`, `csrf_failed`, `email_not_verified`, `account_suspended` |
| 404 | `not_found` (also used for resources the caller may not see) |
| 405 | `method_not_allowed` (with `Allow` header) |
| 413 | `body_too_large` |
| 415 | `unsupported_media_type` |
| 409 | `version_conflict`, `already_resolved`, `quantity_underflow`, `duplicate` |
| 422 | `validation_failed`, `unsupported_unit`, `weak_password`, `idempotency_key_reused` |
| 429 | `rate_limited` (with `Retry-After`) |
| 500 | `internal_error` |

## 3. Authentication

- Cookie `__Host-rc_session` (opaque token, `HttpOnly`, `Secure`, `SameSite=Lax`, `Path=/`); plain
  `rc_session` when the API runs without https in local development.
- CSRF: on login the API returns `csrf_token` and sets readable cookie `__Host-rc_csrf`; every
  authenticated non-GET request must send header `X-CSRF-Token` equal to it (a per-session
  random secret, compared in constant time). All non-GET `/api/v1` requests whose `Origin`
  is not `PUBLIC_BASE_URL`'s origin or listed in `ALLOWED_ORIGINS` — or that browsers mark
  `Sec-Fetch-Site: cross-site` — are rejected with 403 `csrf_failed`, which also covers the
  unauthenticated login/register/reset endpoints.
- API clients for devices (later) use per-integration bearer tokens with narrow scopes.

## 4. Authorization notation

`[public]`, `[auth]` (any logged-in user), `[owner]` (tank owner), `[reader]` (owner or viewer
share), `[verified]` (verified email), `[mod]`, `[admin]`.

## 5. Initial endpoints

### Health
| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | public | liveness |
| GET | `/readyz` | public | DB + migration version |

### Auth & profile
| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/api/v1/auth/register` | public | 202; rate limited |
| POST | `/api/v1/auth/verify-email` | public | `{token}` → 200; invalid/expired/used → 422 `invalid_token` |
| POST | `/api/v1/auth/verify-email/resend` | auth | 202; 409 `already_verified`; 3/hour per user |
| POST | `/api/v1/auth/login` | public | 200 `{user, csrf_token}` + cookie; rate limited |
| POST | `/api/v1/auth/logout` | auth | 204 |
| POST | `/api/v1/auth/logout-all` | auth | 204; revokes every session including the current one; audited |
| POST | `/api/v1/auth/password-reset/request` | public | 202 always |
| POST | `/api/v1/auth/password-reset/confirm` | public | `{token, new_password}` → 204, revokes all sessions; 10/15 min per IP |
| GET | `/api/v1/me` | auth | profile + preferences + roles |
| PATCH | `/api/v1/me` | auth | merge patch, `version` required: display_name, username, bio, experience_level, locale, timezone, unit_prefs, notification_prefs (preference keys merge; `null` removes a key) → 200 user; 409 `version_conflict` / `username_taken` |
| POST | `/api/v1/me/password` | auth | `{current_password, new_password}` → 200 `{csrf_token}`; revokes other sessions, rotates the current session (new cookies + CSRF token), emails a notification; wrong current password → 422 field `current_password` |
| DELETE | `/api/v1/me` | auth | `{password}` → 202 `{status: "deactivated", erase_after}`; deactivates, revokes all sessions and pending email links, emails a confirmation; erasure after 30 days (job, open item J4) |

```jsonc
// POST /api/v1/auth/register
{"email": "lan@example.com", "password": "…", "display_name": "Lan", "locale": "vi", "timezone": "Asia/Ho_Chi_Minh"}
// 202 {"status": "verification_sent"}
```

### Tanks
| Method | Path | Auth |
|---|---|---|
| GET | `/api/v1/tanks` | auth — owned tanks, newest first; `?include=shared,archived` adds shared and archived tanks; each item has `access: owner\|viewer` |
| POST | `/api/v1/tanks` | auth — 201 + `Location`; max 100 tanks per owner |
| GET | `/api/v1/tanks/{tankID}` | reader |
| PATCH | `/api/v1/tanks/{tankID}` | owner (`version` required); archived → 409 `tank_archived` |
| POST | `/api/v1/tanks/{tankID}/archive` | owner — terminal and idempotent |
| GET | `/api/v1/tanks/{tankID}/shares` | owner — grantee id, display name, username (never the email) |
| POST | `/api/v1/tanks/{tankID}/shares` | owner with verified email — `{email}` or `{username}`; 30/hour per owner; max 20 per tank |
| DELETE | `/api/v1/tanks/{tankID}/shares/{userID}` | owner, or the viewer removing themselves |

A malformed `tankID` is answered like an unknown one (404). Viewers get 403 `forbidden` on
writes; anyone else gets 404.
| GET | `/api/v1/tanks/{tankID}/dashboard` | reader — latest readings, freshness, upcoming tasks, recent events (one round trip) |
| GET | `/api/v1/tanks/{tankID}/timeline` | reader — `?category=dosing,livestock&from=&to=&cursor=` |

```jsonc
// POST /api/v1/tanks
{
  "name": "Nano SPS 60",
  "tank_type": "sps_dominant",
  "display_volume_l": "60",
  "sump_volume_l": "25",
  "system_volume_l": "72",
  "dimensions_cm": {"length": "60", "width": "40", "height": "35"},
  "setup_date": "2025-06-01",
  "status": "active",
  "salt_mix": "Red Sea Coral Pro",
  "water_source": "RO/DI"
}
// 201 → Tank {id, owner_id, ..., version: 1, created_at, updated_at}
```

### Water parameters
| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/api/v1/parameter-types` | auth | catalog, units, bounds |
| GET | `/api/v1/tanks/{tankID}/water-tests` | reader | `?from&to&parameter&cursor` |
| POST | `/api/v1/tanks/{tankID}/water-tests` | owner | *idempotent* |
| GET | `/api/v1/water-tests/{testID}` | reader | |
| POST | `/api/v1/readings/{readingID}/corrections` | owner | `{value, unit, reason}` |
| GET | `/api/v1/tanks/{tankID}/water-tests/trends` | reader | `?parameters=alkalinity,calcium&from&to` |
| GET | `/api/v1/tanks/{tankID}/parameter-targets` | reader | current targets |
| PUT | `/api/v1/tanks/{tankID}/parameter-targets` | owner | full replacement → new versions where changed |

```jsonc
// POST /api/v1/tanks/{tankID}/water-tests
{
  "measured_at": "2026-10-29T01:30:00Z",
  "notes": "Before water change",
  "readings": [
    {"parameter": "alkalinity", "value": "8.4", "unit": "dKH", "method": "titration", "test_kit": "Hanna HI772"},
    {"parameter": "salinity", "value": "1.0255", "unit": "SG", "method": "refractometer"},
    {"parameter": "nitrate", "value": "5", "unit": "mg/L NO3", "method": "colorimetric"}
  ]
}
// 201 → {id, measured_at, readings: [{id, parameter, input_value, input_unit, canonical_value,
//        canonical_unit, status, flags: ["outside_target"]}], assessment_job: "queued" | null}

// GET /trends?parameters=alkalinity&from=2026-09-01T00:00:00Z&to=2026-10-29T00:00:00Z
{
  "series": [{
    "parameter": "alkalinity", "unit": "dKH",
    "points": [{"t": "2026-09-02T01:00:00Z", "v": "8.6", "reading_id": "…"}],
    "targets": [{"valid_from": "…", "valid_to": null, "min": "7.8", "ideal": "8.3", "max": "8.8"}],
    "stats": {"count": 12, "min": "7.9", "max": "8.7", "mean": "8.31", "slope_per_day": "-0.012"}
  }],
  "annotations": [{"t": "…", "category": "water_change", "event_type": "water_change.completed", "source_id": "…"}]
}
```

### Dosing
| Method | Path | Auth | Notes |
|---|---|---|---|
| GET/POST | `/api/v1/dosing-products` | auth | global + own; create = private product |
| GET/POST | `/api/v1/product-stocks` | auth | own stocks |
| GET/POST | `/api/v1/tanks/{tankID}/dosing-plans` | reader/owner | |
| PATCH | `/api/v1/dosing-plans/{planID}` | owner | amount, schedule, notes, `version` |
| POST | `/api/v1/dosing-plans/{planID}/pause` · `/resume` · `/complete` | owner | |
| GET | `/api/v1/tanks/{tankID}/dosing-history` | reader | events, `?status&from&to` |
| POST | `/api/v1/dosing-events/{eventID}/resolve` | owner | *idempotent*; `{status, actual_amount, executed_at, reason}` |
| POST | `/api/v1/tanks/{tankID}/dosing-events` | owner | ad-hoc dose; *idempotent* |

```jsonc
// POST /api/v1/tanks/{tankID}/dosing-plans
{
  "product_id": "…", "amount": "12", "amount_unit": "ml",
  "schedule": {"rrule": "FREQ=DAILY;BYHOUR=8,20;BYMINUTE=0", "timezone": "Asia/Ho_Chi_Minh"},
  "starts_on": "2026-11-01", "ends_on": null,
  "delivery_method": "manual", "target_parameters": ["alkalinity"], "stock_id": null
}
```

### Water changes
| Method | Path | Auth |
|---|---|---|
| GET/POST | `/api/v1/tanks/{tankID}/water-change-schedules` | reader/owner |
| PATCH | `/api/v1/water-change-schedules/{scheduleID}` | owner |
| GET | `/api/v1/tanks/{tankID}/water-changes` | reader — `?status=overdue,scheduled,completed` |
| POST | `/api/v1/tanks/{tankID}/water-changes` | owner — record unscheduled change |
| POST | `/api/v1/water-changes/{eventID}/complete` | owner — *idempotent* |
| POST | `/api/v1/water-changes/{eventID}/skip` | owner — reason required |

Every schedule and water change includes `estimate: {fraction_of_system: "0.139", system_volume_l: "72",
assumptions: ["complete_mixing", "system_volume_as_entered"]}` (assumption codes are localized by the
client). `?status=` accepts `overdue`, `scheduled` (upcoming, not overdue), `completed`, `skipped`;
`overdue` is derived (scheduled and > 24 h late) and never auto-completed. `complete`/`skip` are
naturally idempotent (repeating the same request returns 200; a different one 409
`already_resolved`); the `Idempotency-Key` header is not implemented yet (open item W4). Schedules:
`PATCH /water-change-schedules/{id}` with `version`; `status` `active|paused|ended` (`ended` terminal).

### Equipment, maintenance, livestock (Phase 2)
- `GET/POST /tanks/{tankID}/equipment`, `GET/PATCH /equipment/{id}`, `POST /equipment/{id}/remove`
- `GET/POST /equipment/{id}/configurations`
- `GET/POST /tanks/{tankID}/maintenance-plans`, `POST /maintenance-plans/{id}/events`
- `GET/POST /tanks/{tankID}/livestock`, `GET/PATCH /livestock/{id}`,
  `GET/POST /livestock/{id}/events`, `POST /livestock/{id}/reconcile`
- `GET /species?q=` (search curated + own)

### Notifications
- `GET /api/v1/notifications?unread=true`, `POST /api/v1/notifications/{id}/read`,
  `POST /api/v1/notifications/read-all`

### Intelligence (Phase 3)
| Method | Path | Auth | Notes |
|---|---|---|---|
| POST | `/api/v1/tanks/{tankID}/assessments` | owner | 202 + assessment id (job) or 201 when synchronous deterministic run completes < 2 s |
| GET | `/api/v1/tanks/{tankID}/assessments` | reader | |
| GET | `/api/v1/assessments/{assessmentID}` | reader of its tank | |
| POST | `/api/v1/assessments/{assessmentID}/feedback` | owner | |
| GET | `/api/v1/knowledge/articles/{slug}` | auth | published versions only |

### Community and reputation (Phase 4, contracts to be detailed before implementation)
`/posts`, `/posts/{id}/comments`, `/votes`, `/posts/{id}/accept`, `/posts/{id}/status`,
`/posts/{id}/updates`, `/media/uploads` (presigned), `/reports`,
`/users/{id}/reputation`, `/users/{id}/badges`, `/leaderboards`.

## 6. Rate limits (initial)

| Endpoint group | Limit |
|---|---|
| login | 10 / 15 min per email, 30 / 15 min per IP |
| register, password reset | 5 / hour per IP and per email |
| writes (general) | 120 / min per user (all non-GET requests of a signed-in user) |
| tank shares | 30 / hour per owner (the answer reveals whether an account exists) |
| on-demand assessments | 10 / day per tank |
| community posts / comments / votes (R2) | 10 / 60 / 200 per hour per user |

Per-IP limits key on `httpserver.RateLimitKey` (full IPv4 address, IPv6 /64). Each
limiter caps tracked keys and fails closed when full.
Implemented in-process (token bucket keyed in memory) for a single instance; moved to PostgreSQL
or Redis only when running multiple instances (decision D7).
