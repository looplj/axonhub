# Unified Search API Design

> **Status**: In Progress
> **Date**: 2026-02-19
> **Author**: AI Assistant

## Overview

AxonHub 新增 `search` 请求类型，复用已有的 orchestrator 基础设施（负载均衡、重试/Failover、熔断器、连接追踪、请求持久化、Usage Log）来提供统一的 Web Search 服务。

## Implementation Status

- ✅ `POST /v1/search` 后端路由 + OpenAI 风格入站转换器（统一 `SearchRequest`/`SearchResponse`）
- ✅ Search orchestrator：精简 inbound middleware（跳过 quota / model access / model mapping / prompt injection）
- ✅ Search 渠道类型：`search_tavily`、`search_brave`、`search_exa`（并与 `/v1/models`、Model Association 隔离）
- ✅ Provider outbound：Tavily、Brave、Exa（Brave 为 GET query 参数）
- ✅ 前端：Search Channel 专用对话框 / 行操作 / Test 对话框

## Market Research

### Provider Landscape

市场上为 LLM 服务的 Search API 主要分为两类：

| 分类 | Provider | 特点 | 定价 |
|---|---|---|---|
| **AI-Native Search** | Tavily | Citation-ready, LLM 生成答案, 时间/域名过滤 | $0.008/req |
| | Exa | 神经语义搜索, 学术/研究导向, 全文提取 | $0.005/req |
| | Brave Search | 独立索引, 隐私优先, 丰富本地/富数据 | $0.003/req |
| | Linkup | SERP + AI 混合, LLM 连接器丰富 | $0.005/req |
| **SERP Scraper** | Serper | Google SERP, 极快(1-2s), 超低价 | $0.001/req |
| | SerpAPI | 多引擎(Google/Bing/DDG/Yandex/Baidu) | $0.015/req |

**推荐优先支持**: Tavily（LLM search 最流行）和 Brave Search（独立索引、性价比高）。

### Provider API Format Comparison

#### Request Parameters

| 字段 | Tavily | Exa | Brave | Serper |
|---|---|---|---|---|
| 查询 | `query` | `query` | `q` | `q` |
| 结果数 | `max_results` (1-20) | `numResults` (1-100) | `count` (1-20) | `num` (1-10) |
| 搜索深度 | `search_depth` (basic/advanced/fast/ultra-fast) | `type` (neural/auto/fast/deep/instant) | — | — |
| 时间过滤 | `time_range` (day/week/month/year) | `startPublishedDate` / `endPublishedDate` (ISO 8601) | `freshness` (pd/pw/pm/py) | `tbs` |
| 域名包含 | `include_domains` | `includeDomains` | — | `site:` in query |
| 域名排除 | `exclude_domains` | `excludeDomains` | — | `-site:` in query |
| 语言/地区 | `country` | `userLocation` | `country` / `search_lang` | `gl` / `hl` |
| 主题/分类 | `topic` (general/news/finance) | `category` (news/research paper/...) | — | `type` (search/news/...) |
| 含 LLM 答案 | `include_answer` (bool/basic/advanced) | — (单独 `/answer` 端点) | — | — |
| 含完整正文 | `include_raw_content` (bool/markdown/text) | `contents.text` (bool) | `extra_snippets` (bool) | — |
| 含图片 | `include_images` (bool) | — (`.image` per result) | — | — |

#### Response Fields

| 字段 | Tavily | Exa | Brave | Serper |
|---|---|---|---|---|
| 结果列表 | `results[]` | `results[]` | `web.results[]` | `organic[]` |
| 标题 | `.title` | `.title` | `.title` | `.title` |
| URL | `.url` | `.url` | `.url` | `.link` |
| 摘要 | `.content` | `.text` | `.description` | `.snippet` |
| 相关性分数 | `.score` | `.highlightScores` | — | — |
| 发布日期 | — | `.publishedDate` | `.age` | `.date` |
| Favicon | `.favicon` | `.favicon` | `.favicon` | — |
| LLM 答案 | `.answer` | — | — | — |
| 图片 | `.images[]` | `.image` (per result) | — | — |
| 用量 | `.usage.credits` | `.costDollars` | — | — |

## Unified API Design

### Design Principles

