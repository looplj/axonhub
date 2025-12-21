import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Admin Models Management', () => {
  test.beforeEach(async ({ page }) => {
    test.setTimeout(60000)
    await gotoAndEnsureAuth(page, '/models')

    const modelsTable = page.getByTestId('models-table')
    await modelsTable.waitFor({ state: 'visible', timeout: 20000 })
  })

  test('can create, edit, filter, toggle status, and delete a model', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const baseName = `pw-model-${uniqueSuffix}`
    const updatedName = `${baseName}-updated`

    // Open create dialog
    const createButton = page
      .getByRole('button', { name: /Add Model|创建模型|新增模型|创建|Add/i })
      .first()
    await expect(createButton).toBeVisible()
    await createButton.click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()

    // Select developer
    const developerCombo = dialog.locator('[role="combobox"]').first()
    await developerCombo.click()
    const developerOption = page
      .getByRole('option', { name: /Moonshot AI|Moonshot/i })
      .or(page.getByRole('option', { name: /OpenAI/i }))
      .first()
    await developerOption.click()

    // Select a modelId from provider list
    const modelIdInput = dialog.getByPlaceholder(/model id/i).first()
    await modelIdInput.click()
    await modelIdInput.fill('kimi')
    const modelOption = page.getByRole('option', { name: /kimi-k2-thinking/i }).first()
    await expect(modelOption).toBeVisible()
    await modelOption.click()

    // Override default name/group with deterministic values
    const nameInput = dialog.getByLabel(/Name|名称/i)
    await nameInput.fill(baseName)
    const groupInput = dialog.getByLabel(/Group|分组/i)
    await groupInput.fill(`group-${uniqueSuffix}`)
    const remarkInput = dialog.getByLabel(/Remark|备注/i)
    if (await remarkInput.count()) {
      await remarkInput.fill('Created via Playwright E2E')
    }

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateModel'),
      dialog.getByRole('button', { name: /Create|创建|保存|Save/i }).last().click(),
    ])
    await expect(dialog).not.toBeVisible({ timeout: 20000 })
    await waitForGraphQLOperation(page, 'GetModels')

    const modelsTable = page.getByTestId('models-table')
    const createdRow = modelsTable.locator('tbody tr').filter({ hasText: baseName })
    await expect(createdRow).toBeVisible({ timeout: 20000 })

    // Edit the created model
    const rowActions = createdRow.getByTestId('row-actions').first()
    await rowActions.click()
    const editMenuItem = page.getByRole('menuitem', { name: /Edit|编辑/i }).first()
    await editMenuItem.click()

    const editDialog = page.getByRole('dialog').filter({ hasText: /Edit Model|编辑/i }).first()
    await expect(editDialog).toBeVisible()
    const editNameInput = editDialog.getByLabel(/Name|名称/i)
    await editNameInput.fill(updatedName)

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateModel'),
      editDialog.getByRole('button', { name: /Save|保存|Update|更新/i }).last().click(),
    ])
    await expect(editDialog).not.toBeVisible({ timeout: 20000 })
    await waitForGraphQLOperation(page, 'GetModels')

    const updatedRow = modelsTable.locator('tbody tr').filter({ hasText: updatedName })
    await expect(updatedRow).toBeVisible({ timeout: 20000 })

    // Verify filtering by name works with the updated name
    const filterInput = page.getByPlaceholder(/Filter by name|名称|搜索/i)
    await filterInput.fill(updatedName)
    await page.waitForTimeout(800)
    await expect(updatedRow).toBeVisible()
    await filterInput.fill('')
    await page.waitForTimeout(400)

    // Toggle status via switch (enable/disable)
    const statusSwitch = updatedRow.locator('[data-testid="model-status-switch"]').first()
    await statusSwitch.click()
    const statusDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(statusDialog).toBeVisible()
    const confirmStatusButton = statusDialog
      .getByRole('button', { name: /Confirm|确认|确定|Enable|Disable/i })
      .last()
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateModel'),
      confirmStatusButton.click(),
    ])
    await expect(statusDialog).not.toBeVisible({ timeout: 20000 })
    await waitForGraphQLOperation(page, 'GetModels')

    // Delete the created model
    const actionsAfterToggle = updatedRow.getByTestId('row-actions').first()
    await actionsAfterToggle.click()
    const deleteMenuItem = page.getByRole('menuitem', { name: /Delete|删除/i }).first()
    await deleteMenuItem.click()

    const deleteDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(deleteDialog).toBeVisible()
    const deleteButton = deleteDialog.getByRole('button', { name: /Delete|删除|Confirm|确认/i }).last()
    await Promise.all([
      waitForGraphQLOperation(page, 'DeleteModel'),
      deleteButton.click(),
    ])
    await expect(deleteDialog).not.toBeVisible({ timeout: 20000 })
    await waitForGraphQLOperation(page, 'GetModels')

    await expect(modelsTable.locator('tbody tr').filter({ hasText: updatedName })).toHaveCount(0)
  })
})
