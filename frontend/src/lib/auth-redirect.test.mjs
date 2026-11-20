import assert from 'node:assert/strict';
import test from 'node:test';
import {
  consumeOIDCRedirect,
  getSafeRedirect,
  isAuthExchangePath,
  shouldRedirectUnauthorizedToSignIn,
  storeOIDCRedirect,
} from './auth-redirect.ts';

function setup(t, basePath) {
  const values = new Map();
  const previousWindow = globalThis.window;
  const previousDocument = globalThis.document;
  globalThis.document = { querySelector: () => (basePath ? { content: basePath } : null) };
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
    globalThis.document = previousDocument;
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

test('only returns to paths in the current deployment', (t) => {
  setup(t, '/apps/axonhub');
  const redirect = '/apps/axonhub/project/requests?status=error#details';
  assert.equal(getSafeRedirect(redirect), redirect);
  assert.equal(getSafeRedirect('/apps/axonhub'), '/apps/axonhub');
  assert.equal(getSafeRedirect('/apps/axonhub/'), '/apps/axonhub/');

  for (const path of ['/project/requests', '/apps/axonhub2/settings', '/apps/axonhub/../settings', '/apps/axonhub/%73ign-in']) {
    assert.equal(getSafeRedirect(path), undefined, path);
  }

  storeOIDCRedirect(redirect);
  assert.equal(consumeOIDCRedirect(), redirect);
  assert.equal(shouldRedirectUnauthorizedToSignIn('session-token', '/apps/axonhub/oauth/oidc/idp-callback'), false);
  assert.equal(shouldRedirectUnauthorizedToSignIn('session-token', '/apps/axonhub/project/requests'), true);
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

test('a 401 during OIDC exchange does not send the browser to sign-in', () => {
  assert.equal(isAuthExchangePath('/oauth/oidc/idp-callback'), true);
  assert.equal(isAuthExchangePath('/oauth/oidc/idp-callback/'), true);
  assert.equal(isAuthExchangePath('/sign-in'), false);
  assert.equal(shouldRedirectUnauthorizedToSignIn('', '/oauth/oidc/idp-callback'), false);
  assert.equal(shouldRedirectUnauthorizedToSignIn('stale-token', '/oauth/oidc/idp-callback'), false);
  assert.equal(shouldRedirectUnauthorizedToSignIn('', '/'), false);
  assert.equal(shouldRedirectUnauthorizedToSignIn('session-token', '/'), true);
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
