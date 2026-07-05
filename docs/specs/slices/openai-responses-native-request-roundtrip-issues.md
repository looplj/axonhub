# OpenAI Responses Native Request Round-Trip Slices

Parent PRD: `docs/specs/openai-responses-native-roundtrip-prd.md`

This document breaks the first implementation phase into small, reviewable slices. Each slice should be handled through the agent workflow loop:

```text
/tdd → /diagnosing-bugs when needed → review slice → report → next slice
```

If review fails, convert the finding into the smallest red condition and return to `/tdd` or `/diagnosing-bugs`.

## Slice 1 — Establish the request round-trip seam

**Blocked by:** None - can start immediately.

**User stories covered:** 10, 17, 19, 20, 21.

### What to build

Identify and prove the highest practical test seam for OpenAI Responses request round-trip. The seam should exercise inbound request parsing, native request preservation, outbound request reconstruction, and same-protocol OpenAI Responses emission without testing private helpers directly.

### Acceptance criteria

- [ ] The test seam is documented in the slice report.
- [ ] A minimal ordinary Responses request round-trip test exists.
- [ ] The test fixture includes `model`, ordinary `input`, and `stream`.
- [ ] The test can be run with one targeted command.
- [ ] No production behavior changes are made beyond what is necessary to establish the seam.

### Skill loop

Use `/tdd` planning first. If no correct seam exists, document that as an architecture finding before creating lower-level tests.

---

## Slice 2 — Preserve unknown top-level request fields

**Blocked by:** Slice 1.

**User stories covered:** 10, 11, 13, 20, 21.

### What to build

Add request-level raw fallback for top-level OpenAI Responses fields that AxonHub does not yet model. Same-protocol Responses output should preserve these fields.

### Acceptance criteria

- [ ] A red test proves a top-level unknown field is dropped before the change.
- [ ] The same test passes after implementation.
- [ ] Unknown top-level fields are not added to CrossProtocolCanonical.
- [ ] The implementation has a single clear module/seam for top-level request raw fallback.
- [ ] Slice report lists tests run and changed files.

### Skill loop

Use `/tdd`. If field preservation fails in a non-obvious place, switch to `/diagnosing-bugs` and build a minimal feedback loop using the failing test.

---

## Slice 3 — Merge known fields over raw fallback

**Blocked by:** Slice 2.

**User stories covered:** 12, 13, 14, 20, 21.

### What to build

Implement the known-plus-raw merge rule for request re-emission: known structured fields win over same-name raw fallback, while unknown raw fields are retained.

### Acceptance criteria

- [ ] A red test proves stale raw `model` cannot override a mapped known model.
- [ ] Unknown top-level raw fields remain present after known fields are merged.
- [ ] Merge behavior is centralized rather than scattered across inbound/outbound code.
- [ ] The slice report explains the merge seam and why it preserves locality.

### Skill loop

Use `/tdd`. Review must check for scattered special cases and reject them.

---

## Slice 4 — Preserve `client_metadata` as a native known field

**Blocked by:** Slice 3.

**User stories covered:** 6, 10, 18, 20, 21.

### What to build

Model `client_metadata` in the OpenAI Responses native request layer and preserve it through same-protocol request round-trip without conflating it with generic metadata.

### Acceptance criteria

- [ ] A red test proves `client_metadata` is not preserved before the change.
- [ ] The test passes after implementation.
- [ ] `client_metadata` is represented in Responses native structures, not in CrossProtocolCanonical.
- [ ] Existing generic `metadata` behavior is not changed.

### Skill loop

Use `/tdd`. If existing metadata behavior is affected, route through `/diagnosing-bugs` before continuing.

---

## Slice 5 — Preserve namespace tools in same-protocol routing

**Blocked by:** Slice 3.

**User stories covered:** 1, 2, 8, 15, 16, 18, 20, 21.

### What to build

Preserve `tools[].type="namespace"` and its child LeafMethod tools in Responses-to-Responses routing. Namespace flattening must remain available only as an explicit cross-protocol LossyDowngrade where needed.

### Acceptance criteria

- [ ] A red test proves same-protocol routing currently flattens or drops namespace tool structure.
- [ ] Same-protocol output keeps `type: "namespace"`.
- [ ] Child tool names remain LeafMethod names, not CompositeName values.
- [ ] Cross-protocol flattening behavior is either unchanged or explicitly diagnosed as lossy.
- [ ] Review verifies no Codex-only special case was introduced.

### Skill loop

Use `/tdd`. If behavior differs by route, build two focused tests: same-protocol native and cross-protocol downgrade.

---

## Slice 6 — Preserve `tool_search` tool declarations

**Blocked by:** Slice 3.

**User stories covered:** 3, 10, 14, 18, 20, 21.

### What to build

Preserve `tools[].type="tool_search"` as an OpenAI Responses native tool declaration. It must not be rebuilt as an ordinary function named `tool_search`.

### Acceptance criteria

- [ ] A red test proves `tool_search` is dropped or converted incorrectly before the change.
- [ ] Same-protocol output keeps `type: "tool_search"`.
- [ ] `execution`, `description`, and `parameters` survive round-trip.
- [ ] Unknown attributes on the tool are preserved through raw fallback.

