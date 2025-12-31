# AxonHub 实体关系图 (ERD)

## 概述

AxonHub 采用多层级的权限管理架构，支持 Global（全局）和 Project（项目）两个层级。系统通过 RBAC（基于角色的访问控制）模型管理用户权限，实现细粒度的资源访问控制。

---

## 核心概念

### 层级结构

- **Global Level（全局层级）**：系统级别的配置和资源，所有 Project 共享
- **Project Level（项目层级）**：项目级别的资源，属于特定 Project，但全局也可见

### 权限模型

- **Owner（所有者）**：拥有所有权限，可以管理所有资源
- **Custom Roles + Scopes（自定义角色 + 权限范围）**：通过角色和权限范围组合实现细粒度权限控制

---

## 实体详细说明

### 1. User（用户）

**描述**：系统用户实体，代表使用 AxonHub 的个人或服务账号。

**层级**：Global

**字段**：
- `id`: 用户唯一标识
- `email`: 用户邮箱（唯一）
- `status`: 用户状态（activated/deactivated）
- `prefer_language`: 用户偏好语言
- `password`: 密码（敏感字段）
- `first_name`: 名字
- `last_name`: 姓氏
- `avatar`: 用户头像 URL
- `is_owner`: 是否为系统所有者
- `scopes`: 用户特定权限范围（如 write_channels, read_channels, add_users, read_users 等）
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间

**权限**：
- Global Owner：拥有所有权限
- Custom Roles + Scopes：根据分配的角色和权限范围拥有指定权限

**关联关系**：
- 可以属于多个 Projects（通过 Project-User 关联）
- 可以拥有多个 Roles（角色）
- 可以创建多个 API Keys
- 可以创建多个 Channel Override Templates

---

### 2. Project（项目）

**描述**：项目实体，用于组织和隔离不同业务或团队的资源。

**层级**：Global（项目本身是全局管理的）

**字段**：
- `id`: 项目唯一标识
- `name`: 项目名称
- `description`: 项目描述
- `status`: 项目状态
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间

**权限**：
- Project Owner：拥有项目内所有权限
- Custom Roles + Scopes：根据项目内分配的角色和权限范围拥有指定权限

**关联关系**：
- 包含多个 Users（项目成员）
- 包含多个 Project-level Roles
- 包含多个 API Keys
- 包含多个 Threads
- 包含多个 Traces
- 包含多个 Requests
- 包含多个 Usage Logs

---

### 3. Model（模型）

**描述**：AI 模型定义，代表来自各个提供商的可用 AI 模型。

**层级**：Global（所有 Project 共享）

**字段**：
- `id`: 模型唯一标识
- `developer`: 模型开发者（如 deepseek, openai）- 不可变
- `model_id`: 模型标识符（如 deepseek-chat）- 不可变
- `type`: 模型类型（chat/embedding/rerank）- 不可变
- `name`: 模型名称（如 DeepSeek Chat）
- `icon`: 模型图标（来自 lobe-icons，如 DeepSeek）
- `group`: 模型分组（如 deepseek）
- `model_card`: 模型卡片信息（JSON）
- `settings`: 模型设置（JSON）
- `status`: 模型状态（enabled/disabled/archived）
- `remark`: 用户自定义备注
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间

**权限**：
- 需要 `read_channels` 权限读取
- 需要 `write_channels` 权限修改

**关联关系**：无直接关联

---

### 4. Channel（渠道）

**描述**：AI 服务提供商的接入渠道配置，如 OpenAI、Anthropic、Gemini 等。

**层级**：Global（所有 Project 共享）

**字段**：
- `id`: 渠道唯一标识
- `type`: 渠道类型（openai, anthropic, gemini_openai, deepseek 等）- 不可变
- `base_url`: API 基础 URL
- `name`: 渠道名称（唯一）
- `status`: 渠道状态（enabled/disabled/archived）
- `credentials`: 渠道凭证（敏感字段）
- `supported_models`: 支持的模型列表
- `auto_sync_supported_models`: 自动同步支持模型标志
- `default_test_model`: 默认测试模型
- `settings`: 渠道设置（包含模型映射等）
- `tags`: 渠道标签
- `ordering_weight`: 显示排序权重
- `error_message`: 错误信息（可选）
- `remark`: 用户自定义备注
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间

