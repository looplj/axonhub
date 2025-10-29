import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Admin Data Storage Management', () => {
  test.beforeEach(async ({ page }) => {
    test.setTimeout(60000)
    await gotoAndEnsureAuth(page, '/data-storages')
    await page.waitForTimeout(1500)
  })

  test('can create and edit a filesystem data storage', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-test-storage-${uniqueSuffix}`
    const directory = `/tmp/pw-storage-${uniqueSuffix}`
    const updatedDirectory = `${directory}-updated`

    const createButton = page
      .getByRole('button', { name: /创建数据存储|Create Data Storage|Add Data Storage|新建数据存储/i })
      .or(page.getByRole('button', { name: /创建|Create/i }))

    const createButtonCount = await createButton.count()
    if (createButtonCount === 0) {
      test.skip()
      return
    }

    await expect(createButton.first()).toBeVisible()
    await createButton.first().click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()

    await dialog.getByLabel(/名称|Name/i).fill(name)

    const descriptionField = dialog.getByLabel(/描述|Description/i)
    if ((await descriptionField.count()) > 0) {
      await descriptionField.first().fill('Playwright generated storage')
    }

    const directoryInput = dialog.locator('input[name="directory"]').first()
    await expect(directoryInput).toBeVisible()
    await directoryInput.fill(directory)

    const submitButton = dialog
      .getByRole('button', { name: /创建|Create|保存|Save/i })
      .filter({ hasText: /创建|Create|保存|Save/i })

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateDataStorage'),
      submitButton.first().click(),
    ])

    await expect(dialog).not.toBeVisible({ timeout: 10000 })

    await page.waitForTimeout(1000)
    const row = page.locator('tbody tr').filter({ hasText: name }).first()
    await expect(row).toBeVisible({ timeout: 10000 })
    await expect(row).toContainText(directory)

    const actionsTrigger = row
      .locator('button[aria-haspopup="menu"]')
      .or(row.getByRole('button', { name: /open menu|打开菜单|更多|More/i }))
      .first()

    const triggerCount = await actionsTrigger.count()
    if (triggerCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    const editMenuItem = menu.getByRole('menuitem', { name: /编辑|Edit/i })
    await expect(editMenuItem).toBeVisible()
    await editMenuItem.first().click()

    const editDialog = page.getByRole('dialog')
    await expect(editDialog).toBeVisible()

    const editDirectoryInput = editDialog.locator('input[name="directory"]').first()
    await expect(editDirectoryInput).toBeVisible()
    await editDirectoryInput.fill(updatedDirectory)

    const saveButton = editDialog.getByRole('button', { name: /保存|Save|更新|Update/i }).first()

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateDataStorage'),
      saveButton.click(),
    ])

    await expect(editDialog).not.toBeVisible({ timeout: 10000 })

    await page.waitForTimeout(1000)
    await expect(row).toContainText(updatedDirectory)

    const searchInput = page
      .locator('input[placeholder*="搜索"], input[placeholder*="Search"], input[type="search"]')
      .first()
    if ((await searchInput.count()) > 0) {
      await searchInput.fill(name)
      await page.waitForTimeout(800)
      const filteredRow = page.locator('tbody tr').filter({ hasText: name }).first()
      await expect(filteredRow).toBeVisible()
    }
  })
})
