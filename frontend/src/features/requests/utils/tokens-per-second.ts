'use client';

import { useState, useEffect } from 'react';
import { Request } from '../data/schema';

export type DisplayMode = 'latency' | 'tokensPerSecond';

// Minimum latency value (in milliseconds) used for tokens/second calculations
// when a cache hit occurs (effective latency is zero or negative).
// This ensures we display a reasonable tokens/second value instead of infinity.
const MINIMUM_LATENCY_MS_FOR_CACHE_HITS = 10;

const VALID_DISPLAY_MODES: DisplayMode[] = ['latency', 'tokensPerSecond'];

/**
 * Calculate the numeric tokens-per-second rate for a given request.
 *
 * @param request - The request object containing usage logs and metrics
 * @returns The numeric rate or null if calculation is not possible
 */
export function getTokensPerSecondValue(request: Request): number | null {
  const usageLog = request.usageLogs?.edges?.[0]?.node;
  if (!usageLog || request.metricsLatencyMs == null || request.metricsLatencyMs <= 0) {
    return null;
  }

  // Use completion tokens only (reasoning tokens are not included in throughput calculation)
  const completionTokens = usageLog.completionTokens || 0;

  if (completionTokens === 0) {
    return null;
  }

  // Calculate effective latency:
  // For streaming: subtract TTFT (time to first token) to get actual generation time
  // For non-streaming: use full latency
  let effectiveLatencyMs = request.metricsLatencyMs;
  if (request.stream && request.metricsFirstTokenLatencyMs != null) {
    if (request.metricsFirstTokenLatencyMs <= request.metricsLatencyMs) {
      effectiveLatencyMs = request.metricsLatencyMs - request.metricsFirstTokenLatencyMs;
    }
  }

  // For cache hits (TTFT == Latency), effective latency becomes 0 or negative.
  // Use a fixed minimum to avoid division by zero and show reasonable tokens/second.
  if (effectiveLatencyMs <= 0) {
    effectiveLatencyMs = MINIMUM_LATENCY_MS_FOR_CACHE_HITS;
  }

  const latencySeconds = effectiveLatencyMs / 1000;
  return completionTokens / latencySeconds;
}

export function calculateTokensPerSecond(request: Request): string {
  const tokensPerSecond = getTokensPerSecondValue(request);
  return tokensPerSecond == null ? '-' : `${Math.round(tokensPerSecond)} tok/s`;
}

/**
 * Hook to manage display mode state with localStorage persistence.
 * SSR-safe: defaults to 'latency' during server-side rendering.
 * Uses two-phase initialization to avoid hydration mismatches.
 *
 * @returns Tuple of [displayMode, setDisplayMode] similar to useState
 */
export function useDisplayMode(): [DisplayMode, React.Dispatch<React.SetStateAction<DisplayMode>>] {
  const [displayMode, setDisplayMode] = useState<DisplayMode>(() => {
    if (typeof window !== 'undefined') {
      const stored = localStorage.getItem('requests-table-latency-display-mode');
      if (stored && VALID_DISPLAY_MODES.includes(stored as DisplayMode)) {
        return stored as DisplayMode;
      }
    }
    return 'latency';
  });

  useEffect(() => {
    if (typeof window !== 'undefined') {
      localStorage.setItem('requests-table-latency-display-mode', displayMode);
    }
  }, [displayMode]);

  return [displayMode, setDisplayMode];
}
