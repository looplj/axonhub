import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const srcRoot = join(import.meta.dirname, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

function transpile(source) {
  return ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2023,
    },
  }).outputText;
}

// The analytics helpers build on the shared date-range utilities, so both
// modules are concatenated into a single module to inspect the conversions.
const sharedSource = transpile(read('utils/date-range.ts'));
const analyticsSource = transpile(read('features/analytics/utils/date-range.ts')).replace(
  /import\s*\{[\s\S]*?\}\s*from\s*'@\/utils\/date-range';/,
  ''
);
const moduleUrl = `data:text/javascript;base64,${Buffer.from(sharedSource + analyticsSource).toString('base64')}`;
const { dateTimeRangeToTimestamps, parseLocalDate, startOfLocalDay, timestampsToDateTimeRange } = await import(moduleUrl);

test('timestamps round-trip through the picker value', () => {
  const start = new Date(2026, 9, 20, 8, 15, 30);
  const end = new Date(2026, 9, 26, 23, 59, 59);

  const range = timestampsToDateTimeRange(start.toISOString(), end.toISOString());

  assert.deepEqual(range.from, new Date(2026, 9, 20));
  assert.deepEqual(range.to, new Date(2026, 9, 26));
  assert.deepEqual(range.startTime, { hh: '08', mm: '15', ss: '30' });
  assert.deepEqual(range.endTime, { hh: '23', mm: '59', ss: '59' });
});

test('a picked range becomes inclusive millisecond bounds', () => {
  const { startTime, endTime } = dateTimeRangeToTimestamps({
    from: new Date(2026, 9, 20),
    to: new Date(2026, 9, 20),
    startTime: { hh: '10', mm: '20', ss: '30' },
    endTime: { hh: '11', mm: '20', ss: '30' },
  });

  assert.equal(startTime, new Date(2026, 9, 20, 10, 20, 30, 0).toISOString());
  assert.equal(endTime, new Date(2026, 9, 20, 11, 20, 30, 999).toISOString());
});

test('an open range only converts the bounds that are set', () => {
  assert.deepEqual(dateTimeRangeToTimestamps(undefined), { startTime: null, endTime: null });

  const { startTime, endTime } = dateTimeRangeToTimestamps({
    from: new Date(2026, 9, 20),
    startTime: { hh: '00', mm: '00', ss: '00' },
    endTime: { hh: '23', mm: '59', ss: '59' },
  });

  assert.equal(startTime, new Date(2026, 9, 20, 0, 0, 0, 0).toISOString());
  assert.equal(endTime, null);
});

test('backend dates and quick ranges resolve to local day boundaries', () => {
  assert.deepEqual(parseLocalDate('2026-03-10'), new Date(2026, 2, 10));
  assert.equal(parseLocalDate('not-a-date'), undefined);

  const now = new Date(2026, 9, 26, 13, 47, 28);
  assert.deepEqual(startOfLocalDay(now), new Date(2026, 9, 26, 0, 0, 0, 0));
});
