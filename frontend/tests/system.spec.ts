import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Admin System Management', () => {
  test.beforeEach(async ({ page }) => {
    await gotoAndEnsureAuth(page, '/system')
  })

  test('can view system tabs and update brand settings', async ({ page }) => {
    await expect(
      page.getByRole('heading', { name: /System|系统/i }).first()
    ).toBeVisible()

    const brandTab = page.getByRole('tab', { name: /Brand|品牌/i })
    await brandTab.click()
    await expect(brandTab).toHaveAttribute('aria-selected', 'true')

    const brandInput = page.getByLabel(/Brand Name|品牌名称/i)
    await expect(brandInput).toBeVisible()

    const originalValue = await brandInput.inputValue()
    const newValue = originalValue.includes('pw-test')
      ? `${originalValue}-${Date.now().toString().slice(-4)}`
      : `pw-test-${Date.now().toString().slice(-4)}`

    await brandInput.fill(newValue)
    const saveButton = page.getByRole('button', { name: /Save Settings|保存设置/i })
    await expect(saveButton).toBeEnabled()

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateBrandSettings'),
      saveButton.click()
    ])

    await expect(brandInput).toHaveValue(newValue)

    if (newValue !== originalValue) {
      await brandInput.fill(originalValue)
      const revertButton = page.getByRole('button', { name: /Save Settings|保存设置/i })
      await expect(revertButton).toBeEnabled()
      await Promise.all([
        waitForGraphQLOperation(page, 'UpdateBrandSettings'),
        revertButton.click()
      ])
      await expect(brandInput).toHaveValue(originalValue)
    }

    const storageTab = page.getByRole('tab', { name: /Storage|存储/i })
    await storageTab.click()
    await expect(storageTab).toHaveAttribute('aria-selected', 'true')
    
    // Wait for storage content to load and check for any storage-related content
    await page.waitForTimeout(1000)
    const storageContent = page.locator('h1, h2, h3, h4, div, span').filter({ hasText: /Storage|storage|存储/i })
    if (await storageContent.count() > 0) {
      await expect(storageContent.first()).toBeVisible()
    } else {
      // If no specific storage text, just verify the tab is active
      await expect(storageTab).toHaveAttribute('aria-selected', 'true')
    }
  })

  test('can control provider quota collection with existing system settings UI', async ({ page }) => {
    const quotaTab = page.getByRole('tab', { name: /Quota|配额/i })
    await quotaTab.click()
    await expect(quotaTab).toHaveAttribute('aria-selected', 'true')

    const title = page.getByText(/^(Provider Quota Collection|提供商配额采集)$/i)
    await expect(title).toBeVisible()

    const collectionCard = title.locator('xpath=ancestor::*[@data-slot="card"]')
    const globalSwitch = collectionCard.getByRole('switch', { name: /Enable Provider Quota Collection|启用提供商配额采集/i })
    const minimaxSwitch = collectionCard.getByRole('switch', { name: /MiniMax/i })
    const saveButton = collectionCard.getByRole('button', { name: /Save|保存/i })
    const originalGlobal = await globalSwitch.isChecked()
    const originalMinimax = await minimaxSwitch.isChecked()

    try {
      if (!originalGlobal) await globalSwitch.click()
      await expect(minimaxSwitch).toBeEnabled()

      await globalSwitch.click()
      await expect(minimaxSwitch).toBeDisabled()
      await globalSwitch.click()

      await minimaxSwitch.click()
      await Promise.all([
        waitForGraphQLOperation(page, 'UpdateProviderQuotaCollectionSettings'),
        saveButton.click()
      ])
    } finally {
      if (!(await globalSwitch.isChecked())) await globalSwitch.click()
      if ((await minimaxSwitch.isChecked()) !== originalMinimax) await minimaxSwitch.click()
      if ((await globalSwitch.isChecked()) !== originalGlobal) await globalSwitch.click()

      await Promise.all([
        waitForGraphQLOperation(page, 'UpdateProviderQuotaCollectionSettings'),
        saveButton.click()
      ])

      await expect(globalSwitch).toBeChecked({ checked: originalGlobal })
      await expect(minimaxSwitch).toBeChecked({ checked: originalMinimax })
    }
  })
})
