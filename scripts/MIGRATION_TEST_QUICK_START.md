# Migration Test Quick Start

## Overview

The `migration-test.sh` script tests database migrations for AxonHub by:
1. Setting up a database (SQLite, MySQL, or PostgreSQL)
2. Downloading a binary for a specific release tag
3. Initializing the database with the old version
4. Running migration to the current branch version
5. Optionally running e2e tests to verify the migration

## Quick Start

```bash
# SQLite (default, no Docker needed)
./scripts/migration-test.sh v0.1.0

# MySQL (requires Docker)
./scripts/migration-test.sh v0.1.0 --db-type mysql

# PostgreSQL (requires Docker)
./scripts/migration-test.sh v0.1.0 --db-type postgres

# Test all databases
./scripts/test-migration-all-dbs.sh v0.1.0
```

## Prerequisites

### For SQLite (Default)
- No additional dependencies required

### For MySQL
- Docker installed and running
- Port 13306 available (or modify `MYSQL_PORT` in the script)

### For PostgreSQL
- Docker installed and running
- Port 15432 available (or modify `POSTGRES_PORT` in the script)

## Options

| Option | Description |
|--------|-------------|
| `--db-type TYPE` | Database type: `sqlite`, `mysql`, or `postgres` (default: `sqlite`) |
| `--skip-download` | Skip downloading binary if cached version exists |
| `--skip-e2e` | Skip running e2e tests after migration |
| `--keep-artifacts` | Keep work directory after test completion |
| `--keep-db` | Keep database container after test completion (MySQL/PostgreSQL only) |
| `-h, --help` | Show help message |

## Common Usage Examples

### Keep database container for inspection
```bash
./scripts/migration-test.sh v0.1.0 --db-type mysql --keep-db
```

After the test completes, you can connect to the MySQL container:
```bash
docker exec -it axonhub-migration-mysql mysql -u axonhub -paxonhub_test axonhub_test
```

### Skip e2e tests
```bash
./scripts/migration-test.sh v0.1.0 --db-type postgres --skip-e2e
```

### Keep artifacts and database
```bash
./scripts/migration-test.sh v0.1.0 --db-type mysql --keep-artifacts --keep-db
```

### Use cached binary
```bash
./scripts/migration-test.sh v0.1.0 --skip-download
```

### Test with PostgreSQL and skip e2e tests
```bash
./scripts/migration-test.sh v0.1.0 --db-type postgres --skip-e2e
```

## Database Configuration

### MySQL
- **Container Name**: `axonhub-migration-mysql`
- **Port**: 13306
- **Database**: `axonhub_test`
- **User**: `axonhub`
- **Password**: `axonhub_test`
- **Root Password**: `axonhub_test_root`
- **Character Set**: utf8mb4
- **Collation**: utf8mb4_unicode_ci

### PostgreSQL
- **Container Name**: `axonhub-migration-postgres`
- **Port**: 15432
- **Database**: `axonhub_test`
- **User**: `axonhub`
- **Password**: `axonhub_test`

### SQLite
- **Database File**: `scripts/migration-test/work/migration-test.db`

## Database Connections

### MySQL (when using --keep-db)
```bash
# Command line
docker exec -it axonhub-migration-mysql mysql -u axonhub -paxonhub_test axonhub_test

# Connection string
mysql -h 127.0.0.1 -P 13306 -u axonhub -paxonhub_test axonhub_test
```

### PostgreSQL (when using --keep-db)
```bash
# Command line
docker exec -it axonhub-migration-postgres psql -U axonhub -d axonhub_test

# Connection string
psql -h localhost -p 15432 -U axonhub -d axonhub_test
```

### SQLite
```bash
sqlite3 scripts/migration-test/work/migration-test.db
```

## Directory Structure

```
scripts/
├── migration-test.sh           # Main script
├── migration-test/
│   ├── cache/                  # Cached binaries by version
│   │   └── v0.1.0/
│   │       └── axonhub
│   └── work/                   # Test artifacts
│       ├── axonhub-current     # Current branch binary
│       ├── migration-test.db   # SQLite database (if using SQLite)
│       ├── migration-test.log  # Server logs
│       └── migration-plan.json # Migration plan
```

