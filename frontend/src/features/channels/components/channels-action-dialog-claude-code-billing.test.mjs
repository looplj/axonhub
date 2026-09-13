import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// Run the dialog's pure Claude-Code-billing-header mapping helpers and the REAL
// mergeChannelSettingsForUpdate without loading React or its providers.
const dialogPath = new URL('./channels-action-dialog.tsx', import.meta.url);
const source = readFileSync(dialogPath, 'utf8');
const ast = ts.createSourceFile('channels-action-dialog.tsx', source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

const helperNames = ['recallClaudeCodeBillingHeaderMode', 'claudeCodeBillingHeaderSettingsPatch'];
const helperNodes = ast.statements.filter((node) => ts.isFunctionDeclaration(node) && helperNames.includes(node.name?.text));
assert.equal(helperNodes.length, 2, 'mapping helpers must stay in channels-action-dialog.tsx');
const helpers = helperNodes.map((node) => `export ${node.getText(ast).replace(/^export\s+/, '')}`).join('\n');

function loadTsModule(sourceText) {
  const { outputText } = ts.transpileModule(sourceText, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  });
  return import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
}

const { recallClaudeCodeBillingHeaderMode, claudeCodeBillingHeaderSettingsPatch } = await loadTsModule(helpers);

// Type-only imports are erased, so utils/merge.ts loads standalone.
const mergeSource = readFileSync(new URL('../utils/merge.ts', import.meta.url), 'utf8');
const { mergeChannelSettingsForUpdate } = await loadTsModule(mergeSource);

// Wire payload = JSON round-trip: keys with undefined values drop.
const wire = (settings) => JSON.parse(JSON.stringify(settings));

test('recall: absent settings display as AUTO', () => {
  assert.equal(recallClaudeCodeBillingHeaderMode(undefined), 'AUTO');
  assert.equal(recallClaudeCodeBillingHeaderMode(null), 'AUTO');
  assert.equal(recallClaudeCodeBillingHeaderMode({}), 'AUTO');
});

test('recall: edit and duplicate show the stored mode', () => {
  assert.equal(recallClaudeCodeBillingHeaderMode({ claudeCodeBillingHeader: 'KEEP' }), 'KEEP');
  assert.equal(recallClaudeCodeBillingHeaderMode({ claudeCodeBillingHeader: 'STRIP' }), 'STRIP');
});

test('payload mapping: every selection travels on the wire', () => {
  for (const mode of ['AUTO', 'KEEP', 'STRIP']) {
    const patch = claudeCodeBillingHeaderSettingsPatch(mode);
    assert.deepEqual(patch, { claudeCodeBillingHeader: mode });
    const merged = wire(mergeChannelSettingsForUpdate(undefined, patch));
    assert.equal(merged.claudeCodeBillingHeader, mode);
  }
});

test('clobber protection: unrelated edit keeps the stored mode, AUTO selection clears it', () => {
  const existing = { extraModelPrefix: 'prefix-', claudeCodeBillingHeader: 'KEEP' };

  // Unrelated fields only: the patch omits claudeCodeBillingHeader, so the
  // merge whitelist falls back to the stored mode.
  const unrelatedPatch = { passThroughUserAgent: true, passThroughBody: false };
  const merged = wire(mergeChannelSettingsForUpdate(existing, unrelatedPatch));
  assert.equal(merged.claudeCodeBillingHeader, 'KEEP');
  assert.equal(merged.extraModelPrefix, 'prefix-');

  // Same edit while the select sits on AUTO: the explicit value overrides the
  // whitelist fallback and reaches the payload, so the stored override is
  // cleared server-side (the backend maps AUTO to empty storage).
  const patchWithAutoSelected = { ...unrelatedPatch, ...claudeCodeBillingHeaderSettingsPatch('AUTO') };
  const mergedAgain = wire(mergeChannelSettingsForUpdate(existing, patchWithAutoSelected));
  assert.equal(mergedAgain.claudeCodeBillingHeader, 'AUTO');
});

test('mapping helpers stay wired into the dialog payload boundaries', () => {
  const spread = source.split('...claudeCodeBillingHeaderSettingsPatch(claudeCodeBillingHeader)').length - 1;
  const recall = source.split('recallClaudeCodeBillingHeaderMode(initialRow?.settings)').length - 1;
  assert.equal(spread, 2, 'payload mapping must feed the edit settingsPatch and the create merge');
  assert.equal(recall, 2, 'recall mapping must feed the state init and the reopen reset');
});

test('the full-node channel selection requests claudeCodeBillingHeader', () => {
  const channelsSource = readFileSync(new URL('../data/channels.ts', import.meta.url), 'utf8');
  assert.match(channelsSource, /CHANNEL_QUERY_FULL_NODE_SELECTION = `[\s\S]*?claudeCodeBillingHeader/);
});
