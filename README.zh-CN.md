<div align="center">

# AxonHub - All-in-one AI 开发平台

</div>

<div align="center">

[![测试状态](https://github.com/looplj/axonhub/actions/workflows/test.yml/badge.svg)](https://github.com/looplj/axonhub/actions/workflows/test.yml)
[![Lint 状态](https://github.com/looplj/axonhub/actions/workflows/lint.yml/badge.svg)](https://github.com/looplj/axonhub/actions/workflows/lint.yml)
[![Go 版本](https://img.shields.io/github/go-mod/go-version/looplj/axonhub?logo=go&logoColor=white)](https://golang.org/)
[![许可证: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
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

1. [**统一 API** Unified API](docs/zh/api-reference/unified-api.md)：兼容 OpenAI 与 Anthropic 的接口，配合转换管线实现模型互换与映射，无需改动现有代码。
2. [**追踪 / 线程** Tracing / Threads](docs/zh/guides/tracing.md)：线程级追踪实时记录完整调用链路，提升可观测性与问题定位效率。
3. [**细粒度权限** Fine-grained Permission](docs/zh/guides/permissions.md)：基于 RBAC 的权限策略，帮助团队精细管理访问控制、配额与数据隔离。
4. [**自适应负载均衡** Adaptive Load Balancing](docs/zh/guides/load-balance.md): 智能多策略负载均衡，自动选择最优 AI 渠道，确保高可用性和最佳性能。

---

## 📚 文档 | Documentation

### DeepWiki
详细的技术文档、API 参考、架构设计等内容，可以访问 
- [![DeepWiki](https://img.shields.io/badge/DeepWiki-looplj%2Faxonhub-blue.svg?logo=data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACwAAAAyCAYAAAAnWDnqAAAAAXNSR0IArs4c6QAAA05JREFUaEPtmUtyEzEQhtWTQyQLHNak2AB7ZnyXZMEjXMGeK/AIi+QuHrMnbChYY7MIh8g01fJoopFb0uhhEqqcbWTp06/uv1saEDv4O3n3dV60RfP947Mm9/SQc0ICFQgzfc4CYZoTPAswgSJCCUJUnAAoRHOAUOcATwbmVLWdGoH//PB8mnKqScAhsD0kYP3j/Yt5LPQe2KvcXmGvRHcDnpxfL2zOYJ1mFwrryWTz0advv1Ut4CJgf5uhDuDj5eUcAUoahrdY/56ebRWeraTjMt/00Sh3UDtjgHtQNHwcRGOC98BJEAEymycmYcWwOprTgcB6VZ5JK5TAJ+fXGLBm3FDAmn6oPPjR4rKCAoJCal2eAiQp2x0vxTPB3ALO2CRkwmDy5WohzBDwSEFKRwPbknEggCPB/imwrycgxX2NzoMCHhPkDwqYMr9tRcP5qNrMZHkVnOjRMWwLCcr8ohBVb1OMjxLwGCvjTikrsBOiA6fNyCrm8V1rP93iVPpwaE+gO0SsWmPiXB+jikdf6SizrT5qKasx5j8ABbHpFTx+vFXp9EnYQmLx02h1QTTrl6eDqxLnGjporxl3NL3agEvXdT0WmEost648sQOYAeJS9Q7bfUVoMGnjo4AZdUMQku50McDcMWcBPvr0SzbTAFDfvJqwLzgxwATnCgnp4wDl6Aa+Ax283gghmj+vj7feE2KBBRMW3FzOpLOADl0Isb5587h/U4gGvkt5v60Z1VLG8BhYjbzRwyQZemwAd6cCR5/XFWLYZRIMpX39AR0tjaGGiGzLVyhse5C9RKC6ai42ppWPKiBagOvaYk8lO7DajerabOZP46Lby5wKjw1HCRx7p9sVMOWGzb/vA1hwiWc6jm3MvQDTogQkiqIhJV0nBQBTU+3okKCFDy9WwferkHjtxib7t3xIUQtHxnIwtx4mpg26/HfwVNVDb4oI9RHmx5WGelRVlrtiw43zboCLaxv46AZeB3IlTkwouebTr1y2NjSpHz68WNFjHvupy3q8TFn3Hos2IAk4Ju5dCo8B3wP7VPr/FGaKiG+T+v+TQqIrOqMTL1VdWV1DdmcbO8KXBz6esmYWYKPwDL5b5FA1a0hwapHiom0r/cKaoqr+27/XcrS5UwSMbQAAAABJRU5ErkJggg==)](https://deepwiki.com/looplj/axonhub)
- [![zread](https://img.shields.io/badge/Ask_Zread-_.svg?style=flat&color=00b0aa&labelColor=000000&logo=data%3Aimage%2Fsvg%2Bxml%3Bbase64%2CPHN2ZyB3aWR0aD0iMTYiIGhlaWdodD0iMTYiIHZpZXdCb3g9IjAgMCAxNiAxNiIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZD0iTTQuOTYxNTYgMS42MDAxSDIuMjQxNTZDMS44ODgxIDEuNjAwMSAxLjYwMTU2IDEuODg2NjQgMS42MDE1NiAyLjI0MDFWNC45NjAxQzEuNjAxNTYgNS4zMTM1NiAxLjg4ODEgNS42MDAxIDIuMjQxNTYgNS42MDAxSDQuOTYxNTZDNS4zMTUwMiA1LjYwMDEgNS42MDE1NiA1LjMxMzU2IDUuNjAxNTYgNC45NjAxVjIuMjQwMUM1LjYwMTU2IDEuODg2NjQgNS4zMTUwMiAxLjYwMDEgNC45NjE1NiAxLjYwMDFaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00Ljk2MTU2IDEwLjM5OTlIMi4yNDE1NkMxLjg4ODEgMTAuMzk5OSAxLjYwMTU2IDEwLjY4NjQgMS42MDE1NiAxMS4wMzk5VjEzLjc1OTlDMS42MDE1NiAxNC4xMTM0IDEuODg4MSAxNC4zOTk5IDIuMjQxNTYgMTQuMzk5OUg0Ljk2MTU2QzUuMzE1MDIgMTQuMzk5OSA1LjYwMTU2IDE0LjExMzQgNS42MDE1NiAxMy43NTk5VjExLjAzOTlDNS42MDE1NiAxMC42ODY0IDUuMzE1MDIgMTAuMzk5OSA0Ljk2MTU2IDEwLjM5OTlaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik0xMy43NTg0IDEuNjAwMUgxMS4wMzg0QzEwLjY4NSAxLjYwMDEgMTAuMzk4NCAxLjg4NjY0IDEwLjM5ODQgMi4yNDAxVjQuOTYwMUMxMC4zOTg0IDUuMzEzNTYgMTAuNjg1IDUuNjAwMSAxMS4wMzg0IDUuNjAwMUgxMy43NTg0QzE0LjExMTkgNS42MDAxIDE0LjM5ODQgNS4zMTM1NiAxNC4zOTg0IDQuOTYwMVYyLjI0MDFDMTQuMzk4NCAxLjg4NjY0IDE0LjExMTkgMS42MDAxIDEzLjc1ODQgMS42MDAxWiIgZmlsbD0iI2ZmZiIvPgo8cGF0aCBkPSJNNCAxMkwxMiA0TDQgMTJaIiBmaWxsPSIjZmZmIi8%2BCjxwYXRoIGQ9Ik00IDEyTDEyIDQiIHN0cm9rZT0iI2ZmZiIgc3Ryb2tlLXdpZHRoPSIxLjUiIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIvPgo8L3N2Zz4K&logoColor=ffffff)](https://zread.ai/looplj/axonhub)


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
      <a href="docs/screenshots/axonhub-models.png">
        <img src="docs/screenshots/axonhub-models.png" alt="模型" width="250"/>
      </a>
      <br/>
      模型
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="docs/screenshots/axonhub-trace.png">
        <img src="docs/screenshots/axonhub-trace.png" alt="追踪查看" width="250"/>
      </a>
      <br/>
      追踪查看
    </td>
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
  </tr>
</table>

---

### 🚀 API 类型 | API Types

| API 类型 | 状态 | 描述 | 文档 |
|---------|--------|-------------|--------|
| **文本生成（Text Generation）** | ✅ Done | 对话交互接口 | [Unified API](docs/zh/api-reference/unified-api.md) |
| **图片生成（Image Generation）** | ⚠️ Partial | 图片生成 | [Image Generation](docs/zh/api-reference/image-generation.md) |
| **重排序（Rerank）** | ✅ Done | 结果排序 | [Unified API](docs/zh/api-reference/unified-api.md#重排序-api) |
| **嵌入（Embedding）** | ✅ Done | 向量嵌入生成 | [Unified API](docs/zh/api-reference/unified-api.md#嵌入-api) |
| **实时对话（Realtime）** | 📝 Todo | 实时对话功能 | - |

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

了解更多工作原理与使用方式，请参阅 [Tracing Guide](docs/zh/guides/tracing.md)。

### 🔧 接口格式支持 | API Format Support

| 格式 Format | 状态 Status | 兼容性 Compatibility | Modalities |
|-------------|------------|---------------------|----------|
| **OpenAI Chat Completions** | ✅ Done | 完全兼容 | Text, Image |
| **OpenAI Responses** | ⚠️ Partial | 不支持 `previous_response_id` | Text |
| **Anthropic API** | ✅ Done | 完全支持 | Text |
| **Gemini** | ✅ Done | 完全支持 | Text, Image |
| **AI SDK** | ⚠️ Partial | 部分支持 | Text |

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

在管理界面中配置 AI 提供商渠道。关于渠道配置的详细信息，包括模型映射、参数覆盖和故障排除，请参阅 [渠道配置指南](docs/zh/guides/channel-management.md)。

### 3. 模型管理 | Model Management

AxonHub 提供灵活的模型管理系统，支持通过模型关联将抽象模型映射到特定渠道和模型实现。这使您能够：

- **统一模型接口** - 使用抽象模型 ID（如 `gpt-4`、`claude-3-opus`）替代渠道特定的名称
- **智能渠道选择** - 基于关联规则和负载均衡自动将请求路由到最优渠道
- **灵活的映射策略** - 支持精确的渠道-模型匹配、正则表达式模式和基于标签的选择
- **基于优先级的回退** - 配置多个具有优先级的关联以实现自动故障转移

关于模型管理的全面信息，包括关联类型、配置示例和最佳实践，请参阅 [模型管理指南](docs/zh/guides/model-management.md)。

### 4. 创建 API Key | Create API Keys

创建 API 密钥以验证您的应用程序与 AxonHub 的连接。每个 API 密钥可以配置多个配置文件（Profile），用于定义：

- **模型映射** - 使用精确匹配或正则表达式模式将用户请求的模型转换为实际可用的模型
- **渠道限制** - 通过渠道 ID 或标签限制 API 密钥可以使用的渠道
- **模型访问控制** - 控制特定配置文件可以访问的模型
- **配置文件切换** - 通过激活不同的配置文件即时更改行为

关于 API 密钥配置文件的详细信息，包括配置示例、验证规则和最佳实践，请参阅 [API 密钥配置文件指南](docs/zh/guides/api-key-profiles.md)。

### 5. Claude Code/Codex 使用 | Claude Code Integration

关于如何在 Claude Code 与 Claude Codex 中配置与 AxonHub 的集成、排查常见问题以及结合模型配置文件工作流的最佳实践，请参阅专门的 [Claude Code & Codex 集成指南](docs/zh/guides/claude-code-integration.md)。

该文档提供了环境变量示例、Codex 配置模板、模型配置文件说明以及工作流示例，帮助您快速完成接入。

---

### 6. 使用 SDK | SDK Usage

详细的 SDK 使用示例和代码示例，请参阅 [Unified API 文档](docs/zh/api-reference/unified-api.md)。


## 🛠️ 开发指南

详细的开发说明、架构设计和贡献指南，请查看 [docs/zh/guides/development.md](docs/zh/guides/development.md)。

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