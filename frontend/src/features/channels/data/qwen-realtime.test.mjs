import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import {
  QWEN_REALTIME_API_FORMAT,
  QWEN_REALTIME_PATH,
  QWEN_REALTIME_REGIONS,
  buildQwenRealtimeBaseURL,
  buildQwenRealtimeEndpoint,
  isQwenRealtimeEndpointAllowed,
  isValidQwenRealtimeRegion,
  isValidQwenRealtimeWorkspace,
  parseQwenRealtimeBaseURL,
} from './qwen-realtime.ts';
import { getConfigurableApiFormatsForChannelType } from './protocol-options.ts';
import { getChannelRelayProtocols, isRelayProtocol } from './relay-protocols.ts';

const dataDir = import.meta.dirname;
const frontendRoot = join(dataDir, '..', '..', '..');

function parseLocale(locale, file) {
  return JSON.parse(readFileSync(join(frontendRoot, `locales/${locale}/${file}`), 'utf8'));
}

test('uses the Bailian realtime websocket identity and path', () => {
  assert.equal(QWEN_REALTIME_API_FORMAT, 'qwen/realtime-ws');
  assert.equal(QWEN_REALTIME_PATH, '/api-ws/v1/realtime');
  assert.deepEqual([...QWEN_REALTIME_REGIONS], ['cn-beijing', 'ap-southeast-1']);
});

test('builds the trusted workspace and region base URL', () => {
  assert.equal(buildQwenRealtimeBaseURL('demo-ws', 'cn-beijing'), 'wss://demo-ws.cn-beijing.maas.aliyuncs.com');
  assert.equal(buildQwenRealtimeBaseURL('demo-ws', 'ap-southeast-1'), 'wss://demo-ws.ap-southeast-1.maas.aliyuncs.com');
  assert.equal(buildQwenRealtimeBaseURL(' Demo-WS ', 'cn-beijing'), 'wss://demo-ws.cn-beijing.maas.aliyuncs.com');
});

test('rejects unusable workspace identifiers and unsupported regions', () => {
  for (const workspace of ['', ' ', '-demo', 'demo-', 'demo_ws', 'demo.ws', 'a'.repeat(64)]) {
    assert.equal(isValidQwenRealtimeWorkspace(workspace), false, workspace);
  }
  for (const workspace of ['a', 'demo-ws', 'ws123', ' Demo-WS ']) {
    assert.equal(isValidQwenRealtimeWorkspace(workspace), true, workspace);
  }

  assert.equal(isValidQwenRealtimeRegion('cn-beijing'), true);
  assert.equal(isValidQwenRealtimeRegion('ap-southeast-1'), true);
  assert.equal(isValidQwenRealtimeRegion('us-east-1'), false);
  assert.throws(() => buildQwenRealtimeBaseURL('', 'cn-beijing'));
  assert.throws(() => buildQwenRealtimeBaseURL('demo-ws', 'us-east-1'));
});

test('parses only the generated host for the edit echo', () => {
  assert.deepEqual(parseQwenRealtimeBaseURL('wss://demo-ws.cn-beijing.maas.aliyuncs.com'), {
    workspace: 'demo-ws',
    region: 'cn-beijing',
  });
  assert.deepEqual(parseQwenRealtimeBaseURL('WSS://Demo-WS.ap-southeast-1.maas.aliyuncs.com/'), {
    workspace: 'demo-ws',
    region: 'ap-southeast-1',
  });

  for (const baseURL of [
    undefined,
    null,
    '',
    'https://demo-ws.cn-beijing.maas.aliyuncs.com',
    'wss://demo-ws.us-east-1.maas.aliyuncs.com',
    'wss://demo-ws.cn-beijing.maas.aliyuncs.com.example.com',
    'wss://demo-ws.cn-beijing.maas.aliyuncs.com:443',
    'wss://demo-ws.cn-beijing.maas.aliyuncs.com/api-ws/v1/realtime',
    'wss://demo_ws.cn-beijing.maas.aliyuncs.com',
  ]) {
    assert.equal(parseQwenRealtimeBaseURL(baseURL), null, String(baseURL));
  }
});

