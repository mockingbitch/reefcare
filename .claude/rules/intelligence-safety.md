---
paths:
  - "internal/intelligence/**"
  - "internal/knowledge/**"
  - "docs/knowledge-engine.md"
---

# Reef Intelligence and knowledge base rules

The engine produces **provisional assessments**, not diagnoses.

## Pipeline (keep these stages explicit and individually testable)
1. Load the authorized tank context (authorization happens before any data is loaded).
2. Validate completeness and freshness of inputs; record what is missing or stale.
3. Normalize values and units.
4. Compute deterministic metrics (trends, deltas, stability, consumption).
5. Evaluate versioned, explicitly defined rules.
6. Retrieve relevant knowledge articles (respecting provenance and access permissions).
7. Generate a structured assessment (typed model, not free text).
8. Validate that every cited evidence item and knowledge reference exists and supports the claim;
   drop or downgrade unsupported claims.
9. Persist with input snapshot, `rule_version`, `knowledge_version`, `created_at`.
10. Present it and request additional observations where needed.

## Output model
`assessment_id, tank_id, assessment_type, status, summary, observations, hypotheses, evidence,
alternative_explanations, missing_information, recommended_verifications, safety_warnings,
knowledge_references, confidence_label, input_snapshot_version, rule_version, knowledge_version,
created_at`. Each hypothesis lists supporting, missing, and contradictory evidence.

## Mandatory safety rules
- Correlation is never presented as proven causation.
- Never fabricate sources, citations, or knowledge IDs.
- No exact diagnosis from an image alone.
- No aggressive chemical corrections from incomplete data; ask to re-test suspicious values before
  high-impact interventions.
- Never automatically execute dosing or equipment-control commands.
- Highlight urgent livestock welfare concerns and escalation paths.
- Distinguish informational guidance from verified measurements.
- `confidence_label` (low/moderate/high) only with a defined, documented basis; no invented
  probabilities.
- Safety warnings from rules and knowledge must survive into the final output.

## Provider independence
- LLM access sits behind an interface; prompts receive only the authorized snapshot.
- Deterministic metrics, reminders, history, and basic rules work when the LLM is unavailable.
- LLM output is untrusted: parse into the typed model, validate, never let it trigger SQL, shell,
  device actions, or authorization decisions.

## Knowledge base
- Versioned articles with stable IDs, sources (title, publisher, URL, date), review status,
  reviewer, and history.
- Classify each claim: established/manufacturer-supported, expert interpretation, community
  experience, unverified hypothesis. Community anecdotes are never auto-promoted to rules.
- Do not invent reef chemistry facts in seed content; mark uncertain content as unverified and
  leave sources empty rather than fabricating them.
