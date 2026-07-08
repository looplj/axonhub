# Implementation plan

Status: planning only. Do not run `task.py start`, do not edit business code, and do not run tests/lint/build until the user approves this plan.

## Phase 0: Planning gate

- [x] Create Trellis task.
- [x] Clone upstream latest into `/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable`.
- [x] Record git comparison evidence.
- [x] Index current and upstream codebases through MCP/CLI evidence path.
- [x] Capture native struct field comparison.
- [x] Capture key function/caller evidence.
- [x] Extract official OpenAI and Anthropic protocol field inventories.
- [x] Classify field routing, drop policy, and trigger conditions.
- [x] Run architecture review and decide to preserve the author framework.
- [x] Rewrite `prd.md`, `design.md`, and `implement.md` into final planning shape.
- [x] First implementation scope frozen: OpenAI Responses -> OpenAI Responses same-protocol native preservation.
- [ ] User reviews and explicitly approves implementation start.

## Parent task execution loop

This task is a parent task. Implementation must proceed through vertical slices, and each slice must complete all gates before the next slice starts.

```text
Parent Task
  -> vertical slice
  -> TDD
  -> Trellis check
  -> Matt code-review
  -> pass?
      no  -> return to TDD / diagnosing-bugs / planning for the same slice
      yes -> next slice
  -> final parent review after all slices
  -> overall architecture review
  -> update-spec
  -> ADR / CONTEXT updates if needed
  -> finish-work
```

Per-slice gates:

- [ ] Red: add one targeted failing preserve-or-diagnose test for the selected field group.
- [ ] Green: implement the smallest change that satisfies the targeted test.
- [ ] Targeted validation: run only the relevant test/package command for touched code.
- [ ] Trellis check: verify spec compliance, field ownership, no scope creep, and touched-package quality. In current inline mode this check is performed by the main session; in sub-agent mode it may be performed by a check agent.
- [ ] Matt code-review: review the diff for bugs, code quality, architecture drift, dead code, unused code, over-broad abstraction, and violations of `CONTEXT.md`, ADR-0002, and `.trellis/spec/backend/protocol-transformer-guidelines.md`.
- [ ] Gate decision:
  - if behavior is missing, return to TDD for the same slice;
  - if behavior is broken or unclear, run diagnosing-bugs for the same slice;
  - if the slice boundary is wrong, return to planning for the same slice;
  - only if all checks pass may the next slice start.

Parent-level gates after all OpenAI Responses preservation sub-slices pass:

- [ ] Final parent review for cross-slice consistency.
- [ ] Overall architecture review for module depth, seam clarity, locality, field ownership, and useless code.
- [ ] Small architecture cleanup if the architecture review fails.
- [ ] update-spec for durable transformer rules learned during implementation.
- [ ] ADR / CONTEXT updates only if new decisions or terms meet the documented criteria.
- [ ] finish-work after validation and review pass.

## Phase 1: Baseline sync slice

Goal: make future changes apply on top of the author's latest framework, not the stale local base.

Steps:

- [ ] Record current dirty tree before modifying code.
- [ ] Merge/rebase `origin/unstable@97c9351a23df5a3c302cf1c35bf5ca39caf7208f` into current branch, or create a clean implementation branch from upstream latest.
- [ ] Preserve upstream Codex header commit behavior from `e412fab1 feat: add codex headers (#1963)`.
- [ ] Resolve unrelated platform conflicts in favor of upstream.
- [ ] Re-apply only audited transformer/documentation patches.
- [ ] Do not include unrelated docs/vendor snapshots unless needed for this task.

Verification:

- [ ] Git diff shows only expected baseline sync and audited transformer/planning changes.
- [ ] Codex header behavior remains present in the final baseline.

Rollback:

- Reset to pre-sync branch/commit or restore recorded dirty tree.

## Phase 2: OpenAI Responses native preservation slice

Goal: same-protocol OpenAI Responses requests preserve missing top-level native/profile fields without bloating `llm.Request`. This phase is corrected by the upstream reread: upstream already preserves some raw tool/input Codex lazy-loading shapes, so do not reimplement those.

