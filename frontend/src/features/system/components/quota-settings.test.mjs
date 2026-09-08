import assert from 'node:assert/strict';
import { readFileSync, mkdtempSync, cpSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { execFileSync } from 'node:child_process';
import test from 'node:test';

const componentsDir = import.meta.dirname;
const srcRoot = join(componentsDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

// Extract a top-level function's full source (brace-balanced scan).
function extractFunction(source, name) {
  const start = source.indexOf(`function ${name}(`);
  assert.notEqual(start, -1, `function ${name} not found`);
  const bodyStart = source.indexOf('{', start);
  let depth = 0;
  for (let i = bodyStart; i < source.length; i++) {
    if (source[i] === '{') depth++;
    else if (source[i] === '}') {
      depth--;
      if (depth === 0) return source.slice(start, i + 1);
    }
  }
  return source.slice(start);
}

function assertMutationErrorToastHandled(hookSource, label) {
  assert.match(
    hookSource,
    /onError:\s*\(\)\s*=>\s*\{\s*toast\.error\(i18n\.t\('common\.errors\.systemUpdateFailed'\)\)/,
    `${label}: a rejected mutation must surface the error toast handler`
  );
}

const source = read('features/system/components/quota-settings.tsx');
const systemData = read('features/system/data/system.ts');
const enSystem = JSON.parse(read('locales/en/system.json'));
const zhSystem = JSON.parse(read('locales/zh-CN/system.json'));

const ROUTING_KEYS = [
  'system.quota.routing.title',
  'system.quota.routing.description',
  'system.quota.routing.mode.label',
  'system.quota.routing.mode.description',
  'system.quota.routing.modes.IGNORE_QUOTA',
  'system.quota.routing.modes.IGNORE_QUOTA.description',
  'system.quota.routing.modes.REMOVE_ON_EXHAUSTED',
  'system.quota.routing.modes.REMOVE_ON_EXHAUSTED.description',
  'system.quota.routing.modes.BACKPRESSURE',
  'system.quota.routing.modes.BACKPRESSURE.description',
];

test('routing card renders exactly the three global default modes', () => {
  assert.match(source, /<SelectItem value='IGNORE_QUOTA'>/);
  assert.match(source, /<SelectItem value='REMOVE_ON_EXHAUSTED'>/);
  assert.match(source, /<SelectItem value='BACKPRESSURE'>/);
  assert.match(source, /system\.quota\.routing\.modes\.\$\{routingMode\}\.description/, 'per-mode description follows the selected mode');
});

test('legacy enforcement UI and hooks are gone from the card', () => {
  assert.doesNotMatch(source, /ChannelMultiSelect|useQuotaEnforcement|QuotaEnforcement|allowedChannelIDs/);
  assert.doesNotMatch(source, /EXHAUSTED_ONLY|DE_PRIORITIZE/);
  assert.doesNotMatch(source, /system\.quota\.mode\.|system\.quota\.enabled\.|system\.quota\.enforcement\./);
});

test('selection is persisted through the routing mutation hook', () => {
  assert.match(source, /const updateQuotaRoutingSettings = useUpdateQuotaRoutingSettings\(\);/);
  assert.match(source, /await updateQuotaRoutingSettings\.mutateAsync\(\{ defaultMode: routingMode \}\)/);
  assert.match(source, /disabled=\{updateQuotaRoutingSettings\.isPending\}/, 'save button tracks mutation pending state');
});

test('routing query is scope-gated in the data layer', () => {
  const hook = extractFunction(systemData, 'useQuotaRoutingSettings');
  assert.match(hook, /enabled: hasSystemScope\('read_settings'\)/);
});

test('card degrades gracefully when settings data is absent (no read_settings scope)', () => {
  // Form state seeds only from fetched data, so a scope-disabled query leaves the card on its default mode.
  assert.match(source, /const \{ data: routingSettings, isLoading: isRoutingSettingsLoading \} = useQuotaRoutingSettings\(\);/);
  assert.match(source, /if \(routingSettings\) \{\s*setRoutingMode\(routingSettings\.defaultMode\);\s*\}/);
  // Loading gate uses isLoading (false for a disabled query), not isPending, so the card still renders.
  assert.match(source, /isRoutingSettingsLoading \|\| isCollectionSettingsLoading/);
  assert.doesNotMatch(source, /isRoutingSettingsPending/);
});

test('mutation rejection surfaces the error toast handler', () => {
  assertMutationErrorToastHandled(extractFunction(systemData, 'useUpdateQuotaRoutingSettings'), 'useUpdateQuotaRoutingSettings');
  // Invalidation keeps the card in sync after a successful update.
  const hook = extractFunction(systemData, 'useUpdateQuotaRoutingSettings');
  assert.match(hook, /invalidateQueries\(\{ queryKey: \['quotaRoutingSettings'\] \}\)/);
});

test('routing locale keys exist in both locales', () => {
  for (const key of ROUTING_KEYS) {
    assert.ok(enSystem[key], `en/system.json missing ${key}`);
    assert.ok(zhSystem[key], `zh-CN/system.json missing ${key}`);
  }
});

test('zh-CN mode labels match the specified copy', () => {
  assert.equal(zhSystem['system.quota.routing.modes.IGNORE_QUOTA'], '忽略限额状态');
  assert.equal(zhSystem['system.quota.routing.modes.REMOVE_ON_EXHAUSTED'], '达到限额后摘除');
  assert.equal(zhSystem['system.quota.routing.modes.BACKPRESSURE'], '基于用量和时长限流');
});

test('legacy enforcement locale keys removed from both locales', () => {
  for (const [name, obj] of [['en', enSystem], ['zh-CN', zhSystem]]) {
    const legacy = Object.keys(obj).filter(
      (key) =>
        key === 'system.quota.title' ||
        key === 'system.quota.description' ||
        key.startsWith('system.quota.enabled.') ||
        key.startsWith('system.quota.mode.') ||
        key.startsWith('system.quota.enforcement.')
    );
    assert.deepEqual(legacy, [], `${name}/system.json still has legacy quota enforcement keys`);
  }
});

test('negative control: a mutation hook without an error handler is rejected by the validator', () => {
  // Simulates a hook that swallows rejections: the same assertion used above must fail for it.
  const mock = "useMutation({ mutationFn: async () => true, onSuccess: () => {} });";
  assert.throws(() => assertMutationErrorToastHandled(mock, 'mock'), /error toast handler/);
});

test('sandbox regression: stripping the error handler from a copy makes this suite fail', { skip: process.env.SKIP_SANDBOX_TEST }, () => {
  // Copies the suite into a temp fixture where useUpdateQuotaRoutingSettings has no onError
  // handler, then asserts node --test reports the rejection-handling failure there.
  const sandbox = mkdtempSync(join(tmpdir(), 'quota-settings-'));
  try {
    for (const rel of [
      'features/system/components/quota-settings.tsx',
      'features/system/data/system.ts',
      'locales/en/system.json',
      'locales/zh-CN/system.json',
    ]) {
      cpSync(join(srcRoot, rel), join(sandbox, rel));
    }
    const sandboxSystem = join(sandbox, 'features/system/data/system.ts');
    // Strip the handler INSIDE useUpdateQuotaRoutingSettings only: system.ts has many
    // identical onError blocks, so a whole-file replace would hit the wrong hook.
    const original = read('features/system/data/system.ts');
    const hook = extractFunction(original, 'useUpdateQuotaRoutingSettings');
    const strippedHook = hook.replace(
      /onError: \(\) => \{\s*toast\.error\(i18n\.t\('common\.errors\.systemUpdateFailed'\)\);\s*\}/,
      ''
    );
    assert.notEqual(strippedHook, hook, 'sandbox mutation did not change the fixture');
    writeFileSync(sandboxSystem, original.replace(hook, strippedHook));
    cpSync(join(componentsDir, 'quota-settings.test.mjs'), join(sandbox, 'features/system/components/quota-settings.test.mjs'));

    let output = '';
    try {
      // Strip the outer runner's child-context vars: a nested `node --test` that
      // inherits NODE_TEST_CONTEXT assumes a parent runner owns reporting and exits 0.
      const { NODE_TEST_CONTEXT: _runnerContext, NODE_TEST_WORKER_ID: _runnerWorker, ...innerEnv } = process.env;
      execFileSync(process.execPath, ['--test', 'features/system/components/quota-settings.test.mjs'], {
        cwd: sandbox,
        encoding: 'utf8',
        env: { ...innerEnv, SKIP_SANDBOX_TEST: '1' },
      });
      assert.fail('sandboxed suite unexpectedly passed without the error handler');
    } catch (err) {
      output = err.stdout || String(err);
    }
    assert.match(output, /mutation rejection surfaces the error toast handler/, 'sandbox must fail exactly on the rejection-handling test');
    assert.match(output, /fail 1\b/, 'exactly one test should fail in the sandbox');
  } finally {
    rmSync(sandbox, { recursive: true, force: true });
  }
});
