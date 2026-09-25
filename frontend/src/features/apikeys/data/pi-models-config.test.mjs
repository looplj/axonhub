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

const { PI_THINKING_LEVELS, buildPiThinkingLevelMap, buildPiModelsConfig, serializePiModelsConfig } = await loadModule(
  'features/apikeys/data/pi-models-config.ts'
);
const { deriveReasoningEfforts } = await loadModule('features/models/data/reasoning-efforts.ts');

const catalog = JSON.parse(read('features/models/data/providers.json'));

function catalogOptions(modelID) {
  const match = Object.values(catalog.providers)
    .flatMap((provider) => provider.models ?? [])
    .find((model) => model.id === modelID);

  assert.ok(match, `catalog should contain ${modelID}`);
  return match.reasoning_options;
}

function buildModel({ modelID, developer, name, card, reasoningEfforts }) {
  return {
    id: '1',
    modelID,
    developer,
    name: name ?? modelID,
    type: 'chat',
    modelCard: {
      reasoning: { supported: false, default: false },
      modalities: { input: ['text'], output: ['text'] },
      reasoningEfforts,
      ...card,
    },
  };
}

function buildConfig(models) {
  return buildPiModelsConfig({ origin: 'http://localhost:8090', apiKey: 'ah-test', models });
}

