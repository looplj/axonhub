#!/usr/bin/env node
// UI E2E for the client-cancellation cooldown controls.
//
// Boots a throwaway AxonHub, then drives the real settings page in Chromium:
// pick the custom cooldown duration, save, reload, and verify the stored policy.
// Also exercises the project-first override, which needs the project GUID
// decoded into the numeric id the API expects.
//
// Usage (from frontend/): node tests/client-cancellation.ui.mjs

import { spawn } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { chromium } from '@playwright/test';

const APP_PORT = Number(process.env.E2E_APP_PORT || 8092);
const BASE = 'http://127.0.0.1:' + APP_PORT;
const BIN = process.env.E2E_BIN || join(process.cwd(), '..', 'axonhub.exe');
const OWNER = { email: 'e2e-ui@example.com', password: 'e2e-password-123' };

const results = [];
function check(name, ok, detail) {
  results.push({ name, ok });
  console.log((ok ? 'PASS' : 'FAIL') + '  ' + name + (detail ? '  -> ' + detail : ''));
  if (!ok) {
    process.exitCode = 1;
  }
}

const sleep = function (ms) {
  return new Promise(function (r) { setTimeout(r, ms); });
};

async function waitForHealth(timeoutMs) {
  const deadline = Date.now() + (timeoutMs || 60000);
  while (Date.now() < deadline) {
    try {
      const res = await fetch(BASE + '/health');
      if (res.ok) return true;
    } catch {
      // not up yet
    }
    await sleep(250);
  }
  throw new Error('axonhub did not become healthy');
}

