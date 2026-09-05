import assert from 'node:assert/strict';
import test from 'node:test';
import { getVideoLastFrameURL, isVideoRequestFormat } from './video-display.ts';

test('recognizes ZenMux native video requests alongside existing video formats', () => {
  assert.equal(isVideoRequestFormat('zenmux/video'), true);
  assert.equal(isVideoRequestFormat('openai/video'), true);
  assert.equal(isVideoRequestFormat('seedance/video'), true);
  assert.equal(isVideoRequestFormat('openai/chat_completions'), false);
});

test('reads a last-frame URL from the persisted unified video response', () => {
  assert.equal(
    getVideoLastFrameURL({ last_frame_url: 'https://example.invalid/unified-last-frame.jpg' }),
    'https://example.invalid/unified-last-frame.jpg'
  );
});

test('reads a last-frame URL from the native provider response snapshot', () => {
  assert.equal(
    getVideoLastFrameURL({ content: { last_frame_url: 'https://example.invalid/native-last-frame.jpg' } }),
    'https://example.invalid/native-last-frame.jpg'
  );
});

test('reads a last-frame URL from a persisted nested video snapshot', () => {
  assert.equal(
    getVideoLastFrameURL({ video: { last_frame_url: 'https://example.invalid/nested-last-frame.jpg' } }),
    'https://example.invalid/nested-last-frame.jpg'
  );
});
