import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(join(import.meta.dirname, 'zenmux-quota-display.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;
const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const { capitalizeZenmuxTier, getZenmuxMonthlyQuotaUSD, getZenmuxUsagePercentage } = await import(moduleUrl);

test('uses normalized window limits for ZenMux battery percentage', () => {
  const limits = [
    { window: '5h', usageRatio: 0.86 },
    { window: '7d', usageRatio: 0.45 },
  ];

  assert.equal(getZenmuxUsagePercentage(limits), 86);
});

test('uses the ZenMux monthly quota USD value', () => {
  const data = {
    quota_monthly: {
      max_flows: 34560,
      max_value_usd: 1367.24,
    },
  };

  assert.equal(getZenmuxMonthlyQuotaUSD(data), 1367.24);
});

test('capitalizes the ZenMux plan tier', () => {
  assert.equal(capitalizeZenmuxTier('ultra'), 'Ultra');
});
