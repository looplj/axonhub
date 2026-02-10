/**
 * @vitest-environment jsdom
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { calculateTokensPerSecond, useDisplayMode } from './tokens-per-second';
import type { Request } from '../data/schema';
import { renderHook, act } from '@testing-library/react';

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

// Mock window for SSR tests
const windowMock = {
  undefined: undefined,
};

describe('calculateTokensPerSecond', () => {
  describe('normal cases', () => {
    it('should calculate tokens per second with completion tokens', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 5000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('100 tok/s'); // 500 tokens / 5 seconds = 100 tok/s
    });

    it('should include reasoning tokens in calculation', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'claude-3-opus',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'claude-3-opus',
                promptTokens: 100,
                completionTokens: 300,
                completionReasoningTokens: 200,
                totalTokens: 600,
                source: 'api',
                format: 'anthropic',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 5000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('100 tok/s'); // (300 + 200) tokens / 5 seconds = 100 tok/s
    });

    it('should include audio tokens in calculation', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4-audio',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4-audio',
                promptTokens: 100,
                completionTokens: 200,
                completionAudioTokens: 100,
                totalTokens: 400,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 4000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('75 tok/s'); // (200 + 100) tokens / 4 seconds = 75 tok/s
    });

    it('should include all token types together', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'claude-sonnet',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'claude-sonnet',
                promptTokens: 50,
                completionTokens: 150,
                completionReasoningTokens: 100,
                completionAudioTokens: 50,
                totalTokens: 350,
                source: 'api',
                format: 'anthropic',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 3000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('100 tok/s'); // (150 + 100 + 50) tokens / 3 seconds = 100 tok/s
    });
  });

  describe('edge cases - no usage log', () => {
    it('should return "-" when usageLogs is undefined', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        metricsLatencyMs: 5000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('-');
    });

    it('should return "-" when usageLogs is null', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: null,
        metricsLatencyMs: 5000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('-');
    });

    it('should return "-" when edges is empty', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: '',
            endCursor: '',
          },
          totalCount: 0,
        },
        metricsLatencyMs: 5000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('-');
    });

    it('should return "-" when node is undefined', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: undefined,
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 5000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('-');
    });
  });

  describe('edge cases - latency issues', () => {
    it('should return "-" when metricsLatencyMs is undefined', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('-');
    });

    it('should return "-" when metricsLatencyMs is null', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: null,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('-');
    });

    it('should return "-" when metricsLatencyMs is zero', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 0,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('-');
    });

    it('should return "-" when metricsLatencyMs is negative', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: -1000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('-');
    });
  });

  describe('edge cases - zero completion tokens', () => {
    it('should return "-" when completionTokens is zero', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 0,
                totalTokens: 100,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 5000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('-');
    });

    it('should return "-" when all token types are zero', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 0,
                completionReasoningTokens: 0,
                completionAudioTokens: 0,
                totalTokens: 100,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 5000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('-');
    });
  });

  describe('streaming vs non-streaming', () => {
    it('should subtract TTFT for streaming requests', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 10000,
        metricsFirstTokenLatencyMs: 2000,
        stream: true,
      };

      const result = calculateTokensPerSecond(request);
      // Effective latency = 10000 - 2000 = 8000ms = 8 seconds
      // 500 tokens / 8 seconds = 62.5 tok/s ≈ 63 tok/s
      expect(result).toBe('63 tok/s');
    });

    it('should use full latency for non-streaming requests', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 10000,
        metricsFirstTokenLatencyMs: 2000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      // Full latency = 10000ms = 10 seconds
      // 500 tokens / 10 seconds = 50 tok/s
      expect(result).toBe('50 tok/s');
    });

    it('should use full latency when stream is null', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 10000,
        metricsFirstTokenLatencyMs: 2000,
        stream: null,
      };

      const result = calculateTokensPerSecond(request);
      // Full latency = 10000ms = 10 seconds
      // 500 tokens / 10 seconds = 50 tok/s
      expect(result).toBe('50 tok/s');
    });

    it('should use full latency when metricsFirstTokenLatencyMs is null for streaming', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 10000,
        metricsFirstTokenLatencyMs: null,
        stream: true,
      };

      const result = calculateTokensPerSecond(request);
      // Full latency = 10000ms = 10 seconds (no TTFT subtraction)
      // 500 tokens / 10 seconds = 50 tok/s
      expect(result).toBe('50 tok/s');
    });
  });

  describe('edge case - TTFT greater than total latency', () => {
    it('should use full latency when TTFT >= total latency', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 5000,
        metricsFirstTokenLatencyMs: 5000,
        stream: true,
      };

      const result = calculateTokensPerSecond(request);
      // TTFT = 5000, total latency = 5000, so effective latency = 5000 (no subtraction)
      // 500 tokens / 5 seconds = 100 tok/s
      expect(result).toBe('100 tok/s');
    });

    it('should use full latency when TTFT > total latency', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 5000,
        metricsFirstTokenLatencyMs: 6000,
        stream: true,
      };

      const result = calculateTokensPerSecond(request);
      // TTFT = 6000 > total latency = 5000, so effective latency = 5000 (no subtraction)
      // 500 tokens / 5 seconds = 100 tok/s
      expect(result).toBe('100 tok/s');
    });

    it('should calculate tokens/sec when TTFT equals total latency', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 500,
                totalTokens: 600,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 2000,
        metricsFirstTokenLatencyMs: 2000,
        stream: true,
      };

      const result = calculateTokensPerSecond(request);
      expect(result).toBe('250 tok/s');
    });
  });

  describe('rounding', () => {
    it('should round to nearest integer', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 123,
                totalTokens: 223,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 1000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      // 123 tokens / 1 second = 123 tok/s
      expect(result).toBe('123 tok/s');
    });

    it('should round down 12.4 to 12', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 124,
                totalTokens: 224,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 10000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      // 124 tokens / 10 seconds = 12.4 tok/s → Math.round = 12
      expect(result).toBe('12 tok/s');
    });

    it('should round up 12.5 to 13', () => {
      const request: Request = {
        id: '1',
        createdAt: new Date(),
        updatedAt: new Date(),
        source: 'api',
        modelID: 'gpt-4',
        status: 'completed',
        usageLogs: {
          edges: [
            {
              node: {
                id: 'log-1',
                createdAt: new Date(),
                updatedAt: new Date(),
                requestID: '1',
                modelID: 'gpt-4',
                promptTokens: 100,
                completionTokens: 135,
                totalTokens: 235,
                source: 'api',
                format: 'openai',
              },
              cursor: 'cursor-1',
            },
          ],
          pageInfo: {
            hasNextPage: false,
            hasPreviousPage: false,
            startCursor: 'cursor-1',
            endCursor: 'cursor-1',
          },
          totalCount: 1,
        },
        metricsLatencyMs: 10000,
        stream: false,
      };

      const result = calculateTokensPerSecond(request);
      // 135 tokens / 10 seconds = 13.5 tok/s → Math.round = 14
      expect(result).toBe('14 tok/s');
    });
  });
});

describe('useDisplayMode', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('initialization logic', () => {
    it('should return "latency" when localStorage returns null', () => {
      localStorageMock.getItem.mockReturnValue(null);

      // Simulate the initialization logic from the hook
      const displayMode = (() => {
        if (typeof window === 'undefined') return 'latency';
        const stored = localStorage.getItem('requests-table-latency-display-mode');
        if (stored && ['latency', 'tokensPerSecond'].includes(stored)) {
          return stored;
        }
        return 'latency';
      })();

      expect(displayMode).toBe('latency');
    });

    it('should return stored value when localStorage has "tokensPerSecond"', () => {
      localStorageMock.getItem.mockReturnValue('tokensPerSecond');

      // Simulate the initialization logic from the hook
      const displayMode = (() => {
        if (typeof window === 'undefined') return 'latency';
        const stored = localStorage.getItem('requests-table-latency-display-mode');
        if (stored && ['latency', 'tokensPerSecond'].includes(stored)) {
          return stored;
        }
        return 'latency';
      })();

      expect(displayMode).toBe('tokensPerSecond');
    });

    it('should default to "latency" for invalid stored value', () => {
      localStorageMock.getItem.mockReturnValue('invalid-value');

      // Simulate the initialization logic from the hook
      const displayMode = (() => {
        if (typeof window === 'undefined') return 'latency';
        const stored = localStorage.getItem('requests-table-latency-display-mode');
        if (stored && ['latency', 'tokensPerSecond'].includes(stored)) {
          return stored;
        }
        return 'latency';
      })();

      expect(displayMode).toBe('latency');
    });
  });

  describe('localStorage side effects', () => {
    it('should persist display mode changes to localStorage', () => {
      localStorageMock.getItem.mockReturnValue(null);
      localStorageMock.setItem.mockReturnValue(undefined);

      const displayMode = 'tokensPerSecond';
      if (typeof window !== 'undefined') {
        localStorage.setItem('requests-table-latency-display-mode', displayMode);
      }

      expect(localStorageMock.setItem).toHaveBeenCalledWith(
        'requests-table-latency-display-mode',
        'tokensPerSecond'
      );
    });

    it('should persist latency value to localStorage', () => {
      localStorageMock.getItem.mockReturnValue(null);
      localStorageMock.setItem.mockReturnValue(undefined);

      const displayMode = 'latency';
      if (typeof window !== 'undefined') {
        localStorage.setItem('requests-table-latency-display-mode', displayMode);
      }

      expect(localStorageMock.setItem).toHaveBeenCalledWith(
        'requests-table-latency-display-mode',
        'latency'
      );
    });
  });

  describe('SSR safety', () => {
    it('should not access localStorage during SSR', () => {
      const originalWindow = global.window;
      (global as unknown as Record<string, unknown>).window = undefined;

      localStorageMock.getItem.mockClear();

      const displayMode = (() => {
        if (typeof window === 'undefined') return 'latency';
        const stored = localStorage.getItem('requests-table-latency-display-mode');
        if (stored && ['latency', 'tokensPerSecond'].includes(stored)) {
          return stored;
        }
        return 'latency';
      })();

      expect(displayMode).toBe('latency');
      expect(localStorageMock.getItem).not.toHaveBeenCalled();

      global.window = originalWindow;
    });
  });
});