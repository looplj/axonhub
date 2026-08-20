import assert from 'node:assert/strict';
import test from 'node:test';
import { MAX_SEQUENTIAL_ANIMATED_ITEMS, getMaxSequentialAnimatedItems, planAnimatedListUpdate } from './animated-list.ts';

test('derives the animation limit from the duration budget and hard cap', () => {
  assert.equal(getMaxSequentialAnimatedItems(500), 3);
  assert.equal(getMaxSequentialAnimatedItems(300), 5);
  assert.equal(getMaxSequentialAnimatedItems(200), MAX_SEQUENTIAL_ANIMATED_ITEMS);
  assert.equal(getMaxSequentialAnimatedItems(1000), 1);
  assert.equal(getMaxSequentialAnimatedItems(2000), 0);
  assert.equal(getMaxSequentialAnimatedItems(0), 0);
});

test('queues top insertions within the animation budget from oldest to newest', () => {
  const previousIds = ['current-1', 'current-2', 'current-3', 'current-4', 'current-5'];
  const incomingIds = ['new-3', 'new-2', 'new-1', 'current-1', 'current-2'];

  assert.deepEqual(planAnimatedListUpdate(previousIds, incomingIds, [], incomingIds.length, 3), {
    shouldReplace: false,
    queuedIds: ['new-1', 'new-2', 'new-3'],
  });
});

test('replaces a snapshot when one update exceeds the animation budget', () => {
  const previousIds = ['current-1', 'current-2', 'current-3', 'current-4', 'current-5'];
  const incomingIds = ['new-4', 'new-3', 'new-2', 'new-1', 'current-1'];

  assert.deepEqual(planAnimatedListUpdate(previousIds, incomingIds, [], incomingIds.length, 3), {
    shouldReplace: true,
    queuedIds: [],
  });
});

test('replaces a snapshot when queued and newly added items exceed the limit', () => {
  assert.deepEqual(
    planAnimatedListUpdate(
      ['queued-2', 'queued-1', 'current-1', 'current-2', 'current-3'],
      ['new-2', 'new-1', 'queued-2', 'queued-1', 'current-1'],
      ['queued-1', 'queued-2'],
      5,
      3
    ),
    { shouldReplace: true, queuedIds: [] }
  );
});

test('replaces unrelated and structurally changed snapshots', () => {
  assert.equal(planAnimatedListUpdate(['page-2-a', 'page-2-b'], ['page-1-a', 'page-1-b'], []).shouldReplace, true);
  assert.equal(planAnimatedListUpdate(['a', 'b', 'c'], ['a', 'c'], []).shouldReplace, true);
  assert.equal(planAnimatedListUpdate(['a', 'b', 'c'], ['b', 'a', 'c'], []).shouldReplace, true);
  assert.equal(planAnimatedListUpdate(['a', 'b', 'c', 'd'], ['new', 'a', 'b'], []).shouldReplace, true);
});

test('replaces growing or changing snapshots that do not fill the page', () => {
  assert.equal(planAnimatedListUpdate(['a', 'b'], ['new', 'a', 'b'], [], 10).shouldReplace, true);
  assert.equal(planAnimatedListUpdate(['a', 'b', 'c'], ['new', 'a', 'b'], [], 10).shouldReplace, true);
});

test('replaces an oversized stale snapshot after the page size changes', () => {
  assert.equal(planAnimatedListUpdate(['a', 'b', 'c'], ['new', 'a', 'b'], [], 2).shouldReplace, true);
  assert.equal(planAnimatedListUpdate(['new', 'a', 'b'], ['new', 'a', 'b'], ['new'], 2).shouldReplace, true);
});

test('keeps only queued items that still exist in an unchanged snapshot', () => {
  assert.deepEqual(planAnimatedListUpdate(['new', 'current'], ['new', 'current'], ['stale', 'new', 'new']), {
    shouldReplace: false,
    queuedIds: ['new'],
  });
});

test('replaces queued updates when the page size grows beyond the current snapshot', () => {
  assert.deepEqual(planAnimatedListUpdate(['new', 'current'], ['new', 'current'], ['new'], 20), {
    shouldReplace: true,
    queuedIds: [],
  });
});
