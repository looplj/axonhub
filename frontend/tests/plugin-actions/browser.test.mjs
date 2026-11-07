import assert from 'node:assert/strict';
import { test, before, after } from 'node:test';
import { readFile, mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { createServer } from 'vite';
import react from '@vitejs/plugin-react-swc';
import { chromium, expect } from '@playwright/test';

let server, browser, cache, origin;
const frontend = resolve(import.meta.dirname, '../..');
const bridge = await readFile(resolve(frontend, '../internal/server/plugin/uisdk/bridge.js'), 'utf8');
before(async () => {
  cache = await mkdtemp(join(tmpdir(), 'axonhub-actions-test-'));
  server = await createServer({
    configFile: false, root: frontend, cacheDir: cache,
    plugins: [react()], resolve: { alias: { '@': resolve(frontend, 'src') } },
    server: { host: '127.0.0.1', port: 0 }, logLevel: 'error',
  });
  await server.listen();
  origin = `http://127.0.0.1:${server.httpServer.address().port}`;
  browser = await chromium.launch({ headless: true, channel: process.env.PLAYWRIGHT_CHANNEL || undefined });
});
after(async () => { await browser?.close(); await server?.close(); if (cache) await rm(cache, { recursive: true, force: true }); });

async function fixture({ mobile = false, withPage = false, noHello = false } = {}) {
  let silent = noHello;
  const page = await browser.newPage();
  const calls = [], sessions = [];
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  await page.route('**/admin/graphql', async route => {
    const body = route.request().postDataJSON();
    if (body.query.includes('createPluginUISession')) {
      sessions.push(body.variables);
      return route.fulfill({ json: { data: { createPluginUISession: { token: `token${sessions.length}`, config: {}, canWriteKV: true } } } });
    }
    return route.fulfill({ json: { data: { pluginUIEntries: { pages: [], actions: [1, 2, 3].map(n => ({
      pluginID: 'example.actions', key: `a${n}`, slot: 'app.header.actions', title: { en: `Action ${n}`, 'zh-CN': `操作 ${n}` },
      icon: 'search', effect: { type: 'dialog', entry: 'ui/action.html' }, revision: 'v1',
    })) } } } });
  });
  await page.route('**/admin/plugins/**', async route => {
    calls.push({ url: route.request().url(), headers: route.request().headers() });
    return route.fulfill({ json: route.request().url().endsWith('/fetch') ? { status: 200, headers: {}, body: 'ok' } : { data: { __typename: 'Query' } } });
  });
  await page.route('**/admin/plugin-ui/**', async route => {
    const isBridge = route.request().url().endsWith('/_axonhub/bridge.js');
    return route.fulfill({ headers: { 'Access-Control-Allow-Origin': '*', 'Content-Security-Policy': "sandbox allow-scripts allow-forms; default-src 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'none'; frame-ancestors 'self'" },
      contentType: isBridge ? 'text/javascript' : 'text/html', body: isBridge ? bridge : silent ? '<html>no handshake</html>' : `<!doctype html><html><body>
      <pre id="init"></pre><span id="theme"></span><button id="query">Query</button><button id="fetch">Fetch</button><button id="navigate">Navigate</button>
      <script src="../_axonhub/bridge.js"></script><script>
      axonhub.ready().then(init => document.querySelector('#init').textContent = JSON.stringify(init));
      axonhub.onTheme(theme => document.querySelector('#theme').textContent = theme.theme);
      document.querySelector('#query').onclick = () => axonhub.graphql('{ __typename }');
      document.querySelector('#fetch').onclick = () => axonhub.fetch('https://example.com/status');
      document.querySelector('#navigate').onclick = () => axonhub.navigate('/');
      </script></body></html>` });
  });
  await page.goto(`${origin}/tests/plugin-actions/index.html?${new URLSearchParams({ ...(mobile ? { mobile: '1' } : {}), ...(withPage ? { withPage: '1' } : {}) })}`);
  return { page, calls, sessions, errors, enableHello: () => { silent = false; } };
}

test('desktop actions load lazily, use global scope, and dispose on close/navigation/disable', async () => {
  const { page, calls, sessions, errors } = await fixture();
  try {
    await expect(page.getByTestId('plugin-header-action')).toHaveCount(2);
    await expect(page.locator('iframe')).toHaveCount(0);
    assert.equal(sessions.length, 0);
    await page.getByTestId('plugin-header-action').first().click();
    const frame = page.frameLocator('iframe');
    await expect(frame.locator('#init')).toContainText('app.header.actions');
    const init = JSON.parse(await frame.locator('#init').textContent());
    assert.equal(init.projectId, null); assert.equal(init.page, null); assert.equal(init.action, 'a1'); assert.deepEqual(init.context, {});
    await frame.locator('#query').click(); await frame.locator('#fetch').click();
    await expect.poll(() => calls.length).toBe(2);
    assert.ok(calls.every(call => !('x-project-id' in call.headers)));
    await page.evaluate(() => window.__pluginTest.project());
    assert.equal(sessions.length, 1);
    await page.getByRole('button', { name: 'Close', exact: true }).click();
    await expect(page.locator('iframe')).toHaveCount(0);
    await expect(page.getByTestId('plugin-header-action').first()).toBeFocused();
    await page.getByTestId('plugin-header-action').first().click();
    await expect.poll(() => sessions.length).toBe(2);
    await expect(frame.locator('#init')).toContainText('app.header.actions');
    await frame.locator('#navigate').click();
    await expect(page.locator('iframe')).toHaveCount(0);
    await page.getByTestId('plugin-header-action').first().click();
    await expect(frame.locator('#init')).toContainText('app.header.actions');
    await page.evaluate(() => window.__pluginTest.remove());
    await expect(page.locator('iframe')).toHaveCount(0);
    await expect(page.getByTestId('plugin-header-action')).toHaveCount(0);
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});

test('overflow and mobile menus open a persistent dialog; locale change creates a fresh session', async () => {
  for (const mobile of [false, true]) {
    const { page, sessions, errors } = await fixture({ mobile });
    try {
      await expect(page.getByTestId('plugin-header-action')).toHaveCount(mobile ? 0 : 2);
      await page.getByTestId('plugin-header-overflow').click();
      await expect(page.getByRole('menuitem')).toHaveCount(mobile ? 3 : 1);
      await page.getByRole('menuitem').last().click();
      await expect(page.frameLocator('iframe').locator('#init')).toContainText('"action":"a3"');
      await expect(page.getByRole('menuitem')).toHaveCount(0);
      await page.evaluate(() => window.__pluginTest.locale());
      await expect(page.getByRole('heading', { name: '操作 3' })).toBeVisible();
      await expect.poll(() => sessions.length).toBe(2);
      await expect(page.frameLocator('iframe').locator('#init')).toContainText('"locale":"zh"');
      await page.getByRole('button', { name: 'Close', exact: true }).click();
      await expect(page.getByTestId('plugin-header-overflow')).toBeFocused();
      assert.deepEqual(errors, []);
    } finally { await page.close(); }
  }
});


test('handshake timeout stays inside the dialog and retry uses a new instance', async () => {
  const { page, sessions, enableHello } = await fixture({ noHello: true });
  try {
    await page.clock.install();
    await page.getByTestId('plugin-header-action').first().click();
    await expect(page.frameLocator('iframe').locator('body')).toContainText('no handshake');
    await page.clock.runFor(10_100);
    await expect(page.getByTestId('plugin-frame-error')).toBeVisible();
    await expect(page.getByText('Host content')).toBeVisible();
    enableHello();
    await page.getByTestId('plugin-frame-retry').click();
    await expect(page.frameLocator('iframe').locator('#init')).toContainText('app.header.actions');
    assert.equal(sessions.length, 2);
  } finally { await page.close(); }
});

test('page and action sharing an entry get separate tokens, scope and theme messages', async () => {
  const { page, sessions, errors } = await fixture({ withPage: true });
  try {
    await expect(page.frameLocator('iframe').locator('#init')).toContainText('"page":"main"');
    await page.getByTestId('plugin-header-action').first().click();
    await expect(page.locator('iframe')).toHaveCount(2);
    const dialogFrame = page.getByTestId('plugin-action-dialog').frameLocator('iframe');
    await expect(dialogFrame.locator('#init')).toContainText('"action":"a1"');
    const sources = await page.locator('iframe').evaluateAll(frames => frames.map(frame => frame.src));
    assert.equal(new Set(sources).size, 2);
    assert.equal(sessions.length, 2);
    await page.evaluate(() => window.__pluginTest.theme('dark'));
    await expect(dialogFrame.locator('#theme')).toHaveText('dark');
    await page.getByRole('button', { name: 'Close', exact: true }).click();
    await expect(page.locator('iframe')).toHaveCount(1);
    await expect(page.frameLocator('iframe').locator('#theme')).toHaveText('dark');
    const init = JSON.parse(await page.frameLocator('iframe').locator('#init').textContent());
    assert.equal(init.projectId, 17);
    assert.equal(init.action, null);
    assert.deepEqual(errors, []);
  } finally { await page.close(); }
});
