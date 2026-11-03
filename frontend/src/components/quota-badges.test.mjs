import React from 'react';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import { renderToStaticMarkup } from 'react-dom/server';
import ts from 'typescript';

const componentsDir = import.meta.dirname;
const srcRoot = join(componentsDir, '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

// Isolate the Codex render branch (between the Codex and XAI branch markers)
// so assertions about the usage bars cannot bleed into neighboring providers.
function isolateCodexBlock(source) {
  const start = source.indexOf("{channel.type === 'codex' &&");
  const end = source.indexOf("{channel.type === 'xai_subscription' &&", start);

  assert.ok(start !== -1, 'Codex render branch should exist in quota-badges source');
  assert.ok(end !== -1 && end > start, 'XAI render branch should follow the Codex branch');

  return source.slice(start, end);
}

test('Codex usage windows render as combined usage and time bars', () => {
  const quotaBadges = read('components/quota-badges.tsx');
  const codexBlock = isolateCodexBlock(quotaBadges);

  // Each window has one shared bar whose fill reflects usage and whose marker
  // reflects the reset-window elapsed time.
  assert.equal((codexBlock.match(/<UsageTimeBar\s/g) || []).length, 1, 'Codex normalized limits should use one combined bar');
  assert.match(codexBlock, /quota\.limits\s*\.filter\([\s\S]*?\.map\(\(limit,\s*index\)/, 'Codex bars should render normalized limits');
  assert.match(codexBlock, /limit\.window === '5h'/, 'Codex five-hour label should be selected from the normalized window');
  assert.match(codexBlock, /limit\.window === '7d'/, 'Codex seven-day label should be selected from the normalized window');
  assert.match(codexBlock, /WINDOW_LABEL_KEYS\[limit\.window\]/, 'Codex labels should resolve through the shared translation map');
  assert.match(codexBlock, /t\('quota\.label\.token_usage'\)/, 'Codex unknown windows should use the neutral quota label');
  assert.doesNotMatch(codexBlock, /quota\.label\.primary_window/);
  assert.doesNotMatch(codexBlock, /quota\.label\.secondary_window/);
});

test('quota popover has no standalone progress-bar renders', () => {
  const quotaBadges = read('components/quota-badges.tsx');

  assert.doesNotMatch(quotaBadges, /\bProgressBar\b/, 'all quota bars should use the combined UsageTimeBar component');
});

test('Command Code monthly hover matches the other windows', () => {
  const source = read('components/quota-badges.tsx');
  const commandCodeBlock = source.slice(
    source.indexOf('{isCommandCodeType(channel.type) &&'),
    source.indexOf("{channel.type === 'moonshot_coding' &&")
  );

  assert.match(commandCodeBlock, /monthlyDurationPct[\s\S]*quota\.label\.time_elapsed/);
  assert.doesNotMatch(commandCodeBlock, /quota\.label\.commandcode\.monthly_remaining/);
  assert.doesNotMatch(commandCodeBlock, /subscription_status/);
});

test('Codex reset can be attempted after a transient reset-list failure', () => {
  const quotaBadges = read('components/quota-badges.tsx');
  const codexBlock = isolateCodexBlock(quotaBadges);

  assert.match(
    codexBlock,
    /const canAttemptReset =\s*qd\._resets\?\.supported === true && \(Boolean\(qd\._resets\.error\) \|\| availableResetCount > 0\)/,
    'a supported provider should allow a fresh reset attempt when reset-list metadata failed'
  );
  assert.match(
    codexBlock,
    /disabled=\{isResetting \|\| !canAttemptReset\}/,
    'the reset button should use the retry-aware availability condition'
  );
});

test('Ollama badge derives percentage from the heaviest of the monthly/5h/weekly windows', () => {
  const source = read('components/quota-badges.tsx');
  const start = source.indexOf('} else if (isOllamaType(channel.type)) {');
  const end = source.indexOf('} else if (', start + 5);
  const percentBlock = source.slice(start, end);

  assert.match(
    percentBlock,
    /Math\.max\(\s*qd\?\.windows\?\.monthly\?\.usage_percent \?\? 0,\s*qd\?\.windows\?\.\[['"]5h['"]\]\?\.usage_percent \?\? 0,\s*qd\?\.windows\?\.weekly\?\.usage_percent \?\? 0\s*\)/,
    'ollama badge percentage should be the max of the monthly, 5h and weekly window usage'
  );
});

test('Ollama badge stays out of empty while purchased credits remain', () => {
  const source = read('components/quota-badges.tsx');
  const start = source.indexOf('} else if (isOllamaType(channel.type)) {');
  const end = source.indexOf('} else if (', start + 5);
  const percentBlock = source.slice(start, end);

  assert.match(
    percentBlock,
    /if \(\(qd\?\.credits\?\.purchased_usd \?\? 0\) > 0\) \{\s*percentage = Math\.min\(percentage, 94\);/,
    'purchased credits should cap the ollama badge percentage below the empty threshold'
  );
});

test('Ollama badge renders both the 5h and weekly windows with a reset countdown', () => {
  const source = read('components/quota-badges.tsx');
  const start = source.indexOf('{isOllamaType(channel.type) &&');
  const end = source.indexOf('{isCommandCodeType(channel.type) &&', start);
  const ollamaBlock = source.slice(start, end);

  assert.match(ollamaBlock, /QuotaWindow5h|'5h'/);
  assert.match(ollamaBlock, /'quota\.window\.5h'/);
  assert.match(ollamaBlock, /'quota\.window\.weekly'/);
  assert.match(ollamaBlock, /formatTimeToReset\(window\.reset_time\)/);
  assert.doesNotMatch(ollamaBlock, /durationPercent/);
});

test('quota window identifiers resolve to localized labels', () => {
  const quotaBadges = read('components/quota-badges.tsx');

  assert.match(quotaBadges, /payg:\s*'quota\.label\.token_usage'/);
  assert.match(quotaBadges, /credits:\s*'quota\.label\.credits_remaining'/);
  assert.match(quotaBadges, /'5h':\s*'quota\.window\.5h'/);
  assert.match(quotaBadges, /'7d':\s*'quota\.window\.7d'/);
  assert.match(quotaBadges, /limit\.window === 'primary' \|\| limit\.window === 'secondary'/);
});

test('Apertis duration marker shares timestamp validation', () => {
  const quotaBadges = read('components/quota-badges.tsx');
  const apertisStart = quotaBadges.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'apertis' &&");
  const apertisEnd = quotaBadges.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'charm_hyper' &&");

  assert.match(quotaBadges, /function getDurationPercent\([\s\S]*?Number\.isFinite\(start\)[\s\S]*?end <= start/);
  assert.match(
    quotaBadges.slice(apertisStart, apertisEnd),
    /durationPercent=\{getDurationPercent\(qd\.subscription\.cycle_start, qd\.subscription\.cycle_end\)\}/
  );
});

test('NeuralWatt kWh bar shares the elapsed-window marker and label', () => {
  const quotaBadges = read('components/quota-badges.tsx');
  const start = quotaBadges.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'neuralwatt' &&");
  const end = quotaBadges.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'apertis' &&");

  assert.ok(start !== -1, 'NeuralWatt popover branch should exist in quota-badges source');
  assert.ok(end !== -1 && end > start, 'the Apertis branch should follow the NeuralWatt branch');

  const neuralwattBlock = quotaBadges.slice(start, end);

  assert.match(neuralwattBlock, /const durationPercent = kwhLimit \? getLimitDurationPercent\(kwhLimit\) : undefined/);
  assert.match(neuralwattBlock, /durationPercent=\{durationPercent\}/);
  assert.match(neuralwattBlock, /quota\.label\.time_elapsed/);
  assert.match(quotaBadges, /kwh:\s*'quota\.label\.kwh_remaining'/);
});

// Regression: the quota-display refactor dropped the OpenCode Go popover
// without adding the shared-limits replacement, so the channels showed a
// battery percentage in the trigger but an empty detail popover.
test('OpenCode Go renders its normalized limit windows', () => {
  const quotaBadges = read('components/quota-badges.tsx');
  const start = quotaBadges.indexOf("{(channel.type === 'opencode_go' || channel.type === 'opencode_go_anthropic') &&");
  const end = quotaBadges.indexOf('{isOllamaType(channel.type) &&', start);

  assert.ok(start !== -1, 'OpenCode Go popover branch should exist in quota-badges source');
  assert.ok(end !== -1 && end > start, 'the Ollama branch should follow the OpenCode Go branch');

  const opencodeBlock = quotaBadges.slice(start, end);

  // Rendering must be driven by the normalized limits the backend still sends,
  // not by the removed provider-specific quotaData shape.
  assert.match(opencodeBlock, /quota\.limits/, 'OpenCode Go should render normalized limits');
  assert.match(
    opencodeBlock,
    /limit\.window === '5h'|preferredWindows = \['5h', 'weekly', 'monthly'\]/,
    'OpenCode Go should map its windows onto the shared labels'
  );
  assert.match(opencodeBlock, /WINDOW_LABEL_KEYS\[limit\.window\]/, 'OpenCode Go labels should resolve through the shared map');
  assert.match(opencodeBlock, /getLimitDurationPercent\(limit\)/, 'OpenCode Go bars should share the elapsed-window marker');
  assert.match(opencodeBlock, /formatTimeToReset\(limit\.nextResetAt\)/, 'OpenCode Go should show a reset countdown');
  assert.match(opencodeBlock, /quota\.label\.unavailable/, 'OpenCode Go should degrade gracefully when no limits are usable');
  assert.doesNotMatch(
    opencodeBlock,
    /ProviderOpenCodeGoQuotaData|qd\.windows/,
    'OpenCode Go should not depend on the removed provider-specific quota data'
  );
});

// --- Mode-aware quota badges (quota routing) ---

// Extract the pure mode helpers from quota-badges.tsx without loading React.
const badgesSource = read('components/quota-badges.tsx');
const badgesAst = ts.createSourceFile('quota-badges.tsx', badgesSource, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
const modeHelperNames = [
  'resolveEffectiveRoutingMode',
  'mostRestrictiveRoutingMode',
  'buildEffectiveModeByChannel',
  'getQuotaSortLabelKey',
  'resolveActiveQuotaCategory',
  'sortQuotaChannels',
  'readStoredBoolean',
  'readStoredSortMode',
  'writeStoredPreference',
  'isOpenaiType',
  'isCommandCodeType',
  'isOllamaType',
  'isMinimaxType',
  'isZenmuxType',
  'getChannelCategory',
  'buildQuotaDisplayItems',
];
const modeHelperNodes = badgesAst.statements.filter((node) => ts.isFunctionDeclaration(node) && modeHelperNames.includes(node.name?.text));
assert.equal(modeHelperNodes.length, 16, 'helpers must stay in quota-badges.tsx');
// The active-category resolver reads this module constant, so the extracted
// module needs it too.
const helperConstNames = ['WIDE_QUOTA_POPOVER_CHANNEL_COUNT'];
const helperConstNodes = badgesAst.statements.filter(
  (node) =>
    ts.isVariableStatement(node) &&
    node.declarationList.declarations.some((decl) => ts.isIdentifier(decl.name) && helperConstNames.includes(decl.name.text))
);
assert.equal(helperConstNodes.length, 1, 'the wide-popover threshold must stay a module constant');
const modeHelpers = [...helperConstNodes, ...modeHelperNodes]
  .map((node) => `export ${node.getText(badgesAst).replace(/^export\s+/, '')}`)
  .join('\n');

function loadTsModule(sourceText) {
  const { outputText } = ts.transpileModule(sourceText, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  });
  return import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
}

const {
  resolveEffectiveRoutingMode,
  mostRestrictiveRoutingMode,
  buildEffectiveModeByChannel,
  getQuotaSortLabelKey,
  resolveActiveQuotaCategory,
  sortQuotaChannels,
  readStoredBoolean,
  readStoredSortMode,
  writeStoredPreference,
  getChannelCategory,
  buildQuotaDisplayItems,
} = await loadTsModule(modeHelpers);

test('per-channel mode falls back to the global default when the channel defers via INHERIT', () => {
  // parseChannelNode maps channels without settings.quotaRoutingMode to INHERIT.
  assert.match(read('features/system/data/quotas.ts'), /quotaRoutingMode: node\.settings\?\.quotaRoutingMode \?\? 'INHERIT'/);
  assert.equal(resolveEffectiveRoutingMode('INHERIT', 'BACKPRESSURE'), 'BACKPRESSURE');
  assert.equal(resolveEffectiveRoutingMode('INHERIT', 'REMOVE_ON_EXHAUSTED'), 'REMOVE_ON_EXHAUSTED');
  // An explicit channel mode wins over the global default.
  assert.equal(resolveEffectiveRoutingMode('IGNORE_QUOTA', 'BACKPRESSURE'), 'IGNORE_QUOTA');
});

test('without read_settings scope data the mode label degrades gracefully', () => {
  // The scope-gated hook returns undefined data; an INHERIT channel defers to
  // a global default the viewer cannot see, so resolution yields null instead
  // of guessing a mode. Explicit channel modes still resolve.
  assert.equal(resolveEffectiveRoutingMode('INHERIT', undefined), null);
  assert.equal(resolveEffectiveRoutingMode('BACKPRESSURE', undefined), 'BACKPRESSURE');
  // QuotaBadges consumes the hook through optional chaining and QuotaRow
  // omits the badge entirely when no mode resolves — no crash, no label.
  assert.match(badgesSource, /const \{ data: routingSettings \} = useQuotaRoutingSettings\(\)/);
  assert.match(badgesSource, /routingSettings\?\.defaultMode/);
  assert.doesNotMatch(badgesSource, /useQuotaEnforcementSettings|QuotaEnforcementMode|allowedChannelIDs|enforcementEffect/);
  assert.match(badgesSource, /\{modeBadge && \(/);
});

test('effective modes are precomputed correctly for grouped and standalone quota rows', () => {
  const channels = [
    { id: 'account-1', accountKey: 'shared', quotaRoutingMode: 'INHERIT' },
    { id: 'account-2', accountKey: 'shared', quotaRoutingMode: 'REMOVE_ON_EXHAUSTED' },
    { id: 'standalone', quotaRoutingMode: 'INHERIT' },
  ];
  const displayChannels = [
    { id: 'account-1', accountKey: 'shared', sharedAccountNames: ['one', 'two'], quotaRoutingMode: 'INHERIT' },
    { id: 'standalone', quotaRoutingMode: 'INHERIT' },
  ];

  const modes = buildEffectiveModeByChannel(channels, displayChannels, 'IGNORE_QUOTA');
  assert.equal(modes.get('account-1'), 'REMOVE_ON_EXHAUSTED', 'shared row uses the most restrictive member mode');
  assert.equal(modes.get('standalone'), 'IGNORE_QUOTA', 'standalone row uses the global default');
});

test('sort modes resolve to keys that exist in both locale bundles', () => {
  const locales = ['en', 'zh-CN'].map((locale) => JSON.parse(read(`locales/${locale}/system.json`)));
  const expected = {
    preferred: 'system.providerQuota.sort.channelList',
    alphabetical: 'system.providerQuota.sort.alphabetical',
    created: 'system.providerQuota.sort.created',
  };

  for (const [mode, key] of Object.entries(expected)) {
    assert.equal(getQuotaSortLabelKey(mode), key);
    for (const locale of locales) {
      assert.ok(locale[key], `locale missing ${key}`);
      assert.notEqual(locale[key], key, `${key} must not render as a literal translation key`);
    }
  }

  for (const locale of locales) {
    assert.ok(locale['system.providerQuota.grouped'], 'grouped toggle label must exist in both locales');
  }
});

test('quota grouping and sort preferences persist through browser storage', () => {
  const values = new Map();
  const previous = globalThis.localStorage;
  globalThis.localStorage = {
    getItem: (key) => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, value),
  };
  try {
    // A missing key keeps the grouped default; ungrouped is a persisted choice.
    assert.equal(readStoredBoolean('quota-grouped', true), true);
    writeStoredPreference('quota-grouped', 'false');
    assert.equal(readStoredBoolean('quota-grouped', true), false);
    // A corrupt value falls back instead of silently disabling grouping.
    values.set('quota-grouped', 'yes');
    assert.equal(readStoredBoolean('quota-grouped', true), true);
    // Sort mode round-trips; unknown stored values fall back to preferred.
    writeStoredPreference('quota-sort', 'created');
    assert.equal(readStoredSortMode('quota-sort'), 'created');
    values.set('quota-sort', 'bogus');
    assert.equal(readStoredSortMode('quota-sort'), 'preferred');
    // Blocked storage never throws into the interaction.
    globalThis.localStorage = {
      getItem: () => {
        throw new Error('blocked');
      },
      setItem: () => {
        throw new Error('blocked');
      },
    };
    assert.equal(readStoredBoolean('quota-grouped', true), true);
    assert.equal(readStoredSortMode('quota-sort'), 'preferred');
    writeStoredPreference('quota-sort', 'alphabetical');
  } finally {
    globalThis.localStorage = previous;
  }
});

test('grouped toggle controls pills and section headers and is persisted', () => {
  assert.match(badgesSource, /const \[grouped, setGrouped\] = useState\(\(\) => readStoredBoolean\(QUOTA_GROUPED_STORAGE_KEY, true\)\)/);
  assert.match(badgesSource, /writeStoredPreference\(QUOTA_GROUPED_STORAGE_KEY, String\(grouped\)\)/);
  assert.match(badgesSource, /writeStoredPreference\(QUOTA_SORT_MODE_STORAGE_KEY, sortMode\)/);
  assert.match(badgesSource, /onCheckedChange=\{\(checked\) => setGrouped\(checked === true\)\}/);
  assert.match(
    badgesSource,
    /\{sortedChannels\.length > WIDE_QUOTA_POPOVER_CHANNEL_COUNT && \(\s*\/\/ Same wide-popover gate as the pill strip/
  );
  assert.match(badgesSource, /system\.providerQuota\.grouped/);
  // Off: flat rows without headers, no pills.
  assert.match(badgesSource, /grouped\s*\?\s*buildQuotaDisplayItems/);
  assert.match(badgesSource, /:\s*groupedChannels\.map\(\(channel\) => \(\{ kind: 'row' as const, channel \}\)\)/);
  assert.match(
    badgesSource,
    /resolveActiveQuotaCategory\(grouped, sortedChannels\.length, selectedCategory, categories\)/,
    'The active category must go through the visibility-aware resolver'
  );
  assert.match(badgesSource, /\{grouped && !isLoading && !isError && sortedChannels\.length > WIDE_QUOTA_POPOVER_CHANNEL_COUNT && \(/);
});

test('a hidden category filter can never keep rows out of the list', () => {
  const categories = [{ key: 'codex' }, { key: 'nanogpt' }];

  // Visible controls in the wide grouped popover: the selection applies.
  assert.equal(resolveActiveQuotaCategory(true, 7, 'codex', categories), 'codex');
  // Ungrouped: the pills are gone, so the filter must not narrow anything.
  assert.equal(resolveActiveQuotaCategory(false, 7, 'codex', categories), 'all');
  // Channel count drops to the narrow popover threshold: same reasoning.
  assert.equal(resolveActiveQuotaCategory(true, 4, 'codex', categories), 'all');
  // A selected category that no longer exists also falls back.
  assert.equal(resolveActiveQuotaCategory(true, 7, 'apertis', categories), 'all');
  // No explicit selection stays on all.
  assert.equal(resolveActiveQuotaCategory(true, 7, 'all', categories), 'all');
});

test('account-grouped label uses the most restrictive member mode', () => {
  assert.equal(mostRestrictiveRoutingMode(['BACKPRESSURE', 'REMOVE_ON_EXHAUSTED']), 'BACKPRESSURE');
  assert.equal(mostRestrictiveRoutingMode(['REMOVE_ON_EXHAUSTED', 'IGNORE_QUOTA']), 'REMOVE_ON_EXHAUSTED');
  assert.equal(mostRestrictiveRoutingMode(['IGNORE_QUOTA', null]), 'IGNORE_QUOTA');
  // Unresolvable members are ignored; an all-unresolvable group shows no badge.
  assert.equal(mostRestrictiveRoutingMode([null, null]), null);
  assert.equal(mostRestrictiveRoutingMode([]), null);
});

test('grouped representatives render precomputed effective modes through live quota rows', () => {
  assert.match(badgesSource, /displayItems\.map\(\(item\)/);
  assert.match(badgesSource, /<QuotaRow[\s\S]*?effectiveMode=\{effectiveModeByChannel\.get\(item\.channel\.id\) \?\? null\}/);
  assert.match(badgesSource, /const effectiveModeByChannel = useMemo\([\s\S]*?buildEffectiveModeByChannel\(channels, dedupedChannels/);
  assert.doesNotMatch(badgesSource, /\.filter\(\(c\) => c\.accountKey === channel\.accountKey\)/);
  // Rows read Date.now() for reset countdowns and elapsed markers; structural
  // sharing keeps channel objects identical across unchanged polls, so a
  // memoized row would freeze that text while the popover stays open.
  assert.doesNotMatch(badgesSource, /memo\(QuotaRow\)/, 'QuotaRow must stay unmemoized so clock-derived text advances');
});

test('channels in quota popover are categorized with pill filters and section headers', () => {
  assert.match(badgesSource, /function getChannelCategory\(/, 'getChannelCategory helper must exist');
  // Categories no longer carry a hardcoded priority: the strip is alphabetical.
  assert.doesNotMatch(badgesSource, /order:\s*\d+/, 'categories must not embed hardcoded priority orders');
  assert.match(badgesSource, /catA\.label\.localeCompare\(catB\.label\)/, 'Category groups should sort alphabetically by label');
  assert.match(badgesSource, /a\.name\.localeCompare\(b\.name\)/, 'Channels should be stably sorted by name within categories');
  // Pills live on their own line below the header (only in the wide popover):
  // nothing competes with the title row, and the line scrolls horizontally so
  // it can never wrap into the multi-row strip that ate the popover before.
  assert.match(
    badgesSource,
    /\{grouped && !isLoading && !isError && sortedChannels\.length > WIDE_QUOTA_POPOVER_CHANNEL_COUNT && \(\s*\/\/ Pills as their own single scrollable line/,
    'Pill strip renders on its own line only while grouping is on in the wide popover'
  );
  assert.match(badgesSource, /flex items-center gap-1 overflow-x-auto pb-1\.5/, 'Pill strip is a single scrollable line');
  assert.doesNotMatch(badgesSource, /flex flex-wrap items-center gap-1/, 'Pill strip must not wrap to multiple lines');
  assert.match(badgesSource, /aria-pressed=\{activeCategory === 'all'\}/, 'All pill exposes pressed state');
  assert.match(badgesSource, /aria-pressed=\{activeCategory === cat\.key\}/, 'Category pills expose pressed state');
  assert.match(badgesSource, /shrink-0[^`]*whitespace-nowrap/, 'Pills must not shrink or wrap mid-label');
  assert.match(badgesSource, /col-span-full[\s\S]*?uppercase/, 'Popover should render section header across full grid width');
  assert.match(badgesSource, /item\.cat\.count/, 'Section header counter must read from the category count');
  assert.match(
    badgesSource,
    /function resolveActiveQuotaCategory\([\s\S]*?categories\.some\(\(c\) => c\.key === selectedCategory\)/,
    'Selection must fall back to all when the category filter is hidden or the category disappears'
  );
  assert.match(badgesSource, /text-foreground truncate font-medium|truncate font-medium/, 'Channel name should be truncated with ellipsis');
  assert.match(badgesSource, /ml-2 flex shrink-0/, 'Badge container should not shrink and have left margin');
  assert.match(
    badgesSource,
    /sortedChannels\.length > WIDE_QUOTA_POPOVER_CHANNEL_COUNT \? 'grid grid-cols-1 gap-x-4 sm:grid-cols-2'/,
    'Dual-column grid should be based on total sorted channels'
  );
});

test('channel categories order alphabetically and sort stably by name within groups', () => {
  const t = (k) => k;
  const sampleChannels = [
    { id: '1', name: 'copilot-beta', type: 'github_copilot' },
    { id: '2', name: 'antigravity-zeta', type: 'antigravity' },
    { id: '3', name: 'copilot-alpha', type: 'github_copilot' },
    { id: '4', name: 'antigravity-alpha', type: 'antigravity' },
    { id: '5', name: 'nano-single', type: 'nanogpt' },
    { id: '6', name: 'claude-x', type: 'claudecode' },
    { id: '7', name: 'codex-y', type: 'codex' },
  ];

  const sorted = sortQuotaChannels(sampleChannels, 'alphabetical', t);

  // Pure label alphabetical order across category groups: 'Antigravity' <
  // 'Claude Code' < 'Codex' < 'GitHub Copilot' < 'NanoGPT' — the fixture
  // intentionally includes pairs the old priority order sorted differently.
  assert.deepEqual(
    sorted.map((c) => c.name),
    ['antigravity-alpha', 'antigravity-zeta', 'claude-x', 'codex-y', 'copilot-alpha', 'copilot-beta', 'nano-single']
  );

  // Shared keys across protocol variants
  assert.equal(getChannelCategory({ type: 'openai' }, t).key, 'openai');
  assert.equal(getChannelCategory({ type: 'openai_responses' }, t).key, 'openai');
  assert.equal(getChannelCategory({ type: 'opencode_go' }, t).key, 'opencode_go');
  assert.equal(getChannelCategory({ type: 'opencode_go_anthropic' }, t).key, 'opencode_go');

  // Active category falls back to all when category disappears
  const activeCategories = [{ key: 'antigravity' }];
  const getActive = (selected) => (selected !== 'all' && activeCategories.some((c) => c.key === selected) ? selected : 'all');
  assert.equal(getActive('codex'), 'all');
  assert.equal(getActive('antigravity'), 'antigravity');
});

// Regression: the pills PR capitalized the raw providerType slug, so the pill
// read "Charm_hyper" while the channel is named "Hyper"; the label now matches
// the channel-naming convention users actually use for this provider.
test('openai-compatible provider categories resolve localized provider names', () => {
  const enSystem = JSON.parse(read('locales/en/system.json'));
  const t = (key) => (key in enSystem ? enSystem[key] : key);

  assert.equal(getChannelCategory({ type: 'openai', providerType: 'charm_hyper' }, t).label, 'Hyper');
  assert.equal(getChannelCategory({ type: 'openai_responses', providerType: 'neuralwatt' }, t).label, 'NeuralWatt');
  assert.equal(getChannelCategory({ type: 'openai', providerType: 'apertis' }, t).label, 'Apertis');
  // Same pillar for zh-CN so both locales keep the provider nouns.
  const zhSystem = JSON.parse(read('locales/zh-CN/system.json'));
  const tZh = (key) => (key in zhSystem ? zhSystem[key] : key);
  assert.equal(getChannelCategory({ type: 'openai', providerType: 'charm_hyper' }, tZh).label, 'Hyper');
  // Unknown providerTypes degrade to the raw slug instead of crashing or
  // returning the i18n key itself.
  assert.equal(getChannelCategory({ type: 'openai', providerType: 'future_provider' }, t).label, 'future_provider');
});

// Regression: the pills PR emitted a section header before every category,
// which broke the pre-PR two-per-row packing (a lone channel stranded an empty
// cell beside it) and repeated the channel name in the header.
test('section headers only precede multi-channel categories so rows keep pairing', () => {
  const t = (key) => key;
  const channels = [
    { id: 'c1', name: 'Codex', type: 'codex' },
    { id: 'c2', name: 'Codex2', type: 'codex' },
    { id: 'n1', name: 'NanoGPT', type: 'nanogpt' },
    { id: 'a1', name: 'Apertis', type: 'openai', providerType: 'apertis' },
  ];
  // Categories arrive pre-sorted alphabetically, as QuotaBadges derives them.
  const categoryList = [
    { key: 'apertis', label: 'Apertis', count: 1 },
    { key: 'codex', label: 'Codex', count: 2 },
    { key: 'nanogpt', label: 'NanoGPT', count: 1 },
  ];

  const items = buildQuotaDisplayItems(channels, categoryList, 'all', t);
  assert.deepEqual(
    items.map((item) => item.kind),
    ['row', 'header', 'row', 'row', 'row'],
    'only the 2-channel category gets a header; singles emit just their row'
  );
  assert.equal(items[1].cat.key, 'codex');
  assert.deepEqual(
    items.filter((item) => item.kind === 'row').map((item) => item.channel.name),
    ['Apertis', 'Codex', 'Codex2', 'NanoGPT']
  );

  // An active category filter drops headers entirely for a flat list: the
  // QuotaBadges component pre-filters channels before calling, so the helper
  // maps everything it receives to rows.
  const filtered = buildQuotaDisplayItems(
    channels.filter((channel) => channel.type === 'codex'),
    categoryList,
    'codex',
    t
  );
  assert.deepEqual(
    filtered.map((item) => item.kind),
    ['row', 'row']
  );
  const singleFiltered = buildQuotaDisplayItems(
    channels.filter((channel) => channel.type === 'nanogpt'),
    categoryList,
    'nanogpt',
    t
  );
  assert.deepEqual(
    singleFiltered.map((item) => item.kind),
    ['row']
  );
});

// Creation order = the GraphQL channel id embeds the auto-increment DB id
// (gid://axonhub/Channel/<n>); numeric-aware compare restores the pre-pills-pr
// backend default order without a new API field. Preferred order mirrors the
// operator's channel list (orderingWeight, higher first) as the initial mode.
// The sort control is a labeled dropdown with radio items: three mutually
// exclusive modes are beyond what a hover-only cycling icon can communicate.
test('sort control is a labeled dropdown cycling preferred, alphabetical, creation order', () => {
  const t = (key) => key;
  assert.match(
    badgesSource,
    /sortMode, setSortMode\] = useState<QuotaSortMode>\(\(\) => readStoredSortMode\(QUOTA_SORT_MODE_STORAGE_KEY\)\)/,
    'Sort state restores the stored mode, defaulting to preferred list order'
  );
  // Labeled trigger: current mode text next to the sort glyph, not icon-only.
  assert.match(badgesSource, /<ArrowUpDown className='h-3\.5 w-3\.5' \/>/, 'Trigger shows a sort glyph');
  assert.match(
    badgesSource,
    /<span>\{t\(getQuotaSortLabelKey\(sortMode\)\)\}<\/span>/,
    'Trigger label uses the explicit translation key mapping'
  );
  assert.doesNotMatch(
    badgesSource,
    /t\(`system\.providerQuota\.sort\.\$\{sortMode\}`\)/,
    'No mode may resolve through a missing preferred key'
  );
  assert.match(badgesSource, /aria-label=\{t\('system\.providerQuota\.sort\.label'\)\}/, 'Trigger exposes an accessible name');
  // Radio group semantics: exactly one mode selectable, indicator on the active one.
  assert.match(badgesSource, /<DropdownMenuRadioGroup value=\{sortMode\}/, 'Modes live in a radio group');
  for (const mode of ['preferred', 'alphabetical', 'created']) {
    assert.match(badgesSource, new RegExp(`<DropdownMenuRadioItem value='${mode}'>`), `${mode} mode is a radio item`);
  }
  assert.match(
    badgesSource,
    /<DropdownMenuRadioItem value='preferred'>\{t\('system\.providerQuota\.sort\.channelList'\)\}/,
    'Preferred mode is labeled Channel list order, not the vague Preferred order'
  );
  // The preferred comparator ranks higher orderingWeight first; ties fall back
  // to the backend default (numeric id ASC).
  assert.match(
    badgesSource,
    /b\.orderingWeight - a\.orderingWeight \|\| byCreated\(a, b\)/,
    'Preferred mode sorts by ordering weight desc with the shared creation tiebreak'
  );

  // Exercise the production helper: created mode matches the backend order
  // (numeric ascending on the GUID tail) and preferred mode ranks weight first.
  const byCreation = (ids) =>
    sortQuotaChannels(
      ids.map((id) => ({ id })),
      'created',
      t
    ).map((channel) => channel.id);
  assert.deepEqual(
    byCreation(['gid://axonhub/Channel/9', 'gid://axonhub/Channel/10', 'gid://axonhub/Channel/2']),
    ['gid://axonhub/Channel/2', 'gid://axonhub/Channel/9', 'gid://axonhub/Channel/10'],
    'Numeric compare must not sort ids lexicographically (2 < 10)'
  );
  assert.deepEqual(
    sortQuotaChannels(
      [
        { id: 'gid://axonhub/Channel/7', orderingWeight: 10 },
        { id: 'gid://axonhub/Channel/3', orderingWeight: 50 },
        { id: 'gid://axonhub/Channel/11', orderingWeight: 10 },
      ],
      'preferred',
      t
    ).map((channel) => channel.id),
    ['gid://axonhub/Channel/3', 'gid://axonhub/Channel/7', 'gid://axonhub/Channel/11'],
    'Preferred order ranks higher weight first, ties by numeric id'
  );
});

test('mode badge labels are locale-complete and legacy enforcement keys are gone', () => {
  const enSystem = JSON.parse(read('locales/en/system.json'));
  const zhSystem = JSON.parse(read('locales/zh-CN/system.json'));
  for (const key of ['quota.status.ignore_quota', 'quota.status.remove_on_exhausted', 'quota.status.backpressure']) {
    assert.ok(enSystem[key], `en/system.json missing ${key}`);
    assert.ok(zhSystem[key], `zh-CN/system.json missing ${key}`);
  }
  assert.equal(zhSystem['quota.status.ignore_quota'], '忽略限额');
  assert.equal(zhSystem['quota.status.remove_on_exhausted'], '已摘除');
  assert.equal(zhSystem['quota.status.backpressure'], '限流调度');
  for (const [name, obj] of [
    ['en', enSystem],
    ['zh-CN', zhSystem],
  ]) {
    const legacy = Object.keys(obj).filter(
      (key) =>
        key.startsWith('quota.status.blocked') || key.startsWith('quota.status.deprioritized') || key.startsWith('quota.status.bypassed')
    );
    assert.deepEqual(legacy, [], `${name}/system.json still has legacy quota status keys`);
  }
});
test('Synthetic badge lists the hourly search quota without gating', () => {
  const source = read('components/quota-badges.tsx');
  const start = source.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'synthetic' &&");
  const end = source.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'neuralwatt' &&", start);
  const syntheticBlock = source.slice(start, end);

  assert.match(syntheticBlock, /qd\.search\?\.hourly/);
  assert.match(syntheticBlock, /searchHourly\.requests != null/);
  assert.match(syntheticBlock, /'quota\.label\.search_requests'/);
  assert.match(syntheticBlock, /calcDurationPercent\(3600,/);
  assert.match(syntheticBlock, /formatTimeToReset\(searchHourly\.renewsAt\)/);

  // Provider-reported credits are USD; every other provider keeps the
  // configured display currency.
  assert.match(source, /currency=\{channel\.providerType === 'synthetic' \? 'USD' : undefined\}/);
  assert.match(source, /currency: currency \|\| generalSettings\?\.currencyCode \|\| 'USD'/);
});

// --- Synthetic weekly pool behavior (production code, real data) ---

// The Synthetic weekly pool math lives in module-scope helpers plus expressions
// embedded in the render branch. Extract the exact production source (not a
// re-implementation) so plan-only, legacy-only, and mixed payloads run through
// the actual percentage / reset / regen-tick logic.
const syntheticHelperNames = [
  'isOpenaiType',
  'isCommandCodeType',
  'isOllamaType',
  'isMinimaxType',
  'isZenmuxType',
  'getClineUsagePercent',
  'getApertisPercentage',
  'getChannelPercentage',
];
const syntheticFuncNodes = [];
const collectSyntheticFunctions = (node) => {
  ts.forEachChild(node, (child) => {
    if (ts.isFunctionDeclaration(child) && child.name?.text && syntheticHelperNames.includes(child.name.text)) {
      if (!syntheticFuncNodes.some((n) => n.name.text === child.name.text)) syntheticFuncNodes.push(child);
      return;
    }
    collectSyntheticFunctions(child);
  });
};
collectSyntheticFunctions(badgesAst);
assert.equal(syntheticFuncNodes.length, syntheticHelperNames.length, 'all Synthetic helpers must stay in quota-badges.tsx');

const syntheticConstNames = ['syntheticWeeklyRegenTickPct', 'calcDurationPercent', 'formatTimeToReset'];
const syntheticConstNodes = [];
const collectSyntheticConsts = (node) => {
  ts.forEachChild(node, (child) => {
    if (ts.isVariableStatement(child) && child.declarationList) {
      for (const decl of child.declarationList.declarations) {
        if (ts.isIdentifier(decl.name) && syntheticConstNames.includes(decl.name.text)) {
          if (!syntheticConstNodes.some((n) => n.getText() === child.getText())) syntheticConstNodes.push(child);
          return;
        }
      }
    }
    collectSyntheticConsts(child);
  });
};
collectSyntheticConsts(badgesAst);
assert.equal(syntheticConstNodes.length, syntheticConstNames.length, 'all Synthetic constants must stay in quota-badges.tsx');

// Locate render-branch expressions to rebuild as executable functions.
const findVariableDeclaration = (root, name) => {
  let found;
  const walk = (node) => {
    if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && node.name.text === name) found = node;
    ts.forEachChild(node, walk);
  };
  walk(root);
  return found;
};
const regenTickDecl = findVariableDeclaration(badgesAst, 'regenTick');
const weeklyNextRegenAtDecl = findVariableDeclaration(badgesAst, 'weeklyNextRegenAt');
assert.ok(regenTickDecl?.initializer, 'regenTick expression must stay in the Synthetic render branch');
assert.ok(weeklyNextRegenAtDecl?.initializer, 'weeklyNextRegenAt expression must stay in the Synthetic render branch');

const syntheticModuleSource = [
  // i18n shim: formatTimeToReset branches and rounds through t(); the shim emits
  // key:params so the branch choice and the rounded percent are asserted for real.
  'const t = (key, params) => (params ? key + ":" + JSON.stringify(params) : key);',
  ...syntheticFuncNodes.map((node) => `export ${node.getText(badgesAst)}`),
  ...syntheticConstNodes.map((node) => `export ${node.getText(badgesAst)}`),
  `export const getSyntheticWeeklyRegenTick = (planWeekly) => ${regenTickDecl.initializer.getText(badgesAst)};`,
  `export const getSyntheticWeeklyResetAt = (qd) => { const planWeekly = qd.planWeeklyLimit; return ${weeklyNextRegenAtDecl.initializer.getText(badgesAst)}; };`,
].join('\n');

const {
  getChannelPercentage: badgePercent,
  syntheticWeeklyRegenTickPct,
  calcDurationPercent,
  formatTimeToReset: badgeFormatTimeToReset,
  getSyntheticWeeklyRegenTick,
  getSyntheticWeeklyResetAt,
} = await loadTsModule(syntheticModuleSource);

// --- Synthetic weekly badge render path (real component sources) ---

// Extract the exact Synthetic render branch from quota-badges.tsx (between the
// branch marker and the NeuralWatt branch marker), compile that production JSX,
// and render it with React SSR so the observable badge output — percent label,
// bar fill, duration marker, tooltip countdown — is asserted, not the
// disconnected expressions that feed it.
const syntheticBranchStart = badgesSource.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'synthetic' &&");
const syntheticBranchEnd = badgesSource.indexOf(
  "{isOpenaiType(channel.type) && channel.providerType === 'neuralwatt' &&",
  syntheticBranchStart
);
assert.ok(syntheticBranchStart !== -1, 'Synthetic render branch should exist in quota-badges source');
assert.ok(
  syntheticBranchEnd !== -1 && syntheticBranchEnd > syntheticBranchStart,
  'the NeuralWatt branch should follow the Synthetic branch'
);
const syntheticBranchInner = badgesSource.slice(syntheticBranchStart, syntheticBranchEnd).trim();
assert.ok(syntheticBranchInner.startsWith('{isOpenaiType'), 'Synthetic branch extraction must start at the branch condition');
// The outer braces are the JSX expression-container delimiters, not JS block
// braces: drop them so the remaining expression plugs into `return (...)`.
const syntheticBranchExpr = syntheticBranchInner.slice(1, -1);
assert.ok(syntheticBranchExpr.endsWith(')'), 'Synthetic branch must end with the JSX group close');

// t + formatTimeToReset/calcDurationPercent/regen-tick constant come from the
// production source (syntheticConstNodes); the real en locale is embedded so the
// i18next-style translator inside the module interpolates {{param}} placeholders
// into final user-facing countdown text.
const enSystem = JSON.parse(read('locales/en/system.json'));
const syntheticRenderModule = [
  'const enSystem = ' + JSON.stringify(enSystem) + ';',
  'function t(key, params) {',
  '  let out = enSystem[key] ?? key;',
  '  for (const [name, value] of Object.entries(params || {})) out = out.split(`{{${name}}}`).join(String(value));',
  '  return out;',
  '}',
  ...syntheticFuncNodes.map((node) => node.getText(badgesAst)),
  ...syntheticConstNodes.map((node) => node.getText(badgesAst)),
  // Same clamping as the real shared bar, projected into data-* attributes so
  // renderToStaticMarkup output is assertable without a DOM or Radix context.
  // The transpiled branch references the classic `React.createElement` factory;
  // data: module imports share the host realm, so globalThis.React (set below
  // before the import) provides it.
  'function UsageTimeBar({ usagePercent, durationPercent, tooltip }) {',
  '  const clamped = Math.min(Math.max(usagePercent || 0, 0), 100);',
  '  const markerLeft = durationPercent === undefined ? undefined : Math.min(Math.max(durationPercent, 0), 100);',
  '  const attrs = { className: "bar" };',
  '  attrs["data-fill"] = clamped;',
  '  if (markerLeft !== undefined) attrs["data-duration"] = markerLeft;',
  '  return <div {...attrs}>{tooltip}</div>;',
  '}',
  'function Badge({ children }) { return children; }',
  'function SyntheticBadge({ channel }) {',
  '  return (' + syntheticBranchExpr + ');',
  '}',
  'export default SyntheticBadge;',
].join('\n');
const { outputText: syntheticRenderJs } = ts.transpileModule(syntheticRenderModule, {
  compilerOptions: { jsx: ts.JsxEmit.React, module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
globalThis.React = React;
const { default: SyntheticBadge } = await import(`data:text/javascript;base64,${Buffer.from(syntheticRenderJs).toString('base64')}`);

const syntheticChannel = (quotaData) => ({ type: 'openai', providerType: 'synthetic', quotaStatus: { quotaData, limits: [] } });
const REFERENCE_NOW = Date.now();
const DAY = 24 * 3600 * 1000;
const WEEK_AHEAD = new Date(REFERENCE_NOW + 6 * DAY).toISOString();

function renderSyntheticBadge(quotaData) {
  // Pin Date.now so bar fills, duration markers and countdown strings are
  // deterministic for a fixed reset offset.
  const realNow = Date.now;
  Date.now = () => REFERENCE_NOW;
  try {
    return renderToStaticMarkup(React.createElement(SyntheticBadge, { channel: syntheticChannel(quotaData) }));
  } finally {
    Date.now = realNow;
  }
}

function attrOf(html, name) {
  const m = html.match(new RegExp(`${name}="([^"]*)"`));
  return m ? m[1] : null;
}

test('Synthetic weekly badge renders the plan-only pool: percent, bar fill, elapsed marker, regen countdown', () => {
  const html = renderSyntheticBadge({ planWeeklyLimit: { percentRemaining: 30, nextRegenAt: WEEK_AHEAD, nextRegenPercent: 2 } });
  const usedPct = 100 - 30;
  // The weekly item renders with the plan-derived used percentage.
  assert.match(html, /Weekly Token Limit/);
  assert.match(html, /<span class="text-foreground font-medium">70% used<\/span>/);
  assert.equal(attrOf(html, 'data-fill'), String(usedPct));
  // The elapsed marker sits at 1/7 through the week (reset 6 days out of 7).
  assert.ok(attrOf(html, 'data-duration') !== null, 'weekly bar must carry a duration marker from nextRegenAt');
  assert.ok(Math.abs(Number(attrOf(html, 'data-duration')) - (1 / 7) * 100) < 0.01, 'marker must reflect the plan reset time');
  // The tooltip countdown uses the plan tick scaled by /100 (2 -> "2%"), not the
  // plain "Regenerates in" form — a regenerating branch or missing /100 shows up here.
  assert.match(html, /Regenerates 2% in 6d 0h/);
});

test('Synthetic weekly badge renders the legacy-only pool with the built-in regen tick', () => {
  const html = renderSyntheticBadge({ weeklyTokenLimit: { percentRemaining: 55, nextRegenAt: WEEK_AHEAD } });
  assert.match(html, /Weekly Token Limit/);
  assert.match(html, /<span class="text-foreground font-medium">45% used<\/span>/);
  assert.equal(attrOf(html, 'data-fill'), '45');
  // No plan data, so the countdown falls back to the built-in 2% regen tick.
  assert.ok(
    html.includes(`Regenerates ${Math.round(syntheticWeeklyRegenTickPct * 100)}% in 6d 0h`),
    'legacy pool must use the built-in regen tick'
  );
});

test('Synthetic weekly badge falls back to the legacy shape field by field in the rendered countdown', () => {
  const legacyAt = new Date(REFERENCE_NOW + 5 * DAY).toISOString();
  const qd = {
    planWeeklyLimit: { percentRemaining: null, nextRegenAt: WEEK_AHEAD, nextRegenPercent: null },
    weeklyTokenLimit: { percentRemaining: 55, nextRegenAt: legacyAt },
  };
  const html = renderSyntheticBadge(qd);
  // Percent is missing on the plan side, so the rendered percentage comes from the
  // legacy pool — while the plan's nextRegenAt supplies the reset time and the
  // null plan tick falls back to the built-in tick.
  assert.match(html, /<span class="text-foreground font-medium">45% used<\/span>/);
  assert.equal(attrOf(html, 'data-fill'), '45');
  assert.ok(Math.abs(Number(attrOf(html, 'data-duration')) - (1 / 7) * 100) < 0.01, 'marker must reflect the plan reset time');
  assert.match(html, /Regenerates 2% in 6d 0h/);
});

test('Synthetic weekly badge uses the legacy reset time when the plan field is missing', () => {
  const legacyAt = new Date(REFERENCE_NOW + 5 * DAY).toISOString();
  const html = renderSyntheticBadge({
    planWeeklyLimit: { percentRemaining: 50, nextRegenPercent: 3 },
    weeklyTokenLimit: { nextRegenAt: legacyAt },
  });
  // The plan percentage survives (no legacy percent to mix in) ...
  assert.match(html, /<span class="text-foreground font-medium">50% used<\/span>/);
  assert.equal(attrOf(html, 'data-fill'), '50');
  // ... but with the plan's nextRegenAt absent, the displayed countdown and the
  // elapsed marker derive from the legacy reset 5 days out, not the plan one.
  assert.ok(Math.abs(Number(attrOf(html, 'data-duration')) - (2 / 7) * 100) < 0.01, 'marker must fall back to the legacy reset time');
  assert.ok(html.includes('Regenerates 3% in 5d 0h'), 'countdown must use the legacy reset time with the plan tick');
  assert.ok(!html.includes('6d 0h'), 'the plan reset time must not leak into fallback mode');
});

test('Synthetic plan percent of zero is rendered as an explicit exhaustion, not a missing field', () => {
  const html = renderSyntheticBadge({
    planWeeklyLimit: { percentRemaining: 0, nextRegenAt: WEEK_AHEAD, nextRegenPercent: 2 },
    weeklyTokenLimit: { percentRemaining: 90, nextRegenAt: WEEK_AHEAD },
  });
  // The plan's zero beats the legacy 90%; a `??`-safe check that drops 0 would
  // render the legacy pool (10% used) instead.
  assert.match(html, /<span class="text-foreground font-medium">100% used<\/span>/);
  assert.equal(attrOf(html, 'data-fill'), '100');
  assert.match(html, /Regenerates 2% in 6d 0h/);
});

test('Synthetic plan next-regen percent 2 renders as a 2% regen tick', () => {
  const twoHoursOut = new Date(REFERENCE_NOW + 2 * 3600 * 1000).toISOString();
  const planWeekly = { percentRemaining: 50, nextRegenAt: twoHoursOut, nextRegenPercent: 2 };
  const tick = getSyntheticWeeklyRegenTick(planWeekly);
  assert.equal(tick, 0.02);
  const regenLabel = badgeFormatTimeToReset(twoHoursOut, 50, tick);
  assert.match(regenLabel, /^quota\.label\.regenerates_pct_in_time:\{"percent":2,"time":"[^"]+"\}$/);
  // Bars without data still report nothing used.
  assert.equal(badgeFormatTimeToReset(twoHoursOut, 0, tick), 'quota.label.no_usage_yet');
  // The hourly search window resolves elapsed time the same way: half an hour of a
  // one-hour budget is 50%.
  assert.equal(calcDurationPercent(3600, 1800), 50);
});

test('Synthetic badge renders the hourly search row with count, percentage, renewal', () => {
  const renewalsAhead = new Date(REFERENCE_NOW + 30 * 60 * 1000).toISOString();
  const html = renderSyntheticBadge({
    weeklyTokenLimit: { percentRemaining: 100, nextRegenAt: WEEK_AHEAD },
    search: { hourly: { limit: 250, requests: 125, renewsAt: renewalsAhead } },
  });

  assert.match(html, /Search requests/, 'search label must render');
  // Count is observable in the row header and the tooltip.
  assert.match(html, /125\/250/, 'search count must render');
  // 125/250 rounds to 50% used in the header and the tooltip.
  assert.match(html, /<span class="text-foreground font-medium">50% used<\/span>/);
  const fills = [...html.matchAll(/data-fill="([^"]*)"/g)].map((m) => Number(m[1]));
  assert.ok(fills.includes(50), `search bar must carry a 50% fill (fills: ${fills.join(',')})`);
  // The renewal countdown renders in the tooltip (30 min out is 50% of the hour).
  assert.match(html, /Resets in 30m/);
  const durations = [...html.matchAll(/data-duration="([^"]*)"/g)].map((m) => Number(m[1]));
  assert.ok(
    durations.some((v) => Math.abs(v - 50) < 0.01),
    `search bar must carry a 50% elapsed marker (markers: ${durations.join(',')})`
  );
});

test('Synthetic badge omits the hourly search row without usable counter values', () => {
  const renewalsAhead = new Date(REFERENCE_NOW + 1800000).toISOString();
  const base = renderSyntheticBadge({ weeklyTokenLimit: { percentRemaining: 100, nextRegenAt: WEEK_AHEAD } });

  const missingRequests = renderSyntheticBadge({
    weeklyTokenLimit: { percentRemaining: 100, nextRegenAt: WEEK_AHEAD },
    search: { hourly: { limit: 250, renewsAt: renewalsAhead } },
  });
  assert.doesNotMatch(missingRequests, /Search requests/, 'missing requests must omit the search row');
  assert.match(base, /Weekly Token Limit/, 'sanity: the base badge renders the weekly section');

  const zeroLimit = renderSyntheticBadge({
    weeklyTokenLimit: { percentRemaining: 100, nextRegenAt: WEEK_AHEAD },
    search: { hourly: { limit: 0, requests: 125, renewsAt: renewalsAhead } },
  });
  assert.doesNotMatch(zeroLimit, /Search requests/, 'a non-positive limit must omit the search row');
});

test('Synthetic badge percentage prefers the plan pool and falls back to the legacy pool', () => {
  // The consumer-visible battery percentage is 100 - percentRemaining; the
  // plan-only shape must win over the legacy shape.
  const planChannel = syntheticChannel({
    planWeeklyLimit: { percentRemaining: 30, nextRegenAt: WEEK_AHEAD },
    weeklyTokenLimit: { percentRemaining: 90, nextRegenAt: WEEK_AHEAD },
  });
  assert.equal(badgePercent(planChannel), 70, 'planWeeklyLimit must win over weeklyTokenLimit');

  // Without the plan shape, the legacy pool drives the percentage.
  const legacyChannel = syntheticChannel({ weeklyTokenLimit: { percentRemaining: 55, nextRegenAt: WEEK_AHEAD } });
  assert.equal(badgePercent(legacyChannel), 45, 'legacy weeklyTokenLimit must drive the percentage when the plan shape is absent');

  const planOnlyChannel = syntheticChannel({ planWeeklyLimit: { percentRemaining: 30, nextRegenAt: WEEK_AHEAD } });
  assert.equal(badgePercent(planOnlyChannel), 70, 'plan-only shape must drive the percentage');
});

// --- Period quota rows: provider-reported vs log-derived source (production sources) ---

// The period quota rows come from `quotaData._limits`; `periodStart` is the
// marker of a log-derived row because only limits the backend priced from
// usage logs carry one (provider-reported money replaces the estimate). The
// component must pick its tooltip wording per row source, and mark rows when
// a channel mixes both. Extract the exact production function and render it
// SSR against the real en locale, the same way the Synthetic branch tests do.
const enCurrencies = JSON.parse(read('locales/en/currencies.json'));
const periodQuotaFuncNames = ['periodMoneyIsProviderReported', 'PeriodQuotaEstimate'];
const periodQuotaFuncNodes = [...badgesAst.statements].filter(
  (node) => ts.isFunctionDeclaration(node) && periodQuotaFuncNames.includes(node.name?.text)
);
assert.equal(periodQuotaFuncNodes.length, periodQuotaFuncNames.length, 'period quota components must stay in quota-badges.tsx');
const windowLabelKeysDecl = [...badgesAst.statements].find(
  (node) =>
    ts.isVariableStatement(node) &&
    node.declarationList.declarations.some((d) => ts.isIdentifier(d.name) && d.name.text === 'WINDOW_LABEL_KEYS')
);
assert.ok(windowLabelKeysDecl, 'WINDOW_LABEL_KEYS must stay in quota-badges.tsx');

const enPeriodQuota = { ...enSystem, ...enCurrencies };
const periodQuotaRenderModule = [
  'const table = ' + JSON.stringify(enPeriodQuota) + ';',
  'function t(key, params) {',
  '  let out = table[key] ?? key;',
  '  if (key === "currencies.format") {',
  '    out = new Intl.NumberFormat(params?.locale || "en-US", { style: "currency", currency: params?.currency || "USD", minimumFractionDigits: params?.minimumFractionDigits, maximumFractionDigits: params?.maximumFractionDigits }).format(params?.val ?? 0);',
  '  } else {',
  '    for (const [name, value] of Object.entries(params || {})) out = out.split(`{{${name}}}`).join(String(value));',
  '  }',
  '  return out;',
  '}',
  '// Prop-style shims render children as-is, so the tooltip and label strings reach the SSR markup.',
  'function Tooltip({ children }) { return children; }',
  'function TooltipTrigger({ children }) { return children; }',
  'function TooltipContent({ children }) { return children; }',
  'function useTranslation() { return { t, i18n: { language: "en" } }; }',
  'function useGeneralSettings() { return { data: { currencyCode: "EUR" } }; }',
  windowLabelKeysDecl.getText(badgesAst),
  ...periodQuotaFuncNodes.map((node) => node.getText(badgesAst)),
  'function PeriodQuotaRow({ limits, currency }) { return <PeriodQuotaEstimate limits={limits} currency={currency} />; }',
  'export default PeriodQuotaRow;',
].join('\n');
const { outputText: periodQuotaRenderJs } = ts.transpileModule(periodQuotaRenderModule, {
  compilerOptions: { jsx: ts.JsxEmit.React, module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
});
const { default: PeriodQuotaRow } = await import(`data:text/javascript;base64,${Buffer.from(periodQuotaRenderJs).toString('base64')}`);

const renderPeriodQuota = (limits, currency) =>
  renderToStaticMarkup(React.createElement(PeriodQuotaRow, { limits, currency }))
    // renderToStaticMarkup HTML-escapes text; the hint assertions compare against
    // the plain locale strings.
    .replace(/&#x27;/g, "'")
    .replace(/&amp;/g, '&');

// Log-derived rows look like the windows the backend prices from usage logs
// (a pinned periodStart plus the ratio-aggregated money).
const logLimit = { window: '7d', periodStart: '2026-10-17T00:00:00Z', periodCost: 4.2, periodQuota: 21 };
// Provider-reported rows look like Synthetic's weekly pool: money without any
// window start, because the pool regenerates in ticks rather than resetting.
const providerLimit = { window: 'weekly', periodCost: 0.03, periodQuota: 10.5 };

test('period quota rows from usage logs keep the original log-derived hint', () => {
  const html = renderPeriodQuota([logLimit]);
  const hint = enSystem['quota.label.period_quota_hint'];
  assert.match(html, new RegExp(hint.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), 'log-only rows must show the log-derived hint');
  assert.ok(!html.includes(enSystem['quota.label.period_quota_hint_provider']), 'log-only rows must not show the provider hint');
  assert.ok(!html.includes('reported by provider'), 'log-only rows must not carry a provider source mark');
  // The money formatting is untouched by the source split (no explicit currency
  // falls back to the configured display currency, the test stand-in being EUR).
  assert.match(html, /€4\.20 used \/ ≈ €21\.00/);
});

test('provider-reported period money shows the provider-reported hint', () => {
  const html = renderPeriodQuota([providerLimit], 'USD');
  const hint = enSystem['quota.label.period_quota_hint_provider'];
  assert.match(html, new RegExp(hint.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), 'provider-only rows must show the provider-reported hint');
  assert.ok(!html.includes(enSystem['quota.label.period_quota_hint']), 'provider-only rows must not show the generic log-derived hint');
  assert.ok(!html.includes('reported by provider'), 'a single-source channel does not need per-row source marks');
  // Synthetic reported money renders in USD, matching the provider currency.
  assert.match(html, /\$0\.03 used \/ ≈ \$10\.50/);
});

test('mixed period quota rows get the dual-source hint and per-row provider marks', () => {
  const html = renderPeriodQuota([logLimit, providerLimit]);
  const mixedHint = enSystem['quota.label.period_quota_hint_mixed'];
  assert.match(html, new RegExp(mixedHint.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')), 'mixed rows must show the dual-source hint');
  // Both window labels render, but only the provider-reported row carries the
  // source attribution.
  assert.match(html, /Weekly window · reported by provider/);
  assert.match(html, /7d window(?! · reported)/);
  assert.equal((html.match(/reported by provider/g) || []).length, 1, 'exactly the provider-reported row is marked');
});

test('period quota block is absent without priced rows and unpriced limits render nothing', () => {
  assert.equal(renderPeriodQuota([]), '', 'no limits: no block');
  assert.equal(renderPeriodQuota([{ window: 'search_hourly', usageRatio: 0.5 }]), '', 'a limit without period money is not a priced row');
});

test('period quota hint keys are aligned between en and zh-CN locales', () => {
  const zhSystem = JSON.parse(read('locales/zh-CN/system.json'));
  for (const key of [
    'quota.label.period_quota_hint',
    'quota.label.period_quota_hint_provider',
    'quota.label.period_quota_hint_mixed',
    'quota.label.period_quota_provider_source',
  ]) {
    assert.ok(typeof enSystem[key] === 'string' && enSystem[key].length > 0, `en/system.json missing ${key}`);
    assert.ok(typeof zhSystem[key] === 'string' && zhSystem[key].length > 0, `zh-CN/system.json missing ${key}`);
  }
  // The generic key is the log-derived wording again; the provider explanation
  // no longer lives in it.
  assert.match(enSystem['quota.label.period_quota_hint'], /Estimated from the cost AxonHub logged/);
  assert.doesNotMatch(enSystem['quota.label.period_quota_hint'], /reports the window's money/);
  assert.match(zhSystem['quota.label.period_quota_hint'], /AxonHub 记录/);
});