**权限**：
- 需要 `read_channels` 权限读取
- 需要 `write_channels` 权限修改

**关联关系**：
- 可以被多个 Requests 使用
- 可以被多个 Request Executions 使用
- 关联多个 Usage Logs
- 拥有一个 Channel Performance 记录

---

### 5. Channel Performance（渠道性能）

**描述**：渠道性能指标，跟踪成功率、延迟和吞吐量。

**层级**：Global（关联 Channel）

**字段**：
- `id`: 性能记录唯一标识
- `channel_id`: 关联的渠道 ID（唯一，不可变）
- `success_rate`: 成功率百分比
- `avg_latency_ms`: 平均延迟（毫秒）
- `avg_token_per_second`: 平均每秒 Token 数
- `avg_stream_first_token_latency_ms`: 流式请求平均首字延迟
- `avg_stream_token_per_second`: 流式请求平均每秒 Token 数
- `last_success_at`: 最后成功请求时间戳
- `last_failure_at`: 最后失败请求时间戳
- `request_count`: 总请求数
- `success_count`: 总成功请求数
- `failure_count`: 总失败请求数
- `total_token_count`: 所有请求的总 Token 数
- `total_request_latency_ms`: 总请求延迟（毫秒）
- `stream_success_count`: 总成功流式请求数
- `stream_total_request_count`: 总流式请求数
- `stream_total_token_count`: 流式请求总 Token 数
- `stream_total_request_latency_ms`: 流式请求总延迟（毫秒）
- `stream_total_first_token_latency_ms`: 流式请求总首字延迟
- `consecutive_failures`: 连续失败次数
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间

**权限**：继承自 Channel

**关联关系**：
- 属于一个 Channel

---

### 6. Channel Override Template（渠道覆盖模板）

**描述**：用户定义的模板，用于覆盖渠道请求参数和请求头。

**层级**：User（每个用户私有）

**字段**：
- `id`: 模板唯一标识
- `user_id`: 所有者用户 ID（不可变）
- `name`: 模板名称（对每个用户和渠道类型唯一）
- `description`: 模板描述
- `channel_type`: 模板适用的渠道类型
- `override_parameters`: 覆盖请求体参数（JSON 字符串）
- `override_headers`: 覆盖请求头（JSON 数组）
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间

**权限**：
- 用户只能访问自己的模板
- Owner 可以访问所有模板

**关联关系**：
- 属于一个 User

---

### 7. Data Storage（数据存储）

**描述**：数据存储配置，用于存储请求/响应数据（数据库、文件系统、S3、GCS）。

**层级**：Global（所有 Project 共享）

**字段**：
- `id`: 数据存储唯一标识
- `name`: 数据存储名称
- `description`: 数据存储描述
- `primary`: 是否为主数据存储（不可变）
- `type`: 数据存储类型（database/fs/s3/gcs）- 不可变
- `settings`: 数据存储设置（JSON）
- `status`: 数据存储状态（active/archived）
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间

**权限**：
- 需要 `read_data_storages` 权限读取
- 需要 `write_data_storages` 权限修改

**关联关系**：
- 可以存储多个 Requests
- 可以存储多个 Request Executions

---

### 8. System（系统配置）

**描述**：系统级别的配置项，如 Logo、系统名称等全局设置。

**层级**：Global（所有 Project 共享）

**字段**：
- `id`: 配置唯一标识
- `key`: 配置键（唯一）
- `value`: 配置值
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间

**权限**：
- 需要 `read_settings` 权限读取
- 需要 `write_settings` 权限修改

**关联关系**：无直接关联

---

### 9. Role（角色）

