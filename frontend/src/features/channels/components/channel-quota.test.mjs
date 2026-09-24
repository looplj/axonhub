import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Run the table's pure quota helpers without loading React or its providers.
const source = readFileSync(new URL('./channels-columns.tsx', import.meta.url), 'utf8');
const ast = ts.createSourceFile('channels-columns.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const helperNames = ['getQuotaLimits', 'quotaWindowLabel'];
const helpers = ast.statements
  .filter((node) => ts.isFunctionDeclaration(node) && helperNames.includes(node.name?.text))
  .map((node) => `export ${node.getText(ast)}`)
  .join('\n');
const { outputText } = ts.transpileModule(helpers, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
const parserStub = `
const QUOTA_WINDOW_LABEL_KEYS = {
  '5h': '5h', '7d': '7d', '30d': '30d', daily: 'daily', weekly: 'weekly', monthly: 'monthly', cycle: 'cycle', overage: 'overage'
};
function parseQuotaLimits(quotaData) {
  if (!Array.isArray(quotaData?._limits)) return [];
  return quotaData._limits.filter((limit) =>
    limit && typeof limit.type === 'string' && typeof limit.status === 'string' && typeof limit.ready === 'boolean' &&
    typeof limit.window === 'string' && typeof limit.usageRatio === 'number' && Number.isFinite(limit.usageRatio) &&
    limit.usageRatio >= 0 && limit.usageRatio <= 1
  );
}
`;
const { getQuotaLimits, quotaWindowLabel } = await import(
  `data:text/javascript;base64,${Buffer.from(`${parserStub}\n${outputText}`).toString('base64')}`
);
const t = (key) => key;

/**
 * Create a persisted Codex channel fixture with independently configurable raw and normalized data.
 * @param {object | undefined} rateLimit Raw provider windows, or undefined for legacy records.
 * @param {object[]} limits Normalized quota entries, defaulting to a primary window with 52% used.
 * @returns {object} Channel fixture consumed by the table's quota helpers.
 */
function codex(limits = [{ type: 'token', status: 'available', ready: true, window: '5h', usageRatio: 0.52 }]) {
  return {
    type: 'codex',
    providerQuotaStatus: {
      status: 'available',
      quotaData: { _limits: limits },
    },
  };
}

test('weekly-only Codex quota is shown once as 7d, with its usage preserved', () => {
  const channel = codex([{ type: 'token', status: 'available', ready: true, window: '7d', usageRatio: 0.52 }]);
  const limits = getQuotaLimits(channel);
  assert.deepEqual(limits.map(({ window, usageRatio, status }) => ({ window, usageRatio, status })), [{ window: '7d', usageRatio: 0.52, status: 'available' }]);
  assert.equal(quotaWindowLabel(limits[0].window, t), '7d');
  assert.equal(Math.round(100 - limits[0].usageRatio * 100), 48);
});

test('Codex dual windows and other durations use reported lengths', () => {
  for (const [label, usageRatio] of [['5h', 0.2], ['1d', 0.2], ['2h', 0.2], ['90m', 0.2], ['45s', 0.2]]) {
    const limits = getQuotaLimits(codex([
      { type: 'token', status: 'available', ready: true, window: label, usageRatio },
      { type: 'token', status: 'available', ready: true, window: '7d', usageRatio: 0.6 },
    ]));
    assert.deepEqual(limits.map((limit) => limit.window), [label, '7d']);
    assert.deepEqual(limits.map((limit) => limit.usageRatio), [usageRatio, 0.6]);
  }
});

test('only persisted normalized Codex windows are displayed', () => {
  assert.deepEqual(getQuotaLimits(codex([])), []);
  const limits = getQuotaLimits(codex([{ type: 'token', status: 'available', ready: true, window: '7d', usageRatio: 0 }]));
  assert.deepEqual(limits.map(({ window, usageRatio, status }) => ({ window, usageRatio, status })), [{ window: '7d', usageRatio: 0, status: 'available' }]);
});

test('role identifiers use a neutral token label rather than assumed periods', () => {
  assert.equal(quotaWindowLabel('primary', t), 'quota.label.token_usage');
  assert.equal(quotaWindowLabel('secondary', t), 'quota.label.token_usage');
});

test('malformed normalized Codex data is ignored', () => {
  assert.deepEqual(getQuotaLimits(codex([{ window: 'primary', usageRatio: 0.52 }])), []);
});

test('other provider window labels are preserved', () => {
  const channel = { type: 'claudecode', providerQuotaStatus: { status: 'available', quotaData: {
    _limits: [
      { type: 'token', status: 'available', ready: true, window: '5h', usageRatio: 0.2 },
      { type: 'token', status: 'available', ready: true, window: '7d', usageRatio: 0.4 },
    ],
  } } };
  assert.deepEqual(getQuotaLimits(channel).map((limit) => quotaWindowLabel(limit.window, t)), ['5h', '7d']);
  assert.equal(quotaWindowLabel('weekly', t), 'weekly');
  assert.equal(quotaWindowLabel('monthly', t), 'monthly');
});
