import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import {
  buildCodexResponsesEndpoints,
  getApiFormatsForProvider,
  getAvailableProtocolFormats,
  getChannelTypeForApiFormat,
  getCodexResponsesWebSocketState,
  getCodexResponsesWebSocketURL,
  hasCodexResponsesWebSocketOverrideReplacement,
  getConfigurableApiFormatsForChannelType,
  getEffectiveResponsesFormats,
  getInitialApiFormatForChannel,
  getModelProtocolsForApiFormat,
  isResponsesFamilyFormat,
  toHttpBaseURL,
  translateLegacyCodexModelProtocols,
} from './protocol-options.ts';
import { getApiPath } from '../../requests/utils/curl-paths.ts';

const dataDir = import.meta.dirname;
const frontendRoot = join(dataDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(frontendRoot, relativePath), 'utf8');
}

function parseLocale(locale, file) {
  return JSON.parse(read(`locales/${locale}/${file}`));
}

const providerConfigs = {
  zenmux: { channelTypes: ['zenmux', 'zenmux_responses', 'zenmux_video'] },
  openai: { channelTypes: ['openai', 'openai_responses'] },
};

const channelConfigs = {
  zenmux: { apiFormat: 'openai/chat_completions' },
  zenmux_responses: { apiFormat: 'openai/responses' },
  zenmux_anthropic: { apiFormat: 'anthropic/messages' },
  zenmux_gemini: { apiFormat: 'gemini/contents' },
  zenmux_video: { apiFormat: 'zenmux/video' },
  openai: { apiFormat: 'openai/chat_completions' },
  openai_responses: { apiFormat: 'openai/responses' },
};

const configs = { providerConfigs, channelConfigs };

test('includes ZenMux native video in the add-channel provider formats', () => {
  assert.deepEqual(getApiFormatsForProvider('zenmux', configs), ['openai/chat_completions', 'openai/responses', 'zenmux/video']);
});

test('maps ZenMux native video back to the dedicated ZenMux video channel type', () => {
  assert.equal(getChannelTypeForApiFormat('zenmux', 'zenmux/video', configs), 'zenmux_video');
});

test('exposes the native video default endpoint to model protocol editing', () => {
  assert.deepEqual(getAvailableProtocolFormats([{ apiFormat: 'zenmux/video' }], []), ['zenmux/video']);
});

test('does not expose ZenMux native video to unrelated providers', () => {
  assert.deepEqual(getApiFormatsForProvider('openai', configs), ['openai/chat_completions', 'openai/responses']);
  assert.equal(getChannelTypeForApiFormat('openai', 'zenmux/video', configs), undefined);
});

test('does not expose ZenMux native video to a provider lacking the ZenMux video channel type', () => {
  const openaiOnly = { providerConfigs: { zenmux: { channelTypes: ['zenmux_responses', 'zenmux_anthropic', 'zenmux_gemini'] } }, channelConfigs };
  assert.deepEqual(getApiFormatsForProvider('zenmux', openaiOnly), ['openai/responses', 'anthropic/messages', 'gemini/contents']);
  assert.equal(getChannelTypeForApiFormat('zenmux', 'zenmux/video', openaiOnly), undefined);
});

test('preserves the existing reverse mapping for known formats', () => {
  assert.equal(getChannelTypeForApiFormat('zenmux', 'openai/responses', configs), 'zenmux_responses');
});

test('persists ZenMux native video for every selected supported model', () => {
  assert.deepEqual(getModelProtocolsForApiFormat('zenmux/video', ['sora-2', 'veo-3.1']), [
    { model: 'sora-2', apiFormats: ['zenmux/video'], enabled: true },
    { model: 'veo-3.1', apiFormats: ['zenmux/video'], enabled: true },
  ]);
});