1. **以 Tavily 为蓝本** — LLM search 领域使用最广泛，字段设计最合理
2. **字段命名用 snake_case** — 与 AxonHub 已有风格（OpenAI 风格）保持一致
3. **MVP 最小化** — 首版仅暴露 `query`、`max_results`、`allowed_domains`、`blocked_domains` 四个字段，其余能力通过 `extra_body` 透传
4. **复用 orchestrator** — 与 embedding/rerank/image 走同样的 pipeline 路径

### Endpoint

```
POST /v1/search
```

### Model Naming Convention

Search 请求的 `model` 字段采用特殊后缀约定，用于区分 search 模型与普通 LLM 模型：

| Model 名 | 说明 |
|---|---|
| `__search` | 默认搜索模型，路由到任意可用的 search channel |
| `__tavily_search` | 指定使用 Tavily provider |
| `__brave_search` | 指定使用 Brave Search provider |
| `__exa_search` | 指定使用 Exa provider |

**路由逻辑**：
- 客户端发送 `"model": "__search"` → `selectCandidates` 匹配所有 `search_*` 类型渠道中 `supported_models` 包含 `__search` 的渠道
- 客户端发送 `"model": "__tavily_search"` → 只匹配 `search_tavily` 类型渠道
- 所有 search channel 的 `supported_models` 都**必须包含 `__search`**，可额外包含 `__xxx_search` 用于定向路由

### Request Format

```jsonc
{
  // === Core (required) ===
  "query": "latest developments in quantum computing",  // 搜索查询
  "model": "__search",                                   // 搜索模型 (默认 __search)

  // === Result Control ===
  "max_results": 5,           // 最大结果数, default 5, max 20

  // === Domain Filter ===
  "allowed_domains": ["arxiv.org", "nature.com"],  // 仅搜索这些域名
  "blocked_domains": ["pinterest.com"],             // 排除这些域名

  // === Extra ===
  "extra_body": {}            // Provider 特有参数透传 (search_depth, topic, include_answer 等)
}
```

### Response Format

```jsonc
{
  "query": "latest developments in quantum computing",
  "results": [
    {
      "title": "Quantum Computing Breakthrough at MIT",
      "url": "https://news.mit.edu/2026/quantum-breakthrough",
      "content": "Researchers at MIT have achieved a significant milestone...",  // 摘要/snippet
      "score": 0.95,                                      // 相关性分数 (0-1), 部分 provider 不返回
      "published_date": "2026-02-15",
      "favicon": "https://news.mit.edu/favicon.ico"
    }
  ],
  "response_time": 1.23,          // 响应时间 (秒)
  "usage": {
    "credits": 1
  }
}
```

### Error Format

遵循已有 OpenAI 兼容错误格式：

```json
{
  "error": {
    "message": "Invalid search query: query is required",
    "type": "invalid_request_error",
    "code": "invalid_query"
  }
}
```

## Channel Configuration

Search provider 作为新的渠道类型接入 AxonHub，复用现有的 Channel 管理体系。所有 search 渠道类型统一使用 `search_` 前缀，方便后端统一识别和处理。

### 渠道类型定义

| Channel Type | Provider | Base URL | 认证方式 | 默认 supported_models |
|---|---|---|---|---|
| `search_tavily` | Tavily | `https://api.tavily.com` | `Authorization: Bearer <api_key>` | `__search`, `__tavily_search` |
| `search_brave` | Brave Search | `https://api.search.brave.com` | `X-Subscription-Token: <api_key>` | `__search`, `__brave_search` |
| `search_exa` | Exa | `https://api.exa.ai` | `Authorization: Bearer <api_key>` | `__search`, `__exa_search` |

### 渠道创建示例

在 AxonHub Web UI 中创建 Search 渠道，与 LLM 渠道完全一致的表单流程：

**Tavily 渠道**:
- **Type**: `search_tavily`
- **Name**: `Tavily Search`
- **Base URL**: `https://api.tavily.com` (自动填充)
- **API Keys**: 填入 Tavily API Key
- **Supported Models**: `__search`, `__tavily_search`
- **Default Test Model**: `__search`

