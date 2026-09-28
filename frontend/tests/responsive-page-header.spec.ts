import { test, expect, type Locator, type Page, type ViewportSize } from '@playwright/test'
import { gotoAndEnsureAuth } from './auth.utils'

const NARROW: ViewportSize = { width: 375, height: 667 }
const SHORT: ViewportSize = { width: 375, height: 320 }
const HEADER = '[data-testid="page-header"]'
const DESCRIPTION = '[data-testid="page-header-description"]'
const TOGGLE = '[data-testid="page-header-description-toggle"]'
const MAIN = '#content > main.fixed-main'

async function openPage(page: Page, route: string, viewport: ViewportSize) {
  await page.setViewportSize(viewport)
  await gotoAndEnsureAuth(page, route)
  await expect(page.locator(HEADER)).toBeVisible()
  await expect(page.locator(MAIN)).toBeVisible()
}

async function expectHeaderClearOfBody(page: Page) {
  const geometry = await page.evaluate(({ header, main }) => {
    const heading = document.querySelector(header)!.getBoundingClientRect()
    const body = document.querySelector(main)!.getBoundingClientRect()
    return { bottom: heading.bottom, top: body.top, documentWidth: document.documentElement.scrollWidth, viewportWidth: document.documentElement.clientWidth }
  }, { header: HEADER, main: MAIN })
  expect(geometry.bottom).toBeLessThanOrEqual(geometry.top + 1)
  expect(geometry.documentWidth).toBeLessThanOrEqual(geometry.viewportWidth + 1)
}

async function expectPaginationReachable(page: Page, pagination: Locator, table: Locator) {
  const size = pagination.getByRole('combobox')
  await size.scrollIntoViewIfNeeded()
  await size.click()
  await expect(page.getByRole('option', { name: '10', exact: true })).toBeVisible()
  await page.keyboard.press('Escape')
  const rect = await size.boundingBox()
  expect(rect).not.toBeNull()
  expect(rect!.y + rect!.height).toBeLessThanOrEqual(page.viewportSize()!.height + 1)
  await expect(table).toBeVisible()
  expect(await table.evaluate((el) => el.getBoundingClientRect().height)).toBeGreaterThan(24)
  await expectHeaderClearOfBody(page)
}

