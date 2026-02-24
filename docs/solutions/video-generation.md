# Video Generation 支持方案

## 1. 背景

AxonHub 需要支持视频生成能力，接入 OpenAI Sora 和火山引擎 Seedance 两个视频生成服务。与现有的 chat/embedding/image 等同步请求不同，视频生成是**异步任务**模型：客户端提交任务后获得 task ID，再通过轮询或回调获取最终结果。

## 2. 供应商 API 对比

### 2.1 OpenAI Sora

- 创建: `POST /v1/videos`
- 查询: `GET /v1/videos/{id}` (推测，文档只提供 Create)
- 模型: `sora-2`, `sora-2-pro`
- 参数: `prompt`, `input_reference` (可选图片引用), `seconds` (4/8/12), `size` (720x1280 等), `model`
- 状态: `queued` → `in_progress` → `completed` / `failed`
- 返回: Video 对象包含 `id`, `status`, `progress`, `prompt`, `seconds`, `size`, `model`, `error`, `completed_at`, `expires_at`

### 2.2 火山引擎 Seedance

- 创建: `POST /api/v3/contents/generations/tasks`
- 查询: `GET /api/v3/contents/generations/tasks/{id}`
- 列表: `GET /api/v3/contents/generations/tasks?page_size=N&filter.status=X`
- 删除/取消: `DELETE /api/v3/contents/generations/tasks/{id}`
- 模型: `doubao-seedance-1-5-pro-251215`, `doubao-seedance-1-0-pro-250528`, `doubao-seedance-1-0-pro-fast-251015`, `doubao-seedance-1-0-lite-*`
- 参数:
  - `content[]`: 包含 text (prompt) 和 image_url (首帧/尾帧/参考图)
  - `content[].role`: `first_frame` / `last_frame` / `reference_image` (图片角色)
  - `ratio`: 16:9, 4:3, 1:1, 9:16 等
  - `duration`: 秒数 (Seedance 1.5 pro: 4-12, 其余: 2-12)
  - `resolution`: 480p / 720p / 1080p
  - `generate_audio`: 是否生成音频 (仅 Seedance 1.5 pro)
  - `seed`, `camera_fixed`, `watermark`, `draft`
  - `service_tier`: `default` / `flex` (离线推理)
- 状态: `queued` → `running` → `succeeded` / `failed`
- 返回: `id`, `status`, `content.video_url`, `usage`, `resolution`, `ratio`, `duration`, `framespersecond`, `seed`

### 2.3 关键差异

- OpenAI 用 `prompt` + `input_reference`; Seedance 用 `content[]` 数组 (支持多图 + role)
- OpenAI 用 `size` (WxH); Seedance 用 `ratio` + `resolution`
- Seedance 支持音频生成、首尾帧、参考图、draft 模式
- 状态值不同: OpenAI `in_progress`/`completed` vs Seedance `running`/`succeeded`

## 3. 统一数据模型

### 3.1 新增 RequestType 和 APIFormat

```go path=null start=null
// llm/constants.go
const (
    RequestTypeVideo RequestType = "video"
)

const (
    APIFormatOpenAIVideo    APIFormat = "openai/video"
    APIFormatSeedanceVideo  APIFormat = "seedance/video"
)
```

### 3.2 VideoRequest / VideoResponse

在 `llm/` 包新增 `video.go`。

设计原则: 以 Seedance 的 `content[]` 数组模型为基础（扩展性更强，支持多图+角色），同时兼容 OpenAI 的 `size` 等参数。所有供应商参数均为一等公民，不使用 `extra_body`。

