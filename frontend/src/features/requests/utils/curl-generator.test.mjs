import assert from 'node:assert/strict';
import test from 'node:test';
import { getApiPath } from './curl-paths.ts';

test('uses the ZenMux native video endpoint for generated cURL', () => {
  assert.equal(getApiPath('zenmux/video'), '/v1/videos');
});

test('keeps OpenAI video cURL on the existing video endpoint', () => {
  assert.equal(getApiPath('openai/video'), '/v1/videos');
});
