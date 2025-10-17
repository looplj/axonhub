import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Admin Users Management', () => {
  test.beforeEach(async ({ page }) => {
    await gotoAndEnsureAuth(page, '/users')
  })

  test('can create, deactivate, activate, and edit a user', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const email = `pw-test-user-${uniqueSuffix}@example.com`

    const addUserButton = page.getByRole('button', { name: /添加用户|Add User|新增用户|Create User/i })
    await expect(addUserButton).toBeVisible()
    await addUserButton.click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()

    await dialog.getByLabel(/邮箱|Email/i).fill(email)
    await dialog.getByLabel(/名|First Name/i).fill('pw-test')
    await dialog.getByLabel(/姓|Last Name/i).fill(uniqueSuffix)
    
    // Fill password fields with more flexible selectors
    const passwordField = dialog.locator('input[type="password"]').first()
    await passwordField.fill('Admin123!')
    
    const confirmPasswordField = dialog.locator('input[type="password"]').last()
    await confirmPasswordField.fill('Admin123!')

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateUser'),
      dialog.getByRole('button', { name: /保存|Save|创建|Create|Save changes/i }).click()
    ])

    const usersTable = page.locator('[data-testid="users-table"], table:has(th), table').first()
    const row = usersTable.locator('tbody tr').filter({ hasText: email })
    await expect(row).toBeVisible()

    const actionsTrigger = row.locator('[data-testid="row-actions"], button:has(svg), .dropdown-trigger, .action-button, button:has-text("Open menu")').first()

    // Deactivate user
    await actionsTrigger.click()
    const menu1 = page.getByRole('menu')
    await expect(menu1).toBeVisible()
    await menu1.getByRole('menuitem', { name: /停用|禁用|Deactivate|Disable/i }).focus()
    await page.keyboard.press('Enter')
    const statusDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(statusDialog).toBeVisible()
    await expect(statusDialog).toContainText(/停用|禁用|Deactivate|Disable/i)
    
    // Wait for the button to be stable before clicking
    const deactivateButton = statusDialog.getByRole('button', { name: /停用|激活|Deactivate|Activate|确认|Confirm|保存|Save/i })
    await expect(deactivateButton).toBeVisible()
    await expect(deactivateButton).toBeEnabled()
    
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateUserStatus'),
      deactivateButton.click()
    ])
    
    // Wait for dialog to close before proceeding
    await expect(statusDialog).not.toBeVisible()
    
    // Verify by menu toggle: now it should show Activate
    await actionsTrigger.click()
    const menu2 = page.getByRole('menu')
    await expect(menu2).toBeVisible()
    await expect(menu2.getByRole('menuitem', { name: /激活|Activate|启用/i })).toBeVisible()
    
    // Close the menu before reopening
    await page.keyboard.press('Escape')
    await expect(menu2).not.toBeVisible()

    // Activate user
    await actionsTrigger.click()
    const menu3 = page.getByRole('menu')
    await expect(menu3).toBeVisible()
    await menu3.getByRole('menuitem', { name: /激活|Activate|启用/i }).focus()
    await page.keyboard.press('Enter')
    const activateDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(activateDialog).toBeVisible()
    await expect(activateDialog).toContainText(/激活|Activate|启用/i)
    
    // Wait for the button to be stable before clicking
    const activateButton = activateDialog.getByRole('button', { name: /激活|Activate|确认|Confirm|保存|Save/i })
    await expect(activateButton).toBeVisible()
    await expect(activateButton).toBeEnabled()
    
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateUserStatus'),
      activateButton.click()
    ])
    
    // Wait for dialog to close before proceeding
    await expect(activateDialog).not.toBeVisible()
    
    // Verify by menu toggle: now it should show Deactivate/Disable
    await actionsTrigger.click()
    await expect(page.getByRole('menuitem', { name: /停用|禁用|Deactivate|Disable/i })).toBeVisible()
    
    // Close the menu before reopening
    await page.keyboard.press('Escape')
    await expect(page.getByRole('menu')).not.toBeVisible()

    // Edit user
    await actionsTrigger.click()
    const menu4 = page.getByRole('menu')
    await expect(menu4).toBeVisible()
    await menu4.getByRole('menuitem', { name: /编辑|Edit/i }).focus()
    await page.keyboard.press('Enter')
    const editDialog = page.getByRole('dialog').or(page.getByRole('alertdialog'))
    await expect(editDialog).toBeVisible()
    await expect(editDialog).toContainText(/编辑用户|Edit/i)
    const firstNameInput = editDialog.getByLabel(/名|First Name/i)
    await firstNameInput.fill('pw-test-Updated')

    // Wait for the save button to be stable before clicking
    const saveButton = editDialog.getByRole('button', { name: /保存|Save|更新|Update/i })
    await expect(saveButton).toBeVisible()
    await expect(saveButton).toBeEnabled()

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateUser'),
      saveButton.click()
    ])

    // Wait for edit dialog to close before proceeding
    await expect(editDialog).not.toBeVisible()
    
    await expect(row).toContainText('pw-test-Updated')
  })
})