**Brave Search 渠道**:
- **Type**: `search_brave`
- **Name**: `Brave Web Search`
- **Base URL**: `https://api.search.brave.com` (自动填充)
- **API Keys**: 填入 Brave Search API Key
- **Supported Models**: `__search`, `__brave_search`
- **Default Test Model**: `__search`

### 渠道路由

客户端请求中的 `model` 字段用于路由到对应的 search channel，与 LLM 模型路由逻辑完全一致：

```
POST /v1/search { "model": "__search" }
  → selectCandidates 选择 supported_models 包含 "__search" 的 search_* 渠道
  → 负载均衡选择具体 channel 实例
  → SearchOutboundTransformer 转换为 Provider API 格式发送
```

支持所有现有的渠道能力：
- **多 API Key 轮转**: `credentials.apiKeys` 支持多 key 负载均衡
- **请求/Header 覆盖**: 通过 `settings.bodyOverrideOperations` / `settings.headerOverrideOperations` 自定义请求
- **代理**: 通过 `settings.proxy` 配置代理
- **排序权重**: 通过 `orderingWeight` 控制多渠道优先级

**与普通渠道的差异**：
- **无 Model Mapping**: search model 使用固定后缀约定，不需要模型名映射
- **无 Quota 限制**: search 不走 API Key 配额系统
- **无 API Key Model Access**: search model 不受 API Key 模型访问限制
- **无 Prompt 注入**: search 不涉及 system prompt

### API Format

新增 `APIFormat`：

| APIFormat | 说明 |
|---|---|
| `tavily/search` | Tavily Search API 格式 |
| `brave/search` | Brave Search API 格式 |
| `exa/search` | Exa Search API 格式 |

## 与普通渠道的隔离设计

Search 渠道虽然复用 Channel 管理体系，但在多个维度上需要与普通 LLM 渠道**完全隔离**，避免相互干扰。

### 1. Models API 不返回 Search 渠道模型

`/v1/models` API 仅返回 LLM 模型，不应包含 `__search`、`__tavily_search` 等 search 模型。

**后端改动**：

在 `internal/ent/channel/type.go` 新增 `IsSearch()` 方法：

```go
func (t Type) IsSearch() bool {
    return strings.HasPrefix(string(t), "search_")
}
```

在 `internal/server/biz/model.go` 的 `ListEnabledModels` 中，当 `QueryAllChannelModels=true` 时过滤 search 渠道：

```go
// ListEnabledModels
for _, ch := range channels {
    // 跳过 search 渠道，search model 不应出现在 /v1/models
    if ch.Channel.Type.IsSearch() {
        continue
    }
    entries := ch.GetModelEntries()
    // ...
}
```

### 2. Model Association 排除 Search 渠道

Model Association 功能用于将抽象模型（如 `gpt-4`）关联到渠道，search 渠道不应出现在候选列表中。

**后端改动**：

在 `internal/server/biz/model.go` 的 `findUnassociatedChannels` 中过滤 search 渠道：

```go
func findUnassociatedChannels(channels []*ent.Channel, associations []*objects.ModelAssociation) []*UnassociatedChannel {
    // 过滤掉 search 渠道
    channels = lo.Filter(channels, func(ch *ent.Channel, _ int) bool {
        return !ch.Type.IsSearch()
    })
    // ...
}
```

在 `internal/server/orchestrator/candidates.go` 的 `resolveAssociations` 中，search 渠道不参与 association 匹配：

```go
func (s *DefaultSelector) resolveAssociations(...) {
    channels := s.ChannelService.GetEnabledChannels()
    // 排除 search 渠道
    channels = lo.Filter(channels, func(ch *Channel, _ int) bool {
        return !ch.Channel.Type.IsSearch()
    })
    // ...
}
```

**前端改动**：

`QueryUnassociatedChannels` GraphQL 查询应在后端已排除 search 渠道，前端无需额外处理。

### 3. 独立的创建/编辑 Dialog

Search 渠道的创建/编辑表单与普通渠道差异较大，使用独立的 Dialog 组件：

**普通渠道 Dialog 字段** vs **Search 渠道 Dialog 字段**：

