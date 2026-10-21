# AxonHub Fork Release v1.0.0-beta10

基于上游 looplj/axonhub 的二开版本。本版本聚焦三块：**模型级冷冻（可观测 + 手动冷冻/解冻）**、**模型关联预览拖拽修复**、**备份支持上传**，并附带宝塔 Docker 无缝更新方案。

## Highlights

### 1. 模型级冷冻（渠道 × 模型 局部冷冻）
- 客户端取消冷却新增动作 **「仅冷冻模型」**：连续取消达到阈值后只冷冻 (渠道, 模型) 组合，渠道其他模型继续服务
- 冷冻持久化在渠道 `model_cooldowns`，到期自动恢复；重复触发不叠加（保留最早到期）
- 计数按模型隔离：模型 A 的取消不会把模型 B 冷冻掉
- **可观测**：模型关联预览卡片显示「❄ 冷冻中」徽标；渠道列表行菜单「模型冷冻 (N)」可查看全部并解冻
- **手动冷冻/解冻**：预览卡片 ❄ 菜单（冷冻 30 分钟 / 24 小时 / 永久 / 解冻）；GraphQL 新增 `freezeChannelModel` / `clearChannelModelCooldown`

### 2. 模型关联预览（拖拽与分组修复）
- 预览卡片按 **渠道 × Key分组 × 模型集合** 分组：同渠道两个模型 = 两张卡
- 卡片按 **产生它的规则优先级** 排序：同渠道多卡可独立拖拽换序，保存后持久化
- 拖拽映射覆盖 channel_model / channel_tags_model / channel_regex 全部规则类型
- 防抖预览请求加序号守卫，旧响应不再覆盖新结果

### 3. 备份支持上传
- 系统设置 → 备份新增 **「同时上传到备份存储」**：生成的备份直接写入自动备份配置的数据存储（S3 / WebDAV / 本地 / 数据库）
- 上传成功后 toast 提示；原下载功能不变

### 4. 宝塔 Docker 无缝更新
- `deploy/axonhub-update.sh`：在线备份（pg_dump / SQLite 拷贝）→ `.env` 换镜像 tag → 重建容器 → `/health` 健康检查 → 失败自动回滚
- `deploy/baota-update-guide.md`：完整指南（含从上游源码切换到 fork 的场景）

### 5. 其他（自上一 fork 版本累计）
- Key 分组（per-model routing）、4 位删除确认、查看秘钥修复
- 请求日志「清理日志」对话框（预览 + 后台执行）
- 客户端取消冷却策略（全局/项目覆盖、阈值/动作/时长）
- API Key 模型关联清理、模型关联验证加固

## Docker 镜像

```bash
# 稳定 tag（由本 Release 触发的构建）
docker pull ghcr.io/shifuf/axonhub:v1.0.0-beta10
# 开发跟随（dev 分支最新构建）
docker pull ghcr.io/shifuf/axonhub:dev-latest
```

服务器更新：

```bash
cd /www/wwwroot/axonhub
bash axonhub-update.sh v1.0.0-beta10   # 或 bash axonhub-update.sh 取 dev-latest
```

## 测试

- `frontend/tests/model-cooldown.ui.mjs` — 14/14（双模型卡、拖拽换序、冷冻徽标可见、重开持久、解冻消失）
- `frontend/tests/misc-fixes.ui.mjs` — 17/17
- `scripts/e2e/client-cancellation.e2e.mjs` — 31/31
- `go test ./internal/server/biz/ ./internal/server/orchestrator/ ./internal/server/backup/` 全绿
- `tsc --noEmit` 通过

## 升级说明

- 数据库自动迁移（Ent），无需手工 SQL；`model_cooldowns` 列会自动创建
- 建议更新前执行一次备份（更新脚本内置自动备份）