import type { ChannelEndpoint } from './schema';

/**
 * The three relay protocols AxonHub can transparently pass through. A channel
 * that speaks all of them can serve virtually every client protocol without
 * lossy transformation.
 */
export const RELAY_PROTOCOLS = ['openai/chat_completions', 'openai/responses', 'anthropic/messages'] as const;

export type RelayProtocol = (typeof RELAY_PROTOCOLS)[number];

export function isRelayProtocol(apiFormat: string): apiFormat is RelayProtocol {
  return (RELAY_PROTOCOLS as readonly string[]).includes(apiFormat);
}

/**
 * Returns the relay protocols effectively available on a channel by merging the
 * built-in default endpoints with the user-configured overrides, mirroring the
 * backend `ResolveEndpoints` behavior.
 */
export function getChannelRelayProtocols(
  defaultEndpoints: readonly ChannelEndpoint[] | null | undefined,
  endpoints: readonly ChannelEndpoint[] | null | undefined
): Set<RelayProtocol> {
  const protocols = new Set<RelayProtocol>();

  for (const endpoint of [...(defaultEndpoints ?? []), ...(endpoints ?? [])]) {
    if (isRelayProtocol(endpoint.apiFormat)) {
      protocols.add(endpoint.apiFormat);
    }
  }

  return protocols;
}

export const RELAY_PROTOCOL_LABEL_KEYS: Record<RelayProtocol, string> = {
  'openai/chat_completions': 'channels.endpoints.detect.protocols.chat',
  'openai/responses': 'channels.endpoints.detect.protocols.responses',
  'anthropic/messages': 'channels.endpoints.detect.protocols.messages',
};
