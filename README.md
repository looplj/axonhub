<div align="center">

# AxonHub - Unified AI Gateway System

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

## 📖 Project Introduction

### Unified AI Gateway

AxonHub is a modern AI gateway system that provides a unified OpenAI, Anthropic, and AI SDK compatible API layer, transforming requests to various AI providers through a transformer pipeline architecture. The system features comprehensive tracing capabilities, helping enterprises better manage and monitor AI service usage. It also includes comprehensive test coverage to ensure system stability and reliability.

### Core Problems Solved

| Problem | AxonHub Solution |
|-------------|-------------------------|
| **Vendor Lock-in** | 🔄 Unified API interface, switch providers anytime |
| **Extensibility** | Flexible transformer architecture, supports multiple transformers |
| **Service Outages** | ⚡ Automatic failover, multi-channel redundancy |
| **Cost Control** | 💰 Intelligent routing, cost optimization strategies |
| **Permission Management** | 📊 Comprehensive user permission management |
| **Development Complexity** | 🛠️ Single SDK, unified interface standard |

---

## 📚 Documentation

### DeepWiki
For detailed technical documentation, API references, architecture design, and more, please visit [AxonHub DeepWiki](https://deepwiki.com/looplj/axonhub).

---

## ⭐ Core Features

### 📸 Screenshots

Here are some screenshots of AxonHub in action:

<table>
  <tr>
    <td align="center">
      <a href="screenshots/axonhub-dashboard.png">
        <img src="screenshots/axonhub-dashboard.png" alt="System Dashboard" width="250"/>
      </a>
      <br/>
      System Dashboard
    </td>
    <td align="center">
      <a href="screenshots/axonhub-channels.png">
        <img src="screenshots/axonhub-channels.png" alt="Channel Management" width="250"/>
      </a>
      <br/>
      Channel Management
    </td>
    <td align="center">
      <a href="screenshots/axonhub-users.png">
        <img src="screenshots/axonhub-users.png" alt="User Management" width="250"/>
      </a>
      <br/>
      User Management
    </td>
  </tr>
  <tr>
    <td align="center">
      <a href="screenshots/axonhub-requests.png">
        <img src="screenshots/axonhub-requests.png" alt="Request Monitoring" width="250"/>
      </a>
      <br/>
      Request Monitoring
    </td>
    <td align="center">
      <a href="screenshots/axonhub-ussge-logs.png">
        <img src="screenshots/axonhub-ussge-logs.png" alt="Usage Logs" width="250"/>
      </a>
      <br/>
      Usage Logs
    </td>
    <td align="center">
      <a href="screenshots/axonhub-system.png">
        <img src="screenshots/axonhub-system.png" alt="System Dashboard" width="250"/>
      </a>
      <br/>
      System Setting
    </td>
  </tr>
</table>

---

### 🚀 Supported Features

| Feature | Status | Description |
|---------|--------|-------------|
| **Chat Completion** | ✅ Done | Conversational interface |
| **Planning** | 📝 Todo | Feature planning and design |
| **Rerank** | 📝 Todo | Results ranking |
| **Embedding** | 📝 Todo | Vector embedding generation |
| **Realtime** | 📝 Todo | Live conversation capabilities |

---

### 🌐 Multi-Provider AI Gateway

| Feature | Technical Implementation | Business Value |
|-------------|----------------------|---------------------|
| **Unified API Interface** | OpenAI compatible standard, zero learning curve | Avoid vendor lock-in, reduce migration risk |
| **Intelligent Routing** | Bidirectional transformer architecture, millisecond-level switching | 99.9% availability guarantee, business continuity |
| **Automatic Failover** | Multi-channel retry + load balancing | Service interruption time < 100ms |
| **Stream Processing** | Native SSE support, real-time response | 60% user experience improvement |

---

### 🔧 API Format Support

| Format | Status | Compatibility | Notes |
|-------------|------------|---------------------|----------|
| **OpenAI API** | ✅ Done | Fully compatible | Chat/Completions API |
| **Anthropic API** | ✅ Done | Fully supported | Claude Messages API |
| **AI SDK** | ⚠️ Partial | Partially supported | Vercel AI SDK format |
| **More Formats** | 🔄 Ongoing | Continuously added | New API format support |

---

### 🤖 Supported Providers

| Provider | Status | Supported Models | Notes |
|---------------|------------|---------------------------|----------|
| **OpenAI** | ✅ Done | GPT-4, GPT-4o, GPT-5, etc. | Fully supported, including streaming responses |
| **Anthropic** | ✅ Done | Claude 4.0, Claude 4.1, etc. | Fully supported, including chain of thought |
| **Zhipu AI** | ✅ Done | GLM-4.5, GLM-4.5-air, etc. | Fully supported |
| **Moonshot AI (Kimi)** | ✅ Done | kimi-k2, etc. | Fully supported |
| **DeepSeek** | ✅ Done | DeepSeek-V3.1, etc. | Fully supported |
| **ByteDance Doubao** | ✅ Done | doubao-1.6, etc. | Fully supported |
| **AWS Bedrock** | 🔄 Testing | Claude on AWS | Access via Bedrock |
| **Google Cloud** | 🔄 Testing| Claude on GCP | Access via Vertex AI |
| **Gemini** | 📝 Todo | Gemini 2.5, etc. | Not implemented |

---

### 🏢 Permission Control

| Security Feature | Implementation |   
|-----------------|----------------------|
| **Fine-grained Permission Control** | Role-based access control (RBAC) |
| **Data Localization** | Configurable data storage policies |
| **API Key Management** | JWT + scope control |

---



## 🚀 Deployment Guide

### Database Support

AxonHub supports multiple databases to meet different scale deployment needs:

| Database | Supported Versions | Recommended Scenario | Auto Migration | Links |
|--------|----------|----------|----------|------|
| **TiDB Cloud** | Starter | Serverless, Free tier, Auto Scale | ✅ Supported | [TiDB Cloud](https://www.pingcap.com/tidb-cloud-starter/) |
| **TiDB Cloud** | Dedicated | Distributed deployment, large scale | ✅ Supported | [TiDB Cloud](https://www.pingcap.com/tidb-cloud-dedicated/) |
| **TiDB** | V8.0+ | Distributed deployment, large scale | ✅ Supported | [TiDB](https://tidb.io/) |
| **Neon DB** | - | Serverless, Free tier, Auto Scale | ✅ Supported | [Neon DB](https://neon.com/) |
| **PostgreSQL** | 15+ | Production environment, medium-large deployments | ✅ Supported | [PostgreSQL](https://www.postgresql.org/) |
| **MySQL** | 8.0+ | Production environment, medium-large deployments | ✅ Supported | [MySQL](https://www.mysql.com/) |
| **SQLite** | 3.0+ | Development environment, small deployments | ✅ Supported | [SQLite](https://www.sqlite.org/index.html) |


### Configuration

AxonHub uses YAML configuration files with environment variable override support:

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

Environment variables:
```bash
AXONHUB_SERVER_PORT=8090
AXONHUB_DB_DIALECT="tidb"
AXONHUB_DB_DSN="<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true"
AXONHUB_LOG_LEVEL=info
```

For detailed configuration instructions, please refer to [configuration documentation](config.example.yml).

### Docker Compose Deployment

```bash
# Clone project
git clone https://github.com/looplj/axonhub.git
cd axonhub

# Set environment variables
export AXONHUB_DB_DIALECT="tidb"
export AXONHUB_DB_DSN="<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true"

# Start services
docker-compose up -d

# Check status
docker-compose ps
```

### Virtual Machine Deployment

```bash
# Clone project
git clone https://github.com/looplj/axonhub.git
cd axonhub

# Set environment variables
export AXONHUB_DB_DIALECT="tidb"
export AXONHUB_DB_DSN="<USER>.root:<PASSWORD>@tcp(gateway01.us-west-2.prod.aws.tidbcloud.com:4000)/axonhub?tls=true"

# Build
make build

# Configuration file check
./axonhub config check

# Start service
./axonhub 
```

#### Systemd Service Configuration

Copy `deploy/axonhub.service` to `/etc/systemd/system/axonhub.service`:

```bash
sudo cp deploy/axonhub.service /etc/systemd/system/axonhub.service
```

Start service:

```bash
sudo systemctl daemon-reload
sudo systemctl start axonhub
sudo systemctl enable axonhub
```

---

## 📖 Usage Guide

### 1. Initial Setup

1. **Access Management Interface**
   ```
   http://localhost:8090
   ```

2. **Configure AI Providers**
   - Add API keys in the management interface
   - Test connections to ensure correct configuration

3. **Create Users and Roles**
   - Set up permission management
   - Assign appropriate access permissions

### 2. Channel Configuration

Configure AI provider channels in the management interface:

```yaml
# OpenAI channel example
name: "openai"
type: "openai"
base_url: "https://api.openai.com/v1"
credentials:
  api_key: "your-openai-key"
supported_models: ["gpt-5", "gpt-4o"]
```

#### 2.1 Test Connection

Click the test button. If the test is successful, the configuration is correct.

#### 2.2 Enable Channel

After successful testing, click the enable button to activate the channel.

### 3. Add Users

1. Create user accounts
2. Assign roles and permissions
3. Generate API keys

### 4. API Key Usage

```bash
# Set environment variables
export OPENAI_API_KEY="your-axonhub-api-key"
export OPENAI_BASE_URL="http://localhost:8090/v1"

# Test with curl
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```


### 5. Claude Code Integration

Using AxonHub in Claude Code:

```bash
# Set Claude Code to use AxonHub
export ANTHROPIC_API_KEY="your-axonhub-api-key"
export ANTHROPIC_BASE_URL="http://localhost:8090"
```

---

### 6. SDK Usage

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



## 🛠️ Development Guide

### Architecture Design

AxonHub adopts a highly scalable architecture supporting multiple AI providers and model switching:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client Apps   │    │   Web Frontend  │    │   Mobile Apps   │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          └──────────────────────┼──────────────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │    AxonHub Gateway      │
                    │  ┌─────────────────────┐ │
                    │  │  Unified API Layer  │ │
                    │  │  Smart Router       │ │
                    │  │  Access Control     │ │
                    │  │  Audit Logs         │ │
                    │  └─────────────────────┘ │
                    └────────────┬─────────────┘
                                 │
          ┌──────────────────────┼──────────────────────┐
          │                      │                      │
    ┌─────▼─────┐        ┌─────▼─────┐        ┌─────▼─────┐
    │  OpenAI   │        │ Anthropic │        │  DeepSeek │
    │  Claude   │        │   Gemini  │        │  Moonshot │
    │           │        │           │        │   Zhipu   │
    └───────────┘        └───────────┘        └───────────┘
```

Transformation Flow:

  Client Request → Inbound Transformer → Unified Request → Pipeline → Outbound Transformer → HTTP Client → Provider

### Technology Stack

#### Backend Technology Stack
- **Go 1.24+** - High-performance backend
- **Gin** - HTTP framework
- **Ent ORM** - Type-safe ORM
- **GraphQL** - Flexible API queries
- **JWT** - Authentication

#### Frontend Technology Stack
- **React 19** - Modern UI framework
- **TypeScript** - Type safety
- **Tailwind CSS** - Styling framework
- **TanStack Router** - File-based routing
- **Zustand** - State management

### Development Environment Setup

```bash
# Clone project
git clone https://github.com/looplj/axonhub.git
cd axonhub

# Start backend
make build-backend
./axonhub

# Start frontend (new terminal)
cd frontend
pnpm install
pnpm dev
```

### Build Project

```bash
make build
```

---

## 🤝 Acknowledgments

- 🙏 [musistudio/llms](https://github.com/musistudio/llms) - LLM transformation framework, source of inspiration
- 🎨 [satnaing/shadcn-admin](https://github.com/satnaing/shadcn-admin) - Admin interface template
- 🔧 [99designs/gqlgen](https://github.com/99designs/gqlgen) - GraphQL code generation
- 🌐 [gin-gonic/gin](https://github.com/gin-gonic/gin) - HTTP framework
- 🗄️ [ent/ent](https://github.com/ent/ent) - ORM framework

---

## 📄 License

This project is open source under the MIT License. See [LICENSE](LICENSE) file for details.

---

<div align="center">

**AxonHub** - Unified AI Gateway, making AI service integration simpler

[🏠 Homepage](https://github.com/looplj/axonhub) • [📚 Documentation](https://deepwiki.com/looplj/axonhub) • [🐛 Issue Feedback](https://github.com/looplj/axonhub/issues)

Built with ❤️ by the AxonHub team

</div>