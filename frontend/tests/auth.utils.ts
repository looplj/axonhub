import { expect, Page } from '@playwright/test'

// Type declaration for process
declare const process: {
  env: Record<string, string | undefined>
}

export interface AdminCredentials {
  email: string
  password: string
}

const defaultCredentials: AdminCredentials = {
  email: process.env.AXONHUB_ADMIN_EMAIL || 'my@example.com',
  password: process.env.AXONHUB_ADMIN_PASSWORD || 'pwd123456',
}

export async function signInAsAdmin(page: Page, credentials: AdminCredentials = defaultCredentials) {
  // Listen for console errors
  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      console.log('Browser console error:', msg.text())
    }
  })

  // Listen for page errors
  page.on('pageerror', (error) => {
    console.log('Page error:', error.message)
  })

  // Wait for the page to fully load
  await page.waitForLoadState('domcontentloaded', { timeout: 15000 })

  // Wait for React to mount - check for root element content
  try {
    await page.waitForFunction(
      () => {
        const root = document.getElementById('root')
        return root && root.innerHTML.length > 100
      },
      { timeout: 15000 }
    )
  } catch (error) {
    console.log('Warning: Root element may not be fully loaded')
    console.log('Page URL:', page.url())

    // Check if root exists at all
    const rootExists = await page.evaluate(() => {
      const root = document.getElementById('root')
      return { exists: !!root, innerHTML: root?.innerHTML.substring(0, 200) }
    })
    console.log('Root element state:', rootExists)
  }

  // Wait for the login form to be visible using reliable test IDs
  // Fallback to multiple selectors for backward compatibility
  const emailField = page
    .getByTestId('sign-in-email')
    .or(page.locator('input[type="email"], input[name="email"]'))
    .first()

  await emailField.waitFor({ state: 'visible', timeout: 20000 })

  // Fill in credentials with test IDs and fallback selectors
  const passwordField = page
    .getByTestId('sign-in-password')
    .or(page.locator('input[type="password"], input[name="password"]'))
    .first()

  await emailField.fill(credentials.email)
  await passwordField.fill(credentials.password)

  // Click login button - use test ID with fallback
  const loginButton = page.getByTestId('sign-in-submit').or(page.getByRole('button', { name: /登录|Sign In|Sign in/i }))
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
    const timestamp = Date.now()
    await page.screenshot({ path: `test-results/sign-in-error-${timestamp}.png`, fullPage: true })
    console.log('Page URL:', page.url())
    throw error
  }

  // Wait for navigation away from sign-in page
  await page.waitForURL((url) => !url.toString().includes('/sign-in'), { timeout: 10000 })

  // Verify we're no longer on the sign-in page
  await expect(page.url()).not.toContain('/sign-in')
}

export async function ensureSignedIn(page: Page) {
  if (page.url().includes('/sign-in')) {
    await signInAsAdmin(page)
  }

  // Verify we have a valid token
  const hasToken = await page.evaluate(() => {
    const token = localStorage.getItem('axonhub_access_token')
    return !!token && token.length > 0
  })

  if (!hasToken) {
    console.warn('Warning: No valid auth token found, attempting to sign in')
    await signInAsAdmin(page)
  }
}

export async function gotoAndEnsureAuth(page: Page, path: string) {
  // Navigate to the target path - let the app handle auth redirects naturally
  await page.goto(path, { waitUntil: 'domcontentloaded', timeout: 30000 })

  // Wait for potential redirects and React to mount
  await page.waitForTimeout(2000)

  // Wait for React app to mount
  // try {
  //   await page.waitForFunction(
  //     () => {
  //       const root = document.getElementById('root')
  //       return root && root.innerHTML.length > 50
  //     },
  //     { timeout: 10000 }
  //   )
  // } catch (error) {
  //   console.log('Warning: React app may not have mounted properly')
  //   const rootState = await page.evaluate(() => {
  //     const root = document.getElementById('root')
  //     return { exists: !!root, innerHTML: root?.innerHTML.substring(0, 200) }
  //   })
  //   console.log('Root state:', rootState)
  // }

  // If we got redirected to sign-in OR the login form is rendered within the current route, perform login and navigate back
  let needsLogin = page.url().includes('/sign-in')
  try {
    // Use test IDs with fallback for more reliable detection
    const emailField = page
      .getByTestId('sign-in-email')
      .or(page.locator('input[type="email"], input[name="email"]'))
      .first()
    const passwordField = page
      .getByTestId('sign-in-password')
      .or(page.locator('input[type="password"], input[name="password"]'))
      .first()

    const emailVisible = await emailField.isVisible({ timeout: 2000 })
    const passwordVisible = await passwordField.isVisible({ timeout: 2000 })
    if (emailVisible && passwordVisible) needsLogin = true
  } catch {}

  if (needsLogin) {
    await signInAsAdmin(page)
    // After successful login, navigate to the target path
    await page.goto(path, { waitUntil: 'domcontentloaded' })
    // Wait for page to load after navigation
    await page.waitForTimeout(1000)
  }

  // Verify we have a valid token after login
  const hasToken = await page.evaluate(() => {
    const token = localStorage.getItem('axonhub_access_token')
    return !!token && token.length > 0
  })

  if (!hasToken) {
    console.warn('Warning: No valid auth token found after login')
  }

  // Final wait for page to stabilize
  try {
    await page.waitForLoadState('networkidle', { timeout: 5000 })
  } catch (error) {
    // Ignore load state timeouts to avoid masking downstream assertions.
    console.log('Network idle timeout (expected in some cases)')
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
