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

    // Locate the actions button (three dots menu) in the last cell of the row
    // Avoid clicking the Eye or Copy buttons in the key column
    const actionsTrigger = row.locator('td:last-child button, button:has-text("Open menu")').first()

    // Disable API key
    await actionsTrigger.click()
    const menu1 = page.getByRole('menu')
    await expect(menu1).toBeVisible()
    await menu1.getByRole('menuitem', { name: /禁用|Disable/i }).focus()
    await page.keyboard.press('Enter')
    const statusDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(statusDialog).toBeVisible()
    await expect(statusDialog).toContainText(uniqueName)
    // Click the confirm button - it's the second button (first is Cancel)
    const confirmButton = statusDialog.locator('button').last()
    await confirmButton.click()
    await expect(statusDialog).not.toBeVisible({ timeout: 10000 })
    await expect(row).toContainText(/禁用|Disabled/i)

    // Verify by menu toggle: now it should show Enable
    await actionsTrigger.click()
    const menu2 = page.getByRole('menu')
    await expect(menu2).toBeVisible()
    await expect(menu2.getByRole('menuitem', { name: /启用|Enable/i })).toBeVisible()
    
    // Close the menu before reopening
    await page.keyboard.press('Escape')
    await expect(menu2).not.toBeVisible()

    // Enable API key again
    await actionsTrigger.click()
    const menu3 = page.getByRole('menu')
    await expect(menu3).toBeVisible()
    await menu3.getByRole('menuitem', { name: /启用|Enable/i }).focus()
    await page.keyboard.press('Enter')
    const enableDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(enableDialog).toBeVisible()
    await expect(enableDialog).toContainText(uniqueName)
    // Click the confirm button - it's the second button (first is Cancel)
    const enableConfirmButton = enableDialog.locator('button').last()
    await enableConfirmButton.click()
    await expect(enableDialog).not.toBeVisible({ timeout: 10000 })
    await expect(row).toContainText(/启用|Enabled/i)
  })
})
