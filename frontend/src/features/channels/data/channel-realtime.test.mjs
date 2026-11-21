import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { getChannelBaseURLInputValue, getRealtimeHandshakeTimeoutInputValue, getRealtimeQuickModels, realtimeForSelection, hasLegacyRealtimeEndpoints, withoutRealtimeModelProtocols } from './channel-realtime.ts';
import { getInitialApiFormatForChannel, getAvailableProtocolFormats } from './protocol-options.ts';

test('channel Realtime configuration saves and echoes without an endpoint', () => {
  const config = realtimeForSelection('qwen/realtime-ws', undefined, 'wss://llm-workspace.cn-beijing.maas.aliyuncs.com', '4567');
  assert.deepEqual(config, { enabled: true, protocol: 'qwen/realtime-ws', baseURL: 'wss://llm-workspace.cn-beijing.maas.aliyuncs.com', path: '/api-ws/v1/realtime', handshakeTimeoutMs: 4567 });
  assert.equal(getInitialApiFormatForChannel('bailian', 'openai/chat_completions', [], config), 'qwen/realtime-ws');
  assert.deepEqual(realtimeForSelection('openai/chat_completions', config, '', ''), { ...config, enabled: false });
  assert.equal(getInitialApiFormatForChannel('bailian', 'openai/chat_completions', [], { ...config, enabled: false }), 'openai/chat_completions');
});

test('OpenAI channel uses its official host while malformed hosts/timeouts are rejected', () => {
  assert.deepEqual(realtimeForSelection('openai/realtime-ws', undefined, '', ''), { enabled: true, protocol: 'openai/realtime-ws', baseURL: 'wss://api.openai.com', path: '/v1/realtime' });
  assert.equal(realtimeForSelection('openai/realtime-ws', undefined, 'wss://api.openai.com/v1/realtime', '').baseURL, 'wss://api.openai.com');
  for (const host of ['ws://api.openai.com', 'wss://api.openai.com.evil.test', 'wss://api.openai.com/path']) assert.throws(() => realtimeForSelection('openai/realtime-ws', undefined, host, ''));
  for (const host of ['wss://127.0.0.1', 'wss://workspace.cn-beijing.maas.aliyuncs.com?api_key=x', 'https://workspace.cn-beijing.maas.aliyuncs.com']) assert.throws(() => realtimeForSelection('qwen/realtime-ws', undefined, host, ''));
  for (const timeout of ['NaN', '999', '60001', '1234.5']) assert.throws(() => realtimeForSelection('openai/realtime-ws', undefined, '', timeout));
});

test('legacy Realtime endpoints are detected rather than silently filtered for saving', () => {
  assert.equal(hasLegacyRealtimeEndpoints([{ apiFormat: 'openai/realtime-ws' }]), true);
  assert.equal(hasLegacyRealtimeEndpoints([{ apiFormat: 'qwen/realtime-ws' }]), true);
  assert.equal(hasLegacyRealtimeEndpoints([{ apiFormat: 'openai/chat_completions' }]), false);
  assert.deepEqual(getAvailableProtocolFormats([{ apiFormat: 'openai/chat_completions' }], [{ apiFormat: 'qwen/realtime-ws' }]), ['openai/chat_completions']);
});

test('switching to an incompatible channel type removes only obsolete Realtime mappings', () => {
  const source = [{ model: 'voice', apiFormats: ['qwen/realtime-ws'], enabled: true }, { model: 'mixed', apiFormats: ['qwen/realtime-ws', 'openai/chat_completions'], enabled: true }];
  assert.deepEqual(withoutRealtimeModelProtocols(source), [{ model: 'mixed', apiFormats: ['openai/chat_completions'], enabled: true }]);
  assert.deepEqual(source[0].apiFormats, ['qwen/realtime-ws']);
});

test('channel change snapshot detects Realtime host and timeout edits', () => {
  const actionDialog = readFileSync(new URL('../components/channels-action-dialog.tsx', import.meta.url), 'utf8');
  const snapshot = actionDialog.match(/const localSettingsSnapshot = JSON.stringify\(\{([\s\S]*?)\n  \}\);/)?.[1];
  assert.ok(snapshot, 'local settings snapshot should exist');
  assert.match(snapshot, /\brealtimeHost,/);
  assert.match(snapshot, /\brealtimeTimeout,/);
});

test('quick models follow the selected Realtime protocol', () => {
  assert.deepEqual(getRealtimeQuickModels('openai/realtime-ws'), ['gpt-realtime-2.1', 'gpt-realtime-2.1-mini', 'gpt-realtime-translate']);
  assert.deepEqual(getRealtimeQuickModels('qwen/realtime-ws'), ['qwen3.8-omni-flash-realtime', 'qwen3.5-omni-plus-realtime', 'qwen3.5-omni-flash-realtime']);
  assert.equal(getRealtimeQuickModels('openai/chat_completions'), undefined);
});

test('Realtime reuses the Base URL field and keeps the saved HTTP value when toggled', () => {
  const httpBaseURL = 'https://dashscope.aliyuncs.com/compatible-mode/v1';
  const qwenHost = 'wss://llm-ikngb8j2sjh4l2sw.cn-beijing.maas.aliyuncs.com';
  assert.equal(getChannelBaseURLInputValue('openai/realtime-ws', httpBaseURL, qwenHost), 'wss://api.openai.com');
  assert.equal(getChannelBaseURLInputValue('qwen/realtime-ws', httpBaseURL, qwenHost), qwenHost);
  assert.equal(getChannelBaseURLInputValue('openai/chat_completions', httpBaseURL, qwenHost), httpBaseURL);
});

test('Realtime handshake timeout displays the server default unless the channel has an override', () => {
  assert.equal(getRealtimeHandshakeTimeoutInputValue(undefined), '15000');
  assert.equal(getRealtimeHandshakeTimeoutInputValue(null), '15000');
  assert.equal(getRealtimeHandshakeTimeoutInputValue(30000), '30000');
});


test('Realtime Host reuses the channel Base URL control without a separate panel or path field', () => {
  const actionDialog = readFileSync(new URL('../components/channels-action-dialog.tsx', import.meta.url), 'utf8');
  assert.match(actionDialog, /name='baseURL'/);
  assert.match(actionDialog, /getChannelBaseURLInputValue\(selectedApiFormat/);
  assert.doesNotMatch(actionDialog, /data-testid='channel-realtime-config'/);
  assert.doesNotMatch(actionDialog, /data-testid='channel-realtime-host'/);
  assert.match(actionDialog, /data-testid='channel-realtime-timeout'/);
});
