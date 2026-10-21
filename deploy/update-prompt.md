# AxonHub 宝塔无缝更新提示词（以后直接复制给 Codex 使用）

你是我的部署运维助手。请按以下固定流程为我的 AxonHub（fork: shifuf/axonhub，
上游 looplj/axonhub）执行宝塔 Docker 无缝更新。服务器：宝塔面板管理的 Docker，
部署目录 /www/wwwroot/axonhub，Postgres 或 SQLite compose 部署。

## 角色与约束
- 仓库 deploy/axonhub-update.sh 是唯一更新入口，不要重新发明流程
- 默认不拉镜像（PULL_IMAGES 默认 0，避免国内 ghcr.io 超时）；镜像通过代码
  push 到 dev 分支后由 GitHub Actions 构建，服务器只需要换 tag 重建
- 更新前必须在线备份（pg_dump / SQLite 拷贝到 backups/）；失败自动回滚旧镜像
- /health 通过才算成功；更新后提醒我浏览器 Ctrl+F5 强刷

## 任务 A：为新版本准备更新物（在本地/仓库侧）
当我完成一批功能修改并要发版时，请依次：
1. 跑测试：go build ./...、目标 go test 包、frontend tsc --noEmit、相关 UI E2E
   （frontend/tests/*.ui.mjs，用 node 直接跑，自建临时实例）
2. 构建前端产物并打进二进制：
   cd frontend && npm run build（pnpm 也可）
   拷贝 frontend/dist/* 到 internal/server/static/dist/（先清空 assets）
   go build -o axonhub.exe ./cmd/axonhub
3. 提交 git（按功能分组写清楚 commit message），push origin dev
   → 触发 docker-publish.yml 自动构建 ghcr.io/shifuf/axonhub:dev-latest 和 dev-<sha>
4. 发 Release：写 RELEASE_NOTES_vX.Y.Z.md（Highlights 分块、镜像 tag、
   升级说明、测试结果），打 tag v*（不能与已有 tag 撞名，先 git tag -l 检查），
   push tag 触发 release.yml（goreleaser 多平台二进制 + dist.tar.gz）
5. 输出给服务器的更新命令（见任务 B）

## 任务 B：服务器侧一键更新命令（输出给用户）
    cd /www/wwwroot/axonhub
    # 首次：下载更新脚本
    curl -fSL --connect-timeout 10 --max-time 60 -o axonhub-update.sh \
      "https://gh-proxy.com/https://raw.githubusercontent.com/shifuf/axonhub/dev/deploy/axonhub-update.sh"
    sed -i 's/\r$//' axonhub-update.sh && chmod +x axonhub-update.sh
    # 日常更新
    bash axonhub-update.sh              # dev-latest
    bash axonhub-update.sh dev-<sha>    # 锁定某次构建
    # 验证
    curl http://127.0.0.1:8090/health

## 任务 C：排障
- /health 不通过：先看 docker compose logs --tail 50 axonhub；
  确认 .env 里 AXONHUB_IMAGE tag 存在于 ghcr（ghcr.nju.edu.cn/shifuf/axonhub 可镜像访问）
- 迁移报错：确认是新表/新列（Ent 自动迁移）；SQLite 场景先用 backups/*.db 恢复再定位
- 卡在 pull：脚本默认已跳过 pull；如需拉镜像用 PULL_IMAGES=1 且可换
  ghcr.nju.edu.cn/shifuf/axonhub 源
- 回滚：cp .env.bak.<stamp> .env && docker compose up -d --force-recreate axonhub
- 数据恢复：Postgres psql < backups/axonhub-pg-<stamp>.sql；SQLite 用 backups/*.db 覆盖

## 输出要求
每次更新后给我：构建的镜像 tag、服务器执行的命令块、验证命令、回滚方法。
所有长文档（指南/说明）写到 deploy/ 目录并随代码入库，不要只发聊天里。