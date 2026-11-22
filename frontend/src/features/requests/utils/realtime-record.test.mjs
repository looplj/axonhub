import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { getRealtimeAuditRecordKind } from './realtime-record.ts';

test('distinguishes Realtime session audits from individual response audits', () => {
  assert.equal(getRealtimeAuditRecordKind('openai/realtime', { error_code: 'client_closed' }), 'session');
  assert.equal(getRealtimeAuditRecordKind('openai/realtime', { response_id: 'resp_123' }), 'response');
  assert.equal(getRealtimeAuditRecordKind('openai/realtime', { response_create_rejected: true }), 'response');
  assert.equal(getRealtimeAuditRecordKind('openai/chat_completions', { response_id: 'resp_123' }), undefined);
});

test('request details render localized labels for the detected Realtime record kind', () => {
  const component = readFileSync(new URL('../components/request-detail-content.tsx', import.meta.url), 'utf8');
  assert.match(component, /getRealtimeAuditRecordKind\(request\.format, request\.responseBody\)/);
  assert.match(component, /realtimeRecordKind === 'response' \? 'requests\.detail\.realtimeResponseRecord' : 'requests\.detail\.realtimeSession'/);

  for (const locale of ['en', 'zh-CN']) {
    const messages = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/requests.json`, import.meta.url), 'utf8'));
    assert.ok(messages['requests.detail.realtimeSession']);
    assert.ok(messages['requests.detail.realtimeResponseRecord']);
  }
});
