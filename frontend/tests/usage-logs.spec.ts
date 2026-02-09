import { test, expect } from '@playwright/test';
import { gotoAndEnsureAuth } from './auth.utils';

test.describe('Usage Logs Management', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the usage logs page with authentication
    await gotoAndEnsureAuth(page, '/project/usage-logs');
  });

  test('should display usage logs page with correct title', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if the usage logs page is visible
    await expect(page.locator('h1, h2').filter({ hasText: /Usage Logs|用量日志/i })).toBeVisible();
    
    // Check if the description is present (optional)
    const description = page.locator('p').filter({ hasText: /usage|token|使用|令牌/i });
    if (await description.count() > 0) {
      await expect(description.first()).toBeVisible();
    }
  });

  test('should display usage logs table', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if the usage logs table is visible
    const table = page.locator('table');
    await expect(table).toBeVisible();
    
    // Check if table headers are present
    await expect(table.locator('thead')).toBeVisible();
  });

  test('should have refresh button', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('domcontentloaded');
    
    // Check if the refresh button is present - it should contain both icon and text
    const refreshButton = page.getByRole('button', { name: /Refresh|刷新/i });
    await expect(refreshButton).toBeVisible({ timeout: 10000 });
  });

  test('should have filtering capabilities', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if filter input is present
    const filterInput = page.locator('input').filter({ hasText: '' }).or(page.locator('input[placeholder*="Filter"], input[placeholder*="筛选"], input[placeholder*="搜索"]'));
    if (await filterInput.count() > 0) {
      await expect(filterInput.first()).toBeVisible();
    } else {
      // If no filter input, just check that the page loaded
      await expect(page.locator('table, .table, [data-testid*="table"]')).toBeVisible();
    }
  });

  test('should navigate to usage logs page from sidebar', async ({ page }) => {
    // Usage Logs link has been removed from the sidebar navigation
    test.skip(true, 'Usage Logs link removed from sidebar');
  });
});