## File Locations

| Item | Location |
|------|----------|
| Script | `scripts/migration-test.sh` |
| Cached binaries | `scripts/migration-test/cache/<version>/` |
| Test artifacts | `scripts/migration-test/work/` |
| Logs | `scripts/migration-test/work/migration-test.log` |
| Migration plan | `scripts/migration-test/work/migration-plan.json` |
| SQLite DB | `scripts/migration-test/work/migration-test.db` |

## Troubleshooting

### Docker not running
```
[ERROR] Docker daemon is not running. Please start Docker.
```
**Solution**: Start Docker Desktop or Docker daemon
```bash
open -a Docker  # macOS
```

### Port already in use
If you see errors about ports being in use, you can:
1. Stop the conflicting service
2. Modify the port variables in the script:
   - `MYSQL_PORT` (default: 13306)
   - `POSTGRES_PORT` (default: 15432)

### Container already exists
The script automatically removes existing containers with the same name before starting new ones.

### Binary download fails
If binary download fails:
1. Check your internet connection
2. Verify the tag exists: https://github.com/looplj/axonhub/releases
3. Set `GITHUB_TOKEN` environment variable if you're hitting rate limits

### Migration fails
Check the logs at `scripts/migration-test/work/migration-test.log` for details.

### View logs
```bash
cat scripts/migration-test/work/migration-test.log
```

### Check container logs
```bash
docker logs axonhub-migration-mysql
docker logs axonhub-migration-postgres
```

## Manual Database Inspection

### Keep the database for inspection
```bash
./scripts/migration-test.sh v0.1.0 --db-type mysql --keep-db --keep-artifacts
```

### Connect to MySQL
```bash
docker exec -it axonhub-migration-mysql mysql -u axonhub -paxonhub_test axonhub_test
```

### Connect to PostgreSQL
```bash
docker exec -it axonhub-migration-postgres psql -U axonhub -d axonhub_test
```

### View SQLite database
```bash
sqlite3 scripts/migration-test/work/migration-test.db
```

## Cleanup

### Manual cleanup of Docker containers
```bash
# Remove MySQL container
docker rm -f axonhub-migration-mysql

# Remove PostgreSQL container
docker rm -f axonhub-migration-postgres

# Remove all test artifacts
rm -rf scripts/migration-test/work

# Remove cached binaries
rm -rf scripts/migration-test/cache
```

### Clean all test artifacts
```bash
rm -rf scripts/migration-test/work
```

## Environment Variables

```bash
# Set GitHub token to avoid rate limits
export GITHUB_TOKEN=your_token_here

# Then run the script
./scripts/migration-test.sh v0.1.0
```

## CI/CD Integration

### Test all database types
```bash
#!/bin/bash
set -e

echo "Testing SQLite migration..."
./scripts/migration-test.sh v0.1.0 --db-type sqlite

echo "Testing MySQL migration..."
./scripts/migration-test.sh v0.1.0 --db-type mysql

echo "Testing PostgreSQL migration..."
./scripts/migration-test.sh v0.1.0 --db-type postgres

echo "All migration tests passed!"
```

### Simple loop version
```bash
#!/bin/bash
set -e

# Test all database types
for db_type in sqlite mysql postgres; do
  echo "Testing $db_type..."
  ./scripts/migration-test.sh v0.1.0 --db-type $db_type
done

echo "All tests passed!"
```

### GitHub Actions Example
```yaml
name: Migration Tests

on: [push, pull_request]

jobs:
  migration-test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        db-type: [sqlite, mysql, postgres]
        from-version: [v0.1.0, v0.2.0]
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run migration test
        run: |
          ./scripts/migration-test.sh ${{ matrix.from-version }} \
            --db-type ${{ matrix.db-type }}
```

---

# 迁移测试快速开始

## 概述

`migration-test.sh` 脚本通过以下步骤测试 AxonHub 的数据库迁移：
1. 设置数据库（SQLite、MySQL 或 PostgreSQL）
2. 下载特定发布标签的二进制文件
3. 使用旧版本初始化数据库
4. 运行迁移到当前分支版本
5. 可选地运行 e2e 测试以验证迁移

