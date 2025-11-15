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

  test('profile duplicate name validation - real-time error display', async ({ page }) => {
    // First, create an API key to work with
    const uniqueName = `pw-test-profile-${Date.now().toString().slice(-6)}`
    
    const addApiKeyButton = page.getByRole('button', { name: /创建 API Key|Create API Key|新建/i })
    await addApiKeyButton.click()
    
    const createDialog = page.getByRole('dialog')
    await createDialog.getByLabel(/名称|Name/i).fill(uniqueName)
    
    const userSelect = createDialog.locator('[data-testid="user-select"], [role="combobox"]').first()
    if (await userSelect.isVisible()) {
      await userSelect.click()
      const firstOption = page.locator('[role="option"]:not([aria-disabled="true"])').first()
      if (await firstOption.isVisible()) {
        await firstOption.click()
      }
    }
    
    await createDialog.getByRole('button', { name: /创建|Create|保存|Save/i }).click()
    await expect(createDialog).not.toBeVisible({ timeout: 10000 })
    
    // Close the view API key dialog if it appears
    const viewDialog = page.locator('[role="dialog"]').filter({ hasText: /查看 API 密钥|View API Key|API Key/i })
    if (await viewDialog.count()) {
      const namedClose = viewDialog.getByRole('button', { name: /Close|关闭|确定|OK|Done/i })
      if (await namedClose.count()) {
        await namedClose.click()
      } else {
        await viewDialog.locator('button').last().click()
      }
    }
    
    // Find the API key row and open profiles dialog
    const table = page.locator('[data-testid="api-keys-table"], table:has(th), table').first()
    const row = table.locator('tbody tr').filter({ hasText: uniqueName })
    await expect(row).toBeVisible()
    
    const actionsTrigger = row.locator('td:last-child button, button:has-text("Open menu")').first()
    await actionsTrigger.click()
    
    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()
    await menu.getByRole('menuitem', { name: /配置|Profiles|Settings/i }).click()
    
    // Wait for profiles dialog to open
    const profilesDialog = page.getByRole('dialog').filter({ hasText: /配置|Profiles/i })
    await expect(profilesDialog).toBeVisible()

    const profileInputSelector = 'input[placeholder*="配置名称"], input[placeholder*="Profile"]'
    const addProfileButton = profilesDialog.getByRole('button', { name: /添加配置|Add Profile|新建/i })
    await expect(addProfileButton).toBeVisible()

    // Add first profile card (UI starts empty)
    await addProfileButton.click()
    const profileInputs = profilesDialog.locator(profileInputSelector)
    await expect(profileInputs).toHaveCount(1)

    // Rename the first profile to "production" to set the baseline duplicate
    const firstProfileInput = profileInputs.first()
    await expect(firstProfileInput).toBeVisible()
    await firstProfileInput.clear()
    await firstProfileInput.fill('production')

    // Add a second profile for duplicate validation
    await addProfileButton.click()
    await expect(profileInputs).toHaveCount(2)
    const secondProfileInput = profileInputs.nth(1)

    // Type a duplicate name "production" - should show error immediately
    await secondProfileInput.clear()
    await secondProfileInput.fill('production')

    // Wait a bit for the error to appear (should be immediate, but give it a small buffer)
    await page.waitForTimeout(600)

    // Check that error message is displayed for the duplicated profile
    const secondProfileError = secondProfileInput.locator('xpath=ancestor::*[@data-slot="form-item"]//p[@data-slot="form-message"]')
    await expect(secondProfileError).toBeVisible()
    await expect(secondProfileInput).toHaveAttribute('aria-invalid', 'true')

    // Check that Save button is disabled
    const saveButton = profilesDialog.getByRole('button', { name: /保存|Save/i })
    await expect(saveButton).toBeDisabled()

    // Change to a unique name
    await secondProfileInput.clear()
    await secondProfileInput.fill('production-main')

    // Wait for validation
    await page.waitForTimeout(600)

    // Error should disappear for the edited profile
    await expect(secondProfileInput).not.toHaveAttribute('aria-invalid', 'true')
    await expect(secondProfileError).not.toBeVisible()

    // Close dialog
    const cancelButton = profilesDialog.getByRole('button', { name: /取消|Cancel/i })
    await expect(cancelButton).toBeVisible()
    await cancelButton.focus()
    await page.keyboard.press('Escape')
    await expect(profilesDialog).not.toBeVisible()
  })

  test('profile duplicate name validation - case insensitive', async ({ page }) => {
    // Create an API key
    const uniqueName = `pw-test-case-${Date.now().toString().slice(-6)}`
    
    const addApiKeyButton = page.getByRole('button', { name: /创建 API Key|Create API Key|新建/i })
    await addApiKeyButton.click()
    
    const createDialog = page.getByRole('dialog')
    await createDialog.getByLabel(/名称|Name/i).fill(uniqueName)
    
    const userSelect = createDialog.locator('[data-testid="user-select"], [role="combobox"]').first()
    if (await userSelect.isVisible()) {
      await userSelect.click()
      const firstOption = page.locator('[role="option"]:not([aria-disabled="true"])').first()
      if (await firstOption.isVisible()) {
        await firstOption.click()
      }
    }
    
    await createDialog.getByRole('button', { name: /创建|Create|保存|Save/i }).click()
    await expect(createDialog).not.toBeVisible({ timeout: 10000 })
    
    // Close view dialog if present
    const viewDialog = page.locator('[role="dialog"]').filter({ hasText: /查看 API 密钥|View API Key|API Key/i })
    if (await viewDialog.count()) {
      const namedClose = viewDialog.getByRole('button', { name: /Close|关闭|确定|OK|Done/i })
      if (await namedClose.count()) {
        await namedClose.click()
      } else {
        await viewDialog.locator('button').last().click()
      }
    }
    
    // Open profiles dialog
    const table = page.locator('[data-testid="api-keys-table"], table:has(th), table').first()
    const row = table.locator('tbody tr').filter({ hasText: uniqueName })
    const actionsTrigger = row.locator('td:last-child button, button:has-text("Open menu")').first()
    await actionsTrigger.click()
    
    const menu = page.getByRole('menu')
    await menu.getByRole('menuitem', { name: /配置|Profiles|Settings/i }).click()
    
    const profilesDialog = page.getByRole('dialog').filter({ hasText: /配置|Profiles/i })
    await expect(profilesDialog).toBeVisible()
    
    const profileInputSelector = 'input[placeholder*="配置名称"], input[placeholder*="Profile"]'
    const addProfileButton = profilesDialog.getByRole('button', { name: /添加配置|Add Profile|新建/i })
    await expect(addProfileButton).toBeVisible()

    // Add first profile and set its name to "Default" for baseline comparison
    await addProfileButton.click()
    const profileInputs = profilesDialog.locator(profileInputSelector)
    const firstProfileInput = profileInputs.first()
    await expect(firstProfileInput).toBeVisible()
    await firstProfileInput.clear()
    await firstProfileInput.fill('Default')

    // Add the second profile that will trigger duplicate validation
    await addProfileButton.click()
    await expect(profileInputs).toHaveCount(2)
    const secondProfileInput = profileInputs.nth(1)
    
    // Type "default" (different case) - should still show error
    await secondProfileInput.clear()
    await secondProfileInput.fill('default')
    
    await page.waitForTimeout(600)

    // Should show duplicate error (case-insensitive)
    const duplicateErrorMessages = profilesDialog.locator('p[data-slot="form-message"]').filter({
      hasText: /配置名称不能重复|Profile names must be unique|duplicate/i,
    })
    await expect(duplicateErrorMessages.first()).toBeVisible()
    await expect(secondProfileInput).toHaveAttribute('aria-invalid', 'true')
    
    // Save button should be disabled
    const saveButton = profilesDialog.getByRole('button', { name: /保存|Save/i })
    await expect(saveButton).toBeDisabled()
    
    // Try with whitespace: " default "
    await secondProfileInput.clear()
    await secondProfileInput.fill(' default ')
    
    await page.waitForTimeout(600)

    // Should still show error (whitespace trimmed)
    await expect(duplicateErrorMessages.first()).toBeVisible()
    await expect(saveButton).toBeDisabled()
    
    // Close dialog
    const cancelButton = profilesDialog.getByRole('button', { name: /取消|Cancel/i })
    await expect(cancelButton).toBeVisible()
    await cancelButton.focus()
    await page.keyboard.press('Escape')
    await expect(profilesDialog).not.toBeVisible()
  })
})
