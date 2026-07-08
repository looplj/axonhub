# final comprehensive MCP review xhigh

## 结论

**PASS**

未发现需要阻断合入的 must-fix。分支可以进入 finish-work；不需要继续切片修复。

## 审查对象

- worktree: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
- branch: `codex/responses-top-level-preservation-clean`
- base: `origin/unstable = 97c9351a23df5a3c302cf1c35bf5ca39caf7208f`
- HEAD: `c62c111fa62b38818433713efd5072225a8a790b`
- diff: `origin/unstable...HEAD`
- 范围：`llm/` 协议转换修复分支，覆盖 OpenAI Responses、OpenAI Chat、Anthropic、stream/raw preservation、LossyDowngrade、ProviderExtensions、xjson raw helper、MarshalJSON promoted method 相关风险。

## 只读验证结果

已执行并通过：

```text
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm && go test ./... -count=1
=> PASS
```

```text
git -C /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean diff --check
=> PASS，无输出
```

```text
git -C /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean status --short
=> clean，无输出
```

同时核实：

- 当前分支确为 `codex/responses-top-level-preservation-clean`。
- `origin/unstable` 解析为 `97c9351a23df5a3c302cf1c35bf5ca39caf7208f`。
- `HEAD` 解析为 `c62c111fa62b38818433713efd5072225a8a790b`。
- `origin/unstable..HEAD` 共 25 个提交，diff 仅涉及 `llm/` 下 46 个文件。

## MCP / graph evidence

已成功使用 codebase-memory MCP；不是 FAIL-REVIEW-TOOLING。

### 图谱新鲜度

- `mcp__codebase_memory_mcp.index_status`
  - project: `Users-asuan-AI-axonhub-worktrees-responses-top-level-preservation-clean`
  - 初始状态：ready，`39874` nodes / `203608` edges。
- `mcp__codebase_memory_mcp.index_repository`
  - repo: `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
  - mode: `fast`
  - 重新索引后：`39883` nodes / `203715` edges。
- `mcp__codebase_memory_mcp.get_architecture`
  - 确认主要变更仍落在既有 `llm` transformer / pipeline 架构簇内，没有新建跨层服务或旁路系统。

### 架构边界证据

- `get_code_snippet`: `llm.model.Request`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/model.go:40-285`
  - `TransformerMetadata` 注释明确限制为 bridge/staging；协议 native body fields 不应放这里。
  - `ProviderExtensions *ProviderExtensions json:"-"` 明确排除普通 JSON 输出。
- `get_code_snippet`: `llm.model.Response`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/model.go:624-713`
  - response 同样通过 `ProviderExtensions json:"-"` 承载 provider/API-format private data。
- `get_code_snippet`: `llm.provider_extensions.ProviderExtensions`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/provider_extensions.go:11-16`
  - `OpenAIResponses` / `OpenAIChat` / `Anthropic` / `Diagnostics` 均为 `json:"-"`。
- `get_code_snippet`: `llm.provider_extensions.DiagnosticsProviderExtensions`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/provider_extensions.go:18-23`
  - LossyDowngrades 被明确标为 diagnostic，不是 native field storage。

结论：没有发现把协议字段塞进错误层、TransformerMetadata 垃圾桶、万能 AST 或隐藏可序列化 metadata 的架构错误。

### raw helper / map alias 证据

- `get_code_snippet`: `xjson.CloneRawMessage`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/internal/pkg/xjson/raw.go:7-13`
- `get_code_snippet`: `xjson.CloneRawMessageMap`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/internal/pkg/xjson/raw.go:17-28`
- `get_code_snippet`: `xjson.CaptureRawTopLevelFields`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/internal/pkg/xjson/raw.go:32-54`
  - 只捕获指定 top-level 字段，并 clone `json.RawMessage`。
- `get_code_snippet`: `llm.provider_extensions.CloneProviderExtensions`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/provider_extensions.go:208-272`
  - 对 raw fragments、raw maps、raw messages、diagnostics slice 均做拷贝，未发现 map alias / shared RawMessage mutation 风险。

### OpenAI Chat native/raw preservation 证据

- `get_code_snippet`: `openai.inbound.TransformRequest`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/inbound.go:27-75`
  - JSON unmarshal 后调用 `captureOpenAIChatRequestRawTopLevelFields`，再 `ToLLMRequest`。
