# Encrypted Reasoning Replay Quality Loop

## Goal and Boundary

Fix OpenAI Responses `encrypted_content` conversation replay with TDD. Preserve a valid
reasoning item's identity on normal same-protocol continuation. If the provider rejects
persisted encrypted reasoning/compaction state, retry at most once with only that opaque
state removed while preserving visible conversation and tool lifecycle data.

Out of scope: deployment, service restart, database changes, ports 8090/8091, channel data,
and speculative channel/account fingerprinting.

## Acceptance Evidence

1. A public Responses transform round trip proves `reasoning.id` remains paired with its
   `encrypted_content`, including compact responses where applicable.
2. A public orchestrator pipeline test proves the first provider request contains replayed
   encrypted reasoning, a recognized provider rejection triggers one same-channel retry,
   and the second serialized provider request contains no reasoning signature, reasoning
   item identity, compaction, or compaction summary.
3. The recovery retains visible messages, reasoning summaries, tool calls, and tool results.
4. Non-matching provider errors do not trigger this recovery; the recovery cannot loop.
5. Raw same-protocol pass-through cannot restore fields intentionally removed for recovery.

## Stop Condition

| Gate | Status | Evidence |
|---|---|---|
| All slices pass self-review | passed | S1–S7 self-reviewed |
| Module review findings closed | passed | Independent Behaviour/Architecture/Spec reviewers; Spec finding (compact ResponseReasoningItemID as response provenance) fixed and retested |
| Parent review passes | passed | Goal coverage, non-goals, composition, findings closed, checks green; risks stated below |
| Required checks pass or are explicitly skipped | passed | `go test` responses+pipeline+orchestrator green; lint/build skipped per AGENTS |
| Durable knowledge updated or marked unnecessary | passed | Ledger + Trellis task prd/implement updated; no ADR needed |
| Remaining risks stated | passed | See Risks section |

## Slice Ledger

| Group | Slice | Outcome | Seam | Verification | Write set | Status | Review | Notes |
|---|---|---|---|---|---|---|---|---|
| G1 Transformer | S1 | Preserve reasoning identity with encrypted content | Responses provider response -> canonical/native sidecar -> client response -> continuation request -> provider request | `cd llm && go test ./transformer/openai/responses -run '^(TestResponsesEncryptedReasoningIdentitySurvivesConversationReplay|TestResponsesMultipleEncryptedReasoningItemsSurviveClientResponseReplay|TestG15c_.*Reasoning.*)$' -count=1`; package-wide `go test ./transformer/openai/responses -count=1` | Responses response sidecar, request identity carrier, transformers/tests | passed | passed | Response `output[]` reasoning identity/ciphertext is exact raw sidecar state; request `input[]` identity remains on `ResponseReasoningItemID`. Single, multiple, arbitrary opaque ciphertext and full continuation replay pass. |
| G1 Transformer | S2 | Preserve compact response item identity | Compact provider response -> canonical compact response -> client compact response | `cd llm && go test ./transformer/openai/responses -run '^(TestCompactResponseEncryptedReasoningAndCompactionIdentitySurviveRoundTrip|TestOutboundTransformer_TransformCompactResponse|TestCompactInboundTransformer_TransformResponse)$' -count=1` | Responses round-trip test only; existing compact conversion reused | passed | passed | Exact reasoning `id + opaque encrypted_content`, message id, and `compaction_summary id + encrypted_content` survive. No production change was needed beyond S1's shared response builder behavior. |
| G1 Transformer | S2b | Verify streamed reasoning identity can become a valid continuation | Responses SSE -> canonical chunks -> client completed response -> continuation request -> provider request | `cd llm && go test ./transformer/openai/responses -run '^TestStreamedEncryptedReasoningIdentitySurvivesConversationReplay$' -count=1` plus package-wide `go test ./transformer/openai/responses -count=1` | Responses round-trip test only; existing stream metadata path reused | passed | passed | Final `output_item.done` id/ciphertext wins over provisional value and survives the next serialized provider request. Initial fixture omitted text delta and failed for an unrelated missing message; corrected to a protocol-complete SSE fixture, then passed. |
| G2 Recovery | S3 | One-shot provider-directed encrypted-state recovery | pipeline `Process` recovery interface plus orchestrator request/retry pipeline | `cd llm && go test ./pipeline -run '^TestPipeline_Process_ErrorRecovery' -count=1`; `go test ./internal/server/orchestrator -run '^(TestChatCompletionOrchestrator_Process_RecoversOnceFromInvalidEncryptedReasoning|TestChatCompletionOrchestrator_Process_RecoversEncryptedReasoningWhenRetryPolicyDisabled|TestChatCompletionOrchestrator_Process_DoesNotLoopEncryptedReasoningRecovery|TestPersistentOutboundTransformer_CanRecoverRequiresUpstreamProviderError)$' -count=1` | pipeline recovery seam; orchestrator state/outbound/pass-through/tests | passed | passed | Recovery precedes ordinary policy, consumes no ordinary budget, requires an upstream-provider provenance marker, and is guarded to one use per request. Exact `item_id did not match` text is covered without relying on an error code. |
| G2 Recovery | S4 | Preserve visible/tool history and prevent loops/raw restoration | same orchestrator public seam | `go test ./internal/server/orchestrator -run '^(TestChatCompletionOrchestrator_Process_RecoversOnceFromInvalidEncryptedReasoning|TestChatCompletionOrchestrator_Process_DropsOpaqueReasoningBeforeCrossChannelRetry|TestChatCompletionOrchestrator_Process_DoesNotRecoverForUnrelatedBadRequest|TestChatCompletionOrchestrator_Process_DoesNotRecoverSummaryOnlyReasoning)$' -count=1` | opaque-state cleaner, pass-through guard, orchestrator tests | passed | passed | Second wire request retains summary-only reasoning plus function call/result identity, removes reasoning id/ciphertext and both compaction variants, and cannot be replaced by the original pass-through body. Empty Responses presence marker is not treated as opaque. |
| G3 Provenance | S5 | Never expose Anthropic/Gemini signatures as Responses `encrypted_content`; never pair ciphertext with synthetic `rs_*` | provider response -> canonical response/stream -> Responses client wire response | `cd llm && go test ./transformer/openai/responses -count=1` | `buildReasoningItem` provenance gate; stream metadata gate; golden fixture; compact structured path forces preserve=false | passed | passed | Ciphertext only with Responses-native response provenance (RawOutputItems / stream item metadata). Request ResponseReasoningItemID is never response provenance. |
| G3 Retry boundary | S6 | Drop opaque state before a same-channel retry that switches `ActualModel`, but retain it for a retry of the same model | `PersistentOutboundTransformer.PrepareForRetry` | `go test ./internal/server/orchestrator -run 'PrepareForRetry_DropsOpaque|PrepareForRetry_KeepsOpaque' -count=1` | `outbound.go` PrepareForRetry | passed | passed (self) | Compare ActualModel before/after index advance; drop only on model string change. Same-model retry keeps opaque state. |
| G4 Coverage | S7 | Cover compact input opaque recovery without speculative summary/tool deletion | `PrepareForRecovery` + `Compact.Input` cleaner | `go test ./internal/server/orchestrator -run 'PrepareForRecovery_StripsCompact' -count=1`; chat recovery already covers compaction items in `/v1/responses` body | `encrypted_reasoning.go` already stripped Compact.Input; test added | passed | passed (self) | No separate compact product path required; cleaner already owns Compact.Input. |