test('pi thinking levels follow the unified AxonHub vocabulary', () => {
  assert.deepEqual([...PI_THINKING_LEVELS], ['off', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max']);
});

test('levels the model form fills from the catalog export as pi thinking levels', () => {
  const cases = [
    {
      modelID: 'grok-4.6',
      expected: { off: null, minimal: null, low: 'low', medium: 'medium', high: 'high', xhigh: 'xhigh', max: null },
    },
    {
      modelID: 'deepseek-v4-flash',
      expected: { off: null, minimal: null, low: 'low', medium: null, high: 'high', xhigh: null, max: 'max' },
    },
    {
      modelID: 'glm-5.3-flash',
      expected: { off: null, minimal: null, low: 'low', medium: null, high: 'high', xhigh: null, max: 'max' },
    },
    {
      modelID: 'gpt-5.6-sol',
      expected: { off: 'none', minimal: null, low: 'low', medium: 'medium', high: 'high', xhigh: 'xhigh', max: 'max' },
    },
    {
      modelID: 'gpt-6-astra',
      expected: { off: null, minimal: null, low: 'low', medium: 'medium', high: 'high', xhigh: 'xhigh', max: 'max' },
    },
    {
      modelID: 'qwen3.8-flash',
      expected: { off: null, minimal: null, low: 'low', medium: 'medium', high: null, xhigh: 'xhigh', max: null },
    },
  ];

  for (const { modelID, expected } of cases) {
    const efforts = deriveReasoningEfforts(catalogOptions(modelID));
    assert.deepEqual(buildPiThinkingLevelMap(true, efforts), expected, `thinking level map mismatch for ${modelID}`);
  }
});

test('unknown levels keep no map, so pi falls back to its own defaults', () => {
  assert.equal(buildPiThinkingLevelMap(false, ['low']), undefined);
  assert.equal(buildPiThinkingLevelMap(true, undefined), undefined);
  assert.equal(buildPiThinkingLevelMap(true, []), undefined);
});

test('export reads the stored model card and never the catalog', () => {
  const reasoningCard = { reasoning: { supported: true, default: true } };

  // mimo-v2.6-flash only gets a level list from the model form, never from the
  // catalog, so an untouched model exports without a thinking level map.
  const untouched = buildConfig([buildModel({ modelID: 'mimo-v2.6-flash', developer: 'xiaomi', card: reasoningCard })]);
  assert.equal(untouched.providers.axonhub.models[0].thinkingLevelMap, undefined);

  const edited = buildConfig([
    buildModel({
      modelID: 'mimo-v2.6-flash',
      developer: 'xiaomi',
      card: reasoningCard,
      reasoningEfforts: ['none', 'low', 'high'],
    }),
  ]);
  assert.deepEqual(edited.providers.axonhub.models[0].thinkingLevelMap, {
    off: 'none',
    minimal: null,
    low: 'low',
    medium: null,
    high: 'high',
    xhigh: null,
    max: null,
  });
});

test('provider config carries the key, the /v1 endpoint and only chat models', () => {
  const models = [
    buildModel({
      modelID: 'grok-4.6',
      developer: 'xai',
      name: 'Grok 4.6',
      reasoningEfforts: ['low', 'medium', 'high', 'xhigh'],
      card: {
        reasoning: { supported: true, default: true },
        modalities: { input: ['text', 'image', 'video'], output: ['text'] },
        cost: { input: 2, output: 6, cacheRead: 0.5 },
        limit: { context: 500000, output: 500000 },
      },
    }),
    buildModel({ modelID: 'unpriced', developer: 'xai', card: { cost: { input: 0, output: 0 } } }),
    { ...buildModel({ modelID: 'embedding-model', developer: 'xai' }), type: 'embedding' },
  ];

  const provider = buildConfig(models).providers.axonhub;

  assert.equal(provider.api, 'openai-completions');
  assert.equal(provider.apiKey, 'ah-test');
  assert.equal(provider.baseUrl, 'http://localhost:8090/v1');
  assert.deepEqual(
    provider.models.map((model) => model.id),
    ['grok-4.6', 'unpriced']
  );

  const grok = provider.models.find((model) => model.id === 'grok-4.6');
  assert.deepEqual(grok.input, ['text', 'image']);
  assert.equal(grok.contextWindow, 500000);
  assert.equal(grok.maxTokens, 500000);
  assert.deepEqual(grok.cost, { input: 2, output: 6, cacheRead: 0.5, cacheWrite: 0 });
  assert.equal(grok.reasoning, true);
  assert.deepEqual(grok.thinkingLevelMap, {
    off: null,
    minimal: null,
    low: 'low',
    medium: 'medium',
    high: 'high',
    xhigh: 'xhigh',
    max: null,
  });

  const unpriced = provider.models.find((model) => model.id === 'unpriced');
  assert.equal(unpriced.cost, undefined);
  assert.equal(unpriced.contextWindow, undefined);
  assert.equal(unpriced.reasoning, false);
  assert.equal(unpriced.thinkingLevelMap, undefined);
});

test('serialized config sorts keys and ends with a newline', () => {
  const models = [
    buildModel({
      modelID: 'qwen3.8-flash',
      developer: 'alibaba',
      name: 'Qwen3.8 flash',
      reasoningEfforts: ['low', 'medium', 'xhigh'],
      card: {
        reasoning: { supported: true, default: true },
        modalities: { input: ['image', 'text'], output: ['text'] },
        cost: { input: 0.8, output: 2.7, cacheRead: 0.1, cacheWrite: 0 },
        limit: { context: 1000000, output: 131072 },
      },
    }),
  ];

  const text = serializePiModelsConfig(buildConfig(models));

  assert.ok(text.endsWith('}\n'), 'serialized config should end with a newline');
  assert.deepEqual(Object.keys(JSON.parse(text).providers.axonhub), ['api', 'apiKey', 'baseUrl', 'models']);
  assert.deepEqual(Object.keys(JSON.parse(text).providers.axonhub.models[0]), [
    'contextWindow',
    'cost',
    'id',
    'input',
    'maxTokens',
    'name',
    'reasoning',
    'thinkingLevelMap',
  ]);
  assert.deepEqual(Object.keys(JSON.parse(text).providers.axonhub.models[0].thinkingLevelMap), [
    'high',
    'low',
    'max',
    'medium',
    'minimal',
    'off',
    'xhigh',
  ]);
  assert.equal(JSON.parse(text).providers.axonhub.models[0].id, 'qwen3.8-flash');
});
