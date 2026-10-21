import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const sourcePath = join(import.meta.dirname, 'client-cancellation.ts');
const source = readFileSync(sourcePath, 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;

const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const {
  CLIENT_CANCELLATION_COOLDOWN_PRESETS,
  applyCooldownSelectionChange,
  clampCooldownMinutes,
  projectIDToNumber,
  resolveCooldownSelection,
} = await import(moduleUrl);

test('selecting custom keeps the current minutes and opens the free input', () => {
  const selection = resolveCooldownSelection(30, false);
  assert.deepEqual(selection, { value: '30', custom: false });

  const change = applyCooldownSelectionChange('custom');
  assert.deepEqual(change, { custom: true }, 'custom must not be dropped as a no-op');

  const next = resolveCooldownSelection(30, change.custom);
  assert.deepEqual(next, { value: 'custom', custom: true });
});

test('selecting a preset leaves custom mode and applies its duration', () => {
  const change = applyCooldownSelectionChange('10');
  assert.deepEqual(change, { minutes: 10, custom: false });
  assert.deepEqual(resolveCooldownSelection(change.minutes, change.custom), { value: '10', custom: false });
});

test('every preset is selectable and rendered as itself', () => {
  for (const minutes of CLIENT_CANCELLATION_COOLDOWN_PRESETS) {
    const change = applyCooldownSelectionChange(String(minutes));
    assert.equal(change.minutes, minutes);
    assert.equal(change.custom, false);
    assert.deepEqual(resolveCooldownSelection(minutes, false), { value: String(minutes), custom: false });
  }
});

test('a stored value outside the presets is shown as custom', () => {
  assert.deepEqual(resolveCooldownSelection(45, false), { value: 'custom', custom: true });
  assert.deepEqual(resolveCooldownSelection(1, false), { value: 'custom', custom: true });
});

test('custom minutes are clamped to the supported range', () => {
  assert.equal(clampCooldownMinutes(0), 1);
  assert.equal(clampCooldownMinutes(-5), 1);
  assert.equal(clampCooldownMinutes(10080), 10080);
  assert.equal(clampCooldownMinutes(99999), 10080);
  assert.equal(clampCooldownMinutes(Number.NaN), 1);
  assert.equal(clampCooldownMinutes(12.9), 12);
});

test('an unparsable or empty select value only switches to custom mode', () => {
  assert.deepEqual(applyCooldownSelectionChange('not-a-number'), { custom: true });
  // Radix reports '' when the controlled value matches no item; that must not
  // rewrite the stored duration to 1 minute.
  assert.deepEqual(applyCooldownSelectionChange(''), { custom: true });
  assert.deepEqual(applyCooldownSelectionChange('0'), { custom: true });
});

test('project ids are decoded from the axonhub GUID form', () => {
  assert.equal(projectIDToNumber('gid://axonhub/project/42'), 42);
  assert.equal(projectIDToNumber('gid://axonhub/project/1'), 1);
  assert.equal(projectIDToNumber('7'), 7);
  assert.equal(projectIDToNumber(9), 9);
  assert.equal(projectIDToNumber(''), 0);
  assert.equal(projectIDToNumber(null), 0);
  assert.equal(projectIDToNumber(undefined), 0);
  assert.equal(projectIDToNumber('not-a-guid'), 0);
});
