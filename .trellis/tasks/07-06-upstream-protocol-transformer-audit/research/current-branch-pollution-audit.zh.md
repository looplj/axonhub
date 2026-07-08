# 当前分支旧代码污染 / 可复用审计

日期：2026-07-07

## 0. 结论先行

当前分支 **不能作为下一轮实现基线**。

原因：当前分支相对作者 upstream `origin/unstable` 已有大规模历史改动：

```text
149 files changed, 80194 insertions(+), 700 deletions(-)
```

且这些改动来自早期错误/不完整判断：

- 把三协议字段修复扩大成跨协议大改；
- 把部分 Responses/Codex profile 字段误判为全量缺失；
- 把大量字段经 `TransformerMetadata` 或 shared helper 横向扩散；
- 混入 Chat、Anthropic、Gemini、OpenRouter、stream、metadata propagation 等多个主题。

所以后续实现不应在当前分支直接继续堆代码。正确做法是：

```text
从作者 upstream clean branch/worktree 开始，按新矩阵做最小 P1a/P1b/P1c。
当前分支只作为研究/参考/可摘代码来源。
```

## 1. 当前证据

### 1.1 作者 upstream 基线

```text
/Users/asuan/项目/AI/axonhub-worktrees/upstream-unstable
HEAD: 97c9351a ci: publish Helm chart to GHCR --issue=#1965 (#1966)
```

### 1.2 当前工作分支

```text
branch: codex-transformer-field-fixes
HEAD: c798c6e9 feat: preserve OpenAI Responses native request fields
```

当前分支相对 upstream 的已提交差异很大，不只是本轮未提交改动。

### 1.3 当前未提交业务代码

```text
M llm/provider_extensions.go
M llm/transformer/openai/responses/cross_protocol_test.go
M llm/transformer/openai/responses/outbound_test.go
M llm/transformer/openai/responses/request_extensions.go
M llm/transformer/shared/responses_lossy_downgrade.go
?? llm/openai_responses_classification.go
?? llm/openai_responses_classification_test.go
```

## 2. 当前分支已提交代码的总体判断

| 区域 | 当前分支状态 | 新矩阵判断 | 动作 |
|---|---|---|---|
| `llm/transformer/openai/responses` | 已有大规模 native preservation / raw replay 改动 | 方向部分正确，但实现混入太多旧假设 | 不直接继承；从 upstream 重做最小切片 |
| `llm/provider_extensions.go` | 已加 `RawTopLevelFields`、`ClientMetadata`、`NativeTools`、`AdditionalTools`、`PrependCount` | `RawTopLevelFields` 概念可复用；其余需逐项证明 | 只摘 raw top-level 思路，不整文件继承 |
| `llm/openai_responses_classification.go` | 新增全局 Responses type 分类函数 | 把 Responses 私有分类放到 `llm` 根包，容易污染公共层 | 不采用；若需要，放 `responses` 包内局部 helper |
| `llm/transformer/shared/*` | 增加 metadata/shared/lossy downgrade 等跨协议 helper | 超出 P1，不应先做 | 延后，不能进入 P1 |
| Chat/Anthropic/Gemini/OpenRouter 等 | 大量跨协议修复/测试 | 不是当前 P1 目标 | 延后或另开任务 |
| stream/aggregator/testdata | 大量 stream/testdata 变化 | stream 是独立问题 | 延后或另开任务 |

## 3. 当前未提交业务代码逐项审计

### 3.1 `llm/provider_extensions.go`

当前未提交 diff 只是给 `AdditionalTools` 调整注释和字段排版，但该文件在当前分支 HEAD 已经远离 upstream。

当前分支已有字段：

```text
ClientMetadata
RawTopLevelFields
NativeTools
AdditionalTools
RawTools
ToolSignatures
RawToolChoice
RawInputItems
PrependCount
```

按新矩阵判断：