test('keeps per-model protocol overrides when selecting ZenMux native video in a mixed configuration', () => {
  const existing = [
    { model: 'gpt-5', apiFormats: ['openai/chat_completions'], enabled: true },
    { model: 'sora-2', apiFormats: ['zenmux/video'], enabled: true },
  ];
  assert.deepEqual(getModelProtocolsForApiFormat('zenmux/video', ['gpt-5', 'sora-2', 'veo-3.1'], existing), [
    { model: 'gpt-5', apiFormats: ['openai/chat_completions'], enabled: true },
    { model: 'sora-2', apiFormats: ['zenmux/video'], enabled: true },
    { model: 'veo-3.1', apiFormats: ['zenmux/video'], enabled: true },
  ]);
});

test('edit without format change keeps existing model protocol selections unchanged', () => {
  const existing = [
    { model: 'gpt-5', apiFormats: ['openai/chat_completions'], enabled: true },
    { model: 'sora-2', apiFormats: ['zenmux/video'], enabled: true },
  ];
  const result = getModelProtocolsForApiFormat('zenmux/video', ['gpt-5', 'sora-2'], existing);
  assert.deepEqual(result, existing);
  assert.equal(result[1].apiFormats[0], 'zenmux/video');
});

test('selecting ZenMux native video never replaces non-video overrides', () => {
  const existing = [
    { model: 'sora-2', apiFormats: ['zenmux/video', 'openai/chat_completions'], enabled: true },
    { model: 'veo-3.1', apiFormats: ['openai/chat_completions'], enabled: true },
  ];
  assert.deepEqual(getModelProtocolsForApiFormat('zenmux/video', ['sora-2', 'veo-3.1', 'new-video'], existing), [
    { model: 'sora-2', apiFormats: ['zenmux/video', 'openai/chat_completions'], enabled: true },
    { model: 'veo-3.1', apiFormats: ['openai/chat_completions'], enabled: true },
    { model: 'new-video', apiFormats: ['zenmux/video'], enabled: true },
  ]);
});

test('restores ZenMux native video from persisted model protocols', () => {
  assert.equal(
    getInitialApiFormatForChannel('zenmux', 'openai/chat_completions', [{ model: 'sora-2', apiFormats: ['zenmux/video'], enabled: true }]),
    'zenmux/video'
  );
});

test('keeps non-video protocols unchanged', () => {
  assert.deepEqual(getModelProtocolsForApiFormat('openai/chat_completions', ['gpt-5']), []);
  assert.equal(
    getInitialApiFormatForChannel('zenmux_responses', 'openai/responses', [
      { model: 'sora-2', apiFormats: ['zenmux/video'], enabled: true },
    ]),
    'openai/responses'
  );
});

test('removes stale ZenMux video overrides when an edited channel leaves video format', () => {
  assert.deepEqual(
    getModelProtocolsForApiFormat(
      'openai/chat_completions',
      [],
      [
        { model: 'sora-2', apiFormats: ['zenmux/video'], enabled: true },
        { model: 'gpt-5', apiFormats: ['zenmux/video', 'openai/chat_completions'], enabled: true },
      ]
    ),
    [{ model: 'gpt-5', apiFormats: ['openai/chat_completions'], enabled: true }]
  );
});

const configurableFormats = ['openai/chat_completions', 'openai/responses', 'typesafe/systemone', 'openai/decisions'];

test('shows the OpenAI Decisions endpoint only for the OpenAI create/edit channel type', () => {
  const openai = getConfigurableApiFormatsForChannelType('openai', configurableFormats);
  assert.ok(openai.includes('openai/decisions'), 'openai channel type should offer openai/decisions');

  for (const channelType of ['openai_responses', 'anthropic', 'gemini', 'zenmux', 'typesafe', 'codex', 'xai']) {
    const formats = getConfigurableApiFormatsForChannelType(channelType, configurableFormats);
    assert.ok(!formats.includes('openai/decisions'), `${channelType} must not offer openai/decisions in the UI`);
  }
});

test('keeps the SystemOne endpoint option unchanged when openai/decisions is offered', () => {
  assert.deepEqual(getConfigurableApiFormatsForChannelType('typesafe', configurableFormats), ['typesafe/systemone']);
  assert.ok(getConfigurableApiFormatsForChannelType('openai', configurableFormats).includes('typesafe/systemone'));
});

