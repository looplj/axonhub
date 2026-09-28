import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(new URL('./auth-session.ts', import.meta.url), 'utf8');
const code = ts
  .transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  })
  .outputText.replace(
    "import { getTokenFromStorage, useAuthStore } from '@/stores/authStore';",
    `const getTokenFromStorage = () => globalThis.sessionToken;
   const useAuthStore = { getState: () => ({ auth: { setAccessToken: value => { globalThis.sessionToken = value; } } }) };`
  );
let moduleIndex = 0;
const makeToken = (seconds, authTime = true) =>
  `header.${Buffer.from(
    JSON.stringify({
      exp: Math.floor(Date.now() / 1000) + seconds,
      ...(authTime ? { auth_time: Math.floor(Date.now() / 1000) - 24 * 86400 } : {}),
    })
  ).toString('base64url')}.signature`;

async function setup(t, token = makeToken(86400)) {
  const previous = { window: globalThis.window, fetch: globalThis.fetch, token: globalThis.sessionToken };
  const events = new Map();
  globalThis.window = { addEventListener: (event, listener) => events.set(event, listener) };
  globalThis.sessionToken = token;
  t.after(() => {
    globalThis.window = previous.window;
    globalThis.fetch = previous.fetch;
    globalThis.sessionToken = previous.token;
  });
  const module = await import(`data:text/javascript;base64,${Buffer.from(code).toString('base64')}#${moduleIndex++}`);
  return { ...module, activate: () => events.get('pointerdown')(), token };
}

test('background requests do not renew a session', async (t) => {
  const session = await setup(t);
  globalThis.fetch = () => {
    assert.fail('background request renewed');
  };
  assert.equal(await session.ensureFreshAccessToken(), session.token);
});

test('active concurrent requests share renewal and persist the new token', async (t) => {
  const session = await setup(t);
  session.activate();
  let resolve;
  let calls = 0;
  globalThis.fetch = (url, options) => {
    assert.equal(url, '/admin/auth/refresh');
    assert.equal(options.headers.Authorization, `Bearer ${session.token}`);
    calls++;
    return new Promise((done) => {
      resolve = done;
    });
  };
  const first = session.ensureFreshAccessToken();
  const second = session.ensureFreshAccessToken();
  resolve(Response.json({ token: 'renewed' }));
  assert.deepEqual(await Promise.all([first, second]), ['renewed', 'renewed']);
  assert.equal(calls, 1);
  assert.equal(globalThis.sessionToken, 'renewed');
});

for (const replacement of ['', 'another-account']) {
  test(`late renewal preserves changed session ${JSON.stringify(replacement)}`, async (t) => {
    const session = await setup(t);
    session.activate();
    let resolve;
    globalThis.fetch = () =>
      new Promise((done) => {
        resolve = done;
      });
    const pending = session.ensureFreshAccessToken();
    globalThis.sessionToken = replacement;
    resolve(Response.json({ token: 'stale-renewal' }));
    assert.equal(await pending, replacement);
    assert.equal(globalThis.sessionToken, replacement);
  });
}

test('network failure preserves the current token', async (t) => {
  const session = await setup(t);
  session.activate();
  globalThis.fetch = async () => {
    throw new Error('offline');
  };
  assert.equal(await session.ensureFreshAccessToken(), session.token);
});

for (const stage of ['fetch', 'body']) {
  test(`refresh timeout preserves the token and allows retry when ${stage} stalls`, async (t) => {
    // Given
    const session = await setup(t);
    t.mock.timers.enable({ apis: ['setTimeout'] });
    session.activate();
    let started;
    const stageStarted = new Promise((resolve) => {
      started = resolve;
    });
    let signal;
    globalThis.fetch = async (_url, options) => {
      signal = options.signal;
      if (!signal) {
        started();
        throw new Error('missing timeout signal');
      }
      const stalled = () =>
        new Promise((_resolve, reject) => {
          options.signal.addEventListener('abort', () => reject(options.signal.reason), { once: true });
          started();
        });
      return stage === 'fetch' ? stalled() : { ok: true, status: 200, json: stalled };
    };

    // When
    const pending = session.ensureFreshAccessToken();
    await stageStarted;
    assert.ok(signal instanceof AbortSignal);
    t.mock.timers.tick(5_000);

    // Then
    assert.equal(await pending, session.token);
    globalThis.fetch = async () => Response.json({ token: 'renewed-after-timeout' });
    assert.equal(await session.ensureFreshAccessToken(), 'renewed-after-timeout');
  });
}

for (const [name, token] of [
  ['legacy', makeToken(86400, false)],
  ['expired', makeToken(-1)],
  ['outside renewal window', makeToken(8 * 86400)],
]) {
  test(`${name} token is not renewed`, async (t) => {
    const session = await setup(t, token);
    session.activate();
    globalThis.fetch = () => {
      assert.fail('unexpected renewal');
    };
    assert.equal(await session.ensureFreshAccessToken(), token);
  });
}