**描述**：用户角色定义，包含一组权限范围（Scopes）。

**层级**：可以是 Global 或 Project

**字段**：
- `id`: 角色唯一标识
- `name`: 角色名称
- `level`: 角色层级（global/project）- 不可变
- `project_id`: 项目 ID（可选，Project-level 角色必填）
- `scopes`: 角色包含的权限范围（如 write_channels, read_channels, add_users, read_users 等）
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间

**权限规则**：
- Global Role 只能配置 Global Scopes
- Project Role 可以配置 Global 和 Project Scopes

**关联关系**：
- 可以分配给多个 Users
- 属于一个 Project（Project-level 角色）

---

### 10. Scope（权限范围）

**描述**：细粒度的权限定义，如 `read_channels`、`write_requests` 等。

**层级**：可以是 Global、Project 或同时存在

**示例 Scopes**：
- `read_channels`: 读取渠道
- `write_channels`: 写入渠道
- `read_users`: 读取用户
- `write_users`: 写入用户
- `read_api_keys`: 读取 API Keys
- `write_api_keys`: 写入 API Keys
- `read_requests`: 读取请求
- `write_requests`: 写入请求
- `read_settings`: 读取系统设置
- `write_settings`: 写入系统设置
- `read_roles`: 读取角色
- `write_roles`: 写入角色
- `read_data_storages`: 读取数据存储
- `write_data_storages`: 写入数据存储

---

### 11. API Key（API 密钥）

**描述**：用于 API 认证的密钥，每个 API Key 属于特定用户和项目。

**层级**：Project

**字段**：
- `id`: API Key 唯一标识
- `user_id`: 所属用户 ID（不可变）
- `project_id`: 所属项目 ID（不可变）
- `key`: API 密钥（唯一，不可变）
- `name`: API Key 名称
- `status`: 状态（enabled/disabled/archived）
- `scopes`: API Key 特定权限范围（默认：read_channels, write_requests）
- `profiles`: API Key 配置文件（JSON）
- `created_at`: 创建时间
- `updated_at`: 更新时间
- `deleted_at`: 软删除时间

**权限**：
- 用户只能管理自己的 API Keys
- Owner 可以管理所有 API Keys

**关联关系**：
- 属于一个 User
- 属于一个 Project
- 可以发起多个 Requests

---

### 12. Thread（线程）

**描述**：线程实体，用于组织和追踪相关的 Trace 集合，实现请求链路的可观测性。

**层级**：Project

**字段**：
- `id`: 线程唯一标识
- `project_id`: 所属项目 ID
- `thread_id`: 线程追踪 ID（唯一）
- `created_at`: 创建时间
- `updated_at`: 更新时间

**权限**：
- 用户只能查看和管理项目内的 Threads
- Owner 可以查看和管理所有 Threads

**关联关系**：
- 属于一个 Project
- 包含多个 Traces

---

### 13. Trace（追踪）

**描述**：追踪实体，用于记录和追踪一组相关的 Request，实现分布式链路追踪。

**层级**：Project

**字段**：
- `id`: 追踪唯一标识
- `project_id`: 所属项目 ID
- `trace_id`: 追踪 ID（唯一）
- `thread_id`: 所属线程 ID（可选）
- `created_at`: 创建时间
- `updated_at`: 更新时间

**权限**：
- 用户只能查看和管理项目内的 Traces
- Owner 可以查看和管理所有 Traces

**关联关系**：
- 属于一个 Project
- 可选属于一个 Thread
- 包含多个 Requests

---

### 14. Request（请求）

**描述**：用户通过 API 或 Playground 发起的 AI 模型请求。

**层级**：Project

