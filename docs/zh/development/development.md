# 开发指南

---

## 架构设计

AxonHub 实现了一个双向数据转换管道，确保客户端与 AI 提供商之间的无缝通信。

<div align="center">
  <img src="../../transformation-flow.svg" alt="AxonHub Transformation Flow" width="900"/>
</div>

### 管道组件

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
- 🛡️ **安全与验证**：输入清理与输出校验

## 技术栈

### 后端技术栈

- **Go 1.24+**
- **Gin**
- **Ent ORM**
- **gqlgen**
- **JWT**

### 前端技术栈

- **React 19**
- **TypeScript**
- **Tailwind CSS**
- **TanStack Router**
- **Zustand**

## 开发环境搭建

### 前置要求

- Go 1.24 或更高版本
- Node.js 18+ 与 pnpm
- Git

### 克隆项目

```bash
git clone https://github.com/looplj/axonhub.git
cd axonhub
```

### 启动后端

```bash
# 方式 1：直接构建并运行
make build-backend
./axonhub

# 方式 2：使用 air 热重载（推荐）
go install github.com/air-verse/air@latest
air
```

后端服务默认启动在 `http://localhost:8090`。

### 启动前端

在新的终端窗口中：

```bash
cd frontend
pnpm install
pnpm dev
```

前端开发服务器默认启动在 `http://localhost:5173`。

## 项目构建

### 构建完整项目

```bash
make build
```

该命令会构建后端与前端，并将前端产物嵌入到后端二进制文件中。

### 仅构建后端

```bash
make build-backend
```

### 仅构建前端

```bash
cd frontend
pnpm build
```

## 代码生成

当修改 Ent schema 或 GraphQL schema 后，需要重新生成代码：

```bash
make generate
```

## 测试

### 运行后端测试

```bash
go test ./...
```

### 运行 E2E 测试

```bash
bash ./scripts/e2e-test.sh
```

## 代码质量

### 运行 Go Linter

```bash
golangci-lint run -v
```

### 运行前端 Lint/格式化检查

```bash
cd frontend
pnpm lint
pnpm format:check
```

## 事务处理（Ent）

### 何时使用事务

- 多次写入需要保证“要么全部成功，要么全部失败”。
- 需要在同一个逻辑操作中保证读写一致性。

### 推荐：使用 `AbstractService.RunInTransaction`

`RunInTransaction` 会：
- 如果 `ctx` 已经携带事务，则复用当前事务。
- 否则开启新事务，将 tx 绑定的 `*ent.Client` 放入 `ctx`，并自动 commit/rollback。

```go
func (s *SomeService) doWork(ctx context.Context) error {
    return s.RunInTransaction(ctx, func(ctx context.Context) error {
        // ctx 现在同时携带：
        // - ent.TxFromContext(ctx)（当前 tx）
        // - ent.FromContext(ctx)（绑定到 tx 的 *ent.Client）
        //
        // 可以继续调用其它 service，它们会通过 ctx 复用同一个事务。
        return nil
    })
}
```

### 注意事项

- 事务 client 不适合在多个 goroutine 间共享。
- 事务作用域尽量保持小，并避免在事务内执行耗时 I/O。

## 添加新的 Channel

新增渠道时需要同时关注后端与前端的改动：

1. **在 Ent Schema 中扩展枚举**
   - 在 [internal/ent/schema/channel.go](../../../internal/ent/schema/channel.go) 的 `field.Enum("type")` 列表里添加新的渠道标识
   - 执行 `make generate` 以生成代码与迁移

2. **在业务层构造 Transformer**
   - 在 `ChannelService.buildChannel` 的 switch 中为新枚举返回合适的 outbound transformer
   - 必要时在 `internal/llm/transformer` 下实现新的 transformer

3. **注册 Provider 元数据**
   - 在 [frontend/src/features/channels/data/config_providers.ts](../../../frontend/src/features/channels/data/config_providers.ts) 添加或扩展 Provider 配置
   - 确保 `channelTypes` 中引用的渠道都已经在 `CHANNEL_CONFIGS` 中存在

4. **同步前端的 schema 与展示**
   - 将枚举值加入 [frontend/src/features/channels/data/schema.ts](../../../frontend/src/features/channels/data/schema.ts) 的 Zod schema
   - 在 [frontend/src/features/channels/data/constants.ts](../../../frontend/src/features/channels/data/constants.ts) 中添加渠道配置

5. **添加国际化**
   - 在两个 locale 文件中补充翻译：
     - [frontend/src/locales/en.json](../../../frontend/src/locales/en.json)
     - [frontend/src/locales/zh.json](../../../frontend/src/locales/zh.json)
