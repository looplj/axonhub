# Chat / common post-C2/D1 gap audit

- Date: 2026-07-13
- Worker scope: `llm/transformer/openai/` excluding `responses/`, plus Chat-only `llm/openai_chat.go`; D1 owners outside that scope were read only.
- Public seam: OpenAI Chat HTTP/body inbound → canonical `llm` request/response → OpenAI Chat HTTP/body outbound. Response and stream fixtures use the corresponding public transformer methods.
- Authorities read: the strict matrix, OpenAI Chat / Responses / Anthropic local protocol baselines, the canonical Chat create snapshot, all five field inventories, and `.trellis/spec/backend/protocol-transformer-guidelines.md`.
- Explicit exclusions honored: no edits to Anthropic, Responses, shared `llm` models, provider extensions, or strict matrix; no `temperature > 1` or `service_tier` policy change.

## Result

No additional source-proven Chat-owned production loss was found after the current shared-tree C1/C2 implementation. The four inventory findings are implemented at the Chat-owned seam and have public fixtures. Two previously unproved official child/direction cases remain `TEST_ONLY`: request-history custom calls and the `file_data` child. Temporary public-seam probes for both passed immediately, establishing no additional production repair was needed; the probes were removed because the existing untracked C1/C2 fixture file belongs to another concurrent worker.

D1's safe field-level diagnostics are target-outbound policy. The current implementation and RED/GREEN fixtures are in Anthropic outbound code owned by the parallel Anthropic worker. I did not modify them. Remaining Responses-target diagnostic work is likewise outside this worker's write scope and is not claimed complete here.

## C1 — tools, tool choice, custom calls

### Source and wire evidence

The local Chat baseline requires current custom tools and object tool choices (`openai-chat-completions-protocol.md`, tools section). The canonical snapshot defines:

- Chat custom declaration: `tools[].type="custom"`, with `custom.name`, optional `description`, and `format`;
- named custom choice: `{type:"custom", custom:{name}}`;
- allowed-tools choice: `{type:"allowed_tools", allowed_tools:{mode,tools}}`;
- custom call: `{id,type:"custom",custom:{name,input}}`; stream deltas additionally carry `index`.

Current assignment/return evidence:

- `transformer/openai/model.go`: `Tool.ToLLMTool` carries Chat custom declarations in the Chat-specific canonical carrier; `Tool.MarshalJSON` emits the Chat custom shape and does not emit an empty function object.
- `transformer/openai/inbound_convert.go:36-41`: `ToolCall.ToLLMToolCall` copies custom `name`, `input`, and optional stream `index`.
- `transformer/openai/inbound_convert.go:145-155`: request conversion copies named custom and raw allowed-tools choice state.
- `transformer/openai/outbound_convert.go`: `RequestFromLLM` emits only function tools or Chat-owned custom tools, thereby not relabeling Responses custom tools as Chat custom tools; choice conversion restores named custom / allowed-tools forms; `ToolCallFromLLM` restores Chat custom calls.
- `transformer/openai/model.go:575-625`: `ToolChoice` JSON marshal/unmarshal keeps string, named function, named custom, and allowed-tools variants distinct.
- `Message.ToLLMMessage` and `MessageFromLLMWithConfig` use the same tool-call converter for request history, independently of non-stream response and stream conversion.

### Public fixtures and classification

| Behavior | Evidence | Classification | Audit result |
|---|---|---|---|
| Chat custom declaration + named custom choice | `TestOpenAIChatRequest_CustomToolsChoiceAndContentPartsRoundTrip` | `IMPLEMENTATION` resolved in current tree | Exact declaration/format and choice survive Chat → canonical → Chat. |
| `allowed_tools` object and ordered raw tool references | `TestOpenAIChatRequest_AllowedToolsChoiceRoundTrip` | `IMPLEMENTATION` resolved in current tree | Mode and function/custom references survive exactly; no cross-protocol interpretation is added. |
| Custom call in non-stream response | `TestOpenAIChatResponse_CustomToolCallRoundTrip` | `IMPLEMENTATION` resolved in current tree | `id`, `type`, `name`, and `input` survive. |
| Custom call in request history | temporary public-seam probe (not retained) | `TEST_ONLY` | Probe passed immediately through the public request seam; no production change was needed. Durable fixture ownership remains with C1. |
| Custom call in stream delta | `TestOpenAIChatStream_CustomToolCallRoundTrip` | `IMPLEMENTATION` resolved in current tree | Stream `index`, call `id`, `type`, `name`, and partial `input` survive. |
| Responses custom declaration isolation | existing `TestRequestFromLLM_FiltersResponsesCustomTools` plus the explicit filter in `RequestFromLLM` | `EXPLICIT_NO_IMPLEMENT` | Chat and Responses custom wire schemas are not treated as interchangeable. |

No evidence justified a generic raw custom-tool fallback or a cross-protocol custom-tool mapping.

## C2 — file and refusal content parts

### Source and wire evidence

The canonical Chat snapshot defines file parts as `{type:"file",file:{file_data?,file_id?,filename?}}` and refusal parts as `{type:"refusal",refusal:string}`. These are content-array variants and are not equivalent to the message-level `refusal` field.

Current assignment/return evidence:

- `transformer/openai/model.go`: `MessageContentPart` has native `File` and `Refusal` members; `FileContent` contains pointer fields for `file_data`, `file_id`, and `filename`.
- `transformer/openai/inbound_convert.go:338-348`: `MessageContentPart.ToLLMPart` copies all three file children and the refusal string into Chat-only canonical carriers.
- `transformer/openai/outbound_convert.go` (`MessageContentPartFromLLM`): restores those carriers to their native Chat part variants while preserving array order through the existing content slice conversion.

