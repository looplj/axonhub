import { test, expect, type Page } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

async function createFilesystemDataStorage(page: Page) {
  const uniqueSuffix = Date.now().toString().slice(-6)
  const name = `pw-test-storage-${uniqueSuffix}`
  const directory = `/tmp/pw-storage-${uniqueSuffix}`

  const createButton = page
    .getByRole('button', { name: /创建数据存储|Create Data Storage|Add Data Storage|新建数据存储/i })
    .or(page.getByRole('button', { name: /创建|Create/i }))

  const createButtonCount = await createButton.count()
  if (createButtonCount === 0) {
    test.skip()
    return null
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

  return { name, directory }
}

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

  test('can create and edit an S3 data storage', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-test-s3-${uniqueSuffix}`
    const bucketName = `pw-s3-bucket-${uniqueSuffix}`
    const updatedBucketName = `${bucketName}-updated`
    const accessKey = `AKIA${uniqueSuffix}`
    const secretKey = `secret-${uniqueSuffix}`

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
      await descriptionField.first().fill('Playwright generated S3 storage')
    }

    const typeSelector = dialog.locator('#create-type')
    if ((await typeSelector.count()) > 0) {
      await typeSelector.first().click()
      const s3Option = page
        .getByRole('option', { name: /S3/i })
        .or(page.locator('[data-value="s3"]'))
      await expect(s3Option.first()).toBeVisible()
      await s3Option.first().click()
    }

    const bucketInput = dialog.locator('input[name="s3BucketName"]').first()
    await expect(bucketInput).toBeVisible()
    await bucketInput.fill(bucketName)

    const endpointInput = dialog.locator('input[name="s3Endpoint"]').first()
    if ((await endpointInput.count()) > 0) {
      await endpointInput.fill('https://s3.amazonaws.com')
    }

    const regionInput = dialog.locator('input[name="s3Region"]').first()
    await expect(regionInput).toBeVisible()
    await regionInput.fill('us-east-1')

    const accessKeyInput = dialog.locator('input[name="s3AccessKey"]').first()
    await expect(accessKeyInput).toBeVisible()
    await accessKeyInput.fill(accessKey)

    const secretKeyInput = dialog.locator('input[name="s3SecretKey"]').first()
    await expect(secretKeyInput).toBeVisible()
    await secretKeyInput.fill(secretKey)

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
    await expect(row).toContainText(bucketName)

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

    const editBucketInput = editDialog.locator('input[name="s3BucketName"]').first()
    await expect(editBucketInput).toBeVisible()
    await editBucketInput.fill(updatedBucketName)

    const saveButton = editDialog.getByRole('button', { name: /保存|Save|更新|Update/i }).first()

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateDataStorage'),
      saveButton.click(),
    ])

    await expect(editDialog).not.toBeVisible({ timeout: 10000 })

    await page.waitForTimeout(1000)
    await expect(row).toContainText(updatedBucketName)

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

  test('can create and edit a GCS data storage', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-test-gcs-${uniqueSuffix}`
    const bucketName = `pw-gcs-bucket-${uniqueSuffix}`
    const updatedBucketName = `${bucketName}-updated`
    const credential = JSON.stringify({
      type: 'service_account',
      project_id: `pw-${uniqueSuffix}`,
      private_key_id: uniqueSuffix,
    })

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
      await descriptionField.first().fill('Playwright generated GCS storage')
    }

    const typeSelector = dialog.locator('#create-type')
    if ((await typeSelector.count()) > 0) {
      await typeSelector.first().click()
      const gcsOption = page
        .getByRole('option', { name: /GCS|Google/i })
        .or(page.locator('[data-value="gcs"]'))
      await expect(gcsOption.first()).toBeVisible()
      await gcsOption.first().click()
    }

    const bucketInput = dialog.locator('input[name="gcsBucketName"]').first()
    await expect(bucketInput).toBeVisible()
    await bucketInput.fill(bucketName)

    const credentialTextarea = dialog.locator('textarea[name="gcsCredential"]').first()
    await expect(credentialTextarea).toBeVisible()
    await credentialTextarea.fill(credential)

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
    await expect(row).toContainText(bucketName)

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

    const editBucketInput = editDialog.locator('input[name="gcsBucketName"]').first()
    await expect(editBucketInput).toBeVisible()
    await editBucketInput.fill(updatedBucketName)

    const saveButton = editDialog.getByRole('button', { name: /保存|Save|更新|Update/i }).first()

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateDataStorage'),
      saveButton.click(),
    ])

    await expect(editDialog).not.toBeVisible({ timeout: 10000 })

    await page.waitForTimeout(1000)
    await expect(row).toContainText(updatedBucketName)

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

  test('can archive a data storage from the actions menu', async ({ page }) => {
    const created = await createFilesystemDataStorage(page)
    if (!created) return
    const { name } = created

    const row = page.locator('tbody tr').filter({ hasText: name }).first()
    await expect(row).toBeVisible({ timeout: 10000 })

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

    const archiveMenuItem = menu.getByRole('menuitem', { name: /归档|Archive/i }).first()
    if ((await archiveMenuItem.count()) === 0) {
      test.skip()
      return
    }

    await archiveMenuItem.click()

    const archiveDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(archiveDialog).toBeVisible()

    const confirmButton = archiveDialog.getByRole('button', { name: /归档|Archive/i }).first()

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateDataStorageStatus'),
      confirmButton.click(),
    ])

    await expect(archiveDialog).not.toBeVisible({ timeout: 10000 })

    await page.waitForTimeout(1000)

    const searchInput = page
      .locator('input[placeholder*="搜索"], input[placeholder*="Search"], input[type="search"]')
      .first()
    if ((await searchInput.count()) > 0) {
      await searchInput.fill(name)
      await page.waitForTimeout(800)
    }

    await expect(page.locator('tbody tr').filter({ hasText: name })).toHaveCount(0)
  })

  test('can cancel archiving a data storage', async ({ page }) => {
    const created = await createFilesystemDataStorage(page)
    if (!created) return
    const { name } = created

    const row = page.locator('tbody tr').filter({ hasText: name }).first()
    await expect(row).toBeVisible({ timeout: 10000 })

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

    const archiveMenuItem = menu.getByRole('menuitem', { name: /归档|Archive/i }).first()
    if ((await archiveMenuItem.count()) === 0) {
      test.skip()
      return
    }

    await archiveMenuItem.click()

    const archiveDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(archiveDialog).toBeVisible()

    const cancelButton = archiveDialog.getByRole('button', { name: /取消|Cancel/i }).first()
    await cancelButton.click()

    await expect(archiveDialog).not.toBeVisible({ timeout: 10000 })

    await page.waitForTimeout(500)

    await expect(row).toBeVisible({ timeout: 10000 })
  })
})
