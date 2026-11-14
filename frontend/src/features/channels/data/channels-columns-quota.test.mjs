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

const { parseAPIKeyQuotas, parseQuotaBalances, parseQuotaLimits, applyDisabledKeyOwnershipSync } = await loadModule(
  'features/system/data/quotas.ts',
  (s) => s.replace(/export function useProviderQuotaStatuses[\s\S]*$/m, '')
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

test('normalized limits: invalid reset timestamps fail closed', () => {
  assert.deepEqual(
    parseQuotaLimits({
      _limits: [{ type: 'token', window: '7d', usageRatio: 0.4, status: 'available', ready: true, nextResetAt: 'not-a-date' }],
    }),
    []
  );
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

test('normalized balances: parser returns every _balances entry without fabrication', () => {
  const balances = parseQuotaBalances({
    _balances: [
      { currency: 'CNY', total: 110, granted: 10, topped_up: 100 },
      { currency: 'USD', total: 5 },
    ],
  });

  assert.equal(balances.length, 2);
  assert.deepEqual(balances[0], { account: undefined, currency: 'CNY', total: 110, granted: 10, topped_up: 100 });
  assert.deepEqual(balances[1], { account: undefined, currency: 'USD', total: 5, granted: undefined, topped_up: undefined });
});

test('normalized balances: malformed entries fail closed', () => {
  for (const rawBalances of [null, {}, 'broken', 42]) {
    assert.deepEqual(parseQuotaBalances({ _balances: rawBalances }), [], `malformed _balances value ${String(rawBalances)} must be unavailable`);
  }

  const balances = parseQuotaBalances({
    _balances: [null, 42, { currency: 'CNY' }, { total: '110' }, { total: 10, granted: 'x', topped_up: 5 }],
  });
  assert.deepEqual(balances, [{ account: undefined, currency: undefined, total: 10, granted: undefined, topped_up: 5 }]);
});

function apiKeyQuotaFixture(overrides = {}) {
  return {
    ref: 'opaque-key-one',
    index: 1,
    suffix: 'same',
    disabled: false,
    quotaDisabled: false,
    status: 'available',
    ready: true,
    quotaData: {},
    ...overrides,
  };
}

test('per-key quotas: every key retains all normalized windows and its provider payload', () => {
  const resetAt = '2026-11-13T00:00:00Z';
  const windows = (ratios) =>
    ['5h', 'weekly', 'monthly'].map((window, index) => ({
      type: 'token',
      window,
      usageRatio: ratios[index],
      status: ratios[index] === 1 ? 'exhausted' : 'available',
      ready: ratios[index] !== 1,
      periodStart: '2026-11-12T00:00:00Z',
      nextResetAt: resetAt,
    }));
  const firstPayload = { level: 'PRO', rows: [{ window: 'five_hour', remaining: 50, usage: 100 }], _limits: windows([1, 0.2, 0.3]) };
  const secondPayload = { _limits: windows([0.12, 0.3, 0.35]) };
  const accounts = parseAPIKeyQuotas({
    _api_key_quotas: [
      apiKeyQuotaFixture({
        disabled: true,
        quotaDisabled: true,
        status: 'exhausted',
        ready: false,
        nextResetAt: resetAt,
        quotaData: firstPayload,
      }),
      apiKeyQuotaFixture({ ref: 'opaque-key-two', index: 2, quotaData: secondPayload }),
    ],
  });

  assert.equal(accounts.length, 2);
  assert.deepEqual(
    accounts.map((account) => account.index),
    [1, 2]
  );
  assert.deepEqual(
    accounts.map((account) => account.limits.map((limit) => limit.window)),
    [
      ['5h', 'weekly', 'monthly'],
      ['5h', 'weekly', 'monthly'],
    ]
  );
  assert.deepEqual(
    accounts.map((account) => account.limits.map((limit) => limit.usageRatio)),
    [
      [1, 0.2, 0.3],
      [0.12, 0.3, 0.35],
    ]
  );
  assert.equal(accounts[0].disabled, true);
  assert.equal(accounts[0].quotaDisabled, true);
  assert.equal(accounts[0].nextResetAt, resetAt);
  assert.equal(accounts[0].limits[0].periodStart, '2026-11-12T00:00:00Z');
  assert.deepEqual(accounts[0].quotaData, firstPayload);
  assert.deepEqual(accounts[1].quotaData, secondPayload);
});

test('per-key quotas: malformed rows are omitted without dropping valid neighbors', () => {
  const accounts = parseAPIKeyQuotas({
    _api_key_quotas: [
      apiKeyQuotaFixture(),
      null,
      42,
      [],
      apiKeyQuotaFixture({ ref: ' ' }),
      apiKeyQuotaFixture({ index: 0 }),
      apiKeyQuotaFixture({ index: 1.5 }),
      apiKeyQuotaFixture({ status: 'invalid' }),
      apiKeyQuotaFixture({ ready: 'true' }),
      apiKeyQuotaFixture({ disabled: 'false' }),
      apiKeyQuotaFixture({ quotaDisabled: undefined }),
      apiKeyQuotaFixture({ nextResetAt: 'not-a-date' }),
      apiKeyQuotaFixture({ ref: 'opaque-key-last', index: 7 }),
    ],
  });

  assert.deepEqual(
    accounts.map((account) => account.ref),
    ['opaque-key-one', 'opaque-key-last']
  );
  assert.deepEqual(
    accounts.map((account) => account.index),
    [1, 7]
  );
});

test('per-key ownership: a manually retained key is no longer quota-owned after the mutation', () => {
  const parked = apiKeyQuotaFixture({ disabled: true, quotaDisabled: true, status: 'exhausted', ready: false });
  const healthy = apiKeyQuotaFixture({ ref: 'opaque-key-two', index: 2 });

  // Before the takeover: the persisted view from collection says the key is
  // quota-owned, and the current records agree.
  const current = applyDisabledKeyOwnershipSync([parked, healthy], [{ key: 'sk-one', expiresAt: null, quotaDisabled: true }], {
    'sk-one': 'opaque-key-one',
  });
  assert.equal(current[0].disabled, true, 'the quota-owned record stays as persisted');
  assert.equal(current[0].quotaDisabled, true);

  // The operator keeps the key disabled manually: the record transfers. The
  // persisted _api_key_quotas is only recomputed on the next collection, so
  // the popover must derive the new ownership from the current records.
  const afterTakeover = applyDisabledKeyOwnershipSync([parked, healthy], [{ key: 'sk-one', expiresAt: null, quotaDisabled: false }], {
    'sk-one': 'opaque-key-one',
  });
  assert.equal(afterTakeover[0].disabled, true, 'the key stays disabled');
  assert.equal(afterTakeover[0].quotaDisabled, false, 'the manual record is no longer quota-owned');
  assert.equal(afterTakeover[1].quotaDisabled, false, 'an untouched key keeps its persisted ownership');

  // The reverse direction: re-enabling a key the operator had kept disabled
  // must restore the quota-owned flag a later collection would stamp.
  const reOwned = applyDisabledKeyOwnershipSync(
    [{ ...parked, disabled: false, quotaDisabled: false }],
    [{ key: 'sk-one', expiresAt: null, quotaDisabled: true }],
    { 'sk-one': 'opaque-key-one' }
  );
  assert.equal(reOwned[0].quotaDisabled, true);

  // An expired record no longer disables its key, so the row is usable again
  // even when the persisted view still says parked.
  const expired = applyDisabledKeyOwnershipSync(
    [parked, healthy],
    [{ key: 'sk-one', expiresAt: new Date(Date.now() - 60000).toISOString(), quotaDisabled: false }],
    { 'sk-one': 'opaque-key-one' }
  );
  assert.equal(expired[0].disabled, false, 'an expired record no longer disables the key');
  assert.equal(expired[0].quotaDisabled, false, 'an expired record keeps no ownership');
});

test('per-key ownership: an unresolvable digest keeps the persisted row untouched', () => {
  const parked = apiKeyQuotaFixture({ disabled: true, quotaDisabled: true, status: 'exhausted', ready: false });
  const healthy = apiKeyQuotaFixture({ ref: 'opaque-key-two', index: 2 });

  // The digest could not be computed (crypto.subtle missing or the digest call
  // failed): the absence of a matching record proves nothing, so the stale row
  // keeps its stored usability and ownership.
  const unresolved = applyDisabledKeyOwnershipSync(
    [parked, healthy],
    [{ key: 'sk-one', expiresAt: null, quotaDisabled: true }],
    {},
    new Set(['sk-one'])
  );
  assert.equal(unresolved[0].disabled, true, 'the unresolvable key keeps its stored usability');
  assert.equal(unresolved[0].quotaDisabled, true, 'the unresolvable key keeps its stored ownership');
  assert.equal(unresolved[1].disabled, false, 'an unaffected row is not touched');
  assert.equal(unresolved[1].quotaDisabled, false);

  // Once the digest resolves and no active record matches, the row clears.
  const resolved = applyDisabledKeyOwnershipSync([parked, healthy], [], { 'sk-one': 'opaque-key-one' });
  assert.equal(resolved[0].disabled, false, 'a resolved absence clears the stale row');
  assert.equal(resolved[0].quotaDisabled, false);
});

test('per-key ownership: an unavailable disable list leaves the persisted rows untouched', () => {
  const parked = apiKeyQuotaFixture({ disabled: true, quotaDisabled: true, status: 'exhausted', ready: false });
  const healthy = apiKeyQuotaFixture({ ref: 'opaque-key-two', index: 2 });

  // A null/undefined list means the records are unavailable (e.g. the caller
  // lacks write_channels), not that the channel has none: the persisted rows
  // must stay exactly as stored, never cleared.
  for (const unavailable of [null, undefined]) {
    const kept = applyDisabledKeyOwnershipSync([parked, healthy], unavailable, {}, new Set(['sk-one']));
    assert.equal(kept[0].disabled, true, `a ${String(unavailable)} list keeps the stored usability`);
    assert.equal(kept[0].quotaDisabled, true, `a ${String(unavailable)} list keeps the stored ownership`);
    assert.equal(kept[1].disabled, false, 'the healthy row is untouched');
    assert.equal(kept[1].quotaDisabled, false);
  }
});

test('per-key ownership: an empty disable list is a real answer and clears stale rows', () => {
  const parked = apiKeyQuotaFixture({ disabled: true, quotaDisabled: true, status: 'exhausted', ready: false });

  // Unlike null, an empty list is what the server returned: no active records,
  // so a row that still says parked must clear.
  const cleared = applyDisabledKeyOwnershipSync([parked], [], { 'sk-one': 'opaque-key-one' }, new Set(['sk-unrelated']));
  assert.equal(cleared[0].disabled, false, 'an empty list clears the stale row');
  assert.equal(cleared[0].quotaDisabled, false, 'an empty list clears the stale ownership');
});

test('per-key ownership: an unresolved digest from another channel does not protect this channel', () => {
  const parked = apiKeyQuotaFixture({ disabled: true, quotaDisabled: true, status: 'exhausted', ready: false });

  // Channel A's digest could not be computed, but this channel has no disable
  // records of its own: the global unresolved set says nothing about this
  // channel, so its stale row clears.
  const cleared = applyDisabledKeyOwnershipSync([parked], [], {}, new Set(['sk-channel-a']));
  assert.equal(cleared[0].disabled, false, 'an unrelated unresolved digest does not protect the row');
  assert.equal(cleared[0].quotaDisabled, false);

  // The same digest failing for this channel's own key does protect it.
  const protectedRow = applyDisabledKeyOwnershipSync(
    [parked],
    [{ key: 'sk-one', expiresAt: null, quotaDisabled: true }],
    {},
    new Set(['sk-one'])
  );
  assert.equal(protectedRow[0].disabled, true, 'an own-channel unresolved digest keeps the stored usability');
  assert.equal(protectedRow[0].quotaDisabled, true, 'an own-channel unresolved digest keeps the stored ownership');
});

test('per-key quotas: unknown and failed accounts stay unknown without invented usage', () => {
  const accounts = parseAPIKeyQuotas({
    _api_key_quotas: [
      apiKeyQuotaFixture({ status: 'unknown', ready: false }),
      apiKeyQuotaFixture({ ref: 'opaque-failed-key', index: 2, status: 'unknown', ready: false, errorCode: 'check_failed' }),
    ],
  });

  assert.deepEqual(accounts.map((account) => account.status), ['unknown', 'unknown']);
  assert.deepEqual(accounts.map((account) => account.ready), [false, false]);
  assert.deepEqual(accounts.map((account) => account.errorCode), [undefined, 'check_failed']);
  for (const account of accounts) {
    assert.deepEqual(account.limits, []);
    assert.deepEqual(account.balances, []);
  }
});

test('per-key quotas: balances retain their own key and currency instead of being summed', () => {
  const accounts = parseAPIKeyQuotas({
    _api_key_quotas: [
      apiKeyQuotaFixture({ quotaData: { _balances: [{ currency: 'CNY', total: 110, granted: 10, topped_up: 100 }, { currency: 'USD', total: 5 }] } }),
      apiKeyQuotaFixture({ ref: 'opaque-key-two', index: 2, quotaData: { _balances: [{ currency: 'CNY', total: 2 }] } }),
    ],
  });

  assert.deepEqual(accounts.map((account) => account.balances.map(({ currency, total }) => ({ currency, total }))), [
    [{ currency: 'CNY', total: 110 }, { currency: 'USD', total: 5 }], [{ currency: 'CNY', total: 2 }],
  ]);
  assert.equal(accounts[0].balances[0].granted, 10);
  assert.equal(accounts[0].balances[0].topped_up, 100);
  assert.deepEqual(accounts.map((account) => account.limits), [[], []]);
});

test('per-key quotas: duplicate suffixes remain distinct and short keys have no visible suffix', () => {
  const accounts = parseAPIKeyQuotas({
    _api_key_quotas: [
      apiKeyQuotaFixture(),
      apiKeyQuotaFixture({ ref: 'opaque-key-two', index: 2 }),
      apiKeyQuotaFixture({ ref: 'opaque-short-key', index: 3, suffix: '' }),
    ],
  });

  assert.deepEqual(accounts.map(({ ref, index, suffix }) => ({ ref, index, suffix })), [
    { ref: 'opaque-key-one', index: 1, suffix: 'same' },
    { ref: 'opaque-key-two', index: 2, suffix: 'same' },
    { ref: 'opaque-short-key', index: 3, suffix: '' },
  ]);
});

test('per-key quotas: absent or malformed account containers preserve the legacy empty list', () => {
  for (const quotaData of [undefined, null, 42, 'invalid', {}, { _api_key_quotas: null }, { _api_key_quotas: {} }]) {
    assert.deepEqual(parseAPIKeyQuotas(quotaData), []);
  }
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
    'zenmux_video',
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
  assert.match(
    columnsSource,
    /parseQuotaBalances/,
    'the channel table should consume the shared provider-neutral normalized-balance parser'
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
    /providerType === 'deepseek'/,
    /balance_infos/,
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

    assert.equal(limits.length, 0, `usage ratio ${String(usageRatio)} must be unavailable to the quota cell`);
  }
});

test('hidden quota selection keeps routing status fields for the channel name', () => {
  const visibleQuery = buildQueryChannelsQuery({ quota: true, tags: false });
  const hiddenQuery = buildQueryChannelsQuery({ quota: false, tags: false });

  assert.match(visibleQuery, /providerQuotaStatus/);
  assert.match(hiddenQuery, /providerQuotaStatus/);
  assert.match(hiddenQuery, /status/);
  assert.doesNotMatch(hiddenQuery, /nextResetAt/);
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

test('the search quota reaches the table with its production label', () => {
  assert.match(
    columnsSource,
    /search_hourly:\s*'quota\.window\.search_hourly'/,
    'the table must map the search window through the production label map'
  );

  const limits = parseQuotaLimits({
    _limits: [{ type: 'search', window: 'search_hourly', status: 'available', ready: true, usageRatio: 0.4 }],
  });
  assert.deepEqual(
    limits.map(({ type, window, usageRatio }) => ({ type, window, usageRatio })),
    [{ type: 'search', window: 'search_hourly', usageRatio: 0.4 }]
  );
});

test('channels table container allows horizontal scroll and wraps in min-w-max without table-fixed', () => {
  const tableSource = read('features/channels/components/channels-table.tsx');
  assert.doesNotMatch(tableSource, /overflow-x-hidden/, 'table container must not disable horizontal scroll');
  assert.match(tableSource, /min-w-max/, 'table must be wrapped in min-w-max to preserve column widths');
  assert.doesNotMatch(tableSource, /table-fixed/, 'table must not be table-fixed which crushes columns to 0');
});

test('channel columns have minimum widths and are not hidden by responsive display gates', () => {
  const columnsSource = read('features/channels/components/channels-columns.tsx');

  const columnMeta = (columnId) => {
    const idIndex = columnsSource.indexOf("id: '" + columnId + "'");
    const keyIndex = idIndex === -1 ? columnsSource.indexOf("accessorKey: '" + columnId + "'") : idIndex;
    assert.notEqual(keyIndex, -1, columnId + ' column should exist');
    const columnEnd = columnsSource.indexOf('\n    },', keyIndex);
    assert.notEqual(columnEnd, -1, columnId + ' column boundary should be found');
    const metaIndex = columnsSource.indexOf('meta: {', keyIndex);
    assert.notEqual(metaIndex, -1, columnId + ' column should define meta');
    assert.ok(metaIndex < columnEnd, columnId + ' meta should belong to the column');
    const classNameIndex = columnsSource.indexOf('className:', metaIndex);
    assert.notEqual(classNameIndex, -1, columnId + ' column should define meta.className');
    assert.ok(classNameIndex < columnEnd, columnId + ' className should belong to the column');
    const end = columnsSource.indexOf('\n', classNameIndex);
    return columnsSource.slice(classNameIndex, end);
  };

  for (const columnId of ['quota', 'tags', 'proxy', 'createdAt']) {
    const meta = columnMeta(columnId);
    assert.doesNotMatch(meta, /\bhidden\b/, columnId + ' must not be hidden by responsive display gate');
    assert.doesNotMatch(meta, /(?:xl|2xl):table-cell/, columnId + ' must not be gated by a responsive breakpoint');
  }

  for (const columnId of ['name', 'provider', 'status', 'supportedModels']) {
    const meta = columnMeta(columnId);
    assert.doesNotMatch(meta, /min-w-0/, columnId + ' must have a non-zero minimum width');
    assert.match(meta, /min-w-\d+/, columnId + ' must have an explicit min-w-* class to prevent collapse');
  }
});

test('channel percentage column widths sum to 100', () => {
  const columnsSource = read('features/channels/components/channels-columns.tsx');
  const percentages = [...columnsSource.matchAll(/w-\[([0-9.]+)%\]/g)].map((match) => Number(match[1]));

  assert.ok(percentages.length > 0, 'expected percentage-width channel columns');
  const total = percentages.reduce((sum, value) => sum + value, 0);
  assert.equal(total, 100, 'channel column percentages must sum to 100, got ' + total);
});

test('supported models cell bounds badge container width to prevent horizontal stretching', () => {
  const columnsSource = read('features/channels/components/channels-columns.tsx');
  assert.match(
    columnsSource,
    /max-w-(?:72|80|xs|sm)[^']*flex-wrap/,
    'supported models badge container must have a max-width wrapper so badges wrap instead of stretching the table'
  );
});
