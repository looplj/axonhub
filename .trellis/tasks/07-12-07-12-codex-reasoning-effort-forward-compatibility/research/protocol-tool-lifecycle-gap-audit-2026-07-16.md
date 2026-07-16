# Research: protocol tool lifecycle gap audit

- Query: Identify remaining confirmed OpenAI Responses / Chat / Anthropic conversion gaps related to tool-call lifecycle and state-machine ordering after the uncommitted Responses→Chat consecutive-call grouping fix; specifically reasoning plus parallel function/custom calls, Chat→Responses output pairing/order, and Responses/Chat→Anthropic tool batches.
- Scope: mixed (current repository source/specs plus official OpenAI and Anthropic protocol documentation)
- Date: 2026-07-16

## Findings

### Ranked confirmed gaps

#### P0 — Chat custom tool calls are misclassified as Responses function calls

**Directions:** Chat → Responses request history; Chat-provider response → Responses client response.

**Source symbols / paths:**

- `openai.ToolCall.ToLLMToolCall`, `llm/transformer/openai/inbound_convert.go:12-44`: a Chat `type:"custom"` call is correctly stored in `llm.ToolCall.OpenAIChatCustomToolCall` (`:36-41`), while the common `Function` carrier remains empty.
- `responses.convertAssistantMessage`, `llm/transformer/openai/responses/outbound_convert.go:248-279`: only `ResponseCustomToolCall` selects `custom_tool_call`; every other call becomes `function_call` using `tc.Function.Name/Arguments`.
- `responses.convertToResponsesAPIResponse`, `llm/transformer/openai/responses/inbound.go:1192-1234`: the response path has the same discriminator: only `ResponseCustomToolCall` becomes `custom_tool_call`; Chat-native custom calls fall into the function branch.
- By contrast, the Chat outbound adapter already proves the two custom-call carriers can be bridged deliberately: `openai.ToolCallFromLLM`, `llm/transformer/openai/outbound_convert.go:390-402`.

**Minimal request-history fixture:**

```json
{
  "messages": [
    {"role":"assistant","tool_calls":[{"id":"call_c1","type":"custom","custom":{"name":"run_sql","input":"SELECT 1"}}]},
    {"role":"tool","tool_call_id":"call_c1","content":"ok"}
  ]
}
```

Current Chat→canonical→Responses request behavior emits a `function_call`/`function_call_output` lifecycle whose function name and arguments come from the empty `Function` carrier, instead of a `custom_tool_call`/`custom_tool_call_output` pair carrying `run_sql` and `SELECT 1`.

**Minimal response fixture:**

```json
{
  "choices":[{
    "message":{"role":"assistant","tool_calls":[{"id":"call_c1","type":"custom","custom":{"name":"run_sql","input":"SELECT 1"}}]},
    "finish_reason":"tool_calls"
  }]
}
```

Current Responses reconstruction enters the `function_call` branch and loses the custom name/input semantics.

**Recommended treatment:** **exact repair**. Treat `OpenAIChatCustomToolCall` as an explicit Chat→Responses custom bridge in both request-history and response conversion. Emit `custom_tool_call`; track its `call_id` so the later tool message becomes `custom_tool_call_output`. Do not route it through the function carrier.

---

#### P0 — Chat→Responses response item identity aliases `id` to `call_id`

**Direction:** Chat-provider response → Responses client response (function calls, and custom calls once the exact bridge above is applied).

**Source symbol / path:**

- `responses.convertToResponsesAPIResponse`, `llm/transformer/openai/responses/inbound.go:1192-1234`:
  - custom branch: when `ResponseItemID` is empty, `ctcItemID = toolCall.ID` (`:1195-1199`);
  - function branch: when `ResponseItemID` is empty, `fcItemID = toolCall.ID` (`:1213-1217`);
  - the same value is then emitted independently as output item `id` and correlation `call_id` (`:1204-1211`, `:1225-1232`).

**Minimal fixture:**

```json
{
  "choices":[{
    "message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"weather","arguments":"{}"}}]},
    "finish_reason":"tool_calls"
  }]
}
```

Current Responses output contains the equivalent of:

```json
{"type":"function_call","id":"call_1","call_id":"call_1","name":"weather","arguments":"{}"}
```

This collapses two distinct lifecycle identities. The repository Responses spec explicitly requires `id` and `call_id` to remain independent and forbids fallback between them (`docs/specs/protocols/openai-responses-protocol.md:87-105`).