**字段**：
- `id`: 请求唯一标识
- `api_key_id`: API Key ID（可选，来自 Admin 的请求为空）
- `project_id`: 所属项目 ID
- `trace_id`: 所属追踪 ID（可选）
- `data_storage_id`: 数据存储 ID（可选）
- `source`: 请求来源（api/playground/test）- 不可变
- `model_id`: 模型标识
- `format`: 请求格式（如 openai/chat_completions, claude/messages）
- `request_body`: 原始请求体（用户格式）
- `response_body`: 最终响应体（用户格式）
- `response_chunks`: 流式响应块
- `channel_id`: 使用的渠道 ID
- `external_id`: 外部系统追踪 ID
- `status`: 请求状态（pending/processing/completed/failed/canceled）
- `stream`: 是否为流式请求
- `metrics_latency_ms`: 总延迟（毫秒）
- `metrics_first_token_latency_ms`: 首字延迟（毫秒）
- `created_at`: 创建时间
- `updated_at`: 更新时间

**权限**：
- 用户只能查看和管理自己的 Requests
- Owner 可以查看和管理所有 Requests

**关联关系**：
- 属于一个 Project
- 可选关联一个 API Key
- 可选关联一个 Trace
- 可选关联一个 Data Storage
- 可选关联一个 Channel
- 包含多个 Request Executions
- 关联多个 Usage Logs

---

### 15. Request Execution（请求执行）

**描述**：Request 在特定 Channel 上的实际执行记录，一个 Request 可能有多次执行（如重试、fallback）。

**层级**：Project（跟随 Request）

**字段**：
- `id`: 执行唯一标识
- `project_id`: 项目 ID
- `request_id`: 关联的请求 ID
- `channel_id`: 执行的渠道 ID
- `data_storage_id`: 数据存储 ID（可选）
- `external_id`: 外部系统追踪 ID
- `model_id`: 模型标识
- `format`: 请求格式
- `request_body`: 发送给提供商的请求体（提供商格式）
- `response_body`: 提供商返回的响应体（提供商格式）
- `response_chunks`: 流式响应块（提供商格式）
- `error_message`: 错误信息
- `status`: 执行状态（pending/processing/completed/failed/canceled）
- `metrics_latency_ms`: 总延迟（毫秒）
- `metrics_first_token_latency_ms`: 首字延迟（毫秒）
- `created_at`: 创建时间
- `updated_at`: 更新时间

**关联关系**：
- 属于一个 Request
- 使用一个 Channel
- 可选使用一个 Data Storage

---

### 16. Usage Log（使用日志）

**描述**：记录每个 Request 的 Token 使用情况和成本信息，用于统计和计费。

**层级**：Project

**字段**：
- `id`: 日志唯一标识
- `request_id`: 关联的请求 ID
- `project_id`: 所属项目 ID
- `channel_id`: 使用的渠道 ID
- `model_id`: 模型标识
- `prompt_tokens`: Prompt Token 数量
- `completion_tokens`: Completion Token 数量
- `total_tokens`: 总 Token 数量
- `prompt_audio_tokens`: Prompt 音频 Token 数量
- `prompt_cached_tokens`: Prompt 缓存 Token 数量
- `completion_audio_tokens`: Completion 音频 Token 数量
- `completion_reasoning_tokens`: Completion 推理 Token 数量
- `completion_accepted_prediction_tokens`: 接受的预测 Token 数量
- `completion_rejected_prediction_tokens`: 拒绝的预测 Token 数量
- `source`: 请求来源（api/playground/test）
- `format`: 请求格式
- `created_at`: 创建时间
- `updated_at`: 更新时间

**权限**：
- 用户只能查看自己的 Usage Logs
- Owner 可以查看所有 Usage Logs

**关联关系**：
- 属于一个 Project
- 关联一个 Request
- 可选关联一个 Channel

---

## 实体关系图

### Mermaid ERD

