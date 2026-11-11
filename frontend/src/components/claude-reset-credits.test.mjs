import React from 'react';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import ts from 'typescript';

const srcRoot = join(import.meta.dirname, '..');
const read = (relativePath) => readFileSync(join(srcRoot, relativePath), 'utf8');

// Compile the production component without its imports and render it against
// the real locale tables, so names, reasons and layout text are what users see.
const componentSource = read('components/claude-reset-credits.tsx').replace(/^import[\s\S]*?;\n/gm, '');
const locales = { en: JSON.parse(read('locales/en/system.json')), zh: JSON.parse(read('locales/zh-CN/system.json')) };

const moduleSource = [
  'const { useState } = React;',
  'let table = {};',
  'export function setLocale(next) { table = next; }',
  'function t(key, params) {',
  '  let out = table[key] ?? key;',
  '  for (const [name, value] of Object.entries(params || {})) out = out.split(`{{${name}}}`).join(String(value));',
  '  return out;',
  '}',
  'function useTranslation() { return { t, i18n: { language: "en" } }; }',
  'function useQueryClient() { return { invalidateQueries: async () => {} }; }',
  'const toast = { success() {}, error() {} };',
  'async function resetChannelQuotaNow() {}',
  'function Loader2() { return null; }',
  'function RotateCcw() { return null; }',
  'function Badge({ children }) { return <span data-badge="">{children}</span>; }',
  'function Button({ children, disabled }) { return <button disabled={disabled}>{children}</button>; }',
  componentSource,
].join('\n');
const { outputText } = ts.transpileModule(moduleSource, {
  compilerOptions: { jsx: ts.JsxEmit.React, module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
globalThis.React = React;
const mod = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
const { ClaudeResetCredits, setLocale, isFullClaudeReset, getClaudeGrantBlockReason, getSharedClaudeBlockReason } = mod;

const future = (days) => new Date(Date.now() + days * 86400000).toISOString();
const grant = (id, overrides = {}) => ({ id, status: 'held', remaining: 1, redeemable: true, expiresAt: future(5), ...overrides });
const fullGrant = (overrides) => grant('full', { clears: ['five_hour', 'seven_day', 'seven_day_overage_included'], title: 'One full limit reset', ...overrides });
const partialGrant = (overrides) =>
  grant('partial', { clears: ['seven_day_overage_included'], title: 'New model launch: one reset, launch week only', ...overrides });

function render(resets, locale = 'en') {
  setLocale(locales[locale]);
  return renderToStaticMarkup(React.createElement(ClaudeResetCredits, { channelID: 'ch', resets }))
    .replace(/&#x27;/g, "'")
    .replace(/&amp;/g, '&');
}
const count = (html, text) => html.split(text).length - 1;

test('reset names come from the cleared windows, not upstream labels or model names', () => {
  assert.equal(isFullClaudeReset(['five_hour', 'seven_day']), true);
  assert.equal(isFullClaudeReset(['five_hour', 'seven_day_overage_included']), false);
  assert.equal(isFullClaudeReset(undefined), false);

  const html = render({ supported: true, eligible: true, resets: [partialGrant(), fullGrant({ recommended: true })] });
  assert.match(html, /Full quota reset/);
  assert.match(html, /Partial quota reset/);
  // Without a model name from the backend the scoped window gets a neutral label.
  assert.match(html, /Model weekly limit/);
  assert.doesNotMatch(html, /Fable/);
  // Upstream labels stay available on hover only.
  assert.match(html, /title="New model launch: one reset, launch week only"/);
  assert.equal(count(html, '>New model launch'), 0);
  // The recommended grant is listed first.
  assert.ok(html.indexOf('Full quota reset') < html.indexOf('Partial quota reset'));

  const zh = render({ supported: true, eligible: true, resets: [partialGrant(), fullGrant()] }, 'zh');
  assert.match(zh, /完整额度重置/);
  assert.match(zh, /部分额度重置/);
  assert.match(zh, /模型周额度/);
});

test('model-scoped weekly windows are named from backend data', () => {
  const resets = { supported: true, eligible: true, windowModels: { seven_day_overage_included: 'Fable' }, resets: [partialGrant(), fullGrant()] };
  assert.equal(count(render(resets), 'Fable weekly'), 2);
  assert.equal(count(render(resets, 'zh'), 'Fable 周额度'), 2);
});

test('grants without cleared windows get a neutral name instead of partial', () => {
  const html = render({ supported: true, eligible: true, resets: [grant('legacy', { clears: undefined }), grant('empty', { clears: [] })] });
  assert.equal(count(html, '>Quota reset'), 2);
  assert.doesNotMatch(html, /Partial quota reset/);
});

test('upstream reasons are shown with their code', () => {
  assert.equal(getClaudeGrantBlockReason(grant('a', { redeemable: false, unavailableReason: 'upstream', upstreamReason: 'paused_promo' }), false), 'upstream:paused_promo');
  assert.equal(getClaudeGrantBlockReason(grant('a', { redeemable: false, unavailableReason: 'requires_limit', upstreamReason: 'not_at_limit' }), false), 'requires_limit');

  const shared = render({
    supported: true,
    eligible: true,
    resets: [fullGrant({ redeemable: false, unavailableReason: 'upstream', upstreamReason: 'paused_promo' }), partialGrant({ redeemable: false, unavailableReason: 'upstream', upstreamReason: 'paused_promo' })],
  }, 'zh');
  assert.equal(count(shared, '暂时无法使用（paused_promo）。'), 1);

  const mixed = render({
    supported: true,
    eligible: true,
    resets: [fullGrant({ redeemable: false, unavailableReason: 'upstream', upstreamReason: 'a_code' }), partialGrant({ redeemable: false, unavailableReason: 'upstream', upstreamReason: 'b_code' })],
  });
  assert.equal(count(mixed, 'Not available right now (a_code).'), 1);
  assert.equal(count(mixed, 'Not available right now (b_code).'), 1);
});

test('unknown windows fall back to a readable identifier', () => {
  const html = render({ supported: true, eligible: true, resets: [grant('x', { clears: ['thirty_day_extra'] })] });
  assert.match(html, /thirty day extra/);
  assert.match(html, /Partial quota reset/);
});

test('per-grant count is shown only when a grant holds more than one reset', () => {
  const single = render({ supported: true, eligible: true, resets: [fullGrant()] });
  assert.doesNotMatch(single, /×1/);
  assert.match(single, /1 remaining/, 'the header still shows the total');
  const multiple = render({ supported: true, eligible: true, resets: [fullGrant({ remaining: 3 })] });
  assert.match(multiple, /×3/);
  assert.match(multiple, /3 remaining/);
});

test('a reason shared by every grant is shown once above the cards', () => {
  const resets = {
    supported: true,
    eligible: true,
    resets: [
      fullGrant({ redeemable: false, unavailableReason: 'requires_limit' }),
      partialGrant({ redeemable: false, unavailableReason: 'requires_limit' }),
    ],
  };
  const en = render(resets);
  assert.equal(count(en, 'Resets can be used once a usage limit is reached.'), 1);
  assert.equal(count(en, '<button disabled="">'), 2);
  const zh = render(resets, 'zh');
  assert.equal(count(zh, '达到任一额度上限后才能使用重置。'), 1);
});

test('mixed availability explains only the blocked grant', () => {
  const html = render({
    supported: true,
    eligible: true,
    resets: [fullGrant(), partialGrant({ redeemable: false, unavailableReason: 'blocked' })],
  });
  assert.equal(count(html, 'Another limit is blocking resets right now.'), 1);
  assert.equal(count(html, '<button disabled="">'), 1);
  assert.equal(count(html, '<button>'), 1);
});

test('an active cooldown is reported once with its end time', () => {
  const html = render({
    supported: true,
    eligible: true,
    cooldownUntil: future(1),
    resets: [fullGrant(), partialGrant()],
  });
  assert.equal(count(html, 'Resets are cooling down until '), 1);
  assert.equal(count(html, '<button disabled="">'), 2);
});

test('unknown backend reasons and ineligible accounts do not repeat per card', () => {
  assert.equal(getClaudeGrantBlockReason(grant('a', { redeemable: false, unavailableReason: 'something_new' }), false), 'unavailable');
  assert.equal(getClaudeGrantBlockReason(grant('a', { redeemable: true, remaining: 0 }), false), 'unavailable');
  assert.equal(getClaudeGrantBlockReason(grant('a'), false), null);
  assert.equal(getSharedClaudeBlockReason([]), null);
  assert.equal(getSharedClaudeBlockReason(['blocked', null]), null);
  assert.equal(getSharedClaudeBlockReason(['blocked', 'requires_limit']), null);

  const html = render({ supported: true, eligible: false, ineligibleReason: 'tier', resets: [fullGrant({ redeemable: false, unavailableReason: 'ineligible' })] });
  assert.equal(count(html, 'This account is not eligible for manual resets.'), 1);
  assert.doesNotMatch(html, /Not available right now/);
  assert.match(html, /N\/A/);
});

test('reset text keys exist in both locales', () => {
  const keys = [...new Set([...componentSource.matchAll(/'(quota\.(?:claude|window|label)\.[\w.]+)'/g)].map((m) => m[1]))];
  assert.ok(keys.length > 10);
  for (const key of keys) {
    assert.equal(typeof locales.en[key], 'string', `en missing ${key}`);
    assert.equal(typeof locales.zh[key], 'string', `zh-CN missing ${key}`);
  }
});
