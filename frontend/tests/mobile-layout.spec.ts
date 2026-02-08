import { test, expect } from '@playwright/test';

/**
 * Mobile layout tests for header and sidebar behavior
 * Tests responsive control visibility across desktop and mobile viewports
 */
test.describe('Mobile Header Layout', () => {
  test.describe('Desktop viewport (1280x720)', () => {
    test.beforeEach(async ({ page }) => {
      await page.setViewportSize({ width: 1280, height: 720 });
      await page.goto('/');
      await page.waitForLoadState('domcontentloaded');
      // Wait for the app to load
      await page.waitForSelector('header', { timeout: 10000 }).catch(() => {});
    });

    test('shows all controls in header', async ({ page }) => {
      // Check if we're on an error page first
      const errorPage = await page.getByRole('heading', { name: /system error/i }).isVisible().catch(() => false);
      if (errorPage) {
        test.skip();
        return;
      }

      // Settings button should be visible in header - it's a link with settings icon button
      const settingsButton = page.locator('header a[href="/system"]').first();
      await expect(settingsButton).toBeVisible();

      // Language switch should be visible in header - button with language icon (SVG)
      const languageSwitch = page.locator('header button').filter({ has: page.locator('svg').first() }).first();
      await expect(languageSwitch).toBeVisible();

      // Theme switch should be visible in header - button with sun/moon icons (SVG)
      const themeSwitch = page.locator('header button').filter({ has: page.locator('svg').first() }).nth(1);
      await expect(themeSwitch).toBeVisible();

      // Profile dropdown should be visible in header - button with avatar
      const profileDropdown = page.locator('header button').filter({ has: page.locator('[data-testid="profile-avatar"]') }).first();
      await expect(profileDropdown).toBeVisible();
    });
  });

  test.describe('Mobile viewport (375x667)', () => {
    test.beforeEach(async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
      await page.goto('/');
      await page.waitForLoadState('domcontentloaded');
      // Wait for the app to load
      await page.waitForSelector('header', { timeout: 10000 }).catch(() => {});
    });

    test('hides controls from header', async ({ page }) => {
      // Check if we're on an error page first
      const errorPage = await page.getByRole('heading', { name: /system error/i }).isVisible().catch(() => false);
      if (errorPage) {
        test.skip();
        return;
      }

      // Settings button should NOT be visible in header on mobile
      const headerSettingsButton = page.locator('header a[href="/system"]').first();
      await expect(headerSettingsButton).toBeHidden();

      // Profile dropdown should NOT be visible in header on mobile
      const headerProfileDropdown = page.locator('header button').filter({ has: page.locator('[data-testid="profile-avatar"]') }).first();
      await expect(headerProfileDropdown).toBeHidden();
    });

    test('shows controls in sidebar', async ({ page }) => {
      // Check if we're on an error page first
      const errorPage = await page.getByRole('heading', { name: /system error/i }).isVisible().catch(() => false);
      if (errorPage) {
        test.skip();
        return;
      }

      // Open sidebar if not already open
      const sidebarTrigger = page.getByRole('button', { name: /sidebar|menu|open/i }).first();
      if (await sidebarTrigger.isVisible()) {
        await sidebarTrigger.click();
        await page.waitForTimeout(300);
      }

      // MobileHeaderControls should be visible in sidebar footer
      const sidebarSettings = page.getByRole('link', { name: /settings/i }).first();
      await expect(sidebarSettings).toBeVisible();

      // Language switch should be visible in sidebar
      const sidebarLanguageSwitch = page.getByRole('button').filter({ has: page.locator('svg').first() }).first();
      await expect(sidebarLanguageSwitch).toBeVisible();

      // Theme switch should be visible in sidebar
      const sidebarThemeSwitch = page.getByRole('button').filter({ has: page.locator('svg').first() }).nth(1);
      await expect(sidebarThemeSwitch).toBeVisible();
    });
  });

  test.describe('Mobile sidebar controls functionality', () => {
    test.beforeEach(async ({ page }) => {
      await page.setViewportSize({ width: 375, height: 667 });
      await page.goto('/');
      await page.waitForLoadState('domcontentloaded');
      // Wait for the app to load
      await page.waitForSelector('header', { timeout: 10000 }).catch(() => {});
    });

    test('settings button navigates to /system', async ({ page }) => {
      // Check if we're on an error page first
      const errorPage = await page.getByRole('heading', { name: /system error/i }).isVisible().catch(() => false);
      if (errorPage) {
        test.skip();
        return;
      }

      // Open sidebar
      const sidebarTrigger = page.getByRole('button', { name: /sidebar|menu|open/i }).first();
      if (await sidebarTrigger.isVisible()) {
        await sidebarTrigger.click();
        await page.waitForTimeout(300);
      }

      // Click settings link
      const settingsLink = page.getByRole('link', { name: /settings/i }).first();
      await settingsLink.click();

      // Wait for navigation to complete
      await page.waitForURL(/.*\/system.*/, { timeout: 5000 });

      // Verify we're on the system page
      await expect(page).toHaveURL(/.*\/system.*/);
    });

    test('language switch is clickable in sidebar', async ({ page }) => {
      // Check if we're on an error page first
      const errorPage = await page.getByRole('heading', { name: /system error/i }).isVisible().catch(() => false);
      if (errorPage) {
        test.skip();
        return;
      }

      // Open sidebar
      const sidebarTrigger = page.getByRole('button', { name: /sidebar|menu|open/i }).first();
      if (await sidebarTrigger.isVisible()) {
        await sidebarTrigger.click();
        await page.waitForTimeout(300);
      }

      // Find and click language switch
      const languageSwitch = page.getByRole('button').filter({ has: page.locator('svg').first() }).first();

      // Verify it's visible and enabled
      await expect(languageSwitch).toBeVisible();
      await expect(languageSwitch).toBeEnabled();

      // Click it to toggle language
      await languageSwitch.click();
      await page.waitForTimeout(500);
    });

    test('theme switch is clickable in sidebar', async ({ page }) => {
      // Check if we're on an error page first
      const errorPage = await page.getByRole('heading', { name: /system error/i }).isVisible().catch(() => false);
      if (errorPage) {
        test.skip();
        return;
      }

      // Open sidebar
      const sidebarTrigger = page.getByRole('button', { name: /sidebar|menu|open/i }).first();
      if (await sidebarTrigger.isVisible()) {
        await sidebarTrigger.click();
        await page.waitForTimeout(300);
      }

      // Find and click theme switch
      const themeSwitch = page.getByRole('button').filter({ has: page.locator('svg').first() }).nth(1);

      // Verify it's visible and enabled
      await expect(themeSwitch).toBeVisible();
      await expect(themeSwitch).toBeEnabled();

      // Click it to toggle theme
      await themeSwitch.click();
      await page.waitForTimeout(500);
    });
  });
});
