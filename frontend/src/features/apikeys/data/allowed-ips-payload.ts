import type { UpdateApiKeyInput } from './schema';

// Builds the IP-restriction part of an API key update payload.
//
// The backend models presence and emptiness differently for this field: an empty
// `allowedIps` array means "leave unchanged" and only `clearAllowedIps` empties
// the stored allowlist. Turning the restriction off therefore has to clear
// explicitly, otherwise the mutation reports success while the old allowlist
// survives.
export function buildAllowedIpsUpdate(enabled: boolean, rawInput: string): Pick<UpdateApiKeyInput, 'allowedIps' | 'clearAllowedIps'> {
  if (!enabled) {
    return { clearAllowedIps: true };
  }

  return {
    allowedIps: rawInput
      .split(',')
      .map((entry) => entry.trim())
      .filter((entry) => entry !== ''),
  };
}