## 快速开始

```bash
# SQLite（默认，无需 Docker）
./scripts/migration-test.sh v0.1.0

# MySQL（需要 Docker）
./scripts/migration-test.sh v0.1.0 --db-type mysql

# PostgreSQL（需要 Docker）
./scripts/migration-test.sh v0.1.0 --db-type postgres

# 测试所有数据库
./scripts/test-migration-all-dbs.sh v0.1.0
```

## 前置条件

### SQLite（默认）
- 无需额外依赖

### MySQL
- 已安装并运行 Docker
- 端口 13306 可用（或在脚本中修改 `MYSQL_PORT`）

### PostgreSQL
- 已安装并运行 Docker
- 端口 15432 可用（或在脚本中修改 `POSTGRES_PORT`）

## 选项

| 选项 | 描述 |
|------|------|
| `--db-type TYPE` | 数据库类型：`sqlite`、`mysql` 或 `postgres`（默认：`sqlite`） |
| `--skip-download` | 如果缓存版本存在则跳过下载二进制文件 |
| `--skip-e2e` | 迁移后跳过运行 e2e 测试 |
| `--keep-artifacts` | 测试完成后保留工作目录 |
| `--keep-db` | 测试完成后保留数据库容器（仅限 MySQL/PostgreSQL） |
| `-h, --help` | 显示帮助信息 |

## 常用示例

### 保留数据库容器以供检查
```bash
./scripts/migration-test.sh v0.1.0 --db-type mysql --keep-db
```

测试完成后，可以连接到 MySQL 容器：
```bash
docker exec -it axonhub-migration-mysql mysql -u axonhub -paxonhub_test axonhub_test
```

### 跳过 e2e 测试
```bash
./scripts/migration-test.sh v0.1.0 --db-type postgres --skip-e2e
```

### 保留工作目录和数据库
```bash
./scripts/migration-test.sh v0.1.0 --db-type mysql --keep-artifacts --keep-db
```

### 使用缓存的二进制文件
```bash
./scripts/migration-test.sh v0.1.0 --skip-download
```

### 使用 PostgreSQL 并跳过 e2e 测试
```bash
./scripts/migration-test.sh v0.1.0 --db-type postgres --skip-e2e
```

## 数据库配置

### MySQL
- **容器名称**：`axonhub-migration-mysql`
- **端口**：13306
- **数据库**：`axonhub_test`
- **用户**：`axonhub`
- **密码**：`axonhub_test`
- **Root 密码**：`axonhub_test_root`
- **字符集**：utf8mb4
- **排序规则**：utf8mb4_unicode_ci

### PostgreSQL
- **容器名称**：`axonhub-migration-postgres`
- **端口**：15432
- **数据库**：`axonhub_test`
- **用户**：`axonhub`
- **密码**：`axonhub_test`

### SQLite
- **数据库文件**：`scripts/migration-test/work/migration-test.db`

## 数据库连接

### MySQL（使用 --keep-db 时）
```bash
# 命令行
docker exec -it axonhub-migration-mysql mysql -u axonhub -paxonhub_test axonhub_test

# 连接字符串
mysql -h 127.0.0.1 -P 13306 -u axonhub -paxonhub_test axonhub_test
```

### PostgreSQL（使用 --keep-db 时）
```bash
# 命令行
docker exec -it axonhub-migration-postgres psql -U axonhub -d axonhub_test

# 连接字符串
psql -h localhost -p 15432 -U axonhub -d axonhub_test
```

### SQLite
```bash
sqlite3 scripts/migration-test/work/migration-test.db
```

## 目录结构

```
scripts/
├── migration-test.sh           # 主脚本
├── migration-test/
│   ├── cache/                  # 按版本缓存的二进制文件
│   │   └── v0.1.0/
│   │       └── axonhub
│   └── work/                   # 测试工件
│       ├── axonhub-current     # 当前分支二进制文件
│       ├── migration-test.db   # SQLite 数据库（如果使用 SQLite）
│       ├── migration-test.log  # 服务器日志
│       └── migration-plan.json # 迁移计划
```

