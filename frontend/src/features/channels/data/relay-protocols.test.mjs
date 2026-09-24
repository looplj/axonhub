import assert from 'node:assert/strict';
import test from 'node:test';
import { RELAY_PROTOCOLS, getChannelRelayProtocols, isRelayProtocol } from './relay-protocols.ts';

test('lists exactly the three relay protocols', () => {
  assert.deepEqual([...RELAY_PROTOCOLS], ['openai/chat_completions', 'openai/responses', 'anthropic/messages']);
});

test('recognizes only relay protocols', () => {
  assert.equal(isRelayProtocol('openai/chat_completions'), true);
  assert.equal(isRelayProtocol('anthropic/messages'), true);
  assert.equal(isRelayProtocol('openai/embeddings'), false);
});

test('merges default endpoints and overrides without duplicates', () => {
  const protocols = getChannelRelayProtocols(
    [{ apiFormat: 'openai/chat_completions' }, { apiFormat: 'openai/embeddings' }],
    [{ apiFormat: 'openai/responses' }, { apiFormat: 'openai/chat_completions' }]
  );

  assert.deepEqual([...protocols].sort(), ['openai/chat_completions', 'openai/responses']);
});

test('ignores non-relay formats and tolerates empty input', () => {
  assert.equal(getChannelRelayProtocols([{ apiFormat: 'gemini/contents' }], null).size, 0);
  assert.equal(getChannelRelayProtocols(undefined, undefined).size, 0);
});
