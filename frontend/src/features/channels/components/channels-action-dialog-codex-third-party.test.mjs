import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Regression coverage for the Codex third-party credential modes: a copied or
// edited OAuth/auth.json row switched to third-party must not keep leaking the
// inherited OAuth JSON through `credentials.apiKey`, while official and
// auth.json keeps it. Each case exercises the real helper the dialog calls;
// the exact call sites are pinned against the source below.
const source = readFileSync(new URL('./channels-action-dialog.tsx', import.meta.url), 'utf8');
const schemaSource = readFileSync(new URL('../data/schema.ts', import.meta.url), 'utf8');
const ast = ts.createSourceFile('channels-action-dialog.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

const CODEX_BASE = 'https://chatgpt.com/backend-api/codex#';
const OAUTH_JSON = JSON.stringify({ access_token: 'synthetic-access', refresh_token: 'synthetic-refresh' });
const AUTH_JSON = JSON.stringify({ tokens: { access_token: 'synthetic-access', refresh_token: 'synthetic-refresh' } });

let dialogHelpers;
async function loadDialogHelpers() {
  if (dialogHelpers) return dialogHelpers;

  const helperNames = ['isOfficialCodexChannel', 'getCodexThirdPartySubmitCredentials', 'getFetchModelsApiKey', 'getCodexThirdPartyCredentialError', 'parseOauthToken'];
  const helperNodes = ast.statements.filter((node) => ts.isFunctionDeclaration(node) && helperNames.includes(node.name?.text));
  assert.equal(helperNodes.length, helperNames.length, 'Codex credential-mode helpers must stay in channels-action-dialog.tsx');

  const helperModule = [
    `const getDefaultBaseURL = (type) => (type === 'codex' ? '${CODEX_BASE}' : '');`,
    ...helperNodes.map((node) => `export ${node.getText(ast).replace(/^export\s+/, '')}`),
  ].join('\n');
  const { outputText } = ts.transpileModule(helperModule, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  });
  dialogHelpers = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
  return dialogHelpers;
}

function between(startMarker, endMarker) {
  const start = source.indexOf(startMarker);
  assert.notEqual(start, -1, `missing ${startMarker}`);
  const end = source.indexOf(endMarker, start + startMarker.length);
  assert.notEqual(end, -1, `missing ${endMarker}`);
  return source.slice(start, end);
}

