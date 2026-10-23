import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
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

test('Ollama badge derives percentage from the heavier of the 5h/weekly windows', () => {
  const source = read('components/quota-badges.tsx');
  const start = source.indexOf('} else if (isOllamaType(channel.type)) {');
  const end = source.indexOf('} else if (', start + 5);
  const percentBlock = source.slice(start, end);

  assert.match(
    percentBlock,
    /Math\.max\(\s*qd\?\.windows\?\.\[['"]5h['"]\]\?\.usage_percent \?\? 0,\s*qd\?\.windows\?\.weekly\?\.usage_percent \?\? 0\s*\)/,
    'ollama badge percentage should be the max of the 5h and weekly window usage'
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

test('Wafer and Apertis duration markers share timestamp validation', () => {
  const quotaBadges = read('components/quota-badges.tsx');
  const waferStart = quotaBadges.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'wafer' &&");
  const waferEnd = quotaBadges.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'synthetic' &&");
  const apertisStart = quotaBadges.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'apertis' &&");
  const apertisEnd = quotaBadges.indexOf("{isOpenaiType(channel.type) && channel.providerType === 'charm_hyper' &&");

  assert.match(quotaBadges, /function getDurationPercent\([\s\S]*?Number\.isFinite\(start\)[\s\S]*?end <= start/);
  assert.match(quotaBadges.slice(waferStart, waferEnd), /durationPercent=\{getDurationPercent\(qd\.window_start, qd\.window_end\)\}/);
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
  'isOpenaiType',
  'isCommandCodeType',
  'isOllamaType',
  'isMinimaxType',
  'isZenmuxType',
  'getChannelCategory',
  'buildQuotaDisplayItems',
];
const modeHelperNodes = badgesAst.statements.filter((node) => ts.isFunctionDeclaration(node) && modeHelperNames.includes(node.name?.text));
assert.equal(modeHelperNodes.length, 9, 'helpers must stay in quota-badges.tsx');
const modeHelpers = modeHelperNodes.map((node) => `export ${node.getText(badgesAst).replace(/^export\s+/, '')}`).join('\n');

function loadTsModule(sourceText) {
  const { outputText } = ts.transpileModule(sourceText, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
  });
  return import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
}

const { resolveEffectiveRoutingMode, mostRestrictiveRoutingMode, getChannelCategory, buildQuotaDisplayItems } =
  await loadTsModule(modeHelpers);

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

test('account-grouped label uses the most restrictive member mode', () => {
  assert.equal(mostRestrictiveRoutingMode(['BACKPRESSURE', 'REMOVE_ON_EXHAUSTED']), 'BACKPRESSURE');
  assert.equal(mostRestrictiveRoutingMode(['REMOVE_ON_EXHAUSTED', 'IGNORE_QUOTA']), 'REMOVE_ON_EXHAUSTED');
  assert.equal(mostRestrictiveRoutingMode(['IGNORE_QUOTA', null]), 'IGNORE_QUOTA');
  // Unresolvable members are ignored; an all-unresolvable group shows no badge.
  assert.equal(mostRestrictiveRoutingMode([null, null]), null);
  assert.equal(mostRestrictiveRoutingMode([]), null);
});

test('grouped representatives render the group mode, standalone channels their own', () => {
  // The row list maps over display items and renders QuotaRow with effectiveMode.
  assert.match(badgesSource, /displayItems\.map\(\(item\)/);
  assert.match(
    badgesSource,
    /<QuotaRow key=\{item\.channel\.id\} channel=\{item\.channel\}\s+effectiveMode=\{effectiveModeFor\(item\.channel\)\}/
  );
  assert.match(
    badgesSource,
    /if \(!channel\.sharedAccountNames\) return resolveEffectiveRoutingMode\(channel\.quotaRoutingMode, routingSettings\?\.defaultMode\)/
  );
  assert.match(badgesSource, /\.filter\(\(c\) => c\.accountKey === channel\.accountKey\)/);
  assert.match(badgesSource, /mostRestrictiveRoutingMode\(groupModes\)/);
});

test('channels in quota popover are categorized with filter pills and section headers', () => {
  assert.match(badgesSource, /function getChannelCategory\(/, 'getChannelCategory helper must exist');
  // Categories no longer carry a hardcoded priority: the strip is alphabetical.
  assert.doesNotMatch(badgesSource, /order:\s*\d+/, 'categories must not embed hardcoded priority orders');
  assert.match(badgesSource, /catA\.label\.localeCompare\(catB\.label\)/, 'Category groups should sort alphabetically by label');
  assert.match(badgesSource, /a\.name\.localeCompare\(b\.name\)/, 'Channels should be stably sorted by name within categories');
  assert.match(badgesSource, /setSelectedCategory\('all'\)/, 'Popover should have an all categories filter pill');
  // Strip is hidden unless at least two categories have multiple channels —
  // pills for single-channel categories would just mirror the row below.
  assert.match(
    badgesSource,
    /filterableCategories = categories\.filter\(\(cat\) => cat\.count > 1\)/,
    'Filterable categories are the multi-channel ones'
  );
  assert.match(badgesSource, /filterableCategories\.length > 1/, 'Strip shows only when filtering can isolate two multi-channel groups');
  assert.match(badgesSource, /col-span-full[\s\S]*?uppercase/, 'Popover should render section header across full grid width');
  assert.match(badgesSource, /item\.cat\.count/, 'Section header counter must read from the category count');
  assert.match(badgesSource, /activeCategory[\s\S]*?categories\.some/, 'Selection must fall back to all when selected category disappears');
  assert.match(badgesSource, /text-foreground truncate font-medium|truncate font-medium/, 'Channel name should be truncated with ellipsis');
  assert.match(badgesSource, /ml-2 flex shrink-0/, 'Badge container should not shrink and have left margin');
  assert.match(
    badgesSource,
    /sortedChannels\.length > 4 \? 'grid grid-cols-1 gap-x-4 sm:grid-cols-2'/,
    'Dual-column grid should be based on total sorted channels'
  );
  assert.match(badgesSource, /aria-pressed=\{activeCategory === 'all'\}/, 'All button should expose aria-pressed state');
  assert.match(badgesSource, /aria-pressed=\{activeCategory === cat\.key\}/, 'Category buttons should expose aria-pressed state');
  // Filter strip stays on one line and scrolls horizontally instead of wrapping
  // (a wrapped strip grows the popover outside the 60vh body cap).
  assert.match(badgesSource, /flex items-center gap-1 overflow-x-auto border-b/, 'Filter strip should scroll horizontally on one line');
  assert.doesNotMatch(badgesSource, /flex flex-wrap items-center gap-1/, 'Filter strip must not wrap to multiple lines');
  assert.match(badgesSource, /shrink-0[^`]*whitespace-nowrap/, 'Pills must not shrink or wrap mid-label');
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

  const sorted = [...sampleChannels].sort((a, b) => {
    const catA = getChannelCategory(a, t);
    const catB = getChannelCategory(b, t);
    if (catA.label === catB.label) {
      return catA.key.localeCompare(catB.key) || a.name.localeCompare(b.name);
    }
    return catA.label.localeCompare(catB.label) || a.name.localeCompare(b.name);
  });

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
test('sort toggle cycles preferred, alphabetical, and creation order', () => {
  assert.match(
    badgesSource,
    /sortMode, setSortMode\] = useState<'preferred' \| 'alphabetical' \| 'created'>\('preferred'\)/,
    'Sort state defaults to preferred list order'
  );
  assert.match(badgesSource, /<ListOrdered className='h-4 w-4' \/>/, 'Preferred mode shows the ordered-list icon');
  assert.match(badgesSource, /<ArrowDownAZ className='h-4 w-4' \/>/, 'Alphabetical mode shows the A–Z icon');
  assert.match(badgesSource, /<History className='h-4 w-4' \/>/, 'Creation mode shows the history icon');
  assert.match(
    badgesSource,
    /system\.providerQuota\.sort\.(preferred|alphabetical|created)/,
    'Sort button tooltip/aria-label resolves locale keys'
  );
  // The preferred comparator ranks higher orderingWeight first; ties fall back
  // to the backend default (numeric id ASC).
  assert.match(
    badgesSource,
    /b\.orderingWeight - a\.orderingWeight \|\| a\.id\.localeCompare\(b\.id, 'en', \{ numeric: true \}\)/,
    'Preferred mode sorts by ordering weight desc with id tiebreak'
  );

  // The comparator used in created mode matches how the backend orders
  // channels: numeric ascending on the GUID tail.
  const byCreation = (a, b) => a.localeCompare(b, 'en', { numeric: true });
  assert.deepEqual(
    ['gid://axonhub/Channel/9', 'gid://axonhub/Channel/10', 'gid://axonhub/Channel/2'].sort(byCreation),
    ['gid://axonhub/Channel/2', 'gid://axonhub/Channel/9', 'gid://axonhub/Channel/10'],
    'Numeric compare must not sort ids lexicographically (2 < 10)'
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
