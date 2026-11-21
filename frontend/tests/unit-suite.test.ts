// 职责: 通过标准 Vitest 入口执行完整既有 Node 单元测试，并保留失败状态及输出。
// 关联文件:
//   frontend/vitest.config.ts
//   frontend/package.json
// 注意事项: 不导入 node:test 文件；子进程执行与 test:unit 相同的真实测试命令。
import { spawnSync } from 'node:child_process'
import { expect, test } from 'vitest'

test('existing Node unit suite passes', () => {
  const result = spawnSync(process.execPath, ['--test', 'src/**/*.test.mjs'], {
    cwd: new URL('..', import.meta.url),
    encoding: 'utf8',
    timeout: 110_000,
    maxBuffer: 16 * 1024 * 1024,
  })

  process.stdout.write(result.stdout ?? '')
  process.stderr.write(result.stderr ?? '')
  expect(result.error, 'Node test runner must start and finish').toBeUndefined()
  expect(result.signal, 'Node test runner must finish without a signal').toBeNull()
  expect(result.status, 'every existing Node test must pass').toBe(0)
})
