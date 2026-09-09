import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const source = readFileSync(join(import.meta.dirname, 'quota-routing-status.ts'), 'utf8');
const transpiled = ts.transpileModule(source.replace(/^import[^\n]*\n/gm, ''), {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { getChannelQuotaRoutingIndicator } = await import(`data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`);

const status = (value) => ({ status: value });

test('exhausted status is shown regardless of routing mode', () => {
  assert.equal(
    getChannelQuotaRoutingIndicator({ providerQuotaStatus: status('exhausted'), settings: { quotaRoutingMode: 'IGNORE_QUOTA' } }),
    'exhausted'
  );
});

test('warning status is shown as backpressure for an effective backpressure mode', () => {
  assert.equal(
    getChannelQuotaRoutingIndicator({ providerQuotaStatus: status('warning'), settings: { quotaRoutingMode: 'INHERIT' } }, 'BACKPRESSURE'),
    'backpressure'
  );
});

test('available status does not show a quota routing indicator', () => {
  assert.equal(
    getChannelQuotaRoutingIndicator({ providerQuotaStatus: status('available'), settings: { quotaRoutingMode: 'BACKPRESSURE' } }),
    undefined
  );
});