```go path=null start=null
// VideoRequest 统一视频生成请求 (基于 Seedance content[] 模型)
type VideoRequest struct {
    // Model 模型 ID
    Model string `json:"model"`

    // Content 内容列表，包含文本提示词和图片输入
    // 基于 Seedance 的 content[] 数组设计:
    // - type="text": 文本提示词
    // - type="image_url": 图片输入 (首帧/尾帧/参考图)
    Content []VideoContent `json:"content"`

    // --- 视频规格参数 (覆盖 Seedance + OpenAI) ---

    // Duration 视频时长(秒)
    // Seedance: 2-12s; OpenAI: 4/8/12
    Duration *int64 `json:"duration,omitempty"`

    // Ratio 宽高比 (Seedance 原生)
    // 例: "16:9", "4:3", "1:1", "3:4", "9:16", "21:9", "adaptive"
    Ratio string `json:"ratio,omitempty"`

    // Resolution 分辨率等级 (Seedance 原生)
    // 例: "480p", "720p", "1080p"
    Resolution string `json:"resolution,omitempty"`

    // Size OpenAI 格式的分辨率 (例: "1280x720", "720x1280")
    // 当 Ratio/Resolution 未设置时，outbound transformer 可从 Size 推导
    Size string `json:"size,omitempty"`

    // Frames 视频帧数 (Seedance: 与 duration 二选一)
    // 支持 [29, 289] 区间内满足 25 + 4n 格式的整数值
    Frames *int64 `json:"frames,omitempty"`

    // Seed 随机种子
    Seed *int64 `json:"seed,omitempty"`

    // --- Seedance 特有参数 ---

    // GenerateAudio 是否生成音频 (Seedance 1.5 pro 支持)
    GenerateAudio *bool `json:"generate_audio,omitempty"`

    // CameraFixed 是否固定摄像头
    CameraFixed *bool `json:"camera_fixed,omitempty"`

    // Watermark 是否添加水印
    Watermark *bool `json:"watermark,omitempty"`

    // Draft 是否为预览模式 (Seedance 1.5 pro)
    Draft *bool `json:"draft,omitempty"`

    // ServiceTier 服务层级
    // Seedance: "default" (在线推理) / "flex" (离线推理, 50% 价格)
    ServiceTier string `json:"service_tier,omitempty"`

    // ExecutionExpiresAfter 任务超时时间(秒), 配合 flex 模式使用
    ExecutionExpiresAfter *int64 `json:"execution_expires_after,omitempty"`
}

// VideoContent 视频内容项 (基于 Seedance content[] 设计)
type VideoContent struct {
    // Type 内容类型: "text" 或 "image_url"
    Type string `json:"type"`

    // Text 文本提示词 (当 Type="text" 时)
    Text string `json:"text,omitempty"`

    // ImageURL 图片地址 (当 Type="image_url" 时)
    ImageURL *VideoImageURL `json:"image_url,omitempty"`

    // Role 图片角色 (当 Type="image_url" 时)
    // 可选值: "first_frame", "last_frame", "reference_image"
    // 不设置时默认为首帧
    Role string `json:"role,omitempty"`
}

// VideoImageURL 视频图片地址
type VideoImageURL struct {
    URL string `json:"url"`
}

// VideoResponse 统一视频任务响应 (创建/查询共用)
type VideoResponse struct {
    // ID 任务 ID
    ID string `json:"id"`

    // Status 任务状态: "queued", "running", "succeeded", "failed"
    Status string `json:"status"`

    // VideoURL 生成的视频下载链接
    VideoURL string `json:"video_url,omitempty"`

    // Progress 完成进度 (0-100)
    Progress *float64 `json:"progress,omitempty"`

    // Model 使用的模型
    Model string `json:"model,omitempty"`

    // Prompt 使用的提示词
    Prompt string `json:"prompt,omitempty"`

    // Duration 视频时长(秒)
    Duration *int64 `json:"duration,omitempty"`

    // Size 视频分辨率
    Size string `json:"size,omitempty"`

    // Ratio 宽高比
    Ratio string `json:"ratio,omitempty"`

    // Resolution 分辨率等级
    Resolution string `json:"resolution,omitempty"`

    // FPS 帧率
    FPS *int64 `json:"fps,omitempty"`

    // Seed 使用的种子值
    Seed *int64 `json:"seed,omitempty"`

    // Usage token 用量
    Usage *VideoUsage `json:"usage,omitempty"`

    // Error 错误信息
    Error *VideoError `json:"error,omitempty"`

    // CreatedAt 创建时间 (unix timestamp)
    CreatedAt int64 `json:"created_at,omitempty"`

    // CompletedAt 完成时间 (unix timestamp)
    CompletedAt int64 `json:"completed_at,omitempty"`

    // ExpiresAt 资源过期时间 (unix timestamp)
    ExpiresAt int64 `json:"expires_at,omitempty"`
}

type VideoUsage struct {
    CompletionTokens int64 `json:"completion_tokens"`
    TotalTokens      int64 `json:"total_tokens"`
}

type VideoError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}
```

### 3.3 扩展 llm.Request / llm.Response

