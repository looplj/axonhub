# 开发指南 | Development Guide

---

## English Version

### Architecture Design

AxonHub implements a sophisticated bidirectional data transformation pipeline that ensures seamless communication between clients and AI providers:

<div align="center">
  <img src="docs/transformation-flow.svg" alt="AxonHub Transformation Flow" width="900"/>
</div>

#### Pipeline Components

| Component | Purpose | Key Features |
| --- | --- | --- |
| **Client** | Application layer | Web apps, mobile apps, API clients |
| **Inbound Transformer** | Request preprocessing | Parse, validate, normalize input |
| **Unified Request** | Core processing | Route selection, load balancing, failover |
| **Outbound Transformer** | Provider adaptation | Format conversion, protocol mapping |
| **Provider** | AI services | OpenAI, Anthropic, DeepSeek, etc. |

This architecture ensures:

- ⚡ **Low Latency**: Optimized processing pipeline
- 🔄 **Auto Failover**: Seamless provider switching
- 📊 **Real-time Monitoring**: Complete request tracing
- 🛡️ **Security & Validation**: Input sanitization and output verification

### Technology Stack

#### Backend Technology Stack

- **Go 1.24+** - High-performance backend
- **Gin** - HTTP framework
- **Ent ORM** - Type-safe ORM
- **gqlgen** - GraphQL code generation
- **JWT** - Authentication

#### Frontend Technology Stack

- **React 19** - Modern UI framework
- **TypeScript** - Type safety
- **Tailwind CSS** - Styling framework
- **TanStack Router** - File-based routing
- **Zustand** - State management

### Development Environment Setup

#### Prerequisites

- Go 1.24 or higher
- Node.js 18+ and pnpm
- Git

#### Clone the Project

```bash
git clone https://github.com/looplj/axonhub.git
cd axonhub
```

#### Start Backend

```bash
# Option 1: Build and run directly
make build-backend
./axonhub

# Option 2: Use air for hot reload (recommended for development)
go install github.com/air-verse/air@latest
air
```

The backend server will start at `http://localhost:8090`

#### Start Frontend

In a new terminal window:

```bash
cd frontend
pnpm install
pnpm dev
```

The frontend development server will start at `http://localhost:5173`

### Building the Project

#### Build Complete Project

```bash
make build
```

This will build both backend and frontend, and embed frontend assets into the backend binary.

#### Build Backend Only

```bash
make build-backend
```

#### Build Frontend Only

```bash
cd frontend
pnpm build
```

### Testing

#### Run Backend Tests

```bash
make test
```

#### Run Frontend Tests

```bash
cd frontend
pnpm test
```

#### Run E2E Tests

```bash
bash ./scripts/e2e-test.sh
```

### Code Quality

#### Run Linter

```bash
golangci-lint run -v
```

### Development Workflow

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make changes and test**
   - Write code
   - Add tests
   - Run tests to ensure they pass
   - Run linter to check code quality

3. **Commit changes**
   ```bash
   git add .
   git commit -m "feat: your feature description"
   ```

4. **Push and create Pull Request**
   ```bash
   git push origin feature/your-feature-name
   ```

### Commit Convention

We follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

- `feat:` New feature
- `fix:` Bug fix
- `docs:` Documentation changes
- `style:` Code formatting changes
- `refactor:` Code refactoring
- `test:` Test-related changes
- `chore:` Build process or auxiliary tool changes

---

## 中文版本

### 架构设计

AxonHub 实现了一个复杂的双向数据转换管道，确保客户端和 AI 提供商之间的无缝通信。

<div align="center">
  <img src="docs/transformation-flow.svg" alt="AxonHub Transformation Flow" width="900"/>
</div>

#### 管道组件

| 组件 | 用途 | 关键特性 |
| --- | --- | --- |
| **客户端** | 应用层 | Web 应用、移动应用、API 客户端 |
| **入站转换器** | 请求预处理 | 解析、验证、规范化输入 |
| **统一请求** | 核心处理 | 路由选择、负载均衡、故障转移 |
| **出站转换器** | 提供商适配 | 格式转换、协议映射 |
| **提供商** | AI 服务 | OpenAI、Anthropic、DeepSeek 等 |

该架构确保：

- ⚡ **低延迟**：优化的处理管道
- 🔄 **自动故障转移**：无缝提供商切换
- 📊 **实时监控**：完整的请求追踪
- 🛡️ **安全与验证**：输入清理和输出验证

### 技术栈

#### 后端技术栈

- **Go 1.24+** - 高性能后端
- **Gin** - HTTP 框架
- **Ent ORM** - 类型安全的 ORM
- **gqlgen** - GraphQL 代码生成
- **JWT** - 身份认证

#### 前端技术栈

- **React 19** - 现代 UI 框架
- **TypeScript** - 类型安全
- **Tailwind CSS** - 样式框架
- **TanStack Router** - 文件路由
- **Zustand** - 状态管理

### 开发环境设置

#### 前置要求

- Go 1.24 或更高版本
- Node.js 18+ 和 pnpm
- Git

#### 克隆项目

```bash
git clone https://github.com/looplj/axonhub.git
cd axonhub
```

#### 启动后端

```bash
# 方式 1: 直接构建并运行
make build-backend
./axonhub

# 方式 2: 使用 air 进行热重载（推荐开发使用）
go install github.com/air-verse/air@latest
air
```

后端服务将在 `http://localhost:8090` 启动

#### 启动前端

在新的终端窗口中：

```bash
cd frontend
pnpm install
pnpm dev
```

前端开发服务器将在 `http://localhost:5173` 启动

### 项目构建

#### 构建完整项目

```bash
make build
```

这将构建后端和前端，并将前端资源嵌入到后端二进制文件中。

#### 仅构建后端

```bash
make build-backend
```

#### 仅构建前端

```bash
cd frontend
pnpm build
```

### 测试

#### 运行后端测试

```bash
make test
```

#### 运行 E2E 测试

```bash
make ./scripts/e2e-test.sh
```

### 代码质量

#### 运行 Linter

```bash
golangci-lint run -v
```

### 开发工作流

1. **创建功能分支**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **进行更改并测试**
   - 编写代码
   - 添加测试
   - 运行测试确保通过
   - 运行 linter 检查代码质量

3. **提交更改**
   ```bash
   git add .
   git commit -m "feat: your feature description"
   ```

4. **推送并创建 Pull Request**
   ```bash
   git push origin feature/your-feature-name
   ```

### 提交规范

我们使用 [Conventional Commits](https://www.conventionalcommits.org/) 规范：

- `feat:` 新功能
- `fix:` 错误修复
- `docs:` 文档更改
- `style:` 代码格式更改
- `refactor:` 代码重构
- `test:` 测试相关
- `chore:` 构建过程或辅助工具的变动


<div align="center">

**AxonHub** - All-in-one AI Development Platform

[🏠 Homepage](https://github.com/looplj/axonhub) • [📚 Documentation](https://deepwiki.com/looplj/axonhub) • [🐛 Issue Feedback](https://github.com/looplj/axonhub/issues)

Built with ❤️ by the AxonHub team

</div>

