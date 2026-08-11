import assert from 'node:assert/strict';
import test from 'node:test';
import {
  AUTO_REFRESH_INTERVALS,
  parseAutoRefreshInterval,
  readAutoRefreshInterval,
  readBrowserAutoRefreshInterval,
  writeAutoRefreshInterval,
} from './use-auto-refresh-interval.ts';

class MemoryStorage {
  #values = new Map();

  getItem(key) {
    return this.#values.get(key) ?? null;
  }

  setItem(key, value) {
    this.#values.set(key, value);
  }

  removeItem(key) {
    this.#values.delete(key);
  }
}

test('accepts only supported auto-refresh intervals', () => {
  for (const interval of AUTO_REFRESH_INTERVALS) {
    assert.equal(parseAutoRefreshInterval(interval.toString()), interval);
  }

  for (const value of [null, '', '0', '5001', '05000', '10000 ', 'invalid']) {
    assert.equal(parseAutoRefreshInterval(value), null);
  }
});

test('removes invalid stored values', () => {
  const storage = new MemoryStorage();
  storage.setItem('refresh-key', 'invalid');

  assert.equal(readAutoRefreshInterval('refresh-key', storage), null);
  assert.equal(storage.getItem('refresh-key'), null);
});

test('returns disabled when the browser localStorage accessor throws', () => {
  const previousWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
  const throwingWindow = {};
  Object.defineProperty(throwingWindow, 'localStorage', {
    get() {
      throw new Error('localStorage is blocked');
    },
  });
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: throwingWindow,
  });

  try {
    assert.equal(readBrowserAutoRefreshInterval('refresh-key'), null);
  } finally {
    if (previousWindow) {
      Object.defineProperty(globalThis, 'window', previousWindow);
    } else {
      delete globalThis.window;
    }
  }
});

test('persists intervals independently and removes disabled settings', () => {
  const storage = new MemoryStorage();

  writeAutoRefreshInterval('requests', 5000, storage);
  writeAutoRefreshInterval('traces', 30000, storage);

  assert.equal(readAutoRefreshInterval('requests', storage), 5000);
  assert.equal(readAutoRefreshInterval('traces', storage), 30000);

  writeAutoRefreshInterval('requests', null, storage);

  assert.equal(storage.getItem('requests'), null);
  assert.equal(readAutoRefreshInterval('traces', storage), 30000);
});
