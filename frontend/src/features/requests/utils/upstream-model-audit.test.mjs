import i18next from 'i18next';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { getExecutionModelAuditVerdict, getRequestModelAuditTooltip, getUpstreamModelAudit } from './upstream-model-audit.ts';

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
  const retry = { status: 'failed', modelID: 'glm-5.3:free', upstreamModelID: 'glm-5.3:free' };
  const success = { status: 'completed', modelID: 'glm-5.3', upstreamModelID: 'glm-5.3' };
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
    }
  }
});

test('unknown retries retain their count alongside the successful model in both list languages', () => {
  const audit = getUpstreamModelAudit([
    { status: 'failed', modelID: 'glm-5.3:free' },
    { status: 'completed', modelID: 'glm-5.3', upstreamModelID: 'glm-5.3' },
  ]);
  for (const locale of ['en', 'zh-CN']) {
    const tooltip = getRequestModelAuditTooltip(audit, 'completed', i18n.getFixedT(locale));
    assert.ok(tooltip.includes('glm-5.3'));
    assert.ok(tooltip.includes('1'));
    assert.ok(!tooltip.includes('glm-5.3:free'));
    assert.ok(!tooltip.includes('{{'));
  }
});

test('list warnings still display a mismatch from a failed retry after a successful match', () => {
  const audit = getUpstreamModelAudit([
    { status: 'failed', modelID: 'sent', upstreamModelID: 'different' },
    { status: 'completed', modelID: 'glm-5.3', upstreamModelID: 'glm-5.3' },
  ]);
  assert.equal(audit.status, 'mismatched');
  for (const locale of ['en', 'zh-CN']) {
    assert.ok(getRequestModelAuditTooltip(audit, 'completed', i18n.getFixedT(locale)).includes('different'));
  }
});

test('missing successful evidence never falls back to a failed retry model', () => {
  const retry = { status: 'failed', modelID: 'glm-5.3:free', upstreamModelID: 'glm-5.3:free' };
  for (const executions of [[retry], [retry, { status: 'completed', modelID: 'glm-5.3' }]]) {
    const audit = getUpstreamModelAudit(executions);
    assert.deepEqual(audit.matchedUpstreamIds, []);
    assert.ok(!getRequestModelAuditTooltip(audit, 'completed', i18n.t).includes('glm-5.3:free'));
  }
});

test('request lifecycle takes precedence over model match tooltips', () => {
  const audit = getUpstreamModelAudit([{ status: 'completed', modelID: 'sent', upstreamModelID: 'sent' }]);
  for (const status of ['pending', 'processing', 'failed', 'canceled']) {
    const key = status === 'pending' || status === 'processing' ? 'upstreamModelRequestProcessing' : 'upstreamModelRequestFailed';
    assert.equal(getRequestModelAuditTooltip(audit, status, i18n.t), i18n.t('requests.tooltips.' + key));
  }
});

test('successful matching retries stay matched without discarding failed or canceled unknowns', () => {
  for (const status of ['failed', 'canceled']) {
    const executions = [
      ...Array.from({ length: 12 }, () => ({ status, modelID: 'sent' })),
      { status: 'completed', modelID: 'sent', upstreamModelID: 'sent' },
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
      { status: 'completed', modelID: 'sent', upstreamModelID: 'sent' },
      { status, modelID: 'sent' },
    ]);
    assert.equal(audit.status, 'unknown');
  }
});

test('a failed comparison cannot establish a match when final success is unknown or absent', () => {
  for (const status of ['completed', 'failed']) {
    const audit = getUpstreamModelAudit([
      { status: 'failed', modelID: 'sent', upstreamModelID: 'sent' },
      { status, modelID: 'sent' },
    ]);
    assert.equal(audit.status, 'unknown');
  }
});

test('successful retries do not hide an earlier mismatch', () => {
  const audit = getUpstreamModelAudit([
    { status: 'failed', modelID: 'sent', upstreamModelID: 'different' },
    { status: 'failed' },
    { status: 'completed', modelID: 'sent', upstreamModelID: 'sent' },
  ]);
  assert.equal(audit.status, 'mismatched');
  assert.deepEqual(audit.mismatchedModelIds, ['different']);
  assert.equal(audit.unknownCount, 1);
});

test('matches each execution against its own recorded channel model', () => {
  const audit = getUpstreamModelAudit([
    { modelID: 'model-a', upstreamModelID: 'model-a' },
    { modelID: 'model-b', upstreamModelID: 'model-b' },
  ]);
  assert.equal(audit.status, 'matched');
  assert.equal(audit.comparedCount, 2);
  assert.equal(audit.unknownCount, 0);
});

test('does not match a retry against a previous execution model', () => {
  const audit = getUpstreamModelAudit([
    { modelID: 'model-a', upstreamModelID: null },
    { modelID: 'model-b', upstreamModelID: 'model-a' },
  ]);
  assert.equal(audit.status, 'mismatched');
  assert.deepEqual(audit.mismatchedModelIds, ['model-a']);
  assert.equal(audit.unknownCount, 1);
});

test('detects swapped models even when both appear in the channel model set', () => {
  const audit = getUpstreamModelAudit([
    { modelID: 'model-a', upstreamModelID: 'model-b' },
    { modelID: 'model-b', upstreamModelID: 'model-a' },
  ]);
  assert.equal(audit.status, 'mismatched');
  assert.deepEqual(audit.mismatchedModelIds, ['model-b', 'model-a']);
});

test('does not mark partially unknown executions as matched', () => {
  const audit = getUpstreamModelAudit([{ modelID: 'model-a', upstreamModelID: 'model-a' }, { modelID: 'model-b' }]);
  assert.equal(audit.status, 'unknown');
  assert.equal(audit.comparedCount, 1);
  assert.equal(audit.unknownCount, 1);
});

