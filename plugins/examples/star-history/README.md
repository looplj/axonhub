# Star 历史示例插件

这个目录是一个可以安装的 UI 插件，用来演示页面通过 `axonhub.fetch` 让宿主代发已授权的出站请求。规则写在 manifest 的 `network` 里。

这个例子只演示页面，没有 `plugin.js`。浏览器不会连接 `api.github.com` 或 `api.star-history.com`。页面脚本只调用 `window.axonhub.fetch`，由宿主按已授予的规则代发。CSP 的 `connect-src` 仍是 `'none'`，`img-src` 仍是 `'self' data:`。

`ctx.fetch` 使用同一份已授予规则，但这个例子没有后端脚本。

## 页面做什么

- 使用宿主推送的主题变量，跟随控制台的明暗和配色。握手完成前用 CSS 里的回退色。
- 读取 `https://api.github.com/repos/looplj/axonhub`，请求头是 `Accept: application/vnd.github+json`。页面显示 `full_name`、`stargazers_count` 和 `html_url`。
- 再读取 `https://api.star-history.com/chart?repos=looplj/axonhub&type=date&legend=top-left`。深色主题加上 `theme=dark`。响应是 SVG，页面把它放进 `data:image/svg+xml` 的图片里。query 不参与授权，所以明暗两种地址都落在同一条精确 URL 上。
- 不读取 GitHub stargazers。带 `starred_at` 的列表需要登录，这张图由 star-history 提供。
- 响应不是 2xx 时，页面显示状态码和一小段响应正文。`axonhub.fetch` 被拒绝时，显示 bridge 错误码。仓库和图片互不影响：一边失败时，另一边已经拿到的内容仍留在页面上。
- 不发送 `Authorization`，也不保存 token。请求只附加 `Accept` 和一个不含凭证的 `User-Agent`。只请求上面两条规则里的 URL。

侧栏标题随语言切换：英文 Star history，简体中文「Star 历史」。页面正文在 `zh` / `zh-CN` 下用中文，其他语言用英文。

## 目录

```text
manifest.json
ui/stars/index.html    入口，bridge 相对路径是 ../../_axonhub/bridge.js
ui/stars/app.js        页面逻辑，只通过 axonhub.fetch 取数
ui/stars/format.js     文案、数字格式和图地址
ui/stars/style.css
```

HTML、CSS、JS 都使用相对路径。不要写 `/assets/...` 这种站点根路径，否则会丢掉插件资源 URL 上的 token。`_axonhub/bridge.js` 由宿主提供，不要放进包里。

## 打包

在仓库根目录执行：

```bash
go run -C cmd/pack-plugin . ../../plugins/examples/star-history
```

脚本打包 `manifest.json` 和 `ui/`。这个例子没有后端脚本，所以包里只有页面文件。根目录的 `README.md` 不会进包。默认产物是 `plugins/examples/star-history.axplugin`，这个文件已被 Git 忽略。也可以用 `-o` 指定输出路径。

## 安装

1. 用 Owner 打开侧栏「插件」›「插件管理」（`/plugins`），上传 `star-history.axplugin`。
2. 在授权对话框里勾选两条 network 规则，然后启用。两条都要授予：`https://api.github.com/repos/looplj/axonhub` 和 `https://api.star-history.com/chart`。缺一条时，对应请求会被宿主拒绝，浏览器仍然不会自己去连这些地址。
3. 打开 `/plugins/example.star-history/pages/stars`。侧栏「插件」分组会出现 Star history。
