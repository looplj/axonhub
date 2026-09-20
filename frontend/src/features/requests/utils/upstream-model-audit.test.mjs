import i18next from 'i18next';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { getRequestModelAuditTooltip, getUpstreamModelAudit } from './upstream-model-audit.ts';

const resources = Object.fromEntries(
  ['en', 'zh-CN'].map((locale) => [
    locale,
    {
      translation: JSON.parse(readFileSync(new URL('../../../locales/' + locale + '/requests.json', import.meta.url), 'utf8')),
    },
  ])
);
const i18n = i18next.createInstance();
await i18n.init({ lng: 'zh-CN', fallbackLng: 'en', keySeparator: false, resources });

test('request 73927 shows only the successful model in both list languages and retains each execution audit', () => {
  const retry = { status: 'failed', outboundModelID: 'glm-5.3:free', upstreamModelID: 'glm-5.3:free' };
  const success = { status: 'completed', outboundModelID: 'glm-5.3', upstreamModelID: 'glm-5.3' };
  for (const executions of [
    [retry, retry, retry, success],
    [success, retry, retry, retry],
  ]) {
    const audit = getUpstreamModelAudit(executions);
    assert.equal(audit.status, 'matched');
    assert.deepEqual(audit.matchedUpstreamIds, ['glm-5.3']);
    for (const locale of ['en', 'zh-CN']) {
      const tooltip = getRequestModelAuditTooltip(audit, 'completed', i18n.getFixedT(locale));
      assert.ok(tooltip.includes('glm-5.3'));
      assert.ok(!tooltip.includes('glm-5.3:free'));
      assert.ok(!tooltip.includes('{{'));
    }
    for (const execution of executions) {
      const detail = getUpstreamModelAudit([execution]);
      assert.equal(detail.status, 'matched');
      assert.deepEqual(detail.upstreamModelIds, [execution.upstreamModelID]);
    }
  }
});

test('unknown retries retain their count alongside the successful model in both list languages', () => {
  const audit = getUpstreamModelAudit([
    { status: 'failed', outboundModelID: 'glm-5.3:free' },
    { status: 'completed', outboundModelID: 'glm-5.3', upstreamModelID: 'glm-5.3' },
  ]);
  for (const locale of ['en', 'zh-CN']) {
    const tooltip = getRequestModelAuditTooltip(audit, 'completed', i18n.getFixedT(locale));
    assert.ok(tooltip.includes('glm-5.3'));
    assert.ok(tooltip.includes('1'));
    assert.ok(!tooltip.includes('glm-5.3:free'));
    assert.ok(!tooltip.includes('{{'));
  }
});

test('list warnings still display anomalies from failed retries after a successful match', () => {
  for (const upstreamModelIds of [['different'], ['sent', 'different']]) {
    const audit = getUpstreamModelAudit([
      { status: 'failed', outboundModelID: 'sent', upstreamModelIds },
      { status: 'completed', outboundModelID: 'glm-5.3', upstreamModelID: 'glm-5.3' },
    ]);
    for (const locale of ['en', 'zh-CN']) {
      assert.ok(getRequestModelAuditTooltip(audit, 'completed', i18n.getFixedT(locale)).includes('different'));
    }
  }
});

test('missing successful evidence never falls back to a failed retry model', () => {
  const retry = { status: 'failed', outboundModelID: 'glm-5.3:free', upstreamModelID: 'glm-5.3:free' };
  for (const executions of [[retry], [retry, { status: 'completed', outboundModelID: 'glm-5.3' }]]) {
    const audit = getUpstreamModelAudit(executions);
    assert.deepEqual(audit.matchedUpstreamIds, []);
    assert.ok(!getRequestModelAuditTooltip(audit, 'completed', i18n.t).includes('glm-5.3:free'));
  }
});

test('request lifecycle takes precedence over model match tooltips', () => {
  const audit = getUpstreamModelAudit([{ status: 'completed', outboundModelID: 'sent', upstreamModelID: 'sent' }]);
  for (const status of ['pending', 'processing', 'failed', 'canceled']) {
    const key = status === 'pending' || status === 'processing' ? 'upstreamModelRequestProcessing' : 'upstreamModelRequestFailed';
    assert.equal(getRequestModelAuditTooltip(audit, status, i18n.t), i18n.t('requests.tooltips.' + key));
  }
});

