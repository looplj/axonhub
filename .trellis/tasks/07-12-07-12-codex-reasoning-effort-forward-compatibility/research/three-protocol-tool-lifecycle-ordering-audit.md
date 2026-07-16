# Research: three-protocol tool lifecycle ordering audit

- Query: 只读检查 OpenAI Chat Completions、OpenAI Responses、Anthropic Messages 三协议工具调用生命周期，以及与请求 #50829 同类的顺序、并行分组和 call/output 配对问题。
- Scope: mixed
- Date: 2026-07-16

## Findings

### Executive conclusion

1. **#50829 的已知根因类在当前工作树已有定点修复，而且定向测试通过。** Responses 请求中连续的 `function_call` / `custom_tool_call` item 现在会先合并为一个 canonical `assistant` message 的多个 `ToolCalls`，再把后续 output 转成各自的 `tool` message；因此典型的 `[call A, call B, output A, output B]` 不再变成非法 Chat 历史 `[assistant(A), assistant(B), tool(A), tool(B)]`，而是 `[assistant(A,B), tool(A), tool(B)]`。
2. **OpenAI Chat 的直接转换没有生命周期状态机。** 入站和出站都按消息切片顺序一对一映射，并按 `ToolCalls` 切片顺序映射。合法 Chat 历史不会被主动重排；但 orphan output、重复 output、缺失 output、batch 中插入普通消息等非法历史也不会在该层被拒绝。
3. **Chat → Responses / canonical → Responses 有已证实的配对校验缺口。** `convertInputFromMessages` 只用此前见过的 `call_id -> output item type` map 决定 `function_call_output` 还是 `custom_tool_call_output`，未验证 output 是否引用一个此前未完成的 call；找不到时默认发成 `function_call_output`。仓库测试甚至明确接受 `ToolCallID=nil` 并发出 `call_id:""`。
4. **Responses → canonical 仍只有局部结构校验，不是完整生命周期校验。** `function_call_output` / `custom_tool_call_output` 只要求 `Output != nil`，然后直接生成 role=`tool`；不检查 `call_id` 是否已发起、是否重复、是否越过 batch 边界。因此 #50829 的连续调用分组已修，但全局 call/output validator 仍不存在。
5. **Anthropic 的正常并行调用分组路径可用。** 一个 canonical assistant 的多个 tool calls 会生成同一 assistant `content[]` 中多个 `tool_use`；匹配结果按 assistant `ToolCalls` 的顺序收集为紧随其后的单一 user message 中多个 `tool_result`。现有并行 fixture 与定向测试通过。
6. **Anthropic outbound 会“修复式重排”而不是验证。** `findToolResultsForAssistant` 会扫描整个 message 列表寻找匹配 ID，即使结果并非紧随 assistant，也会把它提前放到对应 assistant 后面；这满足 Anthropic 的严格邻接规则，却可能跨过原有普通消息并改变 transcript 顺序。该行为在源码中确证，但本轮没有生产失败实例证明其已造成事故，应标为“已证实行为/风险”，不是已证实线上故障。
7. **合法 Anthropic user tool-result 消息的 block 顺序有覆盖。** 入站把 `tool_result` 拆为 canonical tool message，并用 `MessageIndex` 将同一原始 user message 的后续 text/raw block 关联回来；outbound 先放全部 `tool_result`，再附加 user content。官方 Anthropic 文档正要求 `tool_result` 必须在 user `content[]` 最前，普通 text 在所有结果之后。现有 same-protocol fixture 验证 `tool_result -> text -> raw block` 保留。
8. **流式工具事件的现有定向覆盖通过，但不能外推为所有 hosted/MCP 工具家族完整。** 本轮通过了 Chat tool-call index 聚合、Responses 已有 stream fixtures、Anthropic 并行/tool-use stream fixtures；未扩大到所有 `mcp_call`、`tool_search`、shell、computer、code-interpreter 等原生事件。

### #50829 exact class: fixed grouping

Responses input grouping is performed in `convertInputToMessages`:

