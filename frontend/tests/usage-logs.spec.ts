import { test, expect } from '@playwright/test';

test.describe('Usage Logs Management', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the usage logs page
    await page.goto('/usage-logs');
  });

  test('should display usage logs page with correct title', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if the usage logs page is visible
    await expect(page.locator('h2')).toContainText('Usage Logs');
    
    // Check if the description is present
    await expect(page.locator('p')).toContainText('View and analyze AI model usage and token consumption');
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
    const refreshButton = page.locator('button:has-text("Refresh")');
    await expect(refreshButton).toBeVisible();
  });

  test('should have filtering capabilities', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if filter input is present
    const filterInput = page.locator('input[placeholder*="Filter by ID"]');
    await expect(filterInput).toBeVisible();
  });

  test('should navigate to usage logs page from sidebar', async ({ page }) => {
    // Navigate to home page first
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    
    // Click on the usage logs link in the sidebar
    await page.click('a:has-text("Usage Logs")');
    
    // Check if we're on the usage logs page
    await expect(page).toHaveURL(/.*usage-logs/);
    await expect(page.locator('h2')).toContainText('Usage Logs');
  });
});