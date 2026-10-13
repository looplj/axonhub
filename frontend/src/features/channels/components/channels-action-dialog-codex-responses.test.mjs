import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Exercise the dialog's Codex form-state recall without loading React, then pin
// how the Codex form wires the WebSocket checkbox into the submitted input.
const source = readFileSync(new URL('./channels-action-dialog.tsx', import.meta.url), 'utf8');
const ast = ts.createSourceFile('channels-action-dialog.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

const CODEX_BASE = 'https://chatgpt.com/backend-api/codex#';

let dialogHelpers;
async function loadDialogHelpers() {
  if (dialogHelpers) return dialogHelpers;

  const helperNames = ['isOfficialCodexChannel', 'getInitialBaseURL', 'getResponsesTransportFromBaseURL', 'getResponsesTransportFromChannel'];
  const helperNodes = ast.statements.filter((node) => ts.isFunctionDeclaration(node) && helperNames.includes(node.name?.text));
  assert.equal(helperNodes.length, helperNames.length, 'Codex form-state helpers must stay in channels-action-dialog.tsx');

  const helperModule = [
    `import { toHttpBaseURL } from '${new URL('../data/protocol-options.ts', import.meta.url).href}';`,
    `const getDefaultBaseURL = (type) => (type === 'codex' ? '${CODEX_BASE}' : '');`,
    `const OPENAI_RESPONSES = 'openai/responses';`,
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

test('edit and duplicate show the canonical HTTPS Base URL for official and auth.json Codex credentials', async () => {
  const { getInitialBaseURL } = await loadDialogHelpers();
  const oauth = JSON.stringify({ access_token: 'synthetic-access', refresh_token: 'synthetic-refresh' });
  const authJSON = JSON.stringify({ tokens: { access_token: 'synthetic-access', refresh_token: 'synthetic-refresh' } });
  assert.equal(getInitialBaseURL({ type: 'codex', baseURL: 'wss://chatgpt.com/backend-api/codex#', credentials: { apiKey: oauth } }), CODEX_BASE);
  assert.equal(getInitialBaseURL({ type: 'codex', baseURL: 'https://api.openai.com/v1', credentials: { apiKey: authJSON } }), CODEX_BASE);
});

test('edit and duplicate show a legacy third-party Codex WebSocket Base URL as HTTP(S)', async () => {
  const { getInitialBaseURL } = await loadDialogHelpers();
  assert.equal(
    getInitialBaseURL({ type: 'codex', baseURL: 'wss://proxy.test:8443/codex#', credentials: { apiKeys: ['sk-synthetic'] } }),
    'https://proxy.test:8443/codex#'
  );
  assert.equal(getInitialBaseURL({ type: 'codex', baseURL: 'http://127.0.0.1:8080/v1', credentials: null }), 'http://127.0.0.1:8080/v1');
});

test('non-Codex channels keep their stored Base URL, including OpenAI Responses WebSocket', async () => {
  const { getInitialBaseURL } = await loadDialogHelpers();
  assert.equal(getInitialBaseURL({ type: 'openai_responses', baseURL: 'wss://api.openai.com/v1#' }), 'wss://api.openai.com/v1#');
});

test('Codex form state never carries a WebSocket transport, other providers keep theirs', async () => {
  const { getResponsesTransportFromChannel } = await loadDialogHelpers();
  assert.equal(getResponsesTransportFromChannel({ type: 'codex', baseURL: 'wss://chatgpt.com/backend-api/codex#', endpoints: [] }), 'http');
  assert.equal(
    getResponsesTransportFromChannel({ type: 'codex', baseURL: CODEX_BASE, endpoints: [{ apiFormat: 'openai/responses', transport: 'websocket' }] }),
    'http'
  );
  assert.equal(getResponsesTransportFromChannel({ type: 'openai_responses', baseURL: 'wss://api.openai.com/v1#', endpoints: [] }), 'websocket');
});

test('Codex API format is a fixed Responses label with a right-aligned, labeled WebSocket checkbox', () => {
  const block = between("{selectedProvider === 'codex' && (", "{selectedProvider === 'claudecode' && (");
  assert.doesNotMatch(block, /SelectDropdown|OPENAI_RESPONSES_WEBSOCKET/, 'Codex must not offer a Responses / Responses WS selector');
  assert.match(block, /getApiFormatLabel\(OPENAI_RESPONSES\)/);
  assert.match(block, /justify-between/);
  assert.match(block, /<label[^>]*>\s*<Checkbox/, 'clicking the label toggles the checkbox');
  assert.match(block, /checked=\{codexResponsesWebSocket\}/);
  assert.match(block, /onCheckedChange=\{\(checked\) => setCodexResponsesWebSocket\(checked === true\)\}/);
  assert.match(block, /data-testid='codex-responses-websocket-checkbox'/);
  assert.match(block, /t\('channels\.dialogs\.fields\.apiFormat\.codexWebSocket\.label'\)/);
  assert.match(block, /data-testid='codex-responses-websocket-url'/);
  assert.match(block, /data-testid='codex-responses-websocket-upgrade-note'/);
});

test('Codex submit serializes the checkbox into canonical endpoints and requires an HTTP(S) Base URL', () => {
  const submit = between('const onSubmit = async', 'const addModel = () =>');
  assert.match(submit, /buildCodexResponsesEndpoints\(initialRow\?\.endpoints, codexResponsesWebSocket\)/);
  assert.match(submit, /isCodexType \? 'http' : responsesTransport/);
  assert.match(submit, /translateLegacyCodexModelProtocols\(initialRow/);
});

test('Codex never submits or derives a WebSocket channel Base URL, and official Base URLs stay locked', () => {
  assert.doesNotMatch(source, /CODEX_RESPONSES_WEBSOCKET_BASE_URL|getResponsesWebSocketBaseURL\('codex'\)|wss:\/\/chatgpt\.com/);
  assert.match(source, /disabled=\{\s*\(isCodexType && authMode !== 'third-party'\)/);
});

test('official and auth.json submit the canonical Base URL, and switching back to third-party restores its URL', () => {
  const submit = between('const onSubmit = async', 'const addModel = () =>');
  assert.match(submit, /isCodexType && \(authMode === 'official' \|\| authMode === 'auth-json'\)/);
  assert.match(submit, /const baseURL = getDefaultBaseURL\(currentType\);/);

  const authTabs = between('value={authMode}', '<TabsList');
  assert.match(authTabs, /if \(isCodexType && authMode === 'third-party'\) \{\s*codexThirdPartyBaseURLRef\.current = form\.getValues\('baseURL'\)/);
  assert.match(authTabs, /getDefaultBaseURL\(selectedType \|\| derivedChannelType\)/);
  assert.match(authTabs, /form\.setValue\('baseURL', codexThirdPartyBaseURLRef\.current\)/);
});

test('localizes the Codex WebSocket checkbox, derived URL and legacy upgrade note', () => {
  for (const locale of ['en', 'zh-CN']) {
    const messages = JSON.parse(readFileSync(new URL(`../../../locales/${locale}/channels.json`, import.meta.url), 'utf8'));
    assert.ok(messages['channels.dialogs.fields.apiFormat.codexWebSocket.label'], `${locale} checkbox label missing`);
    assert.match(messages['channels.dialogs.fields.apiFormat.codexWebSocket.derivedURL'] ?? '', /\{\{url\}\}/, `${locale} derived URL missing`);
    assert.ok(messages['channels.dialogs.fields.apiFormat.codexWebSocket.legacyUpgrade'], `${locale} legacy note missing`);
  }
});