## Failed Gates

| Gate | Finding | Evidence | Route | Owner slice | Status |
|---|---|---|---|---|---|
| S1 baseline | Response conversion detached `reasoning.id` from ciphertext and later synthesized a random id | `TestResponsesEncryptedReasoningIdentitySurvivesConversationReplay`: before fix expected `rs_original_from_provider`, got random `rs_*` | TDD | S1 | closed |
| S1 opaque provenance | Responses-native opaque ciphertext was rejected by prefix guessing | Same test with `opaque_provider_ciphertext`: before fix replayed `encrypted_content` was nil; after provenance-based fix test passes | TDD | S1 | closed |
| S3 independent recovery seam | Provider-directed encrypted-state recovery incorrectly depended on ordinary same-channel retry policy | `TestPipeline_Process_ErrorRecoveryIgnoresOrdinaryRetryBudgets`: expected a second attempt with zero ordinary budgets, received the first provider error; dedicated `ErrorRecoverable` seam now passes targeted and package-wide pipeline tests | TDD | S3 | closed |
| S3 summary retention | Opaque-state cleanup removed the Responses reasoning presence marker along with ID/ciphertext | Orchestrator recovery/cross-channel tests were red because the second request contained no `reasoning` item; cleanup now retains a non-nil empty request identity marker and tests pass | TDD | S3 | closed |
| S5 provenance boundary | `buildReasoningItem` and the Responses inbound stream trusted shared `ReasoningSignature` without Responses-native item id | Cross-protocol tests + `TestResponsesClientResponseDoesNotPairEncryptedContentWithSyntheticID`; package-wide responses green after fixture provenance fix | TDD | S5 | closed |
| S6 actual-model switch | `PrepareForRetry` advanced `CurrentModelIndex` without stripping opaque state | `TestPersistentOutboundTransformer_PrepareForRetry_DropsOpaqueStateOnActualModelSwitch` / `...KeepsOpaqueStateOnSameModelRetry` green after drop on ActualModel change | TDD | S6 | closed |

## Review Findings

| Finding | Axis | Evidence | Owner slice | Route | Status |
|---|---|---|---|---|---|
| Response `output[]` reasoning identity was assigned to request-only `Message.ResponseReasoningItemID` | Architecture / Spec | OpenAI Responses baseline; fixed via RawOutputItems | S1 | TDD | fixed |
| Recovery classifier trusted matching text without proving upstream provenance | Behaviour / Safety | `CanRecover` requires `pipeline.IsUpstreamError` | S3 | TDD | fixed |
| Compact structured path used `ResponseReasoningItemID` as response provenance for ciphertext | Spec | Module Spec review 2026-07-22; `reasoningMetadataFromMessage` in compact_inbound.go; fixed by always `buildReasoningItem(..., preserve=false)` + RawOutputItems merge; compact test asserts nil EncryptedContent without sidecar | S5 | TDD | fixed |

## Parent review (2026-07-22)

| Check | Result |
|---|---|
| Goal coverage | pass — identity preserve, issuer drop, one-shot recovery, provenance gate |
| Non-goals | pass — no deploy/8090/DB/fingerprint primary recovery |
| Composition | pass — S1–S7 compose via RawOutputItems + dropOpaque + ErrorRecoverable |
| Findings | pass — Spec finding closed after retest |
| Checks | pass — responses/pipeline/orchestrator packages |
| Risks | see below |

## Risks

- Independent Gemini-named fixture not added; covered by shared stream metadata gate + Anthropic cross-protocol tests (non-blocking coverage).
- No full end-to-end orchestrator stream recovery integration test beyond pipeline-level stream recovery (non-blocking).
- Worktree still uncommitted relative to `601fcfa3`.
