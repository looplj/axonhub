# 操作插槽

[插件系统](README.md)的 UI 扩展能力。首版只开放全局工具栏 `app.header.actions`：插件向已登录布局右上角注册操作，点击后打开插件弹窗。渠道操作菜单留待后续，不实现内嵌 panel；`ui.panels` 继续拒绝。

实施任务为总览 T-17，依赖 T-07、T-16，首版已实现。以下描述当前合同。

## 1. 挂载位置与范围

`frontend/src/components/layout/app-header.tsx` 的右侧控件区，在配额控件之后、设置/语言/主题控件之前挂载。它属于公共认证布局，插件独立页面也能显示该入口。移动端仍在顶栏显示一个插件操作菜单。

- 桌面最多直接显示两个插件图标按钮，其余进入「更多插件操作」菜单；移动端全部收进一个菜单。
- 标题、tooltip、aria-label、菜单项由宿主渲染。图标仅允许宿主预定义名称，不接受 HTML、SVG 字符串或外部 URL。
- 点击前只有描述数据，不签发 token、不创建 iframe、不执行插件 JS。
- 全局插槽不继承当前选中的项目：初始化 `projectId: null`，Bridge 请求不发送 `X-Project-ID`。渠道等资源上下文只在未来对应插槽中引入。

## 2. 通用结构

| 组件 | 职责 |
|------|------|
| Slot registry | 稳定名称、允许的 effect、上下文合同；首版只有 `app.header.actions` |
| PluginActionItems | 原生按钮/菜单、标题回退、图标、稳定排序 |
| PluginActionHost | 单个弹窗及当前 action；由公共布局持有，不挂在菜单项内部 |
| PluginFrame / Bridge | 复用资源 token、sandbox、主题、GraphQL、fetch、错误隔离与重试 |

每个新位置需要宿主显式挂载并构造上下文，公共加载和通信机制不重复实现。不通过 CSS selector 寻找任意 DOM，也不执行插件提供的 React 组件。首版将 Items 和 Host 封装在一个 `PluginHeaderActions` 组件中，待第二个插槽出现再提取公共渲染组件。

## 3. Manifest

```json
{
  "ui": {
    "actions": [{
      "key": "inspect",
      "slot": "app.header.actions",
      "title": { "en": "Request inspector", "zh-CN": "请求审查" },
      "icon": "search",
      "effect": { "type": "dialog", "entry": "ui/inspect.html" }
    }]
  }
}
```

