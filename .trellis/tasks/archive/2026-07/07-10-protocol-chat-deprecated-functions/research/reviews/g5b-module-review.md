# G5b Module Review — Deprecated Chat functions / function_call

- **结论**: FAIL
- **agent id**: g5b-module-review
- **commit**: `628e659d047e4cfb7fd1f0ec005efdce866141d3`
- **branch**: `codex-transformer-field-fixes`
- **审查时间**: 2026-07-12

## 审查范围

仅审查 commit `628e659d` 中与本任务相关的实现：

- `llm/transformer/openai/chat_n.go`
- `llm/transformer/openai/model.go`
- `llm/transformer/openai/inbound_convert.go`
- `llm/transformer/openai/chat_deprecated_functions_test.go`
- `llm/transformer/openai/testdata/openai-deprecated-*.json`
- `llm/transformer/anthropic/outbound.go`
- `llm/transformer/anthropic/outbound_test.go`
- `.trellis/tasks/07-10-protocol-chat-deprecated-functions/**`（design/prd/ledger）

对照设计意图：

1. request `functions` / `function_call`：same-protocol Chat raw preserve；不 widen `llm.Request`
2. legacy+modern 并存：modern `tools`/`tool_choice` 走 common；legacy raw 仍 preserve
3. response `message.function_call`：入站桥接到 modern `tool_calls`；`finish_reason=function_call` 时客户端回写 legacy shape
4. 跨协议：Responses no-synth；Anthropic LossyDowngrade
5. modern tools 路径不回归

## 结论摘要

Request 顶层 raw preserve、不 widen `llm.Request`、Responses no-synth、Anthropic lossy、modern non-stream 回归路径整体正确。

但 response bridge 被放在共享的 `Message.ToLLMMessage()` 上，导致：

1. **流式中间 delta** 无法回写 legacy `function_call`（只在 `finish_reason=function_call` 时回写）
2. **请求历史里的 assistant `message.function_call`** 被改写成 modern `tool_calls`（且无 id），破坏 same-protocol Chat 消息保真

这两项均被探针实测证实，且现有测试未覆盖，构成 false green。按判定规则：有 major => **FAIL**。

## 发现列表

### Blocker

无。

### Major

#### M1. 流式 `delta.function_call` 无法回写 legacy shape（协议正确性 / 代码 bug）

- **位置**: `llm/transformer/openai/inbound_convert.go` `ToLLMMessage` + `ChoiceFromLLM`
- **现象**:
  - 入站：stream chunk 的 `delta.function_call` 被桥成 `delta.tool_calls`
  - 出站：`ChoiceFromLLM` 仅当 `finish_reason == "function_call"` 时才把 `tool_calls` 回写成 `function_call`
  - 流式中间 chunk 通常 `finish_reason=null`，因此客户端收到 modern `tool_calls`，不是 legacy `function_call`
- **证据**（临时探针，已删除，未进入仓库）:
  - 输入 chunk: `delta.function_call={name,arguments partial}`, `finish_reason=null`
  - 客户端输出: `delta.tool_calls=[...]`，**没有** `function_call`
- **影响**:
  - 设计意图第 3 条“客户端回写 legacy shape”在 stream 路径不成立
  - 依赖 `delta.function_call` 的 legacy 客户端会断
- **可执行修复**:
  1. 不要只靠 `finish_reason` 判定 legacy 回写。
  2. 在 bridge 时写入可识别的 origin 标记，例如 `TransformerMetadata["openai.chat.function_call_origin"]=true`，或保留原始 `FunctionCall` sidecar。
  3. `ChoiceFromLLM` 对 message/delta：若 origin 为 legacy function_call，则始终 emit `function_call` 并清空 `tool_calls`；`finish_reason` 仅作辅助。
  4. 增加 stream fixture：
     - partial delta (`finish_reason=null`)
     - final chunk (`finish_reason=function_call`)
     - 断言中间与最终都是 `function_call`，且不出现 `tool_calls`

#### M2. 请求历史 assistant `message.function_call` 被 same-protocol 改写为 `tool_calls`（协议正确性 / 架构）

- **位置**:
  - bridge: `Message.ToLLMMessage()`（请求消息与响应消息共用）
  - reverse 缺失: `MessageFromLLMWithConfig()` 只会 emit `tool_calls`
- **现象**:
  - 客户端请求历史包含 assistant `function_call`
  - inbound 桥成 `llm.Message.ToolCalls`（`ID=""`）
  - Chat outbound 回放成 modern `tool_calls`，丢失 `function_call`
- **证据**（临时探针，已删除）:
  - outbound assistant message:
    `{"role":"assistant","tool_calls":[{"function":{"arguments":"{\"location\":\"NYC\"}","name":"get_weather"},"index":0,"type":"function"}]}`
  - **没有** `function_call`
