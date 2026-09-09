// Structural guard: every `settings { ... }` selection block in channels.ts must
// request `quotaRoutingMode`, so a new or edited query/mutation cannot silently
// drop the field and starve the zod schema / merge whitelist downstream.
import assert from 'node:assert/strict';
import test from 'node:test';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const source = readFileSync(join(import.meta.dirname, 'channels.ts'), 'utf8');
const lines = source.split('\n');

function enclosingDeclaration(lineIndex) {
  for (let i = lineIndex; i >= 0; i--) {
    const m = lines[i].match(/const (\w+)\s*=/);
    if (m) return m[1];
  }
  return `line ${lineIndex + 1}`;
}

function extractSettingsBlocks() {
  const blocks = [];
  for (let i = 0; i < lines.length; i++) {
    const open = lines[i].indexOf('settings {');
    if (open === -1) continue;
    let depth = 0;
    let body = '';
    for (let j = i; j < lines.length; j++) {
      const line = j === i ? lines[j].slice(open) : lines[j];
      for (const ch of line) {
        if (ch === '{') depth++;
        else if (ch === '}') depth--;
      }
      body += `${line}\n`;
      if (depth === 0) break;
    }
    blocks.push({ name: enclosingDeclaration(i), body });
  }
  return blocks;
}

test('every settings selection block in channels.ts requests quotaRoutingMode', () => {
  const blocks = extractSettingsBlocks();
  assert.ok(blocks.length >= 1, 'no `settings {` blocks found — selection extraction is broken');

  const missing = blocks.filter((b) => !/\bquotaRoutingMode\b/.test(b.body));
  assert.deepEqual(
    missing.map((b) => b.name),
    [],
    `settings selection block(s) missing quotaRoutingMode: ${missing.map((b) => b.name).join(', ')}`
  );
});
