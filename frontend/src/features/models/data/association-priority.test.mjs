import assert from 'node:assert/strict';
import test from 'node:test';
import { MAX_ASSOCIATION_PRIORITY, associationPrioritySchema, nextAssociationPriority } from './association-priority.ts';

function firstIssue(value) {
  const result = associationPrioritySchema.safeParse(value);
  return result.success ? null : result.error.issues[0].message;
}

test('association priority accepts the documented 0-100 range', () => {
  assert.equal(MAX_ASSOCIATION_PRIORITY, 100);
  for (const priority of [0, 1, 10, 50, 99, 100]) {
    assert.equal(firstIssue(priority), null, `priority ${priority} should be valid`);
  }
});

test('association priority reports out-of-range values instead of clamping them', () => {
  assert.equal(firstIssue(101), 'Priority cannot exceed 100');
  assert.equal(firstIssue(-1), 'Priority must be at least 0');
});

test('association priority rejects non-integers because the GraphQL field is an Int', () => {
  assert.equal(firstIssue(1.5), 'Priority must be an integer');
  assert.equal(firstIssue(99.9), 'Priority must be an integer');
});

test('association priority requires a value once the input is cleared', () => {
  // The dialog stores null for an empty input; undefined and NaN cannot be
  // produced by a number input but must not slip through either.
  assert.equal(firstIssue(null), 'Priority is required');
  assert.equal(firstIssue(undefined), 'Priority is required');
  assert.equal(firstIssue(Number.NaN), 'Priority is required');
});

test('new rules take the next priority after the current highest one', () => {
  assert.equal(nextAssociationPriority([]), 0);
  assert.equal(nextAssociationPriority([0]), 1);
  assert.equal(nextAssociationPriority([20, 5]), 21);
  assert.equal(nextAssociationPriority([99]), 100);
});

test('new rules saturate at the cap and share priority 100 (same tier)', () => {
  assert.equal(nextAssociationPriority([100]), 100);
  assert.equal(nextAssociationPriority([100, 100]), 100);
  assert.equal(nextAssociationPriority([150]), 100);
});

test('new rules ignore cleared inputs and normalize pending invalid values', () => {
  assert.equal(nextAssociationPriority([null]), 0);
  assert.equal(nextAssociationPriority([3, null, undefined]), 4);
  assert.equal(nextAssociationPriority([1.5]), 2);
  assert.equal(nextAssociationPriority([-5]), 0);
});