```mermaid
erDiagram
    AxonHub ||--o{ User : "manages"
    AxonHub ||--o{ Project : "manages"
    AxonHub ||--o{ Model : "defines"
    AxonHub ||--o{ Channel : "manages"
    AxonHub ||--o{ System : "configures"
    AxonHub ||--o{ Role : "defines"
    AxonHub ||--o{ DataStorage : "configures"

    User ||--o{ ProjectUser : "belongs to"
    Project ||--o{ ProjectUser : "has"
    User ||--o{ UserRole : "has"
    Role ||--o{ UserRole : "assigned to"

    Project ||--o{ Role : "contains"
    Project ||--o{ APIKey : "contains"
    Project ||--o{ Thread : "contains"
    Project ||--o{ Trace : "contains"
    Thread ||--o{ Trace : "contains"
    Trace ||--o{ Request : "contains"

    APIKey }o--|| Project : "belongs to"
    APIKey ||--o{ Request : "authenticates"

    Request }o--|| Project : "belongs to"
    Request }o--o| APIKey : "uses"
    Request }o--o| Trace : "belongs to"
    Request }o--o| Channel : "routes to"
    Request }o--o| DataStorage : "stored in"
    Request ||--o{ RequestExecution : "has"
    Request ||--o{ UsageLog : "generates"

    RequestExecution }o--|| Request : "executes"
    RequestExecution }o--|| Channel : "uses"
    RequestExecution }o--o| DataStorage : "stored in"

    Channel ||--o{ Request : "processes"
    Channel ||--o{ RequestExecution : "executes"
    Channel ||--o{ UsageLog : "tracks"
    Channel ||--|| ChannelPerformance : "has"

    UsageLog }o--|| Project : "belongs to"
    UsageLog }o--|| Request : "records"
    UsageLog }o--o| Channel : "uses"

    User ||--o{ ChannelOverrideTemplate : "creates"

    Role ||--o{ RoleScope : "contains"

    AxonHub {    }

    User {
        uuid id PK
        string email UK
        boolean is_owner
        jsonb scopes
    }

    Project {
        uuid id PK
        string name UK
        string status
    }

    Model {
        uuid id PK
        string developer
        string model_id
        string type
        string name
        string status
    }

    Channel {
        uuid id PK
        string name UK
        string type
        string status
    }

    ChannelPerformance {
        uuid id PK
        uuid channel_id FK
        int success_rate
        int avg_latency_ms
    }

    ChannelOverrideTemplate {
        uuid id PK
        uuid user_id FK
        string name
        string channel_type
    }

    DataStorage {
        uuid id PK
        string name UK
        string type
        boolean primary
        string status
    }

    System {
        uuid id PK
        string key UK
        jsonb value
    }

    Role {
        uuid id PK
        string name
        string level
        uuid project_id FK
        jsonb scopes
    }

    Thread {
        uuid id PK
        uuid project_id FK
        string thread_id UK
    }

    Trace {
        uuid id PK
        uuid project_id FK
        string trace_id UK
        uuid thread_id FK
    }

    APIKey {
        uuid id PK
        string key UK
        uuid user_id FK
        uuid project_id FK
        jsonb scopes
    }

    Request {
        uuid id PK
        uuid project_id FK
        uuid trace_id FK
        uuid api_key_id FK
        uuid channel_id FK
        uuid data_storage_id FK
        string model_id
        string status
    }

    RequestExecution {
        uuid id PK
        uuid request_id FK
        uuid channel_id FK
        uuid data_storage_id FK
        string status
    }

    UsageLog {
        uuid id PK
        uuid request_id FK
        uuid project_id FK
        uuid channel_id FK
        integer total_tokens
    }

    ProjectUser {
        uuid user_id FK
        uuid project_id FK
        boolean is_owner
    }

    UserRole {
        uuid user_id FK
        uuid role_id FK
    }

    RoleScope {
        uuid role_id FK
        string scope
    }
```

### Global Level 关系

```
Global
├── Users (多个)
│   ├── is_owner: true (Owner 用户)
│   └── scopes + roles (自定义权限)
├── Models (多个，所有 Project 共享)
├── Channels (多个，所有 Project 共享)
│   └── Channel Performance (每个 Channel 一个)
├── Data Storages (多个，所有 Project 共享)
├── System Configs (多个，所有 Project 共享)
└── Global Roles (多个)
```

### Project Level 关系

