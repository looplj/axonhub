import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from '../auth.utils'

/**
 * Test suite to verify that pages remain clickable after closing dialogs
 * that were opened from dropdown menus.
 * 
 * This addresses a Radix UI issue where nested DropdownMenu + Dialog/AlertDialog
 * can cause body pointer-events to remain 'none' after dialog closes.
 */
test.describe('Dialog Interaction Fix', () => {
  test('roles page remains clickable after delete dialog', async ({ page }) => {
    await gotoAndEnsureAuth(page, '/roles')
    
    const uniqueSuffix = Date.now().toString().slice(-5)
    const roleCode = `pw-test-role-${uniqueSuffix}`
    const roleName = `pw-test-Role ${uniqueSuffix}`

    // Create a role
    const createButton = page.getByRole('button', { name: /新建角色|创建角色|Create Role/i })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByLabel(/角色代码|Role Code|代码/i).fill(roleCode)
    await createDialog.getByLabel(/角色名称|Role Name|名称/i).fill(roleName)
    await createDialog.getByRole('checkbox').first().click()

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateRole'),
      createDialog.getByRole('button', { name: /保存|Save|创建|Create/i }).click()
    ])

    // Delete the role
    const row = page.locator('table tbody tr').filter({ hasText: roleName })
    const actionButton = row.locator('button').filter({ hasText: /打开菜单|Actions|操作/ }).or(row.locator('button:has(svg)')).first()
    await actionButton.click()
    
    const deleteMenuItem = page.getByRole('menuitem', { name: /删除|Delete/i })
    await deleteMenuItem.waitFor({ state: 'visible', timeout: 5000 })
    await deleteMenuItem.click()

    const deleteDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(deleteDialog).toBeVisible()
    const deleteButton = deleteDialog.getByRole('button', { name: /删除|Delete|确认|Confirm/i })
    await expect(deleteButton).toBeVisible()
    await Promise.all([
      waitForGraphQLOperation(page, 'DeleteRole'),
      deleteButton.click()
    ])

    await expect(deleteDialog).not.toBeVisible({ timeout: 5000 })

    // Verify page is clickable
    await page.waitForTimeout(500)
    await createButton.click({ timeout: 5000 })
    await expect(page.getByRole('dialog')).toBeVisible()
  })

  test('projects page remains clickable after archive dialog', async ({ page }) => {
    await gotoAndEnsureAuth(page, '/projects')
    
    // Try to interact with an existing project
    const firstActionButton = page.locator('table tbody tr button').first()
    if (await firstActionButton.count() > 0) {
      await firstActionButton.click()
      
      // Try to click archive if available
      const archiveItem = page.getByRole('menuitem', { name: /归档|Archive/i })
      if (await archiveItem.count() > 0) {
        await archiveItem.click()
        
        const dialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
        if (await dialog.count() > 0) {
          // Cancel the dialog
          const cancelButton = dialog.getByRole('button', { name: /取消|Cancel/i })
          await cancelButton.click()
          await expect(dialog).not.toBeVisible({ timeout: 5000 })
          
          // Verify page is clickable
          await page.waitForTimeout(500)
          const createButton = page.getByRole('button', { name: /新建|创建|Create/i }).first()
          await expect(createButton).toBeEnabled()
        }
      }
    }
  })
})