```go path=null start=null
// llm/model.go - 在 Request 中添加
type Request struct {
    // ... 已有字段 ...

    // Video 是视频请求, 当 RequestType == "video" 时设置
    Video *VideoRequest `json:"video,omitempty"`
}

// llm/model.go - 在 Response 中添加
type Response struct {
    // ... 已有字段 ...

    // Video 是视频响应, 当 RequestType == "video" 时设置
    Video *VideoResponse `json:"video,omitempty"`
}
```

## 4. 异步任务处理架构

视频生成与现有同步请求流程不同，需要引入**异步任务管理**。

### 4.1 方案: Proxy-Polling + Request external_id

复用现有 `Request` 表的 `external_id` 字段存储 provider 任务 ID，无需新建表。

**创建任务流程** (走正常 pipeline):

```
Client POST /v1/videos
  → Inbound Transformer 解析请求
  → Pipeline: channel 选择 → 负载均衡 → 鉴权 → persist request
  → Outbound Transformer 转换为 provider 格式
  → Provider 返回 task_id (e.g. "cgt-2025xxxx")
  → 将 provider task_id 存入 request.external_id
  → 对外返回 AxonHub request_id 作为任务 ID
```

**查询进度流程** (绕过 pipeline，直接转发):

```
Client GET /v1/videos/:id
  → Handler 提取 axonhub_request_id
  → DB 查 Request 表 → 获得 external_id (provider task_id) + channel_id
  → ChannelService.GetChannel(channel_id) → 获得 outbound transformer + httpClient
  → VideoOutbound.BuildGetTaskRequest(external_id) → 构造 HTTP 请求
  → httpClient.Do(request) → provider 响应
  → VideoOutbound.TransformGetTaskResponse(response) → llm.VideoResponse
  → Inbound Transformer 转为对应格式 (OpenAI/Seedance) 返回客户端
```

**删除/取消流程** (同查询类似):

```
Client DELETE /v1/videos/:id
  → DB 查 Request 表 → 获得 external_id + channel_id
  → VideoOutbound.BuildDeleteTaskRequest(external_id)
  → 转发到 provider
```

### 4.2 复用现有 Request 表

现有 `Request` schema 已具备所需字段，无需新建表:

- `request.external_id` → 存储 provider 的原始任务 ID (e.g. `cgt-2025xxxx`, Sora video id)
- `request.channel_id` → 确定下游 provider channel
- `request.model_id` → 请求的模型
- `request.status` → 任务状态 (pending/processing/completed/failed)
- `request.id` → 对外暴露的 AxonHub 任务 ID

`UpdateRequestCompleted` 和 `UpdateRequestExecutionCompleted` 已支持写入 `external_id`，创建任务完成时自然写入 provider task_id。

### 4.3 任务 ID 策略

- 对外: 使用 AxonHub 的 `request.id` (或其 GUID)
- 内部: 通过 `request.external_id` 查找 provider 原始 task_id
- 查询时根据 `request.id` + `request.channel_id` 定位 channel 并转发

## 5. API 设计

参考现有路由分组模式 (OpenAI `/v1/`, Anthropic `/anthropic/v1/`)，视频 API 也分为两套独立入口，path 和请求/响应格式各自对应原生 provider。

### 5.1 OpenAI Sora API (`/v1/videos`)

路径和格式完全兼容 OpenAI Video API:

```
POST   /v1/videos           # 创建视频生成任务
GET    /v1/videos/:id       # 查询任务状态和结果
DELETE /v1/videos/:id       # 取消/删除任务
```

#### 创建任务请求

```json path=null start=null
{
  "model": "sora-2",
  "prompt": "A cat walking on the beach at sunset",
  "input_reference": "https://example.com/image.png",
  "seconds": 8,
  "size": "1280x720"
}
```

#### 响应格式 (创建/查询共用)

```json path=null start=null
{
  "id": "vid_20260223_a1b2c3",
  "object": "video",
  "status": "queued",
  "model": "sora-2",
  "prompt": "A cat walking on the beach at sunset",
  "seconds": 8,
  "size": "1280x720",
  "progress": 0,
  "created_at": 1740278400,
  "completed_at": null,
  "expires_at": null,
  "error": null
}
```

#### 完成后响应