| 字段 | 普通渠道 | Search 渠道 |
|---|---|---|
| Type (provider) | ✅ 所有 LLM providers | ✅ 仅 `search_*` providers |
| Name | ✅ | ✅ |
| Base URL | ✅ | ✅ (自动填充) |
| API Keys | ✅ (多 key, OAuth 等) | ✅ (仅普通 API Key) |
| Supported Models | ✅ (手动/自动同步) | ✅ (固定 `__search` + `__xxx_search`，不可编辑) |
| Default Test Model | ✅ | ✅ (固定 `__search`) |
| Stream Policy | ✅ | ❌ (search 无流式) |
| Auto Sync Models | ✅ | ❌ |
| Manual Models | ✅ | ❌ |
| Tags | ✅ | ✅ |
| Ordering Weight | ✅ | ✅ |
| Remark | ✅ | ✅ |

**前端改动**：

新增 `frontend/src/features/channels/components/search-channel-dialog.tsx`：
- 独立的 Dialog 组件，精简表单字段
- `supported_models` 由选中的 channel type 自动决定（如选 `search_tavily` 自动填入 `['__search', '__tavily_search']`），不暴露给用户编辑
- 不显示 Stream Policy、Auto Sync、Manual Models 等不适用字段
- 凭据区域仅显示 API Keys 输入（不显示 OAuth、GCP 等）

渠道列表页 toolbar 的 "Add Channel" 按钮旁新增 "Add Search Channel" 按钮（或下拉选择）。

### 4. 不同的 Row Actions

Search 渠道在列表中的行操作与普通渠道不同，需要精简不适用的操作项。

**普通渠道 Row Actions** vs **Search 渠道 Row Actions**：

| Action | 普通渠道 | Search 渠道 | 说明 |
|---|---|---|---|
| Edit | ✅ | ✅ (打开 search-channel-dialog) | |
| Test | ✅ | ✅ (发送 search 请求测试) | |
| Duplicate | ✅ | ✅ | |
| Model Mapping | ✅ | ❌ | search 不需要模型映射 |
| Model Price | ✅ | ❌ | search 按次计费，不需要 token 定价 |
| Overrides | ✅ | ✅ | 仍可覆盖请求参数 |
| Proxy | ✅ | ✅ | 仍可配置代理 |
| Transform Options | ✅ | ❌ | search 不涉及消息格式转换 |
| Disabled API Keys | ✅ (条件) | ✅ (条件) | |
| Mark Error Resolved | ✅ (条件) | ✅ (条件) | |
| Archive | ✅ | ✅ | |
| Delete | ✅ | ✅ | |

**前端改动**：

在 `channels-columns.tsx` 的 `ActionCell` 中，根据 `channel.type.startsWith('search_')` 判断：

```tsx
const isSearch = channel.type.startsWith('search_');

// Edit 按钮 → 打开对应的 dialog
const handleEdit = () => {
  setCurrentRow(channel);
  setOpen(isSearch ? 'editSearch' : 'edit');
};

// DropdownMenu 中条件隐藏
{!isSearch && (
  <DropdownMenuItem onClick={() => { setOpen('modelMapping'); }}>
    Model Mapping
  </DropdownMenuItem>
)}
{!isSearch && (
  <DropdownMenuItem onClick={() => { setOpen('price'); }}>
    Model Price
  </DropdownMenuItem>
)}
{!isSearch && (
  <DropdownMenuItem onClick={() => { setOpen('transformOptions'); }}>
    Transform Options
  </DropdownMenuItem>
)}
```

**Test 行为差异**：普通渠道测试发送 chat completion 请求，search 渠道测试应发送 `POST /v1/search` 请求（query 使用 default test query）。

## Implementation Plan

### Architecture

```
Client
  │
  ▼
POST /v1/search
  │
  ▼
SearchInboundTransformer          ← 解析统一 SearchRequest → llm.Request{Search: ...}
  │
  ▼
ChatCompletionOrchestrator        ← 精简中间件: selectCandidates, persistRequest + outbound 全部
  │
  ▼
SearchOutboundTransformer         ← llm.Request{Search: ...} → Provider HTTP Request
  │
  ├──► Tavily   (POST https://api.tavily.com/search)
  └──► Brave    (GET  https://api.search.brave.com/res/v1/web/search)
```

### Middleware Chain (与普通渠道的差异)