- `ui.actions` 可省略，出现时必须是数组。`key` 规则同 page，在 actions 内唯一；page 与 action 可以同 key，通过类型区分。
- `slot` 首版只接受 `app.header.actions`。`channel.actions` 等未知位置拒绝安装，避免安装成功但入口不出现。
- `title` 同 [manifest title](manifest.md#title)，保留字符串/多语言对象。
- `icon` 可省略，默认 `puzzle`；允许 `puzzle`、`search`、`activity`、`settings`。
- `effect` 必填，首版只接受 `{type: "dialog", entry: "ui/…html"}`。entry 校验、包内存在性、打包检查、资源访问规则与 page 一致。
- actions 条目及 effect 都使用固定字段白名单，错误包含完整字段路径。
- 不开放 `height`、`resize`、后台直接执行、任意导航 URL 或动态 badge。弹窗尺寸由宿主决定。
- 排序按 pluginID 升序，同插件按 manifest 声明顺序；与后端钩子 priority 无关。

新增 action，或现有 action 改变 slot/effect 类型时，加入升级预览；仅改 title/icon/entry 沿用页面代码升级规则。预览展示新增操作名称（至少保留 key），已启用插件确认后才替换。

## 4. 目录、资源与权限

`pluginUIEntries` 增加 `actions`，每项包含 `pluginID`、`key`、`slot`、`title`、`icon`、`effect { type entry }` 和包版本标识 `revision`，不返回包、secret 或用户凭据。对登录用户可查，只返回启用且运行状态有效的插件。目录沿用现有管理操作失效刷新与窗口重新聚焦刷新机制，不新增实时推送。

后端统一 UI 能力判断与入口枚举：

1. `createPluginUISession` 接受 manifest 声明的 page entry 或 action.effect.entry。
2. GraphQL 与出站 fetch 端点同时接受 pages/actions 至少一项非空的插件；两者都没有、停用或不存在时仍为 404。
3. 只有 actions 的插件也可读数据、按原权限读写本插件 KV、按 granted_network 出站。
4. 不新增用户或插件权限。数据操作继续取用户与插件授权交集，自助操作限制、凭证保护和 KV 写资格不变。入口可见不代表有数据访问权限。

UI entry token 绑定资源入口，不授予新数据权限。每次弹窗挂载独立签发 token，避免同 entry 的 page/action 被查询缓存合并；Bridge session 与 pending 调用也独立。

## 5. 弹窗与 Bridge

点击宿主操作后打开一个 Dialog，标题使用 action 的本地化 title，正文复用 PluginFrame。宽度不超过视口，正文占可用高度并在 iframe 内滚动；移动端同样可用。关闭按钮、键盘关闭和焦点恢复由宿主 Dialog 管理。

`UIInit` 向后兼容新增字段：

```ts
// 独立页面：page 为 key，action/slot 为 null，context 为 {}。
// 工具栏弹窗：page/panel 为 null，action 为 key。
action: string | null;
slot: "app.header.actions" | null;
context: {}; // 首版全局插槽无资源上下文
```

旧 `panel` 字段保留为 null，不开放 panels。ready/onInit/onTheme/graphql/fetch/navigate/toast 的签名与行为保持不变。navigate 被接受后关闭弹窗，防止遮挡目标页面；路由变化也关闭弹窗。首版不增加关闭/刷新宿主的 Bridge 方法。

- 同时只打开一个 action 弹窗；切换 action、关闭、语言切换、重试均销毁旧 iframe 会话、取消在途请求。语言切换重新初始化当前 action；全局插槽不因选中项目变化而重建。
- 目录刷新发现插件停用、卸载或 action 消失时关闭；升级后 revision 变化时重新挂载，签发当前版本 token。
- 错误、握手超时仅替换弹窗正文，不影响工具栏和当前页面；重试创建新会话。复用 10 秒 hello 超时与现有 Bridge 调用预算。
- 列表加载失败不阻塞内置控件；在插件入口提供重试。无 action 时不占位。

## 6. 实施清单

- `internal/objects/plugin.go`：action/effect 类型、解析、白名单、校验、UI 入口枚举。
- `internal/server/plugin/{manifest,manager}.go`：升级 diff/preview 增加 actions。
- `internal/server/plugin/{package,pack,ui,ui_catalog}.go`：包入口、签发、目录及 actions-only。
- `internal/server/gql/plugin_call.go`：共用准入由 page 改为 UI；GraphQL 与 fetch 同步。
- `internal/server/gql/plugin.graphql`、gqlgen 映射和生成代码：目录及升级预览完整传递。
- `features/plugins/data/plugin-ui.ts`、管理页升级预览查询和组件：读取 actions。
- `features/plugins/components/plugin-frame.tsx`：page/action 上下文、显式项目作用域、每实例 token。
- `features/plugins/components/plugin-header-actions.tsx`、`components/layout/app-header.tsx`：工具栏和弹窗。
- 本地化、插件开发文档和可安装的工具栏示例包源码。

## 7. 验收

| 类别 | 标准 |
|------|------|
| Manifest | actions-only 可解析；重复 key、未知 slot/icon/effect/字段、非法或缺失 entry 被拒；panels 仍拒绝 |
| 升级 | 新增 action 返回预览，确认后生效，目录无敏感配置 |
| 资源 | action entry 可签发和加载；未声明 HTML、后端脚本和越界路径仍拒绝；打包/安装均检查 entry 存在 |
| 权限 | actions-only 的 GraphQL、KV、fetch 沿用权限矩阵；停用和无 UI 插件为 404 |
| UI | 工具栏图标、溢出菜单、移动端菜单、标题回退；点击前不创建 iframe；点击后握手和主题同步 |
| 作用域 | 全局 action init.projectId 为 null，请求无项目头；页面原有项目行为不变 |
| 生命周期 | 关闭/切换/重试/停用清理会话；同 entry page/action 不共用 token；旧消息无效；导航后关闭 |
| 错误 | 不握手/脚本异常仅影响弹窗，当前页面仍可操作；目录查询失败可重试 |

## 8. 后续扩展

`channel.actions` 可复用 action 描述和弹窗宿主，增加渠道行菜单挂载与 `{channelId}` 的有类型上下文。未来遇到实际内嵌内容需求再定义 panel 类型，不在本次实现中预留布局、自动 resize 或用户拖拽配置。


## 9. 验证记录

- 后端针对性测试通过：manifest、包入口、升级预览、资源 token、GraphQL/KV/fetch 与 actions-only 目录及会话。
- `node --test src/features/plugins/plugin-ui.test.mjs ../internal/server/plugin/uisdk/bridge.test.mjs`：21 项通过。
- 在 frontend 运行 `PLAYWRIGHT_CHANNEL=chrome node --test tests/plugin-actions/browser.test.mjs`：4 项通过。测试自建临时 Vite 夹具与独立无头浏览器，mock API 使用真实 PluginHeaderActions、PluginFrame 和 sandbox Bridge，不连接业务数据库，不重启开发服务。未安装系统 Chrome 时，安装 Playwright Chromium 并省略 PLAYWRIGHT_CHANNEL。
- 全量 TypeScript 检查未通过（207 条诊断）；插件和 app-header 范围无诊断。未运行 lint 或应用构建。
- 示例源码：[toolbar-notes](../../../plugins/examples/toolbar-notes/README.md)，打包/解析由 `TestUIActionsExample` 验证。
