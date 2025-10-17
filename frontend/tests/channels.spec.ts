import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Admin Channels Management', () => {
  test.beforeEach(async ({ page }) => {
    // Increase timeout for authentication
    test.setTimeout(60000)
    await gotoAndEnsureAuth(page, '/channels')
  })

  test('can create, edit, and archive a channel', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-test-Channel ${uniqueSuffix}`
    const baseURL = `https://api.test-${uniqueSuffix}.example.com`

    // Step 1: Create a new channel
    const createButton = page.getByRole('button', { name: /Create Channel|创建渠道/i })
    await expect(createButton).toBeVisible()
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await expect(createDialog).toBeVisible()
    await expect(createDialog).toContainText(/创建渠道|Create Channel/i)

    // Fill in channel details
    await createDialog.getByLabel(/名称|Name/i).fill(name)
    
    // Select channel type (OpenAI)
    const typeSelect = createDialog.locator('[name="type"]').or(
      createDialog.getByLabel(/类型|Type/i)
    )
    await typeSelect.click()
    
    // Wait for dropdown and select OpenAI
    const openaiOption = page.getByRole('option', { name: /OpenAI/i }).or(
      page.locator('[role="option"]').filter({ hasText: /OpenAI/i })
    )
    await openaiOption.first().click()

    // Fill in base URL
    await createDialog.getByLabel(/Base URL/i).fill(baseURL)

    // Fill in API Key
    const apiKeyInput = createDialog.getByLabel(/API Key/i)
    await apiKeyInput.fill('sk-test-key-' + uniqueSuffix)

    // Submit the form
    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByRole('button', { name: /创建|Create|保存|Save/i }).click()
    ])

    // Wait for dialog to close
    await expect(createDialog).not.toBeVisible({ timeout: 5000 })

    // Verify channel appears in the table
    await page.waitForTimeout(1000)
    const channelRow = page.locator('tbody tr').filter({ hasText: name })
    await expect(channelRow).toBeVisible()
    await expect(channelRow).toContainText(/enabled|启用/i)

    // Step 2: Edit the channel
    const actionsTrigger = channelRow.locator('button:has(svg)').last()
    await actionsTrigger.click()

    const editMenu = page.getByRole('menu')
    await expect(editMenu).toBeVisible()
    await editMenu.getByRole('menuitem', { name: /编辑|Edit/i }).focus()
    await page.keyboard.press('Enter')

    const editDialog = page.getByRole('dialog')
    await expect(editDialog).toBeVisible()
    await expect(editDialog).toContainText(/编辑渠道|Edit Channel/i)

    // Update the name
    const updatedName = `${name} - Updated`
    const nameInput = editDialog.getByLabel(/名称|Name/i)
    await nameInput.clear()
    await nameInput.fill(updatedName)

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannel'),
      editDialog.getByRole('button', { name: /保存|Save|更新|Update/i }).click()
    ])

    // Verify the updated name appears in the table
    await expect(channelRow).toContainText(updatedName)

    // Step 3: Archive the channel
    await actionsTrigger.click()
    const archiveMenu = page.getByRole('menu')
    await expect(archiveMenu).toBeVisible()
    await archiveMenu.getByRole('menuitem', { name: /归档|Archive/i }).focus()
    await page.keyboard.press('Enter')

    const archiveDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(archiveDialog).toContainText(/归档|Archive/i)
    
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannelStatus'),
      archiveDialog.getByRole('button', { name: /归档|Archive|确认|Confirm/i }).click()
    ])

    // Verify status changed to archived
    await expect(channelRow).toContainText(/archived|归档/i)

    // Step 4: Enable the channel
    await actionsTrigger.click()
    const enableMenu = page.getByRole('menu')
    await expect(enableMenu).toBeVisible()
    await enableMenu.getByRole('menuitem', { name: /启用|Enable/i }).focus()
    await page.keyboard.press('Enter')

    const enableDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(enableDialog).toContainText(/启用|Enable/i)
    
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannelStatus'),
      enableDialog.getByRole('button', { name: /启用|Enable|确认|Confirm/i }).click()
    ])

    // Verify status changed back to enabled
    await expect(channelRow).toContainText(/enabled|启用/i)
  })

  test('can test a channel', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(1000)

    // Find the first enabled channel with a test button
    const testButton = page.getByRole('button', { name: /Test|测试/i }).first()
    
    // Check if test button exists
    const testButtonCount = await testButton.count()
    if (testButtonCount === 0) {
      test.skip()
      return
    }

    await expect(testButton).toBeVisible()
    
    // Click test button
    await Promise.all([
      waitForGraphQLOperation(page, 'TestChannel'),
      testButton.click()
    ])

    // Wait for toast notification (success or error)
    await page.waitForTimeout(2000)
  })

  test('can search channels by name', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const searchTerm = `pw-test-SearchChannel${uniqueSuffix}`
    
    // Create a channel with a unique name for searching
    const createButton = page.getByRole('button', { name: /Create Channel|创建渠道/i })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByLabel(/名称|Name/i).fill(searchTerm)
    
    // Select channel type
    const typeSelect = createDialog.locator('[name="type"]').or(
      createDialog.getByLabel(/类型|Type/i)
    )
    await typeSelect.click()
    const openaiOption = page.getByRole('option', { name: /OpenAI/i }).or(
      page.locator('[role="option"]').filter({ hasText: /OpenAI/i })
    )
    await openaiOption.first().click()

    await createDialog.getByLabel(/Base URL/i).fill('https://api.openai.com/v1')
    await createDialog.getByLabel(/API Key/i).fill('sk-test-key-' + uniqueSuffix)
    
    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByRole('button', { name: /创建|Create|保存|Save/i }).click()
    ])

    // Wait for the table to update
    await page.waitForTimeout(1000)

    // Use the search filter
    const searchInput = page.locator('input[placeholder*="搜索"], input[placeholder*="Search"], input[type="search"]').first()
    await searchInput.fill(searchTerm)

    // Wait for debounce and API call
    await page.waitForTimeout(1000)

    // Verify the searched channel appears
    const searchedRow = page.locator('tbody tr').filter({ hasText: searchTerm })
    await expect(searchedRow).toBeVisible()
  })

  test('can filter channels by type', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(1000)

    // Look for type filter button/dropdown
    const typeFilterButton = page.getByRole('button', { name: /Type|类型/i }).or(
      page.locator('button').filter({ hasText: /Type|类型/i })
    )

    const typeFilterCount = await typeFilterButton.count()
    if (typeFilterCount === 0) {
      test.skip()
      return
    }

    await typeFilterButton.first().click()

    // Wait for filter menu
    await page.waitForTimeout(500)

    // Select OpenAI filter
    const openaiFilter = page.getByRole('menuitemcheckbox', { name: /OpenAI/i }).or(
      page.locator('[role="menuitemcheckbox"]').filter({ hasText: /OpenAI/i })
    )

    const openaiFilterCount = await openaiFilter.count()
    if (openaiFilterCount > 0) {
      await openaiFilter.first().click()
      
      // Wait for filter to apply
      await page.waitForTimeout(1000)

      // Verify filtered results
      const rows = page.locator('tbody tr')
      const rowCount = await rows.count()
      
      if (rowCount > 0) {
        // Check that visible rows contain OpenAI type
        const firstRow = rows.first()
        await expect(firstRow).toContainText(/OpenAI/i)
      }
    }
  })

  test('can filter channels by status', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(1000)

    // Look for status filter button/dropdown
    const statusFilterButton = page.getByRole('button', { name: /Status|状态/i }).or(
      page.locator('button').filter({ hasText: /Status|状态/i })
    )

    const statusFilterCount = await statusFilterButton.count()
    if (statusFilterCount === 0) {
      test.skip()
      return
    }

    await statusFilterButton.first().click()

    // Wait for filter menu
    await page.waitForTimeout(500)

    // Select Enabled filter
    const enabledFilter = page.getByRole('menuitemcheckbox', { name: /Enabled|启用/i }).or(
      page.locator('[role="menuitemcheckbox"]').filter({ hasText: /Enabled|启用/i })
    )

    const enabledFilterCount = await enabledFilter.count()
    if (enabledFilterCount > 0) {
      await enabledFilter.first().click()
      
      // Wait for filter to apply
      await page.waitForTimeout(1000)

      // Verify filtered results
      const rows = page.locator('tbody tr')
      const rowCount = await rows.count()
      
      if (rowCount > 0) {
        // Check that visible rows contain enabled status
        const firstRow = rows.first()
        await expect(firstRow).toContainText(/enabled|启用/i)
      }
    }
  })

  test('validates required fields when creating a channel', async ({ page }) => {
    const createButton = page.getByRole('button', { name: /Create Channel|创建渠道/i })
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
    // Wait for table to load
    await page.waitForTimeout(1000)

    // Look for pagination controls
    const pagination = page.locator('[data-testid="pagination"]').or(
      page.locator('nav').filter({ hasText: /页|Page|Previous|Next/i })
    )

    // Check if pagination exists
    const paginationCount = await pagination.count()
    if (paginationCount === 0) {
      test.skip()
      return
    }

    // Check if Next button exists and is enabled
    const nextButton = pagination.getByRole('button', { name: /下一页|Next/i })
    const nextButtonCount = await nextButton.count()
    
    if (nextButtonCount === 0) {
      test.skip()
      return
    }
    
    // Only test pagination if there are multiple pages
    const isEnabled = await nextButton.isEnabled().catch(() => false)
    if (isEnabled) {
      const firstPageContent = await page.locator('tbody tr').first().textContent()
      
      await nextButton.click()
      await page.waitForTimeout(1000)
      
      const secondPageContent = await page.locator('tbody tr').first().textContent()
      
      // Content should be different on the second page
      expect(firstPageContent).not.toBe(secondPageContent)
      
      // Go back to previous page
      const prevButton = pagination.getByRole('button', { name: /上一页|Previous/i })
      await expect(prevButton).toBeEnabled()
      await prevButton.click()
      await page.waitForTimeout(1000)
    } else {
      test.skip()
    }
  })

  test('can open channel settings dialog', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(1000)

    // Find the first channel row
    const firstRow = page.locator('tbody tr').first()
    const rowCount = await page.locator('tbody tr').count()
    
    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('button:has(svg)').last()
    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    // Look for settings option
    const settingsOption = menu.getByRole('menuitem', { name: /设置|Settings/i })
    const settingsCount = await settingsOption.count()
    
    if (settingsCount > 0) {
      await settingsOption.focus()
      await page.keyboard.press('Enter')

      // Verify settings dialog opens
      const settingsDialog = page.getByRole('dialog')
      await expect(settingsDialog).toBeVisible()
      await expect(settingsDialog).toContainText(/设置|Settings/i)
    }
  })

  test('can bulk import channels', async ({ page }) => {
    // Look for bulk import button
    const bulkImportButton = page.getByRole('button', { name: /Bulk Import|批量导入/i })
    
    const bulkImportCount = await bulkImportButton.count()
    if (bulkImportCount === 0) {
      test.skip()
      return
    }

    await bulkImportButton.click()

    // Verify bulk import dialog opens
    const bulkImportDialog = page.getByRole('dialog')
    await expect(bulkImportDialog).toBeVisible()
    await expect(bulkImportDialog).toContainText(/Bulk Import|批量导入/i)

    // Close the dialog
    const closeButton = bulkImportDialog.getByRole('button', { name: /取消|Cancel|Close/i })
    if (await closeButton.count() > 0) {
      await closeButton.click()
    } else {
      await page.keyboard.press('Escape')
    }
  })
})