test.describe('Responsive Page Header', () => {
  test('short description stays readable without a disclosure', async ({ page }) => {
    await openPage(page, '/projects', { width: 768, height: 800 })
    const description = page.locator(DESCRIPTION)
    await expect(description).toBeVisible()
    expect(await description.evaluate((el) => el.scrollHeight <= el.clientHeight + 1)).toBe(true)
    await expect(page.locator(TOGGLE)).toBeHidden()
    await expectHeaderClearOfBody(page)
  })

  test('long description expands in flow and the disclosure stays left aligned', async ({ page }) => {
    await openPage(page, '/data-storages', NARROW)
    const description = page.locator(DESCRIPTION)
    const toggle = page.locator(TOGGLE)
    await expect(toggle).toBeVisible()
    const descriptionId = await description.getAttribute('id')
    expect(descriptionId).toBeTruthy()
    await expect(toggle).toHaveAttribute('aria-controls', descriptionId!)
    await expect(toggle).toHaveAttribute('aria-expanded', 'false')
    const positions = await toggle.evaluate((el) => {
      const button = el.getBoundingClientRect()
      const range = document.createRange()
      range.selectNodeContents(el)
      return { left: button.left, textLeft: range.getBoundingClientRect().left, descriptionLeft: document.querySelector('[data-testid="page-header-description"]')!.getBoundingClientRect().left }
    })
    expect(Math.abs(positions.left - positions.descriptionLeft)).toBeLessThanOrEqual(1)
    expect(Math.abs(positions.textLeft - positions.left)).toBeLessThanOrEqual(1)
    const collapsed = await description.evaluate((el) => el.getBoundingClientRect().height)
    const bodyTop = await page.locator(MAIN).evaluate((el) => el.getBoundingClientRect().top)
    await toggle.click()
    await expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(await description.evaluate((el) => el.getBoundingClientRect().height)).toBeGreaterThan(collapsed)
    expect(await page.locator(MAIN).evaluate((el) => el.getBoundingClientRect().top)).toBeGreaterThan(bodyTop)
    await expectHeaderClearOfBody(page)
    await toggle.click()
    await expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await expectHeaderClearOfBody(page)
  })

  test('actual header container controls the 64rem disclosure boundary', async ({ page }) => {
    await openPage(page, '/data-storages', { width: 1440, height: 900 })
    const header = page.locator(HEADER)
    const description = page.locator(DESCRIPTION)
    // Query units refer to the content box. Test text is deliberately long so
    // the narrow branch cannot pass merely because the description fits anyway.
    await header.evaluate((el) => {
      const visible = el.querySelector('[data-testid="page-header-description"]')!
      const probe = visible.parentElement!.querySelector('[aria-hidden="true"]')!
      for (const copy of [visible, probe]) {
        const paragraph = copy.querySelector('p')!
        paragraph.textContent = paragraph.textContent!.repeat(8)
      }
    })
    for (const [width, wide] of [[1023.5, false], [1024, true]] as const) {
      await header.evaluate((el, value) => {
        el.style.boxSizing = 'content-box'
        el.style.width = `${value}px`
      }, width)
      await expect.poll(() => header.evaluate((el, expected) => Math.abs(el.getBoundingClientRect().width - parseFloat(getComputedStyle(el).paddingLeft) - parseFloat(getComputedStyle(el).paddingRight) - expected), width)).toBeLessThan(0.1)
      if (wide) {
        await expect(page.locator(TOGGLE)).toBeHidden()
        expect(await description.evaluate((el) => el.scrollHeight <= el.clientHeight + 1)).toBe(true)
      } else {
        expect(await description.evaluate((el) => el.scrollHeight > el.clientHeight + 1)).toBe(true)
        await expect(page.locator(TOGGLE)).toBeVisible()
      }
      await expectHeaderClearOfBody(page)
    }
  })

  test('create action remains clickable on a narrow page', async ({ page }) => {
    await openPage(page, '/data-storages', NARROW)
    const create = page.getByTestId('page-header-actions').getByRole('button', { name: /创建载荷存储|Create Payload Storage/i })
    await expect(create).toBeVisible()
    await create.click()
    await expect(page.getByRole('dialog')).toBeVisible()
    await page.keyboard.press('Escape')
    await expect(page.getByRole('dialog')).toBeHidden()
    await expectHeaderClearOfBody(page)
  })

  test('Tab traverses horizontally scrolling actions while model metadata remains visible', async ({ page }) => {
    await openPage(page, '/channels', NARROW)
    const scroller = page.getByTestId('channel-actions-scroller')
    const buttons = scroller.getByRole('button')
    const count = await buttons.count()
    expect(count).toBeGreaterThan(1)
    expect(await scroller.evaluate((el) => el.scrollWidth > el.clientWidth + 1)).toBe(true)
    // Establish the initial tab stop; all subsequent transitions use the keyboard.
    await buttons.first().focus()
    for (let i = 0; i < count; i++) {
      await expect(buttons.nth(i)).toBeFocused()
      const position = await buttons.nth(i).evaluate((el) => {
        const button = el.getBoundingClientRect()
        const scroll = el.parentElement! as HTMLElement
        const clip = scroll.getBoundingClientRect()
        return { left: button.left - clip.left, right: clip.right - button.right }
      })
      expect(position.left).toBeGreaterThanOrEqual(3)
      expect(position.right).toBeGreaterThanOrEqual(3)
      if (i < count - 1) await page.keyboard.press('Tab')
    }
    await page.keyboard.press('Shift+Tab')
    await expect(buttons.nth(count - 2)).toBeFocused()
    await expectHeaderClearOfBody(page)

    // A right-aligned scroll container is offset from the document origin;
    // focusing its already-visible first button must not scroll it needlessly.
    await openPage(page, '/channels', { width: 1100, height: 800 })
    const desktopScroller = page.getByTestId('channel-actions-scroller')
    const desktopButtons = desktopScroller.getByRole('button')
    await desktopScroller.evaluate((el) => {
      el.style.width = '500px'
      el.style.marginInlineStart = 'auto'
      el.scrollLeft = 0
      ;(document.activeElement as HTMLElement)?.blur()
    })
    const desktopOffset = await desktopScroller.evaluate((el) => el.getBoundingClientRect().left)
    expect(desktopOffset).toBeGreaterThan(100)
    await desktopButtons.first().focus()
    expect(await desktopScroller.evaluate((el) => el.scrollLeft)).toBe(0)
    await expect(desktopButtons.first()).toBeFocused()
    await expectHeaderClearOfBody(page)

    // Keep the real page/actions while opting out of the account's first-run
    // tour, whose focus trap would prevent testing the header's tab order.
    await page.route('**/admin/graphql', async (route) => {
      if (!route.request().postDataJSON()?.query?.includes('query OnboardingInfo')) return route.continue()
      const response = await route.fetch()
      const body = await response.json()
      if (body.data?.onboardingInfo?.systemModelSetting) body.data.onboardingInfo.systemModelSetting.onboarded = true
      await route.fulfill({ response, json: body })
    })
    await openPage(page, '/models', NARROW)
    const metadata = page.getByTestId('page-header-metadata')
    await expect(metadata).toBeVisible()
    expect(await metadata.evaluate((el) => el.getBoundingClientRect().height)).toBeGreaterThan(0)
    const modelScroller = page.getByTestId('model-actions-scroller')
    const modelButtons = modelScroller.getByRole('button')
    const modelCount = await modelButtons.count()
    expect(modelCount).toBeGreaterThan(1)
    expect(await modelScroller.evaluate((el) => el.scrollWidth > el.clientWidth + 1)).toBe(true)
    await modelButtons.first().focus()
    for (let i = 0; i < modelCount; i++) {
      await expect(modelButtons.nth(i)).toBeFocused()
      const position = await modelButtons.nth(i).evaluate((el) => {
        const button = el.getBoundingClientRect()
        const clip = el.parentElement!.getBoundingClientRect()
        return { left: button.left - clip.left, right: clip.right - button.right }
      })
      expect(position.left).toBeGreaterThanOrEqual(3)
      expect(position.right).toBeGreaterThanOrEqual(3)
      if (i < modelCount - 1) await page.keyboard.press('Tab')
    }
    await page.keyboard.press('Shift+Tab')
    await expect(modelButtons.nth(modelCount - 2)).toBeFocused()
    await expectHeaderClearOfBody(page)
  })

  test('short viewports keep simple and complex page pagination usable', async ({ page }) => {
    await openPage(page, '/data-storages', SHORT)
    await expect(page.locator(TOGGLE)).toBeVisible()
    await page.locator(TOGGLE).click()
    await expectPaginationReachable(page, page.getByTestId('data-storages-pagination'), page.getByTestId('data-storages-table-scroll'))

    await openPage(page, '/api-keys', SHORT)
    const table = page.getByTestId('api-keys-table-scroll')
    const pagination = page.getByTestId('api-keys-pagination')
    await expectPaginationReachable(page, pagination, table)
  })

  test('real sidebar toggle changes content width without stale disclosure', async ({ page }) => {
    await openPage(page, '/data-storages', { width: 1270, height: 800 })
    const header = page.locator(HEADER)
    await header.evaluate((el) => {
      const visible = el.querySelector('[data-testid="page-header-description"]')!
      const probe = visible.parentElement!.querySelector('[aria-hidden="true"]')!
      for (const copy of [visible, probe]) {
        const paragraph = copy.querySelector('p')!
        paragraph.textContent = paragraph.textContent!.repeat(8)
      }
    })
    const description = page.locator(DESCRIPTION)
    const toggle = page.locator(TOGGLE)
    const assertDisclosure = async (state: string) => {
      const contentWidth = await header.evaluate((el) => el.getBoundingClientRect().width - parseFloat(getComputedStyle(el).paddingLeft) - parseFloat(getComputedStyle(el).paddingRight))
      if (state === 'expanded') {
        expect(contentWidth).toBeLessThan(1024)
        expect(await description.evaluate((el) => el.scrollHeight > el.clientHeight + 1)).toBe(true)
        await expect(toggle).toBeVisible()
      } else {
        expect(contentWidth).toBeGreaterThanOrEqual(1024)
        await expect(toggle).toBeHidden()
        expect(await description.evaluate((el) => el.scrollHeight <= el.clientHeight + 1)).toBe(true)
      }
    }
    const sidebar = page.locator('[data-slot="sidebar"][data-state]')
    const trigger = page.locator('[data-sidebar="trigger"]')
    const initialState = await sidebar.getAttribute('data-state')
    expect(initialState).toMatch(/expanded|collapsed/)
    await expect.poll(() => header.evaluate((el) => el.getBoundingClientRect().width - parseFloat(getComputedStyle(el).paddingLeft) - parseFloat(getComputedStyle(el).paddingRight))).toBeLessThan(1024)
    const initialWidth = await header.evaluate((el) => el.getBoundingClientRect().width)
    await assertDisclosure(initialState!)
    await trigger.click()
    await expect(sidebar).toHaveAttribute('data-state', initialState === 'expanded' ? 'collapsed' : 'expanded')
    await expect.poll(() => header.evaluate((el, original) => Math.abs(el.getBoundingClientRect().width - original), initialWidth)).toBeGreaterThan(10)
    await assertDisclosure(initialState === 'expanded' ? 'collapsed' : 'expanded')
    await expectHeaderClearOfBody(page)
    await trigger.click()
    await expect(sidebar).toHaveAttribute('data-state', initialState!)
    await expect.poll(() => header.evaluate((el, original) => Math.abs(el.getBoundingClientRect().width - original), initialWidth)).toBeLessThan(1)
    await assertDisclosure(initialState!)
    await expectHeaderClearOfBody(page)
  })

  test('switching languages remeasures description and restores the preference', async ({ page }) => {
    await openPage(page, '/data-storages', { width: 900, height: 800 })
    const description = page.locator(DESCRIPTION)
    const toggle = page.locator(TOGGLE)
    const initialText = (await description.innerText()).replace(/\s+/g, ' ').trim()
    const initialLanguage = /配置请求|可用于/.test(initialText) ? 'zh' : 'en'
    const switcher = page.getByRole('button', { name: /toggle language|切换语言/i }).first()
    try {
      await switcher.click()
      await page.getByRole('menuitem', { name: initialLanguage === 'zh' ? 'English' : '中文' }).click()
      await expect.poll(async () => (await description.innerText()).replace(/\s+/g, ' ').trim()).not.toBe(initialText)
      await expect(toggle).toBeVisible()
      await expect(toggle).toHaveAttribute('aria-expanded', 'false')
      await toggle.click()
      await expect(toggle).toHaveAttribute('aria-expanded', 'true')
      await expectHeaderClearOfBody(page)
    } finally {
      await page.keyboard.press('Escape').catch(() => {})
      if ((await description.innerText()).replace(/\s+/g, ' ').trim() !== initialText) {
        await switcher.click()
        await page.getByRole('menuitem', { name: initialLanguage === 'zh' ? '中文' : 'English' }).click()
      }
    }
  })
})
