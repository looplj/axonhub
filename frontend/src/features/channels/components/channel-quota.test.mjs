import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Run the table's pure quota helpers without loading React or its providers.
const source = readFileSync(new URL('./channels-columns.tsx', import.meta.url), 'utf8');
const ast = ts.createSourceFile('channels-columns.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const helperNames = ['getQuotaLimits', 'codexWindowDuration', 'quotaWindowLabel'];
const helpers = ast.statements
  .filter((node) => ts.isFunctionDeclaration(node) && helperNames.includes(node.name?.text))
  .map((node) => `export ${node.getText(ast)}`)
  .join('\n');
const { outputText } = ts.transpileModule(helpers, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
const { getQuotaLimits, quotaWindowLabel } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
const t = (key) => key;

function codex(rateLimit, limits = [{ window: 'primary', usageRatio: 0.52 }]) {
  return {
    type: 'codex',
    providerQuotaStatus: {
      status: 'available',
      quotaData: { _limits: limits, rate_limit: rateLimit },
    },
  };
}

test('weekly-only Codex primary quota is shown once as 7d, with its usage preserved', () => {
  const channel = codex({ primary_window: { used_percent: 52, limit_window_seconds: 604800 }, secondary_window: null });
  const limits = getQuotaLimits(channel);
  assert.deepEqual(limits, [{ window: '7d', usageRatio: 0.52, status: 'available' }]);
  assert.equal(quotaWindowLabel(limits[0].window, t), '7d');
  assert.equal(Math.round(100 - limits[0].usageRatio * 100), 48);
});

test('Codex dual windows and other durations use reported lengths', () => {
  for (const [seconds, label] of [[18000, '5h'], [86400, '1d'], [7200, '2h'], [5400, '90m'], [45, '45s']]) {
    const limits = getQuotaLimits(codex({
      primary_window: { used_percent: 20, limit_window_seconds: seconds },
      secondary_window: { used_percent: 60, limit_window_seconds: 604800 },
    }));
    assert.deepEqual(limits.map((limit) => limit.window), [label, '7d']);
    assert.deepEqual(limits.map((limit) => limit.usageRatio), [0.2, 0.6]);
  }
});

test('absent windows do not inherit a synthetic normalized primary quota', () => {
  assert.deepEqual(getQuotaLimits(codex({ primary_window: null, secondary_window: null })), []);
  const limits = getQuotaLimits(codex({ secondary_window: { used_percent: 0, limit_window_seconds: 604800 } }));
  assert.deepEqual(limits, [{ window: '7d', usageRatio: 0, status: 'available' }]);
});

test('unknown or invalid durations use localized role labels rather than assumed periods', () => {
  for (const seconds of [undefined, null, 0, -1, NaN, Infinity, '604800']) {
    const limits = getQuotaLimits(codex({
      primary_window: { used_percent: 52, limit_window_seconds: seconds },
      secondary_window: { used_percent: 0, limit_window_seconds: seconds },
    }));
    assert.equal(quotaWindowLabel(limits[0].window, t), 'quota.label.primary_window');
    assert.equal(quotaWindowLabel(limits[1].window, t), 'quota.label.secondary_window');
  }
});

test('legacy normalized-only Codex data keeps usage without guessing a duration', () => {
  const limits = getQuotaLimits(codex(undefined));
  assert.equal(limits[0].usageRatio, 0.52);
  assert.equal(quotaWindowLabel(limits[0].window, t), 'quota.label.primary_window');
});

test('other provider window labels are preserved', () => {
  const channel = { type: 'claudecode', providerQuotaStatus: { status: 'available', quotaData: {
    windows: { '5h': { utilization: 0.2 }, '7d': { utilization: 0.4 } },
  } } };
  assert.deepEqual(getQuotaLimits(channel).map((limit) => quotaWindowLabel(limit.window, t)), ['5h', '7d']);
  assert.equal(quotaWindowLabel('weekly', t), '7d');
  assert.equal(quotaWindowLabel('monthly', t), '30d');
});
