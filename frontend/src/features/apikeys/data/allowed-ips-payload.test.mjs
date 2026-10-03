import assert from 'node:assert/strict';
import test from 'node:test';

// Mirrors the payload built by the API key edit dialog. The backend ignores an
// empty allowedIps array, so turning the restriction off has to send
// clearAllowedIps instead; otherwise the update reports success while the
// stored restriction survives.
function buildUpdateInput({ ipRestrictionEnabled, ipInput }) {
  const input = { name: 'paid' };

  if (ipRestrictionEnabled) {
    input.allowedIps = ipInput
      .split(',')
      .map((entry) => entry.trim())
      .filter((entry) => entry !== '');
  } else {
    input.clearAllowedIps = true;
  }

  return input;
}

test('clearing the restriction sends clearAllowedIps instead of an empty array', () => {
  const input = buildUpdateInput({ ipRestrictionEnabled: false, ipInput: '103.112.1.155' });

  assert.equal(input.clearAllowedIps, true);
  assert.equal('allowedIps' in input, false);
});

test('an enabled restriction sends the parsed CIDR list and never clears', () => {
  const input = buildUpdateInput({ ipRestrictionEnabled: true, ipInput: ' 10.0.0.0/8 , 192.168.1.5, , ' });

  assert.deepEqual(input.allowedIps, ['10.0.0.0/8', '192.168.1.5']);
  assert.equal('clearAllowedIps' in input, false);
});

test('enabling the restriction with empty input leaves the list untouched', () => {
  const input = buildUpdateInput({ ipRestrictionEnabled: true, ipInput: '' });

  assert.deepEqual(input.allowedIps, []);
  assert.equal('clearAllowedIps' in input, false);
});
