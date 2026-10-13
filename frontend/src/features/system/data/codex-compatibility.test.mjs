import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const dataDir = import.meta.dirname;
const localesDir = join(dataDir, '..', '..', '..', 'locales');

const transpiled = ts.transpileModule(readFileSync(join(dataDir, 'codex-compatibility.ts'), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const codex = await import(`data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`);

const scopesOf =
  (...granted) =>
  (scope) =>
    granted.includes(scope);

const saved = { enabled: false, channelID: null };
const channels = [
  { id: 3, name: 'official-oauth', status: 'enabled' },
  { id: 4, name: 'relay-api-key', status: 'disabled' },
  { id: 5, name: 'old-relay', status: 'archived' },
];
const fullAccess = () => codex.codexAccess(scopesOf('read_settings', 'write_settings', 'read_channels'));

function controls(overrides = {}) {
  return codex.codexFormControls({
    access: fullAccess(),
    form: { enabled: false, channelID: 4 },
    saved,
    saving: false,
    test: codex.idleCodexTest,
    ...overrides,
  });
}

function run(actions, from = codex.idleCodexTest) {
  return actions.reduce((state, action) => codex.codexTestReducer(state, action), from);
}

const okResult = { success: true, modelCount: 7, upstreamStatus: null, error: null };
const notFoundResult = { success: false, modelCount: 0, upstreamStatus: 404, error: 'Codex catalog upstream HTTP 404' };

// --- access: settings write/read and channel read are independent scopes ------------------------------

test('settings read alone can view the tab but neither list channels nor edit', () => {
  assert.deepEqual(codex.codexAccess(scopesOf('read_settings')), { canListChannels: false, canEdit: false });
});

test('settings without read permission render permission state instead of load failure', () => {
  assert.equal(codex.codexSettingsViewState({ canReadSettings: false, isLoading: false, hasData: false }), 'permission');
  assert.equal(codex.codexSettingsViewState({ canReadSettings: true, isLoading: true, hasData: false }), 'loading');
  assert.equal(codex.codexSettingsViewState({ canReadSettings: true, isLoading: false, hasData: false }), 'loadFailed');
  assert.equal(codex.codexSettingsViewState({ canReadSettings: true, isLoading: false, hasData: true }), 'ready');
});

test('settings write without channel read cannot list, test, or save a channel reference', () => {
  assert.deepEqual(codex.codexAccess(scopesOf('read_settings', 'write_settings')), { canListChannels: false, canEdit: false });
});

test('channel read without settings write can list channels but not edit', () => {
  assert.deepEqual(codex.codexAccess(scopesOf('read_settings', 'read_channels')), { canListChannels: true, canEdit: false });
});

test('a non-owner holding exactly the three required scopes can list and edit', () => {
  assert.deepEqual(fullAccess(), { canListChannels: true, canEdit: true });
});

test('write and channel scopes without settings read cannot do anything', () => {
  assert.deepEqual(codex.codexAccess(scopesOf('write_settings', 'read_channels')), { canListChannels: false, canEdit: false });
});

// --- option order --------------------------------------------------------------------------------------

test('channel options are ordered by name then id so the list never shuffles with backend row order', () => {
  const shuffled = [
    { id: 9, name: 'relay b', status: 'enabled' },
    { id: 4, name: 'Relay A', status: 'archived' },
    { id: 7, name: 'relay a10', status: 'disabled' },
    { id: 2, name: 'relay a2', status: 'enabled' },
    { id: 8, name: 'relay b', status: 'disabled' },
  ];
  const sorted = codex.sortCodexChannels(shuffled);
  assert.deepEqual(
    sorted.map((channel) => channel.id),
    [4, 2, 7, 8, 9]
  );
  assert.deepEqual(
    shuffled.map((channel) => channel.id),
    [9, 4, 7, 2, 8],
    'the input must not be mutated'
  );
});

// --- saved reference restoration -----------------------------------------------------------------------

test('a saved channel that is still listed is restored into the form', () => {
  assert.deepEqual(codex.resolveSelection({ enabled: true, channelID: 4 }, channels), { channelID: 4, referenceMissing: false });
});

test('a saved channel that disappeared is cleared and flagged instead of silently re-submitted', () => {
  assert.deepEqual(codex.resolveSelection({ enabled: true, channelID: 99 }, channels), { channelID: null, referenceMissing: true });
});

test('without a channel list the saved reference is kept as is', () => {
  assert.deepEqual(codex.resolveSelection({ enabled: true, channelID: 99 }, undefined), { channelID: 99, referenceMissing: false });
});

test('an unset reference stays unset and is not reported as missing', () => {
  assert.deepEqual(codex.resolveSelection(saved, channels), { channelID: null, referenceMissing: false });
});

// --- form rules ----------------------------------------------------------------------------------------

test('enabling without a channel is an issue while disabling without one is fine', () => {
  assert.equal(controls({ form: { enabled: true, channelID: null } }).issue, 'channel_required');
  assert.equal(controls({ form: { enabled: false, channelID: null } }).issue, null);
  assert.equal(controls({ form: { enabled: true, channelID: 4 } }).issue, null);
});

test('an unchanged form cannot be saved but any edited valid form can', () => {
  assert.equal(controls({ form: saved }).canSave, false);
  assert.equal(controls({ form: { enabled: false, channelID: 4 } }).canSave, true);
  assert.equal(controls({ form: { enabled: true, channelID: 4 } }).canSave, true);
});

test('saving is blocked while a save is pending and for an invalid enabled form without channel', () => {
  assert.equal(controls({ saving: true }).canSave, false);
  const invalid = controls({ form: { enabled: true, channelID: null } });
  assert.equal(invalid.canSave, false);
  assert.equal(invalid.issue, 'channel_required');
});

test('a failed or running catalog test never blocks saving', () => {
  const failed = run([
    { type: 'started', seq: 1, channelID: 4 },
    { type: 'finished', seq: 1, result: notFoundResult },
  ]);
  assert.equal(failed.phase, 'failed');
  assert.equal(controls({ test: failed }).canSave, true);
  assert.equal(controls({ test: { phase: 'running', seq: 2, channelID: 4 } }).canSave, true);
});

test('testing works with the global switch off and for any channel status', () => {
  for (const channel of channels) {
    assert.equal(controls({ form: { enabled: false, channelID: channel.id } }).canTest, true, `status ${channel.status}`);
  }
});

test('testing needs a selected channel, edit access, and no test already running', () => {
  assert.equal(controls({ form: { enabled: false, channelID: null } }).canTest, false);
  assert.equal(controls({ access: codex.codexAccess(scopesOf('read_settings', 'read_channels')) }).canTest, false);
  assert.equal(controls({ access: codex.codexAccess(scopesOf('read_settings', 'write_settings')) }).canTest, false);
  assert.equal(controls({ test: { phase: 'running', seq: 1, channelID: 4 } }).canTest, false);
});

test('saving without edit access is impossible even for a valid edited form', () => {
  assert.equal(controls({ access: codex.codexAccess(scopesOf('read_settings', 'read_channels')) }).canSave, false);
  assert.equal(controls({ access: codex.codexAccess(scopesOf('read_settings', 'write_settings')) }).canSave, false);
});

test('a cleared missing reference can be saved as disabled but not as enabled', () => {
  const missing = { enabled: true, channelID: 99 };
  assert.equal(controls({ saved: missing, form: { enabled: false, channelID: null } }).canSave, true);
  assert.equal(controls({ saved: missing, form: { enabled: true, channelID: null } }).canSave, false);
});

// --- write contract ------------------------------------------------------------------------------------

test('the update input carries exactly the enabled flag and a numeric-or-null channel id', () => {
  const input = codex.toUpdateInput({ enabled: true, channelID: 12 });
  assert.deepEqual(input, { enabled: true, channelID: 12 });
  assert.equal(typeof input.channelID, 'number');
  assert.deepEqual(Object.keys(input), ['enabled', 'channelID']);
  assert.deepEqual(codex.toUpdateInput({ enabled: false, channelID: null }), { enabled: false, channelID: null });
});

test('select values round-trip numeric ids and the none sentinel', () => {
  assert.equal(codex.parseChannelSelectValue(codex.channelSelectValue(12)), 12);
  assert.equal(codex.parseChannelSelectValue(codex.channelSelectValue(null)), null);
  assert.equal(codex.channelSelectValue(12), '12');
  assert.notEqual(codex.channelSelectValue(null), '');
});

test('malformed select values mean no selection instead of NaN', () => {
  for (const value of ['', 'abc', '1.5', '-3', '0']) {
    assert.equal(codex.parseChannelSelectValue(value), null, JSON.stringify(value));
  }
});

// --- test-state machine --------------------------------------------------------------------------------

test('a successful test records the model count for that channel', () => {
  const state = run([
    { type: 'started', seq: 1, channelID: 4 },
    { type: 'finished', seq: 1, result: okResult },
  ]);
  assert.deepEqual(state, { phase: 'succeeded', channelID: 4, modelCount: 7 });
});

test('an upstream failure is a readable failure carrying the sanitized message and status, never a success', () => {
  const state = run([
    { type: 'started', seq: 1, channelID: 4 },
    { type: 'finished', seq: 1, result: notFoundResult },
  ]);
  assert.deepEqual(state, { phase: 'failed', channelID: 4, message: 'Codex catalog upstream HTTP 404', upstreamStatus: 404 });
});

test('a rejected request becomes a failure without an upstream status', () => {
  const state = run([
    { type: 'started', seq: 1, channelID: 4 },
    { type: 'rejected', seq: 1, message: 'Network error' },
  ]);
  assert.deepEqual(state, { phase: 'failed', channelID: 4, message: 'Network error', upstreamStatus: null });
});

test('a result that arrives after switching channels is ignored', () => {
  const state = run([
    { type: 'started', seq: 1, channelID: 4 },
    { type: 'selectionChanged', channelID: 3 },
    { type: 'finished', seq: 1, result: okResult },
  ]);
  assert.deepEqual(state, codex.idleCodexTest);
});

test('a result that arrives after clearing the selection is ignored', () => {
  const state = run([
    { type: 'started', seq: 1, channelID: 4 },
    { type: 'selectionChanged', channelID: null },
    { type: 'rejected', seq: 1, message: 'late' },
  ]);
  assert.deepEqual(state, codex.idleCodexTest);
});

test('a result that arrives after cancel is ignored', () => {
  const state = run([
    { type: 'started', seq: 1, channelID: 4 },
    { type: 'cancelled' },
    { type: 'finished', seq: 1, result: okResult },
  ]);
  assert.deepEqual(state, codex.idleCodexTest);
});

test('an older response cannot overwrite a newer test of the same channel', () => {
  const state = run([
    { type: 'started', seq: 1, channelID: 4 },
    { type: 'cancelled' },
    { type: 'started', seq: 2, channelID: 4 },
    { type: 'finished', seq: 1, result: okResult },
  ]);
  assert.deepEqual(state, { phase: 'running', seq: 2, channelID: 4 });
});

test('switching channels discards a previous success, re-selecting the same channel keeps it', () => {
  const succeeded = { phase: 'succeeded', channelID: 4, modelCount: 7 };
  assert.deepEqual(run([{ type: 'selectionChanged', channelID: 3 }], succeeded), codex.idleCodexTest);
  assert.deepEqual(run([{ type: 'selectionChanged', channelID: 4 }], succeeded), succeeded);
});

test('cancelling only affects a running test', () => {
  const succeeded = { phase: 'succeeded', channelID: 4, modelCount: 7 };
  assert.deepEqual(run([{ type: 'cancelled' }], succeeded), succeeded);
  assert.deepEqual(run([{ type: 'cancelled' }]), codex.idleCodexTest);
});

test('the visibility selector never exposes another channel result, even before a reset', () => {
  const succeeded = { phase: 'succeeded', channelID: 4, modelCount: 7 };
  assert.deepEqual(codex.visibleCodexTest(succeeded, 4), succeeded);
  assert.deepEqual(codex.visibleCodexTest(succeeded, 3), codex.idleCodexTest);
  assert.deepEqual(codex.visibleCodexTest(succeeded, null), codex.idleCodexTest);
});

test('starting a new test replaces a previous result with the in-progress state', () => {
  const failed = { phase: 'failed', channelID: 4, message: 'x', upstreamStatus: 404 };
  assert.deepEqual(run([{ type: 'started', seq: 5, channelID: 4 }], failed), { phase: 'running', seq: 5, channelID: 4 });
});

// --- locale parity -------------------------------------------------------------------------------------

test('Codex settings strings exist in both locales with identical keys and interpolation placeholders', () => {
  const en = JSON.parse(readFileSync(join(localesDir, 'en', 'system.json'), 'utf8'));
  const zh = JSON.parse(readFileSync(join(localesDir, 'zh-CN', 'system.json'), 'utf8'));
  const isCodexKey = (key) => key === 'system.tabs.codex' || key.startsWith('system.codex.');
  const enKeys = Object.keys(en).filter(isCodexKey).sort();
  const zhKeys = Object.keys(zh).filter(isCodexKey).sort();
  assert.ok(enKeys.length > 0, 'no Codex keys in en/system.json');
  assert.deepEqual(zhKeys, enKeys);
  const placeholders = (text) => [...text.matchAll(/\{\{\s*(\w+)\s*\}\}/g)].map((match) => match[1]).sort();
  for (const key of enKeys) {
    assert.equal(typeof en[key], 'string');
    assert.ok(en[key].trim() && zh[key].trim(), `${key} must be non-empty in both locales`);
    assert.deepEqual(placeholders(zh[key]), placeholders(en[key]), `${key} placeholders differ between locales`);
  }
});
