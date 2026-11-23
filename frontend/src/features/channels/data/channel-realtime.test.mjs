import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { getChannelBaseURLInputValue, getModelFetchBaseURL, getRealtimeDefaultEndpoint, getRealtimeHandshakeTimeoutInputValue, getRealtimeProviderBadgeClass, getRealtimeQuickModels, getRealtimeClientEventsPerSecondInputValue, realtimeForSelection, hasLegacyRealtimeEndpoints, withoutRealtimeModelProtocols } from './channel-realtime.ts';
import { getInitialApiFormatForChannel, getAvailableProtocolFormats } from './protocol-options.ts';

test('channel Realtime configuration saves and echoes without an endpoint', () => {
  const config = realtimeForSelection('qwen/realtime-ws', undefined, 'wss://llm-workspace.cn-beijing.maas.aliyuncs.com', '4567');
  assert.deepEqual(config, { enabled: true, protocol: 'qwen/realtime-ws', baseURL: 'wss://llm-workspace.cn-beijing.maas.aliyuncs.com', path: '/api-ws/v1/realtime', handshakeTimeoutMs: 4567 });
  assert.equal(getInitialApiFormatForChannel('bailian', 'openai/chat_completions', [], config), 'qwen/realtime-ws');
  assert.deepEqual(realtimeForSelection('openai/chat_completions', config, '', ''), { ...config, enabled: false });
  assert.equal(getInitialApiFormatForChannel('bailian', 'openai/chat_completions', [], { ...config, enabled: false }), 'openai/chat_completions');
});

test('Realtime Base URL is editable, defaults by provider, and accepts only secure WSS URLs', () => {
  assert.deepEqual(realtimeForSelection('openai/realtime-ws', undefined, '', ''), { enabled: true, protocol: 'openai/realtime-ws', baseURL: 'wss://api.openai.com', path: '/v1/realtime' });
  assert.equal(getChannelBaseURLInputValue('openai/realtime-ws', '', ''), 'wss://api.openai.com');
  assert.equal(realtimeForSelection('openai/realtime-ws', undefined, 'wss://proxy.example.test', '').baseURL, 'wss://proxy.example.test');
  assert.equal(getChannelBaseURLInputValue('qwen/realtime-ws', '', ''), '', 'the example belongs in the placeholder, not the editable value');
  for (const host of ['http://proxy.example.test', 'ws://proxy.example.test', 'wss://', 'wss://127.0.0.1', 'wss://proxy.example.test/path', 'wss://user@proxy.example.test']) assert.throws(() => realtimeForSelection('openai/realtime-ws', undefined, host, ''));
  for (const host of ['ws://llm-workspace.cn-beijing.maas.aliyuncs.com', 'wss://127.0.0.1', 'wss://workspace.cn-beijing.maas.aliyuncs.com?api_key=x', 'https://workspace.cn-beijing.maas.aliyuncs.com', 'wss://llm-<workspace-id>.cn-beijing.maas.aliyuncs.com']) assert.throws(() => realtimeForSelection('qwen/realtime-ws', undefined, host, ''));
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
  assert.match(snapshot, /\brealtimeClientEventsPerSecond,/);
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
  assert.equal(getChannelBaseURLInputValue('qwen/realtime-ws', httpBaseURL, 'wss://llm-w'), 'wss://llm-w');
  assert.equal(getChannelBaseURLInputValue('qwen/realtime-ws', httpBaseURL, ''), '', 'an empty edit must stay empty');
  assert.equal(getChannelBaseURLInputValue('openai/chat_completions', httpBaseURL, qwenHost), httpBaseURL);

  const actionDialog = readFileSync(new URL('../components/channels-action-dialog.tsx', import.meta.url), 'utf8');
  assert.match(actionDialog, /selectedApiFormat === 'qwen\/realtime-ws' \? t\('channels\.realtime\.qwenHostPlaceholder'\)/);
});