- `get_code_snippet`: `openai.chat_extensions.preserveOpenAIChatRequestExtensions`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/chat_extensions.go:45-70`
  - raw Chat 字段进入 `ProviderExtensions.OpenAIChat.Request.RawTopLevelFields`。
- `get_code_snippet`: `openai.outbound.TransformRequest`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/outbound.go:142-233`
  - 仅 `PlatformOpenAI` 路径调用 `applyOpenAIChatRequestExtensions`，避免 OpenAI-compatible provider blast radius。
- `get_code_snippet`: `marshalOpenAIChatRequest`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/chat_extensions.go:129-153`
  - 使用 `type requestAlias Request` 后 marshal，规避 promoted `MarshalJSON` 干扰。
- `get_code_snippet`: `mergeOpenAIChatRawTopLevelFields`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/chat_extensions.go:155-172`
  - raw-only 字段、explicit null common fields、stream_options、tool union 分开合并。

结论：OpenAI Chat raw/native same-protocol replay 边界清楚；typed common fields 不会被 non-null raw 覆盖。

### OpenAI Responses request preservation 证据

- `get_code_snippet`: `responses.inbound.TransformRequest`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/inbound.go:38-64`
  - 解析后进入 `convertToLLMRequest(&req, httpReq.Body)`，保留原始 body 用于 raw capture。
- `get_code_snippet`: `responses.inbound.convertToLLMRequest`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/inbound.go:169-285`
  - typed 字段进入 common `llm.Request`；随后调用 `attachOpenAIResponsesRequestExtensions`。
- `get_code_snippet`: `responses.request_extensions.attachOpenAIResponsesRequestExtensions`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/request_extensions.go:12-44`
  - raw tools、tool_choice、input items、prompt/conversation/context_management、owned top-level fields、unknown top-level fields进入 `ProviderExtensions.OpenAIResponses.Request`。
- `get_code_snippet`: `responses.request_extensions.rawTopLevelFields`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/request_extensions.go:147-177`
  - 先保存 `openAIResponsesRawPreservedRequestFields`，再删除 owned request JSON fields，剩余 unknown fields 才作为 same-protocol raw top-level。
- `get_code_snippet`: `responses.request_extensions.marshalRequestPayload`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/responses/request_extensions.go:430-478`
  - same-protocol outbound 时合并 raw native top-level、stream_options、unknown top-level、raw-only tools/input/tool_choice。

结论：Responses request raw preservation 放在 ProviderExtensions sidecar，非 TransformerMetadata；unknown top-level 与 typed/native owned fields 分层处理。

### Anthropic native/raw preservation 证据

- `search_graph`: `captureAnthropicRequestRawTopLevelFields`, `preserveAnthropicRequestExtensions`, `applyAnthropicRequestExtensions`, `shouldReplayAnthropicRequestExtensions`
  - 文件：`llm/transformer/anthropic/request_extensions.go`
  - 图谱显示 request capture/preserve/apply/replay guard 已形成闭环。
- `search_graph`: `captureAnthropicResponseRawTopLevelFields`, `preserveAnthropicResponseExtensions`, `applyAnthropicResponseExtensions`, `applyAnthropicRawStopSequence`
  - 文件：`llm/transformer/anthropic/response_extensions.go`
  - 图谱显示 response raw top-level 与 stop_sequence raw/null preservation 已形成闭环。
- `get_code_snippet`: `llm.provider_extensions.AnthropicRequestExtensions`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/provider_extensions.go:55-60`
  - 注释明确 only replay to Anthropic direct same-protocol targets。

结论：Anthropic direct-only replay 边界明确；未发现跨协议静默重放 raw 字段。

### LossyDowngrade diagnostics 证据

- `get_code_snippet`: `llm.lossy_downgrade.LossyDowngrade`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/lossy_downgrade.go:11-17`
- `get_code_snippet`: `llm.lossy_downgrade.AddLossyDowngrade`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/lossy_downgrade.go:31-48`
  - nil/empty guard + duplicate exact diagnostic suppression。
