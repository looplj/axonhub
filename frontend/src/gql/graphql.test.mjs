import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const sourcePath = join(import.meta.dirname, 'graphql.ts');
const source = readFileSync(sourcePath, 'utf8');
const transpiled = ts
  .transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2023,
    },
  })
  .outputText.replaceAll("import { toast } from 'sonner';", 'const toast = { error() {} };')
  .replaceAll("import { ensureFreshAccessToken } from '@/lib/auth-session';", 'const ensureFreshAccessToken = async () => "";')
  .replaceAll(
    "import { getTokenFromStorage, removeTokenFromStorage } from '@/stores/authStore';",
    'const getTokenFromStorage = () => ""; const removeTokenFromStorage = () => {};'
  )
  .replace("'@/lib/app-path'", JSON.stringify(new URL('../lib/app-path.ts', import.meta.url).href))
  .replace("'@/lib/auth-redirect'", JSON.stringify(new URL('../lib/auth-redirect.ts', import.meta.url).href))
  .replaceAll("import i18n from '@/lib/i18n';", 'const i18n = { t: (key) => key };');

const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { graphqlRequest, isUnauthorizedGraphQLError } = await import(moduleUrl);

test('sends GraphQL requests to the current deployment', async (t) => {
  const previousDocument = globalThis.document;
  const previousFetch = globalThis.fetch;
  globalThis.document = { querySelector: () => ({ content: '/apps/axonhub' }) };
  globalThis.fetch = async (url) => {
    assert.equal(url, '/apps/axonhub/admin/graphql');
    return Response.json({ data: { version: 'test' } });
  };
  t.after(() => {
    globalThis.document = previousDocument;
    globalThis.fetch = previousFetch;
  });

  const { graphqlRequest: request } = await import(`${moduleUrl}#prefixed`);
  assert.deepEqual(await request('query Version { version }'), { version: 'test' });
});

test('does not classify upstream unauthorized provider failures as login expiration', () => {
  const error = {
    message: 'model fetch returned error: failed to fetch models: GET - https://provider.example/v1/models with status 401 Unauthorized',
    path: ['syncChannelModels'],
  };

  assert.equal(isUnauthorizedGraphQLError(error), false);
});

test('classifies explicit GraphQL authentication codes as login expiration', () => {
  assert.equal(isUnauthorizedGraphQLError({ extensions: { code: 'UNAUTHENTICATED' } }), true);
});

test('a 401 without a session does not leave the OIDC callback', async (t) => {
  const redirects = [];
  const previousWindow = globalThis.window;
  const previousFetch = globalThis.fetch;
  globalThis.window = {
    location: {
      pathname: '/oauth/oidc/idp-callback',
      search: '?code=abc',
      hash: '',
      set href(value) {
        redirects.push(value);
      },
    },
  };
  globalThis.fetch = async () =>
    new Response(JSON.stringify({ error: { type: 'Unauthorized', message: 'API key is required' } }), {
      status: 401,
      headers: { 'content-type': 'application/json' },
    });
  t.after(() => {
    globalThis.window = previousWindow;
    globalThis.fetch = previousFetch;
  });

  await assert.rejects(
    () => graphqlRequest('query PluginUIEntries { pluginUIEntries { pages { key } } }'),
    (error) => {
      assert.equal(error instanceof Error && error.message, 'Unauthorized');
      return true;
    }
  );
  assert.deepEqual(redirects, []);
});