| 字段 | 判断 | 动作 |
|---|---|---|
| `RawTopLevelFields` | 符合 P1a 核心方向，用于 `context_management` / unknown top-level / profile top-level | 可复用概念，但应在 clean branch 重新实现 |
| `RawTools` / `RawToolChoice` / `RawInputItems` / `ToolSignatures` | 作者 upstream 已有 | 保留 upstream 原机制 |
| `ClientMetadata` | 不在当前 OpenAI Responses 官方 request 矩阵主缺口里；可能是旧 Codex profile 假设 | 不进 P1，除非找到真实入站 payload 证据 |
| `NativeTools` | 试图把 known native tool raw 独立出来；但 upstream 已有 raw tools 机制 | 不进 P1，避免重复 raw tool/input 设计 |
| `AdditionalTools` | 如果是 top-level `additional_tools`，应走 top-level raw fallback；当前把它当 input item 特例不清楚 | 不继承；按真实出现位置重做 |
| `PrependCount` | 与 prompt pipeline 插入消息相关，不是字段保真核心 | 不进 P1 |

结论：**不直接保留当前文件改动；只保留 `RawTopLevelFields` 这个思路。**

### 3.2 `llm/transformer/openai/responses/request_extensions.go`

当前分支已有复杂功能：

```text
RawTopLevelFields
ClientMetadata
NativeTools
AdditionalTools
preservation diagnostics
known native tool/input classification
merge native tools
prepend-aware input merge
```

新矩阵判断：

| 片段 | 判断 | 动作 |
|---|---|---|
| top-level raw fallback | 符合 P1a | 重新按 upstream 最小实现 |
| raw tools/input/tool_choice replay | upstream 已有 | 不重复实现，保留 upstream 机制 |
| ClientMetadata replay | 缺真实协议证据 | 不进 P1 |
| NativeTools 单独 replay | 与 upstream raw tools 机制重叠 | 不进 P1 |
| AdditionalTools input-item 特例 | 位置不清，容易把 top-level/profile 和 input item 混掉 | 不继承；按出现位置处理 |
| diagnostics metadata | 有价值但非 P1 必需；容易扩大 `TransformerMetadata` | 延后 |

结论：**当前实现思路过宽；P1 只重做 raw top-level fallback。**

### 3.3 `llm/transformer/openai/responses/model.go`（当前分支已提交差异）

当前分支 Request 已新增：

```text
FrequencyPenalty
PresencePenalty
ClientMetadata
CacheControl
Prompt
TopK
Modalities
```

与 upstream 对比，新矩阵判断：

| 字段 | 判断 | 动作 |
|---|---|---|
| `Prompt` | 官方 Responses request 字段；upstream 已 TODO | P1b 应保留/恢复 typed field |
| `Conversation` | 仍被注释；官方 Responses request 字段 | P1b 应恢复 typed field |
| `context_management` | 当前仍未建模 | P1a raw/native opaque |
| `FrequencyPenalty` / `PresencePenalty` | 可能来自 Chat/OpenRouter 兼容字段，非当前 Responses P1 缺口 | 不进 P1 |
| `ClientMetadata` | 非当前官方矩阵缺口 | 不进 P1，除非有真实 payload 证据 |
| `CacheControl` | 更像跨协议/Anthropic/OpenRouter cache_control 旧修复 | 不进 P1 |
| `TopK` / `Modalities` | 非当前 Responses P1 缺口 | 不进 P1 |

结论：**只采纳 `Prompt` / `Conversation` typed TODO 方向；不继承其他字段扩散。**

### 3.4 `llm/openai_responses_classification.go`

当前新增在 `llm` 根包：

```text
IsKnownOpenAIResponsesNativeToolType
IsKnownOpenAIResponsesInputItemType
```

问题：

- 把 OpenAI Responses 私有工具/输入项分类放到公共 `llm` 根包；
- 变成全局知识表，后续容易被其他 transformer 误用；
- 当前 P1 不需要重新分类 tools/input，因为 upstream 已有 raw replay 测试。

结论：**不采用。删除或不要带入 clean branch。若以后需要，放在 `llm/transformer/openai/responses` 包内。**

### 3.5 `llm/transformer/shared/responses_lossy_downgrade.go`

当前属于跨协议 lossy downgrade 诊断。

新矩阵判断：这是后续有价值的 P2/P3 能力，但不是当前 P1：

```text
P1 只修 OpenAI Responses -> OpenAI Responses 同协议保真。
```

