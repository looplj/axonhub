import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Admin Projects Management', () => {
  test.beforeEach(async ({ page }) => {
    // Increase timeout for authentication
    test.setTimeout(60000)
    await gotoAndEnsureAuth(page, '/projects')
  })

  test('can create, edit, archive, and activate a project', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-test-Project ${uniqueSuffix}`
    const description = `This is a test project created by Playwright at ${new Date().toISOString()}`

    // Step 1: Create a new project
    const createButton = page.getByRole('button', { name: /Create Project/i })
    await expect(createButton).toBeVisible()
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await expect(createDialog).toBeVisible()
    await expect(createDialog).toContainText(/创建项目|Create Project/i)

    // Fill in project details
    await createDialog.getByLabel(/名称|Name/i).fill(name)
    await createDialog.getByLabel(/描述|Description/i).fill(description)

    // Submit the form
    await Promise.all([
      waitForGraphQLOperation(page, 'CreateProject'),
      createDialog.getByRole('button', { name: /创建|Create|保存|Save/i }).click()
    ])

    // Verify project appears in the table
    const projectsTable = page.locator('[data-testid="projects-table"]')
    await expect(projectsTable).toBeVisible()
    
    const projectRow = projectsTable.locator('tbody tr').filter({ hasText: name })
    await expect(projectRow).toBeVisible()
    await expect(projectRow).toContainText(name)
    await expect(projectRow).toContainText(/active|激活|活跃/i)

    // Step 2: Edit the project
    const actionsTrigger = projectRow.locator('button:has(svg)').last()
    await actionsTrigger.click()

    const editMenu = page.getByRole('menu')
    await expect(editMenu).toBeVisible()
    await editMenu.getByRole('menuitem', { name: /编辑|Edit/i }).focus()
    await page.keyboard.press('Enter')

    const editDialog = page.getByRole('dialog')
    await expect(editDialog).toBeVisible()
    await expect(editDialog).toContainText(/编辑项目|Edit Project/i)

    // Update the name
    const updatedName = `${name} - Updated`
    const nameInput = editDialog.getByLabel(/名称|Name/i)
    await nameInput.clear()
    await nameInput.fill(updatedName)

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateProject'),
      editDialog.getByRole('button', { name: /保存|Save|更新|Update/i }).click()
    ])

    // Verify the updated name appears in the table
    await expect(projectRow).toContainText(updatedName)

    // Step 3: Archive the project
    await actionsTrigger.click()
    const archiveMenu = page.getByRole('menu')
    await expect(archiveMenu).toBeVisible()
    await archiveMenu.getByRole('menuitem', { name: /归档|Archive/i }).focus()
    await page.keyboard.press('Enter')

    const archiveDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(archiveDialog).toContainText(/归档|Archive/i)
    
    await Promise.all([
      waitForGraphQLOperation(page, 'ArchiveProject'),
      archiveDialog.getByRole('button', { name: /归档|Archive|确认|Confirm/i }).click()
    ])

    // Verify status changed to archived
    await expect(projectRow).toContainText(/archived|归档/i)

    // Verify menu now shows Activate option
    await actionsTrigger.click()
    const verifyMenu = page.getByRole('menu')
    await expect(verifyMenu).toBeVisible()
    await expect(verifyMenu.getByRole('menuitem', { name: /激活|Activate/i })).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(verifyMenu).not.toBeVisible()

    // Step 4: Activate the project
    await actionsTrigger.click()
    const activateMenu = page.getByRole('menu')
    await expect(activateMenu).toBeVisible()
    await activateMenu.getByRole('menuitem', { name: /激活|Activate/i }).focus()
    await page.keyboard.press('Enter')

    const activateDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(activateDialog).toContainText(/激活|Activate/i)
    
    await Promise.all([
      waitForGraphQLOperation(page, 'ActivateProject'),
      activateDialog.getByRole('button', { name: /激活|Activate|确认|Confirm/i }).click()
    ])

    // Verify status changed back to active
    await expect(projectRow).toContainText(/active|激活|活跃/i)
  })

  test('can search projects by name', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const searchTerm = `pw-test-SearchTest${uniqueSuffix}`
    
    // Create a project with a unique name for searching
    const createButton = page.getByRole('button', { name: /Create Project/i })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByLabel(/名称|Name/i).fill(searchTerm)
    
    await Promise.all([
      waitForGraphQLOperation(page, 'CreateProject'),
      createDialog.getByRole('button', { name: /创建|Create|保存|Save/i }).click()
    ])

    // Wait for the table to update
    await page.waitForTimeout(500)

    // Use the search filter
    const searchInput = page.locator('input[placeholder*="搜索"], input[placeholder*="Search"], input[type="search"]').first()
    await searchInput.fill(searchTerm)

    // Wait for debounce and API call
    await page.waitForTimeout(500)

    // Verify the searched project appears
    const projectsTable = page.locator('[data-testid="projects-table"]')
    const searchedRow = projectsTable.locator('tbody tr').filter({ hasText: searchTerm })
    await expect(searchedRow).toBeVisible()
  })

  test('can search projects by name (additional test)', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const projectName = `pw-test-Project for Name Search ${uniqueSuffix}`
    
    // Create a project with a unique name for searching
    const createButton = page.getByRole('button', { name: /Create Project/i })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByLabel(/名称|Name/i).fill(projectName)
    
    await Promise.all([
      waitForGraphQLOperation(page, 'CreateProject'),
      createDialog.getByRole('button', { name: /创建|Create|保存|Save/i }).click()
    ])

    // Wait for the table to update
    await page.waitForTimeout(500)

    // Use the search filter with project name
    const searchInput = page.locator('input[placeholder*="搜索"], input[placeholder*="Search"], input[type="search"]').first()
    await searchInput.fill(projectName)

    // Wait for debounce and API call
    await page.waitForTimeout(500)

    // Verify the searched project appears
    const projectsTable = page.locator('[data-testid="projects-table"]')
    const searchedRow = projectsTable.locator('tbody tr').filter({ hasText: projectName })
    await expect(searchedRow).toBeVisible()
  })

  test('validates required fields when creating a project', async ({ page }) => {
    const createButton = page.getByRole('button', { name: /Create Project/i })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await expect(createDialog).toBeVisible()

    // Try to submit without filling required fields
    const submitButton = createDialog.getByRole('button', { name: /创建|Create|保存|Save/i })
    await submitButton.click()

    // Verify validation messages appear (form should not close)
    await expect(createDialog).toBeVisible()
    
    // Check for validation error indicators
    const nameInput = createDialog.getByLabel(/名称|Name/i)
    await expect(nameInput).toHaveAttribute('aria-invalid', 'true')
  })

  test('can navigate between pages', async ({ page }) => {
    // This test assumes there are enough projects to paginate
    const projectsTable = page.locator('[data-testid="projects-table"]')
    await expect(projectsTable).toBeVisible()

    // Look for pagination controls
    const pagination = page.locator('[data-testid="pagination"]').or(
      page.locator('nav').filter({ hasText: /页|Page|Previous|Next/i })
    )

    // Check if pagination exists
    const paginationCount = await pagination.count()
    if (paginationCount === 0) {
      // No pagination, skip test
      test.skip()
      return
    }

    // Check if Next button exists and is enabled
    const nextButton = pagination.getByRole('button', { name: /下一页|Next/i })
    const nextButtonCount = await nextButton.count()
    
    if (nextButtonCount === 0) {
      // No next button, skip test
      test.skip()
      return
    }
    
    // Only test pagination if there are multiple pages
    const isEnabled = await nextButton.isEnabled().catch(() => false)
    if (isEnabled) {
      const firstPageContent = await projectsTable.locator('tbody tr').first().textContent()
      
      await nextButton.click()
      await page.waitForTimeout(500)
      
      const secondPageContent = await projectsTable.locator('tbody tr').first().textContent()
      
      // Content should be different on the second page
      expect(firstPageContent).not.toBe(secondPageContent)
      
      // Go back to previous page
      const prevButton = pagination.getByRole('button', { name: /上一页|Previous/i })
      await expect(prevButton).toBeEnabled()
      await prevButton.click()
      await page.waitForTimeout(500)
    } else {
      // Not enough data to test pagination
      test.skip()
    }
  })
})
