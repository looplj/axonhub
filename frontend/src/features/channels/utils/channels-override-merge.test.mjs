import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(new URL('./merge.ts', import.meta.url), 'utf8');
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
const { mergeChannelSettingsForUpdate } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
const wire = (settings) => JSON.parse(JSON.stringify(settings));

test('syncModelPrices preserves stored values and applies explicit updates', () => {
  const existing = { syncModelPrices: true };
  const unrelated = wire(mergeChannelSettingsForUpdate(existing, { extraModelPrefix: 'prefix-' }));
  assert.equal(unrelated.syncModelPrices, true);

  const disabled = wire(mergeChannelSettingsForUpdate(existing, { syncModelPrices: false }));
  assert.equal(disabled.syncModelPrices, false);

  const enabled = wire(mergeChannelSettingsForUpdate(undefined, { syncModelPrices: true }));
  assert.equal(enabled.syncModelPrices, true);
});

test('WebSocket preference preserves stored values and supports explicit opt-in and opt-out', () => {
  const existing = { preferResponsesWebSocket: true };
  assert.equal(wire(mergeChannelSettingsForUpdate(existing, { extraModelPrefix: 'prefix-' })).preferResponsesWebSocket, true);
  assert.equal(wire(mergeChannelSettingsForUpdate(existing, { preferResponsesWebSocket: false })).preferResponsesWebSocket, false);
  assert.equal(wire(mergeChannelSettingsForUpdate(undefined, { preferResponsesWebSocket: true })).preferResponsesWebSocket, true);
  assert.equal(wire(mergeChannelSettingsForUpdate(undefined, {})).preferResponsesWebSocket, undefined);
});