test('offers the SystemOne endpoint on Cloudflare Workers AI channels', () => {
  assert.deepEqual(getConfigurableApiFormatsForChannelType('cloudflare', configurableFormats), [
    'openai/chat_completions',
    'openai/responses',
    'typesafe/systemone',
  ]);
});

test('accepts openai/decisions in the API-format schema and configurable endpoint list', () => {
  const schema = read('features/channels/data/schema.ts');
  assert.match(schema, /apiFormatSchema[\s\S]*'openai\/decisions'/, 'apiFormatSchema should accept openai/decisions');
  assert.match(
    schema,
    /configurableChannelEndpointApiFormats[^;]*'openai\/decisions'/,
    'the endpoints dialog list should offer openai/decisions'
  );
});

test('routes the Decisions request to /v1/decisions for generated cURL', () => {
  assert.equal(getApiPath('openai/decisions'), '/v1/decisions');
});

test('localizes the openai/decisions endpoint label in channels and models locales', () => {
  for (const locale of ['en', 'zh-CN']) {
    const channels = parseLocale(locale, 'channels.json');
    const models = parseLocale(locale, 'models.json');
    assert.ok(channels['channels.dialogs.fields.apiFormat.formats.openai/decisions'], `${locale} channels label missing`);
    assert.ok(models['models.dialogs.association.conditions.formatOptions.openai/decisions'], `${locale} models label missing`);
  }
});

test('keeps the openai/decisions association format listed for model association conditions', () => {
  const dialog = read('features/models/components/models-association-dialog.tsx');
  assert.match(dialog, /requestFormatConditionOptions[\s\S]*'openai\/decisions'/, 'association format options should include openai/decisions');
});

const H = 'openai/responses';
const W = 'openai/responses-ws';
const CODEX_BASE = 'https://chatgpt.com/backend-api/codex#';

test('defaults the Codex WebSocket checkbox on for a new channel', () => {
  assert.deepEqual(getCodexResponsesWebSocketState(undefined), { enabled: true, webSocketOnly: false });
});

test('projects every persisted Codex Responses intent onto the WebSocket checkbox', () => {
  const cases = [
    ['no family entries + HTTPS base -> H+W defaults', { baseURL: CODEX_BASE, endpoints: [] }, { enabled: true, webSocketOnly: false }],
    ['no family entries + legacy WSS base -> W only', { baseURL: 'wss://chatgpt.com/backend-api/codex#', endpoints: null }, { enabled: true, webSocketOnly: true }],
    ['explicit H only', { baseURL: CODEX_BASE, endpoints: [{ apiFormat: H }] }, { enabled: false, webSocketOnly: false }],
    ['legacy explicit HTTP on a WSS base', { baseURL: 'wss://proxy.test/codex#', endpoints: [{ apiFormat: H, transport: 'http' }] }, { enabled: false, webSocketOnly: false }],
    ['legacy H + websocket transport', { baseURL: CODEX_BASE, endpoints: [{ apiFormat: H, transport: 'websocket' }] }, { enabled: true, webSocketOnly: true }],
    ['legacy H inheriting a WSS base', { baseURL: 'wss://proxy.test/codex#', endpoints: [{ apiFormat: H }] }, { enabled: true, webSocketOnly: true }],
    ['legacy H with a distinct WS address', { baseURL: CODEX_BASE, endpoints: [{ apiFormat: H, baseURL: 'wss://other.test/custom#' }] }, { enabled: true, webSocketOnly: true }],
    ['canonical W only', { baseURL: CODEX_BASE, endpoints: [{ apiFormat: W }] }, { enabled: true, webSocketOnly: true }],
    ['explicit H and W', { baseURL: CODEX_BASE, endpoints: [{ apiFormat: H }, { apiFormat: W }] }, { enabled: true, webSocketOnly: false }],
    ['only unrelated entries + HTTPS base', { baseURL: CODEX_BASE, endpoints: [{ apiFormat: 'openai/image_generation' }] }, { enabled: true, webSocketOnly: false }],
  ];
  for (const [name, channel, expected] of cases) {
    assert.deepEqual(getCodexResponsesWebSocketState(channel), expected, name);
  }
});

