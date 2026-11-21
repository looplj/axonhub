// 职责: 验证 Claude Code 渠道手动获取目录时的来源提示和模型清理行为。
// 关联文件:
//   frontend/src/features/channels/components/channels-action-dialog.tsx
//   frontend/src/features/channels/data/channels.ts
//   frontend/tests/auth.utils.ts
// 注意事项:
//   测试使用模拟的 OAuth 授权结果和已授权目录，未授权请求走本地测试后端。
import { expect, test } from '@playwright/test'
import { gotoAndEnsureAuth } from './auth.utils'

test('Claude Code catalog protects preset and manual models until explicit cleanup', async ({ page }) => {
  const credential = JSON.stringify({ access_token: 'fixture-account' })
  const fetchInputs: Array<{ apiKey?: string }> = []
  const pendingPreset = Promise.withResolvers<void>()
  const releasePreset = Promise.withResolvers<void>()
  await page.route('**/admin/claudecode/oauth/start', (route) =>
    route.fulfill({ json: { session_id: 'fixture', auth_url: 'https://example.invalid/authorize' } })
  )
  await page.route('**/admin/claudecode/oauth/exchange', (route) =>
    route.fulfill({ json: { credentials: credential } })
  )
  await page.route('**/admin/graphql', async (route) => {
    const request = route.request().postDataJSON()
    if (request.operationName !== 'FetchModels') return route.continue()
    fetchInputs.push(request.variables.input)
    if (!request.variables.input.apiKey) {
      if (fetchInputs.length === 3) {
        pendingPreset.resolve()
        await releasePreset.promise
      }
      return route.continue()
    }
    await route.fulfill({
      json: {
        data: {
          fetchModels: {
            models: [{ id: 'fixture-new', pricing: null }],
            fallback: false,
            fallbackReason: null,
            error: null,
            codexCatalogVersion: null,
            codexCatalogSkippedReason: null,
          },
        },
      },
    })
  })

  const skipTour = page.getByRole('button', { name: 'Skip Tour' })
  await page.addLocatorHandler(skipTour, () => skipTour.click())
  await gotoAndEnsureAuth(page, '/channels')
  await page.getByTestId('add-channel-button').click()
  const dialog = page.getByRole('dialog')
  await dialog.getByTestId('provider-claudecode').click()
  const fetch = dialog.getByTestId('fetch-channel-models')
  const clean = dialog.getByTestId('clean-fetched-models')
  const source = dialog.getByTestId('claude-catalog-source')
  const manual = 'fixture-manual'
  await dialog.getByPlaceholder('Model ID', { exact: true }).fill(manual)
  await dialog.getByPlaceholder('Model ID', { exact: true }).press('Enter')
  expect(fetchInputs).toHaveLength(0)
  await fetch.click()
  await expect(source).toHaveText('Preset list · Authorize first')
  await expect(clean).toBeDisabled()

  // Add one preset, then check that fetching presets again cannot remove it.
  const preset = dialog.getByTestId('catalog-model-row').first()
  const presetID = await preset.getByTestId('catalog-model-id').innerText()
  await preset.getByRole('checkbox').click()
  await dialog.getByRole('button', { name: 'Add Selected (1)', exact: true }).click()
  await fetch.click()
  await expect(preset.getByRole('checkbox')).toBeDisabled()
  await expect(clean).toBeDisabled()

  // A response started before authorization must not reopen the old account's list.
  await fetch.click()
  await pendingPreset.promise
  await dialog.getByRole('button', { name: 'Start OAuth' }).click()
  await dialog.getByPlaceholder('Paste the full callback URL here').fill('https://example.invalid/fixture')
  await dialog.getByRole('button', { name: 'Exchange & Fill API Key' }).click()
  await expect(source).toHaveCount(0)
  expect(fetchInputs).toHaveLength(3)
  releasePreset.resolve()
  await expect(fetch).toBeEnabled()
  await expect(source).toHaveCount(0)
  await fetch.click()
  await expect(source).toHaveText('Upstream models (1)')
  expect(fetchInputs.map((input) => input.apiKey)).toEqual([undefined, undefined, undefined, credential])
  await expect(clean).toHaveText('Clean (1)')
  await expect(dialog.getByText(presetID, { exact: true }).first()).toBeVisible()
  await clean.click()
  await expect(clean).toBeDisabled()
  await expect(dialog.getByText(manual, { exact: true }).first()).toBeVisible()
  await expect(dialog.getByText(presetID, { exact: true })).toHaveCount(0)
})
