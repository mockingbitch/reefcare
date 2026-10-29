---
name: feature
description: Implement one ReefCare vertical feature slice following the project's Inspect → Analyze → Design → Implement → Verify → Review → Summarize workflow. Use when asked to build or extend a backend/frontend feature (e.g. "implement water tests", "add dosing plans").
argument-hint: "<feature or module name>"
---

Implement the feature: **$ARGUMENTS**

Follow these steps in order. Keep the slice small and reviewable.

## 1. Inspect
- Read `CLAUDE.md`, relevant `.claude/rules/*`, `docs/` for this module, existing code, migrations,
  queries, OpenAPI, and tests. Note conventions to follow.
- Confirm the feature belongs to the current roadmap phase. If it belongs to a later phase, say so
  and ask before proceeding.

## 2. Analyze (write it down briefly in the conversation)
User problem · actors & permissions · business rules · data model · API contract · failure cases ·
security implications · acceptance criteria · explicit assumptions.
Ask a question only if a missing decision materially affects correctness; otherwise choose a
default and record it in the relevant doc.

## 3. Design
Short plan: files to create/modify, migration(s), sqlc queries, endpoints, domain logic, tests,
dependencies, risks.

## 4. Implement
- Migration (use `/new-migration`) → queries → `sqlc generate` → domain logic → application
  service (explicit transactions, authz) → handler → OpenAPI → docs.
- Tests alongside: unit for domain rules, integration for constraints/transactions/authz,
  API tests for validation and authorization failures.
- No placeholder code, no speculative abstractions.

## 5. Verify
Run `/verify`. Fix failures and re-run.

## 6. Review
Delegate an independent review to the `go-reviewer` agent (and `db-architect` for schema changes,
`security-auditor` for auth/sharing/uploads/community, `reef-domain-expert` for chemistry or
intelligence logic). Address confirmed findings.

## 7. Summarize
1. What was implemented
2. Important design decisions
3. Files created/modified
4. Database migrations
5. Tests and their **actual** results (passed / failed / skipped / could not run)
6. Known limitations
7. Recommended next step