Search orchestrator 复用 `ChatCompletionOrchestrator`，但**精简 inbound 中间件链**：

```go
// 普通 chat/embedding/rerank 的 inbound 中间件链:
middlewares = append(middlewares,
    enforceQuota(inbound, processor.QuotaService),         // ❌ Search 不需要
    checkApiKeyModelAccess(inbound),                        // ❌ Search 不需要
    applyModelMapping(inbound),                             // ❌ Search 不需要
    selectCandidates(inbound),                              // ✅ 保留 - 核心路由能力
    injectPrompts(inbound),                                 // ❌ Search 不需要
    persistRequest(inbound),                                // ✅ 保留 - 记录搜索请求
)

// Search 精简后的 inbound 中间件链:
middlewares = append(middlewares,
    selectCandidates(inbound),                              // ✅ 路由到 search channel
    persistRequest(inbound),                                // ✅ 记录搜索请求
)

// outbound 中间件链: 完全保留，与普通渠道一致
middlewares = append(middlewares,
    applyOverrideRequestBody(outbound),
    applyOverrideRequestHeaders(outbound),
    withPerformanceRecording(outbound),
    withModelCircuitBreaker(outbound, ...),
    persistRequestExecution(outbound),
    withConnectionTracking(outbound, ...),
)
```

**实现方式**: 在 `ChatCompletionOrchestrator` 中新增 `NewSearchOrchestrator` 构造函数（或在 `Process` 方法中根据 `RequestType` 判断），跳过不需要的 inbound 中间件。

### Code Changes

#### Phase 1: Core Infrastructure

| Status | Layer | File | Change |
|---|---|---|---|
| ✅ | Constants | `llm/constants.go` | Add `RequestTypeSearch`, `APIFormatTavilySearch`, `APIFormatBraveSearch` |
| ✅ | Constants | `llm/constants.go` | Add `APIFormatExaSearch` |
| ✅ | Model | `llm/search.go` (new) | Define `SearchRequest`, `SearchResponse`, `SearchResult` |
| ✅ | Model | `llm/model.go` | Add `Search *SearchRequest` to `Request`, `Search *SearchResponse` to `Response` |

`llm/search.go` 数据结构：
```go
type SearchRequest struct {
    Query          string   `json:"query" binding:"required"`
    MaxResults     *int     `json:"max_results,omitempty"`     // default 5, max 20
    AllowedDomains []string `json:"allowed_domains,omitempty"`
    BlockedDomains []string `json:"blocked_domains,omitempty"`
    ExtraBody      json.RawMessage `json:"extra_body,omitempty"`
}

type SearchResponse struct {
    Query        string         `json:"query"`
    Results      []SearchResult `json:"results"`
    ResponseTime float64        `json:"response_time"`
    Usage        *SearchUsage   `json:"usage,omitempty"`
}

type SearchResult struct {
    Title         string   `json:"title"`
    URL           string   `json:"url"`
    Content       string   `json:"content"`
    Score         *float64 `json:"score,omitempty"`
    PublishedDate string   `json:"published_date,omitempty"`
    Favicon       string   `json:"favicon,omitempty"`
}

type SearchUsage struct {
    Credits int `json:"credits"`
}
```

#### Phase 2: Transformers

| Status | Layer | File | Change |
|---|---|---|---|
| ✅ | Inbound | `llm/transformer/openai/search_inbound.go` (new) | Parse client request → `llm.Request{Search: ...}`, set `RequestType = RequestTypeSearch` |
| ✅ | Outbound | `llm/transformer/tavily/outbound.go` (new) | `llm.Request{Search}` → Tavily HTTP POST, Tavily Response → `llm.Response{Search}` |
| ✅ | Outbound | `llm/transformer/tavily/model.go` (new) | Tavily provider-specific request/response structs |
| ✅ | Outbound | `llm/transformer/brave/outbound.go` (new) | `llm.Request{Search}` → Brave HTTP GET, Brave Response → `llm.Response{Search}` |
| ✅ | Outbound | `llm/transformer/brave/model.go` (new) | Brave provider-specific request/response structs |
| ✅ | Outbound | `llm/transformer/exa/outbound.go` (new) | `llm.Request{Search}` → Exa HTTP POST, Exa Response → `llm.Response{Search}` |
| ✅ | Outbound | `llm/transformer/exa/model.go` (new) | Exa provider-specific request/response structs |

