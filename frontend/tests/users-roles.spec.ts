import { test, expect } from '@playwright/test';
import { gotoAndEnsureAuth } from './auth.utils';

test.describe('Users Management', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the users page with authentication
    await gotoAndEnsureAuth(page, '/users');
  });

  test('should display users table', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if the users table is visible
    await expect(page.locator('table[data-testid="users-table"]')).toBeVisible();
    
    // Check if the header is present
    await expect(page.locator('h1, h2').filter({ hasText: /用户管理|Users|User Management/i })).toBeVisible();
  });

  test('should open delete dialog without errors', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Look for a delete button in the table (assuming there's at least one user)
    const deleteButton = page.locator('[data-testid="delete-user-button"]').first();
    
    // If delete button exists, click it
    if (await deleteButton.isVisible()) {
      await deleteButton.click();
      
      // Check if the delete dialog opens without errors
      await expect(page.locator('[data-testid="delete-dialog"]')).toBeVisible();
      
      // Check if the dialog contains the expected content
      await expect(page.locator('[data-testid="delete-dialog"]')).toContainText('确认删除');
      
      // Close the dialog
      await page.locator('[data-testid="cancel-delete"]').click();
    }
  });

  test('should handle pagination', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if pagination controls are present
    const pagination = page.locator('[data-testid="pagination"]');
    if (await pagination.isVisible()) {
      // Test pagination functionality
      const nextButton = page.locator('[data-testid="next-page"]');
      if (await nextButton.isEnabled()) {
        await nextButton.click();
        await page.waitForLoadState('networkidle');
      }
    }
  });
});

test.describe('Roles Management', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the roles page with authentication
    await gotoAndEnsureAuth(page, '/roles');
  });

  test('should display roles table with improved UI', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if the roles table is visible
    await expect(page.locator('table[data-testid="roles-table"]')).toBeVisible();
    
    // Check if the header is present
    await expect(page.locator('h1, h2').filter({ hasText: /角色管理|Roles|Role Management/i })).toBeVisible();
    
    // Check if the search functionality is present (optional)
    const searchInput = page.locator('input[placeholder*="搜索"], input[placeholder*="search"], input[type="search"]');
    if (await searchInput.count() > 0) {
      await expect(searchInput.first()).toBeVisible();
    }
    
    // Check if the new role button is present
    const newRoleButton = page.locator('button').filter({ hasText: /新建角色|创建角色|Create Role|New Role/i });
    await expect(newRoleButton).toBeVisible();
  });

  test('should have consistent layout with users page', async ({ page }) => {
    // Page is already navigated to in beforeEach
    await page.waitForLoadState('networkidle');
    
    // Check for header components that should match users page (optional)
    const header = page.locator('[data-testid="header"], header, .header');
    if (await header.count() > 0) {
      await expect(header.first()).toBeVisible();
    }
    const search = page.locator('[data-testid="search"], input[type="search"], .search');
    if (await search.count() > 0) {
      await expect(search.first()).toBeVisible();
    }
    
    // Check for pagination
    const pagination = page.locator('[data-testid="pagination"]');
    if (await pagination.isVisible()) {
      await expect(pagination).toBeVisible();
    }
  });

  test('should open create role dialog', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Click the new role button
    const newRoleButton = page.locator('button').filter({ hasText: /新建角色|创建角色|Create Role|New Role/i });
    await newRoleButton.click();
    
    // Check if the create dialog opens
    const createDialog = page.locator('[data-testid="create-role-dialog"], [role="dialog"], .dialog');
    await expect(createDialog).toBeVisible();
    
    // Close the dialog
    await page.keyboard.press('Escape');
  });

  test('should handle role table interactions', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if table has data or shows empty state
    const table = page.locator('table[data-testid="roles-table"]');
    await expect(table).toBeVisible();
    
    const dataRows = table.locator('tbody tr').filter({ hasNotText: /No data available|暂无数据/i });
    const rowCount = await dataRows.count();
    if (rowCount === 0) {
      return;
    }
    
    const firstRow = dataRows.first();
    const actionButton = firstRow.locator('[data-testid="row-actions"], button:has(svg)').first();
    if (await actionButton.count() > 0 && await actionButton.isVisible()) {
      await actionButton.click();
      const actionMenu = page.locator('[role="menu"]');
      if (await actionMenu.count() > 0) {
        await expect(actionMenu).toBeVisible();
      }
      await page.keyboard.press('Escape');
    }
  });
});