test('builds a websocket endpoint with the derived base URL and fixed path', () => {
  assert.deepEqual(buildQwenRealtimeEndpoint('demo-ws', 'ap-southeast-1'), {
    apiFormat: 'qwen/realtime-ws',
    baseURL: 'wss://demo-ws.ap-southeast-1.maas.aliyuncs.com',
    path: '/api-ws/v1/realtime',
    transport: 'websocket',
  });
});

test('is only configurable on a Bailian channel', () => {
  assert.equal(isQwenRealtimeEndpointAllowed('bailian'), true);
  for (const channelType of ['bailian_responses', 'bailian_anthropic', 'openai', 'zenmux']) {
    assert.equal(isQwenRealtimeEndpointAllowed(channelType), false, channelType);
  }
});

test('never offers realtime as a custom endpoint, including Bailian', () => {
  const formats = ['openai/chat_completions', 'qwen/realtime-ws'];
  assert.deepEqual(getConfigurableApiFormatsForChannelType('bailian', formats), ['openai/chat_completions']);
  for (const channelType of ['bailian_responses', 'bailian_anthropic', 'openai', 'zenmux']) {
    assert.deepEqual(getConfigurableApiFormatsForChannelType(channelType, formats), ['openai/chat_completions'], channelType);
  }
});

test('never treats the realtime websocket endpoint as a relay protocol', () => {
  assert.equal(isRelayProtocol('qwen/realtime-ws'), false);
  assert.equal(isRelayProtocol('openai/realtime'), false);
  assert.deepEqual([...getChannelRelayProtocols([], [{ apiFormat: 'qwen/realtime-ws' }])], []);
});

test('localizes the realtime formats distinctly from Responses WebSocket', () => {
  for (const locale of ['en', 'zh-CN']) {
    const channels = parseLocale(locale, 'channels.json');
    const models = parseLocale(locale, 'models.json');

    const openaiRealtime = channels['channels.dialogs.fields.apiFormat.formats.openai/realtime'];
    const qwenRealtime = channels['channels.dialogs.fields.apiFormat.formats.qwen/realtime-ws'];
    const responsesWebSocket = channels['channels.dialogs.fields.apiFormat.formats.openai/responses-ws'];

    assert.ok(openaiRealtime, `${locale} OpenAI Realtime label missing`);
    assert.ok(qwenRealtime, `${locale} Qwen Realtime label missing`);
    assert.ok(responsesWebSocket, `${locale} Responses WebSocket label missing`);
    assert.notEqual(openaiRealtime, qwenRealtime);
    assert.notEqual(openaiRealtime, responsesWebSocket);
    assert.notEqual(qwenRealtime, responsesWebSocket);
    assert.equal(qwenRealtime, 'Qwen (Realtime WebSocket)');
    assert.equal(channels['channels.endpoints.realtime.handshakeTimeout'], locale === 'zh-CN' ? '握手超时' : 'Handshake timeout');
    assert.match(channels['channels.endpoints.realtime.handshakeTimeoutHint'], locale === 'zh-CN' ? /毫秒/ : /ms/);
    assert.equal(channels['channels.endpoints.realtime.clientEventsPerSecond'], locale === 'zh-CN' ? '客户端事件速率上限' : 'Client event rate limit');
    assert.equal(channels['channels.endpoints.realtime.clientEventsPerSecondPlaceholder'], locale === 'zh-CN' ? '服务端默认值（1024 次/秒）' : 'Server default (1024 events/second)');
    assert.match(channels['channels.endpoints.realtime.clientEventsPerSecondHint'], locale === 'zh-CN' ? /256–4096.*125.*375/ : /256 to 4096.*125.*375/);
    assert.match(channels['channels.endpoints.realtime.invalidClientEventsPerSecond'], locale === 'zh-CN' ? /256–4096/ : /256 and 4096/);

    assert.ok(models['models.dialogs.association.conditions.formatOptions.openai/realtime'], `${locale} model format label missing`);
  }
});
