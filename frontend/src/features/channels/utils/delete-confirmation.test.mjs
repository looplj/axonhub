import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Load the real delete-confirmation helpers without pulling in React.
const source = readFileSync(new URL('./delete-confirmation.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
const { generateConfirmationCode, isConfirmationCodeMatched, CONFIRMATION_CODE_LENGTH } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`
);

test('generated code is always exactly 4 digits', () => {
  for (let i = 0; i < 500; i++) {
    const code = generateConfirmationCode();
    assert.equal(code.length, CONFIRMATION_CODE_LENGTH, `code ${code} is not 4 digits`);
    assert.match(code, /^[1-9]\d{3}$/, `code ${code} must be 1000-9999`);
  }
});

test('codes span the full range and are not constant', () => {
  const seen = new Set();
  // Deterministic inputs cover both bounds and the interior.
  assert.equal(generateConfirmationCode(() => 0), '1000');
  assert.equal(generateConfirmationCode(() => 0.999999), '9999');

  for (let i = 0; i < 200; i++) {
    seen.add(generateConfirmationCode());
  }
  assert.ok(seen.size > 50, `expected varied codes, saw ${seen.size} distinct`);
});

test('a fresh code is generated each time the dialog opens', () => {
  // The dialog regenerates on every open; two draws should not be identical
  // often. Assert at least one difference across a handful of draws.
  const draws = Array.from({ length: 10 }, () => generateConfirmationCode());
  assert.ok(new Set(draws).size > 1, 'regeneration must not return a constant code');
});

test('matching requires an exact 4-digit match', () => {
  assert.equal(isConfirmationCodeMatched('4321', '4321'), true);
  assert.equal(isConfirmationCodeMatched(' 4321 ', '4321'), true, 'surrounding whitespace is tolerated');
  assert.equal(isConfirmationCodeMatched('432', '4321'), false);
  assert.equal(isConfirmationCodeMatched('43210', '4321'), false);
  assert.equal(isConfirmationCodeMatched('4322', '4321'), false);
  assert.equal(isConfirmationCodeMatched('', '4321'), false);
});

test('an empty generated code never matches', () => {
  assert.equal(isConfirmationCodeMatched('', ''), false, 'dialog not yet opened must not allow delete');
  assert.equal(isConfirmationCodeMatched('1000', ''), false);
});