test('checked Codex WebSocket serializes explicit HTTP and WebSocket Responses endpoints', () => {
  assert.deepEqual(buildCodexResponsesEndpoints(undefined, true), [{ apiFormat: H }, { apiFormat: W }]);
  assert.deepEqual(buildCodexResponsesEndpoints([{ apiFormat: H }], true), [{ apiFormat: H }, { apiFormat: W }]);
});

test('detects when canonicalization replaces a distinct WebSocket override', () => {
  assert.equal(hasCodexResponsesWebSocketOverrideReplacement({
    baseURL: CODEX_BASE,
    endpoints: [
      { apiFormat: H, path: '/backend-api/codex/responses' },
      { apiFormat: W, baseURL: 'https://other.test', path: '/responses' },
    ],
  }), true);
  assert.equal(hasCodexResponsesWebSocketOverrideReplacement({
    baseURL: CODEX_BASE,
    endpoints: [{ apiFormat: H }, { apiFormat: W }],
  }), false);
});

test('unchecked Codex WebSocket serializes explicit HTTP Responses only, never an empty list', () => {
  assert.deepEqual(buildCodexResponsesEndpoints(null, false), [{ apiFormat: H }]);
  assert.deepEqual(buildCodexResponsesEndpoints([{ apiFormat: H }, { apiFormat: W }], false), [{ apiFormat: H }]);
});

test('Codex serialization keeps Images and other unrelated endpoints unchanged', () => {
  const images = { apiFormat: 'openai/image_generation', path: '/images/generations' };
  const edits = { apiFormat: 'openai/image_edit', baseURL: 'https://images.test/v1' };
  assert.deepEqual(buildCodexResponsesEndpoints([images, { apiFormat: H }, edits], true), [images, { apiFormat: H }, { apiFormat: W }, edits]);
  assert.deepEqual(buildCodexResponsesEndpoints([images, edits], false), [images, edits, { apiFormat: H }]);
});

test('Codex serialization upgrades legacy entries to canonical formats without transport', () => {
  assert.deepEqual(buildCodexResponsesEndpoints([{ apiFormat: H, transport: 'websocket', baseURL: 'wss://other.test/custom#' }], true), [
    { apiFormat: H },
    { apiFormat: W },
  ]);
  assert.deepEqual(buildCodexResponsesEndpoints([{ apiFormat: W }], true), [{ apiFormat: H }, { apiFormat: W }]);
  assert.deepEqual(buildCodexResponsesEndpoints([{ apiFormat: H, transport: 'http' }], false), [{ apiFormat: H }]);
});

test('Codex WebSocket endpoint mirrors the HTTP Responses path and HTTP(S) base override only', () => {
  const shared = { path: '/custom/responses', baseURL: 'https://other.test:8443/api' };
  assert.deepEqual(buildCodexResponsesEndpoints([{ apiFormat: H, ...shared }], true), [
    { apiFormat: H, ...shared },
    { apiFormat: W, ...shared },
  ]);
  assert.deepEqual(
    buildCodexResponsesEndpoints([{ apiFormat: H, transport: 'websocket', path: '/ws/responses', baseURL: 'wss://other.test/custom#' }], true),
    [
      { apiFormat: H, path: '/ws/responses' },
      { apiFormat: W, path: '/ws/responses' },
    ]
  );
});

test('derives the official Codex WebSocket URL without /v1 or a fragment', () => {
  assert.equal(getCodexResponsesWebSocketURL(CODEX_BASE), 'wss://chatgpt.com/backend-api/codex/responses');
  assert.equal(getCodexResponsesWebSocketURL(CODEX_BASE, []), 'wss://chatgpt.com/backend-api/codex/responses');
});