- **影响**:
  - same-protocol capture/replay 对 `message.function_call` 字段族失真
  - 改写后的 modern shape 还缺 `id`，对要求 tool_call id 的上游更脆
  - bridge 放在共享 `ToLLMMessage`，把“response 入站桥接”副作用扩散到“request 历史消息”
- **可执行修复**（优先选 A）:
  - **A（推荐，最小且贴合框架）**:
    1. 从 `ToLLMMessage()` 移除 function_call bridge。
    2. 仅在 response 路径 bridge：`Choice.ToLLMChoice()` / outbound response convert。
    3. request 历史中的 `Message.FunctionCall` 走 raw/native preserve，或在 `MessageFromLLM` 原样回写 `FunctionCall`。
  - **B**:
    1. bridge 时写入 origin metadata。
    2. `MessageFromLLMWithConfig` 若 origin=legacy，则 emit `function_call` 而非 `tool_calls`。
  - 增加 multi-turn fixture：
    - messages 含 assistant `function_call` + role=`function` result
    - 断言 Chat->Chat outbound 仍保留 assistant `function_call`
    - 断言不出现无 id 的 `tool_calls`

### Minor

#### m1. 非流式 response 测试未断言 `tool_calls` 缺席（测试充分性 / false green 风险）

- **位置**: `TestOpenAIChatResponseDeprecatedMessageFunctionCallBridge`
- **问题**: 只断言客户端有 `function_call` 与 `finish_reason`，未断言 message **没有** `tool_calls`
- **补充**: 临时探针确认当前实现确实会清掉 `tool_calls`；属于测试缺口，不是当前功能 bug
- **修复**: 增加 `require.NotContains(message, "tool_calls")`

#### m2. legacy bridge 产生空 `ToolCall.ID`（协议语义边角）

- **位置**: `ToLLMMessage` bridge
- **问题**: legacy `function_call` 本无 id，桥成 modern `tool_calls` 后 `ID=""`
- **影响**: 对本 commit 的 Chat non-stream 客户端回写路径影响有限；但若 common lifecycle 被跨协议/工具结果配对消费，会变脆
- **修复**: 若继续 bridge 到 common tool lifecycle，显式文档“legacy 无 id”；跨协议消费前生成稳定 id，或保持仅 response-choice 内部使用

#### m3. legacy+modern 并存时双写 top-level 字段（设计取舍，需文档/测试钉死）

- **位置**: raw preserve + modern tools path
- **问题**: coexistence fixture 同时输出 `functions`/`function_call` 与 `tools`/`tool_choice`
- **影响**: 这是当前设计选择（raw preserve + modern common path），不是 silent rewrite；但真实 OpenAI 上游对双写兼容性未在本 commit 用契约测试钉死
- **修复**: 在 design/ledger 明确“双写是有意行为”；若产品要 precedence collapse，另开 slice，不要 silently drop

#### m4. Anthropic lossy 只覆盖 request raw 字段存在性（测试边界）

- **位置**: `DiagnosesChatDeprecatedFunctionsLoss`
- **问题**: 只证明 top-level `functions`/`function_call` 记 lossy；未覆盖 response bridge 后的 cross-protocol tool semantic
- **修复**: 可接受为本 slice 范围外；若后续做 cross-protocol function semantic，另补

## 已验证为正确的部分

1. **Request raw preserve**  
   `functions` / `function_call` 加入 `openAIChatRawPreserveFields`，经 `marshalOpenAIChatRequest` same-protocol re-emit；未 widen `llm.Request`。

2. **不污染 modern tools 抽象**  
   legacy-only request 不会被静默改写成 `llm.Request.Tools` / `ToolChoice`。

3. **legacy+modern coexistence（top-level）**  
   modern `tools`/`tool_choice` 仍走 common；legacy raw 仍保留。

4. **非流式 response round-trip（有限范围）**  
   upstream `message.function_call` + `finish_reason=function_call`：
   - inbound bridge 到 `tool_calls`
   - client re-emit `function_call`
   - 且当前实现会清掉 client `tool_calls`（探针确认）

5. **Responses no-synth**  
   `TestOpenAIChatRequestDeprecatedFunctionsNotSynthesizedForResponses` 覆盖 3 个 request fixtures。

6. **Anthropic LossyDowngrade**  
   `functions` / `function_call` 进入 lossy field list，测试断言两条 warning。

7. **modern tool path non-stream 不回归**  
   `TestOpenAIChatModernToolPathUnaffectedByDeprecatedBridge` 通过。

