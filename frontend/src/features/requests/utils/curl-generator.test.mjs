import assert from 'node:assert/strict';
import test from 'node:test';
import { getApiPath } from './curl-paths.ts';
import { escapeShellValue } from './curl-shell.ts';

test('uses the ZenMux native video endpoint for generated cURL', () => {
  assert.equal(getApiPath('zenmux/video'), '/v1/videos');
});

test('keeps OpenAI video cURL on the existing video endpoint', () => {
  assert.equal(getApiPath('openai/video'), '/v1/videos');
});

test('uses the Seedance task endpoint for generated cURL', () => {
  assert.equal(getApiPath('seedance/video'), '/api/v3/contents/generations/tasks');
});

test('shell-escapes model-derived URLs in generated cURL', () => {
  assert.equal(
    escapeShellValue("https://example.com/models/a'b:generateContent"),
    "https://example.com/models/a'\\''b:generateContent",
  );
});