test('Realtime endpoint management projects only the selected protocol as its default endpoint', () => {
  assert.deepEqual(
    getRealtimeDefaultEndpoint({ enabled: true, protocol: 'openai/realtime-ws', baseURL: 'wss://api.openai.com', path: '/v1/realtime' }),
    { apiFormat: 'openai/realtime-ws' }
  );
  assert.deepEqual(
    getRealtimeDefaultEndpoint({ enabled: true, protocol: 'qwen/realtime-ws', baseURL: 'wss://llm-workspace.cn-beijing.maas.aliyuncs.com', path: '/api-ws/v1/realtime', handshakeTimeoutMs: 2345 }),
    { apiFormat: 'qwen/realtime-ws' }
  );
  assert.equal(getRealtimeDefaultEndpoint({ enabled: false, protocol: '', baseURL: '', path: '' }), undefined);

  const endpointDialog = readFileSync(new URL('../components/channels-endpoints-dialog.tsx', import.meta.url), 'utf8');
  assert.match(endpointDialog, /getRealtimeDefaultEndpoint\(channel\.settings\?\.realtime\)/);
  assert.match(endpointDialog, /if \(realtimeDefaultEndpoint\) return \[realtimeDefaultEndpoint\]/);
  assert.match(endpointDialog, /const defaults = channel\.defaultEndpoints \?\? \[\]/);
  assert.match(endpointDialog, /EndpointTable endpoints=\{defaultEndpoints\} readOnly hideBaseURL/);
  assert.doesNotMatch(endpointDialog, /if \(channel\.settings\?\.realtime\?\.enabled\) \{\s*return \(/);
});

test('Realtime model discovery uses the HTTP API host derived from the selected WebSocket host', () => {
  const ordinaryBaseURL = 'https://dashscope.aliyuncs.com/compatible-mode/v1';
  const qwenHost = 'wss://llm-workspace.cn-beijing.maas.aliyuncs.com';
  assert.equal(getModelFetchBaseURL('openai/realtime-ws', ordinaryBaseURL, ''), 'https://api.openai.com');
  assert.equal(getModelFetchBaseURL('qwen/realtime-ws', ordinaryBaseURL, qwenHost), 'https://llm-workspace.cn-beijing.maas.aliyuncs.com');
  assert.equal(getModelFetchBaseURL('openai/chat_completions', ordinaryBaseURL, qwenHost), ordinaryBaseURL);
  assert.equal(getModelFetchBaseURL('qwen/realtime-ws', ordinaryBaseURL, ''), '');

  const actionDialog = readFileSync(new URL('../components/channels-action-dialog.tsx', import.meta.url), 'utf8');
  const fetchModelsHandler = actionDialog.match(/const handleFetchModels = useCallback\([\s\S]*?\n  \}, \[/)?.[0];
  assert.ok(fetchModelsHandler, 'model fetch handler should exist');
  assert.match(fetchModelsHandler, /getModelFetchBaseURL\(selectedApiFormat/);
});

test('Realtime provider badges vary by protocol while preserving the endpoint protocol column', () => {
  assert.match(getRealtimeProviderBadgeClass({ enabled: true, protocol: 'openai/realtime-ws' }), /teal/);
  assert.match(getRealtimeProviderBadgeClass({ enabled: true, protocol: 'qwen/realtime-ws' }), /fuchsia/);
  assert.equal(getRealtimeProviderBadgeClass({ enabled: false, protocol: 'openai/realtime-ws' }), undefined);

  const columns = readFileSync(new URL('../components/channels-columns.tsx', import.meta.url), 'utf8');
  const providerCell = columns.match(/const ProviderCell[\s\S]*?ProviderCell\.displayName/)?.[0];
  const endpointCell = columns.match(/const EndpointProtocolsCell[\s\S]*?EndpointProtocolsCell\.displayName/)?.[0];
  assert.match(providerCell, /getRealtimeProviderBadgeClass\(row\.original\.settings\?\.realtime\)/);
  assert.match(providerCell, /<span>\{t\(`channels\.providers\.\$\{provider\}`\)\}<\/span>/);
  assert.match(endpointCell, /getChannelRelayProtocols\(channel\.defaultEndpoints, channel\.endpoints, channel\.settings\?\.realtime\?\.enabled\)/);
  assert.doesNotMatch(endpointCell, /REALTIME_PROTOCOL_CLASSES|apiFormat\.formats/);
});

test('Realtime handshake timeout keeps inherited server defaults unset in the form', () => {
  assert.equal(getRealtimeHandshakeTimeoutInputValue(undefined), '');
  assert.equal(getRealtimeHandshakeTimeoutInputValue(null), '');
  assert.equal(getRealtimeHandshakeTimeoutInputValue(30000), '30000');

  const inherited = realtimeForSelection(
    'openai/realtime-ws',
    { enabled: true, protocol: 'openai/realtime-ws', baseURL: 'wss://api.openai.com', path: '/v1/realtime' },
    'wss://api.openai.com',
    getRealtimeHandshakeTimeoutInputValue(undefined)
  );
  assert.equal('handshakeTimeoutMs' in inherited, false, 'unrelated edits must not persist an inherited default as an override');

  const configured = realtimeForSelection(
    'openai/realtime-ws',
    null,
    'wss://api.openai.com',
    getRealtimeHandshakeTimeoutInputValue(30000)
  );
  assert.equal(configured.handshakeTimeoutMs, 30000, 'an explicit channel override must still be saved');

  const actionDialog = readFileSync(new URL('../components/channels-action-dialog.tsx', import.meta.url), 'utf8');
  assert.match(actionDialog, /placeholder=\{t\('channels\.endpoints\.realtime\.handshakeTimeoutPlaceholder'\)\}/);
});


test('Realtime Host reuses the channel Base URL control without a separate panel or path field', () => {
  const actionDialog = readFileSync(new URL('../components/channels-action-dialog.tsx', import.meta.url), 'utf8');
  assert.match(actionDialog, /name='baseURL'/);
  assert.match(actionDialog, /getChannelBaseURLInputValue\(selectedApiFormat/);
  assert.doesNotMatch(actionDialog, /readOnly=\{selectedApiFormat === 'openai\/realtime-ws'\}/);
  assert.match(actionDialog, /onChange=\{isRealtimeProtocol\(selectedApiFormat\) \? \(event\) => setRealtimeHost\(event\.target\.value\) : field\.onChange\}/);
  assert.doesNotMatch(actionDialog, /isRealtimeProtocol\(selectedApiFormat\) \? t\('channels\.realtime\.host'\)/);
  assert.doesNotMatch(actionDialog, /data-testid='channel-realtime-config'/);
  assert.doesNotMatch(actionDialog, /data-testid='channel-realtime-host'/);
  assert.match(actionDialog, /data-testid='channel-realtime-timeout'/);
});

test('Realtime handshake timeout uses an info tooltip with localized default guidance', () => {
  const actionDialog = readFileSync(new URL('../components/channels-action-dialog.tsx', import.meta.url), 'utf8');
  const timeoutRow = actionDialog.match(/data-testid='channel-realtime-timeout'>([\s\S]*?)<\/FormItem>/)?.[1];
  assert.ok(timeoutRow, 'Realtime handshake timeout row should exist');
  assert.ok(
    actionDialog.indexOf("t('channels.dialogs.responseTimeout.label')") < actionDialog.indexOf("data-testid='channel-realtime-timeout'"),
    'Realtime handshake timeout should follow response timeout settings'
  );
  assert.match(timeoutRow, /md:justify-start/);
  assert.match(timeoutRow, /aria-label=\{t\('channels\.endpoints\.realtime\.handshakeTimeoutHint'\)\}/);
  assert.match(timeoutRow, /<p>\{t\('channels\.endpoints\.realtime\.handshakeTimeoutHint'\)\}<\/p>/);
});

test('Realtime client event limit saves explicit values and leaves inherited defaults null', () => {
  assert.equal(getRealtimeClientEventsPerSecondInputValue(undefined), '');
  assert.equal(getRealtimeClientEventsPerSecondInputValue(null), '');
  assert.equal(getRealtimeClientEventsPerSecondInputValue(300), '300');
  assert.equal(realtimeForSelection('openai/realtime-ws', null, 'wss://api.openai.com', '', '').clientEventsPerSecond, null);
  assert.equal(realtimeForSelection('openai/realtime-ws', null, 'wss://api.openai.com', '', '300').clientEventsPerSecond, 300);
  for (const rate of ['100', '255', '4097', '5000', '300.5', 'NaN']) {
    assert.throws(() => realtimeForSelection('openai/realtime-ws', null, 'wss://api.openai.com', '', rate), /invalidClientEventsPerSecond/);
  }

  const actionDialog = readFileSync(new URL('../components/channels-action-dialog.tsx', import.meta.url), 'utf8');
  assert.match(actionDialog, /data-testid='channel-realtime-client-events-per-second'/);
  assert.match(actionDialog, /channels\.endpoints\.realtime\.clientEventsPerSecondHint/);
});
