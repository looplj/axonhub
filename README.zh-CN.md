<div align="center">

# AxonHub - All-in-one AI 开发平台

</div>

<div align="center">

[![Test Status](https://github.com/looplj/axonhub/actions/workflows/test.yml/badge.svg)](https://github.com/looplj/axonhub/actions/workflows/test.yml)
[![Lint Status](https://github.com/looplj/axonhub/actions/workflows/lint.yml/badge.svg)](https://github.com/looplj/axonhub/actions/workflows/lint.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/looplj/axonhub?logo=go&logoColor=white)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker Ready](https://img.shields.io/badge/docker-ready-2496ED?logo=docker&logoColor=white)](https://docker.com)

[English](README.md) | [中文](README.zh-CN.md)

</div>

---

## 📖 项目介绍

### All-in-one AI 开发平台

AxonHub 是一个 All-in-one AI 开发平台，提供统一的 API 网关、项目管理和全面的开发工具。平台提供 OpenAI、Anthropic 和 AI SDK 兼容的 API 层，通过转换器管道架构将请求转换到各种 AI 提供商。系统具备完整的追踪能力、基于项目的组织结构以及集成的 Playground 快速原型开发，帮助开发者和企业更好地管理 AI 开发工作流。

<div align="center">
  <img src="docs/axonhub-architecture-light.svg" alt="AxonHub Architecture" width="700"/>
</div>

### 核心特性 Core Features

1. [**统一 API** Unified API](docs/unified-api.md)：兼容 OpenAI 与 Anthropic 的接口，配合转换管线实现模型互换与映射，无需改动现有代码。
2. [**追踪 / 线程** Tracing / Threads](docs/traces.md)：线程级追踪实时记录完整调用链路，提升可观测性与问题定位效率。
3. [**细粒度权限** Fine-grained Permission](docs/fine-grained-permission.md)：基于 RBAC 的权限策略，帮助团队精细管理访问控制、配额与数据隔离。

---

## 📚 文档 | Documentation

### DeepWiki
详细的技术文档、API 参考、架构设计等内容，可以访问 
- [DeepWiki](https://deepwiki.com/looplj/axonhub)
- [Zread](https://zread.ai/looplj/axonhub)

---

## 🎯 演示 | Demo

在我们的 [演示实例](https://axonhub.onrender.com) 上体验 AxonHub！

**注意**：演示网站目前配置了 Zhipu 和 OpenRouter 的免费模型。

### 演示账号 | Demo Account
- **邮箱 Email**: demo@example.com
- **密码 Password**: 12345678

---

## ⭐ 特性 | Features

### 📸 截图 | Screenshots

以下是 AxonHub 的实际运行截图：

<table>
  <tr>
    <td align="center">
      <a href="docs/screenshots/axonhub-dashboard.png">
        <img src="docs/screenshots/axonhub-dashboard.png" alt="系统仪表板" width="250"/>
      </a>
      <br/>
      系统仪表板
    </td>
    <td align="center">
      <a href="docs/screenshots/axonhub-channels.png">
        <img src="docs/screenshots/axonhub-channels.png" alt="渠道管理" width="250"/>
      </a>
      <br/>
      渠道管理
    </td>
    <td align="center">
      <a href="docs/screenshots/axonhub-users.png">
        <img src="docs/screenshots/axonhub-users.png" alt="用户管理" width="250"/>
      </a>
      <br/>
      用户管理
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="docs/screenshots/axonhub-requests.png">
        <img src="docs/screenshots/axonhub-requests.png" alt="请求监控" width="250"/>
      </a>
      <br/>
      请求监控
    </td>
    <td align="center">
      <a href="docs/screenshots/axonhub-usage-logs.png">
        <img src="docs/screenshots/axonhub-usage-logs.png" alt="用量日志" width="250"/>
      </a>
      <br/>
      用量日志
    </td>
    <td align="center">
      <a href="docs/screenshots/axonhub-system.png">
        <img src="docs/screenshots/axonhub-system.png" alt="系统设置" width="250"/>
      </a>
      <br/>
      系统设置
    </td>
  </tr>
</table>

---

### 🚀 API 类型 | API Types

| API 类型 | 状态 | 描述 | 文档 |
|---------|--------|-------------|--------|
| **文本生成（Text Generation）** | ✅ Done | 对话交互接口 | [Chat Completions](docs/chat-completions.md) |
| **图片生成（Image Generation）** | ⚠️ Partial | 图片生成 | [Image Generations](docs/image-generations.md) |
| **重排序（Rerank）** | 📝 Todo | 结果排序 | - |
| **实时对话（Realtime）** | 📝 Todo | 实时对话功能 | - |
| **嵌入（Embedding）** | 📝 Todo | 向量嵌入生成 | - |

---

### 🌐 多提供商 AI 网关 | Multi-Provider AI Gateway

| 特性 Feature | 技术实现 Implementation | 企业价值 Business Value |
|-------------|----------------------|---------------------|
| **统一 API 接口** | OpenAI 兼容标准，零学习成本 | 避免供应商锁定，降低迁移风险 |
| **自动故障转移** | 多渠道级重试 + 负载均衡 | 服务中断时间 < 100ms |
| **流式处理** | 原生 SSE 支持，实时响应 | 用户体验提升 60% |

---

### 🧵 线程与追踪 | Threads & Tracing

AxonHub 可以在不改动现有 OpenAI 兼容客户端的前提下，为每一次请求建立线程级追踪：

- 需要显式传入 `AH-Trace-Id` 请求头才能将多次请求串联到同一追踪；若缺失该请求头，AxonHub 会记录单次调用但无法自动关联相关请求
- 将追踪与线程关联，串联整段会话的上下文
- 捕获模型元数据、请求/响应片段以及耗时信息，便于快速定位问题

了解更多工作原理与使用方式，请参阅 [Tracing Guide](docs/traces.md)。

### 🔧 接口格式支持 | API Format Support

| 格式 Format | 状态 Status | 兼容性 Compatibility | Modalities |
|-------------|------------|---------------------|----------|
| **OpenAI Chat Completions** | ✅ Done | 完全兼容 | Text, Image |
| **Anthropic API** | ✅ Done | 完全支持 | Text |
| **AI SDK** | ⚠️ Partial | 部分支持 | Text |
| **Gemini** | 🔄 Todo | - | - |

---

### 🏢 权限控制 | Permission Control

| 安全特性 Security | 实现方式 Implementation |
|-----------------|----------------------|
| **细粒度权限控制** | 基于角色的访问控制 (RBAC) |
| **数据本地化** | 可配置数据存储策略 |
| **API 密钥管理** | JWT + 作用域控制 |

---


## 🚀 部署指南 | Deployment Guide

### 💻 个人电脑部署 | Personal Computer Deployment

适合个人开发者和小团队使用，无需复杂配置。

#### 快速下载运行 | Quick Download & Run

1. **下载最新版本** 从 [GitHub Releases](https://github.com/looplj/axonhub/releases)
   - 选择适合您操作系统的版本：

2. **解压并运行**
   ```bash
   # 解压下载的文件
   unzip axonhub_*.zip
   cd axonhub_*
   
   # 添加执行权限 (仅限 Linux/macOS)
   chmod +x axonhub
   
   # 直接运行 - 默认使用 SQLite 数据库
   # 安装 AxonHub 到系统
   ./install.sh

   # 启动 AxonHub 服务
   ./start.sh

   # 停止 AxonHub 服务
   ./stop.sh
   ```

3. **访问应用**
   ```
   http://localhost:8090
   ```

---

### 🖥️ 服务器部署 | Server Deployment

适用于生产环境、高可用性和企业级部署。

#### 数据库支持 | Database Support

AxonHub 支持多种数据库，满足不同规模的部署需求：

| 数据库 | 支持版本 | 推荐场景 | 自动迁移 | 链接 |
|--------|----------|----------|----------|------|
| **SQLite** | 3.0+ | 开发环境、小型部署 | ✅ 支持 | [SQLite](https://www.sqlite.org/index.html) |
| **TiDB Cloud** | Starter | Serverless, Free tier, Auto Scale | ✅ 支持 | [TiDB Cloud](https://www.pingcap.com/tidb-cloud-starter/) |
| **TiDB Cloud** | Dedicated | 分布式部署、大规模 | ✅ 支持 | [TiDB Cloud](https://www.pingcap.com/tidb-cloud-dedicated/) |
| **TiDB** | V8.0+ | 分布式部署、大规模 | ✅ 支持 | [TiDB](https://tidb.io/) |
| **Neon DB** | - | Serverless, Free tier, Auto Scale | ✅ 支持 | [Neon DB](https://neon.com/) |
| **PostgreSQL** | 15+ | 生产环境、中大型部署 | ✅ 支持 | [PostgreSQL](https://www.postgresql.org/) |
| **MySQL** | 8.0+ | 生产环境、中大型部署 | ✅ 支持 | [MySQL](https://www.mysql.com/) |

#### 配置文件 | Configuration

AxonHub 使用 YAML 配置文件，支持环境变量覆盖：

```yaml
# config.yml
server:
  port: 8090
  name: "AxonHub"
  debug: false

db:
  dialect: "tidb"
  dsn: "<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true"

log:
  level: "info"
  encoding: "json"
```

环境变量：
```bash
AXONHUB_SERVER_PORT=8090
AXONHUB_DB_DIALECT="tidb"
AXONHUB_DB_DSN="<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true"
AXONHUB_LOG_LEVEL=info
```

详细配置说明请参考 [配置文档](config.example.yml)。

#### Docker Compose 部署

```bash
# 克隆项目
git clone https://github.com/looplj/axonhub.git
cd axonhub

# 设置环境变量
export AXONHUB_DB_DIALECT="tidb"
export AXONHUB_DB_DSN="<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true"

# 启动服务
docker-compose up -d

# 查看状态
docker-compose ps
```

#### 虚拟机部署 | Virtual Machine Deployment

下载最新版本从 [GitHub Releases](https://github.com/looplj/axonhub/releases)

```bash
# 克隆项目
git clone https://github.com/looplj/axonhub.git
cd axonhub

# 设置环境变量
export AXONHUB_DB_DIALECT="tidb"
export AXONHUB_DB_DSN="<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true"

# 安装
sudo ./install.sh

# 配置文件检查
axonhub config check

# 使用管理脚本管理 AxonHub

# 启动
./start.sh

# 停止
./stop.sh
```

---

## 📖 使用指南 | Usage Guide

### 1. 初始化设置 | Initial Setup

1. **访问管理界面**
   ```
   http://localhost:8090
   ```

2. **配置 AI 提供商**
   - 在管理界面中添加 API 密钥
   - 测试连接确保配置正确

3. **创建用户和角色**
   - 设置权限管理
   - 分配适当的访问权限

### 2. Channel 配置 | Channel Configuration

在管理界面中配置 AI 提供商渠道：

```yaml
# OpenAI 渠道示例
name: "openai"
type: "openai"
base_url: "https://api.openai.com/v1"
credentials:
  api_key: "your-openai-key"
supported_models: ["gpt-5", "gpt-4o"]
```

#### 2.1 测试连接

点击测试按钮，如果测试成功，说明配置正确。

#### 2.2 启用渠道

测试成功后，点击启用按钮，启用该渠道。

#### 2.3 模型映射 | Model Mappings

当请求中的模型名称与上游提供商支持的名称不一致时，可以通过模型映射在网关侧自动重写模型。

- 将不支持或旧版本的模型 ID 映射到可用的替代模型
- 为多渠道场景设置回退逻辑（不同渠道对应不同提供商）

```yaml
# 示例：将产品自定义别名映射到上游模型
settings:
  modelMappings:
    - from: "gpt-4o-mini"
      to: "gpt-4o"
    - from: "claude-3-sonnet"
      to: "claude-3.5-sonnet"
```

> 注意：AxonHub 仅接受映射到 `supported_models` 中已声明的模型。

#### 2.4 请求参数覆盖 | Override Parameters

请求参数覆盖允许为渠道强制设置默认参数，无论上游请求携带了什么内容。配置时提供一个 JSON 对象，系统会在转发请求前自动合并。

- 支持顶层字段（如 `temperature`、`max_tokens`、`top_p`）
- 支持使用点分写法的嵌套字段（如 `response_format.type`）
- 若 JSON 无法解析，系统会记录告警日志并保持原始请求不变

```yaml
# 示例：强制输出确定性的 JSON 结构
settings:
  overrideParameters: |
    {
      "temperature": 0.3,
      "max_tokens": 1024,
      "response_format.type": "json_object"
    }
```


### 3. 添加用户 | Add Users

1. 创建用户账户
2. 分配角色和权限
3. 生成 API 密钥

### 4. Claude Code/Codex 使用 | Claude Code Integration

关于如何在 Claude Code 与 Claude Codex 中配置与 AxonHub 的集成、排查常见问题以及结合模型配置文件工作流的最佳实践，请参阅专门的 [Claude Code & Codex 集成指南](docs/claude-code-integration.md)。

该文档提供了环境变量示例、Codex 配置模板、模型配置文件说明以及工作流示例，帮助您快速完成接入。

---

### 5. 使用 SDK | SDK Usage

#### Python SDK
```python
from openai import OpenAI

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

response = client.chat.completions.create(
    model="gpt-4o",
    messages=[{"role": "user", "content": "Hello!"}]
)
print(response.choices[0].message.content)
```

#### Node.js SDK
```javascript
import OpenAI from 'openai';

const openai = new OpenAI({
  apiKey: 'your-axonhub-api-key',
  baseURL: 'http://localhost:8090/v1',
});

const completion = await openai.chat.completions.create({
  messages: [{ role: 'user', content: 'Hello!' }],
  model: 'gpt-4o',
});
```


## 🛠️ 开发指南

详细的开发说明、架构设计和贡献指南，请查看 [DEVELOPMENT.md](DEVELOPMENT.md)。

---

## 🤝 致谢 | Acknowledgments

- 🙏 [musistudio/llms](https://github.com/musistudio/llms) - LLM 转换框架，灵感来源
- 🎨 [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin) - 管理界面模板
- 🔧 [99designs/gqlgen](https://github.com/99designs/gqlgen) - GraphQL 代码生成
- 🌐 [gin-gonic/gin](https://github.com/gin-gonic/gin) - HTTP 框架
- 🗄️ [ent/ent](https://github.com/ent/ent) - ORM 框架
- 🔧 [air-verse/air](https://github.com/air-verse/air) - 自动重载 Go 服务
- ☁️ [render](https://render.com) - 免费云部署平台，用于部署 demo
- 🗄️ [tidbcloud](https://www.pingcap.com/tidb-cloud/) - Serverless 数据库平台，用于部署 demo

---

## 📄 许可证 | License

本项目采用 MIT 许可证开源。详见 [LICENSE](LICENSE) 文件。

---

<div align="center">

**AxonHub** - All-in-one AI 开发平台，让 AI 开发更简单

[🏠 官网](https://github.com/looplj/axonhub) • [📚 文档](https://deepwiki.com/looplj/axonhub) • [🐛 问题反馈](https://github.com/looplj/axonhub/issues)

Built with ❤️ by the AxonHub team

</div>