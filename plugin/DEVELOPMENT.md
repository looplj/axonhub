# 插件开发

插件页面跑在沙箱 iframe 里，样式表由插件自己携带。宿主只把已经算好的主题变量写进页面，不提供组件，也不把控制台的 CSS 文件交给插件。

## 页面主题

握手成功时，以及用户切换明暗或配色时，`bridge.js` 把父页面 `<html>` 上的计算值写到插件页面的 `<html>`。同时按当前明暗设置 `color-scheme`。页面用同名变量引用：

```css
body {
  background: var(--background, #f4f5f7);
  color: var(--foreground, #1c1f24);
  font-family: var(--font-sans, ui-sans-serif, system-ui, sans-serif);
}

code, .mono {
  font-family: var(--font-mono, ui-monospace, monospace);
}

.card {
  background: var(--card, #ffffff);
  color: var(--card-foreground, inherit);
  border: 1px solid var(--border, #e3e6eb);
  border-radius: var(--radius, 0.75rem);
  box-shadow: var(--shadow-sm);
}

button {
  background: var(--primary, #1c1f24);
  color: var(--primary-foreground, #ffffff);
  border-radius: var(--radius, 0.75rem);
}

button.secondary {
  background: var(--secondary, transparent);
  color: var(--secondary-foreground, inherit);
}

input, textarea {
  border: 1px solid var(--input, #e3e6eb);
  border-radius: var(--radius, 0.75rem);
}

input:focus-visible {
  outline: 2px solid var(--ring, transparent);
}

.popover {
  background: var(--popover, #ffffff);
  color: var(--popover-foreground, inherit);
  box-shadow: var(--shadow-md);
}
```

`var()` 的第二个参数只在握手完成前，或当前配色没有这个变量时生效。宿主写上变量之后，回退色不再使用。

`--muted` 是浅色表面，次要文字用 `--muted-foreground`。`--accent` 也是表面色，不是按钮主色；按钮主色是 `--primary`。

### 会传递的变量

| 用途 | 变量 |
|------|------|
| 页面底色和正文 | `--background`、`--foreground` |
| 卡片 | `--card`、`--card-foreground` |
| 弹出层 | `--popover`、`--popover-foreground` |
| 主按钮 | `--primary`、`--primary-foreground` |
| 次按钮 | `--secondary`、`--secondary-foreground` |
| 弱化表面和次要文字 | `--muted`、`--muted-foreground` |
| 高亮表面 | `--accent`、`--accent-foreground` |
| 危险操作 | `--destructive`、`--destructive-foreground` |
| 边框、输入框、焦点环 | `--border`、`--input`、`--ring` |
| 圆角 | `--radius` |
| 字体族 | `--font-sans`、`--font-mono` |
| 阴影 | `--shadow-2xs`、`--shadow-xs`、`--shadow-sm`、`--shadow`、`--shadow-md`、`--shadow-lg`、`--shadow-xl`、`--shadow-2xl` |

字体只传家族名称，字体文件不会进入 iframe。系统没有对应字体时，页面应在 `font-family` 里保留通用族作为回退。

侧栏、图表、表格、字距和间距不传。这些变量跟着控制台内部布局走，不是插件页面的契约。

空值，以及含控制字符、`;`、`{}`、`<>`、`\` 的值会被丢掉。某次消息里没有的白名单变量会从插件页面上移除，避免上一个配色的值留下来。
