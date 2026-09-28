import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';
import * as redirects from '../../../lib/auth-redirect.ts';

const code = ts
  .transpileModule(readFileSync(new URL('./auth.ts', import.meta.url), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  })
  .outputText.replace(/^import .*;\n/gm, '');
let moduleIndex = 0;

async function setup(t) {
  const previousWindow = globalThis.window;
  const values = new Map();
  globalThis.window = {
    location: { origin: 'https://axonhub.example', href: 'https://axonhub.example/sign-in' },
    sessionStorage: {
      getItem: (key) => values.get(key) ?? null,
      setItem: (key, value) => values.set(key, value),
      removeItem: (key) => values.delete(key),
    },
  };
  const pushed = [];
  const navigated = [];
  const auth = { setUser() {}, setAccessToken() {} };
  globalThis.authRedirectTest = {
    ...redirects,
    useMutation: (options) => options,
    useAuthStore: (select) => select({ auth }),
    useRouter: () => ({ history: { push: (path) => pushed.push(path) }, navigate: (options) => navigated.push(options) }),
    setTokenToStorage() {},
    toast: { success() {}, error() {} },
    i18n: { language: 'en', t: (key) => key },
    getHiddenNavItems: () => [],
    pickFallbackNavUrl: (path) => path,
  };
  t.after(() => {
    globalThis.window = previousWindow;
    delete globalThis.authRedirectTest;
  });
  const bindings = `const { ${Object.keys(globalThis.authRedirectTest).join(', ')} } = globalThis.authRedirectTest;\n`;
  const module = await import(`data:text/javascript;base64,${Buffer.from(bindings + code).toString('base64')}#${moduleIndex++}`);
  return { ...module, pushed, navigated };
}

const data = { token: 'test-token', user: { isOwner: false, preferLanguage: 'en' } };

test('password login rejects a cross-origin redirect and uses the default destination', async (t) => {
  const auth = await setup(t);
  auth.useSignIn('/\\evil.example').onSuccess(data);
  assert.deepEqual(auth.pushed, []);
  assert.deepEqual(auth.navigated, [{ to: '/project/playground' }]);
});

test('OIDC authorization and exchange preserve the full original destination', async (t) => {
  const auth = await setup(t);
  const redirect = '/project/requests?status=error#details';
  auth.useOIDCAuthorize(redirect).onSuccess({ data: { url: 'https://idp.example/authorize' } });
  assert.equal(window.location.href, 'https://idp.example/authorize');
  auth.useOIDCExchange().onSuccess({ data });
  assert.deepEqual(auth.pushed, [redirect]);
  assert.deepEqual(auth.navigated, []);
  assert.equal(redirects.consumeOIDCRedirect(), undefined);
});

test('OIDC without a return path uses the default destination', async (t) => {
  const auth = await setup(t);
  auth.useOIDCAuthorize().onSuccess({ data: { url: 'https://idp.example/authorize' } });
  auth.useOIDCExchange().onSuccess({ data });
  assert.deepEqual(auth.pushed, []);
  assert.deepEqual(auth.navigated, [{ to: '/project/playground' }]);
});

test('failed OIDC exchange preserves the safe destination for another login attempt', async (t) => {
  const auth = await setup(t);
  redirects.storeOIDCRedirect('/project/requests');
  auth.useOIDCExchange().onError(new Error('exchange failed'));
  assert.deepEqual(auth.navigated, [{ to: '/sign-in', search: { redirect: '/project/requests' } }]);
  assert.equal(redirects.consumeOIDCRedirect(), undefined);
});
