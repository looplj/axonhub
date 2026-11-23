# 重置监控

AxonHub 顶栏插件。点击右上角的活动图标，在弹窗里看 Codex、Antigravity、Claude 的额度重置和重置卡。

数据来自 [AIHOT 重置监控](https://aihot.news/resets) 使用的公开接口，接入方式见 [Agent 接入 · REST API](https://aihot.news/agent?tab=api)：

| 产品 | 地址 |
| --- | --- |
| Codex | `https://aihot.news/api/v1/codex-resets` |
| Antigravity | `https://aihot.news/api/v1/antigravity-resets` |
| Claude | `https://aihot.news/api/v1/claude-resets` |

这是完整日历，不是 `/recent`。AIHOT 要求同一个来源最多 10 分钟拉一次，更频繁会返回 429。弹窗每次打开都会新建 iframe，所以快照放在本插件的 KV 里：正文一个键，`ETag`、取到时间和重试时间一个键。10 分钟内再次打开直接用这份快照，并用 `If-None-Match`；没变化时是 304。

## 429

返回 429 时不再立刻重试。响应里的 `Retry-After` 会记下来；没有这个头就等 10 分钟。已经有快照就继续显示，并写明何时可以再试。还没有快照就只显示限流说明。宿主自己返回的限流也按同样的方式处理。

## 没有 KV 写权限

能打开弹窗的人都可以读已有快照。写入需要当前用户有 `write_plugins` 或是 Owner。没有写权限时：

- 有人已经存过的日历照常显示，10 分钟内不会再请求。
- 没有快照，或快照已过期时，仍然请求 AIHOT，并在当前弹窗里显示结果。
- 不调用 `setPluginKV`。关闭弹窗后这份新结果不会留下来，页面上会说明这一点。

## 目录

```text
manifest.json
ui/resets/
format.test.mjs
```

界面里的判断在 `ui/resets/format.js`，不依赖宿主。本地测试：

```bash
node --test format.test.mjs
```

## 打包

按 [插件目录说明](../../README.md) 安装打包器，然后在 AxonHub 仓库根目录运行：

```sh
pack-plugin plugins/community/reset-watch -o /tmp/reset-watch.axplugin
```

在 AxonHub 的 PR 中递增 `manifest.json` 版本。合并到 `unstable` 后，AxonHub 向 [axonhub-plugins](https://github.com/looplj/axonhub-plugins) 发送事件，由那边检出这个提交并打包发布。

在本目录运行 `node --test *.test.mjs` 可执行插件测试。

## 安装

1. Owner 在「插件」›「插件管理」上传 `reset-watch-0.1.0.axplugin`。
2. 启用时授予这条出站规则：`^https://aihot\.news/api/v1/(codex|antigravity|claude)-resets$`。
3. 右上角出现「重置监控」。桌面端最多直接显示两个插件按钮，更多的在溢出菜单里。

不需要插件权限。个人额度和重置卡余额不在这个接口里，仍以各产品里看到的为准。确认帖的时间不是到账时间，插件也不预测还没宣布的下一次重置。
