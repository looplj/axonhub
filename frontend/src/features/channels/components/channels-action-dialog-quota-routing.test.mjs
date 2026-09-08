import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Run the dialog's pure quota-routing-mode mapping helpers and the REAL
// mergeChannelSettingsForUpdate without loading React or its providers.
const dialogPath = new URL('./channels-action-dialog.tsx', import.meta.url);
const source = readFileSync(dialogPath, 'utf8');
const ast = ts.createSourceFile('channels-action-dialog.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

const helperNames = ['recallQuotaRoutingMode', 'quotaRoutingModeSettingsPatch'];
const helperNodes = ast.statements.filter((node) => ts.isFunctionDeclaration(node) && helperNames.includes(node.name?.text));
assert.equal(helperNodes.length, 2, 'mapping helpers must stay in channels-action-dialog.tsx');
const helpers = helperNodes.map((node) => `export ${node.getText(ast).replace(/^export\s+/, '')}`).join('\n');

function loadTsModule(sourceText) {
  const { outputText } = ts.transpileModule(sourceText, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  });
  return import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
}

const { recallQuotaRoutingMode, quotaRoutingModeSettingsPatch } = await loadTsModule(helpers);

// Type-only imports are erased, so utils/merge.ts loads standalone.
const mergeSource = readFileSync(new URL('../utils/merge.ts', import.meta.url), 'utf8');
const { mergeChannelSettingsForUpdate } = await loadTsModule(mergeSource);

// Wire payload = JSON round-trip: keys with undefined values drop.
const wire = (settings) => JSON.parse(JSON.stringify(settings));

test('recall: absent settings display as INHERIT', () => {
  assert.equal(recallQuotaRoutingMode(undefined), 'INHERIT');
  assert.equal(recallQuotaRoutingMode(null), 'INHERIT');
  assert.equal(recallQuotaRoutingMode({}), 'INHERIT');
});

test('recall: edit and duplicate show the stored mode', () => {
  assert.equal(recallQuotaRoutingMode({ quotaRoutingMode: 'BACKPRESSURE' }), 'BACKPRESSURE');
  assert.equal(recallQuotaRoutingMode({ quotaRoutingMode: 'REMOVE_ON_EXHAUSTED' }), 'REMOVE_ON_EXHAUSTED');
});

test('payload mapping: INHERIT selection sends explicit INHERIT (clears server-side)', () => {
  const patch = quotaRoutingModeSettingsPatch('INHERIT');
  assert.deepEqual(patch, { quotaRoutingMode: 'INHERIT' });
  // Create path (existing=null): pick() passes the explicit value through, so
  // the payload carries INHERIT; the backend maps INHERIT to empty storage.
  const merged = wire(mergeChannelSettingsForUpdate(undefined, patch));
  assert.equal(merged.quotaRoutingMode, 'INHERIT');
});

test('payload mapping: BACKPRESSURE survives into the payload', () => {
  const patch = quotaRoutingModeSettingsPatch('BACKPRESSURE');
  assert.deepEqual(patch, { quotaRoutingMode: 'BACKPRESSURE' });
  const merged = wire(mergeChannelSettingsForUpdate(undefined, patch));
  assert.equal(merged.quotaRoutingMode, 'BACKPRESSURE');
});

test('clobber protection: unrelated edit keeps the stored mode, INHERIT selection clears it', () => {
  const existing = { extraModelPrefix: 'prefix-', quotaRoutingMode: 'REMOVE_ON_EXHAUSTED' };

  // Unrelated fields only: the patch omits quotaRoutingMode, so the merge
  // whitelist falls back to the stored mode.
  const unrelatedPatch = { passThroughUserAgent: true, passThroughBody: false };
  const merged = wire(mergeChannelSettingsForUpdate(existing, unrelatedPatch));
  assert.equal(merged.quotaRoutingMode, 'REMOVE_ON_EXHAUSTED');
  assert.equal(merged.extraModelPrefix, 'prefix-');

  // Same edit while the select sits on INHERIT: the explicit value overrides
  // the whitelist fallback and reaches the payload, so the stored override is
  // cleared server-side (backend maps INHERIT to empty storage).
  const patchWithInheritSelected = { ...unrelatedPatch, ...quotaRoutingModeSettingsPatch('INHERIT') };
  assert.equal(patchWithInheritSelected.quotaRoutingMode, 'INHERIT');
  const mergedAgain = wire(mergeChannelSettingsForUpdate(existing, patchWithInheritSelected));
  assert.equal(mergedAgain.quotaRoutingMode, 'INHERIT');
});

test('mapping helpers stay wired into the dialog payload boundaries', () => {
  const spread = source.split('...quotaRoutingModeSettingsPatch(quotaRoutingMode)').length - 1;
  const recall = source.split('recallQuotaRoutingMode(initialRow?.settings)').length - 1;
  assert.equal(spread, 2, 'payload mapping must feed the edit settingsPatch and the create merge');
  assert.equal(recall, 2, 'recall mapping must feed the state init and the reopen reset');
});
