// 职责: 将标准测试入口接入既有 Node 测试套件，避免 Vite 应用插件影响测试运行。
// 关联文件:
//   frontend/tests/unit-suite.test.ts
//   frontend/package.json
// 注意事项: 既有 .test.mjs 使用 node:test，必须由 Node 执行，不能作为 Vitest 模块导入。
import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    include: ['tests/unit-suite.test.ts'],
    testTimeout: 120_000,
  },
})