8. **框架契合度（部分）**  
   request 顶层采用 raw preserve、不 widen common model，符合 transformer guidelines。  
   但 response bridge 放进共享 `ToLLMMessage` 破坏了 request/response 边界，偏离 “response compatibility path” 的最小缝合。

## 已验证证据

### 阅读/对照

- `git show --stat 628e659d` / `git show 628e659d -- <scoped paths>`
- `llm/transformer/openai/chat_n.go`
- `llm/transformer/openai/model.go`
- `llm/transformer/openai/inbound_convert.go`（`ToLLMMessage`, `ChoiceFromLLM`）
- `llm/transformer/openai/outbound_convert.go`（`MessageFromLLMWithConfig`, `ToLLMChoice`）
- `llm/transformer/openai/outbound.go`（`marshalOpenAIChatRequest` 调用；stream chunk 复用 `TransformResponse`）
- `llm/transformer/openai/inbound.go`（stream 复用 `ResponseFromLLM`）
- `llm/transformer/openai/chat_deprecated_functions_test.go`
- fixtures:
  - `openai-deprecated-functions.request.json`
  - `openai-deprecated-function-call.request.json`
  - `openai-deprecated-and-modern-tools.request.json`
  - `openai-deprecated-message-function-call.response.json`
- `llm/transformer/anthropic/outbound.go` / `outbound_test.go`
- task docs: `prd.md`, `design.md`, `implement.md`, `research/g5b-slice-ledger.md`
- guidelines: `.trellis/spec/backend/protocol-transformer-guidelines.md`

### 运行测试

```bash
cd llm && go test ./transformer/openai -count=1 -run 'TestOpenAIChat(RequestDeprecated|ResponseDeprecated|ModernToolPathUnaffected)'
# ok

cd llm && go test ./transformer/anthropic -count=1 -run DiagnosesChatDeprecatedFunctionsLoss
# ok
```

### 额外探针（只读验证后已删除，未提交）

- `TestProbe_RequestHistoryFunctionCallRewritten` => FAIL（历史被改写成 tool_calls）
- `TestProbe_StreamDeltaBeforeFinishReason` => FAIL（中间 delta 泄露 tool_calls）
- `TestProbe_ClientResponseDropsToolCallsField` => PASS（non-stream final 清 tool_calls）

## 架构/屎山评估

- **不是大范围屎山**：改动面小，request 侧沿用既有 raw preserve 列表，方向正确。
- **主要架构问题**: 把 response-only bridge 塞进共享 `ToLLMMessage()`，导致 request history 与 stream delta 被同一逻辑误伤。
- **多余代码**: 无显著多余抽象；问题是 seam 放错层，不是代码膨胀。

## 测试充分性与 false green

| 设计要求 | 现有测试 | 结果 |
|---|---|---|
| legacy-only request replay | 有 | 覆盖 top-level |
| legacy-only response replay | 有，仅 non-stream final | 部分 |
| legacy+modern precedence | 有 top-level | 覆盖 |
| modern-only regression | 有 non-stream | 覆盖 |
| Responses no-synth | 有 | 覆盖 |
| Anthropic lossy | 有 | 覆盖 |
| stream delta legacy shape | **无** | **false green** |
| multi-turn history `message.function_call` | **无** | **false green** |
| client response 无 `tool_calls` | **无显式断言** | 弱 |

## 若不通过：最小修复建议（可执行）

按优先级：

1. **拆 seam（修 M2 根因）**
   - 从 `ToLLMMessage()` 删除 function_call -> tool_calls bridge
   - 仅在 `Choice.ToLLMChoice()`（outbound response -> llm）做 bridge
   - request messages 保留/回写 native `FunctionCall`

2. **修 stream 回写（修 M1）**
   - bridge 时打 origin metadata，或在 llm choice/message 保留 legacy 标记
   - `ChoiceFromLLM` 基于 origin（而不是只看 finish_reason）回写 `function_call` 并清空 `tool_calls`

3. **补测试**
   - stream partial + final fixtures
   - multi-turn history request fixture
   - non-stream client response 断言 `tool_calls` 不存在
   - 保持现有 modern regression / Responses no-synth / Anthropic lossy

4. **回归命令**
   ```bash
   cd llm && go test ./transformer/openai -count=1 -run 'TestOpenAIChat(RequestDeprecated|ResponseDeprecated|ModernToolPathUnaffected)|History|Stream'
   cd llm && go test ./transformer/anthropic -count=1 -run DiagnosesChatDeprecatedFunctionsLoss
   ```

## 最终判定

- blocker: 0
- major: 2（M1 stream 回写缺失；M2 请求历史 same-protocol 改写）
- minor: 4

**FAIL** — 需要先修 M1/M2 并补对应测试后再进 module review gate。