- `get_code_snippet`: `llm.lossy_downgrade.AddLossyDowngradeIfPresent`
  - `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/lossy_downgrade.go:53-65`
- `trace_path`: `AddLossyDowngradeIfPresent`, inbound callers depth 4
  - 覆盖 OpenAI Chat -> Responses/Anthropic、Responses -> Chat/Anthropic、Anthropic -> Responses/Chat 的 downgrade recorder 路径。

结论：diagnostic 存储边界正确；未发现 diagnostic 漏入 provider JSON。

### MarshalJSON promoted method / provider blast radius 证据

- `query_graph`: `MarshalJSON` in `llm/`
  - 识别到 `llm/transformer/openai/model.go`, `llm/transformer/openai/responses/model.go`, `llm/transformer/anthropic/model.go`, `llm/transformer/longcat/model.go`, `llm/model.go` 等 MarshalJSON 节点。
- `get_code_snippet`: `openai.chat_extensions.marshalOpenAIChatRequest`
  - 使用 alias marshal，避免 Request promoted method 影响。
- 测试结果：`go test ./... -count=1` 在 `llm/` 全量通过，包括 `deepseek`, `longcat`, `zai`, `openai`, `openai/responses`, `anthropic` 包。

结论：未发现本分支引入的 Request marshal promoted method 回归；deepseek/longcat/zai 等嵌入结构的现有测试未回归。

## 代码 / 架构 / bug 发现

### Must-fix

无。

### 非阻断观察

1. **ProviderExtensions 变大，但职责仍单一**  
   `CloneProviderExtensions` 现在需要显式 clone 多类 raw sidecar 字段，函数较长；但它仍是集中复制边界，不属于 must-fix。当前实现比 map alias 更安全。

2. **Responses request raw replay 逻辑复杂度偏高**  
   `marshalRequestPayload` 同时处理 raw native top-level、unknown top-level、stream_options、tools、tool_choice、input items。测试覆盖较多，当前未发现 bug；后续可考虑只在有新协议字段时补充表驱动 case，不建议现在重构。

3. **LossyDowngrade 是 warning-only 诊断**  
   这符合当前分支目标，但它不会阻止丢字段，只记录跨协议无等价语义。调用方如果未来需要 UI/日志暴露，需要另开 slice，不属于当前 must-fix。

## 回归风险评估

- **OpenAI-compatible shared builder provider blast radius**：低。OpenAI Chat raw replay 只在 `PlatformOpenAI` 分支启用；Google 等兼容 provider 不会收到 Chat native raw fields。
- **Request MarshalJSON promoted method**：低。OpenAI Chat request marshal 使用 alias；`llm/` 全量测试覆盖 deepseek/longcat/zai 未回归。
- **ProviderExtensions json:"-" 泄漏**：低。Request/Response 与 ProviderExtensions 节点均显示 `json:"-"`。
- **Anthropic direct-only replay**：低。ProviderExtensions 注释和图谱路径显示 same-protocol/direct 边界。
- **Responses same-protocol replay**：低。raw unknown top-level 与 native owned field 分离；typed 覆盖 raw 的关键测试存在并通过。
- **stream event preservation**：低到中。MCP/search stream 相关新增测试数量多，`openai/responses` 包通过；建议未来继续以事件类型表驱动补足新增事件。

## 建议项

1. 保留 `ProviderExtensions` 为唯一 provider-private sidecar；不要把协议 native fields 放回 `TransformerMetadata`。
2. 后续新增协议字段时，优先补充同协议 replay 测试，而不是扩大 unknown raw replay 的范围。
3. 对 stream event 新增类型保持表驱动测试，尤其是 MCP/search event 的 raw field round-trip。
4. 如果未来将 LossyDowngrade 展示到 UI/trace，单独做 consumer slice；不要让 transformer 层承担展示职责。

## 是否可以 finish-work / 是否需要继续切片

- **可以 finish-work**：是。
- **需要继续切片**：否。
- **合入前状态**：worktree clean，diff check 通过，`llm/go test ./... -count=1` 通过。