结论：**延后，不进第一轮实现。**

### 3.6 `llm/transformer/openai/responses/cross_protocol_test.go`

从文件名和当前任务目标看，它测试跨协议行为。

新矩阵判断：跨协议是后续阶段，不应阻塞 P1。

结论：**延后，不进第一轮实现。**

### 3.7 `llm/transformer/openai/responses/outbound_test.go`

当前已有部分测试可能覆盖：

```text
RawTopLevelFields
ClientMetadata
NativeTools
AdditionalTools
```

判断：

- `RawTopLevelFields` 相关测试可作为 P1a 测试灵感；
- `ClientMetadata` / `NativeTools` / `AdditionalTools` 的测试不应直接继承；
- 应在 clean branch 写更窄的 P1a 测试：`context_management` / unknown top-level / raw model 不覆盖 mapped model。

结论：**只摘测试思路，不直接保留当前测试。**

## 4. 文档/研究文件判断

| 路径 | 判断 | 动作 |
|---|---|---|
| `.trellis/tasks/07-06-upstream-protocol-transformer-audit/**` | 当前任务规划和新矩阵所在 | 保留 |
| `.trellis/spec/**` | Trellis 项目规范 | 保留 |
| `docs/specs/vendor/**` | 官方协议快照/抽取源 | 保留，但标明日期 |
| `docs/specs/protocols/**` | 三协议整理文档 | 保留，但以后以新矩阵为准修订 |
| `docs/adr/0002...` | 字段归属 ADR | 保留，但需追加新矩阵修正 |
| `.agents/**` / `.codex/**` / `.trellis/**` | Trellis/Codex 工作流配置 | 保留，和业务代码分开提交 |
| `Dockerfile.cn` / docker-compose.* | 与当前任务无关 | 不纳入本任务，另行确认 |

## 5. 推荐下一步

### 推荐路径 A：最稳

1. 当前分支不继续实现。
2. 创建新的 clean implementation worktree/branch：

```text
从 origin/unstable 创建：codex/responses-top-level-preservation-clean
```

3. 只从当前分支复制/参考这些文档：

```text
.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/verified-author-architecture-and-field-matrix.zh.md
.trellis/tasks/07-06-upstream-protocol-transformer-audit/research/upstream-architecture-reread-2026-07-06.zh.md
```

4. 在 clean branch 做 P1a：

```text
RawTopLevelFields 最小 sidecar
request_extensions.go 捕获 unknown/profile top-level
marshalRequestPayload merge raw top-level without overriding structured fields
```

5. P1a 审查通过后再 P1b：

```text
Prompt
Conversation
```

### 不推荐路径 B

继续在当前 `codex-transformer-field-fixes` 分支上修。

风险：

- 149 个文件大改会掩盖真正 diff；
- 旧跨协议修改和新 P1 修改会混在一起；
- 审查时很难确认 bug 是新引入还是旧污染；
- 很容易再次把非 P1 字段塞进公共层。

## 6. 当前可复用项清单

| 可复用项 | 如何复用 |
|---|---|
| `RawTopLevelFields` 概念 | 在 clean branch 用最小结构重写 |
| `merge raw top-level 不覆盖结构化字段` 思路 | 写 P1a 测试后实现 |
| `Prompt *Prompt` 启用方向 | P1b 在 `responses.Request` typed field 中实现 |
| 已有新矩阵和架构审计文档 | 作为实现约束 |

## 7. 当前应丢弃/延后项清单

| 项 | 原因 |
|---|---|
| `ClientMetadata` | 缺真实协议证据，不在当前矩阵主缺口 |
| `NativeTools` 单独结构 | 与 upstream raw tool replay 重叠 |
| `AdditionalTools` 作为 input item 特例 | 位置不清，应按真实 top-level/tool/input 出现位置分别处理 |
| `llm/openai_responses_classification.go` | OpenAI Responses 私有分类污染公共 llm 包 |
| `responses_lossy_downgrade.go` | 跨协议诊断，非 P1 |
| `cross_protocol_test.go` | 跨协议测试，非 P1 |
| Chat/Anthropic/Gemini/OpenRouter 修改 | 后续阶段，不能混入当前 P1 |
