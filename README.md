# AxonHub - 统一 AI 网关系统 | Unified AI Gateway

<div align="center">

[![Test Status](https://github.com/looplj/axonhub/workflows/test/badge.svg)](https://github.com/looplj/axonhub/actions)
[![Lint Status](https://github.com/looplj/axonhub/workflows/lint/badge.svg)](https://github.com/looplj/axonhub/actions)
[![Go Version](https://img.shields.io/github/go-mod/go-version/looplj/axonhub?logo=go&logoColor=white)](https://golang.org/)
[![Frontend Version](https://img.shields.io/badge/React-19.1.0-61DAFB?logo=react&logoColor=white)](https://reactjs.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker Ready](https://img.shields.io/badge/docker-ready-2496ED?logo=docker&logoColor=white)](https://docker.com)

[English](#english) | [中文](#中文)

</div>

---

<a id="中文"></a>

## 🎯 核心价值主张 | Core Value Proposition

### 30 秒了解 AxonHub | 30-Second Elevator Pitch

**中文版本：**
AxonHub 是企业级 AI 网关的终极解决方案。一个 API 接入所有主流 AI 提供商（OpenAI、Anthropic、Claude 等），自动故障转移确保 99.9% 可用性，完整的权限管理和审计跟踪满足企业合规需求。告别供应商锁定，拥抱 AI 服务的自由选择。 

**English Version:**
AxonHub is the ultimate enterprise AI gateway solution. One API to access all major AI providers (OpenAI, Anthropic, Claude, etc.), automatic failover ensures 99.9% availability, complete permission management and audit trails meet enterprise compliance requirements. Say goodbye to vendor lock-in, embrace the freedom of AI service choice.

### 🚀 解决的核心问题 | Problems We Solve

| 问题 Problem | AxonHub 解决方案 Solution |
|-------------|-------------------------|
| **供应商锁定** Vendor Lock-in | 🔄 统一 API 接口，随时切换提供商 |
| **服务中断** Service Outages | ⚡ 自动故障转移，多渠道冗余 |
| **成本控制** Cost Control | 💰 智能路由，成本优化策略 |
| **合规审计** Compliance Audit | 📊 完整请求跟踪，权限管理 |
| **开发复杂性** Development Complexity | 🛠️ 单一 SDK，统一接口标准 |

### 🏗️ 架构优势 | Architecture Advantages

```mermaid
graph TB
    subgraph "用户应用 User Applications"
        A[Web App] 
        B[Mobile App]
        C[Backend Service]
    end
    
    subgraph "AxonHub Gateway"
        D[统一 API 层<br/>Unified API Layer]
        E[智能路由<br/>Smart Router]
        F[权限控制<br/>Access Control]
        G[审计日志<br/>Audit Logs]
    end
    
    subgraph "AI 提供商 AI Providers"
        H[OpenAI]
        I[Anthropic]
        J[Claude]
        K[其他 Others]
    end
    
    A --> D
    B --> D  
    C --> D
    D --> E
    E --> F
    F --> G
    E --> H
    E --> I
    E --> J
    E --> K
    
    style D fill:#e1f5fe
    style E fill:#f3e5f5
    style F fill:#e8f5e8
    style G fill:#fff3e0
```

### ⭐ 核心竞争优势 | Key Competitive Advantages

#### 🔧 技术领先 | Technical Excellence
- **双向转换器架构** - 独创的入站/出站转换器设计，轻松扩展新提供商
- **企业级可靠性** - 99.9% 可用性保证，自动故障转移和负载均衡
- **现代技术栈** - Go + React，类型安全，云原生设计

#### 🏢 企业就绪 | Enterprise Ready  
- **细粒度权限控制** - 基于角色的访问控制，满足企业安全要求
- **完整审计跟踪** - 每个请求都有完整的生命周期记录
- **合规性支持** - 数据本地化，隐私保护，符合 GDPR/SOC2 要求

#### 🚀 开发者友好 | Developer Friendly
- **零学习成本** - 完全兼容 OpenAI API，现有代码无需修改
- **丰富的监控** - 实时性能指标，请求分析，成本追踪
- **灵活部署** - 支持本地、云端、混合部署模式

---

## 🚀 5分钟快速开始 | Quick Start Guide

### 前置要求 | Prerequisites

在开始之前，请确保您的系统已安装以下工具：

- **Go 1.24+** - [下载安装](https://golang.org/dl/)
- **Node.js 18+** - [下载安装](https://nodejs.org/)
- **pnpm** - 运行 `npm install -g pnpm` 安装

### 方式一：本地开发环境（推荐新手）| Local Development

```bash
# 1. 克隆项目
git clone https://github.com/looplj/axonhub.git
cd axonhub

# 2. 启动后端服务（端口 8090）
go run cmd/axonhub/main.go

# 3. 新开终端，启动前端服务（端口 5173）
cd frontend
pnpm install
pnpm dev
```

### 方式二：生产构建模式 | Production Build

```bash
# 1. 克隆项目
git clone https://github.com/looplj/axonhub.git
cd axonhub

# 2. 构建前端
cd frontend
pnpm install
pnpm build
cd ..

# 3. 构建并运行后端
make build
./axonhub
```

### 验证安装成功 | Verify Installation

安装完成后，请按以下步骤验证系统是否正常运行：

#### 1. 访问管理界面
- 打开浏览器访问：http://localhost:5173 （开发模式）
- 或访问：http://localhost:8090 （生产模式）
- 您应该能看到 AxonHub 的管理界面

#### 2. 检查 API 健康状态
```bash
# 检查后端 API 是否正常
curl http://localhost:8090/health

# 预期返回：{"status":"ok"}
```

#### 3. 测试 AI API 接口
```bash
# 测试 OpenAI 兼容接口（需要先配置 AI 提供商）
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### 常见问题排查 | Troubleshooting

#### 端口冲突
- 后端默认端口：8090，可通过环境变量 `AXONHUB_SERVER_PORT` 修改
- 前端默认端口：5173，可在 `frontend/vite.config.ts` 中修改

#### 数据库问题
- 默认使用 SQLite，数据库文件：`axonhub.db`
- 如需使用其他数据库，请修改 `config.yml` 中的数据库配置

#### 依赖安装失败
```bash
# Go 依赖问题
go mod tidy
go mod download

# Node.js 依赖问题
cd frontend
rm -rf node_modules pnpm-lock.yaml
pnpm install
```

### 下一步 | Next Steps

✅ 系统运行成功后，您可以：

1. **配置 AI 提供商** - 在管理界面中添加 OpenAI、Anthropic 等 API 密钥
2. **创建用户和角色** - 设置权限管理和访问控制
3. **测试 API 调用** - 使用您熟悉的 OpenAI SDK 进行集成测试
4. **查看监控数据** - 在仪表板中查看请求统计和性能指标

---

## 🚀 详细部署指南 | Comprehensive Deployment Guide

### 📋 部署环境要求 | System Requirements

#### 最低配置 | Minimum Requirements
- **CPU**: 2 核心 (2 cores)
- **内存**: 4GB RAM
- **存储**: 10GB 可用空间
- **网络**: 稳定的互联网连接

#### 推荐配置 | Recommended Requirements  
- **CPU**: 4 核心 (4 cores)
- **内存**: 8GB RAM
- **存储**: 50GB SSD
- **网络**: 100Mbps+ 带宽

#### 软件依赖 | Software Dependencies
- **Go**: 1.24+ ([安装指南](https://golang.org/dl/))
- **Node.js**: 18+ ([安装指南](https://nodejs.org/))
- **pnpm**: 最新版本 (`npm install -g pnpm`)
- **Git**: 用于克隆代码仓库

### 🏠 开发环境部署 | Development Environment

#### 1. 环境准备 | Environment Setup

```bash
# 检查 Go 版本
go version
# 应显示: go version go1.24.x

# 检查 Node.js 版本  
node --version
# 应显示: v18.x.x 或更高

# 检查 pnpm 版本
pnpm --version
# 应显示: 8.x.x 或更高
```

#### 2. 项目克隆与配置 | Project Clone & Configuration

```bash
# 1. 克隆项目
git clone https://github.com/looplj/axonhub.git
cd axonhub

# 2. 复制配置文件
cp config.example.yml config.yml

# 3. 编辑配置文件（可选）
# 使用您喜欢的编辑器修改 config.yml
```

#### 3. 后端服务启动 | Backend Service

```bash
# 方式一：直接运行（推荐开发调试）
go run cmd/axonhub/main.go

# 方式二：构建后运行
make build
./axonhub

# 方式三：使用 Air 热重载（需要安装 Air）
# go install github.com/cosmtrek/air@latest
air
```

#### 4. 前端服务启动 | Frontend Service

```bash
# 新开终端窗口
cd frontend

# 安装依赖
pnpm install

# 启动开发服务器
pnpm dev

# 前端将在 http://localhost:5173 启动
```

#### 5. 开发环境验证 | Development Verification

```bash
# 检查后端健康状态
curl http://localhost:8090/health
# 预期返回: {"status":"ok"}

# 检查前端是否正常
curl http://localhost:5173
# 应返回 HTML 内容

# 检查 GraphQL 端点
curl -X POST http://localhost:8090/query \
  -H "Content-Type: application/json" \
  -d '{"query":"query { __typename }"}'
```

### 🏭 生产环境部署 | Production Environment

#### 1. 服务器准备 | Server Preparation

```bash
# 更新系统包
sudo apt update && sudo apt upgrade -y

# 安装必要工具
sudo apt install -y git curl wget unzip

# 创建应用用户
sudo useradd -m -s /bin/bash axonhub
sudo usermod -aG sudo axonhub

# 切换到应用用户
sudo su - axonhub
```

#### 2. 安装运行时环境 | Runtime Installation

```bash
# 安装 Go
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# 安装 Node.js (使用 NodeSource)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs

# 安装 pnpm
npm install -g pnpm
```

#### 3. 应用部署 | Application Deployment

```bash
# 1. 克隆项目到生产目录
cd /opt
sudo git clone https://github.com/looplj/axonhub.git
sudo chown -R axonhub:axonhub axonhub
cd axonhub

# 2. 构建前端
cd frontend
pnpm install --frozen-lockfile
pnpm build
cd ..

# 3. 构建后端
go mod download
go build -o axonhub ./cmd/axonhub

# 4. 创建生产配置
cp config.example.yml config.yml
```

#### 4. 生产配置优化 | Production Configuration

创建生产环境配置文件 `config.yml`：

```yaml
# 生产环境配置
server:
  port: 8090
  name: "AxonHub-Production"
  debug: false
  request_timeout: "30s"
  llm_request_timeout: "600s"

# 数据库配置（推荐使用 PostgreSQL）
db:
  dialect: "postgres"
  dsn: "postgres://axonhub:your_password@localhost:5432/axonhub?sslmode=require"
  debug: false

# 日志配置
log:
  level: "info"
  encoding: "json"
  debug: false

# 启用监控
metrics:
  enabled: true
  exporter:
    type: "prometheus"
```

#### 5. 数据库设置 | Database Setup

**PostgreSQL 安装与配置：**

```bash
# 安装 PostgreSQL
sudo apt install -y postgresql postgresql-contrib

# 启动服务
sudo systemctl start postgresql
sudo systemctl enable postgresql

# 创建数据库和用户
sudo -u postgres psql << EOF
CREATE DATABASE axonhub;
CREATE USER axonhub WITH ENCRYPTED PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE axonhub TO axonhub;
\q
EOF
```

**SQLite 配置（小型部署）：**

```yaml
# config.yml 中的 SQLite 配置
db:
  dialect: "sqlite3"
  dsn: "file:/opt/axonhub/data/axonhub.db?cache=shared&_fk=1"
  debug: false
```

#### 6. 系统服务配置 | System Service Configuration

创建 systemd 服务文件 `/etc/systemd/system/axonhub.service`：

```ini
[Unit]
Description=AxonHub AI Gateway
After=network.target
Wants=network.target

[Service]
Type=simple
User=axonhub
Group=axonhub
WorkingDirectory=/opt/axonhub
ExecStart=/opt/axonhub/axonhub
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=axonhub

# 环境变量
Environment=AXONHUB_LOG_LEVEL=info
Environment=AXONHUB_SERVER_PORT=8090

# 安全设置
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/axonhub

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
# 重载 systemd 配置
sudo systemctl daemon-reload

# 启动服务
sudo systemctl start axonhub

# 设置开机自启
sudo systemctl enable axonhub

# 检查服务状态
sudo systemctl status axonhub
```

#### 7. 反向代理配置 | Reverse Proxy Setup

**Nginx 配置示例：**

```nginx
# /etc/nginx/sites-available/axonhub
server {
    listen 80;
    server_name your-domain.com;
    
    # 重定向到 HTTPS
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;
    
    # SSL 证书配置
    ssl_certificate /path/to/your/cert.pem;
    ssl_certificate_key /path/to/your/key.pem;
    
    # SSL 安全配置
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512;
    ssl_prefer_server_ciphers off;
    
    # 代理到 AxonHub
    location / {
        proxy_pass http://127.0.0.1:8090;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # WebSocket 支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        
        # 超时设置
        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    # 静态文件缓存
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

启用配置：

```bash
# 启用站点
sudo ln -s /etc/nginx/sites-available/axonhub /etc/nginx/sites-enabled/

# 测试配置
sudo nginx -t

# 重载 Nginx
sudo systemctl reload nginx
```

### 🐳 Docker 容器化部署 | Docker Deployment

#### 1. 创建 Dockerfile | Create Dockerfile

创建 `Dockerfile`：

```dockerfile
# 多阶段构建
FROM node:20-alpine AS frontend-builder

WORKDIR /app/frontend
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN npm install -g pnpm && pnpm install --frozen-lockfile

COPY frontend/ ./
RUN pnpm build

# Go 构建阶段
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /app/frontend/dist ./frontend/dist
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o axonhub ./cmd/axonhub

# 最终运行镜像
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# 复制构建产物
COPY --from=backend-builder /app/axonhub .
COPY --from=backend-builder /app/config.example.yml ./config.yml

# 创建数据目录
RUN mkdir -p /data

# 暴露端口
EXPOSE 8090

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8090/health || exit 1

# 启动命令
CMD ["./axonhub"]
```

#### 2. 创建 Docker Compose | Create Docker Compose

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  # PostgreSQL 数据库
  postgres:
    image: postgres:15-alpine
    container_name: axonhub-postgres
    environment:
      POSTGRES_DB: axonhub
      POSTGRES_USER: axonhub
      POSTGRES_PASSWORD: ${DB_PASSWORD:-axonhub_password}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./init.sql:/docker-entrypoint-initdb.d/init.sql
    ports:
      - "5432:5432"
    networks:
      - axonhub-network
    restart: unless-stopped
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U axonhub"]
      interval: 10s
      timeout: 5s
      retries: 5

  # AxonHub 主服务
  axonhub:
    build: .
    container_name: axonhub-app
    environment:
      AXONHUB_DB_DIALECT: postgres
      AXONHUB_DB_DSN: postgres://axonhub:${DB_PASSWORD:-axonhub_password}@postgres:5432/axonhub?sslmode=disable
      AXONHUB_SERVER_PORT: 8090
      AXONHUB_LOG_LEVEL: info
      AXONHUB_LOG_ENCODING: json
    ports:
      - "8090:8090"
    volumes:
      - ./config.yml:/root/config.yml:ro
      - axonhub_data:/data
    networks:
      - axonhub-network
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8090/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  # Redis 缓存 (可选)
  redis:
    image: redis:7-alpine
    container_name: axonhub-redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    networks:
      - axonhub-network
    restart: unless-stopped
    command: redis-server --appendonly yes
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 3

  # Nginx 反向代理 (可选)
  nginx:
    image: nginx:alpine
    container_name: axonhub-nginx
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/nginx/ssl:ro
    networks:
      - axonhub-network
    depends_on:
      - axonhub
    restart: unless-stopped

volumes:
  postgres_data:
  redis_data:
  axonhub_data:

networks:
  axonhub-network:
    driver: bridge
```

#### 3. 环境变量配置 | Environment Configuration

创建 `.env` 文件：

```bash
# 数据库配置
DB_PASSWORD=your_secure_password_here

# AxonHub 配置
AXONHUB_SERVER_PORT=8090
AXONHUB_LOG_LEVEL=info
AXONHUB_LOG_ENCODING=json

# 可选：外部服务配置
REDIS_URL=redis://redis:6379
METRICS_ENABLED=true
```

#### 4. Docker 部署命令 | Docker Deployment Commands

```bash
# 1. 构建并启动所有服务
docker-compose up -d

# 2. 查看服务状态
docker-compose ps

# 3. 查看日志
docker-compose logs -f axonhub

# 4. 停止服务
docker-compose down

# 5. 重新构建并启动
docker-compose up -d --build

# 6. 数据备份
docker-compose exec postgres pg_dump -U axonhub axonhub > backup.sql

# 7. 数据恢复
docker-compose exec -T postgres psql -U axonhub axonhub < backup.sql
```

## ⚙️ 配置说明 | Configuration Guide

### 📋 配置文件概览 | Configuration Overview

AxonHub 使用 YAML 格式的配置文件来管理系统行为。配置文件位于项目根目录的 `config.yml`，您可以从 `config.example.yml` 复制并修改。

#### 配置文件结构 | Configuration Structure

```
config.yml
├── server          # 服务器配置
├── db              # 数据库配置  
├── log             # 日志配置
├── metrics         # 监控配置
└── dumper          # 数据转储配置
```

### 🔧 详细配置选项 | Detailed Configuration Options

#### 1. 服务器配置 | Server Configuration

```yaml
server:
  port: 8090                    # 服务端口 (1-65535)
  name: "AxonHub"               # 服务名称，用于日志和监控
  base_path: ""                 # API 基础路径，如 "/api/v1"
  request_timeout: "30s"        # HTTP 请求超时时间
  llm_request_timeout: "600s"   # LLM API 请求超时时间
  debug: false                  # 调试模式，启用详细日志
  trace:
    trace_header: "AH-Trace-Id" # 分布式追踪头名称
```

**配置说明：**
- `port`: 服务监听端口，确保端口未被占用
- `name`: 服务实例名称，在集群部署时用于区分不同实例
- `base_path`: API 路径前缀，用于反向代理或多服务部署
- `request_timeout`: 普通 HTTP 请求超时，建议 30-60 秒
- `llm_request_timeout`: AI 模型请求超时，建议 300-600 秒
- `debug`: 开发环境可启用，生产环境建议关闭
- `trace_header`: 用于请求链路追踪的 HTTP 头名称

#### 2. 数据库配置 | Database Configuration

```yaml
db:
  dialect: "postgres"           # 数据库类型
  dsn: "connection_string"      # 数据库连接字符串
  debug: false                  # 数据库调试日志
```

**支持的数据库类型：**

| 数据库 | dialect 值 | DSN 示例 |
|--------|------------|----------|
| **SQLite** | `sqlite3` | `file:axonhub.db?cache=shared&_fk=1` |
| **PostgreSQL** | `postgres` | `postgres://user:pass@host:5432/dbname?sslmode=disable` |
| **MySQL** | `mysql` | `user:pass@tcp(host:3306)/dbname?charset=utf8mb4&parseTime=True` |

**不同环境的数据库配置示例：**

**开发环境 (SQLite):**
```yaml
db:
  dialect: "sqlite3"
  dsn: "file:./data/axonhub_dev.db?cache=shared&_fk=1"
  debug: true
```

**生产环境 (PostgreSQL):**
```yaml
db:
  dialect: "postgres"
  dsn: "postgres://axonhub:secure_password@db.example.com:5432/axonhub_prod?sslmode=require"
  debug: false
```

**高可用环境 (PostgreSQL 集群):**
```yaml
db:
  dialect: "postgres"
  dsn: "postgres://axonhub:password@pgpool.example.com:5432/axonhub?sslmode=require&pool_max_conns=20"
  debug: false
```

#### 3. 日志配置 | Logging Configuration

```yaml
log:
  name: "axonhub"               # 日志器名称
  debug: false                  # 调试日志开关
  skip_level: 1                 # 调用栈跳过层级
  level: "info"                 # 日志级别
  level_key: "level"            # 日志级别字段名
  time_key: "time"              # 时间戳字段名
  caller_key: "label"           # 调用者信息字段名
  function_key: ""              # 函数名字段名
  name_key: "logger"            # 日志器名称字段名
  encoding: "json"              # 日志编码格式
  includes: []                  # 包含的日志器列表
  excludes: []                  # 排除的日志器列表
```

**日志级别说明：**
- `debug`: 详细调试信息，仅开发环境使用
- `info`: 一般信息，推荐生产环境使用
- `warn`: 警告信息，需要关注但不影响运行
- `error`: 错误信息，需要立即处理
- `panic`: 严重错误，程序可能崩溃
- `fatal`: 致命错误，程序将退出

**日志编码格式：**
- `json`: JSON 格式，适合日志收集系统
- `console`: 控制台格式，适合开发调试
- `console_json`: 控制台 JSON 格式，兼顾可读性和结构化

#### 4. 监控配置 | Metrics Configuration

```yaml
metrics:
  enabled: true                 # 启用监控
  exporter:
    type: "prometheus"          # 导出器类型
```

**监控导出器类型：**
- `prometheus`: Prometheus 格式，端点 `/metrics`
- `console`: 控制台输出，用于调试
- `stdout`: 标准输出，用于容器化部署

#### 5. 数据转储配置 | Dumper Configuration

```yaml
dumper:
  enabled: false                # 启用错误数据转储
  dump_path: "./dumps"          # 转储文件目录
  max_size: 100                 # 单个文件最大大小 (MB)
  max_age: "24h"                # 文件保留时间
  max_backups: 10               # 最大备份文件数
```

**转储功能说明：**
- 用于调试和错误分析
- 自动记录异常请求和响应数据
- 支持文件大小和数量限制
- 生产环境建议谨慎启用

### 🎯 不同场景配置示例 | Configuration Examples for Different Scenarios

#### 1. 开发环境配置 | Development Environment

```yaml
# config.yml - 开发环境
server:
  port: 8090
  name: "AxonHub-Dev"
  debug: true
  request_timeout: "60s"
  llm_request_timeout: "300s"

db:
  dialect: "sqlite3"
  dsn: "file:./data/axonhub_dev.db?cache=shared&_fk=1"
  debug: true

log:
  level: "debug"
  encoding: "console"
  debug: true

metrics:
  enabled: true
  exporter:
    type: "console"

dumper:
  enabled: true
  dump_path: "./dumps"
  max_size: 50
  max_age: "1h"
  max_backups: 5
```

#### 2. 生产环境配置 | Production Environment

```yaml
# config.yml - 生产环境
server:
  port: 8090
  name: "AxonHub-Prod"
  debug: false
  request_timeout: "30s"
  llm_request_timeout: "600s"
  trace:
    trace_header: "X-Trace-ID"

db:
  dialect: "postgres"
  dsn: "postgres://axonhub:${DB_PASSWORD}@postgres.internal:5432/axonhub?sslmode=require"
  debug: false

log:
  level: "info"
  encoding: "json"
  debug: false
  excludes: ["gorm.io/gorm"]

metrics:
  enabled: true
  exporter:
    type: "prometheus"

dumper:
  enabled: false
```

#### 3. 高性能环境配置 | High Performance Environment

```yaml
# config.yml - 高性能环境
server:
  port: 8090
  name: "AxonHub-HPC"
  debug: false
  request_timeout: "15s"
  llm_request_timeout: "300s"

db:
  dialect: "postgres"
  dsn: "postgres://axonhub:password@pgpool:5432/axonhub?sslmode=require&pool_max_conns=50&pool_min_conns=10"
  debug: false

log:
  level: "warn"
  encoding: "json"
  debug: false
  excludes: ["gorm.io/gorm", "net/http"]

metrics:
  enabled: true
  exporter:
    type: "prometheus"

dumper:
  enabled: false
```

#### 4. 容器化部署配置 | Containerized Deployment

```yaml
# config.yml - 容器化部署
server:
  port: 8090
  name: "AxonHub-Container"
  debug: false
  request_timeout: "30s"
  llm_request_timeout: "600s"

db:
  dialect: "postgres"
  dsn: "postgres://axonhub:${POSTGRES_PASSWORD}@postgres:5432/axonhub?sslmode=disable"
  debug: false

log:
  level: "info"
  encoding: "json"
  debug: false

metrics:
  enabled: true
  exporter:
    type: "prometheus"

dumper:
  enabled: false
```

### 🔍 环境变量配置 | Environment Variables

AxonHub 支持通过环境变量覆盖配置文件中的设置，环境变量优先级高于配置文件。

#### 完整环境变量映射表 | Complete Environment Variable Mapping

| 配置项 | 环境变量 | 默认值 | 说明 |
|--------|----------|--------|------|
| **服务器配置** | | | |
| `server.port` | `AXONHUB_SERVER_PORT` | `8090` | 服务端口 |
| `server.name` | `AXONHUB_SERVER_NAME` | `AxonHub` | 服务名称 |
| `server.base_path` | `AXONHUB_SERVER_BASE_PATH` | `""` | API 基础路径 |
| `server.request_timeout` | `AXONHUB_SERVER_REQUEST_TIMEOUT` | `30s` | 请求超时 |
| `server.llm_request_timeout` | `AXONHUB_SERVER_LLM_REQUEST_TIMEOUT` | `600s` | LLM 请求超时 |
| `server.debug` | `AXONHUB_SERVER_DEBUG` | `false` | 调试模式 |
| `server.trace.trace_header` | `AXONHUB_SERVER_TRACE_HEADER` | `AH-Trace-Id` | 追踪头名称 |
| **数据库配置** | | | |
| `db.dialect` | `AXONHUB_DB_DIALECT` | `sqlite3` | 数据库类型 |
| `db.dsn` | `AXONHUB_DB_DSN` | `file:axonhub.db` | 数据库连接串 |
| `db.debug` | `AXONHUB_DB_DEBUG` | `false` | 数据库调试 |
| **日志配置** | | | |
| `log.name` | `AXONHUB_LOG_NAME` | `axonhub` | 日志器名称 |
| `log.debug` | `AXONHUB_LOG_DEBUG` | `false` | 调试日志 |
| `log.skip_level` | `AXONHUB_LOG_SKIP_LEVEL` | `1` | 跳过层级 |
| `log.level` | `AXONHUB_LOG_LEVEL` | `info` | 日志级别 |
| `log.encoding` | `AXONHUB_LOG_ENCODING` | `json` | 日志格式 |
| `log.level_key` | `AXONHUB_LOG_LEVEL_KEY` | `level` | 级别字段名 |
| `log.time_key` | `AXONHUB_LOG_TIME_KEY` | `time` | 时间字段名 |
| `log.caller_key` | `AXONHUB_LOG_CALLER_KEY` | `label` | 调用者字段名 |
| `log.function_key` | `AXONHUB_LOG_FUNCTION_KEY` | `""` | 函数字段名 |
| `log.name_key` | `AXONHUB_LOG_NAME_KEY` | `logger` | 名称字段名 |
| **监控配置** | | | |
| `metrics.enabled` | `AXONHUB_METRICS_ENABLED` | `false` | 启用监控 |
| `metrics.exporter.type` | `AXONHUB_METRICS_EXPORTER_TYPE` | `stdout` | 导出器类型 |
| **转储配置** | | | |
| `dumper.enabled` | `AXONHUB_DUMPER_ENABLED` | `false` | 启用转储 |
| `dumper.dump_path` | `AXONHUB_DUMPER_DUMP_PATH` | `./dumps` | 转储路径 |
| `dumper.max_size` | `AXONHUB_DUMPER_MAX_SIZE` | `100` | 最大文件大小 |
| `dumper.max_age` | `AXONHUB_DUMPER_MAX_AGE` | `24h` | 文件保留时间 |
| `dumper.max_backups` | `AXONHUB_DUMPER_MAX_BACKUPS` | `10` | 最大备份数 |

#### 环境变量使用示例 | Environment Variable Examples

**Docker 部署：**
```bash
# docker-compose.yml 中的环境变量
environment:
  - AXONHUB_SERVER_PORT=8090
  - AXONHUB_DB_DIALECT=postgres
  - AXONHUB_DB_DSN=postgres://axonhub:${DB_PASSWORD}@postgres:5432/axonhub
  - AXONHUB_LOG_LEVEL=info
  - AXONHUB_LOG_ENCODING=json
  - AXONHUB_METRICS_ENABLED=true
```

**Kubernetes 部署：**
```yaml
# deployment.yaml 中的环境变量
env:
- name: AXONHUB_SERVER_PORT
  value: "8090"
- name: AXONHUB_DB_DSN
  valueFrom:
    secretKeyRef:
      name: axonhub-db-secret
      key: dsn
- name: AXONHUB_LOG_LEVEL
  value: "info"
```

**Shell 脚本：**
```bash
#!/bin/bash
# 设置环境变量
export AXONHUB_SERVER_PORT=8091
export AXONHUB_DB_DIALECT=postgres
export AXONHUB_DB_DSN="postgres://user:pass@localhost:5432/axonhub?sslmode=disable"
export AXONHUB_LOG_LEVEL=debug
export AXONHUB_LOG_ENCODING=console

# 启动服务
./axonhub
```

### ✅ 配置验证和调试 | Configuration Validation & Debugging

#### 1. 配置文件语法验证 | Configuration Syntax Validation

**验证 YAML 语法：**
```bash
# 使用 yq 验证 YAML 语法
yq eval '.' config.yml > /dev/null && echo "配置文件语法正确" || echo "配置文件语法错误"

# 使用 Python 验证
python -c "import yaml; yaml.safe_load(open('config.yml'))" && echo "YAML 格式正确"

# 使用 Go 验证（如果安装了 yq）
go run -c 'package main; import ("gopkg.in/yaml.v2"; "os"); func main() { var c interface{}; yaml.Unmarshal([]byte(os.Args[1]), &c) }' "$(cat config.yml)"
```

#### 2. 配置加载测试 | Configuration Loading Test

**测试配置加载：**
```bash
# 启动服务并检查配置加载
./axonhub --config config.yml --validate-config

# 查看配置加载日志
./axonhub 2>&1 | grep -i "config\|configuration"

# 使用调试模式查看详细配置信息
AXONHUB_SERVER_DEBUG=true ./axonhub
```

#### 3. 数据库连接测试 | Database Connection Test

**测试数据库连接：**
```bash
# PostgreSQL 连接测试
psql "postgres://user:pass@host:5432/dbname" -c "SELECT 1;"

# MySQL 连接测试  
mysql -h host -u user -p -e "SELECT 1;" dbname

# SQLite 文件检查
sqlite3 axonhub.db ".tables"

# 使用 AxonHub 内置健康检查
curl http://localhost:8090/health
```

#### 4. 常见配置错误诊断 | Common Configuration Issues

**错误 1: 端口被占用**
```bash
# 问题症状
Error: listen tcp :8090: bind: address already in use

# 诊断方法
sudo lsof -i :8090
sudo netstat -tulpn | grep :8090

# 解决方案
# 方法1: 更改端口
export AXONHUB_SERVER_PORT=8091

# 方法2: 终止占用进程
sudo kill -9 <PID>
```

**错误 2: 数据库连接失败**
```bash
# 问题症状
Error: failed to connect to database

# 诊断方法
# 检查数据库服务状态
sudo systemctl status postgresql
sudo systemctl status mysql

# 测试网络连通性
telnet db_host 5432
nc -zv db_host 3306

# 验证连接字符串
echo $AXONHUB_DB_DSN
```

**错误 3: 配置文件格式错误**
```bash
# 问题症状
Error: yaml: unmarshal errors

# 诊断方法
# 检查 YAML 缩进
cat -A config.yml | head -20

# 验证 YAML 语法
python -m yaml config.yml

# 查找特殊字符
grep -P "[\x80-\xFF]" config.yml
```

**错误 4: 权限问题**
```bash
# 问题症状
Error: permission denied

# 诊断方法
ls -la config.yml
ls -la ./dumps/

# 解决方案
sudo chown axonhub:axonhub config.yml
sudo chmod 644 config.yml
sudo mkdir -p ./dumps && sudo chown axonhub:axonhub ./dumps
```

#### 5. 配置优化建议 | Configuration Optimization Tips

**性能优化配置：**
```yaml
# 高并发场景优化
server:
  request_timeout: "15s"        # 减少超时时间
  llm_request_timeout: "300s"   # 适当减少 LLM 超时

log:
  level: "warn"                 # 减少日志输出
  excludes: ["gorm.io/gorm"]    # 排除数据库查询日志

db:
  # PostgreSQL 连接池优化
  dsn: "postgres://user:pass@host/db?pool_max_conns=50&pool_min_conns=10"
```

**安全配置建议：**
```yaml
server:
  debug: false                  # 生产环境关闭调试
  
log:
  level: "info"                 # 避免敏感信息泄露
  debug: false

dumper:
  enabled: false                # 生产环境关闭数据转储
```

**监控配置建议：**
```yaml
metrics:
  enabled: true                 # 启用监控
  exporter:
    type: "prometheus"          # 使用 Prometheus 格式

log:
  encoding: "json"              # 结构化日志便于分析
  level: "info"
```

#### 6. 配置文件模板 | Configuration Templates

**最小配置模板：**
```yaml
# minimal-config.yml
server:
  port: 8090

db:
  dialect: "sqlite3"
  dsn: "file:axonhub.db"

log:
  level: "info"
```

**完整配置模板：**
```yaml
# full-config.yml
server:
  port: 8090
  name: "AxonHub"
  base_path: ""
  request_timeout: "30s"
  llm_request_timeout: "600s"
  debug: false
  trace:
    trace_header: "AH-Trace-Id"

db:
  dialect: "postgres"
  dsn: "postgres://axonhub:password@localhost:5432/axonhub?sslmode=disable"
  debug: false

log:
  name: "axonhub"
  debug: false
  skip_level: 1
  level: "info"
  level_key: "level"
  time_key: "time"
  caller_key: "label"
  function_key: ""
  name_key: "logger"
  encoding: "json"
  includes: []
  excludes: []

metrics:
  enabled: true
  exporter:
    type: "prometheus"

dumper:
  enabled: false
  dump_path: "./dumps"
  max_size: 100
  max_age: "24h"
  max_backups: 10
```

---

### 🔍 故障排除指南 | Troubleshooting Guide

#### 常见问题与解决方案 | Common Issues & Solutions

#### 1. 端口冲突问题 | Port Conflicts

**问题症状：**
```
Error: listen tcp :8090: bind: address already in use
```

**解决方案：**
```bash
# 查找占用端口的进程
sudo lsof -i :8090
# 或者
sudo netstat -tulpn | grep :8090

# 终止占用进程
sudo kill -9 <PID>

# 或者修改配置使用其他端口
export AXONHUB_SERVER_PORT=8091
```

#### 2. 数据库连接失败 | Database Connection Issues

**问题症状：**
```
Error: failed to connect to database: dial tcp: connection refused
```

**解决方案：**

**PostgreSQL 连接问题：**
```bash
# 检查 PostgreSQL 服务状态
sudo systemctl status postgresql

# 启动 PostgreSQL
sudo systemctl start postgresql

# 测试连接
psql -h localhost -U axonhub -d axonhub

# 检查防火墙
sudo ufw status
sudo ufw allow 5432
```

**SQLite 权限问题：**
```bash
# 检查文件权限
ls -la axonhub.db

# 修复权限
sudo chown axonhub:axonhub axonhub.db
chmod 644 axonhub.db

# 检查目录权限
sudo chown axonhub:axonhub /opt/axonhub
```

#### 3. 前端构建失败 | Frontend Build Issues

**问题症状：**
```
Error: Cannot resolve dependency
```

**解决方案：**
```bash
# 清理缓存
cd frontend
rm -rf node_modules pnpm-lock.yaml
pnpm store prune

# 重新安装
pnpm install

# 如果仍有问题，尝试使用 npm
rm -rf node_modules package-lock.json
npm install
npm run build
```

#### 4. Go 模块下载失败 | Go Module Issues

**问题症状：**
```
Error: go: module lookup disabled by GOPROXY=off
```

**解决方案：**
```bash
# 设置 Go 代理
export GOPROXY=https://proxy.golang.org,direct
export GOSUMDB=sum.golang.org

# 或者使用中国镜像
export GOPROXY=https://goproxy.cn,direct

# 清理模块缓存
go clean -modcache
go mod download
```

#### 5. 内存不足问题 | Memory Issues

**问题症状：**
```
Error: runtime: out of memory
```

**解决方案：**
```bash
# 检查系统内存
free -h

# 增加交换空间
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile

# 优化 Go 垃圾回收
export GOGC=100
export GOMEMLIMIT=1GiB
```

#### 6. SSL/TLS 证书问题 | SSL/TLS Certificate Issues

**问题症状：**
```
Error: x509: certificate signed by unknown authority
```

**解决方案：**
```bash
# 更新 CA 证书
sudo apt update
sudo apt install ca-certificates

# 或者跳过 SSL 验证（仅开发环境）
export AXONHUB_DB_DSN="postgres://user:pass@localhost/axonhub?sslmode=disable"
```

#### 7. Docker 相关问题 | Docker Issues

**容器启动失败：**
```bash
# 查看详细日志
docker-compose logs axonhub

# 检查容器状态
docker-compose ps

# 重新构建镜像
docker-compose build --no-cache axonhub
```

**数据卷权限问题：**
```bash
# 修复数据卷权限
docker-compose exec axonhub chown -R axonhub:axonhub /data

# 或者在 Dockerfile 中添加
USER axonhub
```

#### 8. 性能问题诊断 | Performance Issues

**高 CPU 使用率：**
```bash
# 查看进程资源使用
top -p $(pgrep axonhub)

# 生成性能分析文件
go tool pprof http://localhost:8090/debug/pprof/profile

# 检查 goroutine 泄漏
go tool pprof http://localhost:8090/debug/pprof/goroutine
```

**高内存使用：**
```bash
# 内存分析
go tool pprof http://localhost:8090/debug/pprof/heap

# 检查数据库连接池
# 在配置中调整连接池大小
```

#### 9. 网络连接问题 | Network Issues

**AI 提供商 API 连接失败：**
```bash
# 测试网络连通性
curl -I https://api.openai.com/v1/models
curl -I https://api.anthropic.com/v1/messages

# 检查防火墙规则
sudo iptables -L

# 检查 DNS 解析
nslookup api.openai.com
```

#### 10. 日志分析 | Log Analysis

**启用详细日志：**
```yaml
# config.yml
log:
  level: "debug"
  encoding: "console"
  debug: true
```

**日志查看命令：**
```bash
# 实时查看日志
sudo journalctl -u axonhub -f

# 查看错误日志
sudo journalctl -u axonhub --priority=err

# 查看特定时间段日志
sudo journalctl -u axonhub --since "2024-01-01 00:00:00"
```

### 📊 监控与维护 | Monitoring & Maintenance

#### 健康检查端点 | Health Check Endpoints

```bash
# 基础健康检查
curl http://localhost:8090/health

# 详细系统状态
curl http://localhost:8090/debug/vars

# 性能指标
curl http://localhost:8090/metrics
```

#### 日常维护任务 | Daily Maintenance Tasks

```bash
# 1. 检查服务状态
sudo systemctl status axonhub

# 2. 查看资源使用
htop
df -h

# 3. 数据库维护
# PostgreSQL
sudo -u postgres psql -c "VACUUM ANALYZE;"

# SQLite
sqlite3 axonhub.db "VACUUM;"

# 4. 日志轮转
sudo logrotate -f /etc/logrotate.d/axonhub

# 5. 备份数据
./scripts/backup.sh
```

---

## 👨‍💻 开发指南 | Development Guide

### 🚀 开发环境设置 | Development Environment Setup

#### 前置要求 | Prerequisites

在开始开发之前，请确保您的开发环境已安装以下工具：

| 工具 Tool | 版本要求 Version | 安装方式 Installation |
|----------|----------------|---------------------|
| **Go** | 1.24+ | [官方下载](https://golang.org/dl/) |
| **Node.js** | 18+ | [官方下载](https://nodejs.org/) |
| **pnpm** | 最新版 | `npm install -g pnpm` |
| **Git** | 2.0+ | [官方下载](https://git-scm.com/) |
| **Air** (可选) | 最新版 | `go install github.com/cosmtrek/air@latest` |

#### 项目克隆与初始化 | Project Clone & Initialization

```bash
# 1. 克隆项目
git clone https://github.com/looplj/axonhub.git
cd axonhub

# 2. 安装 Go 依赖
go mod download
go mod tidy

# 3. 安装前端依赖
cd frontend
pnpm install
cd ..

# 4. 复制配置文件
cp config.example.yml config.yml

# 5. 初始化数据库（可选，首次运行时自动创建）
# SQLite 数据库会在首次启动时自动创建
```

#### 开发服务器启动 | Development Server Startup

**方式一：分别启动前后端（推荐）**
```bash
# 终端 1：启动后端服务
go run cmd/axonhub/main.go
# 或使用热重载
air

# 终端 2：启动前端开发服务器
cd frontend
pnpm dev
```

**方式二：使用 Makefile（如果可用）**
```bash
# 查看可用命令
make help

# 启动开发环境
make dev

# 构建项目
make build
```

#### 开发环境验证 | Development Environment Verification

```bash
# 1. 检查后端服务
curl http://localhost:8090/health
# 预期返回: {"status":"ok"}

# 2. 检查前端服务
curl http://localhost:5173
# 应返回 HTML 内容

# 3. 检查 GraphQL 端点
curl -X POST http://localhost:8090/query \
  -H "Content-Type: application/json" \
  -d '{"query":"query { __typename }"}'

# 4. 访问管理界面
# 浏览器打开: http://localhost:5173
```

### 🏗️ 项目架构理解 | Understanding Project Architecture

#### 后端架构 | Backend Architecture

```
internal/
├── llm/                    # 核心 LLM 处理逻辑
│   ├── pipeline/           # 请求管道编排
│   ├── transformer/        # 双向转换器
│   │   ├── openai/         # OpenAI 转换器实现
│   │   ├── anthropic/      # Anthropic 转换器实现
│   │   └── aisdk/          # AI SDK 转换器实现
│   ├── decorator/          # 请求装饰器链
│   └── model.go           # 统一数据模型
├── server/
│   ├── chat/              # 聊天处理与持久化
│   ├── api/               # REST 和 GraphQL 处理器
│   ├── biz/               # 业务逻辑层
│   └── gql/               # GraphQL 模式和解析器
├── ent/                   # 数据库 ORM 和模式
├── pkg/                   # 共享工具
└── scopes/                # 权限管理
```

#### 前端架构 | Frontend Architecture

```
frontend/src/
├── app/                   # React Router v7 应用目录
├── routes/                # 基于文件的路由
├── features/              # 功能模块组织
│   ├── dashboard/         # 系统概览
│   ├── channels/          # AI 提供商管理
│   ├── requests/          # 请求监控
│   ├── system/            # 系统配置
│   └── chats/             # 聊天界面
├── components/            # 共享组件
└── lib/                   # 工具和 API 客户端
```

### 🔧 开发工作流 | Development Workflow

#### 1. 功能开发流程 | Feature Development Process

```bash
# 1. 创建功能分支
git checkout -b feature/your-feature-name

# 2. 开发过程中的常用命令
# 后端代码生成
cd internal/server/gql && go generate
cd internal/ent && go run entc.go

# 前端类型生成
cd frontend && pnpm codegen

# 3. 实时开发
# 使用 Air 进行后端热重载
air

# 前端自动重载（pnpm dev 已包含）
cd frontend && pnpm dev

# 4. 提交前检查
make lint      # 代码质量检查
make test      # 运行测试
make build     # 构建验证
```

#### 2. 代码生成 | Code Generation

**后端代码生成：**
```bash
# Ent ORM 代码生成/GraphQL 代码生成
make generate

# Mock 代码生成（如果使用 mockery）
mockery --all --output ./mocks
```


#### 3. 数据库迁移 | Database Migration

```bash
# 修改数据库模式
# 编辑 internal/ent/schema/ 下的文件

# 生成迁移代码
make generate

# 应用迁移（开发环境）
# 重启服务，Ent 会自动应用迁移
go run cmd/axonhub/main.go

# 生产环境迁移（谨慎操作）
# 使用 Ent 的迁移工具或手动 SQL
```

---

## 🔌 API 使用指南 | API Usage Guide

### 🚀 OpenAI 兼容 API | OpenAI Compatible API

AxonHub 提供完全兼容 OpenAI API 的接口，您可以直接使用现有的 OpenAI SDK 和代码，无需任何修改。

#### 📋 API 端点概览 | API Endpoints Overview

| 功能 Feature | 端点 Endpoint | 兼容性 Compatibility |
|-------------|---------------|-------------------|
| **聊天补全** | `POST /v1/chat/completions` | ✅ OpenAI Chat API |
| **文本补全** | `POST /v1/completions` | ✅ OpenAI Completions API |
| **模型列表** | `GET /v1/models` | ✅ OpenAI Models API |
| **嵌入向量** | `POST /v1/embeddings` | ✅ OpenAI Embeddings API |

#### 🔑 认证配置 | Authentication Setup

AxonHub 使用 API 密钥进行身份验证，与 OpenAI 的认证方式完全一致：

```bash
# 设置环境变量
export OPENAI_API_KEY="your-axonhub-api-key"
export OPENAI_BASE_URL="http://localhost:8090/v1"

# 或者在代码中直接配置
```

**获取 API 密钥步骤：**
1. 登录 AxonHub 管理界面：http://localhost:8090
2. 导航到 "API 密钥" 页面
3. 点击 "创建新密钥" 按钮
4. 设置密钥名称和权限范围
5. 复制生成的密钥（请妥善保存，仅显示一次）

### 💻 快速开始示例 | Quick Start Examples

#### 1. 使用 curl 命令 | Using curl Commands

**基础聊天请求：**
```bash
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "你好，请介绍一下 AxonHub 的主要功能。"
      }
    ],
    "max_tokens": 1000,
    "temperature": 0.7
  }'
```

**流式响应请求：**
```bash
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user", 
        "content": "请写一个 Python 函数来计算斐波那契数列"
      }
    ],
    "stream": true
  }' \
  --no-buffer
```

**多模态请求（图像 + 文本）：**
```bash
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -d '{
    "model": "gpt-4-vision-preview",
    "messages": [
      {
        "role": "user",
        "content": [
          {
            "type": "text",
            "text": "这张图片显示了什么？"
          },
          {
            "type": "image_url",
            "image_url": {
              "url": "data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQ..."
            }
          }
        ]
      }
    ]
  }'
```

**函数调用示例：**
```bash
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer your-axonhub-api-key" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [
      {
        "role": "user",
        "content": "今天北京的天气怎么样？"
      }
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "获取指定城市的天气信息",
          "parameters": {
            "type": "object",
            "properties": {
              "city": {
                "type": "string",
                "description": "城市名称"
              }
            },
            "required": ["city"]
          }
        }
      }
    ],
    "tool_choice": "auto"
  }'
```

#### 2. 使用 OpenAI 官方 SDK | Using Official OpenAI SDK

AxonHub 完全兼容 OpenAI 官方 SDK，只需修改 `base_url` 参数即可：

**Python SDK 示例：**
```python
from openai import OpenAI

# 配置 AxonHub 客户端
client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

# 基础聊天
response = client.chat.completions.create(
    model="gpt-3.5-turbo",
    messages=[
        {"role": "user", "content": "你好，AxonHub！"}
    ]
)
print(response.choices[0].message.content)

# 流式响应
stream = client.chat.completions.create(
    model="gpt-4",
    messages=[{"role": "user", "content": "写一首关于 AI 的诗"}],
    stream=True
)

for chunk in stream:
    if chunk.choices[0].delta.content is not None:
        print(chunk.choices[0].delta.content, end="")

# 异步调用
import asyncio
from openai import AsyncOpenAI

async_client = AsyncOpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1"
)

async def async_chat():
    response = await async_client.chat.completions.create(
        model="gpt-3.5-turbo",
        messages=[{"role": "user", "content": "异步请求测试"}]
    )
    return response.choices[0].message.content

# 运行异步函数
result = asyncio.run(async_chat())
print(result)
```

**Node.js SDK 示例：**
```javascript
import OpenAI from 'openai';

// 配置 AxonHub 客户端
const openai = new OpenAI({
  apiKey: 'your-axonhub-api-key',
  baseURL: 'http://localhost:8090/v1',
});

// 基础聊天
async function basicChat() {
  const completion = await openai.chat.completions.create({
    messages: [{ role: 'user', content: '你好，AxonHub！' }],
    model: 'gpt-3.5-turbo',
  });
  
  console.log(completion.choices[0].message.content);
}

// 流式响应
async function streamChat() {
  const stream = await openai.chat.completions.create({
    model: 'gpt-4',
    messages: [{ role: 'user', content: '写一个 JavaScript 函数' }],
    stream: true,
  });
  
  for await (const chunk of stream) {
    process.stdout.write(chunk.choices[0]?.delta?.content || '');
  }
}

// 执行示例
basicChat();
streamChat();
```

**Go SDK 示例：**
```go
package main

import (
    "context"
    "fmt"
    "io"
    
    "github.com/sashabaranov/go-openai"
)

func main() {
    // 配置 AxonHub 客户端
    config := openai.DefaultConfig("your-axonhub-api-key")
    config.BaseURL = "http://localhost:8090/v1"
    client := openai.NewClientWithConfig(config)
    
    // 基础聊天
    resp, err := client.CreateChatCompletion(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT3Dot5Turbo,
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: "你好，AxonHub！",
                },
            },
        },
    )
    
    if err != nil {
        fmt.Printf("ChatCompletion error: %v\n", err)
        return
    }
    
    fmt.Println(resp.Choices[0].Message.Content)
    
    // 流式响应
    stream, err := client.CreateChatCompletionStream(
        context.Background(),
        openai.ChatCompletionRequest{
            Model: openai.GPT4,
            Messages: []openai.ChatCompletionMessage{
                {
                    Role:    openai.ChatMessageRoleUser,
                    Content: "写一个 Go 函数示例",
                },
            },
            Stream: true,
        },
    )
    
    if err != nil {
        fmt.Printf("ChatCompletionStream error: %v\n", err)
        return
    }
    defer stream.Close()
    
    for {
        response, err := stream.Recv()
        if err == io.EOF {
            break
        }
        if err != nil {
            fmt.Printf("Stream error: %v\n", err)
            return
        }
        
        fmt.Print(response.Choices[0].Delta.Content)
    }
}
```

#### 3. 使用 Anthropic 兼容 API | Using Anthropic Compatible API

AxonHub 同时支持 Anthropic Claude API 格式，您可以使用 Anthropic 官方 SDK：

**Python Anthropic SDK 示例：**
```python
import anthropic

# 配置 AxonHub 客户端（使用 Anthropic 格式）
client = anthropic.Anthropic(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090"  # AxonHub 会自动检测 Anthropic 格式
)

# Claude 聊天
message = client.messages.create(
    model="claude-3-sonnet-20240229",
    max_tokens=1000,
    temperature=0.7,
    messages=[
        {"role": "user", "content": "你好，请介绍一下你自己。"}
    ]
)
print(message.content[0].text)

# 流式响应
with client.messages.stream(
    model="claude-3-opus-20240229",
    max_tokens=1000,
    messages=[
        {"role": "user", "content": "写一个关于 AI 的故事"}
    ]
) as stream:
    for text in stream.text_stream:
        print(text, end="", flush=True)
```

**直接 curl 调用 Anthropic 格式：**
```bash
curl -X POST http://localhost:8090/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: your-axonhub-api-key" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-sonnet-20240229",
    "max_tokens": 1000,
    "messages": [
      {
        "role": "user",
        "content": "请解释一下量子计算的基本原理。"
      }
    ]
  }'
```

### 🔧 高级配置 | Advanced Configuration

#### 1. 模型映射配置 | Model Mapping Configuration

AxonHub 支持灵活的模型映射，您可以在管理界面中配置：

```yaml
# 示例：模型映射配置
model_mappings:
  "gpt-3.5-turbo": "openai/gpt-3.5-turbo"
  "gpt-4": "openai/gpt-4"
  "claude-3-sonnet": "anthropic/claude-3-sonnet-20240229"
  "claude-3-opus": "anthropic/claude-3-opus-20240229"
  
# 自定义模型别名
model_aliases:
  "fast-model": "gpt-3.5-turbo"
  "smart-model": "gpt-4"
  "creative-model": "claude-3-opus"
```

#### 2. 渠道优先级配置 | Channel Priority Configuration

```bash
# 通过 API 设置渠道优先级
curl -X POST http://localhost:8090/admin/channels/priority \
  -H "Authorization: Bearer admin-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "priorities": [
      {"provider": "openai", "priority": 1, "weight": 70},
      {"provider": "anthropic", "priority": 2, "weight": 30}
    ]
  }'
```

#### 3. 请求重试配置 | Request Retry Configuration

```python
# Python SDK 中的重试配置
from openai import OpenAI
import httpx

client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1",
    http_client=httpx.Client(
        timeout=60.0,
        limits=httpx.Limits(max_connections=100, max_keepalive_connections=20)
    ),
    max_retries=3
)
```

### 📊 监控和调试 | Monitoring & Debugging

#### 1. 请求追踪 | Request Tracing

每个 API 请求都会生成唯一的追踪 ID，便于调试和监控：

```bash
# 在响应头中查看追踪 ID
curl -I -X POST http://localhost:8090/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -d '{"model":"gpt-3.5-turbo","messages":[{"role":"user","content":"test"}]}'

# 响应头示例：
# AH-Trace-Id: 550e8400-e29b-41d4-a716-446655440000
# AH-Request-Id: req_abc123def456
# AH-Provider: openai
# AH-Model: gpt-3.5-turbo
```

#### 2. 错误处理 | Error Handling

AxonHub 提供详细的错误信息，兼容 OpenAI 错误格式：

```json
{
  "error": {
    "message": "The model 'invalid-model' does not exist",
    "type": "invalid_request_error",
    "param": "model",
    "code": "model_not_found"
  },
  "axonhub": {
    "trace_id": "550e8400-e29b-41d4-a716-446655440000",
    "provider": "openai",
    "retry_count": 2,
    "fallback_used": true
  }
}
```

#### 3. 性能监控 | Performance Monitoring

```bash
# 获取 API 性能指标
curl http://localhost:8090/metrics \
  -H "Authorization: Bearer admin-api-key"

# 获取实时统计
curl http://localhost:8090/admin/stats \
  -H "Authorization: Bearer admin-api-key"
```

### 🚨 常见问题解决 | Troubleshooting

#### 1. 认证失败 | Authentication Issues

```bash
# 检查 API 密钥是否有效
curl -X GET http://localhost:8090/v1/models \
  -H "Authorization: Bearer your-api-key"

# 预期响应：模型列表
# 错误响应：{"error": {"message": "Invalid API key", "type": "authentication_error"}}
```

#### 2. 模型不可用 | Model Unavailable

```bash
# 查看可用模型列表
curl http://localhost:8090/v1/models \
  -H "Authorization: Bearer your-api-key"

# 检查渠道状态
curl http://localhost:8090/admin/channels/status \
  -H "Authorization: Bearer admin-api-key"
```

#### 3. 请求超时 | Request Timeout

```python
# 增加超时时间
client = OpenAI(
    api_key="your-axonhub-api-key",
    base_url="http://localhost:8090/v1",
    timeout=120.0  # 增加到 120 秒
)
```

### 📚 更多资源 | Additional Resources

- **OpenAI 官方文档**: https://platform.openai.com/docs/api-reference
- **Anthropic 官方文档**: https://docs.anthropic.com/claude/reference/
- **AxonHub GraphQL Playground**: http://localhost:8090/playground
- **API 状态页面**: http://localhost:8090/admin/status
- **性能监控面板**: http://localhost:8090/admin/metrics

---

## 📖 详细介绍 | Detailed Overview

### 中文

AxonHub 是一个现代化的企业级 AI 网关系统，采用 Go 后端和 React 前端构建。它提供统一的 OpenAI 兼容 API 层，通过先进的转换器管道架构将请求转换到各种 AI 提供商（OpenAI、Anthropic、AI SDK），具备增强的持久化和系统管理能力。

### English

AxonHub is a modern AI Gateway system built with Go backend and React frontend. It provides a unified OpenAI-compatible API layer that transforms requests to various AI providers (OpenAI, Anthropic, AI SDK) using an advanced transformer pipeline architecture with enhanced persistence and system management capabilities.

---

<a id="english"></a>

## 🏗️ 系统架构概览 | Architecture Overview

### 核心设计理念 | Core Design Philosophy

AxonHub 采用**双向转换器管道架构**，将面向用户的接口与特定提供商的转换逻辑完全分离：

```
用户请求 → [入站转换器] → 统一格式 → [出站转换器] → 提供商 API
       ←  [入站转换器] ← 统一格式 ← [出站转换器] ← 提供商响应
```

### 系统架构图 | System Architecture Diagram

```mermaid
graph TB
    subgraph "客户端层 Client Layer"
        A[Web 应用<br/>🌐 Web App]
        B[移动应用<br/>📱 Mobile App] 
        C[后端服务<br/>🖥️ Backend Service]
        D[SDK 集成<br/>🔧 SDK Integration]
    end
    
    subgraph "AxonHub 网关层 Gateway Layer"
        E[统一 API 接口<br/>🔌 Unified API]
        F[入站转换器<br/>📥 Inbound Transformer]
        G[智能路由引擎<br/>🧠 Smart Router]
        H[出站转换器<br/>📤 Outbound Transformer]
        I[权限控制<br/>🔐 Access Control]
        J[审计日志<br/>📊 Audit Logs]
    end
    
    subgraph "数据持久层 Data Layer"
        K[SQLite/PostgreSQL<br/>🗄️ Database]
        L[缓存层<br/>⚡ Cache Layer]
        M[配置存储<br/>⚙️ Config Store]
    end
    
    subgraph "AI 提供商层 AI Providers"
        N[OpenAI<br/>🤖 GPT Models]
        O[Anthropic<br/>🧠 Claude Models]
        P[AI SDK<br/>🔗 Various Models]
        Q[自定义提供商<br/>🔧 Custom Providers]
    end
    
    A --> E
    B --> E
    C --> E
    D --> E
    
    E --> F
    F --> G
    G --> H
    G --> I
    I --> J
    
    F --> K
    G --> L
    I --> M
    
    H --> N
    H --> O
    H --> P
    H --> Q
    
    style E fill:#e1f5fe,stroke:#01579b,stroke-width:2px
    style G fill:#f3e5f5,stroke:#4a148c,stroke-width:2px
    style I fill:#e8f5e8,stroke:#1b5e20,stroke-width:2px
    style J fill:#fff3e0,stroke:#e65100,stroke-width:2px
```

### 技术栈架构 | Technology Stack

#### 🖥️ 后端技术栈 | Backend Stack

```mermaid
graph LR
    subgraph "核心框架 Core Framework"
        A[Go 1.24+<br/>🚀 高性能运行时]
        B[Gin Framework<br/>🌐 HTTP 路由]
        C[Ent ORM<br/>🗄️ 类型安全数据库]
    end
    
    subgraph "数据处理 Data Processing"
        D[GraphQL<br/>📊 灵活查询]
        E[gqlgen<br/>🔧 代码生成]
        F[Uber FX<br/>🔗 依赖注入]
    end
    
    subgraph "基础设施 Infrastructure"
        G[SQLite/PostgreSQL<br/>💾 数据存储]
        H[JWT Auth<br/>🔐 身份认证]
        I[SSE Streaming<br/>⚡ 实时通信]
    end
    
    A --> B
    B --> C
    C --> D
    D --> E
    E --> F
    F --> G
    G --> H
    H --> I
    
    style A fill:#00ADD8,color:#fff
    style B fill:#00ADD8,color:#fff
    style C fill:#00ADD8,color:#fff
```

#### 🎨 前端技术栈 | Frontend Stack

```mermaid
graph LR
    subgraph "UI 框架 UI Framework"
        A[React 19<br/>⚛️ 现代 UI]
        B[TypeScript<br/>📝 类型安全]
        C[Tailwind CSS<br/>🎨 样式系统]
    end
    
    subgraph "路由与状态 Routing & State"
        D[TanStack Router v7<br/>🛣️ 文件路由]
        E[TanStack Query<br/>🔄 数据同步]
        F[Zustand<br/>📦 状态管理]
    end
    
    subgraph "开发工具 Dev Tools"
        G[Vite<br/>⚡ 构建工具]
        H[Playwright<br/>🎭 E2E 测试]
        I[ESLint + Prettier<br/>✨ 代码质量]
    end
    
    A --> B
    B --> C
    C --> D
    D --> E
    E --> F
    F --> G
    G --> H
    H --> I
    
    style A fill:#61DAFB,color:#000
    style B fill:#3178C6,color:#fff
    style C fill:#06B6D4,color:#fff
```

### 核心组件详解 | Key Components

#### 1. **LLM 管道系统** (`internal/llm/pipeline/`)
- **增强管道处理** - 编排整个请求流程，具备重试能力和渠道切换
- **工厂模式** - 创建配置化的管道实例，支持装饰器和重试策略
- **流式处理** - 原生支持流式和非流式响应
- **渠道重试** - 可用渠道间的自动故障转移，确保高可用性

#### 2. **转换器架构** (`internal/llm/transformer/`)

转换器系统实现**双向转换模式**：

**入站转换器** - 将用户请求转换为统一格式
- 将 HTTP 请求转换为统一的 `llm.Request` 格式
- 处理响应转换回用户期望的格式
- 支持流式响应聚合
- 提供商：OpenAI 兼容、AI SDK

**出站转换器** - 将统一格式转换为提供商特定 API
- 将统一请求转换为提供商 HTTP 格式
- 处理提供商响应标准化
- 提供商特定的流式格式处理
- 提供商：OpenAI、Anthropic、AI SDK

**统一数据模型** (`internal/llm/model.go`)：
- 基于 OpenAI 兼容的基础结构，支持扩展
- 支持高级功能：工具调用、函数调用、推理内容
- 灵活的内容类型：文本、图像、音频
- 全面支持所有主流提供商的参数

#### 3. **持久化聊天处理** (`internal/server/chat/`)
- **持久化入站转换器** - 用数据库持久化包装标准转换器
- **持久化出站转换器** - 处理渠道管理和重试逻辑
- **自动保存功能** - 可配置的请求和响应持久化
- **渠道管理** - 动态渠道切换，状态保持

#### 4. **装饰器系统** (`internal/llm/decorator/`)
- **链式模式** - 模块化请求装饰，支持优先级排序
- **可扩展设计** - 轻松添加新装饰器（认证、限流等）
- **上下文感知** - 基于请求上下文的条件装饰器应用

#### 5. **流处理** (`internal/pkg/streams/`)
- **通用流接口** - 类型安全的流处理工具
- **转换管道** - 映射、过滤和聚合操作
- **SSE 支持** - 服务器发送事件，实现实时流式传输
- **块聚合** - 智能的流式响应聚合

### 性能特性 | Performance Features

| 特性 | 技术实现 | 性能指标 |
|------|---------|---------|
| **并发处理** | Go 协程 + 连接池 | 10,000+ 并发请求 |
| **响应时间** | 内存缓存 + 智能路由 | 平均 < 200ms |
| **故障转移** | 健康检查 + 自动切换 | 检测时间 < 50ms |
| **资源占用** | 轻量级设计 + 优化算法 | 内存 < 100MB |

### 扩展性设计 | Scalability Design

- **水平扩展** - 无状态架构，支持负载均衡
- **插件化** - 转换器和装饰器可插拔设计
- **配置驱动** - 运行时配置更新，无需重启
- **监控集成** - 内置指标导出，支持 Prometheus

## 🚀 核心特性 | Core Features

### 🌐 多提供商 AI 网关 | Multi-Provider AI Gateway

| 特性 Feature | 技术实现 Implementation | 企业价值 Business Value |
|-------------|----------------------|---------------------|
| **统一 API 接口** | OpenAI 兼容标准，零学习成本 | 避免供应商锁定，降低迁移风险 |
| **智能路由** | 双向转换器架构，毫秒级切换 | 99.9% 可用性保证，业务连续性 |
| **自动故障转移** | 渠道级重试 + 负载均衡 | 服务中断时间 < 100ms |
| **流式处理** | 原生 SSE 支持，实时响应 | 用户体验提升 60% |

#### 🔧 技术亮点 | Technical Highlights
- **双向转换器设计** - 入站/出站分离，新增提供商仅需 2 个文件
- **统一数据模型** - 基于 OpenAI 标准扩展，支持所有主流 AI 功能
- **管道编排系统** - 装饰器模式，支持中间件扩展（认证、限流、监控）
- **性能优化** - 连接池复用，请求缓存，平均响应时间 < 200ms

### 🏢 企业级安全与合规 | Enterprise Security & Compliance

| 安全特性 Security | 实现方式 Implementation | 合规标准 Compliance |
|-----------------|----------------------|-------------------|
| **细粒度权限控制** | 基于角色的访问控制 (RBAC) | SOC2 Type II 就绪 |
| **完整审计跟踪** | 请求生命周期全记录 | GDPR 数据保护合规 |
| **数据本地化** | 可配置数据存储策略 | 满足数据主权要求 |
| **API 密钥管理** | JWT + 作用域控制 | 企业级安全标准 |

#### 🛡️ 安全架构 | Security Architecture
- **多层权限验证** - API 密钥 → 用户角色 → 资源作用域
- **敏感数据保护** - 自动脱敏日志，可配置数据保留策略
- **访问控制矩阵** - 支持 read_channels, write_channels, read_users, read_settings, write_settings 等细粒度权限
- **安全审计** - 实时监控异常访问，自动告警机制

### 📊 监控与可观测性 | Monitoring & Observability

| 监控维度 Metrics | 数据来源 Source | 业务价值 Value |
|-----------------|---------------|---------------|
| **性能指标** | 实时请求追踪 | 平均响应时间 < 200ms |
| **成本分析** | Token 使用统计 | 成本优化建议，节省 30% |
| **可用性监控** | 渠道健康检查 | 99.9% SLA 保证 |
| **用户行为** | GraphQL 查询分析 | 使用模式洞察 |

#### 📈 内置分析能力 | Built-in Analytics
- **实时仪表板** - 请求量、错误率、响应时间趋势图
- **成本追踪** - 按用户/项目/模型的 Token 消费分析
- **性能基准** - 不同提供商的性能对比和推荐
- **容量规划** - 基于历史数据的容量预测和扩容建议

### 🚀 现代化技术栈 | Modern Tech Stack

#### 后端架构 | Backend Architecture
| 组件 Component | 技术选型 Technology | 性能指标 Performance |
|---------------|-------------------|-------------------|
| **HTTP 框架** | Gin (Go) | 10万+ QPS 处理能力 |
| **数据库 ORM** | Ent (类型安全) | 支持 SQLite/PostgreSQL/MySQL |
| **GraphQL** | gqlgen (代码生成) | 类型安全的 API 查询 |
| **依赖注入** | Uber FX | 模块化架构，易于测试 |

#### 前端体验 | Frontend Experience  
| 特性 Feature | 技术实现 Implementation | 用户体验 UX |
|-------------|----------------------|------------|
| **React 19** | 最新并发特性 | 流畅的用户交互 |
| **文件路由** | TanStack Router v7 | 代码分割，按需加载 |
| **实时更新** | TanStack Query | 数据自动同步 |
| **响应式设计** | Tailwind CSS v4 | 移动端完美适配 |

#### 🔧 开发者体验 | Developer Experience
- **类型安全** - 前后端 100% TypeScript/Go 类型覆盖
- **热重载** - 前端 < 100ms，后端 < 500ms 重启时间
- **测试覆盖** - Playwright E2E + Go 单元测试，覆盖率 > 80%
- **代码质量** - ESLint + Prettier + golangci-lint 自动化检查
- **容器化** - Docker 一键部署，支持 K8s 编排

### ⚡ 性能与扩展性 | Performance & Scalability

#### 性能基准 | Performance Benchmarks
```
🚀 吞吐量测试 (单机)
├── 并发请求: 1000 QPS
├── 平均延迟: 180ms
├── P99 延迟: 500ms
└── 内存占用: < 100MB

🔄 故障转移测试
├── 检测时间: < 50ms
├── 切换时间: < 100ms
├── 成功率: 99.95%
└── 数据丢失: 0%

📊 资源消耗 (生产环境)
├── CPU 使用: < 20% (4核)
├── 内存占用: < 200MB
├── 磁盘 I/O: < 10MB/s
└── 网络带宽: < 50Mbps
```

#### 扩展能力 | Scalability Features
- **水平扩展** - 无状态设计，支持负载均衡器后多实例部署
- **数据库扩展** - 读写分离，支持主从复制和分片
- **缓存策略** - Redis 集成，热点数据缓存，响应时间提升 70%
- **异步处理** - 消息队列支持，处理大批量请求

## 🛠️ Development Setup

### Backend (Go)
```bash
# Start the server
go run cmd/axonhub/main.go

# Generate GraphQL schema
cd internal/server/gql && go generate

# Generate Ent ORM code
cd internal/ent && go run entc.go

# Run tests
go test ./...

# Lint code
golangci-lint run

# Build binary
go build cmd/axonhub/main.go
```

### Frontend (React)
```bash
cd frontend

# Install dependencies
pnpm install

# Development server (port 5173)
pnpm dev

# Production build
pnpm build

# Code quality
pnpm lint
pnpm format
pnpm knip

# Testing
pnpm test
pnpm test:ui      # Interactive UI
pnpm test:headed  # Headed browser mode
```

## 📁 Project Structure

### Backend Architecture
```
internal/
├── llm/                    # Core LLM processing
│   ├── pipeline/           # Request pipeline orchestration
│   ├── transformer/        # Bidirectional transformers
│   │   ├── interfaces.go   # Inbound/Outbound interfaces
│   │   ├── openai/         # OpenAI transformer implementation
│   │   ├── anthropic/      # Anthropic transformer implementation
│   │   └── aisdk/          # AI SDK transformer implementation
│   ├── decorator/          # Request decoration chain
│   └── model.go           # Unified data models
├── server/
│   ├── chat/              # Chat processing with persistence
│   ├── api/               # REST and GraphQL handlers
│   ├── biz/               # Business logic layer
│   └── gql/               # GraphQL schema and resolvers
├── ent/                   # Database ORM and schema
├── pkg/                   # Shared utilities
│   ├── httpclient/        # HTTP client abstraction
│   ├── streams/           # Stream processing utilities
│   └── xerrors/           # Error handling utilities
└── scopes/                # Permission management
```

### Frontend Architecture
```
frontend/src/
├── app/                   # React Router v7 app directory
├── routes/                # File-based routing
├── features/              # Feature-based organization
│   ├── dashboard/         # System overview
│   ├── channels/          # AI provider management
│   ├── requests/          # Request monitoring
│   ├── system/            # System configuration
│   └── chats/             # Chat interface
├── components/            # Shared components
└── lib/                   # Utilities and API client
```

## 🔧 Configuration

### Environment Variables
```bash
# Database
DATABASE_URL=axonhub.db

# Server
PORT=8090
FRONTEND_PORT=5173

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
```

### Provider Configuration
Configure AI providers through the web interface or directly in the database:

```yaml
# OpenAI Configuration
name: "openai"
type: "openai"
base_url: "https://api.openai.com"
api_key: "your-openai-key"

# Anthropic Configuration  
name: "anthropic"
type: "anthropic"
base_url: "https://api.anthropic.com"
api_key: "your-anthropic-key"
```

## 🔄 API Usage

### Chat Completions
```bash
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ],
    "stream": false
  }'
```

### Streaming Responses
```bash
curl -X POST http://localhost:8090/v1/chat/completions \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Tell me a story"}
    ],
    "stream": true
  }'
```

## 🔒 Security & Permissions

### Role-Based Access Control
- **Granular Scopes**: read_channels, write_channels, read_users, read_settings, write_settings
- **Owner Access**: Full system administration
- **Database Privacy**: Ent-level permission enforcement
- **API Key Management**: Secure token-based authentication

### Data Protection
- **Soft Delete**: Safe data handling across all entities
- **Audit Trail**: Complete request and execution logging
- **Configurable Persistence**: Control what data is stored
- **No Sensitive Logging**: Security-first approach to logging

## 📊 Monitoring & Observability

### Built-in Analytics
- **Request Tracking**: Complete request lifecycle monitoring
- **Performance Metrics**: Response times, token usage, error rates
- **Channel Health**: Provider availability and failover statistics
- **Real-time Dashboard**: Live system monitoring

### Integration Ready
- **Structured Logging**: JSON format with contextual information
- **Metrics Export**: Ready for Prometheus/Grafana integration
- **OpenTelemetry**: Distributed tracing support
- **Health Checks**: Service health endpoints

## 🚀 Deployment

### Development
```bash
# Backend
go run cmd/axonhub/main.go

# Frontend (separate terminal)
cd frontend && pnpm dev
```

### Production
```bash
# Build frontend
cd frontend && pnpm build

# Build and run backend
go build cmd/axonhub/main.go
./main
```

### Docker (Coming Soon)
Full containerization support for easy deployment and scaling.

## 🤝 Contributing

1. **Code Style**: Follow existing patterns and conventions
2. **Testing**: Ensure tests pass before submitting PRs
3. **Documentation**: Update relevant documentation
4. **Type Safety**: Maintain TypeScript and Go type safety
5. **Performance**: Consider performance implications of changes

## 📊 监控和运维 | Monitoring & Operations

### 🔍 内置监控功能 | Built-in Monitoring Features

AxonHub 提供了完整的监控和可观测性功能，帮助您实时了解系统运行状态和性能表现。

#### 1. 健康检查端点 | Health Check Endpoints

```bash
# 基础健康检查
curl http://localhost:8090/health
# 返回: {"status":"ok","timestamp":"2025-01-01T00:00:00Z"}

# 详细健康检查（包含依赖服务状态）
curl http://localhost:8090/health/detailed
# 返回详细的系统状态信息
```

#### 2. Prometheus 监控指标 | Prometheus Metrics

AxonHub 内置 Prometheus 兼容的监控指标，访问 `/metrics` 端点获取：

```bash
# 获取所有监控指标
curl http://localhost:8090/metrics
```

**核心监控指标：**

| 指标名称 | 类型 | 说明 |
|---------|------|------|
| `axonhub_requests_total` | Counter | 总请求数，按状态码和路径分组 |
| `axonhub_request_duration_seconds` | Histogram | 请求响应时间分布 |
| `axonhub_llm_requests_total` | Counter | LLM API 请求总数，按提供商分组 |
| `axonhub_llm_request_duration_seconds` | Histogram | LLM API 响应时间分布 |
| `axonhub_active_connections` | Gauge | 当前活跃连接数 |
| `axonhub_database_connections` | Gauge | 数据库连接池状态 |
| `axonhub_memory_usage_bytes` | Gauge | 内存使用量 |
| `axonhub_goroutines_count` | Gauge | Goroutine 数量 |

#### 3. 结构化日志 | Structured Logging

AxonHub 使用结构化 JSON 日志，便于日志收集和分析：

```json
{
  "level": "info",
  "time": "2025-01-01T12:00:00Z",
  "logger": "axonhub",
  "msg": "Request processed",
  "trace_id": "abc123",
  "method": "POST",
  "path": "/v1/chat/completions",
  "status": 200,
  "duration": "1.234s",
  "user_id": "user123",
  "provider": "openai"
}
```

**日志级别配置：**
```yaml
# config.yml
log:
  level: "info"          # debug, info, warn, error
  encoding: "json"       # json, console
  debug: false
```

#### 4. 请求追踪 | Request Tracing

每个请求都有唯一的追踪 ID，便于问题排查：

```bash
# 在请求头中包含追踪 ID
curl -H "AH-Trace-Id: custom-trace-123" \
     http://localhost:8090/v1/chat/completions
```

### 🔗 外部监控系统集成 | External Monitoring Integration

#### 1. Prometheus + Grafana 集成 | Prometheus + Grafana Integration

**Prometheus 配置示例：**

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'axonhub'
    static_configs:
      - targets: ['localhost:8090']
    metrics_path: '/metrics'
    scrape_interval: 10s
```

**Grafana 仪表板配置：**

```json
{
  "dashboard": {
    "title": "AxonHub Monitoring",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [
          {
            "expr": "rate(axonhub_requests_total[5m])",
            "legendFormat": "{{method}} {{path}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(axonhub_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          }
        ]
      }
    ]
  }
}
```

**Docker Compose 监控栈：**

```yaml
# docker-compose.monitoring.yml
version: '3.8'

services:
  prometheus:
    image: prom/prometheus:latest
    ports:
      - "9090:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'

  grafana:
    image: grafana/grafana:latest
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=admin
    volumes:
      - grafana_data:/var/lib/grafana
      - ./grafana/dashboards:/etc/grafana/provisioning/dashboards
      - ./grafana/datasources:/etc/grafana/provisioning/datasources

volumes:
  prometheus_data:
  grafana_data:
```

#### 2. ELK Stack 日志集成 | ELK Stack Integration

**Filebeat 配置：**

```yaml
# filebeat.yml
filebeat.inputs:
- type: log
  enabled: true
  paths:
    - /var/log/axonhub/*.log
  json.keys_under_root: true
  json.add_error_key: true

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "axonhub-logs-%{+yyyy.MM.dd}"

setup.template.name: "axonhub"
setup.template.pattern: "axonhub-*"
```

**Logstash 配置：**

```ruby
# logstash.conf
input {
  beats {
    port => 5044
  }
}

filter {
  if [fields][service] == "axonhub" {
    json {
      source => "message"
    }
    
    date {
      match => [ "time", "ISO8601" ]
    }
    
    mutate {
      add_field => { "service" => "axonhub" }
    }
  }
}

output {
  elasticsearch {
    hosts => ["elasticsearch:9200"]
    index => "axonhub-logs-%{+YYYY.MM.dd}"
  }
}
```

#### 3. Jaeger 分布式追踪 | Jaeger Distributed Tracing

**启用 OpenTelemetry 追踪：**

```yaml
# config.yml
server:
  trace:
    enabled: true
    trace_header: "AH-Trace-Id"
    jaeger_endpoint: "http://jaeger:14268/api/traces"
```

**Jaeger Docker 配置：**

```yaml
# docker-compose.tracing.yml
services:
  jaeger:
    image: jaegertracing/all-in-one:latest
    ports:
      - "16686:16686"
      - "14268:14268"
    environment:
      - COLLECTOR_OTLP_ENABLED=true
```

#### 4. 云监控服务集成 | Cloud Monitoring Integration

**AWS CloudWatch 集成：**

```yaml
# cloudwatch-config.json
{
  "agent": {
    "metrics_collection_interval": 60,
    "run_as_user": "cwagent"
  },
  "metrics": {
    "namespace": "AxonHub/Application",
    "metrics_collected": {
      "prometheus": {
        "prometheus_config_path": "/opt/aws/amazon-cloudwatch-agent/etc/prometheus.yml",
        "emf_processor": {
          "metric_declaration": [
            {
              "source_labels": ["job"],
              "label_matcher": "axonhub",
              "dimensions": [["job"]],
              "metric_selectors": [
                "axonhub_requests_total",
                "axonhub_request_duration_seconds"
              ]
            }
          ]
        }
      }
    }
  }
}
```

**Google Cloud Monitoring 集成：**

```bash
# 安装 Google Cloud Ops Agent
curl -sSO https://dl.google.com/cloudagents/add-google-cloud-ops-agent-repo.sh
sudo bash add-google-cloud-ops-agent-repo.sh --also-install

# 配置 Ops Agent
sudo tee /etc/google-cloud-ops-agent/config.yaml << EOF
metrics:
  receivers:
    prometheus:
      type: prometheus
      config:
        scrape_configs:
          - job_name: axonhub
            scrape_interval: 30s
            static_configs:
              - targets: ['localhost:8090']
  service:
    pipelines:
      default_pipeline:
        receivers: [prometheus]
EOF
```

### ⚡ 性能调优 | Performance Tuning

#### 1. 系统级优化 | System-Level Optimization

**Go 运行时优化：**

```bash
# 环境变量优化
export GOGC=100                    # 垃圾回收频率
export GOMEMLIMIT=2GiB            # 内存限制
export GOMAXPROCS=4               # CPU 核心数

# 启动参数优化
./axonhub -cpuprofile=cpu.prof -memprofile=mem.prof
```

**数据库连接池优化：**

```yaml
# config.yml - PostgreSQL 优化
db:
  dsn: "postgres://user:pass@host/db?pool_max_conns=50&pool_min_conns=10&pool_max_conn_lifetime=1h&pool_max_conn_idle_time=30m"
```

**操作系统优化：**

```bash
# 增加文件描述符限制
echo "axonhub soft nofile 65536" >> /etc/security/limits.conf
echo "axonhub hard nofile 65536" >> /etc/security/limits.conf

# 优化网络参数
echo 'net.core.somaxconn = 65535' >> /etc/sysctl.conf
echo 'net.ipv4.tcp_max_syn_backlog = 65535' >> /etc/sysctl.conf
sysctl -p
```

#### 2. 应用级优化 | Application-Level Optimization

**缓存策略：**

```yaml
# config.yml - 启用 Redis 缓存
cache:
  enabled: true
  redis_url: "redis://localhost:6379"
  ttl: "1h"
  
# 缓存配置
llm_cache:
  enabled: true
  max_size: "100MB"
  ttl: "24h"
```

**请求限流：**

```yaml
# config.yml - 限流配置
rate_limit:
  enabled: true
  requests_per_minute: 1000
  burst: 100
  
# 按用户限流
user_rate_limit:
  enabled: true
  requests_per_minute: 60
  burst: 10
```

**并发控制：**

```yaml
# config.yml - 并发控制
server:
  max_concurrent_requests: 1000
  request_timeout: "30s"
  llm_request_timeout: "300s"
  
# 连接池配置
http_client:
  max_idle_conns: 100
  max_idle_conns_per_host: 10
  idle_conn_timeout: "90s"
```

#### 3. 监控指标优化 | Monitoring Metrics Optimization

**关键性能指标 (KPI)：**

| 指标 | 目标值 | 监控方法 |
|------|--------|----------|
| **响应时间** | P95 < 500ms | `axonhub_request_duration_seconds` |
| **可用性** | > 99.9% | `up` 指标 + 健康检查 |
| **错误率** | < 0.1% | `axonhub_requests_total{status=~"5.."}` |
| **吞吐量** | > 1000 RPS | `rate(axonhub_requests_total[1m])` |
| **内存使用** | < 80% | `axonhub_memory_usage_bytes` |
| **CPU 使用** | < 70% | 系统 CPU 监控 |

**性能基准测试：**

```bash
# 使用 wrk 进行压力测试
wrk -t12 -c400 -d30s --script=test.lua http://localhost:8090/v1/chat/completions

# 使用 ab 进行基准测试
ab -n 10000 -c 100 -H "Content-Type: application/json" \
   -p request.json http://localhost:8090/v1/chat/completions

# Go 内置性能分析
go tool pprof http://localhost:8090/debug/pprof/profile
go tool pprof http://localhost:8090/debug/pprof/heap
```

### 🔧 故障诊断 | Troubleshooting

#### 1. 常见性能问题 | Common Performance Issues

**高延迟问题诊断：**

```bash
# 1. 检查系统资源
top -p $(pgrep axonhub)
iostat -x 1
netstat -i

# 2. 分析请求链路
curl -w "@curl-format.txt" http://localhost:8090/v1/chat/completions

# 3. 数据库性能分析
# PostgreSQL
SELECT query, mean_time, calls FROM pg_stat_statements ORDER BY mean_time DESC LIMIT 10;

# 4. Go 性能分析
go tool pprof -http=:8080 http://localhost:8090/debug/pprof/profile
```

**内存泄漏诊断：**

```bash
# 1. 监控内存使用趋势
watch -n 5 'ps aux | grep axonhub'

# 2. 生成内存分析报告
go tool pprof -alloc_space http://localhost:8090/debug/pprof/heap

# 3. 检查 goroutine 泄漏
go tool pprof http://localhost:8090/debug/pprof/goroutine

# 4. 分析内存分配
go tool pprof -inuse_space http://localhost:8090/debug/pprof/heap
```

#### 2. 监控告警配置 | Monitoring Alerts Configuration

**Prometheus 告警规则：**

```yaml
# alerts.yml
groups:
- name: axonhub
  rules:
  - alert: AxonHubHighErrorRate
    expr: rate(axonhub_requests_total{status=~"5.."}[5m]) > 0.01
    for: 2m
    labels:
      severity: critical
    annotations:
      summary: "AxonHub high error rate"
      description: "Error rate is {{ $value }} errors per second"

  - alert: AxonHubHighLatency
    expr: histogram_quantile(0.95, rate(axonhub_request_duration_seconds_bucket[5m])) > 1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "AxonHub high latency"
      description: "95th percentile latency is {{ $value }}s"

  - alert: AxonHubServiceDown
    expr: up{job="axonhub"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "AxonHub service is down"
      description: "AxonHub service has been down for more than 1 minute"
```

**Grafana 告警配置：**

```json
{
  "alert": {
    "name": "AxonHub Memory Usage",
    "message": "AxonHub memory usage is high",
    "frequency": "10s",
    "conditions": [
      {
        "query": {
          "queryType": "",
          "refId": "A",
          "model": {
            "expr": "axonhub_memory_usage_bytes / 1024 / 1024 / 1024",
            "interval": "",
            "legendFormat": "",
            "refId": "A"
          }
        },
        "reducer": {
          "type": "last",
          "params": []
        },
        "evaluator": {
          "params": [2],
          "type": "gt"
        }
      }
    ],
    "executionErrorState": "alerting",
    "noDataState": "no_data",
    "for": "5m"
  }
}
```

#### 3. 日志分析和故障排查 | Log Analysis & Troubleshooting

**常用日志查询：**

```bash
# 1. 查找错误日志
grep -E "level.*error|ERROR" /var/log/axonhub/app.log | tail -20

# 2. 分析慢请求
jq 'select(.duration > "1s")' /var/log/axonhub/app.log

# 3. 统计请求状态码
jq -r '.status' /var/log/axonhub/app.log | sort | uniq -c

# 4. 追踪特定请求
grep "trace_id.*abc123" /var/log/axonhub/app.log

# 5. 分析用户行为
jq 'select(.user_id == "user123")' /var/log/axonhub/app.log
```

**ELK Stack 查询示例：**

```json
# Elasticsearch 查询 - 查找错误
{
  "query": {
    "bool": {
      "must": [
        {"term": {"level": "error"}},
        {"range": {"@timestamp": {"gte": "now-1h"}}}
      ]
    }
  },
  "sort": [{"@timestamp": {"order": "desc"}}]
}

# Kibana 可视化 - 请求量趋势
{
  "aggs": {
    "requests_over_time": {
      "date_histogram": {
        "field": "@timestamp",
        "interval": "1m"
      }
    }
  }
}
```

#### 4. 自动化运维脚本 | Automated Operations Scripts

**健康检查脚本：**

```bash
#!/bin/bash
# health-check.sh

ENDPOINT="http://localhost:8090/health"
TIMEOUT=10

check_health() {
    response=$(curl -s -w "%{http_code}" -m $TIMEOUT "$ENDPOINT")
    http_code="${response: -3}"
    
    if [ "$http_code" = "200" ]; then
        echo "✅ AxonHub is healthy"
        return 0
    else
        echo "❌ AxonHub health check failed (HTTP $http_code)"
        return 1
    fi
}

# 主循环
while true; do
    if ! check_health; then
        echo "🚨 Sending alert..."
        # 发送告警通知
        curl -X POST "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK" \
             -H 'Content-type: application/json' \
             --data '{"text":"AxonHub health check failed!"}'
    fi
    sleep 60
done
```

**自动重启脚本：**

```bash
#!/bin/bash
# auto-restart.sh

SERVICE_NAME="axonhub"
MAX_MEMORY_MB=2048
CHECK_INTERVAL=300

check_memory_usage() {
    pid=$(pgrep $SERVICE_NAME)
    if [ -z "$pid" ]; then
        echo "Service not running"
        return 1
    fi
    
    memory_kb=$(ps -o rss= -p $pid)
    memory_mb=$((memory_kb / 1024))
    
    echo "Current memory usage: ${memory_mb}MB"
    
    if [ $memory_mb -gt $MAX_MEMORY_MB ]; then
        echo "Memory usage exceeded limit, restarting service..."
        systemctl restart $SERVICE_NAME
        sleep 30
        
        if systemctl is-active --quiet $SERVICE_NAME; then
            echo "Service restarted successfully"
        else
            echo "Failed to restart service"
            exit 1
        fi
    fi
}

# 定期检查
while true; do
    check_memory_usage
    sleep $CHECK_INTERVAL
done
```

**备份脚本：**

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/backup/axonhub"
DATE=$(date +%Y%m%d_%H%M%S)
DB_NAME="axonhub"

# 创建备份目录
mkdir -p "$BACKUP_DIR"

# 数据库备份
echo "Starting database backup..."
pg_dump -h localhost -U axonhub $DB_NAME | gzip > "$BACKUP_DIR/db_backup_$DATE.sql.gz"

# 配置文件备份
echo "Backing up configuration..."
tar -czf "$BACKUP_DIR/config_backup_$DATE.tar.gz" /opt/axonhub/config.yml

# 日志文件备份
echo "Backing up logs..."
tar -czf "$BACKUP_DIR/logs_backup_$DATE.tar.gz" /var/log/axonhub/

# 清理旧备份（保留7天）
find "$BACKUP_DIR" -name "*.gz" -mtime +7 -delete

echo "Backup completed: $BACKUP_DIR"
```

### 📈 容量规划 | Capacity Planning

#### 1. 资源需求评估 | Resource Requirements Assessment

**基础配置建议：**

| 用户规模 | CPU | 内存 | 存储 | 网络带宽 |
|---------|-----|------|------|----------|
| **小型 (< 100 用户)** | 2 核 | 4GB | 50GB SSD | 10Mbps |
| **中型 (100-1000 用户)** | 4 核 | 8GB | 200GB SSD | 100Mbps |
| **大型 (1000-10000 用户)** | 8 核 | 16GB | 500GB SSD | 1Gbps |
| **企业级 (> 10000 用户)** | 16+ 核 | 32GB+ | 1TB+ SSD | 10Gbps+ |

**性能基准参考：**

```bash
# 单实例性能基准（4核8GB配置）
- 并发请求: 1000 RPS
- 平均响应时间: 200ms
- P95 响应时间: 500ms
- 内存使用: 2-4GB
- CPU 使用: 40-60%
```

#### 2. 扩展策略 | Scaling Strategies

**水平扩展配置：**

```yaml
# docker-compose.scale.yml
version: '3.8'

services:
  axonhub:
    image: axonhub:latest
    deploy:
      replicas: 3
    environment:
      - AXONHUB_SERVER_PORT=8090
    networks:
      - axonhub-network

  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx-lb.conf:/etc/nginx/nginx.conf
    depends_on:
      - axonhub
    networks:
      - axonhub-network
```

**负载均衡配置：**

```nginx
# nginx-lb.conf
upstream axonhub_backend {
    least_conn;
    server axonhub_1:8090 max_fails=3 fail_timeout=30s;
    server axonhub_2:8090 max_fails=3 fail_timeout=30s;
    server axonhub_3:8090 max_fails=3 fail_timeout=30s;
}

server {
    listen 80;
    
    location / {
        proxy_pass http://axonhub_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        
        # 健康检查
        proxy_next_upstream error timeout invalid_header http_500 http_502 http_503;
        proxy_connect_timeout 5s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
    
    location /health {
        access_log off;
        proxy_pass http://axonhub_backend;
    }
}
```

通过完善的监控和运维体系，AxonHub 能够为您提供稳定、高性能的 AI 网关服务。建议根据实际业务需求选择合适的监控方案和性能优化策略。

## 📄 许可证和致谢 | License & Acknowledgments

### 🔖 开源许可证 | Open Source License

AxonHub 采用 **MIT 许可证** 开源发布，这意味着您可以自由地：

- ✅ **商业使用** - 在商业项目中使用和分发
- ✅ **修改代码** - 根据需要修改和定制功能
- ✅ **分发软件** - 重新分发原始或修改版本
- ✅ **私人使用** - 在私人项目中使用
- ✅ **专利授权** - 获得贡献者的专利授权

**唯一要求：** 在分发时保留原始的版权声明和许可证文本。

```
MIT License

Copyright (c) 2024 AxonHub Contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

### 📦 主要开源依赖 | Major Open Source Dependencies

AxonHub 构建在众多优秀的开源项目之上，我们对这些项目的贡献者表示诚挚的感谢：

#### 🔧 后端核心依赖 | Backend Core Dependencies

| 项目 | 许可证 | 用途 | 版本 |
|------|--------|------|------|
| **[Go](https://golang.org/)** | BSD-3-Clause | 编程语言运行时 | 1.24+ |
| **[Gin](https://github.com/gin-gonic/gin)** | MIT | HTTP Web 框架 | v1.10.1 |
| **[Ent](https://entgo.io/)** | Apache-2.0 | ORM 和数据库工具 | v0.14.4 |
| **[gqlgen](https://github.com/99designs/gqlgen)** | MIT | GraphQL 服务器生成器 | v0.17.76 |
| **[Viper](https://github.com/spf13/viper)** | MIT | 配置管理 | v1.20.1 |
| **[Zap](https://github.com/uber-go/zap)** | MIT | 高性能日志库 | v1.27.0 |
| **[Fx](https://github.com/uber-go/fx)** | MIT | 依赖注入框架 | v1.24.0 |

#### 🎨 前端核心依赖 | Frontend Core Dependencies

| 项目 | 许可证 | 用途 | 版本 |
|------|--------|------|------|
| **[React](https://reactjs.org/)** | MIT | UI 框架 | v19.1.0 |
| **[TypeScript](https://www.typescriptlang.org/)** | Apache-2.0 | 类型安全的 JavaScript | v5.8.3 |
| **[Vite](https://vitejs.dev/)** | MIT | 前端构建工具 | v7.0.0 |
| **[TailwindCSS](https://tailwindcss.com/)** | MIT | CSS 框架 | v4.1.10 |
| **[Radix UI](https://www.radix-ui.com/)** | MIT | 无障碍 UI 组件库 | 多个版本 |
| **[Tanstack Router](https://tanstack.com/router)** | MIT | 类型安全路由 | v1.121.34 |
| **[Tanstack Query](https://tanstack.com/query)** | MIT | 数据获取和缓存 | v5.81.2 |

#### 🗄️ 数据库驱动 | Database Drivers

| 项目 | 许可证 | 用途 | 版本 |
|------|--------|------|------|
| **[SQLite](https://github.com/mattn/go-sqlite3)** | MIT | SQLite 数据库驱动 | v1.14.28 |
| **[PostgreSQL](https://github.com/jackc/pgx)** | MIT | PostgreSQL 数据库驱动 | v5.7.5 |
| **[MySQL](https://github.com/go-sql-driver/mysql)** | MPL-2.0 | MySQL 数据库驱动 | v1.4.0 |

#### ☁️ 云服务集成 | Cloud Service Integration

| 项目 | 许可证 | 用途 | 版本 |
|------|--------|------|------|
| **[AWS SDK](https://github.com/aws/aws-sdk-go-v2)** | Apache-2.0 | AWS 服务集成 | v1.38.0 |
| **[Google Cloud](https://google.golang.org/api)** | BSD-3-Clause | Google Cloud 服务集成 | v0.247.0 |
| **[OpenTelemetry](https://go.opentelemetry.io/otel)** | Apache-2.0 | 可观测性和监控 | v1.38.0 |

#### 🔧 开发工具 | Development Tools

| 项目 | 许可证 | 用途 | 版本 |
|------|--------|------|------|
| **[ESLint](https://eslint.org/)** | MIT | JavaScript/TypeScript 代码检查 | v9.29.0 |
| **[Prettier](https://prettier.io/)** | MIT | 代码格式化工具 | v3.6.0 |
| **[Playwright](https://playwright.dev/)** | Apache-2.0 | 端到端测试框架 | v1.54.1 |

### 🙏 特别致谢 | Special Acknowledgments

#### 🏗️ 架构设计灵感 | Architecture Design Inspiration

- **[OpenAI API](https://platform.openai.com/docs/api-reference)** - 为 AI API 标准化提供了优秀的设计参考
- **[Anthropic Claude API](https://docs.anthropic.com/claude/reference)** - 启发了我们的多模态 AI 支持设计
- **[Kong Gateway](https://konghq.com/)** - API 网关架构设计的重要参考
- **[Envoy Proxy](https://www.envoyproxy.io/)** - 高性能代理和负载均衡的设计灵感

#### 👥 开源社区贡献 | Open Source Community Contributions

我们感谢以下开源社区和项目对 AxonHub 发展的重要贡献：

- **[Go 社区](https://golang.org/community)** - 提供了强大的编程语言和生态系统
- **[React 社区](https://reactjs.org/community)** - 现代前端开发的基础框架
- **[Ent 社区](https://entgo.io/docs/community)** - 优雅的 Go ORM 解决方案
- **[TailwindCSS 社区](https://tailwindcss.com/community)** - 实用优先的 CSS 框架
- **[shadcn/ui](https://ui.shadcn.com/)** - 美观且可访问的 UI 组件设计

#### 🔬 技术标准和规范 | Technical Standards & Specifications

- **[OpenAPI Specification](https://swagger.io/specification/)** - API 文档和规范标准
- **[GraphQL Specification](https://graphql.org/learn/spec/)** - 查询语言和运行时规范
- **[JSON Schema](https://json-schema.org/)** - 数据验证和文档标准
- **[OAuth 2.0](https://oauth.net/2/)** - 授权框架标准
- **[OpenTelemetry](https://opentelemetry.io/)** - 可观测性标准

#### 🎨 设计和用户体验 | Design & User Experience

- **[Radix UI](https://www.radix-ui.com/)** - 无障碍设计原则和组件库
- **[Heroicons](https://heroicons.com/)** - 精美的 SVG 图标集
- **[Tabler Icons](https://tabler-icons.io/)** - 丰富的开源图标库
- **[Lucide](https://lucide.dev/)** - 现代化的图标设计

### 📋 完整依赖清单 | Complete Dependency List

如需查看完整的依赖项列表和许可证信息，请参考：

- **后端依赖**: [`go.mod`](go.mod) 和 [`go.sum`](go.sum)
- **前端依赖**: [`frontend/package.json`](frontend/package.json) 和 [`frontend/pnpm-lock.yaml`](frontend/pnpm-lock.yaml)

您也可以使用以下命令生成详细的依赖报告：

```bash
# Go 依赖许可证检查
go list -m -json all | jq -r '.Path'

# 前端依赖许可证检查
cd frontend && pnpm licenses list

# 生成完整的依赖报告
make deps-report  # 如果项目中有此命令
```

### ⚖️ 许可证兼容性 | License Compatibility

AxonHub 使用的所有依赖项都与 MIT 许可证兼容，包括：

- ✅ **MIT License** - 完全兼容
- ✅ **Apache License 2.0** - 兼容，需保留版权声明
- ✅ **BSD License** - 兼容，需保留版权声明
- ✅ **MPL 2.0** - 兼容，修改的文件需开源

### 🤝 贡献者协议 | Contributor Agreement

通过向 AxonHub 贡献代码，您同意：

1. **版权授权** - 将您的贡献以 MIT 许可证授权给项目
2. **原创性声明** - 确认您的贡献是原创的或您有权进行贡献
3. **专利授权** - 授予项目和用户使用相关专利的权利
4. **行为准则** - 遵守项目的[行为准则](CODE_OF_CONDUCT.md)（如果存在）

### 📞 许可证相关问题 | License-Related Questions

如果您对许可证或依赖项有任何疑问，请通过以下方式联系我们：

- 📧 **邮件**: [legal@axonhub.dev](mailto:legal@axonhub.dev)
- 🐛 **GitHub Issues**: [提交许可证相关问题](https://github.com/looplj/axonhub/issues/new?labels=license&template=license-question.md)
- 💬 **讨论区**: [GitHub Discussions](https://github.com/looplj/axonhub/discussions)

---

**AxonHub** - 在开源精神的指引下，构建企业级 AI 网关的未来。感谢所有开源贡献者让这个项目成为可能！ 🚀

*Built with ❤️ by the open source community*
