# Research: Codex Responses wire delta `1f0566d..9e552e9d1`

- Query: Contrast Codex source range `1f0566d..9e552e9d1` for fields/semantics that actually enter OpenAI Responses request/response wire payloads, limited to `codex-rs/core/src/client.rs`, `codex-rs/protocol/src/openai_models.rs`, `codex-rs/protocol/src/items.rs`, `codex-rs/protocol/src/protocol.rs`, `codex-rs/protocol/src/models.rs`, and directly related tests.
- Scope: mixed (Codex source + AxonHub protocol docs for classification only)
- Date: 2026-07-12
- Codex path: `/Users/asuan/项目/ai工具/openai-codex`
- Range endpoints:
  - base: `1f0566d3f59298d1bb88820a0d35294f1eeb07ea` (2026-07-09)
  - head: `9e552e9d15ba52bed7077d5357f3e18e330f8f38` (2026-07-11)

## Files found

| Path | Description |
|---|---|
| `codex-rs/core/src/client.rs` | Builds Responses HTTP/WS request body (`Reasoning`, `include`, `stream_options`, item id filtering). |
| `codex-rs/protocol/src/openai_models.rs` | Model catalog metadata; `ReasoningEffort` enum; capability flag rename. |
| `codex-rs/protocol/src/models.rs` | `ResponseItem` variants and item `id` type. |
| `codex-rs/protocol/src/response_item_id.rs` | New typed `ResponseItemId` (transparent string on wire). |
| `codex-rs/protocol/src/response_item_id_tests.rs` | Prefix recognition / legacy deserialize tests. |
| `codex-rs/protocol/src/items.rs` | Hook prompt builder now creates `msg_`-prefixed IDs. |
| `codex-rs/protocol/src/protocol.rs` | Codex app/event protocol structs (not OpenAI Responses body). |
| `codex-rs/codex-api/src/common.rs` | `ResponsesApiRequest` / `Reasoning` serde shapes used by client. |
| `codex-rs/core/tests/suite/client.rs` | Wire assertions for reasoning/include/stream_options and Azure item ids. |
| `codex-rs/core/tests/suite/client_websockets.rs` | WS request omits unprefixed item ids. |

## Commits in scope touching the listed files

