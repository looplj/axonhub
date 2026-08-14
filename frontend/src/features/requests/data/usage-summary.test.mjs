import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(join(import.meta.dirname, 'usage-summary.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;
const moduleURL = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { aggregatePrimaryUsageConnection, aggregateUsageByPurposeConnection, aggregateUsageConnection, aggregateUsageLogs } = await import(moduleURL);

test('aggregates usage and cost across visual and primary executions', () => {
  const summary = aggregateUsageLogs([
    {
      source: 'api',
      promptTokens: 7,
      completionTokens: 3,
      totalTokens: 10,
      promptCachedTokens: 2,
      promptWriteCachedTokens: 1,
      completionReasoningTokens: 0,
      totalCost: 0.25,
      costItems: [
        { itemCode: 'prompt_tokens', quantity: 7, subtotal: 0.1 },
        { itemCode: 'completion_tokens', quantity: 3, subtotal: 0.15 },
      ],
    },
    {
      source: 'api',
      promptTokens: 11,
      completionTokens: 5,
      totalTokens: 16,
      promptCachedTokens: 4,
      promptWriteCachedTokens: 0,
      completionReasoningTokens: 2,
      totalCost: 0.5,
      costItems: [
        { itemCode: 'prompt_tokens', quantity: 11, subtotal: 0.3 },
        { itemCode: 'completion_tokens', quantity: 5, subtotal: 0.2 },
      ],
    },
  ]);

  assert.deepEqual(summary, {
    source: 'api',
    promptTokens: 18,
    completionTokens: 8,
    totalTokens: 26,
    promptCachedTokens: 6,
    promptWriteCachedTokens: 1,
    completionReasoningTokens: 2,
    completionAudioTokens: 0,
    totalCost: 0.75,
    costItems: [
      { itemCode: 'prompt_tokens', quantity: 18, subtotal: 0.4 },
      { itemCode: 'completion_tokens', quantity: 8, subtotal: 0.35 },
    ],
  });
});

test('returns null when no execution has usage data', () => {
  assert.equal(aggregateUsageLogs([]), null);
  assert.equal(aggregateUsageLogs([null, undefined]), null);
});

test('aggregates the plain usage log list returned by request executions', () => {
  assert.equal(
    aggregateUsageConnection([
      { promptTokens: 4, completionTokens: 2, totalTokens: 6, totalCost: 0.1 },
      { promptTokens: 8, completionTokens: 3, totalTokens: 11, totalCost: 0.2 },
    ])?.totalTokens,
    17
  );
});

test('splits primary and vision delegation usage costs', () => {
  const summary = aggregateUsageByPurposeConnection({
    edges: [
      { node: { totalCost: 0.1, requestExecution: { purpose: 'primary' } } },
      { node: { totalCost: 0.2, requestExecution: { purpose: 'vision_delegation' } } },
      { node: { totalCost: 0.3 } },
    ],
  });

  assert.equal(summary.primary?.totalCost, 0.4);
  assert.equal(summary.visionDelegation?.totalCost, 0.2);
});

test('uses primary execution usage for cache metrics', () => {
  const summary = aggregatePrimaryUsageConnection({
    edges: [
      { node: { promptTokens: 100, promptCachedTokens: 90, totalTokens: 100, requestExecution: { purpose: 'primary' } } },
      { node: { promptTokens: 200, promptCachedTokens: 10, totalTokens: 200, requestExecution: { purpose: 'vision_delegation' } } },
    ],
  });

  assert.equal(summary?.promptTokens, 100);
  assert.equal(summary?.promptCachedTokens, 90);
});

test('does not relabel a vision-only execution as primary usage', () => {
  assert.equal(
    aggregatePrimaryUsageConnection([
      { promptTokens: 200, promptCachedTokens: 10, totalTokens: 200, requestExecution: { purpose: 'vision_delegation' } },
    ]),
    null
  );
});
