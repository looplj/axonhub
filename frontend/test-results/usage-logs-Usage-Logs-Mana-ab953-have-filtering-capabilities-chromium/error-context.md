# Page snapshot

```yaml
- img
- navigation:
  - img
  - heading "AxonHub" [level=1]
  - combobox:
    - option "中文" [selected]
    - option "English"
- text: Sign in to AxonHub Enter your email and password to access your account Email
- textbox "Email"
- text: Password
- link "Forgot password?":
  - /url: /forgot-password
- textbox "Password"
- button:
  - img
- checkbox "Remember me"
- text: Remember me
- button "Sign in"
- region "Notifications alt+T"
- button "Open Tanstack query devtools":
  - img
- contentinfo:
  - button "Open TanStack Router Devtools":
    - img
    - img
    - text: "- TanStack Router"
- text: "[plugin:vite:import-analysis] Failed to resolve import \"../../users/data\" from \"src/features/usage-logs/components/data-table-toolbar.tsx\". Does the file exist? /Users/September_1/Projects/Freedom/axonhub/frontend/src/features/usage-logs/components/data-table-toolbar.tsx:13:25 28 | import { DataTableViewOptions } from './data-table-view-options'; 29 | import { useUsageLogPermissions } from '../../../gql/useUsageLogPermissions'; 30 | import { useUsers } from '../../users/data'; | ^ 31 | import { useChannels } from '../../channels/data'; 32 | export function DataTableToolbar({ table }) { at TransformPluginContext._formatLog (file:///Users/September_1/Projects/Freedom/axonhub/frontend/node_modules/.pnpm/vite@7.0.0_@types+node@24.0.4_jiti@2.4.2_lightningcss@1.30.1_tsx@4.20.3_yaml@2.8.0/node_modules/vite/dist/node/chunks/dep-Bsx9IwL8.js:31446:43) at TransformPluginContext.error (file:///Users/September_1/Projects/Freedom/axonhub/frontend/node_modules/.pnpm/vite@7.0.0_@types+node@24.0.4_jiti@2.4.2_lightningcss@1.30.1_tsx@4.20.3_yaml@2.8.0/node_modules/vite/dist/node/chunks/dep-Bsx9IwL8.js:31443:14) at normalizeUrl (file:///Users/September_1/Projects/Freedom/axonhub/frontend/node_modules/.pnpm/vite@7.0.0_@types+node@24.0.4_jiti@2.4.2_lightningcss@1.30.1_tsx@4.20.3_yaml@2.8.0/node_modules/vite/dist/node/chunks/dep-Bsx9IwL8.js:29992:18) at async file:///Users/September_1/Projects/Freedom/axonhub/frontend/node_modules/.pnpm/vite@7.0.0_@types+node@24.0.4_jiti@2.4.2_lightningcss@1.30.1_tsx@4.20.3_yaml@2.8.0/node_modules/vite/dist/node/chunks/dep-Bsx9IwL8.js:30050:32 at async Promise.all (index 11) at async TransformPluginContext.transform (file:///Users/September_1/Projects/Freedom/axonhub/frontend/node_modules/.pnpm/vite@7.0.0_@types+node@24.0.4_jiti@2.4.2_lightningcss@1.30.1_tsx@4.20.3_yaml@2.8.0/node_modules/vite/dist/node/chunks/dep-Bsx9IwL8.js:30018:4) at async EnvironmentPluginContainer.transform (file:///Users/September_1/Projects/Freedom/axonhub/frontend/node_modules/.pnpm/vite@7.0.0_@types+node@24.0.4_jiti@2.4.2_lightningcss@1.30.1_tsx@4.20.3_yaml@2.8.0/node_modules/vite/dist/node/chunks/dep-Bsx9IwL8.js:31260:14) at async loadAndTransform (file:///Users/September_1/Projects/Freedom/axonhub/frontend/node_modules/.pnpm/vite@7.0.0_@types+node@24.0.4_jiti@2.4.2_lightningcss@1.30.1_tsx@4.20.3_yaml@2.8.0/node_modules/vite/dist/node/chunks/dep-Bsx9IwL8.js:26434:26) at async viteTransformMiddleware (file:///Users/September_1/Projects/Freedom/axonhub/frontend/node_modules/.pnpm/vite@7.0.0_@types+node@24.0.4_jiti@2.4.2_lightningcss@1.30.1_tsx@4.20.3_yaml@2.8.0/node_modules/vite/dist/node/chunks/dep-Bsx9IwL8.js:27519:20) Click outside, press Esc key, or fix the code to dismiss. You can also disable this overlay by setting"
- code: server.hmr.overlay
- text: to
- code: "false"
- text: in
- code: vite.config.ts
- text: .
```