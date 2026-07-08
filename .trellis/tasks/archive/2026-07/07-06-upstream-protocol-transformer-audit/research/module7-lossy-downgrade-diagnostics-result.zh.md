# Module 7：Cross-protocol LossyDowngrade diagnostics 结果记录

日期：2026-07-08
工作树：`/Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean`
分支：`codex/responses-top-level-preservation-clean`
最终修复提交：`5778ebd5 fix: address lossy downgrade review findings`

## 目标

让跨协议转换中无法表达的字段损失变成可见、可测试、可审查的诊断信息，而不是静默丢弃或伪造语义映射。

边界：

- 同协议保真仍由各协议 native/raw preservation seam 负责。
- 跨协议无等价语义时记录 `LossyDowngrade`。
- 诊断信息不是请求语义，不能写入 common `llm.Request`。
- 不把 OpenAI Responses MCP、Anthropic MCP connector、Chat legacy function 等不同工具生态强行互转。

## 实现位置

核心诊断模型：

- `llm/lossy_downgrade.go`
- `llm/provider_extensions.go`

目标协议 outbound adapter 负责做 downgrade 判定：

- `llm/transformer/anthropic/lossy_downgrade.go`
- `llm/transformer/openai/lossy_downgrade.go`
- `llm/transformer/openai/responses/lossy_downgrade.go`

相关桥接：

- `llm/transformer/openai/chat_extensions.go`
- `llm/transformer/openai/outbound.go`
- `llm/transformer/openai/responses/request_extensions.go`
- `llm/transformer/openai/responses/outbound.go`

## 已验证的核心结构

`LossyDowngrade` 字段：

- `SourceProtocol`
- `SourceField`
- `TargetProtocol`
- `Reason`
- `Severity`

诊断存储位置：

- `ProviderExtensions.Diagnostics.LossyDowngrades`
- `json:"-"`，不发送给 provider
- clone 时保留诊断 sidecar

关键决策：

- 不在 common `llm.Request` 上新增 `LossyDowngrades`。
- 不复用 OpenAI/Anthropic native extension 保存诊断。
- 不把诊断数据作为协议 native 字段 replay。

## 覆盖矩阵

### Responses -> Anthropic

诊断字段：

- `prompt`
- `conversation`
- `previous_response_id`
- `background`
- `include`
- `max_tool_calls`
- `prompt_cache_retention`
- `truncation`
- `context_management`
- raw top-level fallback：`additional_tools`、`defer_loading`、未来未知字段
- tools：`mcp`、`file_search`、`code_interpreter`

### Responses -> OpenAI Chat

诊断字段：

- `prompt`
- `conversation`
- `previous_response_id`
- `background`
- `include`
- `max_tool_calls`
- `truncation`
- `context_management`
- raw top-level fallback：`additional_tools`、`defer_loading`、未来未知字段
- tools：`mcp`、`file_search`、`code_interpreter`

不诊断：

- `prompt_cache_retention`，因为已桥接到 Chat 同名字段并有测试覆盖。

### OpenAI Chat -> Anthropic

诊断字段：

- `prompt_cache_retention`
- `n`
- `web_search_options`
- `prediction`
- `audio`
- `functions`
- `function_call`

### OpenAI Chat -> Responses

诊断字段：

- `n`
- `web_search_options`
- `prediction`
- `audio`
- `functions`
- `function_call`

不诊断：

- `prompt_cache_retention`，因为已桥接到 Responses 同名字段并有测试覆盖。

### Anthropic -> Responses

诊断字段：

- `container`
- `inference_geo`
- `mcp_servers`
- `tools[].type=mcp_toolset`

### Anthropic -> OpenAI Chat

诊断字段：

- `container`
- `inference_geo`
- `mcp_servers`
- `tools[].type=mcp_toolset`

## 桥接决策

允许桥接：

- OpenAI Responses `prompt_cache_retention` -> OpenAI Chat `prompt_cache_retention`
- OpenAI Chat `prompt_cache_retention` -> OpenAI Responses `prompt_cache_retention`
- Responses `background` same-protocol round-trip：入站记录 field-present hint，Responses outbound 回放

不允许伪映射：

- OpenAI Responses `tools[].type=mcp` 不映射为 Anthropic `mcp_toolset`。
- Anthropic `mcp_servers` / `mcp_toolset` 不映射为 OpenAI Responses MCP。
- OpenAI Chat `functions` / `function_call` 不伪造为 Anthropic 或 Responses native tool 语义。

## 审查失败与修复

首轮 Module 7 审查发现：

- P1：`LossyDowngrades` 曾放在 common `llm.Request`，污染共享模型。
- P2：diagnostic helper 存在重复。
- P1：Responses 部分字段和 tool types 诊断覆盖不足。
- P2：Chat -> Responses `prompt_cache_retention` 应桥接，不应诊断。

已在 `5778ebd5` 修复：

- `LossyDowngrades` 移入 `ProviderExtensions.Diagnostics`。
- 公共 helper 收敛到 `llm/lossy_downgrade.go`。
- 补齐 Responses raw top-level / tool types 覆盖。
- 补齐 Chat <-> Responses `prompt_cache_retention` 桥接和测试。

## 验证

已验证命令：

```bash
cd /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean/llm

go test . ./transformer/openai ./transformer/anthropic ./transformer/openai/responses -count=1

git -C /Users/asuan/项目/AI/axonhub-worktrees/responses-top-level-preservation-clean diff --check
```

结果：通过。

索引状态：

```bash
codebase-memory-mcp cli index_status '{"project":"Users-asuan-AI-axonhub-worktrees-responses-top-level-preservation-clean"}'
```

结果：`status=ready`。

## 自审结论

通过 Module 7 自审：

- 诊断 sidecar 与协议 native storage 分离。
- downgrade 判定在目标协议 outbound adapter 本地完成，符合作者 adapter 架构。
- same-protocol preservation、cross-protocol bridge、cross-protocol diagnostic 三类路径已分开。
- 不新增万能 AST，不扩大 common `llm.Request`。
- 不做工具生态伪映射。

## 后续

进入 Module 8：字段归属表/spec hardening 与父级整体审查。
