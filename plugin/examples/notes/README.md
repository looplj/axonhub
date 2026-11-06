# Notes 示例插件

这个目录是一个可以安装的 UI 插件，对应插件系统当前已经开放的能力：独立页面、`configSchema`、插件 GraphQL，以及本插件自己的 KV。

当前构建拒绝包内的 `plugin.js`（`JS plugins are not enabled yet`）。事件和请求钩子还不能装，所以这个例子没有后端脚本。页面脚本在浏览器里跑，通过宿主的 `window.axonhub` 通信。

## 页面做什么

- 跟随控制台的明暗主题。
- 读取 `heading` 和 `maxNotes`。还没改过配置时，页面拿到的是 schema 里的默认值。这个例子没有 secret；当前实现会把 secret 字段以 `******` 交给页面，不给原文。
- 列表使用 `pluginKVList(first: 100, after: ...)`，沿 `nextCursor` 分页读取，直到游标为空。
- 用 `pluginKV` / `setPluginKV` / `deletePluginKV` 保存笔记。键形如 `note.<UUID>`，值是 JSON 字符串。能打开页面的人都可以读；写入要求当前用户有 `write_plugins`。
- 查询 `channels(first: 20)` 的名称、类型和状态。插件被授予 `read_channels` 且当前用户也有该权限时才返回数据，否则页面显示权限说明。这里不查询凭证。
- 用宿主 toast 提示保存和删除，并用 `axonhub.navigate("/channels")` 跳到渠道页。

侧栏标题随语言切换：英文 Notes，简体中文「笔记」。页面正文在 `zh` / `zh-CN` 下用中文，其他语言用英文。

## 目录

```text
manifest.json
ui/notes/index.html    入口，bridge 相对路径是 ../../_axonhub/bridge.js
ui/notes/app.js        页面逻辑，静态 import ./format.js
ui/notes/format.js
ui/notes/style.css
ui/assets/mark.svg
```

HTML、CSS、JS 都使用相对路径。不要写 `/assets/...` 这种站点根路径，否则会丢掉插件资源 URL 上的 token。`_axonhub/bridge.js` 由宿主提供，不要放进包里。

## 打包

在仓库根目录执行：

```bash
python3 plugin/scripts/pack-plugin.py plugin/examples/notes
```

脚本只打包 `manifest.json` 和 `ui/`。根目录的 `README.md` 不会进包，其它路径会被拒绝。默认产物是 `plugin/examples/notes.axplugin`，这个文件已被 Git 忽略。也可以用 `-o` 指定输出路径。

## 安装

1. 用 Owner 打开侧栏「插件」›「插件管理」（`/plugins`），上传 `notes.axplugin`。
2. 在授权对话框里勾选 `read_channels`，然后启用。不勾选时页面仍可使用，渠道列表会显示权限说明。
3. 侧栏「插件」分组会出现 Notes。直接地址是 `/plugins/example.notes/pages/notes`。
4. 在同一设置页修改 Heading 或 Maximum notes 后，重新打开该页面才会看到新配置。

保存笔记的账号需要 `write_plugins`。Owner 默认可以写。没有该权限的用户能看列表，输入框是禁用的。
