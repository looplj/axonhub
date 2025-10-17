import { faker } from '@faker-js/faker'
import { test, expect } from '@playwright/test'
import { signInAsAdmin } from './auth.utils'

// Type declaration for process
declare const process: {
  env: Record<string, string | undefined>
}

/**
 * Setup test that runs first to initialize the system
 * This test creates the initial owner account with random credentials
 */
test.describe.configure({ mode: 'serial' })

test.describe('System Setup', () => {
  test('initialize system with owner account', async ({ page }) => {
    // Listen for console messages and errors
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        console.log(`Browser console error: ${msg.text()}`)
      }
    })

    // Listen for page errors
    page.on('pageerror', (error) => {
      console.log(`Page error: ${error.message}`)
    })

    // Listen for failed requests
    page.on('requestfailed', (request) => {
      console.log(`Request failed: ${request.url()} - ${request.failure()?.errorText}`)
    })

    // Generate random owner credentials
    const ownerEmail = process.env.AXONHUB_ADMIN_EMAIL || 'my@example.com'
    const ownerPassword = process.env.AXONHUB_ADMIN_PASSWORD || 'pwd123456'

    // Store credentials in environment for other tests to use
    process.env.AXONHUB_ADMIN_EMAIL = ownerEmail

    console.log(`Initializing system with owner: ${ownerEmail}`)

    // Navigate to the app - should redirect to initialization if system is not initialized
    await page.goto('/', { waitUntil: 'domcontentloaded' })
    await page.waitForTimeout(2000)

    // Check if we're on initialization page or sign-in page
    const currentUrl = page.url()

    if (currentUrl.includes('/initialization')) {
      // System needs initialization
      console.log('System requires initialization')

      // Wait for initialization form
      await page.getByRole('textbox', { name: /Owner First Name/i }).waitFor({ timeout: 10000 })

      // Fill in all required fields
      await page.getByRole('textbox', { name: /Owner First Name/i }).fill('Admin')
      await page.getByRole('textbox', { name: /Owner Last Name/i }).fill('User')
      await page.getByRole('textbox', { name: /Owner Email/i }).fill(ownerEmail)
      await page.getByLabel(/Owner Password/i).fill(ownerPassword)
      await page.getByRole('textbox', { name: /Brand Name/i }).fill('AxonHub')

      // Submit initialization form
      const submitButton = page.getByRole('button', { name: /Initialize System|初始化系统/i })
      await expect(submitButton).toBeVisible()

      // After initialization, system redirects to sign-in page
      await Promise.all([
        page.waitForURL((url) => url.toString().includes('/sign-in'), {
          timeout: 15000,
        }),
        submitButton.click(),
      ])

      console.log('System initialized successfully, now on sign-in page')

      // Now sign in with the owner credentials using the utility function
      await signInAsAdmin(page, { email: ownerEmail, password: ownerPassword })
      console.log('Signed in successfully after initialization')
    } else if (currentUrl.includes('/sign-in')) {
      // System is already initialized, just sign in
      console.log('System already initialized, signing in')

      await signInAsAdmin(page, { email: ownerEmail, password: ownerPassword })
      console.log('Signed in successfully')
    } else {
      // Already logged in or on dashboard
      console.log('System already initialized and logged in')
    }

    // Verify we're logged in by checking we're not on sign-in or initialization page
    await expect(page.url()).not.toContain('/500')
    await expect(page.url()).not.toContain('/sign-in')
    await expect(page.url()).not.toContain('/initialization')

    console.log(`Setup complete. Owner email: ${ownerEmail}`)
  })
})