Dependencies:

- Phase 1 baseline sync complete.
- Use `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/upstream-architecture-reread-2026-07-06.zh.md` as the authoritative architecture constraint.

Corrected field groups:

- already-preserved raw tool/input fields: `tool_search`, `tool_search_call`, `namespace`, raw `tool_choice`, and unknown tool/input item variants inside `tools`, `tool_choice`, or `input`;
- missing top-level official/profile fields: `prompt`, `conversation`, `context_management`, `additional_tools`, `defer_loading`;
- unknown same-protocol top-level fields.

Sub-slices:

### P1a: top-level raw fallback for fields not represented by current `responses.Request`

Fields: `context_management`, `additional_tools`, `defer_loading`, and unknown future top-level Responses request fields.

- [ ] Red: add same-protocol OpenAI Responses round-trip tests proving these top-level fields survive inbound -> outbound.
- [ ] Red: add a guard test proving raw `model` does not overwrite a mapped outbound model.
- [ ] Green: extend `ProviderExtensions.OpenAIResponses.Request` and `request_extensions.go` with same-protocol top-level raw fallback.
- [ ] Review: confirm raw fallback does not leak to Chat, Anthropic, or unrelated providers.

### P1b: official typed TODO fields

Fields: `prompt`, `conversation`.

- [ ] Red: add same-protocol tests for `prompt` and `conversation` based on the existing upstream `Prompt` / `Conversation` structs.
- [ ] Green: enable typed fields in `responses.Request` and ensure outbound emits them when present.
- [ ] Review: confirm typed fields and raw top-level fallback do not duplicate or override each other.

### P1c: preserve existing raw tool/input behavior while adding top-level fallback

Fields already handled upstream: `tool_search`, `tool_search_call`, `namespace`, raw `tool_choice`, raw-only input/tool variants.

- [ ] Red/Regression: keep existing upstream tests for raw tools/input/tool_choice passing.
- [ ] Green: avoid changing existing replay semantics unless a regression test proves it is required.
- [ ] Review: confirm no duplicate Codex/MCP tool conversion layer was introduced.

Shared Phase 2 constraints:

- [ ] Define a clear Responses-native preservation owner inside `llm/transformer/openai/responses`.
- [ ] Move same-protocol preservation decisions into that owner rather than spreading them across ad hoc metadata keys.
- [ ] Keep `llm.Request` additions out unless the field is proven cross-protocol common.
- [ ] Keep raw fallback same-protocol-only.

Verification:

- [ ] Targeted Responses tests prove preserve-or-diagnose behavior.
- [ ] No new broad `llm.Request` field is added for Responses-only data.
- [ ] Unknown/native data is not silently replayed across protocol families.

Rollback:

- Revert Phase 2 files independently from later Chat/Anthropic/stream changes.

Architecture review gate after Phase 2:

- [ ] Review the final Responses preservation module for module depth, seam clarity, locality, field ownership, and unnecessary code.
- [ ] If architecture review fails, perform a small architecture repair before starting Phase 3.

## Phase 3: OpenAI Chat emission policy slice

Status: explicitly deferred until the frozen OpenAI Responses same-protocol preservation slice is complete or the user changes scope.

Goal: preserve official Chat-native fields without accidentally emitting unsupported fields to every OpenAI-compatible provider.

Dependencies:

- Phase 2 complete or explicitly deferred by user.
- Shared builder call blast radius reviewed using `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/requestfromllm-callers.txt`.

Initial field groups:

- `web_search_options`;
- `prediction`;
- top-level `audio`;
- modern custom tool representation if verified against official source;
- deprecated `function_call` / `functions` only as documented compatibility/defer decision.

Steps:

- [ ] Add targeted same-protocol Chat tests for verified missing fields.
- [ ] Add or preserve fields in native Chat model.
- [ ] Introduce or clarify provider emission policy so shared `RequestFromLLM` does not become a provider-specific dump.
- [ ] Gate provider-specific emission by same-protocol or adapter capability.

Verification:

