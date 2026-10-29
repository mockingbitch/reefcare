---
name: assessment-rule
description: Add or change a deterministic Reef Intelligence assessment rule (parameter deviation, instability, dosing mismatch, equipment malfunction, livestock change, missing data) with fixtures, evidence links and safety warnings. Use when working on internal/intelligence rules.
argument-hint: "<rule description>"
---

Add/modify the assessment rule: **$ARGUMENTS**

1. Read `.claude/rules/intelligence-safety.md`, `docs/knowledge-engine.md`, and existing rules in
   `internal/intelligence/`.
2. Specify the rule before coding:
   - Inputs used (parameters, time window, dosing/water-change/equipment/livestock events).
   - Freshness and completeness requirements; what is reported as missing information when unmet.
   - Deterministic condition(s), thresholds, and their source (knowledge article ID or a
     documented product decision — never an invented citation).
   - Output: observations, hypotheses with supporting / missing / contradictory evidence,
     alternative explanations, recommended verifications, safety warnings, confidence label and
     the documented basis for it.
3. Implement as a pure function over the normalized snapshot; no DB, HTTP, or LLM calls inside the
   rule. Bump `rule_version`.
4. Add deterministic fixtures in `testdata/` and table-driven tests covering: triggered, not
   triggered, stale data, missing data, wrong/mixed units, conflicting observations, and that
   safety warnings and knowledge references (real IDs only) are present.
5. Confirm the rule never recommends aggressive correction from a single or stale reading, never
   asserts causation, and never triggers device actions.
6. Ask the `reef-domain-expert` agent to review the chemistry and safety of the rule.
7. Run `/verify` and summarize with actual results.
