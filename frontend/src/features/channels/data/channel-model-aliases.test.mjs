import assert from 'node:assert/strict';
import test from 'node:test';
import { filterChannelModelGroups, groupChannelModelEntries } from './channel-model-aliases.ts';

const entries = [
  { requestModel: 'Vendor/GPT', actualModel: 'Vendor/GPT', source: 'direct' },
  { requestModel: 'chat', actualModel: 'Vendor/GPT', source: 'mapping' },
  { requestModel: 'public/Vendor/GPT', actualModel: 'Vendor/GPT', source: 'prefix' },
  { requestModel: 'gpt-mini', actualModel: 'GPT-Mini', source: 'direct' },
];

test('groups every exposed alias under its upstream model while keeping hidden models', () => {
  const groups = groupChannelModelEntries(['Vendor/GPT', 'GPT-Mini', 'hidden'], entries);
  assert.equal(groups.length, 3);
  assert.equal(groups[0].originalVisible, true);
  assert.deepEqual(groups[0].aliases.map(alias => alias.requestModel), ['chat', 'public/Vendor/GPT']);
  assert.equal(groups[1].originalVisible, false);
  assert.equal(groups[1].aliases[0].requestModel, 'gpt-mini');
  assert.deepEqual(groups[2], { model: 'hidden', originalVisible: false, aliases: [] });
});

test('searching an alias keeps its real model and sibling aliases', () => {
  const groups = groupChannelModelEntries(['Vendor/GPT', 'GPT-Mini'], entries);
  assert.deepEqual(filterChannelModelGroups(groups, ' CHAT '), [groups[0]]);
  assert.deepEqual(filterChannelModelGroups(groups, 'vendor'), [groups[0]]);
  assert.deepEqual(filterChannelModelGroups(groups, ''), groups);
  assert.deepEqual(filterChannelModelGroups(groups, 'missing'), []);
});

test('removed upstream models do not reappear through stale preview entries', () => {
  assert.deepEqual(groupChannelModelEntries([], entries), []);
  assert.equal(groupChannelModelEntries(['GPT-Mini', 'GPT-Mini'], entries).length, 1);
});
