import assert from 'node:assert/strict';
import test from 'node:test';
import { isProjectSelectionValid } from './project-membership.ts';

test('returns false before user data resolves', () => {
  assert.equal(isProjectSelectionValid(undefined, 'proj-1'), false);
  assert.equal(isProjectSelectionValid(null, 'proj-1'), false);
});

test('returns true when no project is selected', () => {
  assert.equal(isProjectSelectionValid({ projects: [] }, null), true);
  assert.equal(isProjectSelectionValid({ projects: [] }, undefined), true);
});

test('returns false when the selected project is not in memberships', () => {
  assert.equal(
    isProjectSelectionValid({ projects: [{ projectID: 'proj-a' }] }, 'proj-b'),
    false,
  );
});

test('returns true when the selected project is in memberships', () => {
  assert.equal(
    isProjectSelectionValid({ projects: [{ projectID: 'proj-a' }] }, 'proj-a'),
    true,
  );
});