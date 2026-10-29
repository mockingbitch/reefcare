---
paths:
  - "internal/community/**"
  - "internal/reputation/**"
  - "internal/moderation/**"
---

# Community, Reef Clinic and reputation rules

## Cases
- A diagnostic case stores an immutable snapshot of the shared tank context at posting time; later
  tank changes never alter it.
- The owner chooses exactly which tank data and images are public.
- Lifecycle: open → awaiting_information → under_discussion → resolution_proposed →
  monitoring_outcome → resolved | closed_without_resolution. Inconclusive cases need not be resolved.
- Helpful-answer marking by the case owner; follow-ups, outcomes, and photos are appended updates.

## Votes
- One vote per user per target (DB uniqueness), no self-votes, vote changes are updates not new rows.
- Vote policy (who may downvote, thresholds) is configurable.

## Reputation
- Append-only `reputation_ledger` (user, event type, delta, source entity, reason, created_at,
  reversal/moderation reference). Scores are derived from the ledger; reversals are new entries.
- Reward processing is transactional and idempotent via a unique key on the source event.
- Anti-abuse: block self-voting and duplicates, damp reciprocal voting and single-viral-case rank
  jumps (caps per source/time window), rate limit, risk-based moderation.
- Never penalize users for disagreeing with popular opinion.
- Track lifetime reputation and recent activity separately. Rank thresholds are configurable.
- Rank ≠ role: reputation never grants moderation or admin permissions.

## Moderation
- Reports, actions, and penalties are audited and reversible through new ledger/audit entries.