test('derives third-party WebSocket URLs preserving host, port and base path with the Responses joining rules', () => {
  const cases = [
    ['https://proxy.example.com:8443/openai', 'wss://proxy.example.com:8443/openai/v1/responses'],
    ['http://127.0.0.1:8080/v1', 'ws://127.0.0.1:8080/v1/responses'],
    ['http://127.0.0.1:8080/v1/', 'ws://127.0.0.1:8080/v1/responses'],
    ['https://gateway.test/base/v1/extra', 'wss://gateway.test/base/v1/extra/responses'],
    ['https://gateway.test/codex/#', 'wss://gateway.test/codex/responses'],
    ['https://gateway.test/v1/responses##', 'wss://gateway.test/v1/responses'],
    ['https://gateway.test/a#frag', 'wss://gateway.test/a'],
  ];
  for (const [baseURL, expected] of cases) {
    assert.equal(getCodexResponsesWebSocketURL(baseURL), expected, baseURL);
  }
});

test('derives the WebSocket URL from the HTTP Responses endpoint overrides, never a legacy WS address', () => {
  assert.equal(
    getCodexResponsesWebSocketURL('https://proxy.test:9443/api#', [{ apiFormat: H, path: '/custom/responses' }]),
    'wss://proxy.test:9443/api/custom/responses'
  );
  assert.equal(
    getCodexResponsesWebSocketURL(CODEX_BASE, [{ apiFormat: H, baseURL: 'http://other.test:8080/v1' }]),
    'ws://other.test:8080/v1/responses'
  );
  assert.equal(
    getCodexResponsesWebSocketURL(CODEX_BASE, [{ apiFormat: H, transport: 'websocket', baseURL: 'wss://other.test/custom#' }]),
    'wss://chatgpt.com/backend-api/codex/responses'
  );
});

test('does not derive a WebSocket URL from a non-HTTP(S) Base URL', () => {
  for (const baseURL of ['wss://proxy.test/v1', 'ws://proxy.test/v1', 'ftp://proxy.test/v1', 'not a url', '', undefined]) {
    assert.equal(getCodexResponsesWebSocketURL(baseURL), undefined, String(baseURL));
  }
});

test('shows legacy WebSocket Base URLs as their HTTP(S) equivalent', () => {
  assert.equal(toHttpBaseURL('wss://chatgpt.com/backend-api/codex#'), CODEX_BASE);
  assert.equal(toHttpBaseURL('ws://127.0.0.1:8080/v1'), 'http://127.0.0.1:8080/v1');
  assert.equal(toHttpBaseURL('WSS://Proxy.test:8443/base'), 'https://Proxy.test:8443/base');
  assert.equal(toHttpBaseURL('https://proxy.test/v1'), 'https://proxy.test/v1');
});

test('translates legacy WebSocket-only Responses model restrictions to the WebSocket format', () => {
  const chat = { model: 'gpt-4o', apiFormats: ['openai/chat_completions'], enabled: false };
  assert.deepEqual(
    translateLegacyCodexModelProtocols({ baseURL: 'wss://chatgpt.com/backend-api/codex#', endpoints: [] }, [
      { model: 'gpt-5', apiFormats: [H], enabled: true },
      chat,
    ]),
    [{ model: 'gpt-5', apiFormats: [W], enabled: true }, chat]
  );
  assert.deepEqual(
    translateLegacyCodexModelProtocols({ baseURL: CODEX_BASE, endpoints: [{ apiFormat: H, transport: 'websocket' }] }, [
      { model: 'gpt-5', apiFormats: [H, W], enabled: true },
    ]),
    [{ model: 'gpt-5', apiFormats: [W], enabled: true }]
  );
});