```json path=null start=null
{
  "id": "vid_20260223_a1b2c3",
  "object": "video",
  "status": "completed",
  "model": "sora-2",
  "prompt": "A cat walking on the beach at sunset",
  "seconds": 8,
  "size": "1280x720",
  "progress": 100,
  "video_url": "https://...",
  "created_at": 1740278400,
  "completed_at": 1740278500,
  "expires_at": 1740364900,
  "usage": {
    "completion_tokens": 246840,
    "total_tokens": 246840
  }
}
```

Inbound transformer 将 OpenAI 格式转换为统一 `VideoRequest`:
- `prompt` → `Content[]{type:"text"}`
- `input_reference` → `Content[]{type:"image_url", role:"first_frame"}`
- `seconds` → `Duration`
- `size` → `Size`
- 状态映射: 对外 `completed`/`in_progress` ↔ 内部 `succeeded`/`running`

### 5.2 Seedance API (`/seedance/v3/contents/generations/tasks`)

路径和格式对齐火山引擎 Ark API:

```
POST   /seedance/v3/contents/generations/tasks           # 创建视频生成任务
GET    /seedance/v3/contents/generations/tasks/:id       # 查询任务状态和结果
GET    /seedance/v3/contents/generations/tasks            # 查询任务列表
DELETE /seedance/v3/contents/generations/tasks/:id       # 取消/删除任务
```

#### 创建任务请求

```json path=null start=null
{
  "model": "doubao-seedance-1-5-pro-251215",
  "content": [
    {
      "type": "text",
      "text": "女孩抱着狐狸，女孩睁开眼，温柔地看向镜头"
    },
    {
      "type": "image_url",
      "image_url": {"url": "https://example.com/first.png"},
      "role": "first_frame"
    },
    {
      "type": "image_url",
      "image_url": {"url": "https://example.com/last.png"},
      "role": "last_frame"
    }
  ],
  "duration": 5,
  "ratio": "16:9",
  "resolution": "720p",
  "generate_audio": true,
  "camera_fixed": false,
  "watermark": false,
  "seed": 42,
  "service_tier": "default"
}
```

#### 创建任务响应

```json path=null start=null
{
  "id": "vid_20260223_a1b2c3"
}
```

#### 查询任务响应 (完成后)

```json path=null start=null
{
  "id": "vid_20260223_a1b2c3",
  "model": "doubao-seedance-1-5-pro-251215",
  "status": "succeeded",
  "content": {
    "video_url": "https://..."
  },
  "usage": {
    "completion_tokens": 246840,
    "total_tokens": 246840
  },
  "created_at": 1740278400,
  "updated_at": 1740278500,
  "seed": 42,
  "resolution": "720p",
  "ratio": "16:9",
  "duration": 5,
  "framespersecond": 24,
  "service_tier": "default"
}
```

Inbound transformer 将 Seedance 格式直接映射为统一 `VideoRequest` (几乎 1:1):
- `content[]` → `Content[]`
- 状态值直接透传 (`succeeded`/`running`/`queued`/`failed`)

### 5.3 路由对照总结

```
# OpenAI Sora 兼容 (类似现有 /v1/chat/completions)
/v1/videos                    → OpenAI Video Inbound Transformer

# Seedance 兼容 (类似现有 /anthropic/v1/messages)
/seedance/v3/contents/generations/tasks  → Seedance Video Inbound Transformer
```

两套 inbound transformer 将各自原生格式统一转换为 `llm.VideoRequest`，再由 outbound transformer 转发到实际 provider。

## 6. Transformer 实现

### 6.1 Inbound Transformers (两套入口)

#### OpenAI Video Inbound

`llm/transformer/openai/video_inbound.go`:

- `TransformRequest`: 解析 OpenAI 原生格式 (`prompt`, `input_reference`, `seconds`, `size`) → `llm.Request` (RequestType=video)
  - `prompt` → `Content[]{type:"text"}`
  - `input_reference` → `Content[]{type:"image_url", role:"first_frame"}`
  - `seconds` → `Duration`
- `TransformResponse`: `llm.VideoResponse` → OpenAI Video 格式 HTTP 响应
  - 状态映射: `succeeded` → `completed`, `running` → `in_progress`
  - `Duration` → `seconds`

#### Seedance Video Inbound

`llm/transformer/doubao/video_inbound.go`:

- `TransformRequest`: 解析 Seedance 原生格式 (`content[]`, `ratio`, `resolution` 等) → `llm.Request` (RequestType=video)
  - 几乎 1:1 映射，直接对应 `llm.VideoRequest` 字段