- [ ] OpenAI Chat same-protocol path preserves supported official fields.
- [ ] OpenAI-compatible provider paths do not receive unsupported fields accidentally.
- [ ] Unsupported fields produce explicit diagnostic or documented omission.

Rollback:

- Revert Chat model/policy files without touching Responses preservation.

## Phase 4: Anthropic native preservation slice

Status: explicitly deferred until the frozen OpenAI Responses same-protocol preservation slice is complete or the user changes scope.

Goal: preserve Anthropic native/companion fields in Anthropic-native paths without conflating them with OpenAI MCP or Codex local MCP.

Dependencies:

- Anthropic official/companion source evidence is confirmed in field inventory.

Initial field groups:

- `container`;
- `inference_geo`;
- verified MCP connector companion fields such as `mcp_servers` / `mcp_toolset` if present in current source evidence;
- current local `ContextManagement` patch only if it remains verified.

Steps:

- [ ] Add targeted same-protocol Anthropic tests for verified fields.
- [ ] Keep fields in Anthropic native structs/adapters.
- [ ] Do not map Anthropic MCP connector fields to OpenAI Responses MCP unless a real equivalent is documented and tested.

Verification:

- [ ] Anthropic same-protocol path preserves verified fields.
- [ ] Cross-protocol incompatible fields are diagnosed or deliberately unsupported.

Rollback:

- Revert Anthropic files independently.

## Phase 5: LossyDowngrade diagnostics slice

Goal: make cross-protocol loss visible and testable.

Dependencies:

- At least one same-protocol preservation slice complete, so diagnostics are not masking accidental same-protocol loss.

Steps:

- [ ] Build a small downgrade matrix from field inventory and native tests.
- [ ] Add diagnostics for fields that cannot map across protocol families.
- [ ] Record source protocol, source field, target protocol, reason, and severity.
- [ ] Keep diagnostics separate from native storage.

Verification:

- [ ] Tests assert diagnostics for non-mappable fields.
- [ ] No fake semantic mapping is introduced for incompatible fields like OpenAI Responses MCP vs Anthropic MCP connector.

Rollback:

- Revert diagnostic module without reverting native field preservation.

## Phase 6: Stream event fidelity slice

Status: explicitly deferred until request/response native preservation work is stable or the user changes scope.

Goal: treat stream event preservation as its own module, not as request-field repair.

Dependencies:

- Request/response preservation priorities are stable.
- Stream event fixture set is chosen from official inventory.

Initial focus:

- OpenAI Responses stream event types, especially MCP, reasoning, audio, output item, and custom tool events.
- OpenAI Chat stream chunks.
- Anthropic Messages stream events and delta types.

Steps:

- [ ] Pick one protocol stream path first; do not fix all stream paths at once.
- [ ] Add event fixture tests through the external transformer stream seam.
- [ ] Deepen internal parser/accumulator/emitter logic while keeping external seam stable.

Verification:

- [ ] Selected stream events are preserved or explicitly diagnosed.
- [ ] Request models are not used as stream event storage.

Rollback:

- Revert stream module changes independently.

## Targeted validation commands after implementation starts

Do not run these during planning. After user approval and code changes, prefer targeted tests for touched packages.

Candidate commands:

```bash
cd /Users/asuan/项目/AI/axonhub/llm && go test ./transformer/openai/responses/...
cd /Users/asuan/项目/AI/axonhub/llm && go test ./transformer/openai/...
cd /Users/asuan/项目/AI/axonhub/llm && go test ./transformer/anthropic/...
cd /Users/asuan/项目/AI/axonhub/llm && go test ./transformer/shared/...
```

Use only the commands relevant to the touched slice unless the user authorizes broader validation.

## Final pre-start checklist

Before running `task.py start`:

- [ ] User has reviewed final `prd.md` / `design.md` / `implement.md`.
- [ ] User explicitly approves implementation start.
- [ ] Current dirty tree is recorded.
- [ ] First implementation slice is selected: Phase 1 baseline sync, then Phase 2 OpenAI Responses native preservation.

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

