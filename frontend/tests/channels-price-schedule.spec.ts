import { test, expect, type Locator, type Page } from '@playwright/test'
import { gotoAndEnsureAuth, waitForGraphQLOperation } from './auth.utils'

/**
 * Regression test for looplj/axonhub#2485.
 *
 * The daily-time picker popup used to be an absolutely positioned descendant of the price
 * dialog's scroll containers. With the price card sitting at the bottom of the scrollable
 * list, the dialog clipped the lower part of the popup, so the last hours (22/23) and
 * minutes (58/59) could not be reached by scrolling the hour/minute columns.
 *
 * The popup is now rendered through a popover portal inside the dialog content (so the list
 * container no longer clips it, and the dialog's scroll lock does not block wheel scrolling
 * inside it), and popper is told to treat the dialog content as its collision boundary so it
 * flips above the trigger instead of spilling out of the dialog.
 *
 * The regression is asserted on the rendered UI rather than on a click: Playwright scrolls
 * clipped elements into view before clicking, so a click on a clipped hour would still pass.
 * Instead the test scrolls the hour column to its end (what a user does with the wheel) and
 * checks that the last hour really is the topmost element at its own centre.
 */
/**
 * Archive a channel created by this test, so a run does not leave test data behind.
 * Used by the happy path and, as a best-effort fallback, by the failure path.
 */
async function archiveChannel(page: Page, name: string) {
  const channelsTable = page.locator('[data-testid="channels-table"]')
  const channelRow = channelsTable.locator('tbody tr').filter({ hasText: name })

  // Leave any dialog or popover behind before touching the table: the first Escape
  // closes an open picker popover, the second one the price dialog.
  for (let attempt = 0; attempt < 2; attempt++) {
    if (await page.getByTestId('channel-price-dialog').isVisible().catch(() => false)) {
      await page.keyboard.press('Escape')
      await page.waitForTimeout(300)
    }
  }

  await expect(channelRow).toBeVisible({ timeout: 15000 })
  await channelRow.getByTestId('row-actions').click()
  const menu = page.getByRole('menu')
  await expect(menu).toBeVisible()
  await menu.getByRole('menuitem', { name: /归档|Archive/i }).click()

  const archiveDialog = page.getByRole('alertdialog').or(page.getByRole('dialog'))
  await expect(archiveDialog).toBeVisible()
  const archiveButton = archiveDialog.getByRole('button', { name: /归档|Archive/i }).last()
  await Promise.all([waitForGraphQLOperation(page, 'UpdateChannelStatus'), archiveButton.click()])
  await expect(archiveDialog).not.toBeVisible({ timeout: 10000 })
}

/**
 * Returns 'itself' when the element with `testId` is the topmost dialog-descendant at its own
 * centre. A clipped or covered popup is reported as whatever element sits on top instead.
 */
async function topmostInsideDialog(dialog: Locator, testId: string): Promise<string> {
  return dialog.evaluate((root, id) => {
    const button = root.querySelector(`[data-testid="${id}"]`)
    if (!button) return 'missing'
    const rect = button.getBoundingClientRect()
    const stack = document.elementsFromPoint(rect.left + rect.width / 2, rect.top + rect.height / 2)
    // Only elements inside the dialog are relevant: the regression is the dialog/list clipping
    // the popup, while viewport-level overlays (e.g. the success toast pinned to the bottom of
    // the viewport) are unrelated to it.
    const top = stack.find((element) => root.contains(element)) ?? null
    if (top === button) return 'itself'
    return top ? `${top.tagName.toLowerCase()}:${top.getAttribute('data-testid') ?? top.className}` : 'null'
  }, testId)
}

