import assert from 'node:assert/strict';
import test from 'node:test';
import {
  getApiFormatsForProvider,
  getAvailableProtocolFormats,
  getChannelTypeForApiFormat,
  getInitialApiFormatForChannel,
  getModelProtocolsForApiFormat,
} from './protocol-options.ts';

const providerConfigs = {
  zenmux: { channelTypes: ['zenmux', 'zenmux_responses'] },
  openai: { channelTypes: ['openai', 'openai_responses'] },
};

const channelConfigs = {
  zenmux: { apiFormat: 'openai/chat_completions' },
  zenmux_responses: { apiFormat: 'openai/responses' },
  zenmux_anthropic: { apiFormat: 'anthropic/messages' },
  zenmux_gemini: { apiFormat: 'gemini/contents' },
  openai: { apiFormat: 'openai/chat_completions' },
  openai_responses: { apiFormat: 'openai/responses' },
};

const configs = { providerConfigs, channelConfigs };

test('includes ZenMux native video in the add-channel provider formats', () => {
  assert.deepEqual(getApiFormatsForProvider('zenmux', configs), ['openai/chat_completions', 'openai/responses', 'zenmux/video']);
});

test('maps ZenMux native video back to the ZenMux channel type', () => {
  assert.equal(getChannelTypeForApiFormat('zenmux', 'zenmux/video', configs), 'zenmux');
});

test('exposes the native video default endpoint to model protocol editing', () => {
  assert.deepEqual(getAvailableProtocolFormats([{ apiFormat: 'zenmux/video' }], []), ['zenmux/video']);
});

test('does not expose ZenMux native video to unrelated providers', () => {
  assert.deepEqual(getApiFormatsForProvider('openai', configs), ['openai/chat_completions', 'openai/responses']);
  assert.equal(getChannelTypeForApiFormat('openai', 'zenmux/video', configs), undefined);
});

test('does not expose ZenMux native video to a provider lacking the ZenMux channel type', () => {
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
