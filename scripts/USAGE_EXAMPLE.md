# E2E 测试使用示例

## 最简单的用法

```bash
cd frontend
pnpm test:e2e
```

就这样！一个命令搞定所有事情 🎉

## 输出示例

```
🚀 Starting E2E Test Suite...

📦 Starting E2E backend server...
Removing old E2E database: axonhub-e2e.db
Building backend...
Starting backend on port 8099 with database axonhub-e2e.db...
E2E backend server started (PID: 12345)
Waiting for server to be ready...
E2E backend server is ready!

✅ Backend server ready

🧪 Running Playwright tests...

Running 15 tests using 4 workers

  ✓  [setup] › setup.spec.ts:12:3 › System Setup › initialize system with owner account (5.2s)
  ✓  [chromium] › api-keys.spec.ts:5:3 › Admin API Keys Management › can create and delete API key (3.1s)
  ✓  [chromium] › channels.spec.ts:5:3 › Admin Channels Management › can create channel (2.8s)
  ✓  [chromium] › users.spec.ts:5:3 › Admin Users Management › can create user (2.5s)
  ...

  15 passed (45.3s)

✅ All tests passed!

🧹 Cleaning up...
Stopping E2E backend server...
```

## 其他常用场景

### 调试失败的测试

```bash
# 1. 运行测试（会显示浏览器）
pnpm test:e2e:headed

# 2. 或者使用调试模式
pnpm test:e2e:debug

# 3. 查看测试报告
pnpm test:e2e:report
```

### 只运行特定测试

```bash
# 运行特定文件
pnpm test:e2e -- tests/users.spec.ts

# 运行匹配的测试
pnpm test:e2e -- --grep "create user"
```

### 使用 UI 模式（推荐用于开发）

```bash
pnpm test:e2e:ui
```

这会打开一个交互式界面，可以：
- 选择要运行的测试
- 查看测试步骤
- 时间旅行调试
- 查看网络请求

## 测试失败后的调试

测试失败时，数据库会保留，方便调试：

```bash
# 查看后端日志
cat ../../scripts/e2e-backend.log

# 检查数据库
sqlite3 ../../scripts/axonhub-e2e.db

# 查看所有表
sqlite> .tables

# 查看用户
sqlite> SELECT * FROM users;

# 退出
sqlite> .quit
```

## 清理环境

```bash
# 清理所有 E2E 文件（数据库、日志、PID）
cd ../..
./scripts/e2e-backend.sh clean
```

## 常见问题

### Q: 端口 8099 被占用怎么办？

```bash
# 查看占用端口的进程
lsof -i :8099

# 停止 E2E 后端
cd ../..
./scripts/e2e-backend.sh stop
```

### Q: 测试卡住不动？

1. 检查后端是否正常运行：`../../scripts/e2e-backend.sh status`
2. 查看后端日志：`cat ../../scripts/e2e-backend.log`
3. 重启后端：`../../scripts/e2e-backend.sh restart`

### Q: 想保留数据库进行手动测试？

数据库默认会保留！你可以：

```bash
# 1. 运行测试
pnpm test:e2e

# 2. 手动启动后端（使用同一个数据库）
cd ../..
./scripts/e2e-backend.sh start

# 3. 现在可以在浏览器中访问 http://localhost:8099
# 使用测试中创建的账户登录

# 4. 完成后停止
./scripts/e2e-backend.sh stop
```

## 性能提示

- **并行执行**：测试会自动并行运行，充分利用多核 CPU
- **复用服务器**：开发时会复用已运行的前端服务器
- **快速反馈**：setup 测试完成后，其他测试立即开始

## CI/CD 集成

在 CI 环境中，测试会：
- 串行运行（更稳定）
- 失败时重试 2 次
- 不复用服务器
- 生成 HTML 报告

```yaml
# GitHub Actions 示例
- name: Run E2E tests
  run: |
    cd frontend
    pnpm test:e2e
```
