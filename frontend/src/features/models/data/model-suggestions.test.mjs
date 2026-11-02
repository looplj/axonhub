import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { isSuggestedModel, withMiniMaxFlash } from './model-suggestions.ts';
import { deriveReasoningEfforts } from './reasoning-efforts.ts';

const read = (path) => readFileSync(new URL(path, import.meta.url), 'utf8');

test('ordinary suggestions include current text routes without inventing gateway IDs', () => {
  const ids = [
    'MiniMax-M3.1-Flash', 'MiniMax-M3', 'minimax/minimax-m3.1-flash:free', 'minimax-m3.1-flash-free',
    'MiniMax-M2.7', 'minimax/minimax-m2.5', 'accounts/fireworks/models/minimax-m2p5',
    'MiniMax-Text-01', 'abab6.5s-chat', 'other-model', 'image-01', 'speech-02-hd',
    'MiniMax-Hailuo-02', 'saved-custom-alias',
  ];
  assert.deepEqual(ids.filter(isSuggestedModel), [
    ...ids.slice(0, 4), ...ids.slice(9),
  ]);
  assert.equal(ids.length, 14, 'filtering must not mutate saved or fetched IDs');
  assert.equal(isSuggestedModel('MiniMax-VL-01'), false);
});

test('direct catalog offers Flash first and retains old metadata for explicit selections', () => {
  const legacy = { id: 'MiniMax-M2.7', cost: { input: 0.3 } };
  const current = { id: 'MiniMax-M3' };
  const catalog = [legacy, current];
  const models = withMiniMaxFlash(catalog, 'minimax');
  assert.deepEqual(models.filter((model) => isSuggestedModel(model.id)).map((model) => model.id), ['MiniMax-M3.1-Flash', 'MiniMax-M3']);
  assert.equal(models.find((model) => model.id === legacy.id), legacy);
  assert.deepEqual(catalog, [legacy, current]);
  assert.deepEqual(models[0].limit, { context: 1000000, output: 524288 });
  assert.deepEqual(models[0].cost, { input: 0.15, output: 0.6, cache_read: 0.015 });
  assert.deepEqual(deriveReasoningEfforts(models[0].reasoning_options), ['low', 'medium', 'high', 'xhigh', 'max']);
  assert.equal(withMiniMaxFlash(models, 'minimax').filter((model) => model.id === models[0].id).length, 1);
  const refreshedFlash = { id: 'MiniMax-M3.1-Flash', cost: { input: 0.12 }, limit: { output: 600000 } };
  assert.equal(withMiniMaxFlash([legacy, refreshedFlash, current], 'minimax')[0], refreshedFlash);
});

test('gateway and plan catalogs retain their own metadata and available routes', () => {
  for (const provider of ['openrouter', 'opencode', 'minimax-coding-plan', 'minimax-cn', 'custom']) {
    const models = [{ id: 'minimax/minimax-m3', cost: { input: 7 }, limit: { output: 123 } }];
    assert.equal(withMiniMaxFlash(models, provider), models);
  }
});

test('both direct presets offer Flash first and keep the regional endpoints', () => {
  const source = read('../../channels/data/config_channels.ts');
  for (const [type, path] of [['minimax', 'v1'], ['minimax_anthropic', 'anthropic']]) {
    const block = source.match(new RegExp(`  ${type}: \\{([\\s\\S]*?)\\n  \\},`))[1];
    assert.ok(block.includes(`baseURL: 'https://api.minimaxi.com/${path}'`));
    assert.ok(block.includes("defaultModels: ['MiniMax-M3.1-Flash', 'MiniMax-M3']"));
  }
  assert.match(source, /defaultModels \|\| \[\]\)\.filter\(isSuggestedModel\)/);
});

test('filtering only affects suggestions, retaining complete fetched data for sync and saved selections', () => {
  const source = read('../../channels/components/channels-action-dialog.tsx');
  assert.match(source, /let models = fetchedModels\.filter\(isSuggestedModel\)/);
  assert.match(source, /items=\{fetchedModels\.filter\(isSuggestedModel\)\.map/);
  assert.match(source, /const fetchedModelsSet = new Set\(fetchedModels\)/);
  assert.match(source, /initialRow\?\.supportedModels \|\| \[\]/);
  assert.match(source, /initialRow\?\.manualModels \|\| \[\]/);
  for (const name of ['models-action-dialog', 'models-batch-create-dialog']) {
    const picker = read(`../components/${name}.tsx`);
    assert.match(picker, /withMiniMaxFlash\(provider\.models \|\| \[\], key\)/);
    assert.match(picker, /\.filter\(\(model\) => isSuggestedModel\(model\.id\)\)\s*\.map/);
  }
});
