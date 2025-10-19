# AxonHub Migration Test Script

自动化测试数据库版本升级迁移的脚本。

## 功能特性

1. **自动下载和缓存二进制文件** - 从 GitHub Releases 下载指定 tag 的可执行文件，并缓存到本地
2. **测试版本升级** - 支持从任意 tag 版本迁移到当前分支最新代码
3. **生成迁移计划** - 自动生成迁移步骤计划（JSON 格式）
4. **执行迁移** - 按计划执行数据库迁移
5. **E2E 测试验证** - 迁移完成后自动运行 E2E 测试验证数据完整性
6. **配置一致性** - 使用与 e2e-test.sh 相同的配置，确保测试环境一致

## 使用方法

### 基本用法

```bash
# 测试从 v0.1.0 迁移到当前分支
./scripts/migration-test.sh v0.1.0

# 测试从 v0.2.0 迁移，跳过 E2E 测试
./scripts/migration-test.sh v0.2.0 --skip-e2e

# 测试迁移并保留测试产物
./scripts/migration-test.sh v0.1.0 --keep-artifacts

# 使用缓存的二进制文件（不重新下载）
./scripts/migration-test.sh v0.1.0 --skip-download
```

### 命令行参数

```
Usage:
  ./migration-test.sh <from-tag> [options]

Arguments:
  from-tag         要测试迁移的起始 Git tag（例如：v0.1.0）

Options:
  --skip-download  如果缓存中已存在二进制文件，跳过下载
  --skip-e2e       迁移后跳过 E2E 测试
  --keep-artifacts 测试完成后保留工作目录
  -h, --help       显示帮助信息
```

## 工作流程

脚本执行以下步骤：

1. **检测系统架构** - 自动检测操作系统和 CPU 架构（linux/darwin, amd64/arm64）
2. **下载旧版本二进制** - 从 GitHub Releases 下载指定 tag 的可执行文件
3. **构建当前版本** - 编译当前分支的最新代码
4. **生成迁移计划** - 创建包含迁移步骤的 JSON 文件
5. **初始化数据库** - 使用旧版本初始化数据库
6. **执行迁移** - 使用新版本运行数据库迁移
7. **运行 E2E 测试** - 验证迁移后的数据库功能正常
8. **清理** - 清理临时文件（可选保留）

## 目录结构

```
scripts/
├── migration-test.sh           # 主脚本
├── migration-test/             # 测试工作目录
│   ├── cache/                  # 二进制文件缓存
│   │   ├── v0.1.0/
│   │   │   └── axonhub         # 缓存的 v0.1.0 二进制
│   │   └── v0.2.0/
│   │       └── axonhub         # 缓存的 v0.2.0 二进制
│   └── work/                   # 工作目录（测试后清理）
│       ├── axonhub-current     # 当前分支编译的二进制
│       ├── migration-test.db   # 测试数据库
│       ├── migration-test.log  # 测试日志
│       └── migration-plan.json # 迁移计划
```

## 迁移计划格式

脚本会生成一个 JSON 格式的迁移计划文件：

```json
{
  "from_tag": "v0.1.0",
  "from_version": "0.1.0",
  "to_version": "0.2.0-dev",
  "platform": "darwin_arm64",
  "steps": [
    {
      "step": 1,
      "action": "initialize",
      "version": "v0.1.0",
      "binary": "/path/to/cache/v0.1.0/axonhub",
      "description": "Initialize database with version 0.1.0"
    },
    {
      "step": 2,
      "action": "migrate",
      "version": "current",
      "binary": "/path/to/work/axonhub-current",
      "description": "Migrate database to version 0.2.0-dev"
    }
  ]
}
```

## 配置说明

脚本使用以下环境变量配置（与 e2e-test.sh 保持一致）：

- `AXONHUB_SERVER_PORT=8099` - 测试服务器端口
- `AXONHUB_DB_DSN` - 数据库连接字符串（SQLite）
- `AXONHUB_LOG_OUTPUT=file` - 日志输出到文件
- `AXONHUB_LOG_LEVEL=debug` - 日志级别
- `GITHUB_TOKEN` - （可选）GitHub API Token，用于避免 API 限流