**Inbound Transformer** 职责（参考 `openai/embedding_inbound.go`）：
1. 从 `httpclient.Request.Body` 解析 JSON → `SearchRequest`
2. 设置 `llm.Request.Model` = 请求中的 `model` 字段（默认 `__search`）
3. 设置 `llm.Request.Search = &searchRequest`
4. 设置 `llm.Request.RequestType = llm.RequestTypeSearch`
5. 设置 `llm.Request.APIFormat = llm.APIFormatTavilySearch`（默认 inbound 格式）

**Outbound Transformer** 职责（参考 `jina/outbound.go`）：
1. `TransformRequest`: 将 `llm.Request.Search` 转换为 provider-specific HTTP 请求
2. `TransformResponse`: 将 provider HTTP 响应转换为 `llm.Response{Search: &SearchResponse{...}}`
3. search 是非流式请求，不需要处理 stream

#### Phase 3: Backend Wiring

| Status | Layer | File | Change |
|---|---|---|---|
| ✅ | Channel Enum | `internal/ent/schema/channel.go` | Add `search_tavily`, `search_brave` to `field.Enum("type").Values(...)` |
| ✅ | Channel Enum | `internal/ent/schema/channel.go` | Add `search_exa` to `field.Enum("type").Values(...)` |
| ✅ | Channel Type Helper | `internal/ent/channel/type.go` | Add `IsSearch() bool` method: `strings.HasPrefix(string(t), "search_")` |
| ✅ | Channel Biz | `internal/server/biz/channel_llm.go` | Add `case channel.TypeSearchTavily:` / `case channel.TypeSearchBrave:` in `buildChannelWithTransformer`, construct corresponding outbound transformer |
| ✅ | Channel Biz | `internal/server/biz/channel_llm.go` | Add `case channel.TypeSearchExa:` wiring to Exa outbound transformer |
| ✅ | Models API 隔离 | `internal/server/biz/model.go` | `ListEnabledModels` 中跳过 `ch.Channel.Type.IsSearch()` 的渠道 |
| ✅ | Model Association 隔离 | `internal/server/biz/model.go` | `findUnassociatedChannels` 中过滤 `ch.Type.IsSearch()` 的渠道 |
| ✅ | Candidate 隔离 | `internal/server/orchestrator/candidates.go` | `resolveAssociations` 中排除 `ch.Channel.Type.IsSearch()` 的渠道；search 请求只匹配 search 渠道，非 search 请求只匹配非 search 渠道 |
| ✅ | Orchestrator | `internal/server/orchestrator/orchestrator.go` | Add `NewSearchOrchestrator` constructor,精简 middleware chain (only `selectCandidates` + `persistRequest`) |
| ✅ | API Handler | `internal/server/api/openai.go` | Add `SearchHandlers *ChatCompletionHandlers` field, initialize with `NewSearchOrchestrator` + `openai.NewSearchInboundTransformer()` |
| ✅ | API Handler | `internal/server/api/openai.go` | Add `Search(c *gin.Context)` method delegating to `SearchHandlers.ChatCompletion(c)` |
| ✅ | Routes | `internal/server/routes.go` | Add `openaiGroup.POST("/search", handlers.OpenAI.Search)` |

**`NewSearchOrchestrator` vs `NewChatCompletionOrchestrator` 差异**：
```go
func NewSearchOrchestrator(
    channelService *biz.ChannelService,
    modelService *biz.ModelService,
    requestService *biz.RequestService,
    httpClient *httpclient.HttpClient,
    inbound transformer.Inbound,
    systemService *biz.SystemService,
    usageLogService *biz.UsageLogService,
) *ChatCompletionOrchestrator {
    // 与 NewChatCompletionOrchestrator 相同的 LB / circuit breaker 初始化
    // 但不需要 PromptService 和 QuotaService
    // ...
}
```

在 `Process` 方法中，根据是否为 Search 请求组装不同的 middleware chain。或者更简洁的方案：新增一个 `skipMiddlewares` 选项，让 Search orchestrator 跳过 `enforceQuota`、`checkApiKeyModelAccess`、`applyModelMapping`、`injectPrompts`。

