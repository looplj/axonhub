import assert from 'node:assert/strict';
import test from 'node:test';
import { buildPaginationArgs, updateCursorHistory } from './pagination-search-state.ts';

/**
 * Simulate the URL-driven cursor walk used by the list pages:
 * `setCursors(startCursor, endCursor, direction)` records the displayed page's
 * cursors, then `buildPaginationArgs` decides which page the next fetch lands on.
 */
function createPager(pages) {
  return {
    history: [],
    startCursor: undefined,
    endCursor: undefined,
    direction: 'after',
    // Pages keyed by their fetch boundary: undefined -> 1, e<n> -> n + 1.
    fetch() {
      const args = buildPaginationArgs(this.direction, this.history, this.endCursor, 20);
      assert.deepEqual(args.first, 20);
      const pageIndex = args.after === undefined ? 1 : pages.findIndex((page) => page.endCursor === args.after) + 2;
      return pages[pageIndex - 1] ?? null;
    },
    moveTo(nextPage, direction) {
      this.history = updateCursorHistory(this.history, direction, nextPage.endCursor);
      this.startCursor = nextPage.startCursor;
      this.endCursor = nextPage.endCursor;
      this.direction = direction;
      return this.fetch();
    },
    next() {
      return this.moveTo(this.fetch(), 'after');
    },
    previous() {
      return this.moveTo(this.fetch(), 'before');
    },
  };
}

const pages = [
  { startCursor: 's1', endCursor: 'e1' },
  { startCursor: 's2', endCursor: 'e2' },
  { startCursor: 's3', endCursor: 'e3' },
  { startCursor: 's4', endCursor: 'e4' },
];

test('forward walk records the boundary of each visited page', () => {
  const pager = createPager(pages);

  assert.equal(pager.fetch(), pages[0]);
  pager.next();
  assert.deepEqual(pager.history, ['e1']);
  pager.next();
  assert.deepEqual(pager.history, ['e1', 'e2']);
  pager.next();
  assert.deepEqual(pager.history, ['e1', 'e2', 'e3']);
  assert.equal(pager.fetch(), pages[3]);
});

test('moving back from the last page lands on the immediately previous page', () => {
  const pager = createPager(pages);

  pager.next();
  pager.next();
  pager.next();
  assert.equal(pager.fetch(), pages[3]);

  assert.equal(pager.previous(), pages[2]);
});

test('back-then-forward-then-back walks one page at a time without an empty page', () => {
  const pager = createPager(pages);

  pager.next();
  pager.next();
  pager.next();
  assert.equal(pager.fetch(), pages[3]);

  assert.equal(pager.previous(), pages[2]);
  assert.equal(pager.next(), pages[3]);
  assert.equal(pager.previous(), pages[2]);
  assert.equal(pager.previous(), pages[1]);
});

test('moving back from the first page re-enters the first page', () => {
  const pager = createPager(pages);

  assert.equal(pager.previous(), pages[0]);
  assert.deepEqual(pager.history, []);
});

test('backward fetches from the history boundary, forward from the current end cursor', () => {
  assert.deepEqual(buildPaginationArgs('before', ['e1', 'e2'], 'e9', 10), { first: 10, after: 'e2' });
  assert.deepEqual(buildPaginationArgs('after', ['e1'], 'e9', 10), { first: 10, after: 'e9' });
  assert.deepEqual(buildPaginationArgs('before', [], 'e9', 10), { first: 10, after: undefined });
});