## 缓存机制

- 下载的二进制文件会缓存到 `scripts/migration-test/cache/<tag>/` 目录
- 如果缓存中已存在对应版本的二进制文件，默认会重新下载以确保最新
- 使用 `--skip-download` 选项可以跳过下载，直接使用缓存的文件

## 故障排查

### 下载失败

如果遇到 GitHub API 限流，可以设置 `GITHUB_TOKEN` 环境变量：

```bash
export GITHUB_TOKEN="your_github_token"
./scripts/migration-test.sh v0.1.0
```

### 查看详细日志

测试日志保存在 `scripts/migration-test/work/migration-test.log`：

```bash
tail -f scripts/migration-test/work/migration-test.log
```

### 保留测试产物

使用 `--keep-artifacts` 选项保留测试产物以便调试：

```bash
./scripts/migration-test.sh v0.1.0 --keep-artifacts

# 查看数据库
sqlite3 scripts/migration-test/work/migration-test.db

# 查看迁移计划
cat scripts/migration-test/work/migration-plan.json
```

## 示例输出

```
[INFO] AxonHub Migration Test Script

[INFO] Testing migration from v0.1.0 to current branch

[INFO] Detected platform: darwin_arm64

==> Step 1: Generate migration plan
[INFO] Generating migration plan...
[INFO] Downloading AxonHub v0.1.0 for darwin_arm64...
[INFO] Extracting archive...
[SUCCESS] Binary cached: /path/to/cache/v0.1.0/axonhub
[INFO] Building current branch binary...
[SUCCESS] Current binary built: /path/to/work/axonhub-current
[SUCCESS] Migration plan generated: /path/to/work/migration-plan.json

Migration Plan:
  From: v0.1.0 (0.1.0)
  To:   current (0.2.0-dev)
  Steps:
    1. Initialize database with v0.1.0
    2. Migrate to current branch

==> Step 2: Execute migration plan

==> Step 1: Initialize database with v0.1.0 (0.1.0)
[INFO] Initializing database with version 0.1.0...
[INFO] Waiting for server to initialize...
[SUCCESS] Database initialized with version 0.1.0

==> Step 2: Migrate to current (0.2.0-dev)
[INFO] Running migration with version 0.2.0-dev...
[INFO] Waiting for migration to complete...
[SUCCESS] Migration completed successfully
[SUCCESS] Migration plan executed successfully

==> Step 3: Run e2e tests
[INFO] Database copied to e2e location: /path/to/scripts/axonhub-e2e.db
🚀 Starting E2E Test Suite...
...
✅ All tests passed!
[SUCCESS] E2E tests passed!

[SUCCESS] Migration test completed successfully!

[INFO] Summary:
  From: v0.1.0
  To:   current branch
  Database: /path/to/work/migration-test.db
  Log: /path/to/work/migration-test.log
  Cache: /path/to/cache
```

## 注意事项

1. **需要 Go 环境** - 脚本需要编译当前分支代码，确保已安装 Go
2. **需要 unzip** - 用于解压下载的二进制文件
3. **端口占用** - 确保端口 8099 未被占用
4. **磁盘空间** - 缓存的二进制文件可能占用较多空间
5. **网络连接** - 首次运行需要从 GitHub 下载文件

## 批量测试

使用 `migration-test-all.sh` 可以批量测试多个版本的迁移：

```bash
# 自动测试最近 3 个稳定版本
./scripts/migration-test-all.sh

# 测试指定版本
./scripts/migration-test-all.sh --tags v0.1.0,v0.2.0,v0.2.1

# 批量测试但跳过 E2E
./scripts/migration-test-all.sh --skip-e2e

# 查看帮助
./scripts/migration-test-all.sh --help
```

## 与其他脚本的关系

- `e2e-test.sh` - 运行完整的 E2E 测试套件
- `e2e-backend.sh` - 管理 E2E 测试后端服务器
- `migration-test.sh` - 测试单个版本的数据库迁移
- `migration-test-all.sh` - 批量测试多个版本的迁移

本脚本复用了 e2e 测试的配置和基础设施，确保测试环境的一致性。
