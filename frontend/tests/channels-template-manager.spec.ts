import { test, expect, type Page } from '@playwright/test'
import { signInAsAdmin } from './auth.utils'

const CREATE_OPERATION = 'mutation CreateChannelOverrideTemplate('

function uniqueName(label: string) {
  return `pw-${label}-${Date.now().toString().slice(-6)}`
}

function managerDialog(page: Page) {
  return page.getByRole('dialog').filter({ has: page.getByTestId('template-manager-create') })
}

function templateItem(page: Page, name: string) {
  return managerDialog(page).getByTestId(/^template-manager-item-/).filter({ hasText: name })
}

async function openManager(page: Page) {
  await page.goto('/sign-in')
  await signInAsAdmin(page)
  await page.goto('/channels')
  await page.getByTestId('manage-templates-button').click()
  await expect(managerDialog(page)).toBeVisible()
}

async function submitNewTemplate(page: Page, name: string) {
  const dialog = managerDialog(page)
  await dialog.getByTestId('template-manager-create').click()
  await dialog.getByTestId('template-form-name').fill(name)
  await dialog.getByTestId('template-form-submit').click()
}

// Holds the response of the next create request in the browser so a test can act
// while it is still in flight. The server has already stored the template by then.
async function holdNextCreate(page: Page) {
  let release!: () => void
  const released = new Promise<void>((resolve) => (release = resolve))
  let held = false
  const sent = page.waitForRequest((request) => request.postDataJSON()?.query?.includes(CREATE_OPERATION))

  await page.route('**/admin/graphql', async (route) => {
    if (held || !route.request().postDataJSON()?.query?.includes(CREATE_OPERATION)) return route.continue()
    held = true
    const response = await route.fetch()
    await released
    await route.fulfill({ response })
  })

  return { sent, release }
}

test.describe('Channel override template manager', () => {
  test.beforeEach(() => {
    test.setTimeout(60000)
  })

  test('selects the created template for editing once the create completes', async ({ page }) => {
    const name = uniqueName('created')
    await openManager(page)

    await submitNewTemplate(page, name)

    await expect(templateItem(page, name)).toBeVisible()
    await expect(managerDialog(page).getByTestId('template-form-name')).toHaveValue(name)
    await expect(managerDialog(page).getByTestId('template-form-submit')).toHaveText(/Save|保存/)
  })

  test('keeps the template being edited when a pending create completes', async ({ page }) => {
    const existing = uniqueName('existing')
    const edited = `${existing}-edited`
    const created = uniqueName('created')
    const dialog = managerDialog(page)
    await openManager(page)
    await submitNewTemplate(page, existing)
    await expect(templateItem(page, existing)).toBeVisible()

    const hold = await holdNextCreate(page)
    await submitNewTemplate(page, created)
    await hold.sent

    // While the create is in flight the user opens another template and edits it.
    await templateItem(page, existing).click()
    await expect(dialog.getByTestId('template-form-name')).toHaveValue(existing)
    await dialog.getByTestId('template-form-name').fill(edited)
    hold.release()

    // The list and the success toast still report the new template ...
    await expect(templateItem(page, created)).toBeVisible()
    await expect(page.getByText(/Template created successfully|模板创建成功/).last()).toBeVisible()
    // ... but the editor keeps the user's unsaved work instead of jumping to it.
    await expect(dialog.getByTestId('template-form-name')).toHaveValue(edited)
    await expect(dialog.getByTestId('template-form-submit')).toHaveText(/Save|保存/)
  })

  test('keeps a new create form opened while an earlier create is pending', async ({ page }) => {
    const existing = uniqueName('existing')
    const created = uniqueName('created')
    const dialog = managerDialog(page)
    await openManager(page)
    await submitNewTemplate(page, existing)
    await expect(templateItem(page, existing)).toBeVisible()

    const hold = await holdNextCreate(page)
    await submitNewTemplate(page, created)
    await hold.sent

    // The user leaves that form and starts a fresh one before the response arrives.
    await templateItem(page, existing).click()
    await dialog.getByTestId('template-manager-create').click()
    await dialog.getByTestId('template-add-header-op').click()
    hold.release()

    await expect(templateItem(page, created)).toBeVisible()
    await expect(dialog.getByTestId('template-form-submit')).toHaveText(/Create|创建/)
    await expect(dialog.getByTestId('template-form-name')).toHaveValue('')
    await expect(dialog.getByTestId('template-form-name')).toBeEnabled()
    await expect(dialog.getByTestId('template-add-header-op')).toBeVisible()
  })

  test('selects the created template when New template is clicked while its own create is pending', async ({ page }) => {
    const created = uniqueName('created')
    const dialog = managerDialog(page)
    await openManager(page)

    const hold = await holdNextCreate(page)
    await submitNewTemplate(page, created)
    await hold.sent

    // The form that submitted is still on screen, so this click does not replace it.
    await dialog.getByTestId('template-manager-create').click()
    hold.release()

    await expect(templateItem(page, created)).toBeVisible()
    await expect(dialog.getByTestId('template-form-submit')).toHaveText(/Save|保存/)
    await expect(dialog.getByTestId('template-form-name')).toHaveValue(created)
  })

  test('does not select the created template after the manager was closed', async ({ page }) => {
    const created = uniqueName('created')
    const dialog = managerDialog(page)
    await openManager(page)

    const hold = await holdNextCreate(page)
    await submitNewTemplate(page, created)
    await hold.sent

    await page.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
    hold.release()
    await expect(page.getByText(/Template created successfully|模板创建成功/)).toBeVisible()

    await page.getByTestId('manage-templates-button').click()
    await expect(templateItem(page, created)).toBeVisible()
    await expect(dialog.getByTestId('template-form-name')).toHaveCount(0)
  })
})


test.describe('Channel override template manager layout', () => {
  test.beforeEach(() => {
    test.setTimeout(60000)
  })

  async function boxOf(page: Page, testId: string) {
    const box = await managerDialog(page).getByTestId(testId).boundingBox()
    expect(box, `${testId} should be rendered`).not.toBeNull()
    return box!
  }

  test('keeps the editor and its operation rows inside the dialog on a phone', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 })
    const dialog = managerDialog(page)
    await openManager(page)
    await dialog.getByTestId('template-manager-create').click()
    await dialog.getByTestId('template-add-header-op').click()

    const frame = (await dialog.boundingBox())!
    for (const testId of ['template-form-name', 'template-form-description', 'header-op-path-0', 'remove-header-op-0']) {
      const box = await boxOf(page, testId)
      expect(box.x, `${testId} left edge`).toBeGreaterThanOrEqual(frame.x)
      expect(box.x + box.width, `${testId} right edge`).toBeLessThanOrEqual(frame.x + frame.width + 1)
    }

    // The panes are stacked: the editor starts below the list and the key input keeps a usable width.
    expect((await boxOf(page, 'template-form-name')).y).toBeGreaterThan((await boxOf(page, 'template-manager-create')).y)
    expect((await boxOf(page, 'header-op-path-0')).width).toBeGreaterThan(150)
  })

  test('keeps the list beside the editor on a wide screen', async ({ page }) => {
    const dialog = managerDialog(page)
    await openManager(page)
    await dialog.getByTestId('template-manager-create').click()

    const list = await boxOf(page, 'template-manager-create')
    const name = await boxOf(page, 'template-form-name')
    expect(name.x).toBeGreaterThan(list.x + list.width)
  })
})