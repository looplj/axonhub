import type { ProviderModel } from './providers.schema';

/**
 * Reasoning effort levels AxonHub passes through unchanged (see llm/reasoning.go).
 * A model card lists the subset its upstream accepts.
 */
export const REASONING_EFFORTS = ['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'] as const;

export type ReasoningEffort = (typeof REASONING_EFFORTS)[number];

/**
 * Derives the supported levels from a catalog model's `reasoning_options`.
 *
 * Only the `effort` option enumerates levels. A `toggle` or `budget_tokens`
 * option says nothing about which levels the upstream accepts, and most models
 * carry no options at all, so those stay unknown rather than guessed.
 */
export function deriveReasoningEfforts(options: ProviderModel['reasoning_options']): ReasoningEffort[] | undefined {
  const values = options?.find((option) => option.type === 'effort')?.values?.filter((value): value is string => !!value);
  if (!values?.length) return undefined;

  const efforts = REASONING_EFFORTS.filter((effort) => values.includes(effort));

  return efforts.length ? efforts : undefined;
}