test('keeps HTTP-resolved and dual Responses model restrictions unchanged', () => {
  const restrictions = [{ model: 'gpt-5', apiFormats: [H], enabled: true }];
  assert.equal(translateLegacyCodexModelProtocols({ baseURL: CODEX_BASE, endpoints: [] }, restrictions), undefined);
  assert.equal(translateLegacyCodexModelProtocols({ baseURL: CODEX_BASE, endpoints: [{ apiFormat: H }] }, restrictions), undefined);
  assert.equal(translateLegacyCodexModelProtocols({ baseURL: CODEX_BASE, endpoints: [{ apiFormat: H }, { apiFormat: W }] }, restrictions), undefined);
  assert.equal(translateLegacyCodexModelProtocols({ baseURL: 'wss://proxy.test/codex#', endpoints: [{ apiFormat: H, transport: 'http' }] }, restrictions), undefined);
  assert.equal(translateLegacyCodexModelProtocols({ baseURL: 'wss://proxy.test/codex#', endpoints: [] }, undefined), undefined);
});

test('keeps the Codex Responses family out of the custom endpoint list, other channels unchanged', () => {
  assert.deepEqual(getConfigurableApiFormatsForChannelType('codex', configurableFormats), ['openai/chat_completions']);
  assert.deepEqual(getConfigurableApiFormatsForChannelType('openai_responses', configurableFormats), ['openai/chat_completions', 'openai/responses']);
  assert.deepEqual(getConfigurableApiFormatsForChannelType('fenno', configurableFormats), ['openai/chat_completions', 'openai/responses']);
});

test('recognizes only the HTTP and WebSocket Responses formats as the Responses family', () => {
  assert.equal(isResponsesFamilyFormat(H), true);
  assert.equal(isResponsesFamilyFormat(W), true);
  for (const format of ['openai/chat_completions', 'openai/image_generation', 'openai/responses_compact', '']) {
    assert.equal(isResponsesFamilyFormat(format), false, format);
  }
});

test('resolves the effective Responses formats a channel exposes for display', () => {
  const images = { apiFormat: 'openai/image_generation' };
  const cases = [
    ['Codex HTTP + WebSocket defaults', { baseURL: CODEX_BASE, defaultEndpoints: [{ apiFormat: H }, { apiFormat: W }, images], endpoints: [] }, [H, W]],
    ['Codex WebSocket-only defaults', { baseURL: 'wss://chatgpt.com/backend-api/codex#', defaultEndpoints: [{ apiFormat: W }, images], endpoints: null }, [W]],
    ['explicit HTTP only', { baseURL: CODEX_BASE, defaultEndpoints: [{ apiFormat: H }, images], endpoints: [{ apiFormat: H }] }, [H]],
    ['explicit WebSocket listed before HTTP', { baseURL: CODEX_BASE, defaultEndpoints: [], endpoints: [{ apiFormat: W }, { apiFormat: H }] }, [H, W]],
    ['raw legacy websocket transport', { baseURL: 'https://api.openai.com/v1', defaultEndpoints: [{ apiFormat: W }], endpoints: [{ apiFormat: H, transport: 'websocket' }] }, [W]],
    ['raw legacy entry inheriting a WSS base', { baseURL: 'wss://api.openai.com/v1#', defaultEndpoints: [], endpoints: [{ apiFormat: H }] }, [W]],
    ['raw legacy explicit HTTP on a WSS base', { baseURL: 'wss://proxy.test/v1#', defaultEndpoints: [], endpoints: [{ apiFormat: H, transport: 'http' }] }, [H]],
    ['backend-resolved HTTP defaults on a WSS base are not re-resolved', { baseURL: 'wss://proxy.test/codex#', defaultEndpoints: [{ apiFormat: H }, images], endpoints: [{ apiFormat: H, transport: 'http' }] }, [H]],
    ['no Responses endpoints', { baseURL: 'https://api.openai.com/v1', defaultEndpoints: [{ apiFormat: 'openai/chat_completions' }], endpoints: [] }, []],
  ];
  for (const [name, channel, expected] of cases) {
    assert.deepEqual(getEffectiveResponsesFormats(channel), expected, name);
  }
});
