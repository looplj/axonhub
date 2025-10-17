import { expect, Page } from '@playwright/test'

// Type declaration for process
declare const process: {
  env: Record<string, string | undefined>;
};

export interface AdminCredentials {
  email: string
  password: string
}

const defaultCredentials: AdminCredentials = {
  email: process.env.AXONHUB_ADMIN_EMAIL || 'my@example.com',
  password: process.env.AXONHUB_ADMIN_PASSWORD || 'pwd123456'
}

export async function signInAsAdmin(page: Page, credentials: AdminCredentials = defaultCredentials) {
  // await page.goto('/sign-in')
  // await page.waitForLoadState('domcontentloaded')

  // Wait for the login form to be visible
  await page.waitForSelector('input[type="email"], input[name="email"]', { timeout: 10000 })
  
  // Fill in credentials with more specific selectors
  const emailField = page.locator('input[type="email"], input[name="email"]').first()
  const passwordField = page.locator('input[type="password"], input[name="password"]').first()
  
  await emailField.fill(credentials.email)
  await passwordField.fill(credentials.password)

  // Click login button and wait for navigation
  const loginButton = page.getByRole('button', { name: /登录|Sign In|Sign in/i })
  await expect(loginButton).toBeVisible()
  
  // Wait for the sign-in API response before checking navigation
  const responsePromise = page.waitForResponse(
    (response) => response.url().includes('/admin/auth/signin') && response.status() === 200,
    { timeout: 15000 }
  )

  await loginButton.click()

  try {
    await responsePromise
  } catch (error) {
    console.log(`Sign-in API error: ${error}`)
    // Take a screenshot for debugging
    await page.screenshot({ path: 'sign-in-error.png', fullPage: true })
    throw error
  }

  // Wait for navigation away from sign-in page
  await page.waitForURL(url => !url.toString().includes('/sign-in'), { timeout: 10000 })

  // Verify we're no longer on the sign-in page
  await expect(page.url()).not.toContain('/sign-in')
}

export async function ensureSignedIn(page: Page) {
  if (page.url().includes('/sign-in')) {
    await signInAsAdmin(page)
  }

  // Ensure we have a valid authentication state
  await page.addInitScript(() => {
    const TOKEN_KEY = 'axonhub_access_token'
    const LEGACY_TOKEN_KEY = 'axonhub.auth.accessToken'
    const token = window.localStorage.getItem(TOKEN_KEY) || window.localStorage.getItem(LEGACY_TOKEN_KEY)
    if (!token) {
      window.localStorage.setItem(TOKEN_KEY, 'test-token')
      // Keep legacy key for backward compatibility if any code still reads it
      window.localStorage.setItem(LEGACY_TOKEN_KEY, 'test-token')
    }
  })
}

export async function gotoAndEnsureAuth(page: Page, path: string) {
  // Seed auth token BEFORE any navigation so app requests are authenticated
  await page.addInitScript(() => {
    const TOKEN_KEY = 'axonhub_access_token'
    const LEGACY_TOKEN_KEY = 'axonhub.auth.accessToken'
    const token = window.localStorage.getItem(TOKEN_KEY) || window.localStorage.getItem(LEGACY_TOKEN_KEY)
    if (!token) {
      window.localStorage.setItem(TOKEN_KEY, 'test-token')
      window.localStorage.setItem(LEGACY_TOKEN_KEY, 'test-token')
    }
  })
  // Also set immediately for the current document in case it's already loaded
  try {
    await page.evaluate(() => {
      const TOKEN_KEY = 'axonhub_access_token'
      const LEGACY_TOKEN_KEY = 'axonhub.auth.accessToken'
      const token = window.localStorage.getItem(TOKEN_KEY) || window.localStorage.getItem(LEGACY_TOKEN_KEY)
      if (!token) {
        window.localStorage.setItem(TOKEN_KEY, 'test-token')
        window.localStorage.setItem(LEGACY_TOKEN_KEY, 'test-token')
      }
    })
  } catch {}

  // Now navigate to the target path
  // First, try to navigate to the target path
  await page.goto(path, { waitUntil: 'domcontentloaded' })

  // Wait for potential redirects
  await page.waitForTimeout(1000)
  
  // If we got redirected to sign-in OR the login form is rendered within the current route, perform login and navigate back
  let needsLogin = page.url().includes('/sign-in')
  try {
    const emailVisible = await page.locator('input[type="email"], input[name="email"]').first().isVisible()
    const passwordVisible = await page.locator('input[type="password"], input[name="password"]').first().isVisible()
    if (emailVisible && passwordVisible) needsLogin = true
  } catch {}

  if (needsLogin) {
    await signInAsAdmin(page)
    // After successful login, navigate to the target path
    await page.goto(path, { waitUntil: 'domcontentloaded' })
  }

  // Ensure we have a valid authentication state
  await page.addInitScript(() => {
    const TOKEN_KEY = 'axonhub_access_token'
    const LEGACY_TOKEN_KEY = 'axonhub.auth.accessToken'
    const token = window.localStorage.getItem(TOKEN_KEY) || window.localStorage.getItem(LEGACY_TOKEN_KEY)
    if (!token) {
      window.localStorage.setItem(TOKEN_KEY, 'test-token')
      // Keep legacy key for backward compatibility if any code still reads it
      window.localStorage.setItem(LEGACY_TOKEN_KEY, 'test-token')
    }
  })

  try {
    await page.waitForLoadState('networkidle', { timeout: 10000 })
  } catch (error) {
    // Ignore load state timeouts to avoid masking downstream assertions.
  }
}

export async function waitForGraphQLOperation(page: Page, operationName: string) {
  const lowerCamel = operationName.length
    ? operationName.charAt(0).toLowerCase() + operationName.slice(1)
    : operationName
  try {
    await Promise.race([
      page.waitForResponse((response) => {
        const url = response.url()
        const isGraphQL = url.includes('/admin/graphql') || url.includes('/graphql')
        if (!isGraphQL) return false
        const body = response.request().postData()
        if (!body) return false
        return body.includes(operationName) || body.includes(lowerCamel)
      }),
      // Fallback to a short timeout to avoid hard failures when backend is unavailable
      page.waitForTimeout(4000),
    ])
  } catch {
    // Swallow errors to keep tests resilient in environments without backend
  }
}
