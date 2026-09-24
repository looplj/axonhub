import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(join(import.meta.dirname, 'quota-routing-status.ts'), 'utf8');
const transpiled = ts.transpileModule(
  source
    .replace(/^import type[^\n]*\n/gm, '')
    .replace(/^import[^\n]*\n/gm, 'const parseQuotaLimits = (quotaData) => Array.isArray(quotaData?._limits) ? quotaData._limits : [];\n'),
  {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { getChannelQuotaRoutingIndicator } = await import(`data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`);

const status = (value, quotaData = {}) => ({ status: value, quotaData });

test('exhausted status is shown when the backend removes exhausted channels', () => {
  assert.equal(
    getChannelQuotaRoutingIndicator({ providerQuotaStatus: status('exhausted'), settings: { quotaRoutingMode: 'REMOVE_ON_EXHAUSTED' } }),
    'exhausted'
  );
});

test('ignore quota mode does not show a quota routing indicator', () => {
  assert.equal(
    getChannelQuotaRoutingIndicator({ providerQuotaStatus: status('exhausted'), settings: { quotaRoutingMode: 'IGNORE_QUOTA' } }),
    undefined
  );
});

test('a pressured quota window is shown as backpressure for an effective backpressure mode', () => {
  const now = Date.now();
  assert.equal(
    getChannelQuotaRoutingIndicator(
      {
        providerQuotaStatus: status('available', {
          _limits: [{ type: 'token', window: '5h', status: 'available', usageRatio: 0.8, periodStart: new Date(now - 60 * 60 * 1000).toISOString(), nextResetAt: new Date(now + 60 * 60 * 1000).toISOString() }],
        }),
        settings: { quotaRoutingMode: 'INHERIT' },
      },
      'BACKPRESSURE'
    ),
    'backpressure'
  );
});

test('an available quota window without pressure does not show a quota routing indicator', () => {
  const now = Date.now();
  assert.equal(
    getChannelQuotaRoutingIndicator({
      providerQuotaStatus: status('available', {
        _limits: [{ type: 'token', window: '5h', status: 'available', usageRatio: 0.2, periodStart: new Date(now - 60 * 60 * 1000).toISOString(), nextResetAt: new Date(now + 60 * 60 * 1000).toISOString() }],
      }),
      settings: { quotaRoutingMode: 'BACKPRESSURE' },
    }),
    undefined
  );
});

test('a pressured quota window does not show backpressure when the channel removes only on exhaustion', () => {
  const now = Date.now();
  assert.equal(
    getChannelQuotaRoutingIndicator({
      providerQuotaStatus: status('available', {
        _limits: [{ type: 'token', window: '5h', status: 'available', usageRatio: 0.8, periodStart: new Date(now - 60 * 60 * 1000).toISOString(), nextResetAt: new Date(now + 60 * 60 * 1000).toISOString() }],
      }),
      settings: { quotaRoutingMode: 'REMOVE_ON_EXHAUSTED' },
    }),
    undefined
  );
});
