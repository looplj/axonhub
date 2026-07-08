# PRD: Upstream protocol transformer audit and clean repair plan

## Goal

Preserve AxonHub's author's protocol transformer framework while repairing verified protocol field loss. The repair must start with OpenAI Responses same-protocol native preservation, then expand to Chat, Anthropic, cross-protocol diagnostics, and stream fidelity only through independently verifiable slices.

## Background

The current branch contains prior OpenAI Responses / Chat / Claude protocol conversion work. Some changes were made before the latest upstream code and canonical protocol sources were rechecked, so the branch may contain both real fixes and architecture pollution. The next code changes must start from upstream latest and the newly extracted official protocol field inventory, not from older generated docs alone.

The architecture review concluded that the author's broad design is sound:

```text
Client HTTP protocol
  -> inbound transformer
  -> llm.Request / llm.Response common view
  -> outbound transformer
  -> provider HTTP protocol
```

The problem is not that this framework is wrong. The problem is that several preservation paths are currently too shallow: native fields, raw fallback, `TransformerMetadata`, and `ProviderExtensions` can become a mixed bucket. The repair should deepen those modules, not replace the framework.

## Confirmed facts

- Repository: `/Users/asuan/项目/AI/axonhub`.
- Trellis task: `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit`.
- Current branch at audit start: `codex-transformer-field-fixes`.
- Current HEAD at audit start: `c798c6e9e206e4d6ca44cae49c3d586e4e23c962`.
- Upstream remote: `https://github.com/looplj/axonhub.git`.
- Upstream branch: `unstable`.
- Upstream latest after fetch: `97c9351a23df5a3c302cf1c35bf5ca39caf7208f`.
- Isolated upstream clone: `/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable`.
- Merge base: `6831e03ce7cf1efbc3eb4d2e2eb84bf0cb1722a3`.
- Upstream has 7 commits not in the current branch, including `e412fab1 feat: add codex headers (#1963)`.
- Upstream Codex header behavior must be preserved.
- Current working tree already had uncommitted transformer/doc/Trellis changes; implementation must not overwrite them blindly.
- Codebase-memory MCP server transport failed in the latest architecture-review turn, so codebase-memory CLI was used for read-only graph evidence.
- Architecture report generated at `/var/folders/31/rjj7j9md2t5cb_70bjh10vg40000gn/T/architecture-review-20260706-191914.html`.
- User confirmed on 2026-07-06 that first implementation scope is frozen to OpenAI Responses -> OpenAI Responses same-protocol native preservation.

## Evidence artifacts

- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/git-comparison.md` — git baseline, local/upstream commits, diff names/stat, working tree state.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/native-struct-field-comparison.md` — extracted native request struct field comparisons.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/requestfromllm-callers.txt` — caller evidence for shared OpenAI-compatible Chat builder.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/upstream-transformer-flow-source-notes.txt` — upstream transformer/orchestrator source notes.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/architecture-audit.md` — architecture findings and recommended stance.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/upstream-transformer-complexity.json` — complexity evidence for stream and transformer hotspots.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/protocol-field-extraction/openai-openapi.github.yaml` — parseable official OpenAI OpenAPI YAML from OpenAI GitHub.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/protocol-field-extraction/openai-fields.md` and `.json` — extracted OpenAI request/response/stream/nested schema fields.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/protocol-field-extraction/anthropic-fields.md` and `.json` — extracted Anthropic request/response/stream/MCP companion fields.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/protocol-field-extraction/complete-protocol-field-inventory.md` — combined field inventory against upstream/current code.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/protocol-field-extraction/field-routing-classification.zh.md` — Chinese meaning and handling route for each extracted top-level field and stream/event field.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/protocol-field-extraction/author-drop-policy.zh.md` — author's current drop/weak-preservation patterns and rules for what may be dropped.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/protocol-field-extraction/field-trigger-conditions.zh.md` — fields that normally appear only when clients opt in or server-side tool/model features emit them, with Codex local MCP separated from server-side MCP connectors.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/design.md` — final clean repair design.
- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/implement.md` — approval-gated implementation sequence.

## Requirements

### R1. Preserve the author framework

The repair must keep the current transformer pipeline shape:

- inbound/outbound transformer adapters stay as the protocol seams;
- `llm.Request` / `llm.Response` remain cross-protocol common views;
- protocol native structs carry protocol-native fields;
- provider/native sidecars carry fields that do not belong in common;
- raw fallback is same-protocol preservation, not cross-protocol magic.

### R2. Do not turn `llm.Request` into a universal protocol AST

`llm.Request` must only hold stable cross-protocol semantics. It must not accumulate complete OpenAI Responses, Chat, Anthropic, Codex, provider-private, or stream-event field sets.

### R3. Start with OpenAI Responses same-protocol native preservation

The first implementation target must be OpenAI Responses -> OpenAI Responses round-trip, because it directly addresses Codex Responses / MCP / lazy-loading preservation and avoids cross-protocol ambiguity.

### R4. Keep field ownership explicit

Each field must have one primary owner:

| Field kind | Owner |
|---|---|
| Stable cross-protocol semantics | `llm.Request` / `llm.Response` |
| OpenAI Responses official fields | `llm/transformer/openai/responses` native model/preservation module |
| Codex Responses usage-profile fields | OpenAI Responses native preservation module, not a separate private protocol |
| OpenAI Chat official fields | OpenAI Chat native model and Chat emission policy |
| Anthropic official/companion fields | Anthropic native model / Anthropic adapter |
| Provider-specific controls | Provider extension or provider adapter |
| Same-protocol unknown fields | Raw fallback within same protocol only |
| Cross-protocol incompatible fields | Explicit `LossyDowngrade` diagnostic |
| Stream events | Stream event fidelity module, not request fields |

### R5. Same-protocol fidelity before cross-protocol downgrade

A field must first be proven to survive source-protocol -> same-protocol output where applicable. Only after same-protocol behavior is correct should cross-protocol behavior be defined as direct mapping, opaque same-protocol-only preservation, explicit lossy diagnostic, or deliberate unsupported/drop.

### R6. Loss must be visible

When a field cannot be represented in the target protocol, the behavior must be explicit and test-covered. Silent drop, accidental flattening, and hidden metadata loss are not acceptable.

### R7. Implementation must be sliced and reviewable

Each implementation slice must be independently verifiable with targeted tests. Do not bundle Responses, Chat, Anthropic, stream, and diagnostics into one large change.

### R8. No implementation before planning approval

No business code should be modified, and `task.py start` should not be run, until the user reviews these final planning artifacts and approves implementation.

### R9. Parent task vertical slice loop

The parent task must be split into independently verifiable vertical slices. Each slice must run TDD, Trellis check, and Matt code-review before the next slice starts. If a gate fails, return to TDD, diagnosing-bugs, or planning for the same slice. After all slices pass, run final parent review, overall architecture review, update-spec, optional ADR/CONTEXT updates, and finish-work.

## Key findings

1. Author architecture is a two-stage bridge: client protocol -> inbound transformer -> `llm.Request` common view -> outbound transformer -> provider protocol, with equivalent response/stream paths.
2. `llm.Request` is intentionally chat-centric. It is not a full native model for Responses, Chat, and Anthropic.
3. Upstream `openai.RequestFromLLM` is a shared OpenAI-compatible Chat request builder used by OpenAI, OpenRouter, Doubao, DeepSeek, Moonshot, Zai, Copilot, and Gemini OpenAI-compatible paths. Any change there has high blast radius.
4. Upstream Chat builder filters tools to function tools only. This is a verified protocol drift candidate for modern OpenAI Chat custom tools / newer fields, but it should not be fixed by blindly widening every provider's emitted request.
5. Upstream Responses sidecar already preserves some raw-only tools, tool choice, and input items. The repair should extend and clarify this preservation pattern, not replace the whole framework.
6. Upstream Responses request struct has request-side `conversation` commented out and lacks `context_management`; these are verified same-protocol preservation gaps if clients send those fields.
7. Current branch expands Responses provider extensions from 4 to 9 fields. Some additions are likely useful, but the sidecar boundary needs tightening so it does not become a patch bucket.
8. Upstream Anthropic request struct lacks newer/companion native fields such as `container` and `inference_geo`; current branch only adds `ContextManagement`.
9. Stream handling is a separate high-complexity area. Request-field fixes do not solve Responses stream event fidelity.
10. Codex local MCP/lazy-loading fields, OpenAI Responses server-side MCP fields, and Anthropic MCP connector fields are three different categories and must not be conflated.

## Prioritized repair scope

### P0: Baseline and document guardrails

- Rebase/merge onto upstream latest or create a clean branch from upstream latest.
- Preserve upstream Codex header commit behavior.
- Keep unrelated local changes out of the first repair.
- Treat the field-routing documents as implementation constraints.

### P1: OpenAI Responses native preservation module — first frozen implementation scope

Deepen `llm/transformer/openai/responses` so OpenAI Responses same-protocol requests preserve the fields that upstream still drops:

- missing official/profile top-level fields such as `prompt`, `conversation`, `context_management`, `additional_tools`, and `defer_loading`;
- unknown same-protocol top-level fields where safe to re-emit;
- existing upstream raw tool/input preservation for `tool_search`, `tool_search_call`, `namespace`, raw `tool_choice`, and unknown tool/input variants must remain intact but should not be reimplemented.

### P2: OpenAI Chat emission policy

After P1, add a scoped policy around OpenAI-compatible Chat emission so official Chat fields like `web_search_options`, `prediction`, and top-level `audio` do not pollute every provider adapter accidentally.

### P3: Anthropic native preservation

After P1/P2, preserve Anthropic native/companion fields such as `container`, `inference_geo`, and verified MCP connector fields in Anthropic-native paths.

### P4: LossyDowngrade diagnostics

Centralize cross-protocol loss reporting. This is diagnostic-only; it must not become a second native-field storage path.

### P5: Stream event fidelity

Handle Responses/Chat/Anthropic stream event preservation as its own module/slices. Stream events must not be modeled as request fields.

## Acceptance criteria

- [x] Trellis task exists.
- [x] Isolated upstream clone path and commit are recorded.
- [x] Git diff evidence exists.
- [x] MCP/CLI indexing was run for current and upstream codebases.
- [x] Architecture evidence exists for the transformer flow and key functions.
- [x] Complete field inventory evidence exists for OpenAI Responses, OpenAI Chat, and Anthropic Messages.
- [x] Field routing classification exists and separates native/common/provider/raw/diagnostic/drop paths.
- [x] `design.md` explains the clean repair architecture and the module ownership model.
- [x] `implement.md` splits implementation into approval-gated slices.
- [x] First implementation scope is frozen to OpenAI Responses same-protocol native preservation.
- [ ] User reviews and approves the plan before implementation starts.
- [ ] After implementation starts, each slice has targeted tests proving preserve-or-diagnose behavior.
- [ ] Each slice completes TDD -> Trellis check -> Matt code-review before the next slice starts.
- [ ] Failed slice review returns to TDD / diagnosing-bugs / planning for the same slice.
- [ ] After all OpenAI Responses preservation slices pass, final parent review and overall architecture review pass before moving to Chat/Anthropic/stream work.
- [ ] update-spec is run after the parent review; ADR/CONTEXT are updated only for durable decisions or terms.

## Out of scope before approval

- Running lint/build/test.
- Restarting any server.
- Applying code fixes.
- Rewriting the transformer architecture from scratch.
- Treating old copied protocol docs as source of truth without tests.
- Creating fake semantic mappings between incompatible tool ecosystems.

## Open questions

None blocking the planning artifact. The next decision is user approval to start implementation, beginning with baseline sync and the first OpenAI Responses native preservation slice.

## 2026-07-06 upstream reread correction

Authoritative upstream baseline is now:

```text
/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable
HEAD: 97c9351a
```

See:

```text
.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/upstream-architecture-reread-2026-07-06.zh.md
```

Correction to earlier plan:

- `tool_search`, `tool_search_call`, `namespace`, raw `tool_choice` inside tools/input/tool_choice are already partially preserved in upstream through `ProviderExtensions.OpenAIResponses.Request` and `request_extensions.go`.
- The real first missing slice is top-level Responses preservation: `prompt`, `conversation`, `context_management`, `additional_tools`, `defer_loading`, and future unknown top-level fields.
- Do not reimplement existing raw tool/input replay.
- Keep changes inside the existing author seam: `ProviderExtensions.OpenAIResponses.Request` plus `responses/request_extensions.go` marshal merge.

## Final execution contract — clean upstream implementation

This task must be implemented from the author's upstream clean baseline, not from the current polluted branch.

Authoritative code baseline:

```text
origin/unstable
/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable
```

Current branch `codex-transformer-field-fixes` is research/reference only. Its business-code changes must not be used as the implementation base because they were influenced by older OpenRouter-style conversion assumptions and broad cross-protocol edits.

### Final acceptance standard

The final implementation must:

- preserve or explicitly diagnose all OpenAI Responses, OpenAI Chat, and Anthropic field ownership decisions recorded in the verified field matrix;
- keep protocol-specific fields out of `llm.Request` unless they are proven stable cross-protocol semantics;
- stop expanding `TransformerMetadata` as a magic-key protocol field bus;
- keep same-protocol native/raw preservation separate from cross-protocol lossy downgrade;
- preserve the author's main transformer architecture unless an architecture review proves a deeper module is needed;
- be readable, testable, and maintainable, with no dead code, duplicate conversion layers, or hidden field ownership;
- prefer the smallest change inside the author's framework unless additional code clearly improves locality, leverage, testability, and future maintainability.

### Required slice workflow

For each small vertical slice:

1. Write focused failing tests first.
2. Implement the smallest architecture-aligned change.
3. Run a self-review checklist against the field matrix and architecture report.
4. If self-review fails, return to TDD/debug before moving on.

For each module after several small slices are complete:

1. Trigger code-review.
2. Trigger improve-codebase-architecture.
3. Trigger multi-agent cross-review for bugs, code quality, architecture drift, dead code, and spec compliance.
4. If any review fails, read the review report, fix the issues, and return to the relevant TDD/debug slice.
5. Only after all reviews pass, git-archive/commit the module before entering the next module.

### Module order

1. OpenAI Responses request preservation.
2. OpenAI Responses response/stream fidelity.
3. OpenAI Chat native emission policy.
4. Anthropic native preservation and MCP connector fields.
5. Cross-protocol lossy downgrade diagnostics.
6. FieldOwnershipTable/spec hardening and final architecture review.

### First module slices

P1 OpenAI Responses request preservation:

- P1a: top-level raw fallback for `context_management`, future unknown top-level fields, and verified profile top-level fields such as `additional_tools` / `defer_loading` if present.
- P1b: typed TODO fields `prompt` and `conversation`.
- P1c: regression protection for upstream's existing raw `tools` / `tool_choice` / `input` replay (`tool_search`, `tool_search_call`, `namespace`, raw tool choice, unknown tool/input variants).

P1 must not modify Chat, Anthropic, stream fidelity, shared lossy downgrade, or unrelated provider adapters.

