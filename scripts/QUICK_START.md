# E2E Testing Quick Start

## 快速开始 (Quick Start)

### 🚀 一键运行所有测试 (One-Command Test)
```bash
cd frontend
pnpm test:e2e
```

**就这么简单！** 脚本会自动：
1. 删除旧的 E2E 数据库
2. 启动后端服务（端口 8099）
3. 启动前端服务（端口 5173）
4. 运行初始化测试
5. 并行运行所有测试
6. 测试结束后自动停止后端服务

### 测试执行流程 (Test Execution Flow)

1. ✅ **删除旧数据库** - 删除 `axonhub-e2e.db`
2. ✅ **启动后端服务** - 在端口 8099 上启动，使用 `axonhub-e2e.db`
3. ✅ **启动前端服务** - 在端口 5173 上启动
4. ✅ **初始化系统** - 运行 `setup.spec.ts`，创建随机 owner 账户
5. ✅ **并行测试** - 所有其他测试并行运行
6. ✅ **自动清理** - 测试结束后停止后端服务

### 常用命令 (Common Commands)

```bash
# 运行测试 (Run tests)
pnpm test:e2e                 # 无头模式运行所有测试
pnpm test:e2e:headed          # 有头模式运行（可见浏览器）
pnpm test:e2e:ui              # UI 模式运行（交互式）

# 调试 (Debug)
pnpm test:e2e:debug           # 调试模式
pnpm test:e2e:setup           # 只运行初始化测试

# 查看报告 (View reports)
pnpm test:e2e:report          # 查看测试报告
```

### 手动管理后端 (Manual Backend Management)

**注意：** 通常不需要手动管理后端，`pnpm test:e2e` 会自动处理！

如果需要手动控制：
```bash
cd ../..  # 回到项目根目录

# 启动后端
./scripts/e2e-backend.sh start

# 停止后端
./scripts/e2e-backend.sh stop

# 查看状态
./scripts/e2e-backend.sh status

# 清理所有 E2E 文件
./scripts/e2e-backend.sh clean
```

### 重要文件 (Important Files)

- `../../scripts/axonhub-e2e.db` - E2E 测试数据库（测试后保留，用于复现问题）
- `../../scripts/e2e-backend.log` - 后端服务日志
- `../../scripts/axonhub-e2e` - E2E 后端可执行文件
- `../../scripts/.e2e-backend.pid` - 后端进程 ID
- `playwright-report/` - 测试报告目录

### 环境变量 (Environment Variables)

```bash
# 默认值 (Defaults)
AXONHUB_ADMIN_PASSWORD=pwd123456  # Owner 密码
AXONHUB_API_URL=http://localhost:8099  # 后端 API 地址
```

### 故障排查 (Troubleshooting)

#### 后端启动失败
```bash
# 查看后端日志
cat ../../scripts/e2e-backend.log

# 检查端口占用
lsof -i :8099

# 手动停止并重启
../../scripts/e2e-backend.sh stop
../../scripts/e2e-backend.sh start
```

#### 测试失败
```bash
# 查看测试报告
pnpm test:e2e:report

# 调试模式运行
pnpm test:e2e:debug

# 检查数据库
sqlite3 ../../scripts/axonhub-e2e.db ".tables"
sqlite3 ../../scripts/axonhub-e2e.db "SELECT * FROM users;"
```

#### 清理环境
```bash
# 完全清理 E2E 环境（包括数据库、日志、可执行文件）
../../scripts/e2e-backend.sh clean

# 删除测试报告
rm -rf playwright-report test-results
```

### 测试最佳实践 (Best Practices)

1. ✅ 使用 `pw-test-` 前缀标识测试数据
2. ✅ 使用时间戳或随机字符串保证唯一性
3. ✅ 每个测试应该独立，不依赖其他测试
4. ✅ 使用 `waitForGraphQLOperation()` 等待异步操作
5. ✅ 使用灵活的选择器（支持中英文）

### 配置说明 (Configuration)

**后端配置:**
- 端口: 8099
- 数据库: `axonhub-e2e.db`
- 日志: `e2e-backend.log`

**前端配置:**
- 端口: 5173
- API 地址: `http://localhost:8099`

**测试配置:**
- 初始化测试: `setup.spec.ts` (串行运行)
- 其他测试: 并行运行
- 失败重试: CI 环境 2 次，本地 0 次