## 文件位置

| 项目 | 位置 |
|------|------|
| 脚本 | `scripts/migration-test.sh` |
| 缓存的二进制文件 | `scripts/migration-test/cache/<version>/` |
| 测试工件 | `scripts/migration-test/work/` |
| 日志 | `scripts/migration-test/work/migration-test.log` |
| 迁移计划 | `scripts/migration-test/work/migration-plan.json` |
| SQLite 数据库 | `scripts/migration-test/work/migration-test.db` |

## 故障排查

### Docker 未运行
```
[ERROR] Docker daemon is not running. Please start Docker.
```
**解决方案**：启动 Docker Desktop 或 Docker 守护进程
```bash
open -a Docker  # macOS
```

### 端口已被占用
如果看到端口被占用的错误，可以：
1. 停止冲突的服务
2. 在脚本中修改端口变量：
   - `MYSQL_PORT`（默认：13306）
   - `POSTGRES_PORT`（默认：15432）

### 容器已存在
脚本会在启动新容器之前自动删除同名的现有容器。

### 二进制文件下载失败
如果二进制文件下载失败：
1. 检查网络连接
2. 验证标签是否存在：https://github.com/looplj/axonhub/releases
3. 如果遇到速率限制，设置 `GITHUB_TOKEN` 环境变量

### 迁移失败
查看 `scripts/migration-test/work/migration-test.log` 中的日志以获取详细信息。

### 查看日志
```bash
cat scripts/migration-test/work/migration-test.log
```

### 查看容器日志
```bash
docker logs axonhub-migration-mysql
docker logs axonhub-migration-postgres
```

## 手动数据库检查

### 保留数据库以供检查
```bash
./scripts/migration-test.sh v0.1.0 --db-type mysql --keep-db --keep-artifacts
```

### 连接到 MySQL
```bash
docker exec -it axonhub-migration-mysql mysql -u axonhub -paxonhub_test axonhub_test
```

### 连接到 PostgreSQL
```bash
docker exec -it axonhub-migration-postgres psql -U axonhub -d axonhub_test
```

### 查看 SQLite 数据库
```bash
sqlite3 scripts/migration-test/work/migration-test.db
```

## 清理

### 手动清理 Docker 容器
```bash
# 删除 MySQL 容器
docker rm -f axonhub-migration-mysql

# 删除 PostgreSQL 容器
docker rm -f axonhub-migration-postgres

# 删除所有测试工件
rm -rf scripts/migration-test/work

# 删除缓存的二进制文件
rm -rf scripts/migration-test/cache
```

### 清理所有测试工件
```bash
rm -rf scripts/migration-test/work
```

## 环境变量

```bash
# 设置 GitHub token 以避免速率限制
export GITHUB_TOKEN=your_token_here

# 然后运行脚本
./scripts/migration-test.sh v0.1.0
```

## CI/CD 集成

### 测试所有数据库类型
```bash
#!/bin/bash
set -e

echo "Testing SQLite migration..."
./scripts/migration-test.sh v0.1.0 --db-type sqlite

echo "Testing MySQL migration..."
./scripts/migration-test.sh v0.1.0 --db-type mysql

echo "Testing PostgreSQL migration..."
./scripts/migration-test.sh v0.1.0 --db-type postgres

echo "All migration tests passed!"
```

### 简单循环版本
```bash
#!/bin/bash
set -e

# 测试所有数据库类型
for db_type in sqlite mysql postgres; do
  echo "Testing $db_type..."
  ./scripts/migration-test.sh v0.1.0 --db-type $db_type
done

echo "All tests passed!"
```

### GitHub Actions 示例
```yaml
name: Migration Tests

on: [push, pull_request]

jobs:
  migration-test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        db-type: [sqlite, mysql, postgres]
        from-version: [v0.1.0, v0.2.0]
    
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run migration test
        run: |
          ./scripts/migration-test.sh ${{ matrix.from-version }} \
            --db-type ${{ matrix.db-type }}
```