test('successful matching retries stay matched without discarding failed or canceled unknowns', () => {
  for (const status of ['failed', 'canceled']) {
    const executions = [
      ...Array.from({ length: 12 }, () => ({ status, outboundModelID: 'sent' })),
      { status: 'completed', outboundModelID: 'sent', upstreamModelID: 'sent' },
    ];
    for (const ordered of [executions, executions.toReversed()]) {
      const audit = getUpstreamModelAudit(ordered);
      assert.equal(audit.status, 'matched');
      assert.equal(audit.unknownCount, 12);
      assert.equal(audit.comparedCount, 1);
    }
  }
});

test('successful, active and legacy unknown executions still prevent a match', () => {
  for (const status of ['completed', 'pending', 'processing', undefined]) {
    const audit = getUpstreamModelAudit([
      { status: 'completed', outboundModelID: 'sent', upstreamModelID: 'sent' },
      { status, outboundModelID: 'sent' },
    ]);
    assert.equal(audit.status, 'unknown');
  }
});

test('a failed comparison cannot establish a match when final success is unknown or absent', () => {
  for (const status of ['completed', 'failed']) {
    const audit = getUpstreamModelAudit([
      { status: 'failed', outboundModelID: 'sent', upstreamModelID: 'sent' },
      { status, outboundModelID: 'sent' },
    ]);
    assert.equal(audit.status, 'unknown');
  }
});

test('successful retries do not hide earlier mismatches or conflicts', () => {
  for (const [upstreamModelIds, expected] of [
    [['different'], 'mismatched'],
    [['sent', 'different'], 'conflicting'],
  ]) {
    const audit = getUpstreamModelAudit([
      { status: 'failed', outboundModelID: 'sent', upstreamModelIds },
      { status: 'failed' },
      { status: 'completed', outboundModelID: 'sent', upstreamModelID: 'sent' },
    ]);
    assert.equal(audit.status, expected);
    assert.deepEqual(audit.mismatchedModelIds, ['different']);
    assert.equal(audit.unknownCount, 1);
  }
});

test('matches each execution against its own outbound model', () => {
  const audit = getUpstreamModelAudit([
    { outboundModelID: 'model-a', upstreamModelID: 'model-a' },
    { outboundModelID: 'model-b', upstreamModelID: 'model-b' },
  ]);
  assert.equal(audit.status, 'matched');
  assert.equal(audit.comparedCount, 2);
  assert.equal(audit.unknownCount, 0);
});

test('does not match a retry against a previous execution model', () => {
  const audit = getUpstreamModelAudit([
    { outboundModelID: 'model-a', upstreamModelID: null },
    { outboundModelID: 'model-b', upstreamModelID: 'model-a' },
  ]);
  assert.equal(audit.status, 'mismatched');
  assert.deepEqual(audit.mismatchedModelIds, ['model-a']);
  assert.equal(audit.unknownCount, 1);
});

test('detects swapped models even when both appear in the outbound model set', () => {
  const audit = getUpstreamModelAudit([
    { outboundModelID: 'model-a', upstreamModelID: 'model-b' },
    { outboundModelID: 'model-b', upstreamModelID: 'model-a' },
  ]);
  assert.equal(audit.status, 'mismatched');
  assert.deepEqual(audit.mismatchedModelIds, ['model-b', 'model-a']);
});

test('does not mark partially unknown executions as matched', () => {
  const audit = getUpstreamModelAudit([{ outboundModelID: 'model-a', upstreamModelID: 'model-a' }, { outboundModelID: 'model-b' }]);
  assert.equal(audit.status, 'unknown');
  assert.equal(audit.comparedCount, 1);
  assert.equal(audit.unknownCount, 1);
});

