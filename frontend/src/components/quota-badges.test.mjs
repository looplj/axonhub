import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

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
  assert.equal(
    (codexBlock.match(/<UsageTimeBar\s/g) || []).length,
    1,
    'Codex normalized limits should use one combined bar'
  );
  assert.match(
    codexBlock,
    /quota\.limits\s*\.filter\([\s\S]*?\.map\(\(limit,\s*index\)/,
    'Codex bars should render normalized limits'
  );
  assert.match(
    codexBlock,
    /limit\.window === '5h'/,
    'Codex five-hour label should be selected from the normalized window'
  );
  assert.match(
    codexBlock,
    /limit\.window === '7d'/,
    'Codex seven-day label should be selected from the normalized window'
  );
  assert.match(
    codexBlock,
    /WINDOW_LABEL_KEYS\[limit\.window\]/,
    'Codex labels should resolve through the shared translation map'
  );
  assert.match(
    codexBlock,
    /t\('quota\.label\.token_usage'\)/,
    'Codex unknown windows should use the neutral quota label'
  );
  assert.doesNotMatch(codexBlock, /quota\.label\.primary_window/);
  assert.doesNotMatch(codexBlock, /quota\.label\.secondary_window/);
});

test('quota popover has no standalone progress-bar renders', () => {
  const quotaBadges = read('components/quota-badges.tsx');

  assert.doesNotMatch(quotaBadges, /\bProgressBar\b/, 'all quota bars should use the combined UsageTimeBar component');
});

test('Command Code monthly hover matches the other windows', () => {
  const source = read('components/quota-badges.tsx');
  const commandCodeBlock = source.slice(source.indexOf("{isCommandCodeType(channel.type) &&"), source.indexOf("{channel.type === 'moonshot_coding' &&"));

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
  const start = source.indexOf("{isOllamaType(channel.type) &&");
  const end = source.indexOf("{isCommandCodeType(channel.type) &&", start);
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