#### Phase 4: Frontend - Data & Config

| Status | Layer | File | Change |
|---|---|---|---|
| ✅ | Schema enum | `frontend/src/features/channels/data/schema.ts` | Add `search_tavily`, `search_brave`, `search_exa` to `channelTypeSchema` enum |
| ✅ | Schema enum | `frontend/src/features/channels/data/schema.ts` | Add `tavily/search`, `brave/search`, `exa/search` to `apiFormatSchema` enum |
| ✅ | Channel Config | `frontend/src/features/channels/data/config_channels.ts` | Add `search_tavily` / `search_brave` / `search_exa` entries to `CHANNEL_CONFIGS` |
| ✅ | Channel Config | `frontend/src/features/channels/data/config_channels.ts` | Add to `CHANNEL_TYPE_TO_PROVIDER` mapping |
| ✅ | Channel Config | `frontend/src/features/channels/data/config_channels.ts` | Add `Provider` type union member: `'tavily'`, `'brave_search'`, `'exa'` |
| ✅ | Provider Config | `frontend/src/features/channels/data/config_providers.ts` | Add `tavily` / `brave_search` / `exa` entries mapping to `search_tavily` / `search_brave` / `search_exa` |
| ✅ | i18n EN | `frontend/src/locales/en/channels.json` | Add `channels.types.search_*` and provider keys |
| ✅ | i18n ZH | `frontend/src/locales/zh-CN/channels.json` | Add 对应中文翻译 |

#### Phase 5: Frontend - UI Components

| Status | Layer | File | Change |
|---|---|---|---|
| ✅ | Search Dialog | `frontend/src/features/channels/components/search-channel-dialog.tsx` (new) | 独立的 Search 渠道创建/编辑 Dialog，精简字段，自动设置 models |
| ✅ | Row Actions | `frontend/src/features/channels/components/channels-columns.tsx` | Search 渠道隐藏 Model Mapping / Model Price / Transform Options；Edit/Duplicate 打开 Search dialog |
| ✅ | Dialogs Orchestration | `frontend/src/features/channels/components/channels-dialogs.tsx` | 注册 `editSearch` / `addSearch` / `duplicateSearch` dialog type |
| ✅ | Toolbar | `frontend/src/features/channels/components/channels-primary-buttons.tsx` | 新增 "Add Search Channel" 按钮 |
| ✅ | Channel Test | `internal/server/orchestrator/tester.go` | `testChannel` 对 search_* 渠道发送 `/v1/search` 测试请求 |

**Frontend Channel Config 详情**:

```typescript
// config_channels.ts
import { Tavily, BraveSearch } from '@lobehub/icons'; // 或自定义 icon 组件

// 新增 API Format 常量
export const TAVILY_SEARCH: ApiFormat = 'tavily/search';
export const BRAVE_SEARCH: ApiFormat = 'brave/search';

// CHANNEL_CONFIGS 新增
search_tavily: {
  channelType: 'search_tavily',
  baseURL: 'https://api.tavily.com',
  defaultModels: ['__search', '__tavily_search'],
  apiFormat: TAVILY_SEARCH,
  color: 'bg-blue-100 text-blue-800 border-blue-200',
  icon: TavilyIcon, // 自定义 icon 或通用 Search icon
},
search_brave: {
  channelType: 'search_brave',
  baseURL: 'https://api.search.brave.com',
  defaultModels: ['__search', '__brave_search'],
  apiFormat: BRAVE_SEARCH,
  color: 'bg-orange-100 text-orange-800 border-orange-200',
  icon: BraveIcon,
},

// CHANNEL_TYPE_TO_PROVIDER 新增
search_tavily: 'search_tavily',
search_brave: 'search_brave',

// Provider type 新增
| 'search_tavily'
| 'search_brave'
```

```typescript
// config_providers.ts
search_tavily: {
  provider: 'search_tavily',
  icon: TavilyIcon,
  color: 'bg-blue-100 text-blue-800 border-blue-200',
  channelTypes: ['search_tavily'],
},
search_brave: {
  provider: 'search_brave',
  icon: BraveIcon,
  color: 'bg-orange-100 text-orange-800 border-orange-200',
  channelTypes: ['search_brave'],
},
```