### Skill loop

Use `/tdd`. If handler payload behavior becomes relevant, record it as out of scope for request phase unless request shape itself is wrong.

---

## Slice 7 — Preserve `defer_loading` on function tools

**Blocked by:** Slice 5 and Slice 6.

**User stories covered:** 4, 13, 20, 21.

### What to build

Preserve `defer_loading` on ordinary function tools and namespace child function tools.

### Acceptance criteria

- [ ] A red test proves `defer_loading` is lost before the change.
- [ ] Same-protocol output preserves `defer_loading: true` on ordinary function tools.
- [ ] Same-protocol output preserves `defer_loading: true` on namespace child tools.
- [ ] The field is not represented as a generic canonical function property unless a clean cross-protocol reason exists.

### Skill loop

Use `/tdd`. Review must check that the field lives in the native tool representation.

---

## Slice 8 — Preserve `additional_tools` input items

**Blocked by:** Slice 5 and Slice 6.

**User stories covered:** 5, 10, 13, 18, 20, 21.

### What to build

Preserve request-side input items of type `additional_tools`, including nested tool declarations.

### Acceptance criteria

- [ ] A red test proves `additional_tools` is dropped or converted to an ordinary message before the change.
- [ ] Same-protocol output keeps the `additional_tools` item type.
- [ ] Nested namespace and tool-search tool declarations survive.
- [ ] Unknown attributes on the input item are preserved through raw fallback.

### Skill loop

Use `/tdd`. If existing message conversion makes this hard to express cleanly, document the architecture gap instead of adding string hacks.

---

## Slice 9 — Preserve unknown tool and input item variants

**Blocked by:** Slice 2, Slice 5, Slice 8.

**User stories covered:** 11, 13, 14, 20, 21.

### What to build

Add raw fallback for unknown `tools[]` variants and unknown request-side `input[]` item variants.

### Acceptance criteria

- [ ] A red test proves an unknown tool type is lost before the change.
- [ ] A red test proves an unknown input item type is lost before the change.
- [ ] Same-protocol output preserves complete unknown tool JSON.
- [ ] Same-protocol output preserves complete unknown input item JSON.
- [ ] Known tool and known input item behavior remains structure-driven.

### Skill loop

Use `/tdd`. If test setup becomes broad, split tool fallback and input fallback into separate sub-slices.

---

## Slice 10 — Preserve complex `tool_choice` forms

**Blocked by:** Slice 2 and Slice 3.

**User stories covered:** 10, 11, 13, 14, 20, 21.

### What to build

Preserve OpenAI Responses `tool_choice` request forms, including complex or future forms that the current canonical abstraction cannot represent.

### Acceptance criteria

- [ ] A red test proves a complex `tool_choice` form is not round-tripped before the change.
- [ ] Same-protocol output preserves the original tool-choice structure unless intentionally modified by a known field.
- [ ] Simple existing `tool_choice` behavior continues to work.
- [ ] Unknown tool-choice forms use raw fallback rather than CrossProtocolCanonical expansion.

### Skill loop

Use `/tdd`. Review must reject scattered handling of individual tool-choice shapes outside the native request seam.

---

## Slice 11 — Add request preservation diagnostics

**Blocked by:** Slices 2 through 10.

**User stories covered:** 10, 14, 15, 16, 20, 21, 22, 23.

### What to build

Add centralized diagnostics for request native preservation and lossy downgrade decisions.

### Acceptance criteria

- [ ] Diagnostics identify whether native preservation was used.
- [ ] Diagnostics identify whether LossyDowngrade occurred.
- [ ] Diagnostics include enough information to see namespace, tool-search, unknown top-level, unknown tool, and unknown input item counts.
- [ ] Diagnostics are generated from one clear seam rather than scattered logs.
- [ ] Same-protocol native routing does not report namespace flattening as normal behavior.

### Skill loop

Use `/tdd` where diagnostics are observable. If no clean test seam exists for a diagnostic field, document the seam gap and review it before implementing lower-level tests.

---

## Slice 12 — First-phase milestone review and report

**Blocked by:** Slice 11.

**User stories covered:** 17, 18, 19, 20, 21, 22, 23, 24, 25.

### What to build

Run the first-phase milestone review for request native round-trip.

### Acceptance criteria

- [ ] Targeted tests for all completed slices pass.
- [ ] The milestone review checks `CONTEXT.md`, ADR 0001, the PRD, and the field classification document.
- [ ] Architecture is reviewed using Module, Interface, Depth, Seam, Adapter, Leverage, and Locality vocabulary.
- [ ] Any failed review finding is routed back to the smallest relevant slice.
- [ ] A milestone report is written.
- [ ] If the milestone passes, prepare a handoff document when requested.

### Skill loop

Use `/improve-codebase-architecture` if the implementation exposed architectural friction. Use `/diagnosing-bugs` if any symptom lacks a deterministic red/green feedback loop. Use `/handoff` after the milestone report when the user wants to compact context.