```
Project
├── Users (多个项目成员)
│   ├── owner: true (Project Owner)
│   └── project_roles + scopes (项目内权限)
├── Project Roles (多个)
├── API Keys (多个)
├── Threads (多个)
├── Traces (多个)
├── Requests (多个)
│   ├── Request Executions (多个)
│   └── Usage Logs (多个)
└── Usage Logs (多个)
```

### 详细关联关系

#### User 关联
- **User** → **Projects** (多对多)：用户可以加入多个项目
- **User** → **Roles** (多对多)：用户可以拥有多个角色（Global 和 Project）
- **User** → **API Keys** (一对多)：用户可以创建多个 API Keys
- **User** → **Channel Override Templates** (一对多)：用户可以创建多个模板

#### Project 关联
- **Project** → **Users** (多对多)：项目包含多个用户
- **Project** → **Roles** (一对多)：项目包含多个项目级角色
- **Project** → **API Keys** (一对多)：项目包含多个 API Keys
- **Project** → **Threads** (一对多)：项目包含多个线程
- **Project** → **Traces** (一对多)：项目包含多个追踪
- **Project** → **Requests** (一对多)：项目包含多个请求
- **Project** → **Usage Logs** (一对多)：项目包含多个使用日志

#### Channel 关联
- **Channel** → **Requests** (一对多)：渠道可以处理多个请求
- **Channel** → **Request Executions** (一对多)：渠道可以执行多个请求
- **Channel** → **Usage Logs** (一对多)：渠道关联多个使用日志
- **Channel** → **Channel Performance** (一对一)：每个渠道有一个性能记录

#### Data Storage 关联
- **Data Storage** → **Requests** (一对多)：可以存储多个请求
- **Data Storage** → **Request Executions** (一对多)：可以存储多个请求执行

#### Request 关联
- **Request** → **Project** (多对一)：请求属于一个项目
- **Request** → **API Key** (多对一，可选)：请求可能使用一个 API Key
- **Request** → **Trace** (多对一，可选)：请求可能属于一个追踪
- **Request** → **Channel** (多对一，可选)：请求可能使用一个渠道
- **Request** → **Data Storage** (多对一，可选)：请求可能存储在一个数据存储中
- **Request** → **Request Executions** (一对多)：请求包含多个执行记录
- **Request** → **Usage Logs** (一对多)：请求生成多个使用日志

#### Role 关联
- **Role** → **Users** (多对多)：角色可以分配给多个用户
- **Role** → **Project** (多对一，可选)：项目级角色属于一个项目
- **Role** → **Scopes** (一对多)：角色包含多个权限范围

#### Thread 关联
- **Thread** → **Project** (多对一)：线程属于一个项目
- **Thread** → **Traces** (一对多)：线程包含多个追踪

#### Trace 关联
- **Trace** → **Project** (多对一)：追踪属于一个项目
- **Trace** → **Thread** (多对一，可选)：追踪可能属于一个线程
- **Trace** → **Requests** (一对多)：追踪包含多个请求

---

## RBAC 权限模型

### Scope 层级

| Scope | Level | 描述 |
|-------|-------|------|
| `read_channels` | Global | 读取渠道信息 |
| `write_channels` | Global | 创建/修改渠道 |
| `read_users` | Global | 读取用户信息 |
| `write_users` | Global | 创建/修改用户 |
| `read_settings` | Global | 读取系统设置 |
| `write_settings` | Global | 修改系统设置 |
| `read_roles` | Global/Project | 读取角色信息 |
| `write_roles` | Global/Project | 创建/修改角色 |
| `read_api_keys` | Project | 读取 API Keys |
| `write_api_keys` | Project | 创建/修改 API Keys |
| `read_requests` | Project | 读取请求记录 |
| `write_requests` | Project | 创建请求 |
| `read_data_storages` | Global | 读取数据存储 |
| `write_data_storages` | Global | 写入数据存储 |

### Role 层级规则