**与普通渠道前端的差异**：
- 创建/编辑渠道的 UI 表单完全复用，无需特殊处理
- `defaultModels` 使用 `__search` 前缀的模型名，前端会自动填充到 Supported Models 字段
- 如果未来需要隐藏某些不适用于 search 的设置项（如 Stream Policy），可通过 `channelType.startsWith('search_')` 判断

### Middleware Compatibility

| Middleware | Search 是否使用 | 说明 |
|---|---|---|
| `enforceQuota` | ❌ 跳过 | Search 不走 API Key 配额 |
| `checkApiKeyModelAccess` | ❌ 跳过 | Search model 不受 API Key 访问限制 |
| `applyModelMapping` | ❌ 跳过 | Search model 使用固定后缀约定 |
| `selectCandidates` | ✅ 保留 | 核心能力：选择 search channel |
| `injectPrompts` | ❌ 跳过 | Search 不涉及 system prompt |
| `persistRequest` | ✅ 保留 | 记录搜索请求 |
| `applyOverrideRequestBody/Headers` | ✅ 保留 | 可覆盖请求参数 |
| `withPerformanceRecording` | ✅ 保留 | 记录搜索延迟 |
| `persistRequestExecution` | ✅ 保留 | 记录执行详情 |
| `withConnectionTracking` | ✅ 保留 | 追踪连接数 |
| `withModelCircuitBreaker` | ✅ 保留 | 对故障 provider 熔断 |

### Provider-Specific Mapping

#### Tavily Outbound

```
SearchRequest.query             → body.query
SearchRequest.max_results       → body.max_results
SearchRequest.allowed_domains   → body.include_domains
SearchRequest.blocked_domains   → body.exclude_domains
SearchRequest.extra_body.*      → body.* (透传 search_depth, topic, include_answer 等)

Authorization: Bearer <api_key>
POST https://api.tavily.com/search
```

#### Brave Outbound

```
SearchRequest.query             → query param: q
SearchRequest.max_results       → query param: count
SearchRequest.allowed_domains   → 拼接 "site:domain1 OR site:domain2" 追加到 q
SearchRequest.blocked_domains   → 拼接 "-site:domain" 追加到 q
SearchRequest.extra_body.*      → 对应 query params (country, freshness 等)

X-Subscription-Token: <api_key>
GET https://api.search.brave.com/res/v1/web/search?q=...&count=...
```

#### Exa Outbound

```
SearchRequest.query             → body.query
SearchRequest.max_results       → body.numResults
SearchRequest.allowed_domains   → body.includeDomains
SearchRequest.blocked_domains   → body.excludeDomains
SearchRequest.extra_body.*      → body.* (透传 type, contents.text, startPublishedDate 等)

Authorization: Bearer <api_key>
POST https://api.exa.ai/search
```

## Implementation Priority

1. **P0**: `llm/search.go` + `llm/constants.go` + `llm/model.go` — 定义核心数据结构
2. **P0**: Tavily outbound transformer (`llm/transformer/tavily/`) — 最常用的 LLM search provider
3. **P1**: Search inbound transformer + `NewSearchOrchestrator` + API handler + routes — 端到端打通
4. **P1**: Ent schema + `IsSearch()` + `buildChannelWithTransformer` wiring + Models API / Association 隔离 — 渠道注册与隔离
5. **P1**: Brave outbound transformer (`llm/transformer/brave/`) — 独立索引，互补 Tavily
6. **P2**: Frontend data & config (schema + config_channels + config_providers + i18n)
7. **P2**: Frontend UI (search-channel-dialog + row actions + toolbar + test dialog)
8. **P3**: Exa / Serper outbound transformers — 更多 provider

## References

- [Tavily API Docs](https://docs.tavily.com/documentation/api-reference/endpoint/search)
- [Exa API Docs](https://docs.exa.ai/reference/search)
- [Brave Search API Docs](https://api-dashboard.search.brave.com/app/documentation/web-search/get-started)
- [Serper API](https://serper.dev/)
- [OpenAI Web Search Tool](https://developers.openai.com/api/docs/guides/tools-web-search/)
