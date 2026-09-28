import assert from 'node:assert/strict';
import test from 'node:test';
import { consumeOIDCRedirect, getSafeRedirect, storeOIDCRedirect } from './auth-redirect.ts';

function setup(t) {
  const values = new Map();
  const previousWindow = globalThis.window;
  globalThis.window = {
    location: { origin: 'https://axonhub.example' },
    sessionStorage: {
      getItem: (key) => values.get(key) ?? null,
      setItem: (key, value) => values.set(key, value),
      removeItem: (key) => values.delete(key),
    },
  };
  t.after(() => {
    globalThis.window = previousWindow;
  });
  return values;
}

test('preserves local return paths, queries and fragments after normalization', (t) => {
  setup(t);
  assert.equal(getSafeRedirect('/project/requests?status=error#details'), '/project/requests?status=error#details');
  assert.equal(getSafeRedirect('/project/../settings'), '/settings');
  assert.equal(getSafeRedirect('/'), '/');
});

test('rejects external, malformed and normalized sign-in destinations', (t) => {
  setup(t);
  for (const redirect of [
    undefined,
    '',
    'https://evil.example',
    '//evil.example',
    '/\\evil.example',
    '/\t/evil.example',
    '/.//evil.example',
    '/sign-in?redirect=/',
    '/project/../sign-in',
    '/%73ign-in',
    '/%2e/sign-in',
    '/%invalid',
  ]) {
    assert.equal(getSafeRedirect(redirect), undefined, String(redirect));
  }
});

test('OIDC retains the return path across navigation and consumes it only once', (t) => {
  setup(t);
  storeOIDCRedirect('/project/requests?status=error#details');
  window.location.pathname = '/oauth/oidc/idp-callback';
  assert.equal(consumeOIDCRedirect(), '/project/requests?status=error#details');
  assert.equal(consumeOIDCRedirect(), undefined);
});

test('a new OIDC login without a valid return path clears an earlier destination', (t) => {
  setup(t);
  for (const redirect of [undefined, '/\\evil.example', '/sign-in']) {
    storeOIDCRedirect('/project/requests');
    storeOIDCRedirect(redirect);
    assert.equal(consumeOIDCRedirect(), undefined);
  }
});

test('OIDC validates stored destinations again before consuming them', (t) => {
  const values = setup(t);
  storeOIDCRedirect('/project/requests');
  const [key] = values.keys();
  values.set(key, '/\\evil.example');
  assert.equal(consumeOIDCRedirect(), undefined);
  assert.equal(values.size, 0);
});

test('blocked session storage falls back without failing authentication', (t) => {
  setup(t);
  Object.defineProperty(window, 'sessionStorage', {
    get() {
      throw new Error('storage blocked');
    },
  });
  assert.doesNotThrow(() => storeOIDCRedirect('/project/requests'));
  assert.equal(consumeOIDCRedirect(), undefined);
});
