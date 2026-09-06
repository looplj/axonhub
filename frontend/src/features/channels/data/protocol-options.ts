import type { ApiFormat, ChannelType, ModelProtocol } from './schema';

type ProtocolEndpoint = {
  readonly apiFormat?: string | null;
};

export function getAvailableProtocolFormats(
  defaultEndpoints: readonly ProtocolEndpoint[],
  endpoints: readonly ProtocolEndpoint[]
): string[] {
  const formats = new Set<string>();

  for (const endpoint of [...defaultEndpoints, ...endpoints]) {
    if (typeof endpoint.apiFormat === 'string' && endpoint.apiFormat.length > 0) {
      formats.add(endpoint.apiFormat);
    }
  }

  return Array.from(formats);
}

type ProviderConfig = {
  readonly channelTypes: readonly ChannelType[];
};

type ChannelConfig = {
  readonly apiFormat?: ApiFormat | null;
};

export type ProtocolConfigs = {
  readonly providerConfigs: Readonly<Record<string, ProviderConfig>>;
  readonly channelConfigs: Readonly<Record<string, ChannelConfig>>;
};

export function getApiFormatsForProvider(provider: string, configs: ProtocolConfigs): ApiFormat[] {
  const providerConfig = configs.providerConfigs[provider];
  if (!providerConfig) return [];

  const formats: ApiFormat[] = [];
  for (const channelType of providerConfig.channelTypes) {
    const apiFormat = configs.channelConfigs[channelType]?.apiFormat;
    if (apiFormat && !formats.includes(apiFormat)) {
      formats.push(apiFormat);
    }
  }

  // Native video is a ZenMux channel type, so it is only offered when the
  // provider actually includes the ZenMux channel type in its config.
  if (providerConfig.channelTypes.includes('zenmux') && !formats.includes('zenmux/video')) {
    formats.push('zenmux/video');
  }

  return formats;
}

/**
 * Custom endpoint formats the endpoints dialog may offer for a channel.
 * Native video is a ZenMux channel type, so it must not be offered on other
 * channel types. Other custom endpoint formats remain available everywhere.
 */
export function getConfigurableApiFormatsForChannelType(
  channelType: ChannelType,
  configurableFormats: readonly string[]
): string[] {
  if (channelType === 'zenmux') {
    return [...configurableFormats];
  }
  return configurableFormats.filter((format) => format !== 'zenmux/video');
}

export function getChannelTypeForApiFormat(provider: string, apiFormat: ApiFormat, configs: ProtocolConfigs): ChannelType | undefined {
  const providerConfig = configs.providerConfigs[provider];
  if (!providerConfig) return undefined;

  // Native video is a ZenMux channel type, so it is only mappable back to a
  // channel type when the provider config actually includes the ZenMux
  // channel type.
  if (apiFormat === 'zenmux/video') {
    return providerConfig.channelTypes.includes('zenmux') ? 'zenmux' : undefined;
  }

  for (const channelType of providerConfig.channelTypes) {
    if (configs.channelConfigs[channelType]?.apiFormat === apiFormat) {
      return channelType;
    }
  }

  return undefined;
}

export function getModelProtocolsForApiFormat(
  apiFormat: ApiFormat,
  models: readonly string[],
  existingProtocols: readonly ModelProtocol[] | null | undefined = []
): ModelProtocol[] {
  if (apiFormat !== 'zenmux/video') {
    return (existingProtocols ?? [])
      .map((protocol) => ({
        ...protocol,
        apiFormats: protocol.apiFormats.filter((format) => format !== 'zenmux/video'),
      }))
      .filter((protocol) => protocol.apiFormats.length > 0);
  }

  // Native video selection must not clobber per-model protocol overrides:
  // preserve any existing explicit selection for a model and only assign the
  // video format to models the user has not pinned to a protocol yet.
  const existingByModel = new Map((existingProtocols ?? []).map((protocol) => [protocol.model, protocol]));
  return models.map((model) => {
    const existing = existingByModel.get(model);
    if (existing) {
      return { ...existing };
    }
    return { model, apiFormats: [apiFormat], enabled: true };
  });
}

export function getInitialApiFormatForChannel(
  channelType: ChannelType,
  defaultApiFormat: ApiFormat,
  modelProtocols: readonly ModelProtocol[] | null | undefined
): ApiFormat {
  if (
    channelType === 'zenmux' &&
    modelProtocols?.some((protocol) => protocol.enabled !== false && protocol.apiFormats.includes('zenmux/video'))
  ) {
    return 'zenmux/video';
  }

  return defaultApiFormat;
}