### Public fixtures and classification

| Behavior | Evidence | Classification | Audit result |
|---|---|---|---|
| Mixed text + file-id part, followed by assistant refusal part | `TestOpenAIChatRequest_CustomToolsChoiceAndContentPartsRoundTrip` | `IMPLEMENTATION` resolved in current tree | Part order, file ID/filename, and refusal payload survive. |
| Base64 `file_data` + filename | temporary public-seam probe (not retained) | `TEST_ONLY` | Probe passed immediately; no production edit was needed. Durable fixture ownership remains with C2. |
| Cross-protocol file/refusal invention | no mapping added | `EXPLICIT_NO_IMPLEMENT` | Chat-native part carriers do not establish semantic equivalence with Responses or Anthropic blocks. |

## Same-protocol raw paths

The existing Chat top-level raw replay remains narrowly owned by `openAIChatRawPreserveFields` / `marshalOpenAIChatRequest` (`chat_n.go`). Existing public fixtures cover `n`, `prompt_cache_retention`, output controls, `web_search_options`, and deprecated `functions` / `function_call`, including no-synthesis behavior.

Classification: `EXPLICIT_NO_IMPLEMENT` for broadening this into a generic unknown-field bag. C1/C2 use explicit Chat-native typed carriers because their structured children participate in message/tool conversion; they do not rely on or widen the top-level raw replay seam. No same-family raw field was found accidentally emitted to a different family in the audited Chat code.

## D1 — safe field-level lossy diagnostics

The safe diagnostic rule is: only record a supplied field whose target has no tested equivalent, and leave the target payload unchanged. No value translation is allowed.

### Verified current Anthropic-target implementation (read only)

Target owner: `transformer/anthropic/outbound.go`, `recordAnthropicChatNativeLossyDowngrades`, called by Anthropic `TransformRequest` before serialization.

The current explicit allowlist records:

- `frequency_penalty` and `presence_penalty` for Chat or Responses source;
- `seed` for Chat source only (Responses has no seed wire field);
- `safety_identifier`, `prompt_cache_key`, and non-`user_id` OpenAI metadata remainder for Chat or Responses source;
- the pre-existing raw Chat-only fields such as `prompt_cache_retention`, without turning the recorder into reflection over every request field.

Public tests in `transformer/anthropic/outbound_test.go` assert both sides of the contract:

- `TestOutboundTransformer_TransformRequest_DiagnosesChatSamplingPenaltiesLoss`;
- `TestOutboundTransformer_TransformRequest_DiagnosesResponsesSamplingPenaltiesLoss`;
- `TestOutboundTransformer_TransformRequest_DiagnosesChatSeedLoss` (zero and non-zero presence);
- `TestOutboundTransformer_TransformRequest_DiagnosesOpenAIMetadataOnlyLosses`;
- the existing prompt-cache-retention/raw Chat diagnostic fixtures.

Those tests assert the Anthropic JSON omits the source fields and the original canonical values are not rewritten; diagnostics are sidecar tuples only. Classification: `DIAGNOSTIC`, implemented in the correct target owner. Files are out of this worker's write scope, so no duplicate Chat-side recorder was added.

### Not claimed complete

- Chat/Anthropic `temperature > 1`: `BLOCKED_PRODUCT_DECISION`. The current blind bridge is known, but error/drop/diagnostic policy is not approved. Untouched.
- Cross-family `service_tier`: `BLOCKED_PRODUCT_DECISION`. Enum/value semantics are incompatible and no mapping policy exists. Untouched.
- Stop/stop-sequences loss on a Responses target and any Responses-native retention diagnostic: target ownership is `transformer/openai/responses/`; this worker did not modify or claim those paths.
- Official null/default semantics and aggregate Field Decision Record status: `DOC_ONLY`; Go pointer presence alone is not protocol evidence.
- Broad common-field same-family matrices (model, stream, top-p, token limits, metadata identity, stop presence): `TEST_ONLY` backlog; no source-proven Chat production loss was established in this focused audit.

## RED / GREEN record

- Current C1/C2 production repair and its original RED history predate this audit in the shared uncommitted tree; I did not reproduce RED by reverting another worker's changes, because doing so would violate the shared-tree boundary.
- Focused inherited C1/C2 fixtures: GREEN.
- Temporary request-history custom-call probe: passed on first run → `TEST_ONLY`, no production change; removed to avoid modifying another worker's untracked fixture file.
- Temporary `file_data` child probe: passed on first run → `TEST_ONLY`, no production change; removed for the same ownership reason.
- Exact requested command `cd llm && go test ./transformer/openai -count=1`: attempted, but the managed sandbox denied access to the default macOS Go cache before test execution.
- Equivalent package command with only `GOCACHE=/tmp/axonhub-go-cache` added: PASS. The cache override does not alter package selection or test behavior.
- `git diff --check`: PASS.

## Files changed by this audit

- `.trellis/tasks/07-12-07-12-codex-reasoning-effort-forward-compatibility/research/reviews/chat-common-post-c2-d1-gap-audit.md`
  - this report only.

No production or fixture file is delivered by this audit. The current shared tree contains pre-existing C1/C2 production edits in `llm/openai_chat.go`, `transformer/openai/inbound_convert.go`, `model.go`, and `outbound_convert.go`, plus the untracked C1/C2 fixture file; this audit reviewed them but did not retain any modification to them.