- `llm/transformer/openai/responses/inbound.go:435-503` iterates ordered `input[]` items.
- `llm/transformer/openai/responses/inbound.go:475-491` detects a call item and invokes grouped conversion.
- `llm/transformer/openai/responses/inbound.go:505-529` consumes all adjacent `function_call` and `custom_tool_call` items into one canonical assistant `ToolCalls` slice; the first output ends the group.
- `llm/transformer/openai/responses/inbound_test.go:1751-1781` exercises four adjacent calls followed by four outputs and asserts exactly one assistant with four tool calls followed by four tool messages in matching order.

The exact focused test passed on 2026-07-16:

```text
cd llm
go test ./transformer/openai/responses \
  -run 'TestConvertInputToMessages_GroupsConsecutiveParallelFunctionCalls|TestG15b_ReasoningFollowingToolIdentity_PreservesToolIDs' \
  -count=1
ok github.com/looplj/axonhub/llm/transformer/openai/responses
```

### OpenAI Chat lifecycle

- `llm/transformer/openai/inbound_convert.go:48-204` maps `Request.Messages` with `lo.Map`, preserving supplied message order.
- `llm/transformer/openai/inbound_convert.go:207-281` maps each Chat message independently; `ToolCalls` are mapped in slice order and `ToolCallID` is copied directly.
- `llm/transformer/openai/outbound_convert.go:12-149` maps canonical messages independently in their existing order.
- `llm/transformer/openai/outbound_convert.go:175-262` maps all tool calls on one canonical assistant into one Chat assistant `tool_calls[]` in slice order.

This is order-preserving, not validating. No call-id pending/completed state is maintained across messages in these functions.

### OpenAI Responses lifecycle and remaining validator gap

Inbound Responses:

- `llm/transformer/openai/responses/inbound.go:696-738` maps function/custom call items to assistant calls.
- `llm/transformer/openai/responses/inbound.go:740-772` accepts output items whenever `Output != nil`, then directly maps `call_id` to a tool message. There is no preceding-call lookup.

Outbound Responses:

- `llm/transformer/openai/responses/outbound_convert.go:95-149` walks canonical messages in order and records only a `call_id -> output item type` map from assistant items.
- On a tool message, an unknown/missing ID falls back to `function_call_output`; there is no orphan, duplicate, incomplete-batch, or order check.
- `llm/transformer/openai/responses/outbound_convert.go:342-388` serializes the tool result and uses `lo.FromPtr(msg.ToolCallID)`, yielding an empty `call_id` for nil.
- `llm/transformer/openai/responses/outbound_convert_test.go:168-180` explicitly expects a tool message without `ToolCallID` to become `{type:"function_call_output", call_id:""}`.

Reasoning + tool order:

- `llm/transformer/openai/responses/inbound.go:534-632` merges one reasoning item with all following adjacent function/custom calls and optional assistant message until an output/other item ends the group.
- `llm/transformer/openai/responses/g15b_input_item_identity_test.go:109-190` verifies reasoning/call/output order and IDs for function and custom calls, but does not establish a global pending-call validator.

### Anthropic lifecycle

Inbound:

- `llm/transformer/anthropic/inbound_convert.go:258-330` converts each `tool_result` block into a canonical role=`tool` message, preserves `tool_use_id`, and associates the original message index.
- `llm/transformer/anthropic/inbound_convert.go:339-350` appends each `tool_use` block to the same canonical assistant `ToolCalls` slice.
- `llm/transformer/anthropic/inbound_convert.go:432-437` preserves the companion user content as a message linked by `MessageIndex`.

Outbound:

- `llm/transformer/anthropic/outbound_convert.go:529-588` is the request message lifecycle coordinator.
- `llm/transformer/anthropic/outbound_convert.go:591-651` scans all messages per assistant tool call, appends matching results in tool-call order, and merges related user content.
- `llm/transformer/anthropic/outbound_convert.go:655-711` groups consecutive standalone canonical tool messages into one Anthropic user message.
- `llm/transformer/anthropic/outbound_convert.go:714-756` appends companion user text/image/document/raw blocks after tool results.
- `llm/transformer/anthropic/outbound_convert.go:1153-1257` and `llm/transformer/anthropic/tool_blocks.go:140-163` preserve assistant content/tool-use block order when original Anthropic block indexes exist.

Fixtures:

