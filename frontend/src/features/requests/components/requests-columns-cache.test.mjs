import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

// Pin the tokens column showing the cache hit rate alongside the reasoning count.
const source = readFileSync(new URL('./requests-columns.tsx', import.meta.url), 'utf8');
const tokensColumn = source.slice(source.indexOf("id: 'tokens'"), source.indexOf("id: 'readCache'"));

test('the tokens column shows the cache hit rate', () => {
  assert.match(tokensColumn, /promptCachedTokens \|\| 0/, 'the cache rate reads promptCachedTokens');
  assert.match(tokensColumn, /requests\.columns\.cacheRate/, 'the cache rate uses the cacheRate label');
});

test('the cache rate shares one line with the reasoning count', () => {
  assert.match(
    tokensColumn,
    /\(reasoningTokens > 0 \|\| cachedTokens > 0\) &&/,
    'the shared row renders when either reasoning or cache tokens exist'
  );
  assert.match(tokensColumn, /flex flex-wrap items-center/, 'the shared row lays its children out on one line');

  const sharedRow = tokensColumn.slice(tokensColumn.indexOf('(reasoningTokens > 0 || cachedTokens > 0) &&'));
  assert.match(sharedRow, /requests\.columns\.reasoning/, 'reasoning renders inside the shared row');
  assert.match(sharedRow, /requests\.columns\.cacheRate/, 'the cache rate renders inside the shared row');
});

test('the cache rate only renders when the cache was read', () => {
  assert.match(tokensColumn, /cachedTokens > 0 &&/, 'the cache rate is gated on a non-zero cache read');
});