function startApp(dbPath) {
  const child = spawn(BIN, [], {
    cwd: join(process.cwd(), '..'),
    env: Object.assign({}, process.env, {
      AXONHUB_SERVER_HOST: '127.0.0.1',
      AXONHUB_SERVER_PORT: String(APP_PORT),
      AXONHUB_DB_DIALECT: 'sqlite3',
      AXONHUB_DB_DSN: 'file:' + dbPath + '?_fk=1&_pragma=journal_mode(WAL)',
      AXONHUB_LOG_LEVEL: 'warn',
    }),
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stdout.on('data', function () {});
  child.stderr.on('data', function () {});
  return child;
}

let token = '';
async function gql(query, variables) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers.Authorization = 'Bearer ' + token;
  const res = await fetch(BASE + '/admin/graphql', {
    method: 'POST',
    headers: headers,
    body: JSON.stringify({ query: query, variables: variables || {} }),
  });
  const json = await res.json();
  if (json.errors && json.errors.length) throw new Error(JSON.stringify(json.errors));
  return json.data;
}

async function readPolicy() {
  const data = await gql(
    'query { retryPolicy { clientCancellationPolicy { enabled threshold cooldownMinutes action scope projectOverrides { projectId enabled threshold cooldownMinutes action } } } }'
  );
  return data.retryPolicy.clientCancellationPolicy;
}

async function main() {
  const tmp = await mkdtemp(join(tmpdir(), 'axonhub-ui-e2e-'));
  const app = startApp(join(tmp, 'ui.db'));
  let browser = null;

  try {
    await waitForHealth();

    const initRes = await fetch(BASE + '/admin/system/initialize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ownerEmail: OWNER.email,
        ownerPassword: OWNER.password,
        ownerFirstName: 'UI',
        ownerLastName: 'Owner',
        brandName: 'UI Lab',
        preferLanguage: 'en',
      }),
    });
    check('system initialize', initRes.ok, 'status=' + initRes.status);

    const signInRes = await fetch(BASE + '/admin/auth/signin', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: OWNER.email, password: OWNER.password }),
    });
    const signIn = await signInRes.json();
    token = signIn.token || '';
    check('admin sign in', Boolean(token));

    const me = await gql('query { myProjects { id name } }');
    const projectGUID = me.myProjects[0].id;

    // The bundled Playwright build may be newer than the downloaded browsers, so
    // allow pointing at an installed Chromium explicitly.
    const executablePath =
      process.env.E2E_CHROMIUM ||
      join(process.env.LOCALAPPDATA || process.env.USERPROFILE || '', 'ms-playwright', 'chromium-1228', 'chrome-win64', 'chrome.exe');
    const launchOptions = { headless: true };
    if (process.env.E2E_CHROMIUM || (await import('node:fs')).existsSync(executablePath)) {
      launchOptions.executablePath = executablePath;
    }
    browser = await chromium.launch(launchOptions);
    const context = await browser.newContext();
    await context.addInitScript(function (payload) {
      localStorage.setItem('axonhub_access_token', payload.token);
      localStorage.setItem('axonhub_user_info', JSON.stringify(payload.user));
      localStorage.setItem('axonhub_selected_project_id', payload.projectID);
      localStorage.setItem('i18nextLng', 'en');
      // Record every input/change event so a stray programmatic edit is visible.
      window.__ccEvents = [];
      document.addEventListener(
        'input',
        function (event) {
          if (event.target && event.target.id) {
            window.__ccEvents.push(['input', event.target.id, event.target.value]);
          }
        },
        true
      );
      document.addEventListener(
        'change',
        function (event) {
          if (event.target && event.target.id) {
            window.__ccEvents.push(['change', event.target.id, event.target.value]);
          }
        },
        true
      );
    }, { token: token, user: signIn.user, projectID: projectGUID });

    // The first visit shows the onboarding tour as a modal that would swallow
    // every click, so dismiss it before touching the form.
    async function dismissOverlays(currentPage) {
      for (let attempt = 0; attempt < 4; attempt += 1) {
        const overlay = await currentPage.locator('div.fixed.inset-0.bg-black\\/50').count();
        if (overlay === 0) {
          return;
        }

        const skip = currentPage
          .getByRole('button', { name: /skip|skip tour|later|not now|跳过|稍后|跳过引导/i })
          .first();
        if (await skip.isVisible().catch(function () { return false; })) {
          await skip.click().catch(function () {});
        } else {
          await currentPage.keyboard.press('Escape').catch(function () {});
        }
        await sleep(600);
      }
    }

    const page = await context.newPage();
    const gqlResponses = [];
    page.on('response', async function (response) {
      if (!response.url().includes('/admin/graphql')) return;
      try {
        const body = await response.json();
        if (body && body.data && body.data.retryPolicy) {
          gqlResponses.push(body.data.retryPolicy.clientCancellationPolicy);
        }
      } catch {
        // ignore
      }
    });
    const consoleErrors = [];
    page.on('console', function (msg) {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });

    await page.goto(BASE + '/system?tab=retry', { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('#client-cancellation', { timeout: 45000 });
    await dismissOverlays(page);
    check('client cancellation card rendered', true);

    const presetTrigger = page.locator('#client-cancellation-global-cooldown-preset');
    await presetTrigger.scrollIntoViewIfNeeded();
    await presetTrigger.click();

    const customOption = page.getByRole('option', { name: /custom|自定义/i }).first();
    await customOption.waitFor({ state: 'visible', timeout: 10000 });
    await customOption.click();

    const customInput = page.locator('#client-cancellation-global-cooldown-custom');
    const inputVisible = await customInput.isVisible().catch(function () { return false; });
    check('choosing Custom reveals the minutes input', inputVisible === true);

    if (inputVisible) {
      await customInput.fill('7');
      await customInput.blur();
    }

    const saveButton = page.locator('form button[type="submit"]').first();
    await saveButton.click();
    await sleep(1500);

    let policy = await readPolicy();
    check('custom duration persisted as 7 minutes', policy.cooldownMinutes === 7, JSON.stringify(policy));

    // Reload: the select must come back in custom mode with the stored value.
    await page.reload({ waitUntil: 'domcontentloaded' });
    await page.waitForSelector('#client-cancellation', { timeout: 45000 });
    await dismissOverlays(page);
    const reloadedInput = page.locator('#client-cancellation-global-cooldown-custom');
    const reloadedVisible = await reloadedInput.isVisible().catch(function () { return false; });
    await sleep(1200);
    const reloadedValue = reloadedVisible ? await reloadedInput.inputValue() : '';
    const triggerText = (await page.locator('#client-cancellation-global-cooldown-preset').textContent().catch(function () { return ''; })) || '';
    const storedAfterReload = await readPolicy();
    console.log(
      '  after reload: trigger="' + triggerText.trim() + '" input="' + reloadedValue + '" stored=' + JSON.stringify(storedAfterReload)
    );
    const domDump = await page.evaluate(function () {
      return Array.from(document.querySelectorAll('[id^="client-cancellation"]'))
        .map(function (el) {
          if (el.tagName === 'INPUT' || el.tagName === 'BUTTON' || el.tagName === 'TEXTAREA') {
            return el.id + '=' + (el.value === undefined ? '' : el.value) + ' [' + el.getAttribute('data-state') + ']';
          }
          return el.id;
        })
        .join(' | ');
    });
    console.log('  dom: ' + domDump);
    const detail = await page.evaluate(function () {
      const el = document.querySelector('#client-cancellation-global-cooldown-custom');
      if (!el) return null;
      const propsKey = Object.keys(el).find(function (k) { return k.indexOf('__reactProps') === 0; });
      const props = propsKey ? el[propsKey] : null;
      return {
        value: el.value,
        reactPropValue: props ? props.value : 'n/a',
        thresholdReactPropValue: (function () {
          const th = document.querySelector('#client-cancellation-global-threshold');
          const key = th && Object.keys(th).find(function (k) { return k.indexOf('__reactProps') === 0; });
          return th && key ? th[key].value : 'n/a';
        })(),
      };
    });
    console.log('  detail: ' + JSON.stringify(detail));
    console.log('  retryPolicy payloads seen by the page: ' + JSON.stringify(gqlResponses));
    const events = await page.evaluate(function () { return window.__ccEvents || []; });
    console.log('  input events: ' + JSON.stringify(events.filter(function (e) { return e[1].indexOf('cooldown') >= 0; })));
    check('reload keeps the custom mode', reloadedVisible === true, 'visible=' + reloadedVisible + ' trigger=' + triggerText.trim());
    check('reload shows the stored 7 minutes', reloadedValue === '7', 'value=' + reloadedValue);

    // Project override: the UI sends the numeric project id, not the GUID.
    const scopeTrigger = page.locator('#client-cancellation-scope');
    await scopeTrigger.scrollIntoViewIfNeeded();
    await scopeTrigger.click();
    const projectFirstOption = page.getByRole('option', { name: /project first|项目优先/i }).first();
    await projectFirstOption.waitFor({ state: 'visible', timeout: 10000 });
    await projectFirstOption.click();

    const overrideSwitch = page.locator('#client-cancellation-project-enabled');
    await overrideSwitch.scrollIntoViewIfNeeded();
    const projectSelectText = (await page.locator('#client-cancellation-project').textContent().catch(function () { return ''; })) || '';
    console.log('  project select shows: "' + projectSelectText.trim() + '"');
    // The override switch stays disabled until a project is selected.
    for (let waited = 0; waited < 30; waited += 1) {
      if (!(await overrideSwitch.isDisabled().catch(function () { return true; }))) break;
      await sleep(500);
    }
    const alreadyOn = await overrideSwitch.getAttribute('data-state').catch(function () { return null; });
    if (alreadyOn !== 'checked') {
      await overrideSwitch.click();
      await sleep(300);
    }

    await saveButton.click();
    await sleep(1500);

    policy = await readPolicy();
    const override = (policy.projectOverrides || [])[0];
    check('project override saved with a numeric project id', Boolean(override) && override.projectId > 0, JSON.stringify(policy.projectOverrides));
    check('project override kept the saved action', Boolean(override) && override.action === 'switch_and_cooldown', override ? override.action : 'missing');
    check('global scope switched to project_first', policy.scope === 'project_first', policy.scope);

    check('no console errors on the settings page', consoleErrors.length === 0, consoleErrors.slice(0, 3).join(' | '));
  } finally {
    if (browser) await browser.close().catch(function () {});
    app.kill();
    await sleep(400);
    await rm(tmp, { recursive: true, force: true }).catch(function () {});
  }

  const failed = results.filter(function (r) { return !r.ok; });
  console.log('\n' + (results.length - failed.length) + '/' + results.length + ' checks passed');
  if (failed.length) {
    console.log('failed: ' + failed.map(function (f) { return f.name; }).join(' | '));
    process.exitCode = 1;
  }
}

main().catch(function (err) {
  console.error('UI E2E crashed:', err);
  process.exitCode = 1;
});
