# 每日用量卡

移植自 [magpie 的 day-card](https://github.com/magpie-community/plugins/tree/main/packages/day-card)。页面仍是一张「每日 AI 账单」小票：纸色、墨色、撕边、等宽字体、背景图案和四张风景都沿用原来的绘制。按钮尺寸和背景菜单也按原页面排，只是颜色改接 AxonHub 的主题变量。小票本身不随明暗主题变色。副标题是「经 AxonHub 路由」。移植说明写在这里和源码注释里，不印在小票上。

和 magpie 的差别是 Agent 用 API Key。一行 Key，下面是这把 Key 调用过的模型。没有挂到 API Key 上的用量，差额合成一行「未关联 API KEY」。用量分析不返回厂商图标，行首是一个墨点。

## 数据都在服务端加总

页面不拉取 usage log，也不自己把 token 和费用加起来。AxonHub 的用量分析已经按维度 `GROUP BY`：

- `analyticsOverview`：这一天的请求数、输入、缓存、输出和费用
- `analyticsDailyStats`：最近 400 个日历日，用来算连续天数、对比日均，以及日期菜单
- `analyticsDimensionStats(dimension: "apiKey")`：每把 API Key 一行汇总
- 卡片最多画出 8 把 Key。每一把再查一次 `analyticsDimensionStats(dimension: "model")`，过滤条件是这把 Key 的 `apiKeyIDs`

一次分析查询只有一个维度，没有 Key × 模型的交叉表，所以模型是按 Key 再聚合一次。返回的仍是汇总行。分析接口给出的 Key id 是整数，过滤时改成 `gid://axonhub/APIKey/<id>`。

日期边界用系统设置里的时区，和用量分析页是同一套日历。费用已经是系统货币，不再做汇率换算。Prompt token 里含有缓存部分，页面把它拆开，避免小票上的总量把缓存算两次。

`read_requests` 只用来取两个计数：失败请求的 `totalCount`，以及没有价格的 usage log 的 `totalCount`。查询不取 `edges`。没有这项权限时，小票不画错误行。

## 权限

- `read_dashboard`：用量分析。没有它，小票没有数字
- `read_settings`：系统时区和货币。没有它，日期改用浏览器时区，费用按 USD 显示
- `read_requests`：失败次数，以及没有价格的次数

背景选择写在本插件的 KV（`background`）。保存需要当前用户有 `write_plugins`。没有这项用户权限时，选择只在这次打开里有效。

## 打包

按 [插件目录说明](../../README.md) 安装打包器，然后在 AxonHub 仓库根目录运行：

```sh
pack-plugin plugins/community/day-card -o /tmp/day-card.axplugin
```

在 AxonHub 的 PR 中递增 `manifest.json` 版本。合并到 `unstable` 后，AxonHub 向 [axonhub-plugins](https://github.com/looplj/axonhub-plugins) 发送事件，由那边检出这个提交并打包发布。

## 安装

用 Owner 账号在「插件」页上传 `.axplugin`，授予上面的权限后启用。侧栏入口是「每日用量卡」，地址是 `/plugins/local.day-card/pages/day`。

复制图片走剪贴板，保存 PNG 走浏览器下载。沙箱如果拦住这两项，页面会提示分享失败，小票本身仍然画在页面上。

## 测试

```bash
node --test day-card.test.mjs
```