test('keeps empty, historical, and missing-model executions unknown', () => {
  assert.equal(getUpstreamModelAudit([]).status, 'unknown');
  for (const execution of [
    { outboundModelID: 'model-a' },
    { outboundModelID: 'model-a', upstreamModelID: null },
    { outboundModelID: 'model-a', upstreamModelID: '' },
    { outboundModelID: 'model-a', upstreamModelID: '   ' },
    { outboundModelID: '', upstreamModelID: 'model-a' },
  ]) {
    const audit = getUpstreamModelAudit([execution]);
    assert.equal(audit.status, 'unknown');
    assert.equal(audit.unknownCount, 1);
    assert.equal(audit.comparedCount, 0);
  }
});

test('preserves exact reported names instead of normalizing versions or case', () => {
  const audit = getUpstreamModelAudit([
    { outboundModelID: 'model-a', upstreamModelID: 'Model-A' },
    { outboundModelID: 'model-a', upstreamModelID: 'model-a-2026-09-19' },
  ]);
  assert.equal(audit.status, 'mismatched');
  assert.deepEqual(audit.upstreamModelIds, ['Model-A', 'model-a-2026-09-19']);
});

test('deduplicates reported names only after retaining every execution mismatch', () => {
  const audit = getUpstreamModelAudit([
    { outboundModelID: 'model-a', upstreamModelID: 'model-a' },
    { outboundModelID: 'model-b', upstreamModelID: 'model-a' },
  ]);
  assert.equal(audit.status, 'mismatched');
  assert.deepEqual(audit.upstreamModelIds, ['model-a']);
  assert.deepEqual(audit.mismatchedModelIds, ['model-a']);
});

test('uses the final sent model after channel overrides, preserving the routing name', () => {
  const execution = { modelID: 'routed-a', outboundModelID: 'sent-b' };
  assert.equal(getUpstreamModelAudit([{ ...execution, upstreamModelID: 'sent-b' }]).status, 'matched');
  assert.equal(getUpstreamModelAudit([{ ...execution, upstreamModelID: 'routed-a' }]).status, 'mismatched');
  const historical = getUpstreamModelAudit([{ modelID: 'routed-a', upstreamModelID: 'routed-a' }]);
  assert.equal(historical.status, 'unknown');
  assert.deepEqual(historical.upstreamModelIds, ['routed-a']);
});

test('retains conflicting names from one stream even when its first model matches', () => {
  const audit = getUpstreamModelAudit([
    { outboundModelID: 'model-a', upstreamModelID: 'model-a', upstreamModelIds: ['model-a', 'model-b'] },
  ]);
  assert.equal(audit.status, 'conflicting');
  assert.equal(audit.conflictCount, 1);
  assert.deepEqual(audit.conflictingModelIds, ['model-a', 'model-b']);
  assert.deepEqual(audit.mismatchedModelIds, ['model-b']);
});

test('reports a stream conflict even when the final sent model is unknown', () => {
  const audit = getUpstreamModelAudit([{ upstreamModelIds: ['model-a', 'model-b'] }]);
  assert.equal(audit.status, 'conflicting');
  assert.equal(audit.unknownCount, 1);
  assert.equal(audit.comparedCount, 0);
  assert.deepEqual(audit.conflictingModelIds, ['model-a', 'model-b']);
});

test('does not confuse retry model changes or duplicate observations with a stream conflict', () => {
  const audit = getUpstreamModelAudit([
    { outboundModelID: 'model-a', upstreamModelID: 'model-a', upstreamModelIds: ['model-a', 'model-a'] },
    { outboundModelID: 'model-b', upstreamModelIds: ['model-b'] },
  ]);
  assert.equal(audit.status, 'matched');
  assert.equal(audit.conflictCount, 0);
  assert.deepEqual(audit.upstreamModelIds, ['model-a', 'model-b']);
});

test('a conflict does not hide other known mismatches or unknown executions', () => {
  const audit = getUpstreamModelAudit([
    { outboundModelID: 'model-a', upstreamModelIds: ['model-a', 'model-b'] },
    { outboundModelID: 'model-c', upstreamModelID: 'model-d' },
    { modelID: 'historical', upstreamModelID: 'historical' },
  ]);
  assert.equal(audit.status, 'conflicting');
  assert.equal(audit.unknownCount, 1);
  assert.equal(audit.comparedCount, 2);
  assert.deepEqual(audit.mismatchedModelIds, ['model-b', 'model-d']);
});