- `TransformResponse`: `llm.VideoResponse` → Seedance 格式 HTTP 响应
  - 状态值直接透传 (`succeeded`/`running`)
  - 响应结构对齐 Ark API (content.video_url, framespersecond 等)

### 6.2 Outbound Transformers

#### OpenAI (Sora)

`llm/transformer/openai/video_outbound.go`:

- `TransformRequest`: `llm.Request` → `POST /v1/videos` (OpenAI format)
  - `Content[]` text → `prompt`
  - `Content[]` image_url → `input_reference`
  - `Duration` → `seconds`
  - `Size` 直传; 如果只有 `Ratio`+`Resolution` 则推导为 `Size`
- `TransformResponse`: OpenAI Video 响应 → `llm.Response` (含 VideoResponse)

#### Doubao (Seedance)

`llm/transformer/doubao/video_outbound.go`:

- `TransformRequest`: `llm.Request` → `POST /contents/generations/tasks` (Seedance format)
  - `Content[]` 几乎直传
  - 如果只有 `Size`，推导为 `ratio` + `resolution`
  - 传递全部 Seedance 参数 (generate_audio, camera_fixed, draft, service_tier 等)
- `TransformResponse`: Seedance 响应 → `llm.Response` (含 VideoResponse)

### 6.3 VideoOutbound 扩展接口

查询和删除不经过标准 pipeline，采用 **Transformer 构造请求 + Service 使用 channel.HTTPClient 执行 + Transformer 解析响应** 的方式。

```go path=null start=null
// VideoTaskOutbound 扩展 Outbound，提供视频任务查询/删除能力（仅负责 Build/Parse）
type VideoTaskOutbound interface {
    BuildGetVideoTaskRequest(ctx context.Context, providerTaskID string) (*httpclient.Request, error)
    ParseGetVideoTaskResponse(ctx context.Context, httpResp *httpclient.Response) (*llm.VideoResponse, error)

    BuildDeleteVideoTaskRequest(ctx context.Context, providerTaskID string) (*httpclient.Request, error)
}
```

Handler 通过类型断言使用:

```go path=null start=null
videoOutbound, ok := ch.Outbound.(transformer.VideoTaskOutbound)
if !ok {
    return fmt.Errorf("channel does not support video operations")
}
httpReq, _ := videoOutbound.BuildGetVideoTaskRequest(ctx, req.ExternalID)
httpResp, _ := ch.HTTPClient.Do(ctx, httpReq)
resp, err := videoOutbound.ParseGetVideoTaskResponse(ctx, httpResp)
```

## 7. Handler 和路由

### 7.1 新增 Handler

参考现有 `OpenAIHandlers` / `AnthropicHandlers` 模式，分别创建:

`internal/server/api/video_openai.go`:

```go path=null start=null
// OpenAIVideoHandlers 处理 /v1/videos 路由
type OpenAIVideoHandlers struct {
    VideoService *biz.VideoService
    // 创建任务使用 OpenAI Video Inbound Transformer
    CreateOrchestrator *orchestrator.ChatCompletionOrchestrator
}

func (h *OpenAIVideoHandlers) CreateVideo(c *gin.Context)  { /* ... */ }
func (h *OpenAIVideoHandlers) GetVideo(c *gin.Context)     { /* ... */ }
func (h *OpenAIVideoHandlers) DeleteVideo(c *gin.Context)   { /* ... */ }
```

`internal/server/api/video_seedance.go`:

```go path=null start=null
// SeedanceVideoHandlers 处理 /seedance/v3/contents/generations/tasks 路由
type SeedanceVideoHandlers struct {
    VideoService *biz.VideoService
    // 创建任务使用 Seedance Video Inbound Transformer
    CreateOrchestrator *orchestrator.ChatCompletionOrchestrator
}

func (h *SeedanceVideoHandlers) CreateTask(c *gin.Context)  { /* ... */ }
func (h *SeedanceVideoHandlers) GetTask(c *gin.Context)     { /* ... */ }
func (h *SeedanceVideoHandlers) ListTasks(c *gin.Context)   { /* ... */ }
func (h *SeedanceVideoHandlers) DeleteTask(c *gin.Context)  { /* ... */ }
```

### 7.2 路由注册

`internal/server/routes.go` — 参考现有 openaiGroup / anthropicGroup 分组模式:

```go path=null start=null
// OpenAI Video API (类似 /v1/chat/completions)
openaiGroup.POST("/videos", handlers.OpenAIVideo.CreateVideo)
openaiGroup.GET("/videos/:id", handlers.OpenAIVideo.GetVideo)
openaiGroup.DELETE("/videos/:id", handlers.OpenAIVideo.DeleteVideo)

// Seedance Video API (类似 /anthropic/v1/messages)
seedanceGroup := apiGroup.Group("/seedance/v3")
seedanceGroup.POST("/contents/generations/tasks", handlers.SeedanceVideo.CreateTask)
seedanceGroup.GET("/contents/generations/tasks/:id", handlers.SeedanceVideo.GetTask)
seedanceGroup.GET("/contents/generations/tasks", handlers.SeedanceVideo.ListTasks)
seedanceGroup.DELETE("/contents/generations/tasks/:id", handlers.SeedanceVideo.DeleteTask)
```

### 7.3 VideoService

`internal/server/biz/video.go`:

- `CreateTask()`: 创建任务映射记录, 通过 pipeline 调用下游 provider
- `GetTask()`: 根据 task_id 查 DB 获取 channel, 转发查询到下游 provider
- `DeleteTask()`: 根据 task_id 查 DB 获取 channel, 转发删除到下游 provider
- `UpdateTaskStatus()`: 更新本地任务状态缓存

## 8. Doubao OutboundTransformer 扩展

在现有 `llm/transformer/doubao/outbound.go` 的 `TransformRequest` switch 中新增:

```go path=null start=null
case llm.RequestTypeVideo:
    return t.buildVideoGenerationAPIRequest(ctx, llmReq)
```

新增 `llm/transformer/doubao/video_outbound.go` 实现具体转换逻辑。

## 9. 实现步骤

1. **Phase 1: 数据模型** — 新增 `llm/video.go`, 扩展 constants.go, model.go
2. **Phase 2: Outbound Transformers** — 实现 Doubao + OpenAI 的视频 outbound (VideoOutbound 接口)
3. **Phase 3: Inbound Transformers** — 实现 OpenAI Video + Seedance Video inbound
4. **Phase 4: Service + Handler** — VideoService, Handler, 路由注册
5. **Phase 5: 前端** — Channel 配置中标记支持视频, 管理界面 (可后续迭代)

## 10. 状态映射表

统一对外使用 OpenAI 风格状态:

| 统一状态 (对外) | OpenAI Sora | Seedance |
|---|---|---|
| queued | queued | queued |
| in_progress | in_progress | running |
| completed | completed | succeeded |
| failed | failed | failed |

## 11. 后续优化方案

MVP 采用透传模式 (客户端轮询)，后续迭代为服务端主动轮询 + 事件通知模式。

### 11.1 服务端主动轮询

MVP 中每次查询都透传到 provider，后续改为 AxonHub 后台 worker 主动轮询:

```
创建任务 → request.status = processing
              ↓
         VideoPoller (后台 goroutine)
              ↓
         定时轮询 provider (间隔递增: 5s → 10s → 30s → 60s)
              ↓
         任务完成 → 更新 request.status = completed
                       → 存储 video_url 到 request.response_body
                       → 触发 Webhook 通知 (如配置)
```

优势:
- 客户端查询直接返回本地缓存状态，无需每次透传
- 减少对 provider API 的请求压力
- 支持任务完成后主动通知客户端

### 11.2 Webhook 通知

任务完成后主动回调客户端:

- 客户端创建任务时可指定 `webhook_url`
- AxonHub 检测到任务完成后，POST 结果到 `webhook_url`
- 支持重试和签名验证

### 11.4 视频托管

- Provider 返回的 video_url 通常有过期时间
- AxonHub 可通过「视频存储(Video Storage)」定时扫描已完成的视频请求，将视频下载并保存到非数据库存储（FS/S3/GCS/WebDAV），避免链接过期
- 去重方式：使用 `Request.content_saved` / `Request.content_storage_id` / `Request.content_storage_key` / `Request.content_saved_at` 标记已落盘的任务；worker 每次只扫描 `content_saved=false` 的记录，保存成功后置为 `true`，从而不会重复下载
- 扫描范围：仅处理 `format in (openai/video, seedance/video)` 且 `status in (processing, completed)` 的请求；如果 `response_body` 里已缓存 `video_url` 则直接下载，否则最多向下游 provider 查询一次刷新快照，再按 `status=succeeded` 决定是否下载
- 对外提供持久化的下载链接