**Recommended treatment:** **exact repair**. Preserve `call_id = toolCall.ID`, but allocate a distinct target Responses output-item ID when the Chat source has no Responses-native `ResponseItemID`. This is target-envelope construction, not preservation of a source item identity; it must never reuse `call_id`. Add a fixture asserting `id != call_id` and stable pairing through `call_id`.

---

#### P0 — Anthropic conversion performs non-local tool-result matching and moves results across intervening turns

**Directions:** Responses → Anthropic and Chat → Anthropic for function-tool histories.

**Source symbols / paths:**

- `anthropic.convertMessages`, `llm/transformer/anthropic/outbound_convert.go:529-588`: after every assistant tool-call message, it invokes `findToolResultsForAssistant` before continuing source-order iteration (`:565-581`).
- `anthropic.findToolResultsForAssistant`, `llm/transformer/anthropic/outbound_convert.go:591-651`: for each call, it scans **all messages** for a matching tool result (`:602-620`), immediately appends the gathered result batch after the assistant message (`convertMessages:574-575`), and marks the original later positions processed.

**Minimal fixture:**

```text
0 assistant: tool_call call_A
1 user: "intervening user turn"
2 tool: tool_call_id=call_A, content="late result"
```

Current Anthropic projection becomes effectively:

```text
assistant: tool_use call_A
user: tool_result call_A
user: "intervening user turn"
```

The converter has reordered the state machine rather than preserving or rejecting it. The same global scan can also reorder multiple result messages into assistant-call order rather than source message order.

Anthropic's official lifecycle requires the tool-result user message to immediately follow the assistant tool-use message; no intervening messages are allowed. Reordering an invalid source history hides the invalidity and changes conversation meaning.

**Recommended treatment:** **explicit reject/diagnostic**, with a minimal structural repair to the matcher. Only consume the immediately following contiguous tool-result batch. If a matching result exists later across another message/turn, reject the conversion as an invalid target lifecycle (preferred for request correctness) or emit an explicit lossy/invalid-order diagnostic; do not search globally and hoist it.

---

#### P0 — Anthropic parallel tool batches are emitted with missing results instead of rejected

**Directions:** Responses → Anthropic and Chat → Anthropic for parallel function calls.

**Source symbol / path:**

- `anthropic.findToolResultsForAssistant`, `llm/transformer/anthropic/outbound_convert.go:602-648`: it appends a result only when found, but returns a user `tool_result` message whenever `len(toolResultBlocks) > 0`; it never verifies that every assistant `ToolCall` got exactly one result.

**Minimal fixture:**

```text
assistant: tool_calls [call_A, call_B]
tool: result for call_A only
```

Current Anthropic output contains two `tool_use` blocks followed by a user message containing only the result for `call_A`.

Anthropic's official parallel-tool documentation requires one `tool_result` for each `tool_use`, all together in the next user message; a skipped call must still receive an error result. The emitted request is therefore an invalid/incomplete Anthropic lifecycle, not merely a formatting preference.

**Recommended treatment:** **explicit reject** by default. Validate the immediate batch for exact call-ID coverage, uniqueness, and no unknown result IDs before serialization. Do not synthesize successful results. A product-authorized compatibility mode could synthesize an `is_error:true` “not executed” result only when the source explicitly represents that state; absent such evidence, reject.

---

#### P1 — OpenAI custom-tool lifecycles are silently destroyed or malformed on Anthropic targets

**Directions:** Responses → Anthropic; Chat → Anthropic.

**Source symbols / paths:**

- Responses source:
  - `orchestrator.filterResponseCustomToolMessagesForNonResponsesOutbound`, `internal/server/orchestrator/outbound.go:410-426`, filters Responses custom calls/results for Anthropic before the adapter runs (Chat is explicitly exempt at current disk line `418`).
  - `shared.FilterOutResponseCustomToolMessages`, `llm/transformer/shared/messages.go:13-60`, removes the custom call and every matching tool-result message (`:21-49`).
- Chat source:
  - Chat custom calls are carried by `OpenAIChatCustomToolCall` (`llm/transformer/openai/inbound_convert.go:36-41`).
  - `anthropic.toolUseBlockFromLLM`, `llm/transformer/anthropic/outbound_convert.go:1535-1548`, ignores that carrier and serializes only `toolCall.Function.CompositeName()` / `Function.Arguments`, which are empty for a Chat custom call.
  - `anthropic.convertToolsAnthropic`, `llm/transformer/anthropic/outbound_convert.go:426-477`, emits only common function and supported web-search declarations; Chat/Responses custom declarations hit `default` and are skipped (`:435-474`).
