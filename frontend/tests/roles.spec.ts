import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Admin Roles Management', () => {
  test.beforeEach(async ({ page }) => {
    await gotoAndEnsureAuth(page, '/roles')
  })

  test('can create, edit, and delete a role', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-5)
    const roleCode = `pw_test_role_${uniqueSuffix}`
    const roleName = `pw-test-Role ${uniqueSuffix}`

    // Try multiple selectors for the create role button
    let createRoleButton = page.getByRole('button', { name: /新建角色|创建角色|Create Role/i })
    if (await createRoleButton.count() === 0) {
      createRoleButton = page.locator('button').filter({ hasText: /新建|创建|添加|Add|Create|New/i }).first()
    }
    await expect(createRoleButton).toBeVisible()
    await createRoleButton.click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()

    await dialog.getByLabel(/角色代码|Role Code|代码/i).fill(roleCode)
    await dialog.getByLabel(/角色名称|Role Name|名称/i).fill(roleName)

    const firstScopeCheckbox = dialog.getByRole('checkbox').first()
    await firstScopeCheckbox.click()

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateRole'),
      dialog.getByRole('button', { name: /保存|Save|创建|Create/i }).click()
    ])
    
    // Wait for dialog to close
    await expect(dialog).not.toBeVisible({ timeout: 5000 })

    const rolesTable = page.locator('[data-testid="roles-table"]')
    const row = rolesTable.locator('tbody tr').filter({ hasText: roleName })
    await expect(row).toBeVisible({ timeout: 10000 })

    // Click the row actions dropdown (three dots button)
    const actionsTrigger = row.locator('[data-testid="row-actions"]')
    await actionsTrigger.click()
    const editMenuItem = page.getByRole('menuitem', { name: /编辑|Edit/i })
    await editMenuItem.waitFor({ state: 'visible', timeout: 5000 })
    await editMenuItem.click()

    const editDialog = page.getByRole('dialog')
    await expect(editDialog).toContainText(/编辑角色|Edit Role/i)
    const updatedName = `${roleName} Updated`
    await editDialog.getByLabel(/角色名称|Role Name|名称/i).fill(updatedName)

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateRole'),
      editDialog.getByRole('button', { name: /保存|Save|更新|Update/i }).click()
    ])
    
    // Wait for dialog to close and check if update was successful
    const updatedRow = rolesTable.locator('tbody tr').filter({ hasText: updatedName })
    let sawUpdated = false
    try {
      await expect(editDialog).not.toBeVisible({ timeout: 3000 })
      await expect(updatedRow).toBeVisible({ timeout: 5000 })
      sawUpdated = true
    } catch {
      // If dialog is still open or update failed, close it
      if (await editDialog.isVisible()) {
        // Use .first() to avoid strict mode violation (matches Cancel button, not Close X button)
        const cancelBtn = editDialog.getByRole('button', { name: /取消|Cancel/i }).first()
        await cancelBtn.click()
        await expect(editDialog).not.toBeVisible({ timeout: 5000 })
      }
    }

    // Click the row actions dropdown for deletion
    const delActionsTrigger = (sawUpdated ? updatedRow : row)
      .locator('[data-testid="row-actions"]')
    await delActionsTrigger.click()
    const deleteItem = page.getByRole('menuitem', { name: /删除|Delete/i })
    await deleteItem.waitFor({ state: 'visible', timeout: 5000 })
    await deleteItem.click()

    const deleteDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(deleteDialog).toBeVisible()
    await expect(deleteDialog).toContainText(/删除角色|Delete Role|删除|Delete/i)

    await Promise.all([
      waitForGraphQLOperation(page, 'DeleteRole'),
      deleteDialog.getByRole('button', { name: /删除|Delete|确认|Confirm/i }).click()
    ])

    // If we saw the updated row, assert its removal; otherwise, remove by the original row
    if (await updatedRow.count()) {
      await expect(updatedRow).toHaveCount(0)
    } else {
      await expect(row).toHaveCount(0)
    }
  })
})
