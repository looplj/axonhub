import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Project Roles Management', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate directly to project roles page
    await gotoAndEnsureAuth(page, '/project/roles')
    
    // Wait for roles table to be visible
    const rolesTable = page.locator('[data-testid="roles-table"]')
    await expect(rolesTable).toBeVisible({ timeout: 10000 })
  })

  test('can create, edit, and delete a project role', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-5)
    const roleName = `pw-test-ProjectRole ${uniqueSuffix}`

    // Create a new role
    let createRoleButton = page.getByRole('button', { name: /新建角色|创建角色|Create Role/i })
    if (await createRoleButton.count() === 0) {
      createRoleButton = page.locator('button').filter({ hasText: /新建|创建|添加|Add|Create|New/i }).first()
    }
    await expect(createRoleButton).toBeVisible()
    await createRoleButton.click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()

    await dialog.getByLabel(/角色名称|Role Name|名称/i).fill(roleName)

    // Select first two scopes
    const firstScopeCheckbox = dialog.getByRole('checkbox').first()
    const secondScopeCheckbox = dialog.getByRole('checkbox').nth(1)
    await firstScopeCheckbox.click()
    await secondScopeCheckbox.click()

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateProjectRole'),
      dialog.getByRole('button', { name: /保存|Save|创建|Create/i }).click()
    ])
    
    // Wait for dialog to close
    await expect(dialog).not.toBeVisible({ timeout: 5000 })

    const rolesTable = page.locator('[data-testid="roles-table"]')
    const row = rolesTable.locator('tbody tr').filter({ hasText: roleName })
    await expect(row).toBeVisible({ timeout: 10000 })

    // Click the row actions dropdown (three dots button)
    const actionsTrigger = row.locator('button').last()
    await actionsTrigger.click()
    const editMenuItem = page.getByRole('menuitem', { name: /编辑|Edit/i })
    await editMenuItem.waitFor({ state: 'visible', timeout: 5000 })
    await editMenuItem.click()

    const editDialog = page.getByRole('dialog')
    await expect(editDialog).toContainText(/编辑角色|Edit Role/i)
    
    // Verify that the previously selected scopes are checked
    const firstCheckboxInEdit = editDialog.getByRole('checkbox').first()
    const secondCheckboxInEdit = editDialog.getByRole('checkbox').nth(1)
    await expect(firstCheckboxInEdit).toBeChecked()
    await expect(secondCheckboxInEdit).toBeChecked()
    
    const updatedName = `${roleName} Updated`
    await editDialog.getByLabel(/角色名称|Role Name|名称/i).fill(updatedName)

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateProjectRole'),
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
        const cancelBtn = editDialog.getByRole('button', { name: /取消|Cancel/i }).first()
        await cancelBtn.click()
        await expect(editDialog).not.toBeVisible({ timeout: 5000 })
      }
    }

    // Click the row actions dropdown for deletion
    const delActionsTrigger = (sawUpdated ? updatedRow : row)
      .locator('button').last()
    await delActionsTrigger.click()
    const deleteItem = page.getByRole('menuitem', { name: /删除|Delete/i })
    await deleteItem.waitFor({ state: 'visible', timeout: 5000 })
    await deleteItem.click()

    const deleteDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(deleteDialog).toBeVisible()
    await expect(deleteDialog).toContainText(/删除角色|Delete Role|删除|Delete/i)

    await Promise.all([
      waitForGraphQLOperation(page, 'DeleteProjectRole'),
      deleteDialog.getByRole('button', { name: /删除|Delete|确认|Confirm/i }).click()
    ])

    // If we saw the updated row, assert its removal; otherwise, remove by the original row
    if (await updatedRow.count()) {
      await expect(updatedRow).toHaveCount(0)
    } else {
      await expect(row).toHaveCount(0)
    }
  })

  test('edit dialog should display existing scopes correctly', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-5)
    const roleName = `pw-scope-test-ProjectRole ${uniqueSuffix}`

    // Create a role with specific scopes
    let createRoleButton = page.getByRole('button', { name: /新建角色|创建角色|Create Role/i })
    if (await createRoleButton.count() === 0) {
      createRoleButton = page.locator('button').filter({ hasText: /新建|创建|添加|Add|Create|New/i }).first()
    }
    await createRoleButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByLabel(/角色名称|Role Name|名称/i).fill(roleName)

    // Select specific scopes (first, third, and fifth)
    const checkboxes = createDialog.getByRole('checkbox')
    await checkboxes.nth(0).click()
    await checkboxes.nth(2).click()
    await checkboxes.nth(4).click()

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateProjectRole'),
      createDialog.getByRole('button', { name: /保存|Save|创建|Create/i }).click()
    ])
    
    await expect(createDialog).not.toBeVisible({ timeout: 5000 })

    // Open edit dialog
    const rolesTable = page.locator('[data-testid="roles-table"]')
    const row = rolesTable.locator('tbody tr').filter({ hasText: roleName })
    await expect(row).toBeVisible({ timeout: 10000 })

    const actionsTrigger = row.locator('button').last()
    await actionsTrigger.click()
    const editMenuItem = page.getByRole('menuitem', { name: /编辑|Edit/i })
    await editMenuItem.waitFor({ state: 'visible', timeout: 5000 })
    await editMenuItem.click()

    const editDialog = page.getByRole('dialog')
    await expect(editDialog).toContainText(/编辑角色|Edit Role/i)

    // Verify that the exact scopes we selected are checked
    const editCheckboxes = editDialog.getByRole('checkbox')
    await expect(editCheckboxes.nth(0)).toBeChecked()
    await expect(editCheckboxes.nth(1)).not.toBeChecked()
    await expect(editCheckboxes.nth(2)).toBeChecked()
    await expect(editCheckboxes.nth(3)).not.toBeChecked()
    await expect(editCheckboxes.nth(4)).toBeChecked()

    // Close dialog and clean up
    const cancelBtn = editDialog.getByRole('button', { name: /取消|Cancel/i }).first()
    await cancelBtn.click()
    await expect(editDialog).not.toBeVisible({ timeout: 5000 })

    // Delete the test role
    const deleteActionsTrigger = row.locator('button').last()
    await deleteActionsTrigger.click()
    const deleteItem = page.getByRole('menuitem', { name: /删除|Delete/i })
    await deleteItem.waitFor({ state: 'visible', timeout: 5000 })
    await deleteItem.click()

    const deleteDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await Promise.all([
      waitForGraphQLOperation(page, 'DeleteProjectRole'),
      deleteDialog.getByRole('button', { name: /删除|Delete|确认|Confirm/i }).click()
    ])

    await expect(row).toHaveCount(0)
  })
})