- Diagnostic gap:
  - `anthropic.recordAnthropicChatNativeLossyDowngrades`, `llm/transformer/anthropic/outbound.go:433-515`, diagnoses selected OpenAI request fields but has no custom tool declaration/call/result lifecycle entry.
  - Responses custom history is removed in the orchestrator before the Anthropic adapter can diagnose the loss.

**Minimal Responses fixture:**

```json
{
  "input":[
    {"type":"custom_tool_call","call_id":"call_c1","name":"apply_patch","input":"patch"},
    {"type":"custom_tool_call_output","call_id":"call_c1","output":"done"}
  ],
  "tools":[{"type":"custom","name":"apply_patch"}]
}
```

Current Anthropic routing removes the call/output pair; the custom declaration is unsupported. No field-level lossy lifecycle diagnostic is emitted at the removal site.

**Minimal Chat fixture:**

```json
{
  "messages":[
    {"role":"assistant","tool_calls":[{"id":"call_c1","type":"custom","custom":{"name":"run_sql","input":"SELECT 1"}}]},
    {"role":"tool","tool_call_id":"call_c1","content":"ok"}
  ],
  "tools":[{"type":"custom","custom":{"name":"run_sql"}}]
}
```

Current direct Anthropic projection omits the declaration and builds `tool_use` from an empty function carrier.

**Recommended treatment:** **explicit lossy diagnostic/reject**, not an automatic exact mapping. OpenAI custom tools use free-form input/grammar semantics; Anthropic client tools require JSON `input` conforming to `input_schema`. Move/duplicate the unsupported-lifecycle check to a place that can record source protocol, call IDs, declaration count, and removed result count before filtering. Reject when a pending custom lifecycle would otherwise create an invalid target history; otherwise allow explicit loss only with diagnostics.

### Confirmed non-gap in the requested reasoning/parallel slice

The uncommitted Responses input fix groups consecutive `function_call` and `custom_tool_call` items into one canonical assistant message (`llm/transformer/openai/responses/inbound.go:475-528`). When preceded by a Responses `reasoning` item, `convertReasoningWithFollowing` already consumes both function and custom calls into the same assistant message (`:534-632`). No additional function/custom grouping gap was confirmed in that narrow Responses request path. The remaining confirmed failures are the cross-protocol custom discriminators and Anthropic lifecycle validation above.

## Files Found

| Path | Description |
|---|---|
| `llm/transformer/openai/responses/inbound.go` | Reconstructs Responses responses from canonical Chat-like responses; contains tool item identity fallback and custom discriminator. |
| `llm/transformer/openai/responses/outbound_convert.go` | Converts canonical messages to Responses request items; tracks output type by call ID but recognizes only Responses-native custom carrier. |
| `llm/transformer/openai/responses/inbound.go` | Converts Responses request input items to canonical messages, including the current reasoning/consecutive parallel-call grouping fix. |
| `llm/transformer/openai/inbound_convert.go` | Stores Chat custom calls in the Chat-native custom carrier. |
| `llm/transformer/openai/outbound_convert.go` | Existing explicit custom-call/declaration bridges used by Chat outbound; demonstrates carrier discrimination. |
| `llm/transformer/anthropic/outbound_convert.go` | Converts ordered canonical history to Anthropic blocks; globally matches results, allows incomplete batches, and serializes tool uses from function carrier only. |
| `llm/transformer/anthropic/outbound.go` | Records allowlisted OpenAI→Anthropic lossy diagnostics; custom lifecycle loss is absent. |
| `llm/transformer/shared/messages.go` | Removes Responses custom calls and their result messages. |
| `internal/server/orchestrator/outbound.go` | Applies the Responses-custom filter before non-Responses outbound transformation. |
| `docs/specs/protocols/openai-responses-protocol.md` | Requires output item order and independent Responses item/call identities. |
| `docs/specs/protocols/openai-chat-completions-protocol.md` | Defines ordered Chat messages and current function/custom tool forms. |
| `docs/specs/protocols/anthropic-claude-messages-protocol.md` | Defines Anthropic content-block tool lifecycle and later user `tool_result`. |
| `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md` | Strict field/status ledger used to avoid extrapolating partial coverage. |
| `docs/specs/protocols/hub-protocol-field-matrix.md` | Cross-protocol risk summary and residual audit boundaries. |

