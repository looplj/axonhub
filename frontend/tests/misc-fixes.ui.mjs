#!/usr/bin/env node
// UI regression checks for three fixes:
//  1. channel edit -> key group -> "View keys" must not clobber the keys nor
//     raise the duplicate-key validation error.
//  2. model association rule rows must actually reorder after a drag.
//  3. request logs page exposes a manual "Clean logs" dialog.
//
// Usage (from frontend/): node tests/misc-fixes.ui.mjs

import { spawn } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { existsSync } from 'node:fs';
import { chromium } from '@playwright/test';

const APP_PORT = Number(process.env.E2E_APP_PORT || 8093);
const BASE = 'http://127.0.0.1:' + APP_PORT;
const BIN = process.env.E2E_BIN || join(process.cwd(), '..', 'axonhub.exe');
const OWNER = { email: 'misc-ui@example.com', password: 'e2e-password-123' };

const results = [];
function check(name, ok, detail) {
  results.push({ name, ok });
  console.log((ok ? 'PASS' : 'FAIL') + '  ' + name + (detail ? '  -> ' + detail : ''));
  if (!ok) process.exitCode = 1;
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function waitForHealth(timeoutMs = 60000) {
  const deadline = Date.now() + timeoutMs;
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
  child.stdout.on('data', () => {});
  child.stderr.on('data', () => {});
  return child;
}

let token = '';
async function gql(query, variables) {
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers.Authorization = 'Bearer ' + token;
  const res = await fetch(BASE + '/admin/graphql', {
    method: 'POST',
    headers,
    body: JSON.stringify({ query, variables: variables || {} }),
  });
  const json = await res.json();
  if (json.errors && json.errors.length) throw new Error(JSON.stringify(json.errors));
  return json.data;
}

const guidToInt = (id) => Number(String(id).split('/').pop());

async function main() {
  const tmp = await mkdtemp(join(tmpdir(), 'axonhub-misc-e2e-'));
  const app = startApp(join(tmp, 'misc.db'));
  let browser = null;

  try {
    await waitForHealth();

    await fetch(BASE + '/admin/system/initialize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ownerEmail: OWNER.email,
        ownerPassword: OWNER.password,
        ownerFirstName: 'Misc',
        ownerLastName: 'Owner',
        brandName: 'Misc Lab',
        preferLanguage: 'en',
      }),
    });
    const signIn = await (
      await fetch(BASE + '/admin/auth/signin', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: OWNER.email, password: OWNER.password }),
      })
    ).json();
    token = signIn.token || '';
    check('admin sign in', Boolean(token));

    const me = await gql('query { myProjects { id name } }');
    const projectGUID = me.myProjects[0].id;

    // Two channels: one with grouped keys (the reported scenario), one spare.
    const chA = await gql(
      'mutation ($input: CreateChannelInput!) { createChannel(input: $input) { id name status } }',
      {
        input: {
          type: 'openai',
          name: 'e2e-keygroup',
          baseURL: 'https://api.example.com/v1',
          credentials: {
            keyGroups: [
              { name: 'group-one', apiKeys: ['sk-group-one-a', 'sk-group-one-b'], models: [], enabled: true },
              { name: 'group-two', apiKeys: ['sk-group-two-a'], models: [], enabled: true },
            ],
          },
          supportedModels: ['gpt-4'],
          defaultTestModel: 'gpt-4',
          policies: { apiKeyAutoDisableMode: 'inherit' },
        },
      }
    );
    const chB = await gql(
      'mutation ($input: CreateChannelInput!) { createChannel(input: $input) { id name status } }',
      {
        input: {
          type: 'openai',
          name: 'e2e-assoc-b',
          baseURL: 'https://api.example.com/v1',
          credentials: { apiKeys: ['sk-assoc-b'] },
          supportedModels: ['gpt-4'],
          defaultTestModel: 'gpt-4',
          policies: { apiKeyAutoDisableMode: 'inherit' },
        },
      }
    );
    check('channels created', Boolean(chA.createChannel.id && chB.createChannel.id));
    for (const row of [chA.createChannel, chB.createChannel]) {
      await gql('mutation ($id: ID!, $status: ChannelStatus!) { updateChannelStatus(id: $id, status: $status) { id status } }', {
        id: row.id,
        status: 'enabled',
      });
    }

    const channelAInt = guidToInt(chA.createChannel.id);
    const channelBInt = guidToInt(chB.createChannel.id);

    await gql('mutation ($input: CreateModelInput!) { createModel(input: $input) { id modelID } }', {
      input: {
        developer: 'openai',
        modelID: 'gpt-4',
        type: 'chat',
        name: 'GPT-4',
        icon: 'OpenAI',
        group: 'openai',
        modelCard: {},
        settings: {
          associations: [
            { type: 'channel_model', priority: 0, channelModel: { channelId: channelAInt, modelId: 'gpt-4' } },
            { type: 'channel_model', priority: 1, channelModel: { channelId: channelBInt, modelId: 'gpt-4' } },
          ],
        },
      },
    });

    browser = await chromium.launch({
      headless: true,
      executablePath:
        process.env.E2E_CHROMIUM ||
        (existsSync(join(process.env.LOCALAPPDATA || '', 'ms-playwright', 'chromium-1228', 'chrome-win64', 'chrome.exe'))
          ? join(process.env.LOCALAPPDATA || '', 'ms-playwright', 'chromium-1228', 'chrome-win64', 'chrome.exe')
          : undefined),
    });
    const context = await browser.newContext();
    await context.addInitScript(
      ({ token, user, projectID }) => {
        localStorage.setItem('axonhub_access_token', token);
        localStorage.setItem('axonhub_user_info', JSON.stringify(user));
        localStorage.setItem('axonhub_selected_project_id', projectID);
        localStorage.setItem('i18nextLng', 'en');
      },
      { token, user: signIn.user, projectID: projectGUID }
    );
    const page = await context.newPage();
    const consoleErrors = [];
    page.on('console', (msg) => {
      // Ignore the external font/CDN fetch failures of this sandbox.
      if (msg.type() === 'error' && !msg.text().includes('ERR_TIMED_OUT') && !msg.text().includes('Failed to load resource')) {
        consoleErrors.push(msg.text());
      }
    });

    // Two different overlays can gate the page: the settings onboarding modal and
    // the driver.js product tour. Both intercept pointer events, so clear them.
    async function clearDriver() {
      await page
        .evaluate(() => {
          document.querySelectorAll('.driver-overlay, .driver-popover, .driver-active-element').forEach((el) => el.remove());
          document.body.classList.remove('driver-active', 'driver-fade');
        })
        .catch(() => {});
    }

    async function dismissOverlays() {
      for (let attempt = 0; attempt < 6; attempt += 1) {
        const driver = await page.locator('.driver-overlay, .driver-popover').count();
        const modal = await page.locator('div.fixed.inset-0.bg-black\\/50').count();
        if (driver === 0 && modal === 0) return;

        if (driver > 0) {
          const close = page.locator('.driver-popover-close-btn').first();
          if (await close.isVisible().catch(() => false)) {
            await close.click().catch(() => {});
          }
          await page.keyboard.press('Escape').catch(() => {});
          await page
            .evaluate(() => {
              document.querySelectorAll('.driver-overlay, .driver-popover, .driver-active-element').forEach((el) => el.remove());
              document.body.classList.remove('driver-active', 'driver-fade');
            })
            .catch(() => {});
        }

        if (modal > 0) {
          const skip = page.getByRole('button', { name: /skip|later|not now/i }).first();
          if (await skip.isVisible().catch(() => false)) {
            await skip.click().catch(() => {});
          } else {
            await page.keyboard.press('Escape').catch(() => {});
          }
        }

        await sleep(500);
      }
    }

    async function openAssociationDialog() {
      const dialogOpen = await page
        .getByRole('dialog')
        .first()
        .isVisible()
        .catch(() => false);
      if (dialogOpen) {
        await page.keyboard.press('Escape').catch(() => {});
        await page.waitForTimeout(400);
      }
      const trigger = page.locator('table').getByRole('button', { name: /^[0-9]+$/ }).first();
      await trigger.click();
      await page.waitForSelector('[role="dialog"]', { timeout: 20000 });
      await page.waitForTimeout(1200);
    }

    async function dragHandle(from, to) {
      const a = await from.boundingBox();
      const b = await to.boundingBox();
      if (!a || !b) {
        throw new Error('drag handle is not visible');
      }
      await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
      await page.mouse.down();
      await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2 + 12, { steps: 6 });
      await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2, { steps: 16 });
      await page.mouse.up();
    }

    async function saveDialog() {
      await clearDriver();
      const saveButton = page.getByRole('dialog').first().getByRole('button', { name: /^save$/i }).first();
      if (await saveButton.isVisible().catch(() => false)) {
        await saveButton.click();
      } else {
        console.log('  save button not found in dialog');
      }
      await page.waitForTimeout(2500);
    }

    async function readOrderedAssociations() {
      const data = await gql(
        'query { models(first: 10) { edges { node { modelID settings { associations { type priority channelModel { channelId } } } } } } }'
      );
      const model = (data.models.edges[0] || {}).node;
      const associations = (model && model.settings && model.settings.associations) || [];
      return associations
        .slice()
        .sort((x, y) => (x.priority ?? 0) - (y.priority ?? 0))
        .map((assoc) => (assoc.channelModel ? assoc.channelModel.channelId : null));
    }

    // ---------------------------------------------------------------- fix 1
    await page.goto(BASE + '/channels', { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('table', { timeout: 45000 });
    await dismissOverlays();
    const channelRow = page.locator('tr', { hasText: 'e2e-keygroup' }).first();
    await channelRow.locator('td').last().locator('button').first().click();
    await page.waitForSelector('[role="dialog"]', { timeout: 30000 });
    await page.waitForTimeout(800);

    const groupKeys = page.locator('[data-testid="channel-key-group-keys-0"]');
    await groupKeys.scrollIntoViewIfNeeded();
    check('key group editor rendered', (await groupKeys.count()) === 1);

    // Focus the masked textarea first: clicking any button then blurs it, which is
    // exactly what used to write the mask back as the key list.
    await groupKeys.click();
    const viewKeys = page.getByRole('button', { name: 'View keys' }).first();
    await viewKeys.click();
    await sleep(800);

    await page.getByRole('button', { name: 'Click the eye icon to reveal and edit API keys' }).first().click();
    await sleep(500);
    const revealed = await groupKeys.inputValue();
    check('revealing keys shows the real key list', revealed.includes('sk-group-one-a') && revealed.includes('sk-group-one-b'), JSON.stringify(revealed));

    const dialogText = await page.locator('form').first().innerText().catch(() => '');
    check(
      'no duplicate-key validation error after view keys',
      !dialogText.includes('The same API key cannot appear in more than one group') && !dialogText.includes('duplicateKey'),
      dialogText.includes('duplicateKey') ? 'duplicateKey present' : 'clean'
    );

    const panelText = await page.getByRole('dialog').first().innerText().catch(() => dialogText);
    check(
      'key panel lists both keys of the group',
      panelText.includes('sk-group-one-a') && panelText.includes('sk-group-one-b'),
      panelText.includes('sk-group-one-a') ? 'keys visible' : 'keys missing'
    );

    await page.keyboard.press('Escape').catch(() => {});

    // ---------------------------------------------------------------- fix 2
    await page.goto(BASE + '/models', { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('table', { timeout: 45000 });
    await dismissOverlays();
    await openAssociationDialog();
    await clearDriver();

    const ruleHandles = page.getByRole('button', { name: 'Drag to reorder', exact: true });
    const previewHandles = page.getByRole('button', { name: 'Drag to reorder preview' });
    check('rule rows expose drag handles', (await ruleHandles.count()) >= 2, 'rule handles=' + (await ruleHandles.count()));
    check('preview rows expose drag handles', (await previewHandles.count()) >= 2, 'preview handles=' + (await previewHandles.count()));

    const dialog = page.getByRole('dialog').first();
    const orderBefore = await dialog.innerText();
    check(
      'initial order starts with the first channel',
      orderBefore.indexOf('e2e-keygroup') >= 0 && orderBefore.indexOf('e2e-keygroup') < orderBefore.indexOf('e2e-assoc-b'),
      'ok'
    );

    const previewDebug = await page.evaluate(() =>
      Array.from(document.querySelectorAll('[aria-label="Drag to reorder preview"]')).map((el, index) => {
        const rect = el.getBoundingClientRect();
        const card = el.parentElement ? el.parentElement.textContent : '';
        return index + ' y=' + Math.round(rect.y) + ' x=' + Math.round(rect.x) + ' text=' + card.slice(0, 40).replace(/s+/g, ' ');
      })
    );
    console.log('  preview rows: ' + JSON.stringify(previewDebug));

    // Drag the second preview row above the first one: this is the gesture the
    // operator reported as "drag does nothing".
    // Row 2 is the other channel: dragging it above row 0 must reorder channels.
    await dragHandle(previewHandles.nth(2), previewHandles.nth(0));
    await page.waitForTimeout(600);

    const orderAfterPreview = await dialog.innerText();
    const previewFirst = orderAfterPreview.indexOf('e2e-assoc-b') < orderAfterPreview.indexOf('e2e-keygroup');
    check('preview rows reorder in place after the drop', previewFirst, previewFirst ? 'reordered' : 'unchanged');

    await saveDialog();
    let orderedChannels = await readOrderedAssociations();
    check(
      'preview drag persisted the new priority order',
      orderedChannels[0] === channelBInt && orderedChannels[1] === channelAInt,
      JSON.stringify(orderedChannels) + ' expected [' + channelBInt + ',' + channelAInt + ']'
    );

    // Now the same gesture on the rule rows, which must also reorder and persist.
    await openAssociationDialog();
    await clearDriver();
    const ruleHandles2 = page.getByRole('button', { name: 'Drag to reorder', exact: true });
    await dragHandle(ruleHandles2.nth(0), ruleHandles2.nth(1));
    await page.waitForTimeout(600);
    await saveDialog();
    orderedChannels = await readOrderedAssociations();
    check(
      'rule row drag persisted the new priority order',
      orderedChannels[0] === channelAInt && orderedChannels[1] === channelBInt,
      JSON.stringify(orderedChannels) + ' expected [' + channelAInt + ',' + channelBInt + ']'
    );

    await page.keyboard.press('Escape').catch(() => {});

    // ---------------------------------------------------------------- fix 3
    await page.goto(BASE + '/project/requests', { waitUntil: 'networkidle' }).catch(() => {});
    await page.waitForTimeout(2500);
    await dismissOverlays();
    console.log('  requests url: ' + page.url() + ' toolbar=' + (await page.locator('table').count()));

    await clearDriver();
    const cleanupButton = page.getByTestId('requests-cleanup-open');
    const hasCleanupButton = await cleanupButton.isVisible().catch(() => false);
    check('request logs page exposes the clean-logs action', hasCleanupButton);

    if (hasCleanupButton) {
      await cleanupButton.click();
      const dialog = page.getByTestId('requests-cleanup-dialog');
      await dialog.waitFor({ state: 'visible', timeout: 15000 });
      check('cleanup dialog opens', true);

      const dialogText = await dialog.innerText();
      check('cleanup dialog lists retention and targets', dialogText.includes('Retention') && dialogText.includes('Request logs'), dialogText.slice(0, 120));

      await page.waitForTimeout(1200);
      const confirm = page.getByTestId('requests-cleanup-confirm');
      await confirm.click();
      await page.waitForTimeout(2500);
      const closed = !(await dialog.isVisible().catch(() => false));
      check('cleanup runs and closes the dialog', closed);
    }

    check('no console errors on the tested pages', consoleErrors.length === 0, consoleErrors.slice(0, 3).join(' | '));
  } finally {
    if (browser) await browser.close().catch(() => {});
    app.kill();
    await sleep(400);
    await rm(tmp, { recursive: true, force: true }).catch(() => {});
  }

  const failed = results.filter((r) => !r.ok);
  console.log('\n' + (results.length - failed.length) + '/' + results.length + ' checks passed');
  if (failed.length) {
    console.log('failed: ' + failed.map((f) => f.name).join(' | '));
    process.exitCode = 1;
  }
}

main().catch((err) => {
  console.error('UI checks crashed:', err);
  process.exitCode = 1;
});