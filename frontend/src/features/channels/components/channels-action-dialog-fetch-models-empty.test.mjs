import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Regression coverage for the F3 fix in handleFetchModels: a successful fetch
// that returns an empty model list must clear the previously fetched models and
// their prices instead of leaving stale data in the open panel. The repo has no
// React component-test setup for this dialog, so the REAL state-update block is
// extracted verbatim from handleFetchModels and executed against a state
// container whose setters mirror the dialog's useState setters, and the call
// site plus the empty-state render are pinned against the source.
const source = readFileSync(new URL('./channels-action-dialog.tsx', import.meta.url), 'utf8');

const START_MARKER = 'const models = result.models.map((m) => m.id);';
const END_MARKER = 'const handleSyncNow = useCallback';

const start = source.indexOf(START_MARKER);
const end = source.indexOf(END_MARKER);
assert.notEqual(start, -1, 'the fetch-result state block must stay in channels-action-dialog.tsx');
assert.notEqual(end, -1, 'the handleFetchModels dependency list must stay in channels-action-dialog.tsx');
assert.ok(end > start, 'the fetch-result state block must precede the handleFetchModels close');

// The block ends at the setApplyPatternFilter call; the catch clause that
// follows is not part of the state update. Search after the block start: the
// same call appears earlier in the component (state init / reopen reset).
const BLOCK_END_MARKER = 'setApplyPatternFilter(false);';
const blockEnd = source.indexOf(BLOCK_END_MARKER, start);
assert.ok(blockEnd > start, 'the state-update block must end at its setApplyPatternFilter call');
const blockSource = source.slice(start, blockEnd + BLOCK_END_MARKER.length);

const SETTER_NAMES = [
  'setFetchedModels',
  'setFetchedModelPrices',
  'setUseFetchedModels',
  'setShowFetchedModelsPanel',
  'setShowSupportedModelsPanel',
  'setShowApiKeysPanel',
  'setSelectedFetchedModels',
  'setFetchedModelsSearch',
  'setShowNotAddedModelsOnly',
  'setApplyPatternFilter',
];

// Executes the extracted block against `state`, binding every setter the block
// calls to a useState-style setter on the container.
function runBlock(state, result) {
  const set = (variableName) => (value) => {
    state[variableName] = value;
  };
  // The block carries a TS type annotation (Record<string, ModelPrice>), so
  // transpile it before execution, exactly like the other dialog tests.
  const { outputText } = ts.transpileModule(blockSource, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  });
  const runBlock = new Function(...SETTER_NAMES, 'result', outputText);
  runBlock(
    set('fetchedModels'),
    set('fetchedModelPrices'),
    set('useFetchedModels'),
    set('showFetchedModelsPanel'),
    set('showSupportedModelsPanel'),
    set('showApiKeysPanel'),
    set('selectedFetchedModels'),
    set('fetchedModelsSearch'),
    set('showNotAddedModelsOnly'),
    set('applyPatternFilter'),
    result
  );
}

// A fresh state container mirroring the dialog's useState defaults: an earlier
// fetch populated the panel with models and their prices.
function makePriorState() {
  return {
    fetchedModels: ['stale-model-a', 'stale-model-b'],
    fetchedModelPrices: { 'stale-model-a': { items: [{ itemCode: 'prompt_tokens', pricing: { usagePerUnit: '0.1' } }] } },
    useFetchedModels: false,
    showFetchedModelsPanel: false,
    showSupportedModelsPanel: true,
    showApiKeysPanel: false,
    selectedFetchedModels: ['stale-model-a'],
    fetchedModelsSearch: 'stale',
    showNotAddedModelsOnly: true,
    applyPatternFilter: true,
  };
}

test('a successful empty fetch clears the previously fetched models and prices and shows the empty state', () => {
  const state = makePriorState();
  runBlock(state, { models: [] });

  assert.deepEqual(state.fetchedModels, [], 'a successful empty fetch must clear the previously fetched models');
  assert.deepEqual(state.fetchedModelPrices, {}, 'a successful empty fetch must clear the previously fetched prices');
  assert.equal(state.useFetchedModels, true, 'the fetched-models view must be active so the empty state is shown');
  assert.equal(state.showFetchedModelsPanel, true, 'the fetched-models panel must be open');
  assert.equal(state.showSupportedModelsPanel, false, 'the supported-models panel must be closed');
  assert.equal(state.showApiKeysPanel, false, 'the api-keys panel must be closed');
  assert.deepEqual(state.selectedFetchedModels, [], 'stale selections must be dropped');
  assert.equal(state.fetchedModelsSearch, '', 'the search filter must be reset');
  assert.equal(state.showNotAddedModelsOnly, false, 'the not-added-only filter must be reset');
  assert.equal(state.applyPatternFilter, false, 'the pattern filter must be reset');
});

test('a successful non-empty fetch replaces the previous models and prices', () => {
  const state = makePriorState();
  const priceA = { items: [{ itemCode: 'prompt_tokens', pricing: { usagePerUnit: '0.5' } }] };
  runBlock(state, {
    models: [
      { id: 'fresh-model-a', pricing: priceA },
      { id: 'fresh-model-b', pricing: null },
    ],
  });

  assert.deepEqual(state.fetchedModels, ['fresh-model-a', 'fresh-model-b']);
  assert.deepEqual(Object.keys(state.fetchedModelPrices), ['fresh-model-a'], 'only models with a price get a price entry');
  assert.deepEqual(state.fetchedModelPrices['fresh-model-a'], priceA);
});

test('the extracted block stays the dialog real handleFetchModels state update', () => {
  const fetch = source.slice(source.indexOf('const handleFetchModels = useCallback'), source.indexOf('const handleSyncNow = useCallback'));
  assert.match(fetch, /const result = await fetchModels\.mutateAsync\(/, 'the handler must fetch via the mutation');
  assert.match(fetch, /setFetchedModels\(models\);/, 'the handler must reflect the fetched models');
  assert.match(fetch, /setFetchedModelPrices\(nextFetchedModelPrices\);/, 'the handler must reflect the fetched prices');

  // The empty state the cleared data renders: no fetched models => noModels hint.
  assert.match(source, /fetchedModels\.length === 0 && \(/, 'the fetched-models panel must render an empty state');
  assert.match(source, /channels\.dialogs\.fetchModels\.noModels/, 'the empty state must be the noModels hint');
});
