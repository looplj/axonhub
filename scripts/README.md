# AxonHub Scripts

项目脚本工具集，包含测试、部署和开发辅助脚本。

## 📁 脚本列表

### E2E 测试脚本

#### `e2e/e2e-test.sh`
一键运行完整的 E2E 测试套件。

```bash
# 运行所有 E2E 测试
./scripts/e2e/e2e-test.sh

# 运行特定测试
./scripts/e2e/e2e-test.sh tests/auth.spec.ts
```

**功能：**
- 自动启动后端服务
- 运行 Playwright E2E 测试
- 测试完成后自动清理

#### `e2e/e2e-backend.sh`
管理 E2E 测试后端服务。

```bash
# 启动后端
./scripts/e2e/e2e-backend.sh start

# 停止后端
./scripts/e2e/e2e-backend.sh stop

# 查看状态
./scripts/e2e/e2e-backend.sh status

# 重启后端
./scripts/e2e/e2e-backend.sh restart

# 清理所有 E2E 文件
./scripts/e2e/e2e-backend.sh clean
```

**配置：**
- 端口: 8099
- 数据库: `scripts/e2e/axonhub-e2e.db`
- 日志: `scripts/e2e/e2e-backend.log`

### 数据库迁移测试脚本

#### `migration/migration-test.sh`
测试从指定 tag 到当前分支的数据库迁移。

```bash
# 测试从 v0.1.0 迁移
./scripts/migration/migration-test.sh v0.1.0

# 跳过 E2E 测试
./scripts/migration/migration-test.sh v0.1.0 --skip-e2e

# 保留测试产物
./scripts/migration/migration-test.sh v0.1.0 --keep-artifacts

# 使用缓存的二进制文件
./scripts/migration/migration-test.sh v0.1.0 --skip-download
```

**功能：**
1. 从 GitHub 下载指定 tag 的二进制文件
2. 缓存下载的二进制文件
3. 使用旧版本初始化数据库
4. 使用当前版本运行迁移
5. 运行 E2E 测试验证迁移结果

**详细文档：** [MIGRATION_TEST.md](./migration/MIGRATION_TEST.md)

#### `migration/migration-test-all.sh`
批量测试多个版本的数据库迁移。

```bash
# 自动测试最近 3 个稳定版本
./scripts/migration/migration-test-all.sh

# 测试指定版本
./scripts/migration/migration-test-all.sh --tags v0.1.0,v0.2.0,v0.2.1

# 批量测试但跳过 E2E
./scripts/migration/migration-test-all.sh --skip-e2e
```

**功能：**
- 自动检测最近的稳定版本
- 批量运行迁移测试
- 生成测试摘要报告

### Load Balance 日志分析脚本

#### `utils/filter-load-balance-logs.sh`
过滤和分析负载均衡日志，便于分析 load balance 是否如预期工作。

```bash
# 显示最近 10 分钟的负载均衡日志
./scripts/filter-load-balance-logs.sh --since 10m server.log

# 按渠道 ID 过滤
./scripts/filter-load-balance-logs.sh --channel-id 1 server.log

# 显示特定模型的统计信息
./scripts/filter-load-balance-logs.sh --model gpt-4 --stats server.log

# 显示详细的策略分解
./scripts/filter-load-balance-logs.sh --details --limit 5 server.log

# 显示最近 1 小时的决策摘要
./scripts/filter-load-balance-logs.sh --since 1h --summary server.log

# 导出为 CSV
./scripts/filter-load-balance-logs.sh --format csv --limit 1000 server.log > output.csv

# 按分数范围过滤
./scripts/filter-load-balance-logs.sh --min-score 1000 --max-score 2000 server.log

# 仅显示排名第一的渠道
./scripts/filter-load-balance-logs.sh --max-rank 1 server.log
```

**功能：**
- 按时间范围过滤日志（--since, --until）
- 按渠道 ID 过滤（--channel-id）
- 按模型名称过滤（--model）
- 按分数范围过滤（--min-score, --max-score）
- 按排名过滤（--min-rank, --max-rank）
- 显示决策摘要（--summary）
- 显示统计信息（--stats）
- 显示详细的策略分解（--details）
- 仅显示决策日志或渠道详情（--decision-only, --channel-only）
- 限制输出条数（--limit）
- 多种输出格式：table, json, csv（--format）

**日志类型：**
- `Load balancing decision completed` - 总体决策信息
- `Channel load balancing details` - 每个渠道的详细信息

## 📚 文档

- **[QUICK_START.md](./QUICK_START.md)** - E2E 测试快速入门
- **[MIGRATION_TEST.md](./MIGRATION_TEST.md)** - 数据库迁移测试详细文档
- **[USAGE_EXAMPLE.md](./USAGE_EXAMPLE.md)** - 使用示例