- `llm/transformer/anthropic/testdata/llm-parallel_multiple_tool.request.json:7-38` has one assistant with two calls followed by two tool messages.
- `llm/transformer/anthropic/testdata/anthropic-parallel_multiple_tool.request.json:8-46` expects one assistant with two `tool_use` blocks and one user with two `tool_result` blocks.
- `llm/transformer/anthropic/a1_unknown_content_blocks_test.go:299-390` verifies `tool_result -> text -> raw block` same-protocol replay.

The focused Anthropic test passed:

```text
cd llm
go test ./transformer/anthropic -run 'TestOutboundTransformer_TransformRequest_WithTestData' -count=1
ok github.com/looplj/axonhub/llm/transformer/anthropic
```

### Stream-focused checks executed

All commands were run from the owning `llm/` module and passed:

```text
go test ./transformer/openai \
  -run 'TestAggregateStreamChunksNonZeroToolCallIndex|TestAggregateStreamChunks' -count=1

go test ./transformer/openai/responses \
  -run 'TestInboundTransformer_TransformStream_KeepsResponsesReasoningItemsSeparate|TestOutboundTransformer_StreamTransformation_WithTestData' -count=1

go test ./transformer/anthropic \
  -run 'TestInboundTransformer_StreamTransformation_WithTestData|TestAnthropicStreamRoundTrip_ServerToolUse' -count=1
```

Relevant stream paths:

- `llm/transformer/openai/aggregator.go:17-30,120+` aggregates Chat tool calls by per-choice tool-call index and sorts/finalizes by index.
- `llm/transformer/openai/responses/inbound_stream.go:627-806` initializes and updates Responses function/custom tool calls using output/item indices.
- `llm/transformer/anthropic/outbound_stream.go:175-316` maps `content_block_start` tool-use blocks and subsequent `input_json_delta` fragments to indexed canonical tool calls.
- `llm/transformer/anthropic/inbound_stream.go:160-183` closes an open tool block before transitioning block lifecycle.

## Minimal Reproduction

### A. #50829 fixed class

Input to Responses adapter:

```json
[
  {"type":"function_call","call_id":"A","name":"one","arguments":"{}"},
  {"type":"function_call","call_id":"B","name":"two","arguments":"{}"},
  {"type":"function_call_output","call_id":"A","output":"a"},
  {"type":"function_call_output","call_id":"B","output":"b"}
]
```

Current canonical/Chat shape:

```text
assistant tool_calls=[A,B]
tool tool_call_id=A
tool tool_call_id=B
```

The previous faulty shape was:

```text
assistant tool_calls=[A]
assistant tool_calls=[B]
tool A
tool B
```

### B. Confirmed remaining Responses validator gap

Smallest canonical input:

```json
{
  "messages": [
    {"role":"tool","content":"orphan result"}
  ]
}
```

Current `convertInputFromMessages` output includes:

```json
{
  "type":"function_call_output",
  "call_id":"",
  "output":"orphan result"
}
```

No prior call is required and no conversion error is raised. The existing unit test explicitly asserts this output.

Equivalent direct Responses input also passes structural conversion when output is non-null:

```json
{
  "input":[
    {"type":"function_call_output","call_id":"missing","output":"orphan"}
  ]
}
```

It becomes a canonical tool message referencing `missing`; no prior-call validation occurs.

### C. Confirmed Anthropic repair-by-reordering behavior

Canonical history:

```text
assistant tool_calls=[A,B]
user "interposed"
tool B
tool A
```

`findToolResultsForAssistant` scans the entire list in assistant call order and emits:

```text
assistant content=[tool_use A, tool_use B]
user content=[tool_result A, tool_result B]
user "interposed"
```

This obeys Anthropic adjacency but changes the original order instead of rejecting the malformed/interposed history.

## Files Found

