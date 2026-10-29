---
paths:
  - "internal/tanks/**"
  - "internal/waterparameters/**"
  - "internal/dosing/**"
  - "internal/waterchanges/**"
  - "internal/equipment/**"
  - "internal/livestock/**"
  - "internal/maintenance/**"
  - "internal/timeline/**"
  - "internal/notifications/**"
  - "docs/domain-model.md"
  - "docs/product-requirements.md"
  - "docs/user-stories.md"
---

# Reef management domain rules

## Tanks
- Store display volume, sump volume, and estimated total system volume separately; calculations
  (dose per volume, water-change %) use system volume and label it as an estimate.
- Tank sharing is explicit and granular; the owner controls visibility of each data category.

## Water parameters
- Each observation: tank, parameter, numeric value, unit, `measured_at`, `recorded_at`, method,
  test kit/device, source (manual/imported/sensor), optional verification status, notes, submitter.
- Validate physical and domain plausibility (e.g. salinity, pH, temperature ranges); implausible
  values are rejected or flagged for confirmation — not silently clamped.
- Unit conversion via explicit, tested functions (dKH ↔ meq/L ↔ ppm CaCO3, °C ↔ °F, SG ↔ ppt ↔ PSU,
  NO3/PO4 ion vs N/P forms). Keep the original value and unit.
- Changing preferred units or targets never rewrites historical observations.
- Corrections create audit history; repeat measurements are separate observations.
- "Unusual change" detection flags; it never asserts a cause.

## Dosing
- Plans (planned amount, unit, schedule + timezone, dates, delivery method, status) are separate
  from events (planned vs actual amount, status, executed_at, manual vs device-reported,
  skip/failure reason, related measurements).
- Never infer chemical concentration from a product's nominal volume without reliable product data.
- Device-reported events are ingested idempotently.

## Water changes and maintenance
- One-time and recurring schedules; missed occurrences become **overdue**, never auto-completed.
- Dilution/replacement calculations state their assumptions (e.g. full mixing, system volume estimate).

## Equipment
- Multiple instances of the same model are allowed. Configuration changes are versioned and emit a
  tank timeline event so analysis can correlate them.

## Livestock
- Additions, deaths, transfers, sales, and corrections are events; current quantity must be
  consistent with event history. Provide reconciliation when history is incomplete.
- Never silently discard historical livestock records.

## Timeline
- Every event has a UTC timestamp, source, category, and tank. Filterable by category and range.
- Snapshots used by cases/assessments are immutable copies, not live references.

## Notifications and jobs
- Scheduled work is persisted in PostgreSQL (job table with `FOR UPDATE SKIP LOCKED` or a
  documented library), deduplicated by key, idempotent, retry-safe, with bounded workers.
- Never rely only on in-memory timers for important reminders.
