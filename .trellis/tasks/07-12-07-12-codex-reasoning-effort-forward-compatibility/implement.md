# Execution plan — G13 to G15

## Gate 0 — unified classification (complete before code)

1. [x] Normalize three research reports to the single numbering below; historical G9–G12 are not aliases.
2. [x] Record every delta as G13, G14, G15, existing tool domain/no new delta, or client-only exclusion.
3. [x] Confirm no new public tool/MCP schema in the diff range.

## G13 — Responses reasoning/encrypted-content

Vertical slices:

1. [x] **G13a audit/RED**: public same-protocol fixture for actual `reasoning` plus `include: ["reasoning.encrypted_content"]`; determine omission versus preservation.
2. [x] **G13b implementation if RED**: minimal Responses-native preservation repair only if G13a proves loss.
3. [x] **G13c self-review**: verify Hub does not inject Codex defaults and unknown effort stays open-string on same-family path.
4. [x] **G13 module review**: independent protocol/architecture/code review.

## G14 — reasoning summary and stream options

Vertical slices:

1. [x] **G14a audit/RED**: provided summary + summary delivery option; omitted form remains omitted.
2. [x] **G14b implementation if RED**: repair only preservation/merge loss; never add model catalog capability gate.
3. [x] **G14c self-review**: raw/typed merge and absence behavior.
4. [x] **G14 module review**: independent protocol/architecture/code review.

## G15 — Responses item identity

Vertical slices:

1. [x] **G15a audit/RED**: non-empty supplied id and missing id round-trip fixture at inbound/outbound public seam.
2. [x] **G15b implementation if RED**: smallest adapter-local preservation fix; no Codex prefix validator and no id synthesis.
3. [x] **G15c self-review**: identity, absence, tool/message variants, no accidental cross-protocol claim.
4. [x] **G15 module review**: independent protocol/architecture/code review.

Evidence for G15a/b/c PASS:
- `g15a_input_item_identity_test.go` + `g15a-*.request.json`
- `g15b_input_item_identity_test.go` + `g15b-*.request.json`
- `g15c_reasoning_item_identity_test.go` + `g15c-*.request.json` (standalone / pure standalone / summary-only / reasoning→tool / no cross-protocol invent)

## Final integration

1. [x] Update protocol field matrices and Trellis guideline with G13–G15 evidence and client-only exclusions.
2. [x] Run targeted Go tests from `llm/` only; `git diff --check`.
3. [x] Independent parent review across G13–G15; no P0/P1/P2 finding reopened a slice. See `research/reviews/g13-g15-parent-review.md`.
4. [x] Update spec and make one local, scoped commit: `5c63811d`.

Parent review and scoped commit both passed. This task does not claim the full 101/107 Field ID protocol matrix is complete.
