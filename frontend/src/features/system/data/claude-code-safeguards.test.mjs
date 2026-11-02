import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const output = ts.transpileModule(readFileSync(join(import.meta.dirname, 'claude-code-safeguards.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { parseFallbackModels, safeguardsValidation } = await import(`data:text/javascript;base64,${Buffer.from(output).toString('base64')}`);
const valid = {
  classifierModel: 'primary', fallbackModels: [], policy: '', escalateOnDenial: false,
  timeoutSeconds: 30, attemptTimeoutSeconds: 0, maxTokens: 4096, transcriptCharBudget: 24000,
};

test('fallbacks retain ordering and detect duplicate or primary model entries', () => {
  assert.deepEqual(parseFallbackModels(' first, second\n third\n'), ['first', 'second', 'third']);
  assert.equal(safeguardsValidation({ ...valid, fallbackModels: [' first ', 'first'] }), 'fallbacks');
  assert.equal(safeguardsValidation({ ...valid, fallbackModels: [' primary '] }), 'fallbacks');
  assert.equal(safeguardsValidation({ ...valid, fallbackModels: [''] }), 'fallbacks');
});
test('empty model disables review; blank policy and fractional timeouts remain supported', () => {
  assert.equal(safeguardsValidation({ ...valid, classifierModel: '', policy: ' \n', attemptTimeoutSeconds: 0.5 }), null);
  assert.equal(safeguardsValidation({ ...valid, policy: 'Allow force-pushing feature branches.' }), null);
});
test('invalid numeric inputs cannot be saved', () => {
  for (const override of [
    { timeoutSeconds: 0 }, { timeoutSeconds: 121 }, { timeoutSeconds: NaN },
    { attemptTimeoutSeconds: -1 }, { attemptTimeoutSeconds: Infinity },
    { maxTokens: 1.5 }, { maxTokens: 16385 }, { transcriptCharBudget: 0 },
  ]) assert.equal(safeguardsValidation({ ...valid, ...override }), 'limits');
});
test('Claude Code translations match in both locales', () => {
  const root = join(import.meta.dirname, '../../../locales');
  const en = JSON.parse(readFileSync(join(root, 'en/system.json'), 'utf8'));
  const zh = JSON.parse(readFileSync(join(root, 'zh-CN/system.json'), 'utf8'));
  const keys = Object.keys(en).filter((key) => key.startsWith('system.claudeCode.') || key === 'system.tabs.claudeCode');
  assert.ok(keys.length > 20);
  for (const key of keys) assert.equal(typeof zh[key], 'string', key);
});
