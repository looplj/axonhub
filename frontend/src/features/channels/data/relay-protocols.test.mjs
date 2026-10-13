import assert from 'node:assert/strict';
import test from 'node:test';
import { RELAY_PROTOCOLS, getChannelRelayProtocols, isRelayProtocol } from './relay-protocols.ts';

test('lists the four relay protocols', () => {
  assert.deepEqual([...RELAY_PROTOCOLS], [
    'openai/chat_completions',
    'openai/responses',
    'anthropic/messages',
    'gemini/contents',
  ]);
});

test('recognizes only relay protocols', () => {
  assert.equal(isRelayProtocol('openai/chat_completions'), true);
  assert.equal(isRelayProtocol('anthropic/messages'), true);
  assert.equal(isRelayProtocol('gemini/contents'), true);
  assert.equal(isRelayProtocol('openai/embeddings'), false);
  assert.equal(isRelayProtocol('gemini/embeddings'), false);
});

test('merges default endpoints and overrides without duplicates', () => {
  const protocols = getChannelRelayProtocols(
    [{ apiFormat: 'openai/chat_completions' }, { apiFormat: 'openai/embeddings' }],
    [{ apiFormat: 'openai/responses' }, { apiFormat: 'openai/chat_completions' }]
  );

  assert.deepEqual([...protocols].sort(), ['openai/chat_completions', 'openai/responses']);
});

test('recognizes Gemini contents and ignores embedding formats', () => {
  assert.deepEqual(
    [...getChannelRelayProtocols([{ apiFormat: 'gemini/contents' }, { apiFormat: 'gemini/embeddings' }], null)],
    ['gemini/contents']
  );
  assert.equal(getChannelRelayProtocols(undefined, undefined).size, 0);
});

test('counts WebSocket Responses endpoints as the Responses relay protocol', () => {
  // Codex WebSocket-only defaults and canonical WebSocket overrides still relay Responses.
  assert.deepEqual(
    [...getChannelRelayProtocols([{ apiFormat: 'openai/responses-ws' }, { apiFormat: 'openai/image_generation' }], [])],
    ['openai/responses']
  );
  assert.deepEqual([...getChannelRelayProtocols(null, [{ apiFormat: 'openai/responses-ws' }])], ['openai/responses']);
  assert.deepEqual(
    [...getChannelRelayProtocols([{ apiFormat: 'openai/responses' }], [{ apiFormat: 'openai/responses-ws' }])],
    ['openai/responses']
  );
});

test('keeps the WebSocket format out of the relay protocol list used by detection', () => {
  assert.equal(isRelayProtocol('openai/responses-ws'), false);
});
