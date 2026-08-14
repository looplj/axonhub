import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(join(import.meta.dirname, 'execution-duration.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;
const moduleURL = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { executionDurationMs, sumExecutionDurations } = await import(moduleURL);

test('prefers persisted execution latency', () => {
  assert.equal(
    executionDurationMs({
      status: 'completed',
      metricsLatencyMs: 125,
      createdAt: '2026-08-14T00:00:00Z',
      updatedAt: '2026-08-14T00:00:01Z',
    }),
    125
  );
});

test('falls back to elapsed time for failed executions and sums retries', () => {
  assert.equal(
    sumExecutionDurations([
      {
        status: 'failed',
        createdAt: '2026-08-14T00:00:00Z',
        updatedAt: '2026-08-14T00:00:01Z',
      },
      { status: 'completed', metricsLatencyMs: 250 },
    ]),
    1250
  );
});

test('does not calculate elapsed time for an active execution', () => {
  assert.equal(
    executionDurationMs({
      status: 'processing',
      createdAt: '2026-08-14T00:00:00Z',
      updatedAt: '2026-08-14T00:00:01Z',
    }),
    null
  );
});
