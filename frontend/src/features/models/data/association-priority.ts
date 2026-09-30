import { z } from 'zod';

// Priority range accepted by the association dialog. It mirrors the shared
// modelAssociationSchema (0-100) and the tiers in the Model Management Guide.
// The GraphQL field is an Int, so fractional values are rejected as well.
export const MAX_ASSOCIATION_PRIORITY = 100;

export const associationPrioritySchema = z
  .number({ error: 'Priority is required' })
  .int('Priority must be an integer')
  .min(0, 'Priority must be at least 0')
  .max(MAX_ASSOCIATION_PRIORITY, `Priority cannot exceed ${MAX_ASSOCIATION_PRIORITY}`);

// The preview query forwards the entered rules to a GraphQL Int input, so a
// value the priority field would reject (fractional, out of range) must not be
// sent. A cleared input (null) is pending rather than invalid, and its rule is
// incomplete anyway, so it does not block the preview.
export function hasInvalidAssociationPriority(priorities: ReadonlyArray<number | null | undefined>): boolean {
  return priorities.some(
    (priority) => priority !== null && priority !== undefined && !associationPrioritySchema.safeParse(priority).success
  );
}

// Priority for a newly added rule: one after the current highest priority so
// match order stays predictable. Once the cap is reached, further rules share
// priority 100 (same tier); the backend keeps a stable order within a tier.
export function nextAssociationPriority(priorities: ReadonlyArray<number | null | undefined>): number {
  const highest = Math.max(-1, ...priorities.filter((priority): priority is number => Number.isFinite(priority)));
  return Math.min(Math.floor(highest) + 1, MAX_ASSOCIATION_PRIORITY);
}