test.describe('Channel model price schedule', () => {
  // Name of the channel created by this test, so a failing run can still clean it up.
  let createdChannelName: string | null = null

  test.beforeEach(async ({ page }) => {
    createdChannelName = null
    // Increase timeout: the test creates a channel before opening the price dialog.
    test.setTimeout(120000)
    await gotoAndEnsureAuth(page, '/channels')

    // Wait for page to fully load and channels table to appear
    await page.waitForTimeout(2000)
    const channelsTable = page.locator('[data-testid="channels-table"]')
    await channelsTable.waitFor({ state: 'visible', timeout: 15000 })
  })

  // Best-effort cleanup for the failure path. It must never mask the real assertion
  // failure, so errors are logged and swallowed instead of thrown.
  test.afterEach(async ({ page }) => {
    const nameToArchive = createdChannelName
    if (!nameToArchive) return
    try {
      await archiveChannel(page, nameToArchive)
    } catch (error) {
      console.warn(`[channels-price-schedule] could not archive ${nameToArchive}: ${String(error)}`)
    }
  })

  test('daily time picker can select 22:00 and 23:00 at the bottom of the price dialog', async ({ page }) => {
    const channelsTable = page.locator('[data-testid="channels-table"]')

    // ---------- create a channel so the price dialog can be opened ----------
    const uniqueSuffix = Date.now().toString().slice(-6)
    const name = `pw-price-schedule ${uniqueSuffix}`

    await page.getByTestId('add-channel-button').click()
    const createDialog = page.getByRole('dialog')
    await expect(createDialog).toBeVisible()

    await createDialog.getByTestId('channel-name-input').fill(name)
    await createDialog.getByTestId('provider-openai').click()
    await createDialog.getByTestId('channel-base-url-input').fill(`https://api.price-${uniqueSuffix}.example.com`)
    await createDialog.getByTestId('channel-api-key-input').fill(`sk-test-price-${uniqueSuffix}`)

    // At least one supported model is required to enable the create button.
    const modelBadge = createDialog.getByTestId('quick-model-gpt-4o')
    await expect(modelBadge).toBeVisible({ timeout: 5000 })
    await modelBadge.click()
    await page.waitForTimeout(300)
    const addSelectedButton = createDialog.getByTestId('add-selected-models-button')
    await expect(addSelectedButton).toBeEnabled({ timeout: 5000 })
    await addSelectedButton.click()
    await page.waitForTimeout(500)

    const defaultTestModelSelect = createDialog.getByTestId('default-test-model-select')
    if ((await defaultTestModelSelect.count()) > 0) {
      await defaultTestModelSelect.click()
      await page.getByRole('option').first().click()
      await page.waitForTimeout(300)
    }

    await Promise.all([
      waitForGraphQLOperation(page, 'CreateChannel'),
      createDialog.getByTestId('channel-submit-button').click(),
    ])
    await expect(createDialog).not.toBeVisible({ timeout: 15000 })
    createdChannelName = name

    // New channels are created with 'disabled' status. If an active status filter
    // excludes disabled channels (e.g. "Enabled" is pre-selected), clear the filter.
    await page.waitForTimeout(1000)
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
    await expect(channelRow).toBeVisible({ timeout: 15000 })

    // ---------- open the model price dialog ---------------------------------
    await channelRow.getByTestId('row-actions').click()
    const rowMenu = page.getByRole('menu')
    await expect(rowMenu).toBeVisible()
    await rowMenu.getByRole('menuitem', { name: /模型价格|Model Price/i }).click()

    const priceDialog = page.getByTestId('channel-price-dialog')
    await expect(priceDialog).toBeVisible()

    // ---------- build a daily-time override ---------------------------------
    await priceDialog.getByTestId('price-add-button').click()
    await priceDialog.getByTestId('price-schedule-toggle').click()
    await priceDialog.getByTestId('price-schedule-add-override').click()
    await priceDialog.getByTestId('price-schedule-add-condition').click()

    const startPicker = priceDialog.getByTestId('price-schedule-daily-time-start')
    const endPicker = priceDialog.getByTestId('price-schedule-daily-time-end')
    await expect(startPicker).toHaveText('00:00')
    await expect(endPicker).toHaveText('08:00')

    // Scroll the list to its very bottom: the state reported in the issue, where the card
    // cannot be scrolled up any further to reveal a popup that opens downwards.
    await priceDialog.getByTestId('price-list').evaluate((list) => {
      list.scrollTop = list.scrollHeight
    })

    // Precondition of the scenario: the popup (220px tall + 8px offset) must not fit below
    // the picker inside the scrollable list, otherwise the regression cannot reproduce at all.
    // The list (not the dialog) is the container that used to clip the popup, so measure
    // against it: with the card scrolled to the bottom only a few pixels are left below.
    const roomBelowPicker = await priceDialog.evaluate((dialog) => {
      const picker = dialog.querySelector('[data-testid="price-schedule-daily-time-start"]')
      const list = dialog.querySelector('[data-testid="price-list"]')
      if (!picker || !list) return -1
      return list.getBoundingClientRect().bottom - picker.getBoundingClientRect().bottom
    })
    expect(roomBelowPicker).toBeGreaterThan(0)
    expect(roomBelowPicker).toBeLessThan(228)

    // ---------- start time 22:30 (hours 22/23 used to be unreachable) -------
    await startPicker.click()
    const startPopup = priceDialog.getByTestId('price-schedule-daily-time-start-popup')
    await expect(startPopup).toBeVisible()

    // The popup must stay inside the dialog instead of being clipped by it or spilling out.
    const popupBox = await startPopup.boundingBox()
    const dialogBox = await priceDialog.boundingBox()
    if (!popupBox || !dialogBox) throw new Error('time picker popup or price dialog is not visible')
    expect(popupBox.y).toBeGreaterThanOrEqual(dialogBox.y - 1)
    expect(popupBox.y + popupBox.height).toBeLessThanOrEqual(dialogBox.y + dialogBox.height + 1)

    // Scroll the hour column to its end and verify the last hour is really reachable:
    // with the old absolutely positioned popup it sat in the clipped area and the dialog
    // was the topmost element at its centre.
    await priceDialog.getByTestId('price-schedule-daily-time-start-hour-col').evaluate((col) => {
      col.scrollTop = col.scrollHeight
    })
    expect(await topmostInsideDialog(priceDialog, 'price-schedule-daily-time-start-hour-23')).toBe('itself')

    await priceDialog.getByTestId('price-schedule-daily-time-start-hour-22').click()
    await priceDialog.getByTestId('price-schedule-daily-time-start-minute-30').click()
    await expect(startPicker).toHaveText('22:30')

    // Escape closes the picker only; the price dialog stays open.
    await page.keyboard.press('Escape')
    await expect(startPopup).not.toBeVisible()
    await expect(priceDialog).toBeVisible()

    // ---------- end time 23:00 ---------------------------------------------
    await endPicker.click()
    const endPopup = priceDialog.getByTestId('price-schedule-daily-time-end-popup')
    await expect(endPopup).toBeVisible()

    await priceDialog.getByTestId('price-schedule-daily-time-end-hour-23').click()
    await expect(endPicker).toHaveText('23:00')

    // The dialog is still open and both values are kept.
    await expect(priceDialog).toBeVisible()
    await expect(startPicker).toHaveText('22:30')

    // ---------- flip phase: not enough room below -> popup flips above ----------
    // Close the end-time popup so the list can be repositioned.
    await page.keyboard.press('Escape')
    await expect(endPopup).not.toBeVisible()

    // With the list at its bottom the picker still has more room below it inside the dialog
    // than the popup needs (220px height + 8px offset + 8px collision padding = 236px), so the
    // popup opens downwards. Scroll the list back up just enough to leave less than 236px of
    // dialog below the picker while keeping the picker itself visible inside the list.
    const flipGeom = await priceDialog.evaluate((dialog) => {
      const picker = dialog.querySelector('[data-testid="price-schedule-daily-time-end"]')
      const list = dialog.querySelector('[data-testid="price-list"]')
      if (!picker || !list) return null
      const dialogRect = dialog.getBoundingClientRect()
      const beforeBelow = dialogRect.bottom - picker.getBoundingClientRect().bottom
      const shift = beforeBelow - 200
      if (shift > 0) list.scrollTop = Math.max(0, list.scrollTop - shift)
      const pickerRect = picker.getBoundingClientRect()
      const listRect = list.getBoundingClientRect()
      return {
        beforeBelow,
        below: dialogRect.bottom - pickerRect.bottom,
        above: pickerRect.top - dialogRect.top,
        shift: Math.max(0, shift),
        dialog: { top: dialogRect.top, bottom: dialogRect.bottom, height: dialogRect.height },
        list: { top: listRect.top, bottom: listRect.bottom },
        picker: { top: pickerRect.top, bottom: pickerRect.bottom, height: pickerRect.height },
        pickerInsideList: pickerRect.top >= listRect.top && pickerRect.bottom <= listRect.bottom,
      }
    })
    if (!flipGeom) throw new Error('end-time picker or price list is not available')
    console.log(`[flip] viewport=${JSON.stringify(page.viewportSize())} ${JSON.stringify(flipGeom)}`)
    expect(flipGeom.pickerInsideList).toBe(true)
    expect(flipGeom.below).toBeLessThan(236)
    expect(flipGeom.above).toBeGreaterThan(236)

    await endPicker.click()
    await expect(endPopup).toBeVisible()
    await expect(endPopup).toHaveAttribute('data-side', 'top')

    const flippedPopupBox = await endPopup.boundingBox()
    const flippedDialogBox = await priceDialog.boundingBox()
    const flippedPickerBox = await endPicker.boundingBox()
    if (!flippedPopupBox || !flippedDialogBox || !flippedPickerBox)
      throw new Error('flipped popup, picker or price dialog is not visible')
    console.log(`[flip] data-side=top popup=${JSON.stringify(flippedPopupBox)} trigger=${JSON.stringify(flippedPickerBox)}`)
    expect(flippedPopupBox.y).toBeGreaterThanOrEqual(flippedDialogBox.y - 1)
    expect(flippedPopupBox.y + flippedPopupBox.height).toBeLessThanOrEqual(flippedDialogBox.y + flippedDialogBox.height + 1)
    // The flipped popup sits fully above its trigger.
    expect(flippedPopupBox.y + flippedPopupBox.height).toBeLessThanOrEqual(flippedPickerBox.y)

    // Last hour is reachable in the flipped popup...
    await priceDialog.getByTestId('price-schedule-daily-time-end-hour-col').evaluate((col) => {
      col.scrollTop = col.scrollHeight
    })
    expect(await topmostInsideDialog(priceDialog, 'price-schedule-daily-time-end-hour-23')).toBe('itself')
    await priceDialog.getByTestId('price-schedule-daily-time-end-hour-23').click()

    // ...and so is the last minute.
    await priceDialog.getByTestId('price-schedule-daily-time-end-minute-col').evaluate((col) => {
      col.scrollTop = col.scrollHeight
    })
    expect(await topmostInsideDialog(priceDialog, 'price-schedule-daily-time-end-minute-59')).toBe('itself')
    await priceDialog.getByTestId('price-schedule-daily-time-end-minute-59').click()
    await expect(endPicker).toHaveText('23:59')

    // The dialog is still open and the earlier start value is kept.
    await expect(priceDialog).toBeVisible()
    await expect(startPicker).toHaveText('22:30')

    // ---------- cleanup: close the dialog, then archive the channel -----------
    // The end picker popover is still open after picking 23:00, so the first Escape
    // closes only that popover; the price dialog needs a second one.
    await page.keyboard.press('Escape')
    await expect(endPopup).not.toBeVisible()
    await expect(priceDialog).toBeVisible()

    await page.keyboard.press('Escape')
    await expect(priceDialog).not.toBeVisible()

    await archiveChannel(page, name)
    createdChannelName = null
  })
})
