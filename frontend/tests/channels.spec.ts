import { test, expect, type Locator, type Page, type Request } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

test.describe('Admin Channels Management', () => {
  test.beforeEach(async ({ page }) => {
    // Increase timeout for authentication
    test.setTimeout(60000)
    await gotoAndEnsureAuth(page, '/channels')

    // Wait for page to fully load and channels table to appear
    await page.waitForTimeout(2000)
    
    // Wait for the channels table to be visible (indicates page is loaded)
    const channelsTable = page.locator('[data-testid="channels-table"]')
    await channelsTable.waitFor({ state: 'visible', timeout: 15000 })
  })

  test('can create, edit, and archive a channel', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-test-Channel ${uniqueSuffix}`
    const baseURL = `https://api.test-${uniqueSuffix}.example.com`

    // Step 1: Create a new channel
    // Wait for the add button to appear (requires write_channels permission)
    const createButton = page.getByTestId('add-channel-button')
    await expect(createButton).toBeVisible({ timeout: 10000 })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await expect(createDialog).toBeVisible()
    await expect(createDialog).toContainText(/创建|Create/i)

    // Fill in channel details
    await createDialog.getByTestId('channel-name-input').fill(name)

    // Select provider (OpenAI) - use data-testid for reliable selection
    const openaiProviderRadio = createDialog.getByTestId('provider-openai')
    await openaiProviderRadio.click()

    // Fill in base URL
    await createDialog.getByTestId('channel-base-url-input').fill(baseURL)

    // Fill in API Key
    const apiKeyInput = createDialog.getByTestId('channel-api-key-input')
    await apiKeyInput.fill('  sk-test-key-' + uniqueSuffix + '  ')
    await createDialog.getByTestId('channel-name-input').click()
    await expect(createDialog.getByTestId('channel-name-input')).toBeFocused()

    // Add at least one supported model (required to enable Create button)
    // Wait for Quick Add Models section to appear and click on gpt-4o badge
    const modelBadge = createDialog.getByTestId('quick-model-gpt-4o')
    await expect(modelBadge).toBeVisible({ timeout: 5000 })
    await modelBadge.click()
    // Wait for selection state to update
    await page.waitForTimeout(300)

    // Click "Add Selected" button to add the selected models
    const addSelectedButton = createDialog.getByTestId('add-selected-models-button')
    await expect(addSelectedButton).toBeEnabled({ timeout: 5000 })
    await addSelectedButton.click()
    await page.waitForTimeout(500)

    // Select Default Test Model (required field)
    const defaultTestModelSelect = createDialog.getByTestId('default-test-model-select')
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
      createDialog.getByTestId('channel-submit-button').click(),
    ])

    // Wait for dialog to close
    await expect(createDialog).not.toBeVisible({ timeout: 10000 })

    // Verify channel appears in the table
    await page.waitForTimeout(1000)
    const channelsTable = page.locator('[data-testid="channels-table"]')

    // New channels are created with 'disabled' status by default.
    // If an active status filter excludes disabled channels (e.g. "Enabled" is pre-selected),
    // clear the filter so the new channel is visible.
    const statusBtn = page
      .locator('button')
      .filter({ hasText: /Status|状态/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()
    const statusBtnText = await statusBtn.textContent()
    if (statusBtnText && /Enabled|启用/i.test(statusBtnText) && !/Disabled|禁用/i.test(statusBtnText)) {
      await statusBtn.click()
      await page.waitForTimeout(500)
      const disabledOpt = page
        .getByRole('option', { name: /Disabled|禁用/i })
        .or(page.locator('[role="option"]').filter({ hasText: /Disabled|禁用/i }))
      if ((await disabledOpt.count()) > 0) {
        await disabledOpt.first().click()
        await page.waitForTimeout(500)
      }
      await page.keyboard.press('Escape')
      await page.waitForTimeout(500)
    }

    const channelRow = channelsTable.locator('tbody tr').filter({ hasText: name })
    await expect(channelRow).toBeVisible()
    const statusSwitch = channelRow.locator('[data-testid="channel-status-switch"]')
    await expect(statusSwitch).toBeVisible()
    await expect(statusSwitch).not.toBeChecked()

    // Step 2: Edit the channel (click the dedicated edit button in the actions cell)
    const editButton = channelRow.locator('td:last-child button').first()
    await editButton.click()

    const editDialog = page.getByRole('dialog', { name: /编辑|Edit Channel/i })
    await expect(editDialog).toBeVisible()
    await expect(editDialog).toContainText(/编辑|Edit Channel/i)

    // Update the name
    const updatedName = `${name} - Updated`
    const nameInput = editDialog.getByTestId('channel-name-input')
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
    await archiveMenu.getByRole('menuitem', { name: /归档|Archive/i }).click()

    const archiveDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
    await expect(archiveDialog).toBeVisible()
    await expect(archiveDialog).toContainText(/归档|Archive/i)

    // Wait for dialog to stabilize
    await page.waitForTimeout(500)

    // Click the confirm button - it's the last button (first is Cancel)
    const archiveButton = archiveDialog.getByRole('button', { name: /归档|Archive/i }).last()
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

    // Now verify the archived channel appears - switch should be disabled for archived channels
    const archivedChannelRow = channelsTable.locator('tbody tr').filter({ hasText: updatedName })
    await expect(archivedChannelRow).toBeVisible()
    const archivedSwitch = archivedChannelRow.locator('[data-testid="channel-status-switch"]')
    await expect(archivedSwitch).toBeDisabled()

    // Archived channels have their switch disabled — no Enable action in the menu.
    // The test verifies archive was successful (switch disabled) above.
  })

  test('preserves a third-party Codex base URL when creating a channel', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-test-CodexChannel-${uniqueSuffix}`
    const baseURL = `https://codex-${uniqueSuffix}.example.com/v1`

    await page.getByTestId('add-channel-button').click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByTestId('provider-codex').click()
    await createDialog.getByRole('tab', { name: THIRD_PARTY_TAB }).click()
    await createDialog.getByTestId('channel-name-input').fill(name)
    await createDialog.getByTestId('channel-base-url-input').fill(baseURL)
    await createDialog.getByTestId('channel-api-key-input').fill(`sk-test-key-${uniqueSuffix}`)

    const modelBadge = createDialog.getByTestId(`quick-model-${CODEX_MODEL}`)
    await expect(modelBadge).toBeVisible({ timeout: 5000 })
    await modelBadge.click()
    await createDialog.getByTestId('add-selected-models-button').click()

    const defaultTestModelSelect = createDialog.getByTestId('default-test-model-select')
    await expect(defaultTestModelSelect).toBeVisible({ timeout: 5000 })
    await defaultTestModelSelect.click()
    await page.getByRole('option', { name: CODEX_MODEL, exact: true }).click()

    const createRequest = page.waitForRequest((request) => {
      if (!request.url().includes('/admin/graphql') || request.method() !== 'POST') return false
      return request.postDataJSON()?.operationName === 'CreateChannel'
    })

    await Promise.all([
      createRequest,
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByTestId('channel-submit-button').click(),
    ])

    const payload = (await createRequest).postDataJSON() as { variables: { input: { baseURL: string } } }
    expect(payload.variables.input.baseURL).toBe(baseURL)
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
    const createButton = page.getByTestId('add-channel-button')
    await expect(createButton).toBeVisible({ timeout: 10000 })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByTestId('channel-name-input').fill(searchTerm)

    // Select provider - use data-testid for reliable selection
    const openaiProviderRadio = createDialog.getByTestId('provider-openai')
    await openaiProviderRadio.click()

    await createDialog.getByTestId('channel-base-url-input').fill('https://api.openai.com/v1')
    await createDialog.getByTestId('channel-api-key-input').fill('sk-test-key-' + uniqueSuffix)

    // Add at least one supported model (required to enable Create button)
    // Wait for model badge to appear and click it
    const modelBadge = createDialog.getByTestId('quick-model-gpt-4o')
    await expect(modelBadge).toBeVisible({ timeout: 5000 })
    await modelBadge.click()
    await page.waitForTimeout(300)
    
    const addSelectedButton = createDialog.getByTestId('add-selected-models-button')
    await expect(addSelectedButton).toBeEnabled({ timeout: 5000 })
    await addSelectedButton.click()
    await page.waitForTimeout(500)

    // Select Default Test Model (required field)
    const defaultTestModelSelect = createDialog.getByTestId('default-test-model-select')
    await expect(defaultTestModelSelect).toBeVisible({ timeout: 5000 })
    await defaultTestModelSelect.click()
    const firstOption = page.getByRole('option').first()
    await firstOption.click()
    await page.waitForTimeout(300)

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByTestId('channel-submit-button').click(),
    ])

    // Wait for dialog to close
    await expect(createDialog).not.toBeVisible({ timeout: 10000 })

    // Wait for the table to update
    await page.waitForTimeout(1000)

    // Newly created channels are disabled by default.
    // If an active status filter excludes disabled channels, include them.
    const statusBtn = page
      .locator('button')
      .filter({ hasText: /Status|状态/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()
    const statusBtnText = await statusBtn.textContent()
    if (statusBtnText && /Enabled|启用/i.test(statusBtnText) && !/Disabled|禁用/i.test(statusBtnText)) {
      await statusBtn.click()
      await page.waitForTimeout(500)
      const disabledOpt = page
        .getByRole('option', { name: /Disabled|禁用/i })
        .or(page.locator('[role="option"]').filter({ hasText: /Disabled|禁用/i }))
      if ((await disabledOpt.count()) > 0) {
        await disabledOpt.first().click()
        await page.waitForTimeout(500)
      }
      await page.keyboard.press('Escape')
      await page.waitForTimeout(500)
    }

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

    // Select OpenAI filter - it's a CommandItem with role="option"
    const openaiFilter = page
      .getByRole('option', { name: /OpenAI/i })
      .or(page.locator('[role="option"]').filter({ hasText: /OpenAI/i }))

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
    // The button text may include the selected filter value (e.g. "Status Enabled"),
    // so we match on the base text "Status" or "状态"
    const statusFilterButton = page
      .locator('button')
      .filter({ hasText: /Status|状态/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()

    const statusFilterCount = await statusFilterButton.count()
    if (statusFilterCount === 0) {
      test.skip()
      return
    }

    await statusFilterButton.click()

    // Wait for filter menu
    await page.waitForTimeout(500)

    // "Enabled" may already be pre-selected as the default filter, so clicking it
    // would deselect it instead of applying it. Use "Disabled" to test filtering,
    // which is not pre-selected.
    const disabledFilter = page
      .getByRole('option', { name: /Disabled|禁用/i })
      .or(page.locator('[role="option"]').filter({ hasText: /Disabled|禁用/i }))

    const disabledFilterCount = await disabledFilter.count()
    if (disabledFilterCount > 0) {
      await disabledFilter.first().click()

      // Wait for filter to apply
      await page.waitForTimeout(1000)

      // Verify filtered results - disabled channels have unchecked switches
      const rows = page.locator('tbody tr')
      const rowCount = await rows.count()

      // Skip assertion if no disabled channels exist after filtering
      if (rowCount === 0) {
        return
      }

      const firstRow = rows.first()
      const statusSwitch = firstRow.locator('[data-testid="channel-status-switch"]')
      await expect(statusSwitch).toBeVisible({ timeout: 5000 })
      await expect(statusSwitch).not.toBeChecked({ timeout: 5000 })
    }
  })

  test('validates required fields when creating a channel', async ({ page }) => {
    // Wait for the page to be ready
    await page.waitForTimeout(1000)

    const createButton = page.getByTestId('add-channel-button')

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
    const submitButton = createDialog.getByTestId('channel-submit-button')
    await expect(submitButton).toBeDisabled()

    // Fill in name but leave other required fields empty
    const nameInput = createDialog.getByTestId('channel-name-input')
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

    // Look for model mapping option - uses i18n key channels.dialogs.settings.modelMapping.title
    const modelMappingOption = menu.getByRole('menuitem', {
      name: /模型映射|Model Mapping|模型别名|Model Alias/i,
    })
    const modelMappingCount = await modelMappingOption.count()

    if (modelMappingCount > 0) {
      await modelMappingOption.click()

      // Verify model mapping dialog opens
      const modelMappingDialog = page.getByRole('dialog')
      await expect(modelMappingDialog).toBeVisible()
      await expect(modelMappingDialog).toContainText(/模型映射|Model Mapping|模型别名|Model Alias/i)
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

    // Look for override settings option - uses i18n key channels.dialogs.settings.overrides.action
    const overrideParametersOption = menu.getByRole('menuitem', {
      name: /Overrides|覆盖设置/i,
    })
    const overrideParametersCount = await overrideParametersOption.count()

    if (overrideParametersCount > 0) {
      await overrideParametersOption.click()

      // Verify override settings dialog opens
      const overrideParametersDialog = page.getByRole('dialog')
      await expect(overrideParametersDialog).toBeVisible()
      await expect(overrideParametersDialog).toContainText(/Override Settings|覆盖配置|覆盖设置/i)
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

  test('can configure model mappings in model mapping dialog', async ({ page }) => {
    // Wait for table to load
    await page.waitForTimeout(2000)

    const uniqueSuffix = Date.now().toString().slice(-6)
    const aliasName = `pw-alias-${uniqueSuffix}`

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

    await modelMappingOption.click()

    // Verify model mapping dialog opens
    const settingsDialog = page.getByRole('dialog')
    await expect(settingsDialog).toBeVisible()
    await expect(settingsDialog).toContainText(/模型映射|Model Mapping|模型别名|Model Alias/i)

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
    await originalInput.fill(aliasName)
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
    const mappingContainer = settingsDialog.locator('.rounded-lg.border').filter({ hasText: aliasName })
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
    const reopenMenu = page.getByRole('menu')
    await expect(reopenMenu).toBeVisible({ timeout: 5000 })
    await modelMappingOption.click()

    const reopenedDialog = page.getByRole('dialog')
    await expect(reopenedDialog).toBeVisible({ timeout: 5000 })

    // Verify the mapping still exists
    await expect(reopenedDialog).toContainText(aliasName)

    // Close the dialog via keyboard to avoid flaky button detach
    await page.keyboard.press('Escape')
    await expect(reopenedDialog).not.toBeVisible()
  })

  test('can filter channels by tags', async ({ page }) => {
    const uniqueSuffix = Date.now().toString().slice(-6)
    const tagName = `pw-tag-${uniqueSuffix}`

    // Create a channel with a specific tag
    const createButton = page.getByTestId('add-channel-button')
    await expect(createButton).toBeVisible({ timeout: 10000 })
    await createButton.click()

    const createDialog = page.getByRole('dialog')
    await createDialog.getByTestId('channel-name-input').fill(`Channel-${tagName}`)

    // Select provider - use data-testid for reliable selection
    const openaiProviderRadio = createDialog.getByTestId('provider-openai')
    await openaiProviderRadio.click()

    await createDialog.getByTestId('channel-base-url-input').fill('https://api.openai.com/v1')
    await createDialog.getByTestId('channel-api-key-input').fill('sk-test-' + uniqueSuffix)

    // Add model - wait for badge to be visible then click
    const modelBadge = createDialog.getByTestId('quick-model-gpt-4o')
    await expect(modelBadge).toBeVisible({ timeout: 5000 })
    await modelBadge.click()
    await page.waitForTimeout(300)
    
    const addSelectedButton = createDialog.getByTestId('add-selected-models-button')
    await expect(addSelectedButton).toBeEnabled({ timeout: 5000 })
    await addSelectedButton.click()
    await page.waitForTimeout(500)

    // Select Default Test Model
    const defaultTestModelSelect = createDialog.getByTestId('default-test-model-select')
    await expect(defaultTestModelSelect).toBeVisible({ timeout: 5000 })
    await defaultTestModelSelect.click()
    const firstOption = page.getByRole('option').first()
    await firstOption.click()
    await page.waitForTimeout(300)

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByTestId('channel-submit-button').click(),
    ])

    await expect(createDialog).not.toBeVisible({ timeout: 10000 })
    await page.waitForTimeout(1000)

    // Newly created channels are disabled by default.
    // If an active status filter excludes disabled channels, include them.
    const statusBtn = page
      .locator('button')
      .filter({ hasText: /Status|状态/i })
      .and(page.locator('[aria-haspopup="dialog"]'))
      .first()
    const statusBtnText = await statusBtn.textContent()
    if (statusBtnText && /Enabled|启用/i.test(statusBtnText) && !/Disabled|禁用/i.test(statusBtnText)) {
      await statusBtn.click()
      await page.waitForTimeout(500)
      const disabledOpt = page
        .getByRole('option', { name: /Disabled|禁用/i })
        .or(page.locator('[role="option"]').filter({ hasText: /Disabled|禁用/i }))
      if ((await disabledOpt.count()) > 0) {
        await disabledOpt.first().click()
        await page.waitForTimeout(500)
      }
      await page.keyboard.press('Escape')
      await page.waitForTimeout(500)
    }

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

  test.describe('Codex Responses endpoints', () => {
    test('Codex WebSocket checkbox persists dual endpoints', async ({ page }) => {
      const suffix = uniqueSuffix()
      const name = `pw-codex-dual-${suffix}`
      const baseURL = `https://proxy-${suffix}.example.com:8443/openai`

      const dialog = await startCodexChannel(page, name)
      await useCodexThirdParty(dialog, baseURL)
      const checkbox = dialog.getByTestId('codex-responses-websocket-checkbox')
      await expect(checkbox).toBeChecked()
      await expect(dialog.getByTestId('api-format-select')).toHaveCount(0)
      await expect(dialog.getByTestId('codex-responses-websocket-url')).toContainText(
        `wss://proxy-${suffix}.example.com:8443/openai/v1/responses`
      )
      await addCodexModel(page, dialog)

      const created = await captureGraphQL(page, 'CreateChannel', () => dialog.getByTestId('channel-submit-button').click())
      expect(created.variables.input.baseURL).toBe(baseURL)
      expect(created.variables.input.endpoints).toEqual([{ apiFormat: RESPONSES }, { apiFormat: RESPONSES_WS }])
      await expect(dialog).not.toBeVisible({ timeout: 10000 })

      const saved = await fetchChannel(page, created.data.createChannel.id)
      expect(familyFormats(saved.endpoints)).toEqual([RESPONSES, RESPONSES_WS])
      expect(familyFormats(saved.defaultEndpoints)).toEqual([RESPONSES, RESPONSES_WS])
      expect((saved.endpoints ?? []).every((endpoint) => !endpoint.transport)).toBe(true)
      // Native Images keep their HTTP defaults regardless of the checkbox.
      expect((saved.defaultEndpoints ?? []).map((endpoint) => endpoint.apiFormat)).toEqual(
        expect.arrayContaining(['openai/image_generation', 'openai/image_edit'])
      )

      const row = await showChannel(page, name)
      await expect(row.getByTestId('endpoint-protocol-openai/responses')).toHaveClass(/violet/)
      const editDialog = await openEditDialog(page, row)
      await expect(editDialog.getByTestId('codex-responses-websocket-checkbox')).toBeChecked()
      await expect(editDialog.getByTestId('channel-base-url-input')).toHaveValue(baseURL)
      await editDialog.getByRole('button', { name: /^(Cancel|取消)$/ }).click()
      await expect(editDialog).not.toBeVisible()

      await openRowAction(page, row, /^(Duplicate|复制)$/)
      const duplicateDialog = page.getByRole('dialog')
      await expect(duplicateDialog.getByTestId('codex-responses-websocket-checkbox')).toBeChecked()
      await duplicateDialog.getByTestId('channel-api-key-input').fill(`sk-synthetic-${suffix}`)
      const duplicated = await captureGraphQL(page, 'DuplicateChannel', () =>
        duplicateDialog.getByTestId('channel-submit-button').click()
      )
      expect(duplicated.variables.input.baseURL).toBe(baseURL)
      expect(duplicated.variables.input.endpoints).toEqual([{ apiFormat: RESPONSES }, { apiFormat: RESPONSES_WS }])
    })

    test('Codex unchecked persists HTTP only', async ({ page }) => {
      const suffix = uniqueSuffix()
      const name = `pw-codex-http-${suffix}`
      const baseURL = 'http://127.0.0.1:18080/v1'

      const dialog = await startCodexChannel(page, name)
      await useCodexThirdParty(dialog, baseURL)
      const checkbox = dialog.getByTestId('codex-responses-websocket-checkbox')
      await checkbox.focus()
      await page.keyboard.press('Space')
      await expect(checkbox).not.toBeChecked()
      await expect(dialog.getByTestId('codex-responses-websocket-url')).toHaveCount(0)
      await addCodexModel(page, dialog)

      const created = await captureGraphQL(page, 'CreateChannel', () => dialog.getByTestId('channel-submit-button').click())
      expect(created.variables.input.baseURL).toBe(baseURL)
      expect(created.variables.input.endpoints).toEqual([{ apiFormat: RESPONSES }])
      await expect(dialog).not.toBeVisible({ timeout: 10000 })

      const saved = await fetchChannel(page, created.data.createChannel.id)
      expect(familyFormats(saved.endpoints)).toEqual([RESPONSES])
      // An explicit HTTP-only save suppresses the WebSocket default.
      expect(familyFormats(saved.defaultEndpoints)).toEqual([RESPONSES])

      const row = await showChannel(page, name)
      const editDialog = await openEditDialog(page, row)
      await expect(editDialog.getByTestId('codex-responses-websocket-checkbox')).not.toBeChecked()
      const updated = await captureGraphQL(page, 'UpdateChannel', () => editDialog.getByTestId('channel-submit-button').click())
      expect(updated.variables.input.endpoints).toEqual([{ apiFormat: RESPONSES }])
      expect(familyFormats((await fetchChannel(page, saved.id)).endpoints)).toEqual([RESPONSES])
    })

    test('Codex official and auth.json base URLs are locked', async ({ page }) => {
      const suffix = uniqueSuffix()
      const name = `pw-codex-official-${suffix}`
      const typedURL = `https://typed-${suffix}.example.com/v1`

      const dialog = await startCodexChannel(page, name)
      const baseURLInput = dialog.getByTestId('channel-base-url-input')
      await expect(baseURLInput).toBeDisabled()
      await expect(baseURLInput).toHaveValue(CODEX_BASE_URL)

      await dialog.getByRole('tab', { name: THIRD_PARTY_TAB }).click()
      await expect(baseURLInput).toBeEnabled()
      await baseURLInput.fill(typedURL)
      await dialog.getByRole('tab', { name: OFFICIAL_TAB }).click()
      await expect(baseURLInput).toBeDisabled()
      await expect(baseURLInput).toHaveValue(CODEX_BASE_URL)
      await dialog.getByRole('tab', { name: AUTH_JSON_TAB }).click()
      await expect(baseURLInput).toBeDisabled()
      await expect(baseURLInput).toHaveValue(CODEX_BASE_URL)
      await dialog.getByRole('tab', { name: THIRD_PARTY_TAB }).click()
      await expect(baseURLInput).toHaveValue(typedURL)

      // The stale third-party URL must never be submitted for auth.json credentials.
      await dialog.getByRole('tab', { name: AUTH_JSON_TAB }).click()
      await dialog
        .getByTestId('codex-auth-json-input')
        .fill(JSON.stringify({ tokens: { access_token: `synthetic-access-${suffix}`, refresh_token: `synthetic-refresh-${suffix}` } }))
      await Promise.all([
        page.waitForResponse((response) => response.url().includes('/codex/auth/decode') && response.ok()),
        dialog.getByTestId('codex-auth-json-apply').click(),
      ])
      await addCodexModel(page, dialog)

      const created = await captureGraphQL(page, 'CreateChannel', () => dialog.getByTestId('channel-submit-button').click())
      expect(created.variables.input.baseURL).toBe(CODEX_BASE_URL)
      expect(created.variables.input.endpoints).toEqual([{ apiFormat: RESPONSES }, { apiFormat: RESPONSES_WS }])
      await expect(dialog).not.toBeVisible({ timeout: 10000 })

      // Stored OAuth tokens reopen on the locked Official tab.
      const row = await showChannel(page, name)
      const editDialog = await openEditDialog(page, row)
      await expect(editDialog.getByRole('tab', { name: OFFICIAL_TAB })).toHaveAttribute('aria-selected', 'true')
      const editBaseURL = editDialog.getByTestId('channel-base-url-input')
      await expect(editBaseURL).toBeDisabled()
      await expect(editBaseURL).toHaveValue(CODEX_BASE_URL)
      await editDialog.getByTestId('codex-responses-websocket-checkbox').click()
      const updated = await captureGraphQL(page, 'UpdateChannel', () => editDialog.getByTestId('channel-submit-button').click())
      expect(updated.variables.input.baseURL).toBe(CODEX_BASE_URL)
      expect(updated.variables.input.endpoints).toEqual([{ apiFormat: RESPONSES }])

      const saved = await fetchChannel(page, created.data.createChannel.id)
      expect(saved.baseURL).toBe(CODEX_BASE_URL)
      expect(familyFormats(saved.endpoints)).toEqual([RESPONSES])
      expect(familyFormats(saved.defaultEndpoints)).toEqual([RESPONSES])
    })

    test('Codex third-party rejects WS URLs and derives matching WS endpoint', async ({ page }) => {
      const suffix = uniqueSuffix()
      const name = `pw-codex-ws-${suffix}`

      const dialog = await startCodexChannel(page, name)
      await useCodexThirdParty(dialog, `wss://proxy-${suffix}.example.com/v1`)
      await addCodexModel(page, dialog)
      const derived = dialog.getByTestId('codex-responses-websocket-url')
      await expect(derived).toHaveCount(0)

      const createRequests: string[] = []
      page.on('request', (request) => {
        if (isGraphQLOperation(request, 'CreateChannel')) createRequests.push(request.url())
      })
      await dialog.getByTestId('channel-submit-button').click()
      await expect(dialog.getByText(/http:\/\/ or https:\/\/|http:\/\/ 或 https:\/\//)).toBeVisible()
      expect(createRequests).toEqual([])

      const baseURLInput = dialog.getByTestId('channel-base-url-input')
      for (const [httpURL, webSocketURL] of [
        [`https://proxy-${suffix}.example.com:8443/openai`, `wss://proxy-${suffix}.example.com:8443/openai/v1/responses`],
        [`https://proxy-${suffix}.example.com/codex/#`, `wss://proxy-${suffix}.example.com/codex/responses`],
        [`https://proxy-${suffix}.example.com/v1/responses##`, `wss://proxy-${suffix}.example.com/v1/responses`],
        ['http://127.0.0.1:18080/v1', 'ws://127.0.0.1:18080/v1/responses'],
      ]) {
        await baseURLInput.fill(httpURL)
        await expect(derived).toContainText(webSocketURL)
        await expect(derived).not.toContainText('#')
        await expect(derived).not.toContainText('responses/responses')
      }

      const created = await captureGraphQL(page, 'CreateChannel', () => dialog.getByTestId('channel-submit-button').click())
      expect(created.variables.input.baseURL).toBe('http://127.0.0.1:18080/v1')
      expect(created.variables.input.endpoints).toEqual([{ apiFormat: RESPONSES }, { apiFormat: RESPONSES_WS }])
      expect(JSON.stringify(created.variables.input)).not.toMatch(/wss?:\/\//)
    })

    test('Codex legacy WS-only upgrades only on explicit save', async ({ page }) => {
      const suffix = uniqueSuffix()
      const name = `pw-codex-legacy-${suffix}`
      const legacyBaseURL = `wss://legacy-${suffix}.example.com:8443/codex#`
      const httpBaseURL = `https://legacy-${suffix}.example.com:8443/codex#`
      const seeded = await seedChannel(page, {
        type: 'codex',
        name,
        baseURL: legacyBaseURL,
        credentials: { apiKeys: [`sk-synthetic-${suffix}`] },
        supportedModels: [CODEX_MODEL],
        defaultTestModel: CODEX_MODEL,
      })
      const before = await fetchChannel(page, seeded.id)
      expect(before.baseURL).toBe(legacyBaseURL)
      expect(familyFormats(before.endpoints)).toEqual([])
      expect(familyFormats(before.defaultEndpoints)).toEqual([RESPONSES_WS])

      const row = await showChannel(page, name)
      // List and detail recognize the WebSocket-only endpoint as Responses.
      await expect(row.getByTestId('endpoint-protocol-openai/responses')).toHaveClass(/violet/)
      await row.locator('button:has(svg.tabler-icon-chevron-right)').click()
      await expect(page.getByText('OpenAI (Responses WebSocket)', { exact: true })).toBeVisible()

      const editDialog = await openEditDialog(page, row)
      await expect(editDialog.getByTestId('codex-responses-websocket-checkbox')).toBeChecked()
      await expect(editDialog.getByTestId('codex-responses-websocket-upgrade-note')).toBeVisible()
      await expect(editDialog.getByTestId('channel-base-url-input')).toHaveValue(httpBaseURL)
      await expect(editDialog.getByTestId('codex-responses-websocket-url')).toContainText(
        `wss://legacy-${suffix}.example.com:8443/codex/responses`
      )
      await editDialog.getByRole('button', { name: /^(Cancel|取消)$/ }).click()
      await expect(editDialog).not.toBeVisible()

      const endpointsDialog = await openEndpointsDialog(page, row)
      await expect(endpointsDialog.getByTestId('codex-endpoints-websocket-checkbox')).toBeChecked()
      await expect(endpointsDialog.getByTestId('codex-endpoints-websocket-upgrade-note')).toBeVisible()
      await endpointsDialog.getByRole('button', { name: /^(Cancel|取消)$/ }).click()
      await expect(endpointsDialog).not.toBeVisible()

      const untouched = await fetchChannel(page, seeded.id)
      expect(untouched.baseURL).toBe(legacyBaseURL)
      expect(untouched.endpoints ?? []).toEqual(before.endpoints ?? [])

      const saveDialog = await openEditDialog(page, row)
      const updated = await captureGraphQL(page, 'UpdateChannel', () => saveDialog.getByTestId('channel-submit-button').click())
      expect(updated.variables.input.baseURL).toBe(httpBaseURL)
      expect(updated.variables.input.endpoints).toEqual([{ apiFormat: RESPONSES }, { apiFormat: RESPONSES_WS }])

      const after = await fetchChannel(page, seeded.id)
      expect(after.baseURL).toBe(httpBaseURL)
      expect(familyFormats(after.endpoints)).toEqual([RESPONSES, RESPONSES_WS])
      expect(familyFormats(after.defaultEndpoints)).toEqual([RESPONSES, RESPONSES_WS])
    })

    test('Codex endpoints dialog shares the WebSocket projection and guards model restrictions', async ({ page }) => {
      const suffix = uniqueSuffix()
      const name = `pw-codex-endpoints-${suffix}`
      const seeded = await seedChannel(page, {
        type: 'codex',
        name,
        baseURL: `https://gateway-${suffix}.example.com:9443/api`,
        credentials: { apiKeys: [`sk-synthetic-${suffix}`] },
        supportedModels: [CODEX_MODEL, 'gpt-6-luna'],
        defaultTestModel: CODEX_MODEL,
        endpoints: [
          { apiFormat: RESPONSES, path: '/custom/responses' },
          { apiFormat: RESPONSES_WS, path: '/custom/responses' },
          { apiFormat: 'openai/image_generation', path: '/images/generations' },
        ],
        settings: { modelProtocols: [{ model: CODEX_MODEL, apiFormats: [RESPONSES_WS], enabled: true }] },
      })

      const row = await showChannel(page, name)
      const dialog = await openEndpointsDialog(page, row)
      const checkbox = dialog.getByTestId('codex-endpoints-websocket-checkbox')
      await expect(checkbox).toBeChecked()
      await expect(dialog.getByTestId('codex-endpoints-websocket-url')).toContainText(
        `wss://gateway-${suffix}.example.com:9443/api/custom/responses`
      )
      // Responses rows are previews managed by the checkbox; other endpoints stay removable.
      for (const format of [RESPONSES, RESPONSES_WS]) {
        await expect(dialog.getByRole('button', { name: removeEndpointLabel(format) })).toHaveCount(0)
      }
      await expect(dialog.getByRole('button', { name: removeEndpointLabel('openai/image_generation') })).toBeVisible()
      await expect(dialog.getByText(/^(Unsupported|不支持)$/)).toHaveCount(0)

      await checkbox.click()
      const blocked = page.getByRole('alertdialog')
      await expect(blocked).toBeVisible()
      await expect(blocked).toContainText(RESPONSES_WS)
      await expect(blocked).toContainText(CODEX_MODEL)
      await blocked.getByRole('button', { name: /^(Close|关闭)$/ }).click()
      await expect(blocked).not.toBeVisible()
      await expect(checkbox).toBeChecked()

      const model = escapeRegExp(CODEX_MODEL)
      await dialog.getByRole('button', { name: new RegExp(`^(Remove protocol override for ${model}|删除 ${model} 的协议覆盖)$`) }).click()
      await checkbox.click()
      await expect(checkbox).not.toBeChecked()
      await expect(dialog.getByTestId('codex-endpoints-websocket-url')).toHaveCount(0)

      const updated = await captureGraphQL(page, 'UpdateChannel', () => dialog.getByRole('button', { name: /^(Save|保存)$/ }).click())
      expect(updated.variables.input.baseURL).toBeUndefined()
      expect(updated.variables.input.endpoints).toEqual([
        { apiFormat: RESPONSES, path: '/custom/responses' },
        { apiFormat: 'openai/image_generation', path: '/images/generations' },
      ])
      expect(updated.variables.input.settings.modelProtocols).toEqual([])

      const after = await fetchChannel(page, seeded.id)
      expect(familyFormats(after.endpoints)).toEqual([RESPONSES])
      expect(familyFormats(after.defaultEndpoints)).toEqual([RESPONSES])
      expect(after.endpoints?.find((endpoint) => endpoint.apiFormat === 'openai/image_generation')?.path).toBe('/images/generations')
    })

    test('OpenAI Responses legacy single transport stays unchanged', async ({ page }) => {
      const suffix = uniqueSuffix()
      const name = `pw-openai-ws-${suffix}`

      await page.getByTestId('add-channel-button').click()
      const dialog = page.getByRole('dialog')
      await dialog.getByTestId('provider-openai').click()
      await dialog.getByTestId('api-format-select').click()
      await page.getByRole('option', { name: 'OpenAI (Responses WebSocket)' }).click()
      await expect(dialog.getByTestId('channel-base-url-input')).toHaveValue('wss://api.openai.com/v1#')
      await expect(dialog.getByTestId('codex-responses-websocket-checkbox')).toHaveCount(0)
      await dialog.getByTestId('channel-name-input').fill(name)
      await dialog.getByTestId('channel-api-key-input').fill(`sk-synthetic-${suffix}`)
      await dialog.getByTestId('quick-model-gpt-4o').click()
      await dialog.getByTestId('add-selected-models-button').click()
      await dialog.getByTestId('default-test-model-select').click()
      await page.getByRole('option', { name: 'gpt-4o', exact: true }).click()

      const created = await captureGraphQL(page, 'CreateChannel', () => dialog.getByTestId('channel-submit-button').click())
      expect(created.variables.input.type).toBe('openai_responses')
      expect(created.variables.input.baseURL).toBe('wss://api.openai.com/v1#')
      expect(created.variables.input.endpoints).toBeUndefined()
      await expect(dialog).not.toBeVisible({ timeout: 10000 })
      const saved = await fetchChannel(page, created.data.createChannel.id)
      expect(familyFormats(saved.defaultEndpoints)).toEqual([RESPONSES_WS])

      const row = await showChannel(page, name)
      await expect(row.getByTestId('endpoint-protocol-openai/responses')).toHaveClass(/violet/)
      const endpointsDialog = await openEndpointsDialog(page, row)
      await expect(endpointsDialog.getByTestId('codex-endpoints-websocket-checkbox')).toHaveCount(0)
      await expect(endpointsDialog.getByText(RESPONSES_WS, { exact: true })).toBeVisible()
      await expect(endpointsDialog.getByText(/^(Unsupported|不支持)$/)).toHaveCount(0)
      const updated = await captureGraphQL(page, 'UpdateChannel', () =>
        endpointsDialog.getByRole('button', { name: /^(Save|保存)$/ }).click()
      )
      expect(updated.variables.input.baseURL).toBeUndefined()
      expect(updated.variables.input.endpoints).toEqual([])

      const after = await fetchChannel(page, saved.id)
      expect(after.baseURL).toBe('wss://api.openai.com/v1#')
      expect(familyFormats(after.defaultEndpoints)).toEqual([RESPONSES_WS])

      const editDialog = await openEditDialog(page, row)
      await expect(editDialog.getByTestId('api-format-select')).toContainText('OpenAI (Responses WebSocket)')
    })

    test('Codex WebSocket controls render in English and Chinese', async ({ page }, testInfo) => {
      // Two languages at two widths for both dialogs; the shared beforeEach sets 60s.
      test.setTimeout(120_000)
      const suffix = uniqueSuffix()
      const name = `pw-codex-visual-${suffix}`
      await seedChannel(page, {
        type: 'codex',
        name,
        baseURL: `https://visual-${suffix}.example.com/v1`,
        credentials: { apiKeys: [`sk-synthetic-${suffix}`] },
        supportedModels: [CODEX_MODEL],
        defaultTestModel: CODEX_MODEL,
        endpoints: [{ apiFormat: RESPONSES }, { apiFormat: RESPONSES_WS }],
      })

      for (const language of ['en', 'zh']) {
        await page.setViewportSize({ width: 1280, height: 900 })
        await page.evaluate((lng) => localStorage.setItem('i18nextLng', lng), language)
        // The app applies the signed-in user's preferred language from `me`; override it for
        // this page only so the shared admin profile used by parallel tests stays unchanged.
        await page.unroute('**/admin/graphql')
        await page.route('**/admin/graphql', async (route) => {
          if (!isGraphQLOperation(route.request(), 'Me')) return route.fallback()
          const response = await route.fetch()
          const json = await response.json()
          json.data.me.preferLanguage = language
          await route.fulfill({ response, json })
        })
        const row = await showChannel(page, name)

        const editDialog = await openEditDialog(page, row)
        const checkbox = editDialog.getByTestId('codex-responses-websocket-checkbox')
        await expect(checkbox).toBeChecked()
        const derivedURL = editDialog.getByTestId('codex-responses-websocket-url')
        await expect(derivedURL).toContainText(`wss://visual-${suffix}.example.com/v1/responses`)
        await expect(derivedURL).toContainText(language === 'zh' ? 'WebSocket 端点' : 'WebSocket endpoint')
        for (const width of [1280, 375]) {
          await page.setViewportSize({ width, height: width === 375 ? 812 : 900 })
          await checkbox.scrollIntoViewIfNeeded()
          await expect(checkbox).toBeVisible()
          await page.screenshot({ path: testInfo.outputPath(`codex-form-${language}-${width}.png`) })
        }
        await page.setViewportSize({ width: 1280, height: 900 })
        await editDialog.getByRole('button', { name: /^(Cancel|取消)$/ }).click()
        await expect(editDialog).not.toBeVisible()

        const endpointsDialog = await openEndpointsDialog(page, row)
        const endpointsCheckbox = endpointsDialog.getByTestId('codex-endpoints-websocket-checkbox')
        await expect(endpointsCheckbox).toBeChecked()
        await expect(endpointsDialog.getByTestId('codex-endpoints-websocket-url')).toContainText(
          language === 'zh' ? 'WebSocket 端点' : 'WebSocket endpoint'
        )
        for (const width of [1280, 375]) {
          await page.setViewportSize({ width, height: width === 375 ? 812 : 900 })
          await endpointsCheckbox.scrollIntoViewIfNeeded()
          await expect(endpointsCheckbox).toBeVisible()
          await page.screenshot({ path: testInfo.outputPath(`codex-endpoints-${language}-${width}.png`) })
        }
        await page.setViewportSize({ width: 1280, height: 900 })
        await endpointsDialog.getByRole('button', { name: /^(Cancel|取消)$/ }).click()
        await expect(endpointsDialog).not.toBeVisible()
      }
    })
  })
})

const RESPONSES = 'openai/responses'
const RESPONSES_WS = 'openai/responses-ws'
const CODEX_BASE_URL = 'https://chatgpt.com/backend-api/codex#'
const CODEX_MODEL = 'gpt-6.1-sol'
const OFFICIAL_TAB = /^(Official|官方)$/
const AUTH_JSON_TAB = /^auth\.json$/
const THIRD_PARTY_TAB = /^(Third[- ]party|第三方)$/i

type EndpointShape = { apiFormat: string; path?: string | null; baseURL?: string | null; transport?: string | null }
type ChannelProjection = {
  id: string
  name: string
  baseURL: string
  endpoints: EndpointShape[] | null
  defaultEndpoints: EndpointShape[] | null
  settings: { modelProtocols: { model: string; apiFormats: string[]; enabled: boolean | null }[] | null } | null
}

const CHANNEL_PROJECTION_QUERY = `
  query ResponsesChannelProjection($input: QueryChannelInput!) {
    queryChannels(input: $input) {
      edges {
        node {
          id
          name
          baseURL
          endpoints { apiFormat path baseURL transport }
          defaultEndpoints { apiFormat path baseURL transport }
          settings { modelProtocols { model apiFormats enabled } }
        }
      }
    }
  }
`

const SEED_CHANNEL_MUTATION = `
  mutation SeedResponsesChannel($input: CreateChannelInput!) {
    createChannel(input: $input) { id name }
  }
`

function uniqueSuffix() {
  return `${Date.now().toString().slice(-6)}${Math.random().toString(36).slice(2, 6)}`
}

function escapeRegExp(value: string) {
  return value.replace(/[.*+?^${}()|[\]\\/]/g, '\\$&')
}

function removeEndpointLabel(apiFormat: string) {
  const format = escapeRegExp(apiFormat)
  return new RegExp(`^(Remove ${format} endpoint|删除 ${format} 端点)$`)
}

function familyFormats(endpoints: EndpointShape[] | null | undefined) {
  return (endpoints ?? []).map((endpoint) => endpoint.apiFormat).filter((format) => format === RESPONSES || format === RESPONSES_WS)
}

// Reads or seeds configuration through the admin GraphQL API with the signed-in session.
async function adminGraphQL<T>(page: Page, query: string, variables: Record<string, unknown>): Promise<T> {
  return (await page.evaluate(
    async ({ query, variables }) => {
      const response = await fetch('/admin/graphql', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${localStorage.getItem('axonhub_access_token') ?? ''}`,
        },
        body: JSON.stringify({ query, variables }),
      })
      const body = await response.json()
      if (body.errors?.length) {
        throw new Error(JSON.stringify(body.errors))
      }
      return body.data
    },
    { query, variables }
  )) as T
}

async function fetchChannel(page: Page, id: string): Promise<ChannelProjection> {
  const data = await adminGraphQL<{ queryChannels: { edges: { node: ChannelProjection }[] } }>(page, CHANNEL_PROJECTION_QUERY, {
    input: { first: 1, where: { id } },
  })
  const channel = data.queryChannels.edges[0]?.node
  expect(channel, `channel ${id} exists`).toBeTruthy()
  return channel
}

async function seedChannel(page: Page, input: Record<string, unknown>) {
  const data = await adminGraphQL<{ createChannel: { id: string; name: string } }>(page, SEED_CHANNEL_MUTATION, { input })
  return data.createChannel
}

function isGraphQLOperation(request: Request, operationName: string) {
  if (!request.url().includes('/admin/graphql') || request.method() !== 'POST') return false
  try {
    return request.postDataJSON()?.operationName === operationName
  } catch {
    return false
  }
}

// Captures the exact variables the UI sends and fails on GraphQL errors.
async function captureGraphQL(page: Page, operationName: string, action: () => Promise<unknown>) {
  const [request, response] = await Promise.all([
    page.waitForRequest((request) => isGraphQLOperation(request, operationName)),
    page.waitForResponse((response) => isGraphQLOperation(response.request(), operationName)),
    action(),
  ])
  const body = await response.json()
  expect(body.errors, `${operationName} errors`).toBeUndefined()
  return { variables: request.postDataJSON().variables, data: body.data }
}

async function includeDisabledChannels(page: Page) {
  const statusBtn = page
    .locator('button')
    .filter({ hasText: /Status|状态/i })
    .and(page.locator('[aria-haspopup="dialog"]'))
    .first()
  const statusBtnText = await statusBtn.textContent()
  if (statusBtnText && /Enabled|启用/i.test(statusBtnText) && !/Disabled|禁用/i.test(statusBtnText)) {
    await statusBtn.click()
    const disabledOpt = page
      .getByRole('option', { name: /Disabled|禁用/i })
      .or(page.locator('[role="option"]').filter({ hasText: /Disabled|禁用/i }))
    if ((await disabledOpt.count()) > 0) {
      await disabledOpt.first().click()
    }
    await page.keyboard.press('Escape')
  }
}

async function showChannel(page: Page, name: string) {
  await gotoAndEnsureAuth(page, '/channels')
  const table = page.locator('[data-testid="channels-table"]')
  await table.waitFor({ state: 'visible', timeout: 15000 })
  await includeDisabledChannels(page)
  await page.getByPlaceholder(/^(Search by name|按名称筛选)$/).fill(name)
  const row = table.locator('tbody tr').filter({ hasText: name })
  await expect(row).toBeVisible({ timeout: 15000 })
  return row
}

async function openEditDialog(page: Page, row: Locator) {
  await row.locator('td:last-child button').first().click()
  const dialog = page.getByRole('dialog', { name: /编辑|Edit Channel/i })
  await expect(dialog).toBeVisible()
  return dialog
}

async function openRowAction(page: Page, row: Locator, name: RegExp) {
  await row.getByTestId('row-actions').click()
  await page.getByRole('menuitem', { name }).click()
}

async function openEndpointsDialog(page: Page, row: Locator) {
  await openRowAction(page, row, /^(Endpoints|端点配置)$/)
  const dialog = page.getByRole('dialog', { name: /Endpoints|端点配置/ })
  await expect(dialog).toBeVisible()
  return dialog
}

async function startCodexChannel(page: Page, name: string) {
  await page.getByTestId('add-channel-button').click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  await dialog.getByTestId('provider-codex').click()
  await dialog.getByTestId('channel-name-input').fill(name)
  return dialog
}

async function useCodexThirdParty(dialog: Locator, baseURL: string) {
  await dialog.getByRole('tab', { name: THIRD_PARTY_TAB }).click()
  await dialog.getByTestId('channel-base-url-input').fill(baseURL)
  await dialog.getByTestId('channel-api-key-input').fill(`sk-synthetic-${uniqueSuffix()}`)
}

async function addCodexModel(page: Page, dialog: Locator) {
  await dialog.getByTestId(`quick-model-${CODEX_MODEL}`).click()
  await dialog.getByTestId('add-selected-models-button').click()
  await dialog.getByTestId('default-test-model-select').click()
  await page.getByRole('option', { name: CODEX_MODEL, exact: true }).click()
}
