import assert from 'node:assert/strict';
import test from 'node:test';
import { buildAllowedIpsUpdate } from './allowed-ips-payload.ts';

// The backend reads an empty allowedIps array as "leave unchanged" and only
// empties the allowlist through clearAllowedIps, so clearing the restriction has
// to be signalled explicitly.
test('clearing the restriction sends clearAllowedIps instead of an empty array', () => {
  const input = buildAllowedIpsUpdate(false, '103.112.1.155');

  assert.deepEqual(input, { clearAllowedIps: true });
  assert.equal('allowedIps' in input, false);
});

test('an enabled restriction sends the parsed CIDR list and never clears', () => {
  const input = buildAllowedIpsUpdate(true, ' 10.0.0.0/8 , 192.168.1.5, , ');

  assert.deepEqual(input, { allowedIps: ['10.0.0.0/8', '192.168.1.5'] });
  assert.equal('clearAllowedIps' in input, false);
});

test('enabling the restriction with empty input leaves the list untouched', () => {
  const input = buildAllowedIpsUpdate(true, '');

  assert.deepEqual(input, { allowedIps: [] });
  assert.equal('clearAllowedIps' in input, false);
});