## 🔧 环境要求

### E2E 测试
- Node.js 和 pnpm
- Go 1.21+
- SQLite3

### 迁移测试
- Go 1.21+
- unzip
- curl
- SQLite3
- 可选: jq (用于 JSON 解析)

## 📦 目录结构

```
scripts/
├── README.md                    # 本文件
├── QUICK_START.md               # E2E 测试快速入门
├── MIGRATION_TEST.md            # 迁移测试详细文档
├── USAGE_EXAMPLE.md             # 使用示例
│
├── e2e-test.sh                  # E2E 测试主脚本
├── e2e-backend.sh               # E2E 后端管理脚本
├── migration-test.sh            # 单版本迁移测试
├── migration-test-all.sh        # 批量迁移测试
│
├── axonhub-e2e                  # E2E 测试二进制文件（自动生成）
├── axonhub-e2e.db               # E2E 测试数据库（自动生成）
├── e2e-backend.log              # E2E 后端日志（自动生成）
├── .e2e-backend.pid             # E2E 后端进程 ID（自动生成）
│
└── migration-test/              # 迁移测试工作目录（自动生成）
    ├── cache/                   # 二进制文件缓存
    │   ├── v0.1.0/
    │   │   └── axonhub
    │   └── v0.2.0/
    │       └── axonhub
    └── work/                    # 临时工作目录
        ├── axonhub-current
        ├── migration-test.db
        ├── migration-test.log
        └── migration-plan.json
```

## 🚀 快速开始

### 运行 E2E 测试

```bash
# 方式 1: 使用 pnpm（推荐）
cd frontend
pnpm test:e2e

# 方式 2: 直接运行脚本
./scripts/e2e-test.sh
```

### 测试数据库迁移

```bash
# 测试单个版本
./scripts/migration-test.sh v0.1.0

# 测试多个版本
./scripts/migration-test-all.sh
```

## 🔍 故障排查

### E2E 测试失败

```bash
# 查看后端日志
cat scripts/e2e-backend.log

# 检查后端状态
./scripts/e2e-backend.sh status

# 手动重启后端
./scripts/e2e-backend.sh restart

# 查看测试报告
cd frontend
pnpm test:e2e:report
```

### 迁移测试失败

```bash
# 保留测试产物以便调试
./scripts/migration-test.sh v0.1.0 --keep-artifacts

# 查看迁移日志
cat scripts/migration-test/work/migration-test.log

# 查看迁移计划
cat scripts/migration-test/work/migration-plan.json

# 检查数据库
sqlite3 scripts/migration-test/work/migration-test.db
```

### 端口占用

```bash
# 检查端口 8099 是否被占用
lsof -i :8099

# 停止占用端口的进程
./scripts/e2e-backend.sh stop
```

### 清理环境

```bash
# 清理 E2E 环境
./scripts/e2e-backend.sh clean

# 清理迁移测试缓存
rm -rf scripts/migration-test/

# 清理前端测试报告
cd frontend
rm -rf playwright-report test-results
```

## 🌟 最佳实践

### E2E 测试
1. 使用 `pnpm test:e2e` 一键运行所有测试
2. 使用 `pw-test-` 前缀标识测试数据
3. 每个测试应该独立，不依赖其他测试
4. 使用 `waitForGraphQLOperation()` 等待异步操作

### 迁移测试
1. 在发布前测试所有主要版本的迁移路径
2. 使用 `migration-test-all.sh` 批量测试
3. 保留失败的测试产物以便调试
4. 定期清理缓存目录以释放磁盘空间

### 数据库迁移测试 (Migration Testing)

测试数据库版本升级迁移：

```bash
# 测试从 v0.1.0 迁移到当前分支
./scripts/migration-test.sh v0.1.0

# 批量测试最近 3 个稳定版本
./scripts/migration-test-all.sh

# 查看详细文档
cat scripts/MIGRATION_TEST.md
```

详细说明请参考 [MIGRATION_TEST.md](./MIGRATION_TEST.md)

## 📝 环境变量

### E2E 测试
```bash
AXONHUB_SERVER_PORT=8099              # 后端端口
AXONHUB_DB_DSN="file:..."             # 数据库连接
AXONHUB_LOG_OUTPUT="stdio"            # 日志输出
AXONHUB_LOG_LEVEL="debug"             # 日志级别
AXONHUB_LOG_ENCODING="console"        # 日志格式
```

### 迁移测试
```bash
GITHUB_TOKEN="your_token"             # GitHub API Token（可选）
```

## 🤝 贡献

如果你发现脚本有问题或有改进建议，欢迎提交 Issue 或 PR。

## 📄 许可证

与项目主体相同的许可证。