test('a copied OAuth row switched to third-party submits no OAuth JSON and keeps the custom Base URL', async () => {
  const { getCodexThirdPartySubmitCredentials } = await loadDialogHelpers();
  const credentials = { apiKey: OAUTH_JSON, apiKeys: ['sk-synthetic-third-party'] };

  const submitted = getCodexThirdPartySubmitCredentials(credentials, { type: 'codex', authMode: 'third-party' });
  assert.equal(submitted.apiKey, undefined, 'the inherited OAuth JSON must be dropped in third-party mode');
  assert.deepEqual(submitted.apiKeys, ['sk-synthetic-third-party'], 'the plain API key must be kept');

  // The custom URL survives submit: only official/auth.json override the Base URL.
  const submit = between('const onSubmit = async', 'const addModel = () =>');
  assert.match(submit, /isCodexType && \(authMode === 'official' \|\| authMode === 'auth-json'\)/);
  assert.match(submit, /getCodexThirdPartySubmitCredentials\(/);

  // The submit payload itself must be the sanitized credential.
  assert.match(submit, /const valuesForSubmit = isEdit/);
  assert.match(submit, /credentials:\s*credentials\s*\n\s*\? getCodexThirdPartySubmitCredentials\(credentials/);
  assert.match(submit, /type: valuesForSubmit\.type,/);
});

test('a third-party Codex channel with a synthetic plain key fetches models with that key, not the inherited OAuth JSON', async () => {
  const { getFetchModelsApiKey } = await loadDialogHelpers();

  const plainKey = getFetchModelsApiKey({ type: 'codex', authMode: 'third-party', oauthApiKey: OAUTH_JSON, apiKeys: ['sk-synthetic-third-party'] });
  assert.equal(plainKey, 'sk-synthetic-third-party', 'third-party fetch must send the plain API key');

  const authJSONThirdParty = getFetchModelsApiKey({ type: 'codex', authMode: 'third-party', oauthApiKey: AUTH_JSON, apiKeys: ['sk-synthetic-third-party'] });
  assert.equal(authJSONThirdParty, 'sk-synthetic-third-party', 'auth.json must not leak when the row is switched to third-party');

  const fetch = between('const handleFetchModels = useCallback', 'const handleSyncNow = useCallback');
  assert.match(fetch, /getFetchModelsApiKey\(\{/, 'fetch-models must go through the shared credential choice');
});

test('official and auth.json Codex channels still submit and fetch their OAuth credential with the preset Base URL', async () => {
  const { getCodexThirdPartySubmitCredentials, getFetchModelsApiKey } = await loadDialogHelpers();
  const credentials = { apiKey: OAUTH_JSON, apiKeys: [] };

  for (const authMode of ['official', 'auth-json']) {
    const submitted = getCodexThirdPartySubmitCredentials(credentials, { type: 'codex', authMode });
    assert.equal(submitted, credentials, `${authMode} must submit the OAuth credential unchanged`);
    assert.equal(getFetchModelsApiKey({ type: 'codex', authMode, oauthApiKey: OAUTH_JSON, apiKeys: [] }), OAUTH_JSON, `${authMode} must fetch with the OAuth credential`);
  }

  // auth.json mode keeps sending the converted OAuth JSON and the preset URL.
  const { isOfficialCodexChannel } = await loadDialogHelpers();
  assert.equal(isOfficialCodexChannel({ credentials: { apiKey: AUTH_JSON } }), true, 'auth.json must stay recognized as an official credential');
  const submit = between('const onSubmit = async', 'const addModel = () =>');
  assert.match(submit, /const baseURL = getDefaultBaseURL\(currentType\);/);
});

test('non-Codex providers keep their OAuth credential preference when fetching models', async () => {
  const { getFetchModelsApiKey } = await loadDialogHelpers();
  assert.equal(
    getFetchModelsApiKey({ type: 'github_copilot', authMode: 'third-party', oauthApiKey: OAUTH_JSON, apiKeys: ['sk-synthetic'] }),
    OAUTH_JSON
  );
  assert.equal(getFetchModelsApiKey({ type: 'codex', authMode: 'third-party', oauthApiKey: '', apiKeys: ['  sk-synthetic  '] }), 'sk-synthetic');
});

test('an empty or OAuth-JSON third-party credential is rejected instead of saved', async () => {
  const { getCodexThirdPartyCredentialError } = await loadDialogHelpers();

  const requiredKey = 'channels.dialogs.oauth.errors.codexThirdPartyApiKeyRequired';
  const invalidKey = 'channels.dialogs.oauth.errors.codexThirdPartyApiKeyInvalid';

  assert.equal(
    getCodexThirdPartyCredentialError({ type: 'codex', authMode: 'third-party', apiKeys: [] }),
    requiredKey,
    'an empty third-party credential must be rejected'
  );
  assert.equal(
    getCodexThirdPartyCredentialError({ type: 'codex', authMode: 'third-party', apiKeys: ['   ', ''] }),
    requiredKey,
    'blank keys must not count as a credential'
  );
  assert.equal(
    getCodexThirdPartyCredentialError({ type: 'codex', authMode: 'third-party', apiKeys: [OAUTH_JSON] }),
    invalidKey,
    'the inherited OAuth JSON must not be pasted as a plain key'
  );
  assert.equal(
    getCodexThirdPartyCredentialError({ type: 'codex', authMode: 'third-party', apiKeys: ['sk-synthetic-third-party'] }),
    undefined,
    'a plain key must pass validation'
  );
  assert.equal(
    getCodexThirdPartyCredentialError({ type: 'codex', authMode: 'official', apiKeys: [] }),
    undefined,
    'official mode must not be constrained by the third-party rule'
  );

  // The submit path must consult the guard before building the payload.
  const submit = between('const onSubmit = async', 'const addModel = () =>');
  assert.match(submit, /getCodexThirdPartyCredentialError\(\{/);
  assert.match(submit, /form\.setError\('credentials\.apiKeys'/);

  for (const locale of ['en', 'zh-CN']) {
    const messages = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/channels.json`, import.meta.url), 'utf8'));
    assert.ok(messages[requiredKey], `${locale} required-key error missing`);
    assert.ok(messages[invalidKey], `${locale} invalid-key error missing`);
  }
});

test('schema does not validate inherited Codex OAuth JSON when a third-party key is present', () => {
  assert.match(
    schemaSource,
    /\(data\.type === 'codex' && !hasApiKeys\)/,
    'Codex OAuth JSON validation must be skipped when third-party apiKeys are present'
  );
});
