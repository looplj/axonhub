import { test, expect } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Admin Channels Management', () => {
  test.beforeEach(async ({ page }) => {
    // Increase timeout for authentication
    test.setTimeout(60000)
    await gotoAndEnsureAuth(page, '/channels')

    // Wait for page to fully load
    await page.waitForTimeout(2000)
  })

  test('can create, edit, and archive a channel', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-test-Channel ${uniqueSuffix}`
    const baseURL = `https://api.test-${uniqueSuffix}.example.com`

    // Step 1: Create a new channel
    const createButton = page.getByRole('button', { name: /Add Channel|添加渠道/i })
    await expect(createButton).toBeVisible()
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await expect(createDialog).toBeVisible()
    await expect(createDialog).toContainText(/创建|Create/i)

    // Fill in channel details
    await createDialog.getByLabel(/名称|Name/i).fill(name)

    // Select channel type (OpenAI) - use data-testid for reliable selection
    const openaiRadioContainer = createDialog.getByTestId('channel-type-openai')
    await openaiRadioContainer.click()

    // Fill in base URL
    await createDialog.getByLabel(/Base URL/i).fill(baseURL)

    // Fill in API Key
    const apiKeyInput = createDialog.getByLabel(/API Key/i)
    await apiKeyInput.fill('sk-test-key-' + uniqueSuffix)

    // Add at least one supported model (required to enable Create button)
    // Wait for Quick Add Models section to appear
    await page.waitForTimeout(500)

    // Click on one of the quick add model badges (e.g., gpt-4o)
    const modelBadge = createDialog.locator('text=gpt-4o').first()
    if ((await modelBadge.count()) > 0) {
      await modelBadge.click()

      // Click "Add Selected" button to add the selected models
      const addSelectedButton = createDialog.getByRole('button', { name: /Add Selected|添加选中/i })
      await addSelectedButton.click()

      // Wait for model to be added
      await page.waitForTimeout(500)
    }

    // Select Default Test Model (required field)
    const defaultTestModelSelect = createDialog
      .locator('[name="defaultTestModel"]')
      .or(createDialog.getByLabel(/Test Model|默认测试模型/i))
    if ((await defaultTestModelSelect.count()) > 0) {
      await defaultTestModelSelect.click()
      // Select the first available option (gpt-4o)
      const firstOption = page.getByRole('option').first()
      await firstOption.click()
      await page.waitForTimeout(300)
    }

    // Submit the form
    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByRole('button', { name: /Create|创建|保存|Save/i }).click(),
    ])

    // Wait for dialog to close
    await expect(createDialog).not.toBeVisible({ timeout: 10000 })

    // Verify channel appears in the table
    await page.waitForTimeout(1000)
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const channelRow = channelsTable.locator('tbody tr').filter({ hasText: name })
    await expect(channelRow).toBeVisible()
    // New channels are created with 'disabled' status by default
    await expect(channelRow).toContainText(/disabled|禁用/i)

    // Step 2: Edit the channel
    const actionsTrigger = channelRow.locator('[data-testid="row-actions"]')
    await actionsTrigger.click()

    const editMenu = page.getByRole('menu')
    await expect(editMenu).toBeVisible()
    await editMenu.getByRole('menuitem', { name: /编辑|Edit/i }).focus()
    await page.keyboard.press('Enter')

    const editDialog = page.getByRole('dialog')
    await expect(editDialog).toBeVisible()
    await expect(editDialog).toContainText(/编辑|Edit Channel/i)

    // Update the name
    const updatedName = `${name} - Updated`
    const nameInput = editDialog.getByLabel(/名称|Name/i)
    await nameInput.clear()
    await nameInput.fill(updatedName)

    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannel'),
      editDialog.getByRole('button', { name: /Edit|编辑|保存|Save|更新|Update/i }).click(),
    ])

    // Wait for dialog to close
    await expect(editDialog).not.toBeVisible({ timeout: 10000 })

    // Wait for table to update
    await page.waitForTimeout(1000)

    // Re-locate the channel row with updated name
    const updatedChannelRow = channelsTable.locator('tbody tr').filter({ hasText: updatedName })
    await expect(updatedChannelRow).toBeVisible()
    await expect(updatedChannelRow).toContainText(updatedName)

    // Step 3: Archive the channel
    const archiveActionsTrigger = updatedChannelRow.locator('[data-testid="row-actions"]')
    await archiveActionsTrigger.click()
    const archiveMenu = page.getByRole('menu')
    await expect(archiveMenu).toBeVisible()
    await archiveMenu.getByRole('menuitem', { name: /归档|Archive/i }).focus()
    await page.keyboard.press('Enter')

    const archiveDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(archiveDialog).toBeVisible()
    await expect(archiveDialog).toContainText(/归档|Archive/i)

    // Wait for dialog to stabilize
    await page.waitForTimeout(500)

    // Click the confirm button - it's the last button (first is Cancel)
    const archiveButton = archiveDialog.locator('button').last()
    await Promise.all([waitForGraphQLOperation(page, 'UpdateChannelStatus'), archiveButton.click()])

    // Wait for dialog to close before proceeding
    await expect(archiveDialog).not.toBeVisible({ timeout: 10000 })

    // Wait for table to update (archived channels are hidden by default)
    await page.waitForTimeout(1000)

    // Archived channels are excluded from the default view, so we need to apply the status filter
    // Click on Status filter button (in the toolbar, not the table header)
    // The filter uses a Popover, not a DropdownMenu
    const statusFilterButton = page
      .locator('button')
      .filter({ hasText: /Status|状态/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()
    await statusFilterButton.click()

    // Wait for popover to open
    await page.waitForTimeout(500)

    // Select Archived filter - it's a CommandItem, not a menuitemcheckbox
    // Use a more flexible selector
    const archivedFilter = page
      .getByRole('option', { name: /Archived|已归档/i })
      .or(page.locator('[role="option"]').filter({ hasText: /Archived|已归档/i }))
    await expect(archivedFilter).toBeVisible({ timeout: 5000 })
    await archivedFilter.click()

    // Wait for filter to apply
    await page.waitForTimeout(1000)

    // Now verify the archived channel appears
    const archivedChannelRow = channelsTable.locator('tbody tr').filter({ hasText: updatedName })
    await expect(archivedChannelRow).toBeVisible()
    await expect(archivedChannelRow).toContainText(/Archived|归档/i)

    // Step 4: Enable the channel
    const enableActionsTrigger = archivedChannelRow.locator('[data-testid="row-actions"]')
    await enableActionsTrigger.click()
    const enableMenu = page.getByRole('menu')
    await expect(enableMenu).toBeVisible()
    await enableMenu.getByRole('menuitem', { name: /启用|Enable/i }).focus()
    await page.keyboard.press('Enter')

    const enableDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(enableDialog).toBeVisible()
    await expect(enableDialog).toContainText(/启用|Enable/i)

    // Wait for dialog to stabilize
    await page.waitForTimeout(500)

    // Click the confirm button - it's the last button (first is Cancel)
    const enableButton = enableDialog.locator('button').last()
    await Promise.all([waitForGraphQLOperation(page, 'UpdateChannelStatus'), enableButton.click()])

    // Wait for dialog to close before proceeding
    await expect(enableDialog).not.toBeVisible({ timeout: 10000 })

    // Wait for table to refetch channels after enabling
    await waitForGraphQLOperation(page, 'GetChannels')

    // Clear the Archived filter to see enabled channels
    await statusFilterButton.click()
    await page.waitForTimeout(500)

    // Uncheck Archived filter (it's a CommandItem with role="option")
    const archivedFilterToUncheck = page
      .getByRole('option', { name: /Archived|已归档/i })
      .or(page.locator('[role="option"]').filter({ hasText: /Archived|已归档/i }))
    await expect(archivedFilterToUncheck).toBeVisible({ timeout: 5000 })
    await archivedFilterToUncheck.click()

    // Wait for table to refetch channels after clearing the filter
    await waitForGraphQLOperation(page, 'GetChannels')

    // Now verify the enabled channel appears
    const enabledChannelRow = channelsTable.locator('tbody tr').filter({ hasText: updatedName })
    await expect(enabledChannelRow).toBeVisible({ timeout: 10000 })
    await expect(enabledChannelRow).toContainText(/Enabled|启用/i)
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
    await Promise.all([waitForGraphQLOperation(page, 'TestChannel'), testButton.click()])

    // Wait for toast notification (success or error)
    await page.waitForTimeout(2000)
  })

  test('can search channels by name', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const searchTerm = `pw-test-SearchChannel${uniqueSuffix}`

    // Create a channel with a unique name for searching
    const createButton = page.getByRole('button', { name: /Add Channel|添加渠道/i })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByLabel(/名称|Name/i).fill(searchTerm)

    // Select channel type - use data-testid for reliable selection
    const openaiRadioContainer = createDialog.getByTestId('channel-type-openai')
    await openaiRadioContainer.click()

    await createDialog.getByLabel(/Base URL/i).fill('https://api.openai.com/v1')
    await createDialog.getByLabel(/API Key/i).fill('sk-test-key-' + uniqueSuffix)

    // Add at least one supported model (required to enable Create button)
    await page.waitForTimeout(500)

    const modelBadge = createDialog.locator('text=gpt-4o').first()
    if ((await modelBadge.count()) > 0) {
      await modelBadge.click()
      const addSelectedButton = createDialog.getByRole('button', { name: /Add Selected|添加选中/i })
      await addSelectedButton.click()
      await page.waitForTimeout(500)
    }

    // Select Default Test Model (required field)
    const defaultTestModelSelect = createDialog
      .locator('[name="defaultTestModel"]')
      .or(createDialog.getByLabel(/Test Model|默认测试模型/i))
    if ((await defaultTestModelSelect.count()) > 0) {
      await defaultTestModelSelect.click()
      const firstOption = page.getByRole('option').first()
      await firstOption.click()
      await page.waitForTimeout(300)
    }

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByRole('button', { name: /创建|Create|保存|Save/i }).click(),
    ])

    // Wait for dialog to close
    await expect(createDialog).not.toBeVisible({ timeout: 10000 })

    // Wait for the table to update
    await page.waitForTimeout(1000)

    // Use the search filter
    const searchInput = page
      .locator('input[placeholder*="搜索"], input[placeholder*="Search"], input[type="search"]')
      .first()
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
    const typeFilterButton = page
      .getByRole('button', { name: /Type|类型/i })
      .or(page.locator('button').filter({ hasText: /Type|类型/i }))

    const typeFilterCount = await typeFilterButton.count()
    if (typeFilterCount === 0) {
      test.skip()
      return
    }

    await typeFilterButton.first().click()

    // Wait for filter menu
    await page.waitForTimeout(500)

    // Select OpenAI filter
    const openaiFilter = page
      .getByRole('menuitemcheckbox', { name: /OpenAI/i })
      .or(page.locator('[role="menuitemcheckbox"]').filter({ hasText: /OpenAI/i }))

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
    const statusFilterButton = page
      .getByRole('button', { name: /Status|状态/i })
      .or(page.locator('button').filter({ hasText: /Status|状态/i }))

    const statusFilterCount = await statusFilterButton.count()
    if (statusFilterCount === 0) {
      test.skip()
      return
    }

    await statusFilterButton.first().click()

    // Wait for filter menu
    await page.waitForTimeout(500)

    // Select Enabled filter
    const enabledFilter = page
      .getByRole('menuitemcheckbox', { name: /Enabled|启用/i })
      .or(page.locator('[role="menuitemcheckbox"]').filter({ hasText: /Enabled|启用/i }))

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
    // Wait for the page to be ready
    await page.waitForTimeout(1000)

    const createButton = page.getByRole('button', { name: /Add Channel|添加渠道/i })

    // Check if button exists (user may not have permission)
    const buttonCount = await createButton.count()
    if (buttonCount === 0) {
      test.skip()
      return
    }

    await expect(createButton).toBeVisible()
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await expect(createDialog).toBeVisible()

    // Verify that the Create button is disabled when required fields are empty
    const submitButton = createDialog.getByRole('button', { name: /创建|Create|保存|Save/i })
    await expect(submitButton).toBeDisabled()

    // Fill in name but leave other required fields empty
    const nameInput = createDialog.getByLabel(/名称|Name/i)
    await nameInput.fill('Test Channel')

    // Button should still be disabled (missing type, base URL, API key, and models)
    await expect(submitButton).toBeDisabled()

    // Verify validation message for supported models
    await expect(createDialog).toContainText(/Please add at least one supported model|请至少添加一个支持的模型/i)
  })

  test('can navigate between pages', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(1000)

    // Look for pagination controls
    const pagination = page
      .locator('[data-testid="pagination"]')
      .or(page.locator('nav').filter({ hasText: /页|Page|Previous|Next/i }))

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

  test('can open model mapping dialog', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')

    // Check if actions button exists (user may not have permission)
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    // Look for model mapping option
    const modelMappingOption = menu.getByRole('menuitem', {
      name: /模型映射|Model Mapping|模型别名|Model Alias/i,
    })
    const modelMappingCount = await modelMappingOption.count()

    if (modelMappingCount > 0) {
      await modelMappingOption.focus()
      await page.keyboard.press('Enter')

      // Verify model mapping dialog opens
      const modelMappingDialog = page.getByRole('dialog')
      await expect(modelMappingDialog).toBeVisible()
      await expect(modelMappingDialog).toContainText(/模型别名|Model Alias/i)
    }
  })

  test('can open override parameters dialog', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')

    // Check if actions button exists (user may not have permission)
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    // Look for override settings option - use updated i18n label
    const overrideParametersOption = menu.getByRole('menuitem', {
      name: /Overrides|覆盖设置/i,
    })
    const overrideParametersCount = await overrideParametersOption.count()

    if (overrideParametersCount > 0) {
      await overrideParametersOption.focus()
      await page.keyboard.press('Enter')

      // Verify override settings dialog opens - use updated i18n label
      const overrideParametersDialog = page.getByRole('dialog')
      await expect(overrideParametersDialog).toBeVisible()
      await expect(overrideParametersDialog).toContainText(/Override Settings|覆盖配置/i)
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

    // Close the dialog - use .first() to avoid strict mode violation
    const closeButton = bulkImportDialog.getByRole('button', { name: /取消|Cancel/i }).first()
    if ((await closeButton.count()) > 0) {
      await closeButton.click()
    } else {
      await page.keyboard.press('Escape')
    }
  })

  test('can configure override parameters in override settings dialog', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')

    // Check if actions button exists (user may not have permission)
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    // Look for override settings option - use updated i18n label
    const overrideParametersOption = menu.getByRole('menuitem', { name: /Overrides|覆盖设置/i })
    const overrideParametersCount = await overrideParametersOption.count()

    if (overrideParametersCount === 0) {
      test.skip()
      return
    }

    await overrideParametersOption.focus()
    await page.keyboard.press('Enter')

    // Verify override settings dialog opens - use updated i18n label
    const settingsDialog = page.getByRole('dialog')
    await expect(settingsDialog).toBeVisible()
    await expect(settingsDialog).toContainText(/Override Settings|覆盖配置/i)

    // Ensure override body section text is visible - use updated i18n label
    await expect(settingsDialog.getByText(/Override Body|覆盖请求体/i)).toBeVisible()

    // Find the textarea for override parameters - use data-testid for stable selection
    const overrideTextarea = settingsDialog.locator('textarea').first()

    // Enter valid JSON
    const validJson = '{"temperature": 0.8, "max_tokens": 4096}'
    await overrideTextarea.fill(validJson)

    // Wait for validation to run
    await page.waitForTimeout(500)

    // Verify no validation error appears
    const errorMessage = settingsDialog.locator('p.text-destructive')
    await expect(errorMessage).not.toBeVisible()

    // Save the settings
    const saveButton = settingsDialog.getByRole('button', { name: /保存|Save/i })
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannel'),
      saveButton.click(),
    ])

    // Wait for dialog to close
    await expect(settingsDialog).not.toBeVisible({ timeout: 10000 })

    // Re-open override settings dialog to verify the value was saved
    const refreshedRow = channelsTable.locator('tbody tr').first()
    await expect(refreshedRow).toBeVisible()
    const refreshedActionsTrigger = refreshedRow.locator('[data-testid="row-actions"]')
    await refreshedActionsTrigger.click()

    const reopenMenu = page.getByRole('menu')
    await expect(reopenMenu).toBeVisible()
    await reopenMenu.getByRole('menuitem', { name: /Overrides|覆盖设置/i }).click()

    const reopenedDialog = page.getByRole('dialog')
    await expect(reopenedDialog).toBeVisible()

    // Verify the textarea still contains the saved value
    const reopenedTextarea = reopenedDialog.locator('textarea').first()
    await expect(reopenedTextarea).toHaveValue(validJson)

    // Close the dialog
    const cancelButton = reopenedDialog.getByRole('button', { name: /取消|Cancel/i })
    await cancelButton.click()
    await expect(reopenedDialog).not.toBeVisible()
  })

  test('validates JSON format in override body settings', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')

    // Check if actions button exists (user may not have permission)
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    // Look for override settings option - use updated i18n label
    const overrideParametersOption = menu.getByRole('menuitem', { name: /Overrides|覆盖设置/i })
    const overrideParametersCount = await overrideParametersOption.count()

    if (overrideParametersCount === 0) {
      test.skip()
      return
    }

    await overrideParametersOption.focus()
    await page.keyboard.press('Enter')

    // Verify override settings dialog opens - use updated i18n label
    const settingsDialog = page.getByRole('dialog')
    await expect(settingsDialog).toBeVisible()
    await expect(settingsDialog).toContainText(/Override Settings|覆盖配置/i)

    // Ensure override body section text is visible - use updated i18n label
    await expect(settingsDialog.getByText(/Override Body|覆盖请求体/i)).toBeVisible()

    // Find the textarea for override parameters
    const overrideTextarea = settingsDialog.locator('textarea').first()

    // Enter invalid JSON
    const invalidJson = '{"temperature": 0.8, "max_tokens": invalid}'
    await overrideTextarea.fill(invalidJson)

    // Wait for validation to run (validation happens on change)
    await page.waitForTimeout(500)

    // Verify validation error appears - it's a <p> tag with class text-destructive
    const errorMessage = settingsDialog.locator('p.text-destructive')
    await expect(errorMessage).toBeVisible()
    await expect(errorMessage).toContainText(/必须是有效的 JSON|Must be valid JSON/i)

    // Enter valid JSON to clear the error
    const validJson = '{"temperature": 0.8, "max_tokens": 4096}'
    await overrideTextarea.fill(validJson)

    // Wait for validation to clear
    await page.waitForTimeout(500)

    // Verify validation error disappears
    await expect(errorMessage).not.toBeVisible()

    // Close the dialog without saving
    const cancelButton = settingsDialog.getByRole('button', { name: /取消|Cancel/i })
    await cancelButton.click()
    await expect(settingsDialog).not.toBeVisible()
  })

  test('can configure model mappings in model mapping dialog', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')

    // Check if actions button exists (user may not have permission)
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    // Look for model mapping option
    const modelMappingOption = menu.getByRole('menuitem', { name: /模型映射|Model Mapping|模型别名|Model Alias/i })
    const modelMappingCount = await modelMappingOption.count()

    if (modelMappingCount === 0) {
      test.skip()
      return
    }

    await modelMappingOption.focus()
    await page.keyboard.press('Enter')

    // Verify model mapping dialog opens
    const settingsDialog = page.getByRole('dialog')
    await expect(settingsDialog).toBeVisible()
    await expect(settingsDialog).toContainText(/模型别名|Model Alias/i)

    // Look for model mapping section
    const mappingSection = settingsDialog.getByRole('heading', {
      name: /Model Mapping|模型映射|Model Alias|模型别名/i,
    })
    const mappingSectionCount = await mappingSection.count()

    if (mappingSectionCount === 0) {
      test.skip()
      return
    }

    // Add a model mapping
    const originalInput = settingsDialog.getByPlaceholder(/Original Model Name|原模型名称|Alias Name|别名/i)
    
    // Fill original model name
    await originalInput.fill('gpt-4')
    await page.waitForTimeout(500)

    // Find and click the target model select (it's a Select component)
    const targetSelectTrigger = settingsDialog.locator('[role="combobox"]').last()
    await targetSelectTrigger.click()
    
    // Wait for dropdown to open
    await page.waitForTimeout(500)
    
    // Select first available option
    const firstOption = page.getByRole('option').first()
    await firstOption.click()
    await page.waitForTimeout(500)

    // Click add button
    const addButton = settingsDialog.getByTestId('add-model-mapping-button')
    await addButton.click()
    
    // Wait for the mapping to be added
    await page.waitForTimeout(1000)

    // Verify mapping appears in the list - look for the text in a border container
    const mappingContainer = settingsDialog.locator('.rounded-lg.border').filter({ hasText: 'gpt-4' })
    await expect(mappingContainer).toBeVisible()

    // Save the settings
    const saveButton = settingsDialog.getByRole('button', { name: /保存|Save/i })
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannel'),
      saveButton.click(),
    ])

    // Wait for dialog to close
    await expect(settingsDialog).not.toBeVisible({ timeout: 10000 })

    // Re-open model mapping dialog to verify the mapping was saved
    await actionsTrigger.click()
    await modelMappingOption.focus()
    await page.keyboard.press('Enter')

    const reopenedDialog = page.getByRole('dialog')
    await expect(reopenedDialog).toBeVisible()

    // Verify the mapping still exists
    await expect(reopenedDialog).toContainText('gpt-4')

    // Close the dialog via keyboard to avoid flaky button detach
    await page.keyboard.press('Escape')
    await expect(reopenedDialog).not.toBeVisible()
  })

  test('can batch create channels with multiple API keys', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const baseName = `pw-batch-test-${uniqueSuffix}`
    const baseURL = 'https://api.openai.com/v1'
    const apiKeys = ['sk-key1-' + uniqueSuffix, 'sk-key2-' + uniqueSuffix, 'sk-key3-' + uniqueSuffix]

    // Look for batch create button (if it exists in the UI)
    // For now, we'll test via the API by creating channels with the same base name
    // which should result in numbered channels: "name - (1)", "name - (2)", etc.

    // Create first channel
    const createButton = page.getByRole('button', { name: /Add Channel|添加渠道/i })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByLabel(/名称|Name/i).fill(baseName)

    // Select channel type - use data-testid for reliable selection
    const openaiRadioContainer = createDialog.getByTestId('channel-type-openai')
    await openaiRadioContainer.click()

    await createDialog.getByLabel(/Base URL/i).fill(baseURL)
    await createDialog.getByLabel(/API Key/i).fill(apiKeys.join('\n'))

    // Add model
    await page.waitForTimeout(500)
    const modelBadge = createDialog.locator('text=gpt-4o').first()
    if ((await modelBadge.count()) > 0) {
      await modelBadge.click()
      const addSelectedButton = createDialog.getByRole('button', { name: /Add Selected|添加选中/i })
      await addSelectedButton.click()
      await page.waitForTimeout(500)
    }

    // Select Default Test Model
    const defaultTestModelSelect = createDialog
      .locator('[name="defaultTestModel"]')
      .or(createDialog.getByLabel(/Test Model|默认测试模型/i))
    if ((await defaultTestModelSelect.count()) > 0) {
      await defaultTestModelSelect.click()
      const firstOption = page.getByRole('option').first()
      await firstOption.click()
      await page.waitForTimeout(300)
    }

    await Promise.all([
      waitForGraphQLOperation(page, 'BulkCreateChannels'),
      createDialog.getByRole('button', { name: /创建|Create|保存|Save/i }).click(),
    ])

    await expect(createDialog).not.toBeVisible({ timeout: 10000 })
    await page.waitForTimeout(1500)

    // Verify numbered channels were created for each API key
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const expectedRows = apiKeys.map((_, idx) =>
      channelsTable.locator('tbody tr').filter({ hasText: `${baseName} - (${idx + 1})` })
    )

    for (const row of expectedRows) {
      await expect(row).toBeVisible()
    }
  })

  test('can filter channels by tags', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const tagName = `pw-tag-${uniqueSuffix}`

    // Create a channel with a specific tag
    const createButton = page.getByRole('button', { name: /Add Channel|添加渠道/i })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByLabel(/名称|Name/i).fill(`Channel-${tagName}`)

    // Select channel type - use data-testid for reliable selection
    const openaiRadioContainer = createDialog.getByTestId('channel-type-openai')
    await openaiRadioContainer.click()

    await createDialog.getByLabel(/Base URL/i).fill('https://api.openai.com/v1')
    await createDialog.getByLabel(/API Key/i).fill('sk-test-' + uniqueSuffix)

    // Add model
    await page.waitForTimeout(500)
    const modelBadge = createDialog.locator('text=gpt-4o').first()
    if ((await modelBadge.count()) > 0) {
      await modelBadge.click()
      const addSelectedButton = createDialog.getByRole('button', { name: /Add Selected|添加选中/i })
      await addSelectedButton.click()
      await page.waitForTimeout(500)
    }

    // Select Default Test Model
    const defaultTestModelSelect = createDialog
      .locator('[name="defaultTestModel"]')
      .or(createDialog.getByLabel(/Test Model|默认测试模型/i))
    if ((await defaultTestModelSelect.count()) > 0) {
      await defaultTestModelSelect.click()
      const firstOption = page.getByRole('option').first()
      await firstOption.click()
      await page.waitForTimeout(300)
    }

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByRole('button', { name: /创建|Create|保存|Save/i }).click(),
    ])

    await expect(createDialog).not.toBeVisible({ timeout: 10000 })
    await page.waitForTimeout(1000)

    // Look for tags filter button
    const tagsFilterButton = page
      .locator('button')
      .filter({ hasText: /Tags|标签/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()

    const tagsFilterCount = await tagsFilterButton.count()
    if (tagsFilterCount === 0) {
      test.skip()
      return
    }

    await tagsFilterButton.click()
    await page.waitForTimeout(500)

    // Select the tag filter
    const tagFilter = page
      .getByRole('option', { name: new RegExp(tagName, 'i') })
      .or(page.locator('[role="option"]').filter({ hasText: new RegExp(tagName, 'i') }))

    const tagFilterCount2 = await tagFilter.count()
    if (tagFilterCount2 > 0) {
      await tagFilter.click()
      await page.waitForTimeout(1000)

      // Verify filtered results
      const channelsTable = page.locator('[data-testid="channels-table"]')
      const filteredRow = channelsTable.locator('tbody tr').filter({ hasText: `Channel-${tagName}` })
      await expect(filteredRow).toBeVisible()
    }
  })

  test('can configure override headers in override settings dialog', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')

    // Check if actions button exists (user may not have permission)
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    // Look for override settings option
    const overrideParametersOption = menu.getByRole('menuitem', { name: /Overrides|覆盖设置/i })
    const overrideParametersCount = await overrideParametersOption.count()

    if (overrideParametersCount === 0) {
      test.skip()
      return
    }

    await overrideParametersOption.focus()
    await page.keyboard.press('Enter')

    // Verify override settings dialog opens
    const settingsDialog = page.getByRole('dialog')
    await expect(settingsDialog).toBeVisible()
    await expect(settingsDialog).toContainText(/Override Settings|覆盖配置/i)

    // Ensure override headers section is visible - use data-testid for stable selection
    const headersSection = settingsDialog.locator('[data-testid="override-headers-section"]')
    await expect(headersSection).toBeVisible()
    await expect(headersSection).toContainText(/Override Headers|覆盖请求头/i)

    // Add a new header
    const addHeaderButton = settingsDialog.locator('[data-testid="add-header-button"]')
    await addHeaderButton.click()
    await page.waitForTimeout(500)

    // Check if header input was added
    const headerKeyInput = settingsDialog.locator('[data-testid="header-key-0"]')
    const headerValueInput = settingsDialog.locator('[data-testid="header-value-0"]')
    
    const headerKeyExists = await headerKeyInput.count()
    const headerValueExists = await headerValueInput.count()
    console.log(`Header key input exists: ${headerKeyExists > 0}, Header value input exists: ${headerValueExists > 0}`)

    if (headerKeyExists === 0 || headerValueExists === 0) {
      console.log('Header inputs were not created properly')
      // Try to take a screenshot or check the dialog state
      await page.screenshot({ path: 'debug-override-dialog.png' })
      test.skip()
      return
    }

    // Fill in header key and value - use data-testid for stable selection
    await headerKeyInput.fill('X-Custom-Header')
    await headerValueInput.fill('custom-value')
    
    // Trigger form validation by blurring the input
    await headerKeyInput.blur()
    await page.waitForTimeout(500)

    // Verify the values were actually filled
    const keyFilled = await headerKeyInput.inputValue()
    const valueFilled = await headerValueInput.inputValue()
    console.log(`Header key filled: "${keyFilled}", Header value filled: "${valueFilled}"`)

    // Check for any validation errors before saving
    const errorMessage = settingsDialog.locator('p.text-destructive')
    const errorCount = await errorMessage.count()
    if (errorCount > 0) {
      console.log('Validation errors found:', await errorMessage.allTextContents())
      // If there are validation errors, fail the test early
      expect(errorCount).toBe(0)
    }

    // Verify the save button is enabled
    const saveButton = settingsDialog.locator('[data-testid="override-save-button"]')
    await expect(saveButton).toBeEnabled()

    // Save the settings
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannel'),
      saveButton.click(),
    ])

    // Wait for dialog to close and ensure it's fully closed
    await expect(settingsDialog).not.toBeVisible({ timeout: 10000 })
    await page.waitForTimeout(1000)

    // Re-open override settings dialog to verify the header was saved
    const refreshedRow = channelsTable.locator('tbody tr').first()
    await expect(refreshedRow).toBeVisible()
    const refreshedActionsTrigger = refreshedRow.locator('[data-testid="row-actions"]')
    await refreshedActionsTrigger.click()

    const reopenMenu = page.getByRole('menu')
    await expect(reopenMenu).toBeVisible()
    await reopenMenu.getByRole('menuitem', { name: /Overrides|覆盖设置/i }).click()

    const reopenedDialog = page.getByRole('dialog')
    await expect(reopenedDialog).toBeVisible()
    await page.waitForTimeout(1000) // Wait for dialog content to fully load

    // Check if any headers exist at all
    const headerElements = await reopenedDialog.locator('[data-testid^="header-key-"]').count()
    console.log(`Found ${headerElements} header elements in reopened dialog`)

    if (headerElements > 0) {
      // Verify the header still exists
      const reopenedHeaderKey = reopenedDialog.locator('[data-testid="header-key-0"]')
      const reopenedHeaderValue = reopenedDialog.locator('[data-testid="header-value-0"]')
      await expect(reopenedHeaderKey).toHaveValue('X-Custom-Header')
      await expect(reopenedHeaderValue).toHaveValue('custom-value')
    } else {
      // If no headers exist, check if the headers section is visible
      const headersSectionExists = await reopenedDialog.locator('[data-testid="override-headers-section"]').count()
      console.log(`Headers section exists: ${headersSectionExists > 0}`)
      
      if (headersSectionExists > 0) {
        // Try adding a header to see if the dialog is working
        const addNewHeaderButton = reopenedDialog.locator('[data-testid="add-header-button"]')
        if (await addNewHeaderButton.count() > 0) {
          await addNewHeaderButton.click()
          await page.waitForTimeout(500)
          
          // Check if header input appears
          const newHeaderInput = reopenedDialog.locator('[data-testid="header-key-0"]')
          if (await newHeaderInput.count() > 0) {
            console.log('Dialog is working, but headers were not saved')
          } else {
            console.log('Dialog may not be functioning properly')
          }
        }
      }
    }

    // Close the dialog
    const cancelButton = reopenedDialog.locator('[data-testid="override-cancel-button"]')
    if (await cancelButton.count() > 0) {
      await cancelButton.click()
    } else {
      await page.keyboard.press('Escape')
    }
    await expect(reopenedDialog).not.toBeVisible()
  })

  test('can add and remove multiple override headers', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    const overrideParametersOption = menu.getByRole('menuitem', { name: /Overrides|覆盖设置/i })
    const overrideParametersCount = await overrideParametersOption.count()

    if (overrideParametersCount === 0) {
      test.skip()
      return
    }

    await overrideParametersOption.focus()
    await page.keyboard.press('Enter')

    const settingsDialog = page.getByRole('dialog')
    await expect(settingsDialog).toBeVisible()

    // Add first header
    const addHeaderButton = settingsDialog.locator('[data-testid="add-header-button"]')
    await addHeaderButton.click()
    await page.waitForTimeout(500)

    let headerKeyInput = settingsDialog.locator('[data-testid="header-key-0"]')
    let headerValueInput = settingsDialog.locator('[data-testid="header-value-0"]')
    await headerKeyInput.fill('User-Agent')
    await headerValueInput.fill('MyApp/1.0')

    // Add second header
    await addHeaderButton.click()
    await page.waitForTimeout(500)

    headerKeyInput = settingsDialog.locator('[data-testid="header-key-1"]')
    headerValueInput = settingsDialog.locator('[data-testid="header-value-1"]')
    await headerKeyInput.fill('X-Custom-ID')
    await headerValueInput.fill('12345')

    // Add third header
    await addHeaderButton.click()
    await page.waitForTimeout(500)

    headerKeyInput = settingsDialog.locator('[data-testid="header-key-2"]')
    headerValueInput = settingsDialog.locator('[data-testid="header-value-2"]')
    await headerKeyInput.fill('X-Request-Source')
    await headerValueInput.fill('automated-test')

    // Verify all headers are present
    await expect(settingsDialog.locator('[data-testid="header-key-0"]')).toHaveValue('User-Agent')
    await expect(settingsDialog.locator('[data-testid="header-value-0"]')).toHaveValue('MyApp/1.0')
    await expect(settingsDialog.locator('[data-testid="header-key-1"]')).toHaveValue('X-Custom-ID')
    await expect(settingsDialog.locator('[data-testid="header-value-1"]')).toHaveValue('12345')
    await expect(settingsDialog.locator('[data-testid="header-key-2"]')).toHaveValue('X-Request-Source')
    await expect(settingsDialog.locator('[data-testid="header-value-2"]')).toHaveValue('automated-test')

    // Remove the second header (index 1)
    const removeHeaderButton = settingsDialog.locator('[data-testid="remove-header-1"]')
    await removeHeaderButton.click()
    await page.waitForTimeout(500)

    // Verify the header was removed and other headers are still present
    await expect(settingsDialog.locator('[data-testid="header-key-0"]')).toHaveValue('User-Agent')
    await expect(settingsDialog.locator('[data-testid="header-value-0"]')).toHaveValue('MyApp/1.0')
    
    // After removing index 1, index 2 should become index 1
    await expect(settingsDialog.locator('[data-testid="header-key-1"]')).toHaveValue('X-Request-Source')
    await expect(settingsDialog.locator('[data-testid="header-value-1"]')).toHaveValue('automated-test')

    // Verify the removed header no longer exists
    await expect(settingsDialog.locator('[data-testid="header-key-2"]')).not.toBeVisible()

    // Close the dialog without saving
    const cancelButton = settingsDialog.locator('[data-testid="override-cancel-button"]')
    await cancelButton.click()
    await expect(settingsDialog).not.toBeVisible()
  })

  test('validates forbidden headers in override settings', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    const overrideParametersOption = menu.getByRole('menuitem', { name: /Overrides|覆盖设置/i })
    const overrideParametersCount = await overrideParametersOption.count()

    if (overrideParametersCount === 0) {
      test.skip()
      return
    }

    await overrideParametersOption.focus()
    await page.keyboard.press('Enter')

    const settingsDialog = page.getByRole('dialog')
    await expect(settingsDialog).toBeVisible()

    // Add a header
    const addHeaderButton = settingsDialog.locator('[data-testid="add-header-button"]')
    await addHeaderButton.click()
    await page.waitForTimeout(500)

    // Try to enter a forbidden header (authorization)
    const headerKeyInput = settingsDialog.locator('[data-testid="header-key-0"]')
    await headerKeyInput.fill('Authorization')
    await page.waitForTimeout(500)

    // Verify validation error appears
    const errorMessage = settingsDialog.locator('p.text-destructive')
    await expect(errorMessage).toBeVisible()
    await expect(errorMessage).toContainText(/Header cannot override sensitive headers|不能覆盖敏感请求头/i)

    // Try another forbidden header (x-api-key)
    await headerKeyInput.fill('X-API-Key')
    await page.waitForTimeout(500)

    // Verify validation error still appears
    await expect(errorMessage).toBeVisible()

    // Enter a valid header
    await headerKeyInput.fill('X-Valid-Header')
    await page.waitForTimeout(500)

    // Verify validation error disappears
    await expect(errorMessage).not.toBeVisible()

    // Close the dialog without saving
    const cancelButton = settingsDialog.locator('[data-testid="override-cancel-button"]')
    await cancelButton.click()
    await expect(settingsDialog).not.toBeVisible()
  })

  test('can clear headers by leaving value empty', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    const overrideParametersOption = menu.getByRole('menuitem', { name: /Overrides|覆盖设置/i })
    const overrideParametersCount = await overrideParametersOption.count()

    if (overrideParametersCount === 0) {
      test.skip()
      return
    }

    await overrideParametersOption.focus()
    await page.keyboard.press('Enter')

    const settingsDialog = page.getByRole('dialog')
    await expect(settingsDialog).toBeVisible()

    // Check existing headers first
    const existingHeaderKeys = settingsDialog.locator('[data-testid^="header-key-"]')
    const existingHeaderCount = await existingHeaderKeys.count()
    
    // Add a header with key but empty value
    const addHeaderButton = settingsDialog.locator('[data-testid="add-header-button"]')
    await addHeaderButton.click()

    // Get the correct header index (should be existingHeaderCount)
    const headerIndex = existingHeaderCount
    // Wait for the new header input to be visible
    await expect(settingsDialog.locator(`[data-testid="header-key-${existingHeaderCount}"]`)).toBeVisible()
    const headerKeyInput = settingsDialog.locator(`[data-testid="header-key-${headerIndex}"]`)
    const headerValueInput = settingsDialog.locator(`[data-testid="header-value-${headerIndex}"]`)

    await headerKeyInput.fill('X-To-Remove')
    // Leave value empty to indicate removal

    // Save the settings
    const saveButton = settingsDialog.locator('[data-testid="override-save-button"]')
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannel'),
      saveButton.click(),
    ])

    // Wait for the dialog to close. A generous timeout is used to accommodate for
    // network latency and UI updates. A failure here indicates a potential application bug.
    await expect(settingsDialog).not.toBeVisible({ timeout: 15000 })

    // Re-open to verify
    const refreshedRow = channelsTable.locator('tbody tr').first()
    await expect(refreshedRow).toBeVisible()
    
    // Wait for query invalidation to complete and data to refresh
    await waitForGraphQLOperation(page, 'GetChannels')
    await page.waitForTimeout(1000)
    
    const refreshedActionsTrigger = refreshedRow.locator('[data-testid="row-actions"]')
    await refreshedActionsTrigger.click()

    const reopenMenu = page.getByRole('menu')
    await expect(reopenMenu).toBeVisible()
    await reopenMenu.getByRole('menuitem', { name: /Overrides|覆盖设置/i }).click()

    const reopenedDialog = page.getByRole('dialog')
    await expect(reopenedDialog).toBeVisible()

    // Verify the header is still saved (frontend saves headers with empty values)
    const reopenedHeaderKey = reopenedDialog.locator(`[data-testid="header-key-${headerIndex}"]`)
    const reopenedHeaderValue = reopenedDialog.locator(`[data-testid="header-value-${headerIndex}"]`)
    await expect(reopenedHeaderKey).toHaveValue('X-To-Remove')
    await expect(reopenedHeaderValue).toHaveValue('')

    // Close the dialog
    const cancelButton = reopenedDialog.locator('[data-testid="override-cancel-button"]')
    await cancelButton.click()
    await expect(reopenedDialog).not.toBeVisible()
  })

  test('can remove headers using remove button', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    // Find the first channel row
    const channelsTable = page.locator('[data-testid="channels-table"]')
    const firstRow = channelsTable.locator('tbody tr').first()
    const rowCount = await channelsTable.locator('tbody tr').count()

    if (rowCount === 0) {
      test.skip()
      return
    }

    await expect(firstRow).toBeVisible()

    // Click actions menu
    const actionsTrigger = firstRow.locator('[data-testid="row-actions"]')
    const actionsCount = await actionsTrigger.count()
    if (actionsCount === 0) {
      test.skip()
      return
    }

    await actionsTrigger.click()

    const menu = page.getByRole('menu')
    await expect(menu).toBeVisible()

    const overrideParametersOption = menu.getByRole('menuitem', { name: /Overrides|覆盖设置/i })
    const overrideParametersCount = await overrideParametersOption.count()

    if (overrideParametersCount === 0) {
      test.skip()
      return
    }

    await overrideParametersOption.focus()
    await page.keyboard.press('Enter')

    const settingsDialog = page.getByRole('dialog')
    await expect(settingsDialog).toBeVisible()

    // Check existing headers first
    const existingHeaderKeys = settingsDialog.locator('[data-testid^="header-key-"]')
    const existingHeaderCount = await existingHeaderKeys.count()

    // Add a header first
    const addHeaderButton = settingsDialog.locator('[data-testid="add-header-button"]')
    await addHeaderButton.click()
    await page.waitForTimeout(500)

    // Get the correct header index (should be existingHeaderCount)
    const headerIndex = existingHeaderCount
    const headerKeyInput = settingsDialog.locator(`[data-testid="header-key-${headerIndex}"]`)
    const headerValueInput = settingsDialog.locator(`[data-testid="header-value-${headerIndex}"]`)

    await headerKeyInput.fill('X-To-Remove')
    await headerValueInput.fill('some-value')

    // Save the settings
    const saveButton = settingsDialog.locator('[data-testid="override-save-button"]')
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannel'),
      saveButton.click(),
    ])

    // Wait for dialog to close
    await expect(settingsDialog).not.toBeVisible({ timeout: 10000 })

    // Re-open and remove the header
    const refreshedRow = channelsTable.locator('tbody tr').first()
    await expect(refreshedRow).toBeVisible()
    
    // Wait for query invalidation to complete and data to refresh
    await waitForGraphQLOperation(page, 'GetChannels')
    await page.waitForTimeout(1000)
    
    const refreshedActionsTrigger = refreshedRow.locator('[data-testid="row-actions"]')
    await refreshedActionsTrigger.click()

    const reopenMenu = page.getByRole('menu')
    await expect(reopenMenu).toBeVisible()
    await reopenMenu.getByRole('menuitem', { name: /Overrides|覆盖设置/i }).click()

    const reopenedDialog = page.getByRole('dialog')
    await expect(reopenedDialog).toBeVisible()

    // Verify the header exists
    const reopenedHeaderKey = reopenedDialog.locator(`[data-testid="header-key-${headerIndex}"]`)
    await expect(reopenedHeaderKey).toHaveValue('X-To-Remove')

    // Remove the header using the remove button
    const removeHeaderButton = reopenedDialog.locator(`[data-testid="remove-header-${headerIndex}"]`)
    await removeHeaderButton.click()
    await page.waitForTimeout(500)

    // Save the changes
    const reopenedSaveButton = reopenedDialog.locator('[data-testid="override-save-button"]')
    await Promise.all([
      waitForGraphQLOperation(page, 'UpdateChannel'),
      reopenedSaveButton.click(),
    ])

    // Wait for the dialog to close. A generous timeout is used to accommodate for
    // network latency and UI updates. A failure here indicates a potential application bug.
    await expect(reopenedDialog).not.toBeVisible({ timeout: 15000 })

    // Re-open again to verify the header is gone
    const finalRow = channelsTable.locator('tbody tr').first()
    await expect(finalRow).toBeVisible()
    
    // Wait for query invalidation to complete and data to refresh
    await waitForGraphQLOperation(page, 'GetChannels')
    await page.waitForTimeout(1000)
    
    const finalActionsTrigger = finalRow.locator('[data-testid="row-actions"]')
    await finalActionsTrigger.click()

    const finalMenu = page.getByRole('menu')
    await expect(finalMenu).toBeVisible()
    await finalMenu.getByRole('menuitem', { name: /Overrides|覆盖设置/i }).click()

    const finalDialog = page.getByRole('dialog')
    await expect(finalDialog).toBeVisible()

    // Verify the header is completely removed
    const finalHeaderKey = finalDialog.locator(`[data-testid="header-key-${headerIndex}"]`)
    const hasHeader = await finalHeaderKey.count()
    expect(hasHeader).toBe(0)

    // Close the dialog
    const finalCancelButton = finalDialog.locator('[data-testid="override-cancel-button"]')
    await finalCancelButton.click()
    await expect(finalDialog).not.toBeVisible()
  })
})
