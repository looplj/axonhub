import { test, expect, type APIRequestContext } from '@playwright/test'
import {
  chooseChannel,
  createChannel,
  createUser,
  expectNoLeak,
  graphql,
  openCodexTab,
  openCodexTabAs,
  operationResponse,
  saveCodexSettings,
  saveSettings,
  startFixture,
  stopFixture,
  suffix,
} from './codex-compat.utils'

const names = {
  disabledRelay: `pw-codex-disabled-relay-${suffix}`,
  archived: `pw-codex-archived-${suffix}`,
  noModels: `pw-codex-no-models-${suffix}`,
  slow: `pw-codex-slow-${suffix}`,
  enabledOK: `pw-codex-enabled-ok-${suffix}`,
  retyped: `pw-codex-retyped-${suffix}`,
  notCodex: `pw-codex-not-codex-${suffix}`,
}

test.describe.configure({ mode: 'serial' })

test.describe('Codex compatibility settings', () => {
  const resetSettings = (request: APIRequestContext) => saveCodexSettings(request, { enabled: false, channelID: null })

  test.beforeAll(async ({ request }) => {
    await startFixture(request)
    await createChannel(request, { name: names.disabledRelay, path: '/ok', status: 'disabled' })
    await createChannel(request, { name: names.archived, path: '/ok', status: 'archived' })
    await createChannel(request, { name: names.noModels, path: '/missing', status: 'enabled' })
    await createChannel(request, { name: names.slow, path: '/slow', status: 'disabled' })
    await createChannel(request, { name: names.enabledOK, path: '/ok', status: 'enabled' })
    await createChannel(request, { name: names.notCodex, path: '/ok', status: 'disabled', type: 'openai' })
  })

  test.afterAll(async ({ request }) => {
    await stopFixture(request)
  })

  test('lists every Codex channel with its status and nothing else, without any URL or key input', async ({ page }) => {
    const watched = await openCodexTab(page)

    await page.getByTestId('codex-compat-channel-select').click()
    for (const name of [names.disabledRelay, names.archived, names.noModels, names.slow, names.enabledOK]) {
      await expect(page.getByRole('option', { name: new RegExp(name) })).toBeVisible()
    }
    await expect(page.getByRole('option', { name: new RegExp(`${names.disabledRelay}.*(Disabled|禁用)`) })).toBeVisible()
    await expect(page.getByRole('option', { name: new RegExp(`${names.archived}.*(Archived|已归档)`) })).toBeVisible()
    await expect(page.getByRole('option', { name: new RegExp(names.notCodex) })).toHaveCount(0)
    for (const option of await page.getByRole('option').all()) await expect(option).not.toHaveAttribute('aria-disabled', 'true')
    await page.keyboard.press('Escape')

    await expect(page.getByTestId('codex-compat-card').getByRole('textbox')).toHaveCount(0)
    await expectNoLeak(page, watched)
  })

  test('tests a disabled relay with the global switch off, saves it, and restores it after reload', async ({ page }) => {
    const watched = await openCodexTab(page)
    const enabledSwitch = page.getByTestId('codex-compat-enabled-switch')
    const save = page.getByTestId('codex-compat-save-button')
    await expect(enabledSwitch).not.toBeChecked()
    await expect(page.getByTestId('codex-compat-test-button')).toBeDisabled()
    await expect(save).toBeDisabled()

    await chooseChannel(page, names.disabledRelay)
    const tested = operationResponse(page, 'TestCodexCatalog')
    await page.getByTestId('codex-compat-test-button').click()
    await tested
    await expect(page.getByTestId('codex-compat-test-success')).toContainText('3')

    await saveSettings(page)
    await expect(save).toBeDisabled()

    await page.reload()
    await expect(page.getByTestId('codex-compat-channel-select')).toContainText(names.disabledRelay)
    await expect(enabledSwitch).not.toBeChecked()
    const sent = watched.exchanges.filter((exchange) => exchange.operation === 'UpdateCodexCompatibilitySettings')
    expect(sent.map((exchange) => exchange.variables)).toEqual([{ input: { enabled: false, channelID: expect.any(Number) } }])
    await expectNoLeak(page, watched)
  })

  test('an endpoint without a model catalog shows a readable error yet still saves enabled', async ({ page }) => {
    const watched = await openCodexTab(page)
    await chooseChannel(page, names.noModels)
    await page.getByTestId('codex-compat-test-button').click()
    await expect(page.getByTestId('codex-compat-test-failure')).toContainText('404')
    await expect(page.getByTestId('codex-compat-test-success')).toHaveCount(0)

    await page.getByTestId('codex-compat-enabled-switch').click()
    await expect(page.getByTestId('codex-compat-save-button')).toBeEnabled()
    await saveSettings(page)

    await page.reload()
    await expect(page.getByTestId('codex-compat-enabled-switch')).toBeChecked()
    await expect(page.getByTestId('codex-compat-channel-select')).toContainText(names.noModels)
    await expectNoLeak(page, watched)
  })

  test('enabling without a channel is blocked until one is selected, and an archived channel can be saved', async ({ page, request }) => {
    await resetSettings(request)
    const watched = await openCodexTab(page)
    await page.getByTestId('codex-compat-enabled-switch').click()
    await expect(page.getByTestId('codex-compat-issue')).toBeVisible()
    await expect(page.getByTestId('codex-compat-save-button')).toBeDisabled()
    await chooseChannel(page, names.archived)
    await expect(page.getByTestId('codex-compat-issue')).toHaveCount(0)
    await expect(page.getByTestId('codex-compat-save-button')).toBeEnabled()
    await saveSettings(page)

    await page.reload()
    await expect(page.getByTestId('codex-compat-enabled-switch')).toBeChecked()
    await expect(page.getByTestId('codex-compat-channel-select')).toContainText(names.archived)
    await expectNoLeak(page, watched)
  })

  test('switching channels or cancelling never leaves a stale test result', async ({ page, request }) => {
    await resetSettings(request)
    const watched = await openCodexTab(page)
    const result = page.getByTestId('codex-compat-test-result')

    await chooseChannel(page, names.enabledOK)
    await page.getByTestId('codex-compat-test-button').click()
    await expect(page.getByTestId('codex-compat-test-success')).toBeVisible()
    await chooseChannel(page, names.archived)
    await expect(page.getByTestId('codex-compat-test-success')).toHaveCount(0)

    await chooseChannel(page, names.slow)
    await page.getByTestId('codex-compat-test-button').click()
    await expect(page.getByTestId('codex-compat-test-cancel')).toBeVisible()
    await chooseChannel(page, names.enabledOK)
    await expect(page.getByTestId('codex-compat-test-cancel')).toHaveCount(0)
    await page.waitForTimeout(3000)
    await expect(result).toHaveText('')

    await chooseChannel(page, names.slow)
    await page.getByTestId('codex-compat-test-button').click()
    await page.getByTestId('codex-compat-test-cancel').click()
    await page.waitForTimeout(3000)
    await expect(result).toHaveText('')
    await expect(page.getByTestId('codex-compat-test-button')).toBeEnabled()
    await expectNoLeak(page, watched)
  })

  test('a saved channel that stopped being a Codex channel is flagged and can be cleared', async ({ page, request }) => {
    const channel = await createChannel(request, { name: names.retyped, path: '/ok', status: 'disabled' })
    await saveCodexSettings(request, { enabled: true, channelID: channel.id })
    await graphql(request, `mutation ($id: ID!, $input: UpdateChannelInput!) { updateChannel(id: $id, input: $input) { id } }`, {
      id: channel.guid,
      input: { type: 'openai' },
    })

    const watched = await openCodexTab(page)
    const save = page.getByTestId('codex-compat-save-button')
    await expect(page.getByTestId('codex-compat-reference-missing')).toBeVisible()
    await expect(page.getByTestId('codex-compat-channel-select')).not.toContainText(names.retyped)
    await expect(page.getByTestId('codex-compat-enabled-switch')).toBeChecked()
    await expect(save).toBeDisabled()

    await page.getByTestId('codex-compat-enabled-switch').click()
    await expect(save).toBeEnabled()
    await saveSettings(page)
    await expect(page.getByTestId('codex-compat-reference-missing')).toHaveCount(0)
    const sent = watched.exchanges.filter((exchange) => exchange.operation === 'UpdateCodexCompatibilitySettings')
    expect(sent.map((exchange) => exchange.variables)).toEqual([{ input: { enabled: false, channelID: null } }])
    await expectNoLeak(page, watched)
  })

  test.describe('permissions', () => {
    const scopeSets = {
      readOnly: ['read_settings'],
      writeWithoutChannels: ['read_settings', 'write_settings'],
      channelsWithoutWrite: ['read_settings', 'read_channels'],
      full: ['read_settings', 'write_settings', 'read_channels'],
    }
    const emails = Object.fromEntries(Object.keys(scopeSets).map((key) => [key, `pw-codex-${key.toLowerCase()}-${suffix}@example.com`]))

    test.beforeAll(async ({ request }) => {
      for (const [key, scopes] of Object.entries(scopeSets)) await createUser(request, emails[key], scopes)
    })

    for (const key of ['readOnly', 'writeWithoutChannels', 'channelsWithoutWrite'] as const) {
      test(`a non-owner with ${key} scopes can neither edit nor test`, async ({ page }) => {
        const watched = await openCodexTabAs(page, emails[key])
        await expect(page.getByTestId('codex-compat-permission-notice')).toBeVisible()
        await expect(page.getByTestId('codex-compat-enabled-switch')).toBeDisabled()
        await expect(page.getByTestId('codex-compat-channel-select')).toBeDisabled()
        await expect(page.getByTestId('codex-compat-test-button')).toBeDisabled()
        await expect(page.getByTestId('codex-compat-save-button')).toBeDisabled()
        const listsChannels = watched.exchanges.some((exchange) => exchange.operation === 'CodexCatalogChannels')
        expect(listsChannels).toBe(key === 'channelsWithoutWrite')
      })
    }

    test('a non-owner with settings write and channel read can select, test, and save', async ({ page, request }) => {
      await resetSettings(request)
      const watched = await openCodexTabAs(page, emails.full)
      await expect(page.getByTestId('codex-compat-permission-notice')).toHaveCount(0)
      await chooseChannel(page, names.enabledOK)
      await page.getByTestId('codex-compat-test-button').click()
      await expect(page.getByTestId('codex-compat-test-success')).toBeVisible()
      await saveSettings(page)
      await expectNoLeak(page, watched)
    })
  })
})
