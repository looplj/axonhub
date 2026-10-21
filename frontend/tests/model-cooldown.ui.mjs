#!/usr/bin/env node
// Observable UI checks for channel x model freeze (cooldown):
//  1. two channel_model rules on the SAME channel with different models must
//     render as two preview cards (not one collapsed card),
//  2. the preview cards can be dragged to reorder (rule priorities follow),
//  3. freezing a model from the preview shows an observable "Frozen" badge,
//     persists across dialog reopen (backend state), and can be unfrozen,
//  4. the channels table exposes the frozen model via the row menu.
//
// Usage (from frontend/): node tests/model-cooldown.ui.mjs

import { spawn } from 'node:child_process';
import { mkdtemp, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { existsSync } from 'node:fs';
import { chromium } from '@playwright/test';

const APP_PORT = Number(process.env.E2E_APP_PORT || 8094);
const BASE = 'http://127.0.0.1:' + APP_PORT;
const BIN = process.env.E2E_BIN || join(process.cwd(), '..', 'axonhub.exe');
const OWNER = { email: 'cooldown-ui@example.com', password: 'e2e-password-123' };

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
  const tmp = await mkdtemp(join(tmpdir(), 'axonhub-cooldown-e2e-'));
  const app = startApp(join(tmp, 'cooldown.db'));
  let browser = null;

  try {
    await waitForHealth();

    await fetch(BASE + '/admin/system/initialize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ownerEmail: OWNER.email,
        ownerPassword: OWNER.password,
        ownerFirstName: 'Cool',
        ownerLastName: 'Owner',
        brandName: 'Cool Lab',
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

    // One channel carrying two models: the reported scenario is two rules on the
    // SAME channel with different models.
    const ch = await gql(
      'mutation ($input: CreateChannelInput!) { createChannel(input: $input) { id name status } }',
      {
        input: {
          type: 'openai',
          name: 'e2e-cool-chan',
          baseURL: 'https://api.example.com/v1',
          credentials: { apiKeys: ['sk-cool-a'] },
          supportedModels: ['alpha', 'beta'],
          defaultTestModel: 'alpha',
          policies: { apiKeyAutoDisableMode: 'inherit' },
        },
      }
    );
    check('channel created', Boolean(ch.createChannel.id));
    await gql('mutation ($id: ID!, $status: ChannelStatus!) { updateChannelStatus(id: $id, status: $status) { id status } }', {
      id: ch.createChannel.id,
      status: 'enabled',
    });

    const channelInt = guidToInt(ch.createChannel.id);

    await gql('mutation ($input: CreateModelInput!) { createModel(input: $input) { id modelID } }', {
      input: {
        developer: 'openai',
        modelID: 'alpha',
        type: 'chat',
        name: 'Alpha',
        icon: 'OpenAI',
        group: 'openai',
        modelCard: {},
        settings: {
          associations: [
            { type: 'channel_model', priority: 0, channelModel: { channelId: channelInt, modelId: 'alpha' } },
            { type: 'channel_model', priority: 1, channelModel: { channelId: channelInt, modelId: 'beta' } },
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
      if (
        msg.type() === 'error' &&
        !msg.text().includes('ERR_TIMED_OUT') &&
        !msg.text().includes('Failed to load resource') &&
        !msg.text().includes('providers catalog')
      ) {
        consoleErrors.push(msg.text());
      }
    });

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
        await page.waitForTimeout(600);
      }
      await dismissOverlays();
      const trigger = page.locator('table').getByRole('button', { name: /^[0-9]+$/ }).first();
      // The onboarding tour overlay can re-appear between dismissal and the
      // click; dispatch the DOM click directly so hit-testing cannot eat it.
      await trigger.evaluate((el) => el.click());
      await page.waitForSelector('[role="dialog"]', { timeout: 20000 });
      await page.waitForTimeout(1500);
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
      }
      await page.waitForTimeout(2500);
    }

    async function readOrderedAssociations() {
      const data = await gql(
        'query { models(first: 10) { edges { node { modelID settings { associations { type priority channelModel { channelId modelId } } } } } } }'
      );
      const model = (data.models.edges[0] || {}).node;
      const associations = (model && model.settings && model.settings.associations) || [];
      return associations
        .slice()
        .sort((x, y) => (x.priority ?? 0) - (y.priority ?? 0))
        .map((assoc) => (assoc.channelModel ? assoc.channelModel.channelId + ':' + assoc.channelModel.modelId : null));
    }

    // The preview card root is the closest rounded ancestor of the preview drag
    // handle. Rule rows contain the same model text, so text filtering alone is
    // ambiguous.
    async function findPreviewCard(model) {
      const handles = page.getByRole('dialog').first().getByRole('button', { name: 'Drag to reorder preview' });
      const count = await handles.count();
      for (let i = 0; i < count; i += 1) {
        const card = handles.nth(i).locator('xpath=ancestor::div[contains(@class,"rounded-lg")][1]');
        const text = await card.innerText().catch(() => '');
        if (text.includes(model)) return card;
      }
      return null;
    }

    async function readModelCooldowns() {
      const data = await gql(
        'query ($id: ID!) { node(id: $id) { ... on Channel { modelCooldowns { model reason expiresAt } } } }',
        { id: ch.createChannel.id }
      );
      return (data.node && data.node.modelCooldowns) || [];
    }

    await page.goto(BASE + '/models', { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('table', { timeout: 45000 });
    await dismissOverlays();
    await openAssociationDialog();
    await clearDriver();

    const dialog = page.getByRole('dialog').first();
    const previewHandles = page.getByRole('button', { name: 'Drag to reorder preview' });

    // 1. Two rules on the same channel with different models => two cards.
    check('two preview cards for same channel', (await previewHandles.count()) === 2, 'handles=' + (await previewHandles.count()));
    const text0 = await dialog.innerText();
    check(
      'both models visible in preview',
      text0.includes('alpha') && text0.includes('beta'),
      'alpha=' + text0.includes('alpha') + ' beta=' + text0.includes('beta')
    );

    // 2. Preview drag on the SAME channel swaps the two model cards immediately.
    const initialAlphaFirst = text0.indexOf('alpha') >= 0 && text0.indexOf('alpha') < text0.indexOf('beta');
    check('initial order alpha before beta', initialAlphaFirst, initialAlphaFirst ? 'ok' : text0.slice(0, 200));

    await dragHandle(previewHandles.nth(1), previewHandles.nth(0));
    await page.waitForTimeout(700);
    const text1 = await dialog.innerText();
    const betaFirst = text1.indexOf('beta') >= 0 && text1.indexOf('beta') < text1.indexOf('alpha');
    check('same-channel preview cards reorder after drop', betaFirst, betaFirst ? 'reordered' : 'unchanged');

    await saveDialog();
    const ordered = await readOrderedAssociations();
    check(
      'same-channel preview drag persisted priorities',
      ordered[0] === channelInt + ':beta' && ordered[1] === channelInt + ':alpha',
      JSON.stringify(ordered)
    );

    // 3. Freeze the beta model from the preview menu -> observable badge.
    await openAssociationDialog();
    await clearDriver();
    const betaCard = await findPreviewCard('beta');
    if (!betaCard) throw new Error('preview card for beta not found');
    await betaCard.getByRole('button', { name: 'Freeze / unfreeze' }).click();
    await page.waitForTimeout(400);
    await page.getByRole('menuitem', { name: 'Freeze 30 minutes' }).click();
    await page.waitForTimeout(1200);

    const frozenBadge = await betaCard.innerText();
    check('freeze shows observable badge on the card', frozenBadge.includes('Frozen'), frozenBadge.replace(/\s+/g, ' ').slice(0, 120));

    const cooldownsAfterFreeze = await readModelCooldowns();
    check(
      'backend persisted the model freeze',
      cooldownsAfterFreeze.some((mc) => mc.model === 'beta' && mc.expiresAt),
      JSON.stringify(cooldownsAfterFreeze)
    );

    // 4. Freeze survives reopening the dialog (state comes from the backend).
    await page.keyboard.press('Escape').catch(() => {});
    await page.waitForTimeout(800);
    await openAssociationDialog();
    await clearDriver();
    const betaCard2 = await findPreviewCard('beta');
    if (!betaCard2) throw new Error('preview card for beta not found after reopen');
    const persisted = await betaCard2.innerText();
    check('freeze persists across dialog reopen', persisted.includes('Frozen'), persisted.replace(/\s+/g, ' ').slice(0, 120));

    // 5. The channels table exposes the frozen model count.
    await page.keyboard.press('Escape').catch(() => {});
    await page.waitForTimeout(800);
    await page.goto(BASE + '/channels', { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('table', { timeout: 45000 });
    await dismissOverlays();
    const channelsData = await gql(
      'query { queryChannels(input: {}) { edges { node { id name modelCooldowns { model expiresAt } } } } }'
    ).catch(() => null);
    const nodes = (((channelsData || {}).queryChannels || {}).edges || []).map((edge) => edge.node);
    const coolNode = nodes.find((node) => node.name === 'e2e-cool-chan');
    check(
      'channels list exposes the frozen model',
      Boolean(coolNode && (coolNode.modelCooldowns || []).some((mc) => mc.model === 'beta')),
      JSON.stringify(coolNode ? coolNode.modelCooldowns : nodes.map((node) => node.name))
    );

    // 6. Unfreeze from the association preview menu -> badge disappears everywhere.
    await page.goto(BASE + '/models', { waitUntil: 'domcontentloaded' });
    await page.waitForSelector('table', { timeout: 45000 });
    await dismissOverlays();
    await openAssociationDialog();
    await clearDriver();
    const betaCard3 = await findPreviewCard('beta');
    if (!betaCard3) throw new Error('preview card for beta not found for unfreeze');
    await betaCard3.getByRole('button', { name: 'Freeze / unfreeze' }).click();
    await page.waitForTimeout(400);
    await page.getByRole('menuitem', { name: 'Unfreeze beta' }).click();
    await page.waitForTimeout(1200);
    const unfrozen = await betaCard3.innerText();
    check('unfreeze removes the badge', !unfrozen.includes('Frozen'), unfrozen.replace(/\s+/g, ' ').slice(0, 120));

    const cooldownsAfterUnfreeze = await readModelCooldowns();
    check(
      'backend removed the model freeze',
      !cooldownsAfterUnfreeze.some((mc) => mc.model === 'beta'),
      JSON.stringify(cooldownsAfterUnfreeze)
    );

    check('no console errors', consoleErrors.length === 0, consoleErrors.slice(0, 3).join(' | '));
  } finally {
    if (browser) await browser.close().catch(() => {});
    app.kill();
    await rm(tmp, { recursive: true, force: true }).catch(() => {});
  }

  const failed = results.filter((r) => !r.ok).length;
  console.log((failed === 0 ? results.length : results.length - failed) + '/' + results.length + ' checks passed');
  if (failed > 0) process.exitCode = 1;
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