## Code Patterns

- **Correct local grouping pattern:** consume only consecutive Responses call items and preserve both call types (`llm/transformer/openai/responses/inbound.go:475-528`).
- **Incorrect discriminator pattern:** use `ResponseCustomToolCall != nil` as the only custom test even when source Chat uses `OpenAIChatCustomToolCall` (`llm/transformer/openai/responses/outbound_convert.go:248-279`; `inbound.go:1192-1234`).
- **Incorrect identity fallback:** assign Responses output item `id` from tool correlation `call_id` (`llm/transformer/openai/responses/inbound.go:1195-1217`).
- **Incorrect state-machine normalization:** search all later messages for results and move them immediately after a call (`llm/transformer/anthropic/outbound_convert.go:591-651`).
- **Missing batch invariant:** emit a result message when any result exists, without exact call-set coverage (`llm/transformer/anthropic/outbound_convert.go:602-648`).
- **Silent unsupported filtering:** remove custom call/result pairs before the target adapter can diagnose them (`internal/server/orchestrator/outbound.go:410-426`; `llm/transformer/shared/messages.go:13-60`).

## External References

- Anthropic, **Handle tool calls** (fetched 2026-07-16): https://platform.claude.com/docs/en/agents-and-tools/tool-use/handle-tool-calls
  - Tool results must immediately follow their corresponding tool-use turn.
  - `tool_result` blocks must come first in the next user content array.
- Anthropic, **Parallel tool use** (fetched 2026-07-16): https://platform.claude.com/docs/en/agents-and-tools/tool-use/parallel-tool-use
  - Return one result for every tool use, all together in the next user message; skipped executions still require an error result.
- OpenAI, **Function calling** (fetched 2026-07-16): https://developers.openai.com/api/docs/guides/function-calling
  - Distinguishes function and custom tools, identifies `call_id` as the result correlation key, and requires preserving reasoning/function-call items when manually maintaining Responses state.
- Checked-in OpenAI Responses create snapshot (2026-07-06): `docs/specs/vendor/openai/official-2026-07-06/_try_responses-create.md`
  - Confirms `function_call_output`, `custom_tool_call_output`, custom call/output `call_id`, and reasoning items as distinct typed Responses items.

## Related Specs

- `.trellis/spec/backend/protocol-transformer-guidelines.md`
  - Same-protocol first; cross-protocol conversion requires explicit bridge or lossy/unsupported diagnostic; do not treat similarly named fields/protocol constructs as equivalent.
- `docs/specs/protocols/openai-responses-protocol.md:77-105,125-165`
  - Preserve call/output identity and output order; keep item `id` independent from `call_id`; cross-protocol bridges must be explicit.
- `docs/specs/protocols/openai-chat-completions-protocol.md:78-130`
  - Chat custom tools/calls are current protocol shapes, not function-only compatibility data.
- `docs/specs/protocols/anthropic-claude-messages-protocol.md:86-115`
  - Anthropic tool calls/results are ordered content blocks and not renamed OpenAI roles/items.
- `docs/specs/protocols/hub-protocol-field-matrix.md:180-210`
  - Identifies OpenAI↔Anthropic tool/thinking conversions as explicit bridge/diagnostic territory.

## Caveats / Not Found

- This was a read-only audit. No code, tests, specs, git state, lint/build, or server state were changed.
- Per the researcher-agent write boundary, the report is persisted under the active task's `research/` directory. The user-requested mirror path `.agent/research/protocol-tool-lifecycle-gap-audit-2026-07-16.md` was **not** written because this agent is forbidden to write outside `{TASK_DIR}/research/`.
- No new investigation was started after the user's stop instruction; this report only records evidence already collected.
- No additional gap is claimed for the already-fixed Responses consecutive parallel grouping itself.
- No automatic OpenAI-custom→Anthropic bridge is recommended without a product decision defining how free-form/grammar input becomes Anthropic JSON `input_schema` semantics.
- Stream-specific Chat→Responses custom-call event conversion was not promoted to a separate finding here; the confirmed non-stream/request-history discriminator defect should be fixed first and then covered by a dedicated stream fixture.
