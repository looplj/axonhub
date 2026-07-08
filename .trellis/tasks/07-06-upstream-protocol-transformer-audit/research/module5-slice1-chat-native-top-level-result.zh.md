# Module 5 Slice 5.1 — OpenAI Chat native top-level field preservation

## 范围

本 slice 属于 Phase 3: OpenAI Chat emission policy。

只处理 OpenAI Chat 顶层 native request fields：

- `web_search_options`
- `prediction`
- top-level `audio`

目标：

- OpenAI Chat inbound 同协议请求进入 `llm.Request` 时保留这些 native fields；
- OpenAI Chat outbound 到 `PlatformOpenAI` 时回填这些 fields；
- 不通过共享 `RequestFromLLM` 向所有 OpenAI-compatible provider 扩散；
- 不新增公共 `llm.Request` 字段。

不处理：

- `functions` / `function_call` deprecated compatibility；
- custom tool Chat representation；
- Chat response/stream audio output；
- Anthropic / LossyDowngrade。

## 取证

### codebase-memory CLI

按当前 goal，先用 CLI 刷新索引并搜索：

```bash
codebase-memory-mcp cli --json index_repository '{"repo_path":"/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean","mode":"fast"}'
codebase-memory-mcp cli --json search_graph '{"project":"Users-asuan-AI-axonhub-worktrees-responses-top-level-preservation-clean","query":"OpenAI chat RequestFromLLM chat completion request web_search_options prediction audio","limit":20}'
codebase-memory-mcp cli --json trace_path '{"project":"Users-asuan-AI-axonhub-worktrees-responses-top-level-preservation-clean","function_name":"RequestFromLLM","direction":"inbound","depth":2,"include_tests":true}'
```

证据：

- `RequestFromLLM` 位于 `llm/transformer/openai/outbound_convert.go`，被多个 provider 调用；
- callers 包括 `gemini/openai`、`openrouter`、`doubao`、`deepseek`、`zai`、`moonshot`、`copilot`、`openai` 等；
- 因此不能把 OpenAI Chat-only fields 直接加到 `RequestFromLLM` 的公共输出路径。

### 官方字段清单

本地字段文档：

- `/Users/asuan/项目/AI/axonhub/.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/protocol-field-extraction/openai-fields.md`

Chat request section 记录：

- `web_search_options` — Chat native built-in web search config；
- `audio` — top-level audio output params，配合 `modalities:["audio"]`；
- `prediction` — predicted output config。

## Red test

新增测试：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/inbound_test.go`
  - `TestInboundTransformer_TransformRequest_PreservesOpenAIChatNativeTopLevelFields`
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/outbound_test.go`
  - `TestOutboundTransformer_TransformRequest_PreservesOpenAIChatNativeTopLevelFieldsForOpenAIOnly`

红测试失败证据：

```text
req.ProviderExtensions.OpenAIChat undefined
unknown field OpenAIChat in struct literal of type llm.ProviderExtensions
undefined: llm.OpenAIChatProviderExtensions
undefined: llm.OpenAIChatRequestExtensions
```

说明 Chat native preservation seam 尚不存在。

## Green implementation

修改：

- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/provider_extensions.go`
  - 新增 `ProviderExtensions.OpenAIChat`；
  - 新增 `OpenAIChatProviderExtensions` / `OpenAIChatRequestExtensions`；
  - `RawTopLevelFields map[string]json.RawMessage` 保存 Chat-native top-level raw fields；
  - 更新 `CloneProviderExtensions`。
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/model.go`
  - `Request` 增加 raw fields：`WebSearchOptions`、`Prediction`、`Audio`。
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/chat_extensions.go`
  - 新增 `preserveOpenAIChatRequestExtensions`；
  - 新增 `applyOpenAIChatRequestExtensions`；
  - raw copy helper 保证同协议回填时不共享底层 buffer。
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/inbound_convert.go`
  - `Request.ToLLMRequest` 末尾保存 Chat provider extensions。
- `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm/transformer/openai/outbound.go`
  - `PlatformOpenAI` 分支才调用 `applyOpenAIChatRequestExtensions`；
  - `PlatformGoogle` 不回填，避免污染 OpenAI-compatible provider。

## 验证命令

在 `/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm` 执行：

```bash
go test ./transformer/openai -run 'Test(InboundTransformer_TransformRequest_PreservesOpenAIChatNativeTopLevelFields|OutboundTransformer_TransformRequest_PreservesOpenAIChatNativeTopLevelFieldsForOpenAIOnly)$' -count=1
go test ./transformer/openai -count=1
git diff --check
```

结果：

```text
ok  	github.com/looplj/axonhub/llm/transformer/openai	0.739s
ok  	github.com/looplj/axonhub/llm/transformer/openai	0.258s
```

`git diff --check` 无输出。

## 自审

- 字段归属：OpenAI Chat official fields 归属 `OpenAIChatProviderExtensions` + OpenAI Chat native model，不进入公共 `llm.Request`。
- 发射策略：只有 `openai.OutboundTransformer` 的 `PlatformOpenAI` 回填 raw fields；`PlatformGoogle` 测试断言不回填。
- 共享 builder 风险：没有扩大 `RequestFromLLM`；调用 `RequestFromLLM` 的其他 provider 不会自动收到这些 fields。
- 保真策略：用 `json.RawMessage` 保留 object/union payload；本 slice 不提前建半截类型。
- 范围控制：没有处理 deprecated `functions/function_call`，没有动 Chat response/stream，未触碰 Anthropic。

## 结论

Slice 5.1 通过本地 TDD、自审和 package-level validation。下一步进入 Slice 5.2：根据官方字段和作者架构决定 deprecated `functions/function_call` 或 modern custom tool Chat representation 的最小可验证修复；进入前必须重新用 codebase-memory CLI 搜索相关代码和字段。
