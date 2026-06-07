import assert from 'node:assert/strict';
import test from 'node:test';

import { createFirstByteTimeoutServer } from './first-byte-timeout-server.mjs';

function listenOnRandomPort(server) {
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', () => {
      const address = server.address();
      resolve(`http://127.0.0.1:${address.port}`);
    });
  });
}

function closeServer(server) {
  return new Promise((resolve, reject) => {
    server.close((error) => {
      if (error) {
        reject(error);
        return;
      }

      resolve();
    });
  });
}

test('health responds while chat completions never send response headers by default', async (t) => {
  const server = createFirstByteTimeoutServer();
  const baseUrl = await listenOnRandomPort(server);
  t.after(() => closeServer(server));

  const health = await fetch(`${baseUrl}/health`);
  assert.equal(health.status, 200);
  assert.deepEqual(await health.json(), { status: 'ok' });

  const controller = new AbortController();
  try {
    await Promise.race([
      fetch(`${baseUrl}/v1/chat/completions`, {
        method: 'POST',
        headers: { 'content-type': 'application/json' },
        body: JSON.stringify({
          model: 'first-byte-timeout-test',
          messages: [{ role: 'user', content: 'hold the response open' }],
          stream: true,
        }),
        signal: controller.signal,
      }),
      new Promise((_, reject) => {
        setTimeout(() => {
          controller.abort();
          reject(new Error('timed out waiting for response headers'));
        }, 100);
      }),
    ]);

    assert.fail('expected the response headers to stay pending until the client aborts');
  } catch (error) {
    assert.equal(error.message, 'timed out waiting for response headers');
  }
});

test('sse-no-events mode sends SSE headers without stream events', async (t) => {
  const server = createFirstByteTimeoutServer({ holdMode: 'sse-no-events' });
  const baseUrl = await listenOnRandomPort(server);
  t.after(() => closeServer(server));

  const controller = new AbortController();
  let headersTimer;
  const response = await Promise.race([
    fetch(`${baseUrl}/v1/chat/completions`, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({
        model: 'first-byte-timeout-test',
        messages: [{ role: 'user', content: 'hold the response open' }],
        stream: true,
      }),
      signal: controller.signal,
    }),
    new Promise((_, reject) => {
      headersTimer = setTimeout(() => {
        controller.abort();
        reject(new Error('timed out waiting for response headers'));
      }, 100);
    }),
  ]);
  clearTimeout(headersTimer);

  assert.equal(response.status, 200);
  assert.match(response.headers.get('content-type'), /^text\/event-stream\b/);

  const startedAt = Date.now();
  const abortTimer = setTimeout(() => controller.abort(), 250);

  try {
    await response.text();

    assert.fail('expected the response body to stay pending until the client aborts');
  } catch (error) {
    assert.equal(error.name, 'AbortError');
    assert.ok(Date.now() - startedAt >= 200);
  } finally {
    clearTimeout(abortTimer);
  }
});

test('models endpoint returns a dummy OpenAI-compatible model list', async (t) => {
  const server = createFirstByteTimeoutServer();
  const baseUrl = await listenOnRandomPort(server);
  t.after(() => closeServer(server));

  const response = await fetch(`${baseUrl}/v1/models`);
  assert.equal(response.status, 200);

  const body = await response.json();
  assert.equal(body.object, 'list');
  assert.equal(body.data[0].id, 'first-byte-timeout-test');
});
