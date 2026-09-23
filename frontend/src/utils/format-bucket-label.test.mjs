import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const srcRoot = join(import.meta.dirname, '..', '..');

function load(relativePath) {
  const source = readFileSync(join(srcRoot, relativePath), 'utf8');
  const transpiled = ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2023,
    },
  }).outputText;

  return import(`data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`);
}

const { formatBucketLabel } = await load('utils/format-bucket-label.ts');

test('daily labels carry only the date', () => {
  assert.equal(formatBucketLabel('2026-09-23', 'en-US'), '09/23');
});

// The hour lives in the label's time part, not in the date part. Building the Date from
// year/month/day alone renders every hourly bucket as midnight, which collapses a 24-point
// axis onto one or two labels.
test('hourly labels carry the hour the server emitted', () => {
  assert.equal(formatBucketLabel('2026-09-23 00:00', 'en-US'), '09/23, 00');
  assert.equal(formatBucketLabel('2026-09-23 09:00', 'en-US'), '09/23, 09');
  assert.equal(formatBucketLabel('2026-09-23 14:00', 'en-US'), '09/23, 14');
  assert.equal(formatBucketLabel('2026-09-23 23:00', 'en-US'), '09/23, 23');
});

test('consecutive hourly labels stay distinct', () => {
  const labels = ['00:00', '01:00', '12:00', '13:00', '23:00'].map((time) =>
    formatBucketLabel(`2026-09-23 ${time}`, 'en-US')
  );
  assert.equal(new Set(labels).size, labels.length);
});
