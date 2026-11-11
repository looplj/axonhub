import assert from 'node:assert/strict';
import test from 'node:test';
import { createTourNavigation, waitForVisibleElement } from './onboarding-tour.ts';

function setup(t) {
  const elements = new Map();
  const previous = globalThis.document;
  globalThis.document = { querySelector: (selector) => elements.get(selector) };
  t.after(() => { globalThis.document = previous; });
  t.mock.timers.enable({ apis: ['setTimeout', 'Date'] });
  const show = (selector) => elements.set(selector, { checkVisibility: () => true });
  return { elements, show };
}

const flush = async () => { for (let i = 0; i < 10; i++) await Promise.resolve(); };

test('waits for mounted, visible targets before highlighting in both directions', async (t) => {
  const { elements, show } = setup(t);
  const destinations = [];
  const highlights = [];
  const steps = [
    { destination: 'brand', element: '#brand-logo-upload' },
    { destination: 'retry', element: '[data-value="retry"]' },
    { destination: 'storage', element: '[data-value="storage"]' },
  ];
  const tour = createTourNavigation({
    steps,
    navigate: async (destination) => { destinations.push(destination); elements.clear(); },
    driver: { drive: (index) => highlights.push(index) },
    onError: () => assert.fail('unexpected failure'),
  });
  t.after(tour.cancel);
  const ready = ['#brand-name', '#retry-enabled-switch', '#default-data-storage'];
  for (const index of [0, 1, 2, 1, 0]) {
    const previousCount = highlights.length;
    const move = tour.moveTo(index, index < 2 ? -1 : 1);
    await flush();
    assert.equal(highlights.length, previousCount);
    show(ready[index]);
    elements.set(steps[index].element, { checkVisibility: () => false });
    t.mock.timers.tick(50);
    await flush();
    assert.equal(highlights.length, previousCount, 'hidden target must not be highlighted');
    show(steps[index].element);
    t.mock.timers.tick(50);
    await move;
    assert.equal(highlights.at(-1), index);
  }
  assert.deepEqual(destinations, ['brand', 'retry', 'storage', 'retry', 'brand']);
});

test('closing during navigation prevents a late highlight', async (t) => {
  setup(t);
  let resolveNavigation;
  const tour = createTourNavigation({
    steps: [{ destination: 'general', element: '#system-title' }],
    navigate: () => new Promise((resolve) => { resolveNavigation = resolve; }),
    driver: { drive: () => assert.fail('tour reopened') },
    onError: () => assert.fail('cancellation reported as failure'),
  });
  const move = tour.moveTo(0);
  tour.cancel();
  resolveNavigation();
  await move;
});

test('closing while waiting cancels polling and ignores repeated navigation clicks', async (t) => {
  setup(t);
  let navigations = 0;
  const tour = createTourNavigation({
    steps: [{ destination: 'brand', element: '#brand-name' }],
    navigate: async () => { navigations++; },
    driver: { drive: () => assert.fail('tour reopened') },
    onError: () => assert.fail('cancellation reported as failure'),
  });
  const move = tour.moveTo(0);
  await flush();
  await tour.moveTo(0);
  assert.equal(navigations, 1);
  tour.cancel();
  await move;
  t.mock.timers.tick(10000);
});

test('missing targets time out instead of highlighting a dummy element', async (t) => {
  setup(t);
  const controller = new AbortController();
  const wait = waitForVisibleElement('#missing', controller.signal);
  const rejected = assert.rejects(wait, /Tour target unavailable/);
  t.mock.timers.tick(10000);
  await rejected;
});

test('disabled retry details are skipped forward and backward without changing settings', async (t) => {
  const { elements, show } = setup(t);
  show('#retry-enabled-switch');
  show('#default-data-storage');
  elements.set('#retry-enabled', { getAttribute: () => 'false' });
  const highlights = [];
  const destinations = [];
  const tour = createTourNavigation({
    steps: [
      { destination: 'retry', element: '#retry-enabled-switch' },
      { destination: 'retry', element: '#retry-delay', requiresRetryEnabled: true },
      { destination: 'retry', requiresRetryEnabled: true },
      { destination: 'storage', element: '#default-data-storage' },
    ],
    navigate: async (destination) => { destinations.push(destination); },
    driver: { drive: (index) => highlights.push(index) },
    onError: () => assert.fail('unexpected failure'),
  });
  t.after(tour.cancel);
  await tour.moveTo(1);
  await tour.moveTo(2, -1);
  assert.deepEqual(highlights, [3, 0]);
  assert.deepEqual(destinations, ['retry', 'storage', 'retry']);
});
