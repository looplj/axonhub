import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const componentsDir = import.meta.dirname;
const srcRoot = join(componentsDir, '..');

function read(relativePath) {
  return readFileSync(join(srcRoot, relativePath), 'utf8');
}

// Isolate the Codex render branch (between the Codex and Cline branch
// markers) so assertions about the usage bars cannot bleed into Claude Code
// or Cline, which legitimately keep duration-aware severity.
function isolateCodexBlock(source) {
  const start = source.indexOf("{channel.type === 'codex' &&");
  const end = source.indexOf("{channel.type === 'cline' &&", start);

  assert.ok(start !== -1, 'Codex render branch should exist in quota-badges source');
  assert.ok(end !== -1 && end > start, 'Cline render branch should follow the Codex branch');

  return source.slice(start, end);
}

test('Codex usage bar color tracks used percentage, not reset-window elapsed time', () => {
  const quotaBadges = read('components/quota-badges.tsx');
  const codexBlock = isolateCodexBlock(quotaBadges);

  // Both usage bars must render the user-visible used percentage so their
  // severity reflects actual usage rather than elapsed reset-window time.
  assert.match(
    codexBlock,
    /percentage=\{qd\.rate_limit\.primary_window\.used_percent/,
    'Codex primary usage bar should render the primary window used percentage'
  );
  assert.match(
    codexBlock,
    /percentage=\{qd\.rate_limit\.secondary_window\.used_percent/,
    'Codex secondary usage bar should render the secondary window used percentage'
  );

  // Reset-window elapsed time stays on separate duration bars...
  assert.equal(
    (codexBlock.match(/ProgressBar\s*\n?\s*type='duration'/g) || []).length,
    2,
    'Codex should keep a separate duration bar for the primary and secondary windows'
  );

  // ...so the usage bars must NOT feed durationPercentage into ProgressBar,
  // which would severity-adjust their color by elapsed window time.
  assert.doesNotMatch(
    codexBlock,
    /durationPercentage/,
    'Codex usage bar color must not be severity-adjusted by reset-window elapsed time'
  );
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