| Commit | Title | Wire-relevant? |
|---|---|---|
| `d2d00b663` | Always send reasoning parameters in Responses requests (#32206) | **Yes** — request body `reasoning` / `include` emission |
| `dffe1f02a` | Respect model support for reasoning summaries (#32290) | **Yes** — `reasoning.summary` and `stream_options` emission gate |
| `c9d52de5c` | Require prefixes for outbound response item IDs (#32312) | **Yes** — request `input[].id` emission filter |
| `09ccae2c0` | Honor `personality = "none"` in model instructions (#32277) | Partial — changes `instructions` **text content**, not field schema |
| `bca577d69` | Include start times in terminal turn events (#32263) | No OpenAI Responses body |
| `c4318c386` | Include terminal errors in turn completion events (#32280) | No OpenAI Responses body |
| `5c19155cb` | Add ordinals to paginated rollout records (#32332) | No OpenAI Responses body |

## Findings (wire-relevant)

### 1. Always emit `reasoning` object + always include `reasoning.encrypted_content`

- **Change**
  - Before (`1f0566d`): `build_reasoning` returned `Option<Reasoning>` gated by `model_info.supports_reasoning_summaries`. If that flag was false, request had `reasoning: null/omitted path` and `include: []`.
  - After (`d2d00b663` then refined by `dffe1f02a`): `build_reasoning` always returns a `Reasoning` value; request always sets `reasoning: Some(...)`.
  - `include` is always `["reasoning.encrypted_content"]`, no longer gated on reasoning presence.
- **Evidence**
  - Commit message `d2d00b663`: “Build a reasoning payload for every Responses request and always include `reasoning.encrypted_content`.”
  - `codex-rs/core/src/client.rs` @ `9e552e9d1` around `build_reasoning` / request assembly:
    - `include = vec!["reasoning.encrypted_content".to_string()];`
    - `reasoning: Some(reasoning),`
  - Pre-change gate at `1f0566d` `client.rs`:
    - `if model_info.supports_reasoning_summaries { Some(Reasoning { ... }) } else { None }`
    - `include = if reasoning.is_some() { vec!["reasoning.encrypted_content"] } else { Vec::new() }`
  - Integration test still asserts encrypted include even when summary is omitted:
    - `codex-rs/core/tests/suite/client.rs` `model_without_summary_parameter_support_omits_configured_summary`
    - asserts `request_body["include"] == ["reasoning.encrypted_content"]`
    - asserts `request_body["reasoning"] == {"effort":"high"}`
- **Wire vs client layer**
  - **Wire emission policy (client → OpenAI Responses request body)**.
  - Does **not** introduce a new OpenAI field name. Fields remain existing public Responses fields: top-level `reasoning` and `include[]` member `reasoning.encrypted_content` (also documented in AxonHub baseline / batch-reasoning-stream).
  - Nested fields still use existing serde skip-if-none (`codex-rs/codex-api/src/common.rs` `Reasoning.{effort,summary,context}`).
- **Suggested classification**
  - **G9 (new)** — Responses request emission policy for `reasoning` / `include["reasoning.encrypted_content"]` always-on.
  - Not a new schema field; Hub impact is “preserve/pass through these request fields when present,” not invent Codex client policy.
  - Related to prior G7 reasoning work, but G7 focused on context/generate_summary/stream text; this is request emission always-on.

### 2. Split summary capability: omit `reasoning.summary` and `stream_options.reasoning_summary_delivery` when model rejects summary parameter

- **Change**
  - Metadata rename: `supports_reasoning_summaries` → `supports_reasoning_summary_parameter` on `ModelInfo` (`openai_models.rs`).
  - Default for new flag: `true` (`serde(default = "default_true")`), so missing catalog field means “support summary”.
  - Emission rule after `dffe1f02a`:
    - `summary` included only if `supports_reasoning_summary_parameter && summary != None`
    - `stream_options` with `reasoning_summary_delivery: sequential_cutoff` only if concurrent feature enabled **and** provider is OpenAI **and** `reasoning.summary.is_some()`
  - Effort is no longer blocked by the old global “supports reasoning summaries” gate.
- **Evidence**
  - Commit `dffe1f02a` message: omit `reasoning.summary` and summary-delivery stream option when model does not support the parameter.
  - `client.rs` `build_reasoning`:
    - `summary: (model_info.supports_reasoning_summary_parameter && summary != ReasoningSummaryConfig::None).then_some(summary)`
  - `client.rs` request assembly:
    - `stream_options = (concurrent && is_openai && reasoning.summary.is_some()).then_some(...)`
  - Test `model_without_summary_parameter_support_omits_configured_summary` (`client.rs`):
    - model catalog `supports_reasoning_summary_parameter = false`
    - config sets effort high + summary detailed + concurrent feature on
    - wire: `reasoning == {"effort":"high"}` (no summary key)
    - wire: `stream_options` absent
    - wire: include still has encrypted content
  - Contrast: `configured_reasoning_summary_is_sent` still expects summary `"concise"` and `stream_options.reasoning_summary_delivery == "sequential_cutoff"` when support is default/true.
- **Wire vs client layer**
  - **Client capability-gated emission over existing wire fields**.
  - Wire fields involved: `reasoning.summary`, `stream_options.reasoning_summary_delivery` (Codex/OpenAI concurrent-summary option; not a new field name in this range).
  - Catalog flag itself is **not** an OpenAI Responses request field; it is Codex model metadata.
- **Suggested classification**
  - **G10 (new)** — capability-gated emission of `reasoning.summary` / related `stream_options`.
  - Hub same-protocol path should continue to preserve whatever the inbound client actually sent; Hub should not re-implement Codex model-catalog gating unless acting as Codex itself.

### 3. Outbound `input[].id` must be prefixed; unprefixed/empty IDs are stripped from the request only

- **Change**
  - New type `ResponseItemId` (`response_item_id.rs`): transparent string wrapper; deserialize still accepts arbitrary strings including legacy UUIDs.
  - `is_prefixed()` requires non-empty `prefix_suffix` form (`split_once('_')` with both sides non-empty).
  - All `ResponseItem` variant `id: Option<String>` → `Option<ResponseItemId>` (`models.rs`).
  - `prepare_response_items_for_request` (`client.rs`):
    1. For every item, if `id` exists and `!is_prefixed()`, set `id = None` (so serde omits it).
    2. Existing behavior retained: if item-ids feature disabled and not Azure `store`, strip all ids.
  - Prefix table via `ResponseItem::id_prefix()` (msg/rs/fc/ws/…).
  - Hook prompt builder now assigns `ResponseItemId::new("msg")` (`items.rs`).
- **Evidence**
  - Commit `c9d52de5c` message: omit empty or unprefixed item IDs from HTTP and WebSocket requests; deserialization remains permissive.
  - `response_item_id_tests.rs`:
    - `"msg_test"` prefixed true; `"legacy-id"`, `""`, `"_test"`, `"msg_"` false
    - legacy `"legacy-id"` round-trips as string
  - HTTP Azure test in `client.rs` (`azure_responses_request_includes_store_and_prefixed_item_ids`):
    - prefixed ids retained (`rs_reasoning-id`, `msg_message-id`, …)
    - legacy UUID id and empty id become absent (`body["input"][8|9].get("id") == None`)
  - WS test `responses_websocket_omits_unprefixed_item_ids_without_mutating_prompt`:
    - request omits unprefixed/empty ids
    - original prompt still holds the legacy id (mutation is on the outbound copy / prepared request path, not the stored prompt value asserted in test)
- **Wire vs client layer**
  - **Wire emission policy for existing Responses item field `id`** (still a JSON string when present).
  - Type change is client/protocol Rust typing; JSON shape remains string.
  - Semantic change: Codex no longer re-sends unprefixed historical IDs to the model API.
- **Suggested classification**
  - **G11 (new)** — Responses `input`/output item `id` identity preservation vs client-side strip of unprefixed values.
  - Hub should treat item `id` as open string if present; do not invent Codex prefix rules for non-Codex clients. If Hub proxies Codex multi-turn history, stripping unprefixed IDs is Codex client behavior, not an OpenAI schema change.

### 4. Personality `"none"` can remove `# Personality` section from base instructions text

- **Change**
  - `openai_models.rs` instruction selection: when personality is `None` (or unset), do not warn; when friendly/pragmatic requested without templates, warn and fall back.
  - Actual section stripping is primarily in models-manager (outside the strict file list but caused by this commit). Effect on Responses wire is only the string value of top-level `instructions` (or developer message text under responses-lite).
- **Evidence**
  - Commit `09ccae2c0` message + `openai_models.rs` personality match change.
  - No new JSON field.
- **Wire vs client layer**
  - **Client content generation** for existing `instructions` / developer message content.
- **Suggested classification**
  - **不属于 Hub 协议字段缺口** (content policy).
  - Optional note under instructions fidelity only; not a G-module for schema/mapping.

### 5. Non-OpenAI-Responses protocol changes in listed files (explicit exclusions)

These appear in the allowed files but **do not enter OpenAI Responses request/response bodies**:

| Change | Evidence | Layer | Classification |
|---|---|---|---|
| `TurnCompleteEvent.started_at` / `error` | `protocol.rs` + commits `bca577d69`, `c4318c386` | Codex app/event protocol | **不属于 Hub 协议** (Codex app-server event) |
| `TurnAbortedEvent.started_at` | `protocol.rs` + `bca577d69` | Codex app/event protocol | **不属于 Hub 协议** |
| `RolloutLine.ordinal` | `protocol.rs` + `5c19155cb` | local rollout persistence | **不属于 Hub 协议** |
| `InterAgentCommunication.id: Option<ResponseItemId>` | `protocol.rs` + `c9d52de5c` | Codex multi-agent protocol typing | **不属于 OpenAI Responses wire schema**; id string may later appear inside Responses input only via item conversion path covered in §3 |
| Removal of `supports_reasoning_summaries` catalog field / config override | `d2d00b663` + `openai_models.rs` | Codex model catalog / config | **不属于 OpenAI Responses wire**; client metadata only |
| `ReasoningEffort` known variants / `Custom` / `ultra` | unchanged in this range (`git diff` shows no enum delta) | already present at base | **no delta in range**; baseline still relevant via prior research `.agent/research/codex-reasoning-effort-latest-2026-07-12.md` |

## Non-changes confirmed in range

- No new `ReasoningEffort` wire values added between `1f0566d` and `9e552e9d1`.
- `ultra -> max` normalization in `client.rs` `reasoning_effort_for_request` already existed at base and is **unchanged** in this range (still client-layer normalization before wire).
- No new top-level Responses request keys beyond existing `reasoning` / `include` / `stream_options` / `input[].id` emission behavior.
- No response SSE event schema changes in the audited files.

## Suggested G-map for implement/check agents

| ID | Topic | Hub relevance |
|---|---|---|
| **G9** | Always-on `reasoning` object + `include: ["reasoning.encrypted_content"]` from Codex client | Same-protocol preserve only; do not force always-on for non-Codex clients unless product requires Codex parity |
| **G10** | Capability-gated `reasoning.summary` and summary `stream_options` | Preserve inbound values; do not invent model-catalog gates inside Hub transformers |
| **G11** | Prefixed item `id` outbound filtering | Preserve present string ids; unprefixed strip is Codex client policy |
| — | Personality instruction text | Not a protocol field module |
| — | TurnComplete/TurnAborted/Rollout ordinal | Not OpenAI Responses / not Hub LLM protocol |

If G numbering must continue previous parent G1–G7 without gaps: treat the above as candidate **G8–G10** instead; no pre-existing G8–G15 registry was found in Hub docs (only G1–G7 completed). Labels G9–G11 above follow the user prompt’s “G9–G15 / 新 G” instruction and leave G8 free if another residual claims it.

## Related specs

- `.trellis/spec/backend/protocol-transformer-guidelines.md` — same-protocol preserve vs cross-protocol no-synth.
- `docs/specs/protocols/openai-responses-protocol.md` — `include`, `reasoning`, `stream_options` baseline fields.
- `docs/specs/protocols/drafts/batch-reasoning-stream.md` — `reasoning.encrypted_content` include member and summary fields.
- `docs/specs/protocols/hub-protocol-field-matrix.md` — G1–G7 closed; next residuals listed separately (not these Codex client emission policies).
- Prior baseline research: `.agent/research/codex-reasoning-effort-latest-2026-07-12.md` (`ultra->max`, open string effort).

## Caveats / Not Found

- Audited only the requested files plus direct tests and the `ResponsesApiRequest` serde type they serialize into. Other commits in `1f0566d..9e552e9d1` outside this file set were not fully audited.
- Did not claim OpenAI public API newly requires always-on `reasoning`/`include`; evidence is **Codex client emission policy**, not a published OpenAI schema delta.
- Did not re-diff response stream event enums outside listed files.
- `stream_options.reasoning_summary_delivery` remains a Codex/OpenAI concurrent-summary option already used before this range; only the emission gate tightened.
- `ResponseItemId` wire remains a string (`#[serde(transparent)]`); no object-shaped id.
- Empty reasoning object (all nested fields None) is possible in theory if effort is None and summary omitted and context omitted; this range does not add a test asserting `{}` vs omitted. Observed tests always have at least effort when asserting body.
- Personality section stripping implementation details live mainly in `models-manager`, not fully in the five listed files.