- `llm/transformer/openai/inbound_convert.go` — Chat request/message ingress conversion; direct order-preserving map.
- `llm/transformer/openai/outbound_convert.go` — canonical-to-Chat request conversion; one canonical message maps to one Chat message.
- `llm/transformer/openai/aggregator.go` — Chat stream tool-call aggregation by index.
- `llm/transformer/openai/responses/inbound.go` — Responses input item conversion and the #50829 adjacent-call grouping repair.
- `llm/transformer/openai/responses/outbound_convert.go` — canonical-to-Responses item serialization and call-type lookup map.
- `llm/transformer/openai/responses/inbound_test.go` — regression for consecutive parallel Responses calls.
- `llm/transformer/openai/responses/g15b_input_item_identity_test.go` — reasoning/function/custom call/output identity and order regression.
- `llm/transformer/openai/responses/outbound_convert_test.go` — includes explicit empty `call_id` output expectation.
- `llm/transformer/openai/responses/inbound_stream.go` — Responses stream item/tool argument lifecycle.
- `llm/transformer/anthropic/inbound_convert.go` — Anthropic block-to-canonical tool call/result conversion.
- `llm/transformer/anthropic/outbound_convert.go` — Anthropic message grouping, global matching scan, and result ordering.
- `llm/transformer/anthropic/tool_blocks.go` — original block-index ordering support.
- `llm/transformer/anthropic/a1_unknown_content_blocks_test.go` — same-protocol tool-result + user-content ordering fixture.
- `llm/transformer/anthropic/testdata/*parallel*multiple_tool.request.json` — existing parallel grouping fixtures.

## External References

- OpenAI, “Function calling” (current page fetched 2026-07-16): https://developers.openai.com/api/docs/guides/function-calling
  - Defines the five-step lifecycle, requires outputs to reference a specific call ID, shows Chat appending the assistant tool-call message then one role=`tool` message per call, and shows Responses appending response output then matching `function_call_output` items.
  - States models may call multiple functions in a single turn and `parallel_tool_calls=false` limits this.
- Anthropic, “Handle tool calls” (current page fetched 2026-07-16): https://platform.claude.com/docs/en/agents-and-tools/tool-use/handle-tool-calls.md
  - Requires tool-result messages immediately after corresponding tool-use messages; in a user content array, all `tool_result` blocks must come first and text only after all results.
- OpenAI Codex issue #8107, “Tool result ordering is incorrect”: https://github.com/openai/codex/issues/8107
  - External example of the same Chat invariant violation: an assistant message inserted between a tool call and its output causes provider rejection.
- GitHub Copilot SDK issue #1922, “Chat Completions: image tool-result user messages interleaved between tool responses”: https://github.com/github/copilot-sdk/issues/1922
  - External example of parallel output grouping failure: `tool,user,tool,user` fails while `tool,tool,user,user` succeeds.

## Related Specs

- `.trellis/spec/backend/protocol-transformer-guidelines.md` — same-protocol first, preserve/diagnose, stream fidelity, Responses input item identity rules.
- `docs/specs/protocols/openai-chat-completions-protocol.md` — Chat request fields and parallel tool call control.
- `docs/specs/protocols/openai-responses-protocol.md` — ordered typed `input[]`, call/output identities, and semantic stream events.
- `docs/specs/protocols/anthropic-claude-messages-protocol.md` — content-block lifecycle (`tool_use` followed by later user `tool_result`).
- `docs/specs/protocols/protocol-conversion-strict-verification-matrix.md` — conversion evidence and partial tool-family coverage.
- `.agent/rules/spec-audit-method.md` — source assignment/return line evidence requirement and six-direction audit discipline.

## Caveats / Not Found

- The `#50829` database row was **not re-queried in this final read-only pass**. Its exact production request/error facts came from the same task/session memory; current source and regression tests independently confirm the described grouping defect class and repair.
- The public issue number `#50829` is local AxonHub request identity, not the unrelated public `anthropics/claude-code#50829` search result.
- No code, test, spec, deployment, service, database, or worktree file was modified or reverted. This research note is the sole persisted artifact required by the Trellis researcher contract.
- MCP worked for most source reads, then its transport closed. The required CLI fallback was attempted; that CLI instance did not contain the current `Users-asuan-AI-axonhub` index. Exact known files were then read directly with numbered, read-only output.
- Passing existing stream tests proves only those fixtures. It does not prove complete lifecycle coverage for every Responses hosted/MCP/server-tool family.
- The Anthropic global scan/reorder is a confirmed source behavior, but this pass did not establish a real valid-input production case where it causes failure; classify it as a risk/normalization behavior, not a confirmed incident.
