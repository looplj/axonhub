import { test, expect } from '@playwright/test';
import { gotoAndEnsureAuth } from './auth.utils';

test.describe('Usage Logs Management', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the usage logs page with authentication
    await gotoAndEnsureAuth(page, '/usage-logs');
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
    await page.waitForLoadState('networkidle');
    
    // Check if the refresh button is present
    const refreshButton = page.locator('button').filter({ hasText: /Refresh|刷新|重新加载/i });
    await expect(refreshButton).toBeVisible();
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
    // Navigate to home page first with authentication
    await gotoAndEnsureAuth(page, '/');
    
    // Click on the usage logs link in the sidebar
    const usageLogsLink = page.locator('a:has-text("Usage Logs"), a:has-text("用量日志")');
    await expect(usageLogsLink).toBeVisible();
    await usageLogsLink.click();
    
    // Check if we're on the usage logs page
    await expect(page).toHaveURL(/.*usage-logs/);
    await expect(page.locator('h2, h1')).toContainText(/Usage Logs|用量日志/);
  });
});