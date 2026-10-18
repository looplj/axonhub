import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(new URL('./association-reorder.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
const { computeAssociationReorder } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);

// Rows are displayed sorted by priority, so `id` order == visual order.
const rows = [
  { id: 'a', index: 0 },
  { id: 'b', index: 1 },
  { id: 'c', index: 2 },
];

test('dragging the last row to the top rewrites priority to 0,1,2', () => {
  const result = computeAssociationReorder(rows, 'c', 'a');
  assert.deepEqual(result, [
    { index: 2, priority: 0 },
    { index: 0, priority: 1 },
    { index: 1, priority: 2 },
  ]);
});

test('dragging the first row to the bottom assigns it the highest priority', () => {
  const result = computeAssociationReorder(rows, 'a', 'c');
  assert.deepEqual(result, [
    { index: 1, priority: 0 },
    { index: 2, priority: 1 },
    { index: 0, priority: 2 },
  ]);
});

test('every position receives a dense 0-based priority', () => {
  const result = computeAssociationReorder(rows, 'a', 'b');
  assert.equal(result.length, 3);
  assert.deepEqual(
    result.map((entry) => entry.priority).sort((x, y) => x - y),
    [0, 1, 2]
  );
  // The two swapped rows exchanged priorities; the untouched row kept index 2.
  assert.deepEqual(
    result.find((entry) => entry.index === 2),
    { index: 2, priority: 2 }
  );
});

test('dropping onto itself is a no-op', () => {
  assert.equal(computeAssociationReorder(rows, 'b', 'b'), null);
});

test('an unknown id is a no-op', () => {
  assert.equal(computeAssociationReorder(rows, 'missing', 'a'), null);
  assert.equal(computeAssociationReorder(rows, 'a', 'missing'), null);
});

test('a single row reordered alone is a no-op', () => {
  assert.equal(computeAssociationReorder([{ id: 'only', index: 0 }], 'only', 'only'), null);
});
