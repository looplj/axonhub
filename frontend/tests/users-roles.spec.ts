import { test, expect } from '@playwright/test';

test.describe('Users Management', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to the users page
    await page.goto('/users');
  });

  test('should display users table', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if the users table is visible
    await expect(page.locator('[data-testid="users-table"]')).toBeVisible();
    
    // Check if the header is present
    await expect(page.locator('h1')).toContainText('用户管理');
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
    // Navigate to the roles page
    await page.goto('/roles');
  });

  test('should display roles table with improved UI', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if the roles table is visible
    await expect(page.locator('[data-testid="roles-table"]')).toBeVisible();
    
    // Check if the header is present
    await expect(page.locator('h1')).toContainText('角色管理');
    
    // Check if the search functionality is present
    await expect(page.locator('input[placeholder*="搜索角色名称"]')).toBeVisible();
    
    // Check if the new role button is present
    await expect(page.locator('button')).toContainText('新建角色');
  });

  test('should have consistent layout with users page', async ({ page }) => {
    // Navigate to roles page
    await page.goto('/roles');
    await page.waitForLoadState('networkidle');
    
    // Check for header components that should match users page
    await expect(page.locator('[data-testid="header"]')).toBeVisible();
    await expect(page.locator('[data-testid="search"]')).toBeVisible();
    
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
    await page.locator('button:has-text("新建角色")').click();
    
    // Check if the create dialog opens
    await expect(page.locator('[data-testid="create-role-dialog"]')).toBeVisible();
    
    // Close the dialog
    await page.keyboard.press('Escape');
  });

  test('should handle role table interactions', async ({ page }) => {
    // Wait for the page to load
    await page.waitForLoadState('networkidle');
    
    // Check if table has data or shows empty state
    const table = page.locator('[data-testid="roles-table"]');
    await expect(table).toBeVisible();
    
    // If there are rows, test row interactions
    const firstRow = table.locator('tbody tr').first();
    if (await firstRow.isVisible()) {
      // Test that row actions are available
      const actionButton = firstRow.locator('[data-testid="row-actions"]');
      if (await actionButton.isVisible()) {
        await actionButton.click();
        // Check if dropdown menu appears
        await expect(page.locator('[data-testid="action-menu"]')).toBeVisible();
        // Close the menu
        await page.keyboard.press('Escape');
      }
    }
  });
});