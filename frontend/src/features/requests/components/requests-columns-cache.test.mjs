import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Execute the tokens column's cache-rate display instead of matching its source
// text, so a wrong rate, a wrong rounding rule, or a wrong visibility rule
// fails the test rather than passing silently.
const source = readFileSync(new URL('./requests-columns.tsx', import.meta.url), 'utf8');
const ast = ts.createSourceFile('requests-columns.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

const helperNode = ast.statements.find(
  (node) => ts.isFunctionDeclaration(node) && node.name?.text === 'tokensColumnCacheDisplay'
);
assert.ok(helperNode, 'tokensColumnCacheDisplay must stay a top-level function in requests-columns.tsx');

const { outputText } = ts.transpileModule(`export ${helperNode.getText(ast).replace(/^export\s+/, '')}`, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
const { tokensColumnCacheDisplay } = await import(
  `data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`
);

test('the tokens cell renders the rate this helper returns', () => {
  assert.match(
    source,
    /tokensColumnCacheDisplay\(promptTokens, cachedTokens, reasoningTokens\)/,
    'the cell must consume the helper instead of recomputing the rate inline'
  );
});

test('a cache read shows the cached share of the prompt', () => {
  const display = tokensColumnCacheDisplay(1000, 250, 0);

  assert.equal(display.cacheRate, '25.0');
  assert.equal(display.showCacheRate, true);
  assert.equal(display.showReasoning, false);
  assert.equal(display.showSharedRow, true);
});

test('no cache read hides the rate even when reasoning tokens exist', () => {
  const display = tokensColumnCacheDisplay(1000, 0, 40);

  assert.equal(display.showCacheRate, false);
  assert.equal(display.showReasoning, true);
  assert.equal(display.showSharedRow, true);
});

test('cache hits and reasoning tokens share the row', () => {
  const display = tokensColumnCacheDisplay(1000, 999, 7);

  assert.equal(display.cacheRate, '99.9');
  assert.equal(display.showCacheRate, true);
  assert.equal(display.showReasoning, true);
  assert.equal(display.showSharedRow, true);
});

test('zero prompt tokens report a zero rate instead of dividing by zero', () => {
  const display = tokensColumnCacheDisplay(0, 120, 0);

  assert.equal(display.cacheRate, '0.0');
  assert.equal(display.showCacheRate, true);
});

test('neither reasoning nor cache tokens keep the shared row hidden', () => {
  const display = tokensColumnCacheDisplay(500, 0, 0);

  assert.equal(display.showSharedRow, false);
  assert.equal(display.showReasoning, false);
  assert.equal(display.showCacheRate, false);
});