import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const dataDir = import.meta.dirname;
const srcRoot = join(dataDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

function loadModule(relativePath) {
  const source = read(relativePath);
  const transpiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  }).outputText;

  return import(`data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`);
}

const { buildCodexCompatibilityExamples } = await loadModule('features/apikeys/data/codex-compatibility-examples.ts');

test('codexCompatibility separates config.toml, auth.json, and shell env with standalone copy payloads', () => {
  const apiKey = 'ah-secret-key-1234';
  const maskedApiKey = 'ah-...1234';
  const examples = buildCodexCompatibilityExamples({
    serverURL: 'http://localhost:8090',
    apiKey,
    maskedApiKey,
  });

  assert.equal(examples.baseUrl, 'http://localhost:8090/codex');

  // 1. config.toml must contain valid TOML assignments only (no JSON object, no shell exports)
  assert.equal(examples.configToml.language, 'toml');
  assert.equal(examples.configToml.real, examples.configToml.display);
  assert.match(examples.configToml.real, /^model = "gpt-6\.1-sol"/m);
  assert.match(examples.configToml.real, /^openai_base_url = "http:\/\/localhost:8090\/codex"/m);
  assert.match(examples.configToml.real, /^\[features\]/m);
  assert.doesNotMatch(examples.configToml.real, /personal_access_token|OPENAI_API_KEY|export\s+CODEX_AUTHAPI_BASE_URL|\{|\}/);

  // 2. auth.json must parse directly as valid JSON (both real and masked display payloads)
  assert.equal(examples.authJson.language, 'json');
  assert.deepEqual(JSON.parse(examples.authJson.real), {
    personal_access_token: apiKey,
    OPENAI_API_KEY: apiKey,
  });
  assert.deepEqual(JSON.parse(examples.authJson.display), {
    personal_access_token: maskedApiKey,
    OPENAI_API_KEY: maskedApiKey,
  });
  assert.ok(!examples.authJson.display.includes(apiKey));

  // 3. shell env snippet contains only the export / Windows instruction
  assert.equal(examples.shellEnv.language, 'bash');
  assert.match(examples.shellEnv.real, /^export CODEX_AUTHAPI_BASE_URL="http:\/\/localhost:8090\/codex"/m);
  assert.doesNotMatch(examples.shellEnv.real, /\[features\]|personal_access_token|OPENAI_API_KEY/);
});

test('apikeys-view-dialog renders an independent MaskedCodeBlock and copy button per codexCompatibility destination', () => {
  const dialogSource = read('features/apikeys/components/apikeys-view-dialog.tsx');
  assert.match(dialogSource, /codexCompatibilityExamples\.configToml/);
  assert.match(dialogSource, /codexCompatibilityExamples\.authJson/);
  assert.match(dialogSource, /codexCompatibilityExamples\.shellEnv/);
  assert.match(dialogSource, /<MaskedCodeBlockCopyButton\s*\/>/);

  for (const locale of ['en', 'zh-CN']) {
    const messages = JSON.parse(read(`locales/${locale}/apikeys.json`));
    assert.ok(messages['apikeys.dialogs.view.codexCompatibility.configTomlTitle']);
    assert.ok(messages['apikeys.dialogs.view.codexCompatibility.authJsonTitle']);
    assert.ok(messages['apikeys.dialogs.view.codexCompatibility.shellEnvTitle']);
  }
});
