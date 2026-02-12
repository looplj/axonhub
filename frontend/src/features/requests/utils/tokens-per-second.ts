'use client';

import { useState, useEffect } from 'react';
import { Request } from '../data/schema';

export type DisplayMode = 'latency' | 'tokensPerSecond';

const VALID_DISPLAY_MODES: DisplayMode[] = ['latency', 'tokensPerSecond'];

/**
 * Calculate tokens per second for a given request.
 * Handles all edge cases including no usage log, zero latency, zero completion tokens,
 * and streaming vs non-streaming scenarios.
 *
 * @param request - The request object containing usage logs and metrics
 * @returns Formatted string like "123 tok/s" or "-" if calculation is not possible
 */
export function calculateTokensPerSecond(request: Request): string {
  const usageLog = request.usageLogs?.edges?.[0]?.node;
  if (!usageLog || request.metricsLatencyMs == null || request.metricsLatencyMs <= 0) {
    return '-';
  }

  // Sum all completion token types (matching fastest performers logic)
  const completionTokens =
    (usageLog.completionTokens || 0) +
    (usageLog.completionReasoningTokens || 0) +
    (usageLog.completionAudioTokens || 0);

  if (completionTokens === 0) {
    return '-';
  }

  // Calculate effective latency:
  // For streaming: subtract TTFT (time to first token) to get actual generation time
  // For non-streaming: use full latency
  let effectiveLatencyMs = request.metricsLatencyMs;
  if (request.stream && request.metricsFirstTokenLatencyMs != null) {
    if (request.metricsFirstTokenLatencyMs < request.metricsLatencyMs) {
      effectiveLatencyMs = request.metricsLatencyMs - request.metricsFirstTokenLatencyMs;
    }
  }

  if (effectiveLatencyMs <= 0) {
    return '-';
  }

  const latencySeconds = effectiveLatencyMs / 1000;
  const tokensPerSecond = completionTokens / latencySeconds;

  return `${Math.round(tokensPerSecond)} tok/s`;
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