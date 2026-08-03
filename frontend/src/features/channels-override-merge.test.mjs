import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const srcRoot = join(import.meta.dirname, '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

const mergeSource = read('features/channels/utils/merge.ts');
const transpiledMerge = ts.transpileModule(mergeSource, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;
const mergeModuleUrl = `data:text/javascript;base64,${Buffer.from(transpiledMerge).toString('base64')}`;
const { mergeOverrideOperations } = await import(mergeModuleUrl);

test('set_if_absent participates in scalar body override replacement', () => {
  const setIfAbsent = { op: 'set_if_absent', path: 'max_output_tokens', value: '32000' };
  const temperature = { op: 'set', path: 'temperature', value: '0.5' };

  assert.deepEqual(mergeOverrideOperations([{ op: 'set', path: 'max_output_tokens', value: '8000' }, temperature], [setIfAbsent]), [
    setIfAbsent,
    temperature,
  ]);

  assert.deepEqual(mergeOverrideOperations([setIfAbsent], [{ op: 'delete', path: 'max_output_tokens' }]), [
    { op: 'delete', path: 'max_output_tokens' },
  ]);
});

test('scalar body override replacement removes existing duplicates', () => {
  const temperature = { op: 'set', path: 'temperature', value: '0.5' };

  assert.deepEqual(
    mergeOverrideOperations(
      [
        { op: 'set', path: 'max_output_tokens', value: '8000' },
        temperature,
        { op: 'set_if_absent', path: 'max_output_tokens', value: '32000' },
        { op: 'delete', path: 'max_output_tokens' },
      ],
      [{ op: 'set_if_absent', path: 'max_output_tokens', value: '16000' }]
    ),
    [{ op: 'set_if_absent', path: 'max_output_tokens', value: '16000' }, temperature]
  );
});

test('last template scalar body override at the same path wins', () => {
  assert.deepEqual(
    mergeOverrideOperations(
      [{ op: 'set', path: 'temperature', value: '0.5' }],
      [
        { op: 'set_if_absent', path: 'max_output_tokens', value: '32000' },
        { op: 'set', path: 'max_output_tokens', value: '16000' },
      ]
    ),
    [
      { op: 'set', path: 'temperature', value: '0.5' },
      { op: 'set', path: 'max_output_tokens', value: '16000' },
    ]
  );
});

test('set_if_absent is exposed as a localized body-only operation', () => {
  const schema = read('features/channels/data/schema.ts');
  const dialog = read('features/channels/components/channels-override-dialog.tsx');
  const bodyTypes = dialog.match(/const BODY_OP_TYPES:[\s\S]*?\];/)?.[0] || '';
  const headerTypes = dialog.match(/const HEADER_OP_TYPES:[^\n]+/)?.[0] || '';

  assert.match(schema, /'set_if_absent'/);
  assert.match(bodyTypes, /'set_if_absent'/);
  assert.doesNotMatch(headerTypes, /set_if_absent/);
  assert.match(dialog, /op\.op === 'set_if_absent' && parseValueForDisplay\(op\.value\)\.trim\(\) === ''/);

  for (const locale of ['en', 'zh-CN']) {
    const messages = JSON.parse(read(`locales/${locale}/channels.json`));
    assert.ok(messages['channels.dialogs.settings.overrides.body.opSetIfAbsent']);
    assert.ok(messages['channels.dialogs.settings.overrides.validation.missingValue']);
  }
});
