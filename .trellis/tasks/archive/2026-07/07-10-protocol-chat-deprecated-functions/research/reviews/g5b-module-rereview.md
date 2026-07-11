# G5b Module Re-Review — Deprecated Chat functions / function_call

- **结论**: PASS
- **agent id**: g5b-module-rereview
- **commits**:
  - `628e659d047e4cfb7fd1f0ec005efdce866141d3` — fix(openai): preserve deprecated Chat functions/function_call
  - `97686bd6e753d7fdffe0b6b7e73f318de8db11c3` — fix(openai): restore legacy function_call for stream and history
- **branch**: `codex-transformer-field-fixes`
- **审查时间**: 2026-07-12
- **对照上次 FAIL**: `.trellis/tasks/07-10-protocol-chat-deprecated-functions/research/reviews/g5b-module-review.md`

## 审查范围

复审两笔提交中与 G5b 相关的实现与测试，重点验证上次 major M1/M2 是否关闭，以及既有路径是否回归。

代码：

- `llm/transformer/openai/model.go` — `TransformerMetadataKeyDeprecatedFunctionCallOrigin`
- `llm/transformer/openai/inbound_convert.go` — bridge + origin + stream/client re-emit
- `llm/transformer/openai/outbound_convert.go` — history re-emit
- `llm/transformer/openai/chat_n.go` — top-level raw preserve
- `llm/transformer/openai/chat_deprecated_functions_test.go`
- fixtures:
  - `openai-deprecated-message-function-call.stream.jsonl`
  - `openai-deprecated-message-function-call.history.request.json`
  - 既有 request/response fixtures
- `llm/transformer/anthropic/outbound.go` / `outbound_test.go`

## 结论摘要

上次 FAIL 的两个 major 已由第二笔提交按 **origin metadata** 方案关闭：

1. bridge 时写入 `openai.chat.function_call_origin=true`
2. stream delta / final / history / non-stream client 均按 origin（或 `finish_reason=function_call` 兜底）回写 `function_call` 并抑制 `tool_calls`
3. 新增 stream + history fixtures，覆盖此前 false-green 路径
4. request top-level raw preserve、Responses no-synth、Anthropic lossy、modern tool path 仍绿

无 blocker / major。架构上仍把 bridge 放在共享 `ToLLMMessage()`（上次推荐 A 为 response-only seam），但 reverse path 已补齐，same-protocol 行为正确；记为 residual minor，不挡 gate。

## M1 / M2 关闭判定

| 上次问题 | 状态 | 判定依据 |
|---|---|---|
| **M1** stream delta `function_call` 无法回写，中间 chunk 泄露 `tool_calls` | **关闭** | origin 不依赖 `finish_reason`；`ChoiceFromLLM` → `messageFromLLMPreservingDeprecatedFunctionCall`；stream fixture 覆盖 partial×2 + final |
| **M2** multi-turn history assistant `function_call` 被改写成 `tool_calls` | **关闭** | `MessageFromLLMWithConfig` 对 origin tool_calls emit `function_call` 且不写 `tool_calls`；history fixture 断言 |

### M1 证据

实现：

```go
// ToLLMMessage bridge
TransformerMetadata: map[string]any{
  TransformerMetadataKeyDeprecatedFunctionCallOrigin: true,
}

// shouldEmitDeprecatedFunctionCall
if hasDeprecatedFunctionCallOrigin(toolCalls) { return true }
return finishReason != nil && *finishReason == "function_call" && len(toolCalls) > 0

// ChoiceFromLLM message/delta 均走 messageFromLLMPreservingDeprecatedFunctionCall
```

stream fixture 三行：

1. `delta.function_call.name=get_weather`, `finish_reason=null`
2. `delta.function_call.arguments=...`, `finish_reason=null`
3. `delta={}`, `finish_reason=function_call`

测试 `TestOpenAIChatStreamDeprecatedMessageFunctionCallRoundTrip`：

- chunk 0/1：有 `function_call`，无 `tool_calls`，`finish_reason=null`
- chunk 2：`finish_reason=function_call`，无 `tool_calls`
- 路径：`Outbound.TransformResponse` → `Inbound.TransformStreamChunk`（复用 `ResponseFromLLM`/`ChoiceFromLLM`）

### M2 证据

实现：

```go
// MessageFromLLMWithConfig
if shouldEmitDeprecatedFunctionCall(m.ToolCalls, nil) && len(m.ToolCalls) > 0 {
  msg.FunctionCall = &FunctionCall{...}
} else if m.ToolCalls != nil {
  msg.ToolCalls = ...
}
```

history fixture：assistant message 仅有 `function_call`，无 modern `tool_calls`。

测试 `TestOpenAIChatRequestHistoryDeprecatedMessageFunctionCallRoundTrip`：

- inbound 后 `ToolCalls` 长度 1 且 `hasDeprecatedFunctionCallOrigin`
- outbound assistant 有 `function_call`（name/arguments 正确）
- outbound assistant **无** `tool_calls`

## 必须验证清单

