import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const dataDir = import.meta.dirname;
const srcRoot = join(dataDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

function parseLocale(locale) {
  return JSON.parse(read(`locales/${locale}/system.json`));
}

test('Cline Pass unavailable quota does not require active windows', () => {
  const quotaTypes = read('features/system/data/quotas.ts');
  const quotaBadges = read('components/quota-badges.tsx');

  assert.match(quotaTypes, /pass_state:\s*'unavailable'/, 'quota data should model unavailable Cline Pass explicitly');
  assert.match(
    quotaTypes,
    /qd\.pool === 'cline_pass' && qd\.windows != null/,
    'the active Cline Pass type guard should require window data'
  );
  assert.match(
    quotaBadges,
    /isClineUnavailablePassQuotaData\(qd\)/,
    'the Cline quota row should render unavailable Pass data separately'
  );
  assert.match(quotaBadges, /quota\.status\.cline_pass_unavailable/, 'the status badge should use the Cline Pass unavailable label');
});

test('Cline Pass unavailable copy does not infer subscription expiry', () => {
  const en = parseLocale('en');
  const zh = parseLocale('zh-CN');

  assert.equal(en['quota.status.cline_pass_unavailable'], 'Cline Pass unavailable');
  assert.equal(en['quota.label.cline_pass_unavailable'], 'Cline Pass is currently unavailable for this account.');
  assert.equal(zh['quota.status.cline_pass_unavailable'], 'Cline Pass 不可用');
  assert.equal(zh['quota.label.cline_pass_unavailable'], '此账号当前无法使用 Cline Pass。');

  for (const message of [
    en['quota.status.cline_pass_unavailable'],
    en['quota.label.cline_pass_unavailable'],
    zh['quota.status.cline_pass_unavailable'],
    zh['quota.label.cline_pass_unavailable'],
  ]) {
    assert.doesNotMatch(message, /expired|subscription|到期|订阅/i);
  }
});
