import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import ts from 'typescript';

const dataDir = import.meta.dirname;
const srcRoot = join(dataDir, '..', '..', '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

const source = read('features/channels/data/auto-disable.ts');
const transpiled = ts.transpileModule(source, {
  compilerOptions: {
    module: ts.ModuleKind.ESNext,
    target: ts.ScriptTarget.ES2023,
  },
}).outputText;
const moduleUrl = `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`;
const {
  RECOMMENDED_GLOBAL_AUTO_DISABLE_RULES,
  availabilityPoliciesPayload,
  effectiveAutoDisableMode,
  serializeApiKeyAutoDisableRules,
  toApiKeyAutoDisableRuleFormValues,
} = await import(moduleUrl);

test('effectiveAutoDisableMode keeps explicit off even without rules', () => {
  assert.equal(effectiveAutoDisableMode({ apiKeyAutoDisableMode: 'off', apiKeyAutoDisableRules: [] }), 'off');
  assert.equal(effectiveAutoDisableMode({ apiKeyAutoDisableMode: 'off' }), 'off');
});

test('effectiveAutoDisableMode infers custom from legacy rules and inherit from empty policies', () => {
  assert.equal(
    effectiveAutoDisableMode({
      apiKeyAutoDisableRules: [{ times: 1, action: 'permanent_disable', statusCodes: [401] }],
    }),
    'custom'
  );
  assert.equal(effectiveAutoDisableMode({}), 'inherit');
  assert.equal(effectiveAutoDisableMode(null), 'inherit');
});

test('effectiveAutoDisableMode treats empty custom as inherit', () => {
  assert.equal(effectiveAutoDisableMode({ apiKeyAutoDisableMode: 'custom', apiKeyAutoDisableRules: [] }), 'inherit');
  assert.equal(
    effectiveAutoDisableMode({
      apiKeyAutoDisableMode: 'custom',
      apiKeyAutoDisableRules: [{ times: 2, action: 'temporary_disable', statusCodes: [429], disableDurationMinutes: 30 }],
    }),
    'custom'
  );
});

test('availabilityPoliciesPayload clears rules on inherit and keeps them on off', () => {
  const rules = toApiKeyAutoDisableRuleFormValues([{ statusCodes: [401], times: 3, action: 'permanent_disable' }]);

  const inherited = availabilityPoliciesPayload({ stream: 'unlimited', mode: 'inherit', rules });
  assert.equal(inherited.apiKeyAutoDisableMode, 'inherit');
  assert.equal(inherited.apiKeyAutoDisableRules, null);
  assert.equal(inherited.emptiedCustom, false);

  const off = availabilityPoliciesPayload({ stream: 'unlimited', mode: 'off', rules });
  assert.equal(off.apiKeyAutoDisableMode, 'off');
  assert.deepEqual(off.apiKeyAutoDisableRules, [
    {
      statusCodes: [401],
      keywordPatterns: [],
      times: 3,
      action: 'permanent_disable',
      disableDurationMinutes: null,
      disableUntilCron: null,
      disableUntilTimezone: null,
    },
  ]);
});

test('availabilityPoliciesPayload downgrades empty custom to inherit', () => {
  const payload = availabilityPoliciesPayload({ mode: 'custom', rules: [] });
  assert.equal(payload.emptiedCustom, true);
  assert.equal(payload.apiKeyAutoDisableMode, 'inherit');
  assert.equal(payload.apiKeyAutoDisableRules, null);
});

test('recommended global rules are 401x3 permanent and 429x3 temporary 30 minutes', () => {
  assert.deepEqual(RECOMMENDED_GLOBAL_AUTO_DISABLE_RULES, [
    { statusCodes: [401], times: 3, action: 'permanent_disable' },
    { statusCodes: [429], times: 3, action: 'temporary_disable', disableDurationMinutes: 30 },
  ]);
});

test('serialize strips schedule fields that do not belong to the selected action', () => {
  const serialized = serializeApiKeyAutoDisableRules([
    {
      statusCodes: [429],
      keywordPatterns: [' quota ', ''],
      times: 2,
      action: 'temporary_disable',
      disableDurationMinutes: 15,
      disableUntilCron: '0 0 * * *',
      disableUntilTimezone: 'Asia/Shanghai',
    },
  ]);

  assert.deepEqual(serialized, [
    {
      statusCodes: [429],
      keywordPatterns: ['quota'],
      times: 2,
      action: 'temporary_disable',
      disableDurationMinutes: 15,
      disableUntilCron: null,
      disableUntilTimezone: null,
    },
  ]);
});

test('channel and retry-policy GraphQL selections load the new auto-disable fields', () => {
  const channelsData = read('features/channels/data/channels.ts');
  const systemData = read('features/system/data/system.ts');
  const retrySettings = read('features/system/components/retry-settings.tsx');
  const availabilityDialog = read('features/channels/components/channels-availability-dialog.tsx');

  const policySelections = channelsData.match(/policies\s*\{[\s\S]*?\}/g) ?? [];
  assert.ok(policySelections.length >= 5, 'channel queries should select policies in list/detail/create/update paths');
  for (const selection of policySelections) {
    assert.match(selection, /apiKeyAutoDisableMode/, 'channel policies selection should include mode');
    assert.match(selection, /apiKeyAutoDisableRules/, 'channel policies selection should include rules');
  }

  assert.match(systemData, /autoDisableChannel \{\s*enabled\s*rules \{/, 'retryPolicy query should request rules, not statuses');
  assert.doesNotMatch(systemData, /autoDisableChannel \{\s*enabled\s*statuses/, 'retryPolicy query must not request removed statuses');
  assert.match(retrySettings, /RECOMMENDED_GLOBAL_AUTO_DISABLE_RULES/, 'retry settings should insert the shared recommended rules');
  assert.match(retrySettings, /allowDelete=\{false\}/, 'global editor must not offer credential deletion');
  assert.match(availabilityDialog, /availabilityPoliciesPayload/, 'availability save must use the shared payload helper');
  assert.match(availabilityDialog, /off\.savedRulesHint/, 'off mode must show saved rules as inactive');
});

test('auto-disable mode and recommended-rule copy exist in both locales', () => {
  const enChannels = JSON.parse(read('locales/en/channels.json'));
  const zhChannels = JSON.parse(read('locales/zh-CN/channels.json'));
  const enSystem = JSON.parse(read('locales/en/system.json'));
  const zhSystem = JSON.parse(read('locales/zh-CN/system.json'));

  for (const key of [
    'channels.dialogs.availability.modes.inherit',
    'channels.dialogs.availability.modes.custom',
    'channels.dialogs.availability.modes.off',
    'channels.dialogs.availability.off.savedRulesHint',
    'channels.dialogs.availability.custom.emptyPrompt',
  ]) {
    assert.ok(enChannels[key], `${key} missing in en`);
    assert.ok(zhChannels[key], `${key} missing in zh-CN`);
  }

  assert.ok(enSystem['system.retry.autoDisableChannel.recommendedRules']);
  assert.ok(zhSystem['system.retry.autoDisableChannel.recommendedRules']);
  assert.doesNotMatch(JSON.stringify(enSystem), /autoDisableChannel\.statuses\.(label|add|empty)/);
});
