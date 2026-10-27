import { test, expect } from '@playwright/test';
import { signInAsAdmin } from './auth.utils';

// Mock the API so browser tests need only the frontend and never change live server policy.
test('Claude Code safeguards can edit, save, reload, validate, and reset', async ({ page }, testInfo) => {
  const pageErrors: string[] = [];
  page.on('pageerror', (error) => pageErrors.push(error.message));
  const user = {
    id: 'browser-test-user', email: 'my@example.com', firstName: 'Browser', lastName: 'Test',
    isOwner: true, preferLanguage: 'en', scopes: ['read_settings', 'write_settings'], roles: [], projects: [],
  };
  await page.route('**://*/admin/**', async (route) => {
    const path = new URL(route.request().url()).pathname;
    if (path === '/admin/system/status') return route.fulfill({ json: { isInitialized: true } });
    if (path === '/admin/auth/signin') {
      expect(route.request().postDataJSON()).toEqual({ email: 'my@example.com', password: 'pwd123456' });
      return route.fulfill({ json: { token: 'browser-test-token', user } });
    }
    return route.fulfill({ json: {} });
  });
  await page.route('**://*/oauth/**', (route) => route.fulfill({ json: { providers: [], oidc_login_only: false } }));
  const defaults = {
    classifierModel: 'server-reviewer', policy: '', fallbackModels: [] as string[], escalateOnDenial: false,
    attemptTimeoutSeconds: 0, timeoutSeconds: 30, maxTokens: 4096, transcriptCharBudget: 24000,
  };
  let config = { ...defaults };
  let usingServerDefaults = true;
  const updates: unknown[] = [];
  await page.route('**/admin/graphql', async (route) => {
    const body = route.request().postDataJSON();
    switch (body.operationName) {
      case 'ClaudeCodeSafeguardsSettings':
        await route.fulfill({ json: { data: { claudeCodeSafeguardsSettings: { config, usingServerDefaults, defaultPolicy: 'Built-in review policy.' } } } });
        break;
      case 'UpdateClaudeCodeSafeguardsSettings':
        updates.push(body.variables.input);
        config = body.variables.input;
        usingServerDefaults = false;
        await route.fulfill({ json: { data: { updateClaudeCodeSafeguardsSettings: true } } });
        break;
      case 'ResetClaudeCodeSafeguardsSettings':
        config = { ...defaults };
        usingServerDefaults = true;
        await route.fulfill({ json: { data: { resetClaudeCodeSafeguardsSettings: true } } });
        break;
      case 'Me':
        await route.fulfill({ json: { data: { me: user } } });
        break;
      case 'BrandSettings':
        await route.fulfill({ json: { data: { brandSettings: { brandName: 'AxonHub', brandLogo: '', title: 'AxonHub' } } } });
        break;
      case 'CheckForUpdate':
        await route.fulfill({ json: { data: { checkForUpdate: { hasUpdate: false } } } });
        break;
      default:
        await route.fulfill({ json: { data: {
          onboardingInfo: { onboarded: true, systemModelSetting: { onboarded: true }, autoDisableChannel: { onboarded: true } },
          systemGeneralSettings: { currencyCode: 'USD', timezone: 'UTC' },
          quotaRoutingSettings: { defaultMode: 'disabled' },
          userAgentPassThroughSettings: { enabled: false },
          passThroughSettings: { enabled: false },
          usageCostInjectionSettings: { enabled: false },
        } } });
    }
  });
  await page.goto('/sign-in?redirect=%2Fsystem%3Ftab%3Dclaude-code', { waitUntil: 'domcontentloaded' });
  await signInAsAdmin(page);
  await page.goto('/system?tab=claude-code', { waitUntil: 'domcontentloaded' });
  await page.getByRole('tab', { name: 'Claude Code', exact: true }).click();
  const reviewer = page.getByTestId('claude-reviewer');
  const policy = page.getByTestId('claude-policy');
  const fallbacks = page.getByTestId('claude-fallbacks');
  const save = page.getByTestId('claude-save');
  await expect(reviewer).toHaveValue('server-reviewer');
  await expect(save).toBeDisabled();
  await page.getByRole('button', { name: 'Use default as starting point', exact: true }).click();
  await expect(policy).toHaveValue('Built-in review policy.');
  await page.getByRole('button', { name: 'Use built-in policy', exact: true }).click();
  await expect(policy).toHaveValue('');
  await page.getByText('Advanced settings', { exact: true }).click();
  await page.getByLabel('Overall timeout (seconds)', { exact: true }).fill('0');
  await expect(save).toBeDisabled();
  await page.getByLabel('Overall timeout (seconds)', { exact: true }).fill('30');
  await reviewer.fill('custom-reviewer');
  await fallbacks.fill('backup-a\nbackup-b');
  await policy.fill('Allow updates to feature branches.');
  await save.click();
  await expect(save).toBeDisabled();
  expect(updates).toEqual([{ ...defaults, classifierModel: 'custom-reviewer', fallbackModels: ['backup-a', 'backup-b'], policy: 'Allow updates to feature branches.' }]);
  await page.reload({ waitUntil: 'domcontentloaded' });
  await page.getByRole('tab', { name: 'Claude Code', exact: true }).click();
  await expect(policy).toHaveValue('Allow updates to feature branches.');
  await expect(fallbacks).toHaveValue('backup-a\nbackup-b');
  await fallbacks.fill('custom-reviewer');
  await expect(save).toBeDisabled();
  await page.getByTestId('claude-reset').click();
  await expect(reviewer).toHaveValue('server-reviewer');
  await expect(policy).toHaveValue('');
  await expect(fallbacks).toHaveValue('');
  await expect(page.getByTestId('claude-reset')).toBeDisabled();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(reviewer).toBeVisible();
  const fitsViewport = await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth);
  expect(fitsViewport).toBe(true);
  await page.screenshot({ path: testInfo.outputPath('claude-code-mobile.png'), fullPage: true });

  user.isOwner = false;
  user.scopes = ['read_settings'];
  await page.reload({ waitUntil: 'domcontentloaded' });
  await expect(reviewer).toBeDisabled();
  await expect(policy).toBeDisabled();
  await expect(fallbacks).toBeDisabled();
  await expect(save).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Use default as starting point', exact: true })).toBeDisabled();
  expect(updates).toHaveLength(1);
  expect(pageErrors).toEqual([]);
});
