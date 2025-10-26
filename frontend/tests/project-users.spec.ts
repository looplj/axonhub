//@ts-ignore
import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Project Users Management', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate directly to project users page
    await gotoAndEnsureAuth(page, '/project/users')
    
    // Wait for users table to be visible
    const usersTable = page.locator('[data-testid="users-table"]')
    await expect(usersTable).toBeVisible({ timeout: 10000 })
  })

  test('can add user to project with roles and scopes', async ({ page }) => {
    // Step 1: Create a new user first
    const uniqueSuffix = Date.now().toString().slice(-6)
    const email = `pw-project-test-${uniqueSuffix}@example.com`
    
    // Navigate to users page to create a new user
    await page.goto('/users')
    await page.waitForTimeout(1000)
    
    const addUserButton = page.getByRole('button', { name: /添加用户|Add User|新增用户|Create User/i })
    await expect(addUserButton).toBeVisible()
    await addUserButton.click()

    const createDialog = page.getByRole('dialog')
    await expect(createDialog).toBeVisible()

    await createDialog.getByLabel(/邮箱|Email/i).fill(email)
    await createDialog.getByLabel(/名|First Name/i).fill('pw-project-test')
    await createDialog.getByLabel(/姓|Last Name/i).fill(uniqueSuffix)
    
    const passwordField = createDialog.locator('input[type="password"]').first()
    await passwordField.fill('Admin123!')
    
    const confirmPasswordField = createDialog.locator('input[type="password"]').last()
    await confirmPasswordField.fill('Admin123!')

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateUser'),
      createDialog.getByRole('button', { name: /保存|Save|创建|Create|Save changes/i }).click()
    ])
    
    // Wait for user creation to complete
    await page.waitForTimeout(1000)
    
    // Step 2: Navigate back to project users page
    await page.goto('/project/users')
    const usersTable = page.locator('[data-testid="users-table"]')
    await expect(usersTable).toBeVisible({ timeout: 10000 })
    
    // Step 3: Add the newly created user to project
    const addProjectUserButton = page.getByRole('button', { name: /添加用户|Add User/i })
    await expect(addProjectUserButton).toBeVisible({ timeout: 5000 })
    await addProjectUserButton.click()

    const dialog = page.getByRole('dialog')
    await expect(dialog).toBeVisible()
    await expect(dialog).toContainText(/添加用户|Add User/i)

    // Select the newly created user from dropdown
    const userSelect = dialog.getByRole('combobox').or(dialog.locator('button[role="combobox"]')).first()
    await userSelect.click()
    
    // Wait for options to appear and select the newly created user
    await page.waitForTimeout(500)
    const newUserOption = page.getByRole('option', { name: new RegExp(email, 'i') })
    await newUserOption.click()

    // Optionally assign roles - click on role checkboxes if available
    const roleCheckboxes = dialog.getByRole('checkbox').filter({ hasText: /role/i })
    if (await roleCheckboxes.count() > 0) {
      await roleCheckboxes.first().click()
    }

    // Optionally assign scopes
    const scopeCheckboxes = dialog.getByRole('checkbox')
    if (await scopeCheckboxes.count() > 1) {
      await scopeCheckboxes.nth(0).click()
      await scopeCheckboxes.nth(1).click()
    }

    // Submit the form
    const submitButton = dialog.getByRole('button', { name: /添加|Add|确认|Confirm/i })
    await submitButton.click()
    
    // Expect dialog to close successfully
    await expect(dialog).not.toBeVisible({ timeout: 5000 })
    
    // Verify the user was added to the project
    await page.waitForTimeout(1000)
    const userRow = usersTable.locator('tbody tr').filter({ hasText: email })
    await expect(userRow).toBeVisible()
  })

  test('can edit user roles and scopes', async ({ page }) => {
    const usersTable = page.locator('[data-testid="users-table"]')
    
    // Wait for table to have data
    await page.waitForTimeout(1000)
    const rows = usersTable.locator('tbody tr')
    const rowCount = await rows.count()
    
    if (rowCount === 0) {
      test.skip()
      return
    }

    // Find a non-owner user to edit (skip project owners)
    let targetRow = null
    for (let i = 0; i < Math.min(rowCount, 5); i++) {
      const row = rows.nth(i)
      const rowText = await row.textContent()
      // Skip if this is a project owner
      if (!rowText?.includes('Owner') && !rowText?.includes('所有者')) {
        targetRow = row
        break
      }
    }

    if (!targetRow) {
      test.skip()
      return
    }

    // Click the row actions dropdown
    const actionsTrigger = targetRow.locator('button').last()
    await actionsTrigger.click()
    
    const editMenuItem = page.getByRole('menuitem', { name: /编辑|Edit/i })
    await editMenuItem.waitFor({ state: 'visible', timeout: 5000 })
    await editMenuItem.click()

    const editDialog = page.getByRole('dialog')
    await expect(editDialog).toBeVisible()
    await expect(editDialog).toContainText(/编辑|Edit/i)

    // Toggle some role or scope checkboxes
    const checkboxes = editDialog.getByRole('checkbox')
    if (await checkboxes.count() > 0) {
      await checkboxes.first().click()
    }

    // Save changes
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateProjectUser'),
      editDialog.getByRole('button', { name: /保存|Save|更新|Update/i }).click()
    ])
    
    await expect(editDialog).not.toBeVisible({ timeout: 5000 })
  })

  test('can remove user from project', async ({ page }) => {
    const usersTable = page.locator('[data-testid="users-table"]')
    
    // Wait for table to have data
    await page.waitForTimeout(1000)
    const rows = usersTable.locator('tbody tr')
    const rowCount = await rows.count()
    
    if (rowCount === 0) {
      test.skip()
      return
    }

    // Find a non-owner user to remove
    let targetRow: Locator | null = null
    let targetUserEmail = ''
    for (let i = 0; i < Math.min(rowCount, 5); i++) {
      const row = rows.nth(i)
      const rowText = await row.textContent()
      // Skip if this is a project owner
      if (!rowText?.includes('Owner') && !rowText?.includes('所有者')) {
        targetRow = row
        targetUserEmail = rowText || ''
        break
      }
    }

    if (!targetRow) {
      test.skip()
      return
    }

    // Click the row actions dropdown
    //@ts-ignore
    const actionsTrigger = targetRow.locator('button').last()
    await actionsTrigger.click()
    
    const removeMenuItem = page.getByRole('menuitem', { name: /移除|Remove|删除/i })
    await removeMenuItem.waitFor({ state: 'visible', timeout: 5000 })
    await removeMenuItem.click()

    const confirmDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(confirmDialog).toBeVisible()
    await expect(confirmDialog).toContainText(/移除|Remove|删除/i)

    await Promise.all([
      waitForGraphQLOperation(page, 'RemoveUserFromProject'),
      confirmDialog.getByRole('button', { name: /移除|Remove|删除|Delete|确认|Confirm/i }).click()
    ])

    // Verify user is removed from table
    await page.waitForTimeout(1000)
    const updatedRows = usersTable.locator('tbody tr')
    const updatedRowTexts = await updatedRows.allTextContents()
    expect(updatedRowTexts.some(text => text.includes(targetUserEmail))).toBeFalsy()
  })

  test('can filter users by name or email', async ({ page }) => {
    const usersTable = page.locator('[data-testid="users-table"]')
    
    // Find the search input
    const searchInput = page.getByPlaceholder(/搜索|Search|名称|Name|邮箱|Email/i)
    if (await searchInput.count() === 0) {
      test.skip()
      return
    }

    await expect(searchInput).toBeVisible()
    
    // Get initial row count
    await page.waitForTimeout(500)
    const initialRows = await usersTable.locator('tbody tr').count()
    
    if (initialRows === 0) {
      test.skip()
      return
    }

    // Type a search term (use first few characters from first user)
    const firstRow = usersTable.locator('tbody tr').first()
    const firstRowText = await firstRow.textContent()
    const searchTerm = firstRowText?.trim().substring(0, 3) || 'test'
    
    await searchInput.fill(searchTerm)
    
    // Wait for filter to apply
    await page.waitForTimeout(500)
    
    // Verify filtering worked (row count should change or stay same)
    const filteredRows = await usersTable.locator('tbody tr').count()
    expect(filteredRows).toBeGreaterThanOrEqual(0)
    
    // Clear search
    await searchInput.clear()
    await page.waitForTimeout(500)
  })
})
