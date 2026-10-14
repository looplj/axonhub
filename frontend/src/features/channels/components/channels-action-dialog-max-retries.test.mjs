import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Pin the channel-level "max single channel retries" override: it must reach the
// dialog helpers, the GraphQL selection, and the submitted settings patch.
const dialogSource = readFileSync(new URL('./channels-action-dialog.tsx', import.meta.url), 'utf8');
const dialogAst = ts.createSourceFile('channels-action-dialog.tsx', dialogSource, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

const helperNames = ['formatMaxSingleChannelRetries', 'parseMaxSingleChannelRetriesInput'];

let helpers;
async function loadHelpers() {
  if (helpers) return helpers;

  const nodes = dialogAst.statements.filter((node) => ts.isFunctionDeclaration(node) && helperNames.includes(node.name?.text));
  assert.equal(nodes.length, helperNames.length, 'the max-retries helpers must stay in channels-action-dialog.tsx');

  const module = nodes.map((node) => `export ${node.getText(dialogAst).replace(/^export\s+/, '')}`).join('\n');
  const { outputText } = ts.transpileModule(module, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  });
  helpers = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
  return helpers;
}

test('formatMaxSingleChannelRetries renders empty for "inherit" and the number otherwise', async () => {
  const { formatMaxSingleChannelRetries } = await loadHelpers();

  assert.equal(formatMaxSingleChannelRetries(null), '', 'null must render as empty so the placeholder shows');
  assert.equal(formatMaxSingleChannelRetries(undefined), '', 'undefined must render as empty');
  assert.equal(formatMaxSingleChannelRetries(0), '0', 'a zero override is a real value, not inherit');
  assert.equal(formatMaxSingleChannelRetries(7), '7');
});

test('parseMaxSingleChannelRetriesInput treats empty as inherit and rejects non-integers', async () => {
  const { parseMaxSingleChannelRetriesInput } = await loadHelpers();

  assert.deepEqual(parseMaxSingleChannelRetriesInput(''), { valid: true, value: null }, 'empty input means inherit');
  assert.deepEqual(parseMaxSingleChannelRetriesInput('   '), { valid: true, value: null }, 'whitespace means inherit');
  assert.deepEqual(parseMaxSingleChannelRetriesInput('0'), { valid: true, value: 0 }, '0 is a valid override');
  assert.deepEqual(parseMaxSingleChannelRetriesInput('12'), { valid: true, value: 12 });
  assert.equal(parseMaxSingleChannelRetriesInput('-1').valid, false, 'negative numbers are rejected');
  assert.equal(parseMaxSingleChannelRetriesInput('1.5').valid, false, 'fractions are rejected');
  assert.equal(parseMaxSingleChannelRetriesInput('abc').valid, false, 'non-numeric input is rejected');
});

test('the dialog wires the override into state, reset, and both settings patches', () => {
  assert.match(dialogSource, /useState\(\(\) =>\s*\n?\s*formatMaxSingleChannelRetries\(initialRow\?\.settings\?\.maxSingleChannelRetries\)/, 'initial state must read the channel override');
  assert.match(dialogSource, /setMaxSingleChannelRetriesText\(formatMaxSingleChannelRetries\(initialRow\?\.settings\?\.maxSingleChannelRetries\)\)/, 'reset must restore the override');
  assert.match(dialogSource, /maxSingleChannelRetries,\s*\n\s*apiKeyStrategy,/, 'the create settings patch must carry the override');
  assert.match(dialogSource, /maxSingleChannelRetries,\s*\n\s*\.\.\.quotaRoutingModeSettingsPatch/, 'the update settings patch must carry the override');
});

test('the override input renders above the retryable status codes field with an inherit placeholder', () => {
  const overrideIndex = dialogSource.indexOf("channels.dialogs.maxSingleChannelRetries.label");
  const statusCodesIndex = dialogSource.indexOf("channels.dialogs.retryableStatusCodes.label");

  assert.notEqual(overrideIndex, -1, 'the override field must exist');
  assert.notEqual(statusCodesIndex, -1, 'the retryable status codes field must exist');
  assert.ok(overrideIndex < statusCodesIndex, 'the override field must render above the retryable status codes field');
  assert.match(dialogSource, /channels\.dialogs\.maxSingleChannelRetries\.placeholder/);
});
