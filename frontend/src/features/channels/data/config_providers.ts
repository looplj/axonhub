import type { ComponentType } from 'react';
import {
  OpenAI,
  Anthropic,
  Google,
  DeepSeek,
  Doubao,
  Moonshot,
  Zhipu,
  OpenRouter,
  XAI,
  Volcengine,
  SiliconCloud,
  PPIO,
  ZAI,
  LongCat,
  Minimax,
  BurnCloud,
  Vercel,
  ModelScope,
  Bailian,
  Jina,
  DeepInfra,
  Github,
  AiHubMix,
  Cerebras,
  Claude,
} from '@lobehub/icons';
import { NanoGPTIcon } from '../components/nanogpt-icon';
import { CHANNEL_CONFIGS } from './config_channels';
import { ApiFormat, ChannelType } from './schema';

export interface ProviderConfig {
  provider: string;
  icon: ComponentType<{ size?: number; className?: string }>;
  color: string;
  /** Channel types supported by this provider, ordered by API format preference */
  channelTypes: ChannelType[];
}

/**
 * Provider configurations - groups channel types by provider/vendor
 * Each provider can support multiple API formats (channel types)
 */
export const PROVIDER_CONFIGS: Record<string, ProviderConfig> = {
  openai: {
    provider: 'openai',
    icon: OpenAI,
    color: 'bg-white-100 text-white-800 border-white-200',
    channelTypes: ['openai', 'openai_responses'],
  },
  deepseek: {
    provider: 'deepseek',
    icon: DeepSeek,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    channelTypes: ['deepseek_anthropic', 'deepseek'],
  },
  gemini: {
    provider: 'gemini',
    icon: Google,
    color: 'bg-green-100 text-green-800 border-green-200',
    channelTypes: ['gemini', 'gemini_vertex', 'gemini_openai'],
  },
  anthropic: {
    provider: 'anthropic',
    icon: Anthropic,
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    channelTypes: ['anthropic', 'anthropic_aws', 'anthropic_gcp'],
  },
  moonshot: {
    provider: 'moonshot',
    icon: Moonshot,
    color: 'bg-black-100 text-black-800 border-black-200',
    channelTypes: ['moonshot_anthropic', 'moonshot'],
  },
  zhipu: {
    provider: 'zhipu',
    icon: Zhipu,
    color: 'bg-purple-100 text-purple-800 border-purple-200',
    channelTypes: ['zhipu_anthropic', 'zhipu'],
  },
  minimax: {
    provider: 'minimax',
    icon: Minimax,
    color: 'bg-red-100 text-red-800 border-red-200',
    channelTypes: ['minimax_anthropic', 'minimax'],
  },
  claudecode: {
    provider: 'claudecode',
    icon: Claude,
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    channelTypes: ['claudecode'],
  },
  codex: {
    provider: 'codex',
    icon: OpenAI,
    color: 'bg-[#32746D] text-white border-[#32746D]',
    channelTypes: ['codex'],
  },
  antigravity: {
    provider: 'antigravity',
    icon: Google,
    color: 'bg-green-100 text-green-800 border-green-200',
    channelTypes: ['antigravity'],
  },
  zai: {
    provider: 'zai',
    icon: ZAI,
    color: 'bg-cyan-100 text-cyan-800 border-cyan-200',
    channelTypes: ['zai', 'zai_anthropic'],
  },
  doubao: {
    provider: 'doubao',
    icon: Doubao,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    channelTypes: ['doubao_anthropic', 'doubao'],
  },
  longcat: {
    provider: 'longcat',
    icon: LongCat,
    color: 'bg-green-100 text-green-800 border-green-200',
    channelTypes: ['longcat', 'longcat_anthropic'],
  },
  jina: {
    provider: 'jina',
    icon: Jina,
    color: 'bg-purple-100 text-purple-800 border-purple-200',
    channelTypes: ['jina'],
  },
  xai: {
    provider: 'xai',
    icon: XAI,
    color: 'bg-black-100 text-black-800 border-black-200',
    channelTypes: ['xai'],
  },
  burncloud: {
    provider: 'burncloud',
    icon: BurnCloud,
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    channelTypes: ['burncloud'],
  },
  github: {
    provider: 'github',
    icon: Github,
    color: 'bg-gray-100 text-gray-800 border-gray-200',
    channelTypes: ['github'],
  },
  ppio: {
    provider: 'ppio',
    icon: PPIO,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    channelTypes: ['ppio'],
  },
  siliconflow: {
    provider: 'siliconflow',
    icon: SiliconCloud,
    color: 'bg-purple-100 text-purple-800 border-purple-200',
    channelTypes: ['siliconflow'],
  },
  volcengine: {
    provider: 'volcengine',
    icon: Volcengine,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    channelTypes: ['volcengine'],
  },
  aihubmix: {
    provider: 'aihubmix',
    icon: AiHubMix,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    channelTypes: ['aihubmix'],
  },
  modelscope: {
    provider: 'modelscope',
    icon: ModelScope,
    color: 'bg-purple-100 text-purple-800 border-purple-200',
    channelTypes: ['modelscope'],
  },
  bailian: {
    provider: 'bailian',
    icon: Bailian,
    color: 'bg-green-100 text-green-800 border-green-200',
    channelTypes: ['bailian'],
  },
  openrouter: {
    provider: 'openrouter',
    icon: OpenRouter,
    color: 'bg-gray-100 text-gray-800 border-gray-200',
    channelTypes: ['openrouter'],
  },
  vercel: {
    provider: 'vercel',
    icon: Vercel,
    color: 'bg-black-100 text-black-800 border-black-200',
    channelTypes: ['vercel'],
  },
  deepinfra: {
    provider: 'deepinfra',
    icon: DeepInfra,
    color: 'bg-indigo-100 text-indigo-800 border-indigo-200',
    channelTypes: ['deepinfra'],
  },
  cerebras: {
    provider: 'cerebras',
    icon: Cerebras,
    color: 'bg-[#F15928] text-white border-[#F15928]',
    channelTypes: ['cerebras'],
  },
  nanogpt: {
    provider: 'nanogpt',
    icon: NanoGPTIcon,
    color: 'bg-gray-100 text-gray-800 border-gray-200',
    channelTypes: ['nanogpt'],
  },
};

/**
 * Get provider key from channel type
 */
export const getProviderFromChannelType = (channelType: ChannelType): string | undefined => {
  for (const [providerKey, config] of Object.entries(PROVIDER_CONFIGS)) {
    if (config.channelTypes.includes(channelType)) {
      return providerKey;
    }
  }
  return undefined;
};

/**
 * Get channel type for a provider with specific API format
 */
export const getChannelTypeForApiFormat = (provider: string, apiFormat: ApiFormat): ChannelType | undefined => {
  const providerConfig = PROVIDER_CONFIGS[provider];
  if (!providerConfig) return undefined;

  for (const channelType of providerConfig.channelTypes) {
    const channelConfig = CHANNEL_CONFIGS[channelType];
    if (channelConfig?.apiFormat === apiFormat) {
      return channelType;
    }
  }
  return undefined;
};

/**
 * Get available API formats for a provider
 */
export const getApiFormatsForProvider = (provider: string): ApiFormat[] => {
  const providerConfig = PROVIDER_CONFIGS[provider];
  if (!providerConfig) return [];

  const formats: ApiFormat[] = [];
  for (const channelType of providerConfig.channelTypes) {
    const channelConfig = CHANNEL_CONFIGS[channelType];
    if (channelConfig?.apiFormat && !formats.includes(channelConfig.apiFormat)) {
      formats.push(channelConfig.apiFormat);
    }
  }
  return formats;
};
