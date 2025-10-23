import { defineConfig, devices } from '@playwright/test'

// Set default test environment variables
process.env.AXONHUB_ADMIN_EMAIL = process.env.AXONHUB_ADMIN_EMAIL || 'my@example.com'
process.env.AXONHUB_ADMIN_PASSWORD = process.env.AXONHUB_ADMIN_PASSWORD || 'pwd123456'
process.env.AXONHUB_API_URL = process.env.AXONHUB_API_URL || 'http://localhost:8099'

// Type declaration for process
declare const process: {
  env: Record<string, string | undefined>
}

/**
 * @see https://playwright.dev/docs/test-configuration
 */
export default defineConfig({
  testDir: './tests',
  /* Run setup test first, then run other tests in parallel */
  fullyParallel: true,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Opt out of parallel tests on CI. */
  workers: process.env.CI ? 1 : 3,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: 'html',
  /* Test match pattern - run setup.spec.ts first, then others */
  testMatch: ['**/*.spec.ts'],
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    baseURL: 'http://localhost:9527',

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: 'on-first-retry',

    /* Screenshot on failure */
    screenshot: 'only-on-failure',

    /* Video on failure */
    video: 'retain-on-failure',
  },

  /* Configure projects for major browsers */
  projects: [
    // Setup project - runs first to initialize the system
    {
      name: 'setup',
      testMatch: '**/setup.spec.ts',
      use: { ...devices['Desktop Chrome'] },
    },
    // Main test suite - runs after setup
    {
      name: 'chromium',
      testIgnore: '**/setup.spec.ts',
      dependencies: ['setup'],
      use: { ...devices['Desktop Chrome'] },
    },

    // {
    //   name: 'firefox',
    //   use: { ...devices['Desktop Firefox'] },
    // },

    // {
    //   name: 'webkit',
    //   use: { ...devices['Desktop Safari'] },
    // },

    /* Test against mobile viewports. */
    // {
    //   name: 'Mobile Chrome',
    //   use: { ...devices['Pixel 5'] },
    // },
    // {
    //   name: 'Mobile Safari',
    //   use: { ...devices['iPhone 12'] },
    // },

    /* Test against branded browsers. */
    // {
    //   name: 'Microsoft Edge',
    //   use: { ...devices['Desktop Edge'], channel: 'msedge' },
    // },
    // {
    //   name: 'Google Chrome',
    //   use: { ...devices['Desktop Chrome'], channel: 'chrome' },
    // },
  ],

  /* Run your local dev server before starting the tests */
  webServer: {
    command: 'pnpm dev',
    port: 9527,
    reuseExistingServer: !process.env.CI,
    timeout: 120 * 1000, // 2 minutes timeout
    stdout: 'pipe',
    stderr: 'pipe',
    env: {
      VITE_PORT: process.env.VITE_PORT || '9527',
      VITE_API_URL: process.env.AXONHUB_API_URL || 'http://localhost:8099',
    },
  },
})
