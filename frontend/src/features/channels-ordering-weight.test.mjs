import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const srcRoot = join(import.meta.dirname, '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

const validationSource = read('features/channels/utils/ordering-weight.ts');
const transpiledValidation = ts.transpileModule(validationSource, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;
const validationModuleUrl = `data:text/javascript;base64,${Buffer.from(transpiledValidation).toString('base64')}`;
const { parseOrderingWeightInput } = await import(validationModuleUrl);

test('bulk ordering weight accepts integers from 0 through 100', () => {
  assert.equal(parseOrderingWeightInput('0', 0, 100), 0);
  assert.equal(parseOrderingWeightInput('100', 0, 100), 100);
  assert.equal(parseOrderingWeightInput('42', 0, 100), 42);
});

test('bulk ordering weight rejects invalid manual input', () => {
  assert.equal(parseOrderingWeightInput('-1', 0, 100), null);
  assert.equal(parseOrderingWeightInput('101', 0, 100), null);
  assert.equal(parseOrderingWeightInput('12.5', 0, 100), null);
  assert.equal(parseOrderingWeightInput('', 0, 100), null);
  assert.equal(parseOrderingWeightInput('Infinity', 0, 100), null);
});

test('bulk ordering input restores invalid values without committing them', () => {
  const component = read('features/channels/components/channels-bulk-ordering-dialog.tsx');
  const handlerStart = component.indexOf('const handleWeightBlur = () => {');
  const handlerEnd = component.indexOf('const getTypeDisplayName', handlerStart);

  assert.ok(handlerStart !== -1 && handlerEnd > handlerStart);

  const handler = component.slice(handlerStart, handlerEnd);
  const invalidBranch = handler.match(/if \(value === null\) \{[\s\S]*?return;[\s\S]*?\}/)?.[0] ?? '';

  assert.match(invalidBranch, /setLocalWeight\(orderingWeight\.toString\(\)\)/);
  assert.match(invalidBranch, /toast\.error/);
  assert.doesNotMatch(invalidBranch, /onWeightChange/);
  assert.match(handler, /if \(value !== orderingWeight\) \{\s*onWeightChange\(channel\.id, value\)/);
  assert.match(component, /inputMode='numeric'[\s\S]*?step=\{1\}/);
  assert.match(component, /if \(e\.key === 'Enter'\) \{\s*e\.currentTarget\.blur\(\)/);
});

test('bulk ordering weight error is localized in English and Chinese', () => {
  const key = 'channels.dialogs.bulkOrdering.errors.invalidWeight';
  const en = JSON.parse(read('locales/en/channels.json'));
  const zh = JSON.parse(read('locales/zh-CN/channels.json'));

  assert.ok(en[key]);
  assert.ok(zh[key]);
});
