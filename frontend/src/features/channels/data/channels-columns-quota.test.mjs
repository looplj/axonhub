import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import ts from 'typescript';

const dataDir = import.meta.dirname;
const srcRoot = join(dataDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

// Load the real parser and channel schema through TypeScript transpilation so
// the tests exercise the exact contract the channel table consumes. Bare
// imports are resolved by stubbing the imported bindings on globalThis before
// evaluation via an injected prelude.
function loadModule(relativePath, transform = (s) => s) {
  const source = transform(read(relativePath)).replace(/^import[^\n]*\n/gm, '');
  const transpiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  }).outputText;
  const prelude = 'const { z, pageInfoSchema } = globalThis.__importStubs;\n';
  return import(`data:text/javascript;base64,${Buffer.from(prelude + transpiled).toString('base64')}`);
}

const { z } = await import('zod');
globalThis.__importStubs = { z, pageInfoSchema: undefined };
const paginationSource = read('gql/pagination.ts').replace(/^import[^\n]*\n/gm, '');
const paginationTranspiled = ts.transpileModule(paginationSource, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { pageInfoSchema } = await import(
  `data:text/javascript;base64,${Buffer.from('const { z } = globalThis.__importStubs;\n' + paginationTranspiled).toString('base64')}`
);
globalThis.__importStubs = { z, pageInfoSchema };

const { parseQuotaLimits } = await loadModule('features/system/data/quotas.ts', (s) =>
  s.replace(/export function useProviderQuotaStatuses[\s\S]*$/m, '')
);

const { channelSchema } = await loadModule('features/channels/data/schema.ts', (s) => s.replace(/z\.url\(/g, 'z.string().url('));

const channelQuerySource = read('features/channels/data/channels.ts').replace(/^import(?:.|\n)*?;\n/gm, '');
const channelQueryTranspiled = ts.transpileModule(channelQuerySource, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { buildQueryChannelsQuery } = await import(
  `data:text/javascript;base64,${Buffer.from(
    'const { z, pageInfoSchema } = globalThis.__importStubs;\n' + channelQueryTranspiled
  ).toString('base64')}`
);

const columnsSource = read('features/channels/components/channels-columns.tsx');

function channelFixture({ type, providerQuotaStatus }) {
  return {
    id: 'channel-1',
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    type,
    baseURL: 'https://example.com',
    name: 'fixture',
    status: 'enabled',
    supportedModels: ['gpt-5'],
    defaultTestModel: 'gpt-5',
    providerQuotaStatus,
  };
}

test('normalized limits: parser returns every _limits entry without fabrication', () => {
  const limits = parseQuotaLimits({
    _limits: [
      { type: 'token', window: '5h', usageRatio: 0.4, status: 'available', ready: true },
      { type: 'token', window: '7d', usageRatio: 0.2, status: 'warning', ready: true, nextResetAt: '2026-09-13T00:00:00Z' },
    ],
  });

  assert.equal(limits.length, 2);
  assert.deepEqual(
    limits.map((l) => l.window),
    ['5h', '7d']
  );
});

test('unknown normalized windows use a neutral label instead of provider slot names', () => {
  assert.match(
    columnsSource,
    /window === 'primary' \|\| window === 'secondary' \? t\('quota\.label\.token_usage'\)/,
    'Codex windows without a known duration should not display primary or secondary'
  );
});

test('Codex Pro seven-day-only data keeps its seven-day label', () => {
  const limits = parseQuotaLimits({
    _limits: [{ type: 'token', window: '7d', usageRatio: 0.4, status: 'available', ready: true }],
  });

  assert.deepEqual(limits.map((limit) => limit.window), ['7d']);
  assert.notEqual(limits[0].window, '5h');
});

test('normalized limits: unknown window strings pass through neutrally', () => {
  const limits = parseQuotaLimits({
    _limits: [{ type: 'token', window: 'custom_provider_window', usageRatio: 0.5, status: 'available', ready: true }],
  });

  assert.equal(limits.length, 1);
  assert.equal(limits[0].window, 'custom_provider_window');
});

test('normalized limits: malformed entries do not throw and are defensively typed', () => {
  const limits = parseQuotaLimits({
    _limits: [null, 42, 'broken', { window: 7, usageRatio: 'lots' }, { type: 'token', window: 'daily', usageRatio: 0.1, status: 'available' }],
  });

  assert.ok(Array.isArray(limits));
  const daily = limits.find((l) => l.window === 'daily');
  assert.ok(daily, 'the well-formed entry should survive');
});

test('normalized limits: malformed _limits containers fail closed', () => {
  for (const rawLimits of [null, {}, 'broken', 42]) {
    assert.deepEqual(parseQuotaLimits({ _limits: rawLimits }), [], `malformed _limits value ${String(rawLimits)} must be unavailable`);
  }
});

test('normalized limits: malformed required fields produce no rendered limits', () => {
  // Given entries with malformed identity, status, or usage fields,
  // When the provider quota payload is parsed,
  // Then every malformed entry is omitted instead of becoming a fabricated row.
  const limits = parseQuotaLimits({
    _limits: [
      { type: '', window: '5h', usageRatio: 0.1, status: 'available' },
      { type: 'token', window: ' ', usageRatio: 0.1, status: 'available' },
      { type: 42, window: '5h', usageRatio: 0.1, status: 'available' },
      { type: 'token', window: 5, usageRatio: 0.1, status: 'available' },
      { type: 'token', window: '5h', usageRatio: '0.5', status: 'available' },
      { type: 'token', window: '5h', usageRatio: -0.1, status: 'available' },
      { type: 'token', window: '5h', usageRatio: 1.1, status: 'available' },
      { type: 'token', window: '5h', usageRatio: Number.NaN, status: 'available' },
      { type: 'token', window: '5h', usageRatio: 0.1, status: 'invalid' },
    ],
  });

  assert.deepEqual(limits, []);
});

test('no provider fallback: channels table renders quota for any channel type with normalized limits', () => {
  // Given a non-allowlisted channel type whose quota status carries _limits,
  // When the quota cell decides whether to render rows,
  // Then eligibility must depend on the quota data, not the channel type.
  for (const type of [
    'zenmux',
    'zenmux_responses',
    'zenmux_anthropic',
    'zenmux_gemini',
    'cline',
    'nanogpt',
    'minimax',
    'openai',
  ]) {
    const parsed = channelSchema.safeParse(
      channelFixture({
        type,
        providerQuotaStatus: {
          status: 'available',
          ready: true,
          providerType: type,
          quotaData: { _limits: [{ type: 'token', window: '5h', usageRatio: 0.5, status: 'available', ready: true }] },
        },
      })
    );
    assert.ok(parsed.success, `channel schema should accept ${type} with normalized limits`);

    const limits = parseQuotaLimits(parsed.data.providerQuotaStatus.quotaData);
    assert.equal(limits.length, 1, `${type} should yield its normalized limit row`);
  }

  assert.doesNotMatch(
    columnsSource,
    /OAUTH_CHANNEL_TYPES/,
    'the channel table must not gate quota rendering behind an OAuth channel allowlist'
  );
  assert.match(
    columnsSource,
    /parseQuotaLimits/,
    'the channel table should consume the shared provider-neutral normalized-limit parser'
  );
});

test('no provider fallback: quota cell has no provider-specific raw-data parsing branches', () => {
  const quotaCellStart = columnsSource.indexOf('const QuotaCell');
  const quotaCellEnd = columnsSource.indexOf("QuotaCell.displayName", quotaCellStart);
  assert.ok(quotaCellStart !== -1 && quotaCellEnd > quotaCellStart, 'QuotaCell should exist');
  const quotaCell = columnsSource.slice(quotaCellStart, quotaCellEnd);

  for (const forbidden of [
    /channel\.type === 'xai_subscription'/,
    /channel\.type === 'claudecode'/,
    /channel\.type === 'codex'/,
    /channel\.type === 'antigravity'/,
    /channel\.type === 'github_copilot'/,
    /data\.billing/,
    /data\.windows/,
    /data\.models/,
    /data\.rate_limit/,
  ]) {
    assert.doesNotMatch(quotaCell, forbidden, `quota cell must not contain provider-specific raw-data fallback matching ${forbidden}`);
  }
});

test('no provider fallback: raw-only provider payloads without _limits yield unavailable, not special parsing', () => {
  // Given raw provider payloads in the pre-normalization shapes,
  // When parsed through the provider-neutral parser,
  // Then no limits are produced (the UI shows unavailable) — the frontend must
  // never reconstruct windows from raw provider data.
  const rawPayloads = {
    codex: { rate_limit: { primary_window: { used_percent: 42 }, secondary_window: { used_percent: 10 } } },
    xai_subscription: { billing: { weekly: { usage_percent: 30 }, monthly: { usage_percent: 55 } } },
    claudecode: { windows: { '5h': { utilization: 0.4 }, '7d': { utilization: 0.2 } } },
    antigravity: { models: { 'gemini-2.5-pro': { remainingPercentage: 80, displayName: 'Gemini 2.5 Pro' } } },
    github_copilot: { quota_snapshots: { chat: { percent_remaining: 50 } } },
  };

  for (const [provider, raw] of Object.entries(rawPayloads)) {
    assert.deepEqual(parseQuotaLimits(raw), [], `${provider} raw-only payload must not be parsed into limits by the frontend`);
  }
});

test('unknown usage: limits with missing usage fail closed without rendering exhausted', () => {
  for (const usageRatio of [undefined, Number.NaN, Number.POSITIVE_INFINITY]) {
    const limits = parseQuotaLimits({
      _limits: [{ type: 'token', window: '7d', status: 'exhausted', ready: false, usageRatio }],
    });

    assert.deepEqual(limits, [], `usage ratio ${String(usageRatio)} must be unavailable`);
  }

  assert.doesNotMatch(
    columnsSource,
    /limit\.status === 'exhausted' \? 1|limit\.usageRatio \?\? 1/,
    'generic quota rows must not fabricate 100% usage for missing ratios'
  );
});

test('hidden quota selection removes only quota fields from the real query', () => {
  const visibleQuery = buildQueryChannelsQuery({ quota: true, tags: false });
  const hiddenQuery = buildQueryChannelsQuery({ quota: false, tags: false });

  assert.match(visibleQuery, /providerQuotaStatus/);
  assert.doesNotMatch(hiddenQuery, /providerQuotaStatus/);
  assert.match(hiddenQuery, /supportedModels/);
  assert.match(hiddenQuery, /liveLimiterStats/);
});

test('more than five normalized limits expose the remaining rows for expansion', () => {
  const limits = parseQuotaLimits({
    _limits: [
      { type: 'token', window: '5h', usageRatio: 0.1, status: 'available' },
      { type: 'token', window: '7d', usageRatio: 0.2, status: 'available' },
      { type: 'token', window: '30d', usageRatio: 0.3, status: 'available' },
      { type: 'token', window: 'daily', usageRatio: 0.4, status: 'available' },
      { type: 'token', window: 'weekly', usageRatio: 0.5, status: 'available' },
      { type: 'token', window: 'monthly', usageRatio: 0.6, status: 'available' },
    ],
  });
  const columns = columnsSource.slice(columnsSource.indexOf('const QuotaCell'), columnsSource.indexOf('QuotaCell.displayName'));

  assert.equal(limits.length, 6);
  assert.deepEqual(limits.slice(0, 5).map((limit) => limit.window), ['5h', '7d', '30d', 'daily', 'weekly']);
  assert.deepEqual(limits.map((limit) => limit.window), ['5h', '7d', '30d', 'daily', 'weekly', 'monthly']);
  assert.match(columns, /const visibleLimits = isExpanded \? limits : limits\.slice\(0, QUOTA_VISIBLE_LIMIT\)/);
  assert.match(columns, /const hiddenCount = limits\.length - QUOTA_VISIBLE_LIMIT/);
});
