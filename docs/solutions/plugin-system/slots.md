# 插槽

[插件系统](README.md)的可选能力：插件面板嵌在 AxonHub 已有页面里，v1 的候选位置是 dashboard。

本篇不在 [任务表](README.md#9-任务管理) 里。当前交付只有 [独立页面](pages.md)。manifest 的 `ui` 里出现 `panels` 时拒绝安装，见 [字段表](manifest.md#3-字段)。下面是排期时的实现合同。排期后加入任务表，依赖 T-07。

## 1. 背景

dashboard 在 `frontend/src/features/dashboard/index.tsx`，路由是 `/`。渠道没有详情页，编辑是对话框，所以不在渠道上挂插槽。

面板和页面复用同一个 `PluginFrame` 实现、UI token 机制和插件 GraphQL 端点，资源规则见 [pages.md](pages.md)，共用 SDK 与消息协议见 [ui-bridge.md](ui-bridge.md)。每个 iframe 独立签发绑定入口的 token、建立独立 Bridge 会话。本篇只写面板自己的挂载与限制。

## 2. 目标

1. manifest 的 `ui.panels` 能在 dashboard 底部渲染。
2. 面板报错或 10 秒内不握手时，只换成一张错误卡片，不影响 dashboard 其余部分。
3. 面板的数据读写与页面相同，见 [插件 GraphQL](pages.md#7-插件-graphql)。

### 非目标

- 渠道编辑对话框和系统设置里的插槽。

## 3. User Stories

1. 作为控制台用户，我希望在 dashboard 直接看到插件面板，不用进入插件页面。

## 4. Manifest

开放时并入 [manifest.md](manifest.md)。

```json
"ui": {
  "panels": [{ "key": "overview", "slot": "dashboard", "entry": "ui/overview.html", "height": 320,
               "title": { "en": "Failover", "zh-CN": "故障转移" } }]
}
```

- `slot` 只能是 `dashboard`。
- `title` 接受字符串或多语言对象，校验和显示回退复用 [title](manifest.md#title)；GraphQL 与前端类型沿用页面的约定。
- `entry` 规则与 page 相同。`createPluginUISession` 的 entry 校验改为「某个 panel 或 page 的 entry」。
- 开放面板时，插件 GraphQL 端点的准入条件同步改为「插件存在、已启用，且 `ui.pages` 或 `ui.panels` 至少一项非空」。只有 panels 的插件同样能查询数据及按既有权限读写 KV；两者都没有时仍返回 404。当前交付继续只接受 pages。

## 5. Dashboard

`features/dashboard/index.tsx` 底部渲染 `<PluginSlot slot="dashboard" />`。它读 `pluginUIEntries.panels`，为每个 `slot=dashboard` 的面板渲染 `<PluginFrame>`。

- 桥接增加 `resize`。高度由面板设置，限制 80–2000 px；初始高度取 manifest 的 `height`。
- `init` 使用共用的 `UIInit`：page 为 null，panel 为当前面板 key，slot 为 dashboard。ready、主题、GraphQL、导航和通知均复用 UI Bridge。
- 失败时显示「面板出错」和重试，不影响相邻面板。

## 6. 限制

排期时并入 [运行限制](README.md#运行限制)，在此之前以本表为准。

| 项 | 限制 |
|----|------|
| 面板高度 | 80–2000 px |
| 握手 | 与页面相同，10 s 内需发 `hello` |

## 7. 改动清单

- `internal/objects/plugin.go`：`PluginUIPanel`，`Validate()` 不再拒绝 `panels`。
- `internal/server/plugin/ui.go`：entry 校验包含 panel。
- `internal/server/plugin/graphql.go`：准入条件接受 pages 或 panels，沿用降权、自助 mutation 拒绝及 KV 权限校验。
- `internal/server/plugin/uisdk/bridge.js`：`resize`。
- `features/plugins/components/plugin-slot.tsx`；`plugin-frame.tsx` 支持 resize。
- `plugin.graphql`：`PluginUIEntries.panels`、`PluginUIPanel`。
- `locales/en/plugins.json`、`locales/zh-CN/plugins.json`：面板出错文案。

## 8. 验收

| 类别 | 标准 | 验证方式 |
|------|------|----------|
| 功能 | dashboard 面板完成握手、主题切换、GraphQL 查询、resize | `cd frontend && npm run test:unit`；`make e2e-test` 的 plugin-panel |
| 功能 | 仅声明 `ui.panels`、不声明 `ui.pages` 的已启用插件完成握手、GraphQL 查询及授权 KV 写入；无 pages/panels 或已停用的插件端点返回 404 | plugin-panel E2E；`go test ./internal/server/plugin/ -run TestPluginGraphql` |
| 错误处理 | 不握手或脚本抛错时，dashboard 其余卡片仍在 | E2E |
| 权限 | 与页面共用端点，复用 pages.md 的权限测试 | `go test ./internal/server/plugin/ -run TestPluginGraphql` |

## 9. 妥协

```text
1. 妥协：插槽只有 dashboard
   - 原因：渠道编辑是对话框，没有稳定的详情页
   - 影响：不能在渠道编辑处扩展
   - 升级触发：出现渠道详情页
   - 后续方向：增加 channel.detail，context 传入 channel id
```
