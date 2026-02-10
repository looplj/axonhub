import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

// Clean up after each test
afterEach(() => {
  cleanup();
});

// Mock window for SSR tests
Object.defineProperty(global, 'window', {
  value: global.window || {},
  writable: true,
});

// Mock localStorage
const localStorageMock = {
  getItem: vi.fn(),
  setItem: vi.fn(),
  clear: vi.fn(),
  removeItem: vi.fn(),
};

Object.defineProperty(global, 'localStorage', {
  value: localStorageMock,
  writable: true,
});

// Suppress console errors in tests
const originalError = console.error;
beforeAll(() => {
  console.error = (...args) => {
    if (
      args[0]?.includes?.('Warning: An update to') ||
      args[0]?.includes?.('Invalid hook call')
    ) {
      return;
    }
    originalError.call(console, ...args);
  };
});

afterAll(() => {
  console.error = originalError;
});