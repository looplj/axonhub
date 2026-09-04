import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(join(import.meta.dirname, 'channel-query-error.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;
const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { shouldNotifyChannelQueryError } = await import(moduleUrl);

test('notifies when the initial channel query fails without data', () => {
  assert.equal(shouldNotifyChannelQueryError(new Error('failed'), false, false), true);
});

test('notifies when a new query key fails with placeholder data', () => {
  assert.equal(shouldNotifyChannelQueryError(new Error('failed'), true, true), true);
});

test('does not notify when a background refresh fails with settled data', () => {
  assert.equal(shouldNotifyChannelQueryError(new Error('failed'), true, false), false);
});
