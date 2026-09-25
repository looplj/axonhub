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

const source = read('features/models/data/reasoning-efforts.ts');
const transpiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { REASONING_EFFORTS, deriveReasoningEfforts } = await import(moduleUrl);

const catalog = JSON.parse(read('features/models/data/providers.json'));

function catalogOptions(modelID) {
  const match = Object.values(catalog.providers)
    .flatMap((provider) => provider.models ?? [])
    .find((model) => model.id === modelID);

  assert.ok(match, `catalog should contain ${modelID}`);
  return match.reasoning_options;
}

test('reasoning efforts use the unified AxonHub vocabulary', () => {
  assert.deepEqual([...REASONING_EFFORTS], ['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max']);
});

test('effort options map to the levels they enumerate, in canonical order', () => {
  const cases = [
    { modelID: 'grok-4.6', expected: ['low', 'medium', 'high', 'xhigh'] },
    { modelID: 'deepseek-v4-flash', expected: ['low', 'high', 'max'] },
    { modelID: 'glm-5.3-flash', expected: ['low', 'high', 'max'] },
    { modelID: 'gpt-5.6-sol', expected: ['none', 'low', 'medium', 'high', 'xhigh', 'max'] },
    { modelID: 'gpt-6-astra', expected: ['low', 'medium', 'high', 'xhigh', 'max'] },
    { modelID: 'qwen3.8-flash', expected: ['low', 'medium', 'xhigh'] },
  ];

  for (const { modelID, expected } of cases) {
    assert.deepEqual(deriveReasoningEfforts(catalogOptions(modelID)), expected, `efforts mismatch for ${modelID}`);
  }
});

test('catalog entries without a level enumeration stay unknown', () => {
  // Toggle and budget options only say the upstream can switch thinking on and
  // off; they do not name the accepted levels.
  for (const modelID of ['mimo-v2.6-flash', 'mimo-v2.6-pro']) {
    assert.equal(deriveReasoningEfforts(catalogOptions(modelID)), undefined, `efforts mismatch for ${modelID}`);
  }

  assert.equal(deriveReasoningEfforts([{ type: 'budget_tokens', min: 1024 }]), undefined);
  assert.equal(deriveReasoningEfforts([{ type: 'toggle' }]), undefined);
  assert.equal(deriveReasoningEfforts(undefined), undefined);
  assert.equal(deriveReasoningEfforts([]), undefined);

  // An effort option whose values AxonHub does not know carries no usable metadata.
  assert.equal(deriveReasoningEfforts([{ type: 'effort', values: ['bogus'] }]), undefined);
  assert.equal(deriveReasoningEfforts([{ type: 'effort', values: [] }]), undefined);
});