test('keeps empty, historical, and missing-model executions unknown', () => {
  assert.equal(getUpstreamModelAudit([]).status, 'unknown');
  for (const execution of [
    { modelID: 'model-a' },
    { modelID: 'model-a', upstreamModelID: null },
    { modelID: 'model-a', upstreamModelID: '' },
    { modelID: 'model-a', upstreamModelID: '   ' },
    { modelID: '', upstreamModelID: 'model-a' },
    { upstreamModelID: 'model-a' },
  ]) {
    const audit = getUpstreamModelAudit([execution]);
    assert.equal(audit.status, 'unknown');
    assert.equal(audit.unknownCount, 1);
    assert.equal(audit.comparedCount, 0);
  }
});

test('preserves exact reported names instead of normalizing versions or case', () => {
  const audit = getUpstreamModelAudit([
    { modelID: 'model-a', upstreamModelID: 'Model-A' },
    { modelID: 'model-a', upstreamModelID: 'model-a-2026-09-19' },
  ]);
  assert.equal(audit.status, 'mismatched');
  assert.deepEqual(audit.mismatchedModelIds, ['Model-A', 'model-a-2026-09-19']);
});

test('deduplicates reported names only after retaining every execution mismatch', () => {
  const audit = getUpstreamModelAudit([
    { modelID: 'model-a', upstreamModelID: 'model-a' },
    { modelID: 'model-b', upstreamModelID: 'model-a' },
  ]);
  assert.equal(audit.status, 'mismatched');
  assert.deepEqual(audit.mismatchedModelIds, ['model-a']);
});

test('compares the channel model with the reported name even when the provider transforms it', () => {
  const execution = { status: 'completed', modelID: 'gemini-3-pro', upstreamModelID: 'gemini-3-pro-low' };
  const audit = getUpstreamModelAudit([execution]);
  assert.equal(audit.status, 'mismatched');
  assert.deepEqual(audit.mismatchedModelIds, ['gemini-3-pro-low']);
  assert.equal(getUpstreamModelAudit([{ ...execution, upstreamModelID: 'gemini-3-pro' }]).status, 'matched');
});

test('a failed execution never reports a green success conclusion', () => {
  for (const execution of [
    { status: 'failed', modelID: 'gpt-6-astra', upstreamModelID: 'gpt-6-astra' },
    { status: 'failed', modelID: 'glm-5.3:free', upstreamModelID: 'glm-5.3:free' },
  ]) {
    const audit = getUpstreamModelAudit([execution]);
    assert.equal(audit.status, 'matched');
    assert.deepEqual(audit.matchedUpstreamIds, []);
    for (const locale of ['en', 'zh-CN']) {
      const verdict = getExecutionModelAuditVerdict(execution, i18n.getFixedT(locale));
      assert.equal(verdict.tone, 'muted');
      assert.ok(verdict.message.includes(execution.upstreamModelID));
      assert.ok(!verdict.message.includes('{{'));
    }
  }
});

test('a completed execution keeps the green success conclusion', () => {
  const verdict = getExecutionModelAuditVerdict(
    { status: 'completed', modelID: 'glm-5.3', upstreamModelID: 'glm-5.3' },
    i18n.getFixedT('zh-CN')
  );
  assert.equal(verdict.tone, 'success');
  assert.equal(verdict.message, i18n.t('requests.detail.upstreamModelMatched'));
});

test('a failed execution without model evidence keeps the failure reason prompt', () => {
  for (const status of ['failed', 'canceled']) {
    const verdict = getExecutionModelAuditVerdict({ status, modelID: 'glm-5.3:free' }, i18n.getFixedT('zh-CN'));
    assert.equal(verdict.tone, 'danger');
    assert.equal(verdict.message, i18n.t('requests.tooltips.upstreamModelRequestFailed'));
  }
});

test('a known mismatch outranks the failed lifecycle so it stays red', () => {
  const mismatch = getExecutionModelAuditVerdict(
    { status: 'failed', modelID: 'sent', upstreamModelID: 'different' },
    i18n.getFixedT('zh-CN')
  );
  assert.equal(mismatch.tone, 'danger');
  assert.ok(mismatch.message.includes('different'));
});

test('in-flight executions report the waiting state instead of a name conclusion', () => {
  for (const status of ['pending', 'processing']) {
    const verdict = getExecutionModelAuditVerdict({ status, modelID: 'sent', upstreamModelID: 'sent' }, i18n.getFixedT('zh-CN'));
    assert.equal(verdict.tone, 'pending');
    assert.equal(verdict.message, i18n.t('requests.tooltips.upstreamModelRequestProcessing'));
  }
});

test('each execution yields exactly one verdict and one locale key per tone', () => {
  const cases = [
    { status: 'failed', modelID: 'a', upstreamModelID: 'a' },
    { status: 'failed', modelID: 'a' },
    { status: 'completed', modelID: 'a', upstreamModelID: 'a' },
    { status: 'completed', modelID: 'a' },
    { status: 'failed', modelID: 'a', upstreamModelID: 'b' },
  ];
  for (const execution of cases) {
    for (const locale of ['en', 'zh-CN']) {
      const verdict = getExecutionModelAuditVerdict(execution, i18n.getFixedT(locale));
      assert.equal(typeof verdict.message, 'string');
      assert.ok(verdict.message.length > 0);
      assert.ok(!verdict.message.includes('{{'));
      assert.ok(['success', 'danger', 'pending', 'muted'].includes(verdict.tone));
    }
  }
});
