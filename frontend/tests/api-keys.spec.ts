import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Admin API Keys Management', () => {
  test.beforeEach(async ({ page }) => {
    await gotoAndEnsureAuth(page, '/project/api-keys')
  })

  test('can create, disable, enable an API key', async ({ page }) => {
    const uniqueName = `pw-test-apikey-${Date.now().toString().slice(-6)}`

    const addApiKeyButton = page.getByRole('button', { name: /创建 API Key|Create API Key|新建/i })
    await expect(addApiKeyButton).toBeVisible()
    await addApiKeyButton.click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()

    await dialog.getByLabel(/名称|Name/i).fill(uniqueName)

    const userSelect = dialog.locator('[data-testid="user-select"], [role="combobox"]').first()
    if (await userSelect.isVisible()) {
      await userSelect.click()
      const firstOption = page.locator('[role="option"]:not([aria-disabled="true"])').first()
      if (await firstOption.isVisible()) {
        await firstOption.click()
      }
    }

    // Click create button and wait for response
    const createButton = dialog.getByRole('button', { name: /创建|Create|保存|Save/i })
    await createButton.click()
    
    // Wait for dialog to close or success indication
    await expect(dialog).not.toBeVisible({ timeout: 10000 })
    
    // Handle the "查看 API 密钥" (View API Key) dialog that appears after creation
    // Be robust to localization and aria naming differences
    const viewDialog = page.locator('[role="dialog"]').filter({ hasText: /查看 API 密钥|View API Key|API Key/i })
    if (await viewDialog.count()) {
      // Prefer a close/ok button by name, fallback to the last button
      const namedClose = viewDialog.getByRole('button', { name: /Close|关闭|确定|OK|Done/i })
      if (await namedClose.count()) {
        await namedClose.click()
      } else {
        await viewDialog.locator('button').last().click()
      }
      await expect(viewDialog).not.toBeVisible({ timeout: 10000 })
    }

    const table = page.locator('[data-testid="api-keys-table"], table:has(th), table').first()
    const row = table.locator('tbody tr').filter({ hasText: uniqueName })
    await expect(row).toBeVisible()

    const actionsTrigger = row.locator('[data-testid="row-actions"], button:has(svg), .dropdown-trigger, button:has-text("Open menu")').first()

    // Disable API key
    await actionsTrigger.click()
    const disableItem = page.getByRole('menuitem', { name: /禁用|Disable/i })
    await expect(disableItem).toBeVisible({ timeout: 5000 })
    await disableItem.click()
    const statusDialog = page.getByRole('dialog')
    await expect(statusDialog).toContainText(uniqueName)
    const confirmButton = statusDialog.getByRole('button', { name: /确认|Confirm|保存|Save/i })
    await confirmButton.click()
    await expect(statusDialog).not.toBeVisible({ timeout: 10000 })
    await expect(row).toContainText(/禁用|Disabled/i)

    // Enable API key again
    await actionsTrigger.click()
    const enableItem = page.getByRole('menuitem', { name: /启用|Enable/i })
    await expect(enableItem).toBeVisible({ timeout: 5000 })
    await enableItem.click()
    const enableDialog = page.getByRole('dialog')
    await expect(enableDialog).toContainText(uniqueName)
    const enableConfirmButton = enableDialog.getByRole('button', { name: /确认|Confirm|保存|Save/i })
    await enableConfirmButton.click()
    await expect(enableDialog).not.toBeVisible({ timeout: 10000 })
    await expect(row).toContainText(/启用|Enabled/i)
  })
})
