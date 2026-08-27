import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import test from 'node:test';
import { providersDataSchema } from './providers.schema.ts';

const providersJsonPath = join(import.meta.dirname, 'providers.json');

function formatIssue(issue) {
  const path = issue.path.length > 0 ? issue.path.join('.') : '<root>';
  return `  ${path}: ${issue.message}`;
}

test('bundled providers.json matches providersDataSchema', () => {
  const raw = JSON.parse(readFileSync(providersJsonPath, 'utf8'));
  const result = providersDataSchema.safeParse(raw);

  if (!result.success) {
    const issues = result.error.issues;
    const shown = issues.slice(0, 10).map(formatIssue).join('\n');
    const rest = issues.length > 10 ? `\n  ... and ${issues.length - 10} more` : '';
    assert.fail(
      `providers.json (synced by scripts/sync/sync-model-developers.js) no longer matches the schema that providers.ts parses at import time.\nUpdate providers.schema.ts to accept the new shape:\n${shown}${rest}`
    );
  }

  assert.ok(Object.keys(result.data.providers).length > 0, 'bundled providers.json must contain at least one provider');
});