| # | 项 | 结果 | 证据 |
|---|---|---|---|
| 1 | origin metadata 正确设置 | PASS | `ToLLMMessage` 写入 `TransformerMetadataKeyDeprecatedFunctionCallOrigin=true`；history 测试断言 `hasDeprecatedFunctionCallOrigin` |
| 2 | stream partial/final re-emit `function_call` 且不暴露 `tool_calls` | PASS | stream fixture + `TestOpenAIChatStreamDeprecatedMessageFunctionCallRoundTrip` |
| 3 | history assistant `function_call` same-protocol 保真 | PASS | history fixture + `TestOpenAIChatRequestHistoryDeprecatedMessageFunctionCallRoundTrip` |
| 4 | modern tool path 不回归 | PASS | `TestOpenAIChatModernToolPathUnaffectedByDeprecatedBridge`：不 invent `function_call`，保留 `tool_calls` |
| 5 | request top-level `functions`/`function_call` raw preserve | PASS | `openAIChatRawPreserveFields` 含两字段；RawRoundTrip / Precedence 测试 |
| 6 | Responses no-synth / Anthropic lossy | PASS | `TestOpenAIChatRequestDeprecatedFunctionsNotSynthesizedForResponses`；Anthropic 列表含两字段 + Diagnoses 测试 |
| 7 | 上次 major 关闭、无新 major | PASS | M1/M2 关闭；未见新协议正确性 major |

## 发现列表

### Blocker

无。

### Major

无。

### Minor

#### m1. bridge 仍在共享 `ToLLMMessage`（架构 residual）

- **位置**: `inbound_convert.go` `ToLLMMessage`
- **说明**: 上次推荐 A 是 response-only bridge；当前采用推荐 B（origin metadata + reverse emit）。行为正确，但 request history 仍经 “bridge 再还原”，多一次内部形状变换。
- **影响**: 维护成本略高；不构成协议错误。
- **建议**: 可后续把 bridge 收窄到 response/`Choice` 路径；非本 gate 阻塞项。

#### m2. origin 仅标在 bridged tool_call 上，不在 message 级

- **说明**: 多 tool_calls 混合 origin 时，`shouldEmit` 只要任一 origin=true 就会把 **first** tool_call 回写成单一 `function_call`。
- **影响**: 对纯 legacy `function_call`（单对象）无问题；若未来出现 mixed 输入，语义需再明确。
- **当前任务**: legacy shape 本身是单 `function_call`，可接受。

#### m3. stream final chunk 不强制断言 delta 为空

- fixture final 的 `delta={}`；测试只断言 `finish_reason` 与无 `tool_calls`。
- 足够覆盖 M1；若要加强可再断言 final 无残留 `function_call`/`tool_calls` 字段。

#### m4. 空 name+arguments 的 `function_call` 不 bridge

- 条件：`Name != "" || Arguments != ""`。
- 合理；仅作边界记录。

## 已验证证据

### 阅读/对照

- `git show --stat 628e659d 97686bd6`
- `git show 97686bd6` 对 `inbound_convert.go` / `outbound_convert.go` / `model.go` / tests / fixtures
- 当前源码：
  - origin key / bridge / `hasDeprecatedFunctionCallOrigin` / `shouldEmitDeprecatedFunctionCall` / `messageFromLLMPreservingDeprecatedFunctionCall` / `ChoiceFromLLM`
  - `MessageFromLLMWithConfig` history reverse
  - `openAIChatRawPreserveFields`
  - Anthropic lossy field list
- stream/history fixtures 内容
- 上次 FAIL 报告 M1/M2 修复建议对照

### 运行测试

```bash
cd llm && go test ./transformer/openai -count=1 -run 'TestOpenAIChat(RequestDeprecated|ResponseDeprecated|ModernToolPathUnaffected|RequestHistoryDeprecated|StreamDeprecated)'
# ok  — 全部 PASS，含：
#   RequestDeprecatedFunctionsRawRoundTrip
#   RequestDeprecatedFunctionCallRawRoundTrip
#   RequestDeprecatedAndModernToolsPrecedence
#   RequestDeprecatedFunctionsNotSynthesizedForResponses
#   ResponseDeprecatedMessageFunctionCallBridge
#   ModernToolPathUnaffectedByDeprecatedBridge
#   RequestHistoryDeprecatedMessageFunctionCallRoundTrip
#   StreamDeprecatedMessageFunctionCallRoundTrip
#   ResponseDeprecatedMessageFunctionCallDropsModernToolCalls

cd llm && go test ./transformer/anthropic -count=1 -run DiagnosesChatDeprecatedFunctionsLoss
# ok
```

## 架构评估

- 第二笔提交用最小 reverse-path 修复关闭 M1/M2，未扩大 common model。
- request top-level 继续 raw preserve，不 widen `llm.Request`，符合 transformer guidelines。
- residual：共享 `ToLLMMessage` bridge 仍在；有 origin 后可接受。
- 无多余抽象或屎山扩张。

## 测试充分性

| 设计要求 | 测试 | 结果 |
|---|---|---|
| legacy-only request replay | 有 | PASS |
| legacy-only response replay | 有（含 drops tool_calls） | PASS |
| legacy+modern top-level precedence | 有 | PASS |
| modern-only regression | 有 non-stream | PASS |
| Responses no-synth | 有 | PASS |
| Anthropic lossy | 有 | PASS |
| stream partial + final legacy shape | **已补** | PASS |
| multi-turn history `message.function_call` | **已补** | PASS |
| client response 无 `tool_calls` | **已补** | PASS |

上次 false-green 路径已有直接 fixture/断言，不再仅靠 non-stream final 间接覆盖。

## 最终判定

- blocker: 0
- major: 0
- minor: 4（均为 residual/边界，不挡 gate）
- M1: **关闭**
- M2: **关闭**

**PASS** — G5b module re-review gate 可通过。
