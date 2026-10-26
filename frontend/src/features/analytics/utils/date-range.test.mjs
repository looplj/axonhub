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
const { dateTimeRangeToTimestamps, startOfLocalDay, timestampsToDateTimeRange } = await import(moduleUrl);

test('timestamps round-trip through the picker value', () => {
  const start = new Date(2026, 9, 20, 8, 15, 30);
  const end = new Date(2026, 9, 26, 23, 59, 59);

  const { range } = timestampsToDateTimeRange(start.toISOString(), end.toISOString());

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

test('quick ranges resolve to local day boundaries', () => {
  const now = new Date(2026, 9, 26, 13, 47, 28);

  assert.deepEqual(startOfLocalDay(now), new Date(2026, 9, 26, 0, 0, 0, 0));
});

// Regression: an open-ended filter used to borrow the other bound's day, so
// merely re-confirming it in the picker wrote a fabricated, single-day range.
test('a start-only filter does not gain an end bound when restored', () => {
  const startOnly = new Date(2026, 9, 20, 10, 0, 0);

  const bounds = timestampsToDateTimeRange(startOnly.toISOString(), null);

  assert.deepEqual(bounds.range.from, new Date(2026, 9, 20));
  assert.equal(bounds.range.to, undefined);

  const { startTime, endTime } = dateTimeRangeToTimestamps(bounds.range, bounds);

  assert.equal(startTime, startOnly.toISOString());
  assert.equal(endTime, null);
});

test('an end-only filter does not gain a start bound when restored', () => {
  const endOnly = new Date(2026, 9, 20, 10, 0, 0);

  const bounds = timestampsToDateTimeRange(null, endOnly.toISOString());

  assert.equal(bounds.range.from, undefined);
  assert.deepEqual(bounds.range.to, new Date(2026, 9, 20));

  const { startTime, endTime } = dateTimeRangeToTimestamps(bounds.range, bounds);

  assert.equal(startTime, null);
  assert.notEqual(endTime, null);
});

// Regression: the picker edits whole seconds only, so re-confirming a stored
// range used to rewrite `.123Z`/`.456Z` to `.000Z`/`.999Z` and widen the window.
test('confirming an unchanged range keeps its millisecond components', () => {
  const start = new Date(2026, 9, 20, 10, 20, 30, 123);
  const end = new Date(2026, 9, 21, 11, 22, 33, 456);

  const bounds = timestampsToDateTimeRange(start.toISOString(), end.toISOString());
  const confirmed = dateTimeRangeToTimestamps(bounds.range, bounds);

  assert.equal(confirmed.startTime, start.toISOString());
  assert.equal(confirmed.endTime, end.toISOString());
});

test('editing a bound falls back to whole-second boundaries', () => {
  const start = new Date(2026, 9, 20, 10, 20, 30, 123);
  const end = new Date(2026, 9, 21, 11, 22, 33, 456);

  const bounds = timestampsToDateTimeRange(start.toISOString(), end.toISOString());
  const edited = {
    ...bounds.range,
    startTime: { hh: '09', mm: '20', ss: '30' },
  };
  const confirmed = dateTimeRangeToTimestamps(edited, bounds);

  assert.equal(confirmed.startTime, new Date(2026, 9, 20, 9, 20, 30, 0).toISOString());
  // The untouched end still keeps its milliseconds.
  assert.equal(confirmed.endTime, end.toISOString());
});

// Regression (DST fall-back): 01:30 local occurs twice on 2024-11-03 in
// America/New_York. Rebuilding the bound from wall-clock fields picks the first
// occurrence, so an untouched bound must be reused verbatim.
test('an unchanged bound survives a fall-back DST hour', { skip: process.env.TZ !== 'America/New_York' }, () => {
  const later = '2024-11-03T06:30:00.123Z';

  const bounds = timestampsToDateTimeRange(later, null);
  const confirmed = dateTimeRangeToTimestamps(bounds.range, bounds);

  assert.equal(confirmed.startTime, later);
});
