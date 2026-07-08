# G10 raw JSON helper seam 审查（xhigh）

## 结论

**PASS（代码层面）**。

`CloneRawMessage` / `CloneRawMessageMap` 收敛到 `llm/internal/pkg/xjson` 后，与本次替换掉的旧 helper 行为一致；依赖方向合理；未发现协议 raw preservation 行为变化、死代码或编译隐患。

**提交门禁：当前不能只提交 tracked diff。** 两个新文件仍是 untracked，必须一起纳入提交：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/internal/pkg/xjson/raw.go`
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/internal/pkg/xjson/raw_test.go`

若提交清单包含这两个文件：**可以提交**。若遗漏任一文件：**不可提交**。

## 审查范围与验证

审查仓库：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`

当前未提交变更：

- tracked 修改：
  - `llm/provider_extensions.go`
  - `llm/transformer/anthropic/request_extensions.go`
  - `llm/transformer/anthropic/response_extensions.go`
  - `llm/transformer/openai/chat_extensions.go`
  - `llm/transformer/openai/responses/request_extensions.go`
  - `llm/transformer/openai/responses/search_metadata.go`
- untracked 新增：
  - `llm/internal/pkg/xjson/raw.go`
  - `llm/internal/pkg/xjson/raw_test.go`

已执行的只读/验证动作：

- `git status --short --branch`
- `git diff --stat` / `git diff --name-status`
- `git diff --check`：通过
- 精确旧 helper 名称搜索：未发现残留旧单值/map helper 调用点
- 目标测试：

```text
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm && \
go test ./internal/pkg/xjson ./transformer/openai/responses ./transformer/openai ./transformer/anthropic
```

结果：全部通过。

未运行 lint / build / 全量测试，符合仓库规则“未被显式要求不运行 lint/build”。

## 关键证据

### 1. 新 helper 与旧 helper 行为一致

新实现位于 `llm/internal/pkg/xjson/raw.go`：

- `CloneRawMessage(src json.RawMessage)`：`len(src)==0` 返回 `nil`；非空时 `append(json.RawMessage(nil), src...)` 深拷贝。
- `CloneRawMessageMap(src map[string]json.RawMessage)`：`len(src)==0` 返回 `nil`；非空时 `make(map[string]json.RawMessage, len(src))` 新建 map，并逐项调用 `CloneRawMessage`。

旧 helper 行为对照：

- `llm/provider_extensions.go` 旧 `cloneRawMessage` / `cloneRawMessageMap`：同样 `len==0 -> nil`、`append(json.RawMessage(nil), src...)`、map 逐项 clone。
- `llm/transformer/anthropic/request_extensions.go` 旧 `cloneAnthropicRawMessage` / `cloneAnthropicRawMessageMap`：同样行为。
- `llm/transformer/openai/chat_extensions.go` 旧 `cloneOpenAIChatRawMessage`：同样行为。
- `llm/transformer/openai/responses/request_extensions.go` 旧 `cloneRaw`：同样行为。
- `llm/transformer/openai/responses/search_metadata.go` 旧 `cloneRawMessage`：同样行为。

新测试 `llm/internal/pkg/xjson/raw_test.go` 覆盖：

- `nil` 输入返回 `nil`；
- 空 `json.RawMessage{}` 返回 `nil`；
- 非空 raw bytes 不共享 backing array；
- 源 raw bytes 被修改后 clone 不变；
- map 返回值不为原 map，且 value raw message 深拷贝。

结论：必查项 1 **通过**。

### 2. 放在 `llm/internal/pkg/xjson` 的依赖方向合理

证据：

- `llm/internal/pkg/xjson` 是 llm 模块既有内部 JSON 工具包，已有 `constants.go`、`json.go`、`safe.go`、`schema.go`。
- 现有 llm transformer 已大量导入 `github.com/looplj/axonhub/llm/internal/pkg/xjson`，例如 Anthropic inbound/outbound、OpenAI inbound、Responses model/inbound 等。
- `llm/internal/pkg/xjson` 自身没有导入 `github.com/looplj/axonhub/llm` 或 transformer 包；本次 helper 只依赖标准库 `encoding/json`。
- `llm/provider_extensions.go` 属于 `github.com/looplj/axonhub/llm`，按 Go `internal` 规则允许导入其子树 `llm/internal/pkg/xjson`。

结论：这是低层 JSON byte-copy 工具，不反向依赖协议层，不把协议语义塞进通用包；必查项 2 **通过**。

### 3. 旧 helper 调用点、死代码、编译隐患

证据：

- 精确搜索旧 helper 名称后，只剩 `llm/transformer/openai/responses/websocket_executor.go` 的 `cloneRawMessages`。
- `cloneRawMessages` 是 `[]json.RawMessage` slice clone helper，用于 WebSocket session input 缓存，不是本次 `json.RawMessage` / `map[string]json.RawMessage` helper seam 的旧残留。
- 本次删除的旧 helper 名称未再被调用：
  - `cloneRawMessage`
  - `cloneRawMessageMap`
  - `cloneRaw`
  - `cloneAnthropicRawMessage`
  - `cloneAnthropicRawMessageMap`
  - `cloneOpenAIChatRawMessage`
- `git diff --check` 通过。
- 目标测试通过：`./internal/pkg/xjson`、`./transformer/openai/responses`、`./transformer/openai`、`./transformer/anthropic`。

唯一提交风险不是代码本身，而是 Git 状态：`raw.go` / `raw_test.go` 当前仍为 untracked。若漏加 `raw.go`，已修改文件中的 `xjson.CloneRawMessage` / `xjson.CloneRawMessageMap` 会产生编译错误；若漏加 `raw_test.go`，helper 行为测试缺口会回归。

结论：代码层面必查项 3 **通过**；提交清单必须修正 untracked 状态。

### 4. ProviderExtensions / Responses / Chat / Anthropic raw preservation 行为

本次 diff 是 helper callsite 替换，不改变字段清单、capture/replay 条件或 merge 顺序。

#### ProviderExtensions

`llm/provider_extensions.go` 中 `CloneProviderExtensions` 仍对以下 sidecar 做深拷贝：

- OpenAI Responses request：`RawTools`、`ToolSignatures`、`RawToolChoice`、`RawInputItems`、`RawPrompt`、`RawConversation`、`RawContextManagement`、`RawBackground`、`RawInclude`、`RawMaxToolCalls`、`RawPromptCacheRetention`、`RawTruncation`、`RawStreamOptions`、`RawTopLevelFields`。
- OpenAI Responses response：`RawTopLevelFields`、`RawOutputItems`。
- OpenAI Chat request：`RawTopLevelFields`。
- Anthropic request/response：`RawTopLevelFields`。
- Diagnostics：`LossyDowngrades` slice append clone 保持不变。

字段所有权注释与 `json:"-"` 保持不变，没有把 provider extension 序列化进 common JSON。

#### OpenAI Responses

`llm/transformer/openai/responses/request_extensions.go` 中：

- `attachOpenAIResponsesRequestExtensions` 仍保存 `RawPrompt` / `RawConversation` / `RawContextManagement` / owned top-level raw fields / unknown top-level fields。
- `rawTopLevelFields` 仍把已 owned fields 删除后，将剩余字段 clone 进 unknown `RawTopLevelFields`。
- `marshalRequestPayload` / `mergeRawNativeTopLevelField` / `mergeRawTopLevelFields` / `mergeOpenAIResponsesStreamOptions` / raw input/tool merge 仍只在不存在结构化字段时 replay raw，未改变覆盖策略。
- `search_metadata.go` 中 web/file search metadata 的 raw bytes clone 改为共用 helper，字段来源和迁移路径不变。

目标测试 `./transformer/openai/responses` 通过。

#### OpenAI Chat

`llm/transformer/openai/chat_extensions.go` 中：

- `rawOpenAIResponsesPromptCacheRetention`、`applyOpenAIChatRequestExtensions`、`mergeOpenAIChatTools`、`appendRawField` 只替换 clone helper。
- Chat raw replay field list、explicit null common fields、stream_options merge、tool union merge 条件均未改变。

目标测试 `./transformer/openai` 通过。

#### Anthropic

`llm/transformer/anthropic/request_extensions.go` / `response_extensions.go` 中：

- request replay 的 `container`、`inference_geo`、`mcp_servers`、raw tools clone 行为保持。
- response replay 的 `RawTopLevelFields`、`container`、`stop_details` clone 行为保持。
- `top_k` / `service_tier` / `stop_sequence` 仍按原逻辑 unmarshal，不受 helper 抽取影响。

目标测试 `./transformer/anthropic` 通过。

结论：必查项 4 **通过**。

### 5. 抽象程度、代码气味、注释、测试缺口

未发现阻塞问题。

判断：

- 抽象只包含两个低层 JSON byte-copy 函数，被 ProviderExtensions、OpenAI Responses、OpenAI Chat、Anthropic 多处使用，收敛重复逻辑合理，不是 speculative generality。
- helper 名称直接表达行为；注释“Empty input is normalized to nil to match existing transformer sidecar clone helpers”与旧 helper 实现一致，不误导。
- 没有把协议字段所有权或 replay 策略下沉到 `xjson`，所以没有形成“通用 JSON 包懂协议”的反向耦合。
- 新测试覆盖核心行为。唯一非阻塞建议是补一条 map 结构级别断言，例如修改/删除源 map entry 后确认 cloned map 不变；当前实现已通过 `make(map...)` 明确满足，只是测试可读性可再加强。

结论：必查项 5 **通过**。

### 6. untracked 新文件必须纳入提交

当前 Git 状态显示：

```text
?? llm/internal/pkg/xjson/raw.go
?? llm/internal/pkg/xjson/raw_test.go
```

这两个文件必须纳入提交。理由：

- `raw.go` 定义本次所有 callsite 依赖的 `xjson.CloneRawMessage` / `xjson.CloneRawMessageMap`。
- `raw_test.go` 是 helper 行为等价性的直接回归测试。

结论：必查项 6 **通过确认**，但这是提交前必须执行的清单动作。

## must-fix

1. **提交时必须包含**：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/internal/pkg/xjson/raw.go`。
2. **提交时必须包含**：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/internal/pkg/xjson/raw_test.go`。

除上述提交清单问题外，未发现必须修改的代码问题。

## 建议项

1. 可选增强 `TestCloneRawMessageMap`：增加源 map entry 被改名/删除后 cloned map 不变的断言，让“map deep copy”测试更直观。
2. 可选后续清理：如果未来还出现多个 `[]json.RawMessage` clone helper，再考虑新增 `CloneRawMessages`。当前唯一剩余 `cloneRawMessages` 位于 WebSocket executor，作用域明确，本次不建议顺手扩大抽象。

## 是否可提交

**可以提交，但前提是提交清单包含两个 untracked 新文件 `raw.go` 和 `raw_test.go`。**

如果只提交当前 tracked 修改而遗漏两个新文件，则不可提交。