1. **Global Role**：
   - 只能配置 Global Scopes
   - 在所有 Projects 中生效
   - 示例：System Admin, Channel Manager

2. **Project Role**：
   - 可以配置 Global 和 Project Scopes
   - 只在特定 Project 中生效
   - 示例：Project Admin, Developer, Viewer

### 权限继承规则

1. **Global Owner**：
   - 拥有所有 Global 和 Project 权限
   - 可以管理所有资源

2. **Project Owner**：
   - 拥有项目内所有权限
   - 可以管理项目内所有资源

3. **Custom Roles**：
   - 根据 Role 中定义的 Scopes 获得权限
   - Global Role 的权限在所有 Projects 中生效
   - Project Role 的权限只在特定 Project 中生效

---

## 数据流示例

### API 请求流程

```
1. User 使用 API Key 发起请求
   ↓
2. 系统验证 API Key 权限（scopes）
   ↓
3. 创建 Request 记录（关联 User, Project, API Key）
   ↓
4. 选择合适的 Channel
   ↓
5. 创建 Request Execution 记录
   ↓
6. 调用 Channel 执行请求
   ↓
7. 记录响应到 Request Execution
   ↓
8. 更新 Request 状态和响应
   ↓
9. 创建 Usage Log 记录 Token 使用情况
```

### 权限检查流程

```
1. 用户发起操作请求
   ↓
2. 检查用户是否为 Owner
   ├─ 是 → 允许操作
   └─ 否 → 继续检查
       ↓
3. 检查用户的 Global Roles 和 Scopes
   ├─ 有权限 → 允许操作
   └─ 无权限 → 继续检查
       ↓
4. 检查用户在当前 Project 的 Roles 和 Scopes
   ├─ 有权限 → 允许操作
   └─ 无权限 → 拒绝操作
```

---

## 索引策略

### 性能优化索引

1. **User**：
   - `email` (唯一索引)

2. **Model**：
   - `name` + `deleted_at` (唯一复合索引)

3. **Channel**：
   - `name` + `deleted_at` (唯一复合索引)

4. **Channel Performance**：
   - `channel_id` + `deleted_at` (唯一复合索引)

5. **Channel Override Template**：
   - `user_id` + `channel_type` + `name` + `deleted_at` (唯一复合索引)

6. **Data Storage**：
   - `name` (唯一索引)

7. **Role**：
   - `project_id` + `name` (唯一复合索引)
   - `level` (普通索引)

8. **API Key**：
   - `key` (唯一索引)
   - `user_id` (普通索引)
   - `project_id` (普通索引)

9. **Request**：
   - `api_key_id` (普通索引)
   - `channel_id` (普通索引)
   - `trace_id` (普通索引)
   - `created_at` (普通索引，用于时间范围查询)
   - `status` (普通索引，用于状态过滤)

10. **Thread**：
    - `project_id` (普通索引)
    - `thread_id` (唯一索引)

11. **Trace**：
    - `project_id` (普通索引)
    - `trace_id` (唯一索引)
    - `thread_id` (普通索引)

12. **Request Execution**：
    - `request_id` (普通索引)
    - `channel_id` (普通索引)

13. **Usage Log**：
    - `request_id` (普通索引)
    - `channel_id` (普通索引)
    - `created_at` (普通索引)
    - `model_id` (普通索引)
    - `project_id` + `created_at` (复合索引，用于项目使用分析)
    - `channel_id` + `created_at` (复合索引，用于渠道使用分析)

14. **User Project**：
    - `user_id` + `project_id` + `deleted_at` (唯一复合索引)
    - `project_id` (普通索引)

15. **User Role**：
    - `user_id` + `role_id` + `deleted_at` (唯一复合索引)
    - `role_id` (普通索引)

---

## 软删除机制

以下实体支持软删除（Soft Delete）：

- User
- Model
- Channel
- Channel Performance
- Channel Override Template
- Data Storage
- System
- Role
- API Key
- Thread
- Trace
- Request
- Usage Log
- User Project
- User Role
