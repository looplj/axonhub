import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const srcRoot = join(import.meta.dirname, '..');

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

const { inclusiveCalendarDays } = await load('features/dashboard/utils/time-window.ts');

// 结束日期缺省时取的是"当前时刻"，固定到下午以暴露未归一时间戳的 off-by-one。
const NOW = new Date(2026, 8, 21, 15, 0, 0);
const RealDate = globalThis.Date;

class FrozenDate extends RealDate {
  constructor(...args) {
    if (args.length === 0) {
      super(NOW.getTime());
      return;
    }
    super(...args);
  }

  static now() {
    return NOW.getTime();
  }
}

test('inclusiveCalendarDays counts calendar days, not elapsed time', () => {
  globalThis.Date = FrozenDate;
  try {
    assert.equal(inclusiveCalendarDays('2026-09-21', null), 1);
    assert.equal(inclusiveCalendarDays('2026-09-20', null), 2);
    assert.equal(inclusiveCalendarDays('2026-09-15', null), 7);
    assert.equal(inclusiveCalendarDays('2026-08-22', null), 31);
    assert.equal(inclusiveCalendarDays('2026-08-21', null), 32);
  } finally {
    globalThis.Date = RealDate;
  }
});

test('inclusiveCalendarDays is inclusive of both endpoints', () => {
  assert.equal(inclusiveCalendarDays('2026-09-21', '2026-09-21'), 1);
  assert.equal(inclusiveCalendarDays('2026-09-15', '2026-09-21'), 7);
  assert.equal(inclusiveCalendarDays('2026-01-01', '2026-09-21'), 264);
});
