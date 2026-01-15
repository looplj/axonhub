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
} from '@lobehub/icons';
import { BURNCLOUD_DEFAULT_MODELS } from './burncloud-models';
import { ApiFormat, ChannelType } from './schema';

export const OPENAI_CHAT_COMPLETIONS: ApiFormat = 'openai/chat_completions';
export const OPENAI_RESPONSES: ApiFormat = 'openai/responses';
export const ANTHROPIC_MESSAGES: ApiFormat = 'anthropic/messages';
export const GEMINI_CONTENTS: ApiFormat = 'gemini/contents';

/**
 * Channel configuration interface
 */
export interface ChannelConfig {
  channelType: ChannelType;

  /** Default base URL for the channel type */
  baseURL: string;

  /** Default models available for quick selection */
  defaultModels: string[];

  /** API protocol format used when calling this channel */
  apiFormat: ApiFormat;

  /** Badge color classes for the channel type */
  color: string;

  /** Icon component for the channel type */
  icon: ComponentType<{ size?: number; className?: string }>;
}

/**
 * Unified channel configurations
 * Contains default base URLs and models for each channel type
 */
export const CHANNEL_CONFIGS: Record<ChannelType, ChannelConfig> = {
  openai: {
    channelType: 'openai',
    baseURL: 'https://api.openai.com/v1',
    defaultModels: ['gpt-4o', 'gpt-4o-mini', 'gpt-5', 'gpt-5.1'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-white-100 text-white-800 border-white-200',
    icon: OpenAI,
  },
  openai_responses: {
    channelType: 'openai_responses',
    baseURL: 'https://api.openai.com/v1',
    defaultModels: ['gpt-4o', 'gpt-4o-mini', 'gpt-5', 'gpt-5.1'],
    apiFormat: OPENAI_RESPONSES,
    color: 'bg-green-100 text-green-800 border-green-200',
    icon: OpenAI,
  },
  codex: {
    channelType: 'codex',
    baseURL: 'https://api.openai.com/v1',
    defaultModels: ['gpt-5.2', 'gpt-5.2-codex'],
    apiFormat: OPENAI_RESPONSES,
    color: 'bg-[#32746D] text-white border-[#32746D]',
    icon: OpenAI,
  },
  deepseek: {
    channelType: 'deepseek',
    baseURL: 'https://api.deepseek.com/v1',
    defaultModels: ['deepseek-chat', 'deepseek-reasoner'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: DeepSeek,
  },
  deepseek_anthropic: {
    channelType: 'deepseek_anthropic',
    baseURL: 'https://api.deepseek.com/anthropic',
    defaultModels: ['deepseek-chat', 'deepseek-reasoner'],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: DeepSeek,
  },
  deepinfra: {
    channelType: 'deepinfra',
    baseURL: 'https://api.deepinfra.com/v1/openai',
    defaultModels: ['deepseek-ai/DeepSeek-V3.2', 'moonshotai/Kimi-K2-Thinking'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-indigo-100 text-indigo-800 border-indigo-200',
    icon: DeepInfra,
  },
  anthropic: {
    channelType: 'anthropic',
    baseURL: 'https://api.anthropic.com',
    defaultModels: ['claude-opus-4-5', 'claude-sonnet-4-5'],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    icon: Anthropic,
  },
  gemini_openai: {
    channelType: 'gemini_openai',
    baseURL: 'https://generativelanguage.googleapis.com/v1beta/openai',
    defaultModels: ['gemini-2.5-pro', 'gemini-2.5-flash'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-green-100 text-green-800 border-green-200',
    icon: Google,
  },
  gemini: {
    channelType: 'gemini',
    baseURL: 'https://generativelanguage.googleapis.com/v1beta',
    defaultModels: ['gemini-2.5-pro', 'gemini-2.5-flash'],
    apiFormat: GEMINI_CONTENTS,
    color: 'bg-green-100 text-green-800 border-green-200',
    icon: Google,
  },
  gemini_vertex: {
    channelType: 'gemini_vertex',
    baseURL: 'https://aiplatform.googleapis.com/v1',
    defaultModels: ['gemini-2.5-pro', 'gemini-2.5-flash'],
    apiFormat: GEMINI_CONTENTS,
    color: 'bg-green-100 text-green-800 border-green-200',
    icon: Google,
  },
  minimax: {
    channelType: 'minimax',
    baseURL: 'https://api.minimaxi.com/v1',
    defaultModels: ['MiniMax-M2'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-red-100 text-red-800 border-red-200',
    icon: Minimax,
  },
  minimax_anthropic: {
    channelType: 'minimax_anthropic',
    baseURL: 'https://api.minimaxi.com/anthropic',
    defaultModels: ['MiniMax-M2'],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-red-100 text-red-800 border-red-200',
    icon: Minimax,
  },
  moonshot: {
    channelType: 'moonshot',
    baseURL: 'https://api.moonshot.cn/v1',
    defaultModels: ['kimi-k2-thinking', 'kimi-k2-0905-preview', 'kimi-k2-turbo-preview'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-black-100 text-black-800 border-black-200',
    icon: Moonshot,
  },
  moonshot_anthropic: {
    channelType: 'moonshot_anthropic',
    baseURL: 'https://api.moonshot.cn/anthropic',
    defaultModels: ['kimi-k2-thinking', 'kimi-k2-0905-preview', 'kimi-k2-turbo-preview'],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-black-100 text-black-800 border-black-200',
    icon: Moonshot,
  },
  zhipu: {
    channelType: 'zhipu',
    baseURL: 'https://open.bigmodel.cn/api/paas/v4',
    defaultModels: ['glm-4.7', 'glm-4.6', 'glm-4.5-air'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-purple-100 text-purple-800 border-purple-200',
    icon: Zhipu,
  },
  zai: {
    channelType: 'zai',
    baseURL: 'https://api.z.ai/api/paas/v4',
    defaultModels: ['glm-4.7', 'glm-4.6', 'glm-4.5-air'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-cyan-100 text-cyan-800 border-cyan-200',
    icon: ZAI,
  },
  zhipu_anthropic: {
    channelType: 'zhipu_anthropic',
    baseURL: 'https://open.bigmodel.cn/api/anthropic',
    defaultModels: ['glm-4.7', 'glm-4.6', 'glm-4.5-air'],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-purple-100 text-purple-800 border-purple-200',
    icon: Zhipu,
  },
  zai_anthropic: {
    channelType: 'zai_anthropic',
    baseURL: 'https://api.z.ai/api/anthropic',
    defaultModels: ['glm-4.7', 'glm-4.6', 'glm-4.5-air'],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-cyan-100 text-cyan-800 border-cyan-200',
    icon: ZAI,
  },

  doubao: {
    channelType: 'doubao',
    baseURL: 'https://ark.cn-beijing.volces.com/api/v3',
    defaultModels: ['doubao-seed-1.6', 'doubao-seed-1.6-flash'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: Doubao,
  },
  doubao_anthropic: {
    channelType: 'doubao_anthropic',
    baseURL: 'https://ark.cn-beijing.volces.com/api/compatible',
    defaultModels: ['doubao-seed-code-preview-251028'],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: Doubao,
  },

  vercel: {
    channelType: 'vercel',
    baseURL: 'https://ai-gateway.vercel.sh/v1',
    defaultModels: [
      'deepseek/deepseek-v3.2-exp-thinking',
      'deepseek/deepseek-v3.2-exp',
      'moonshotai/kimi-k2-thinking',
      'moonshotai/kimi-k2',
    ],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-black-100 text-black-800 border-black-200',
    icon: Vercel,
  },
  openrouter: {
    channelType: 'openrouter',
    baseURL: 'https://openrouter.ai/api/v1',
    defaultModels: [
      // Moonshot
      'moonshotai/kimi-k2:free',
      'moonshotai/kimi-k2-0905',

      // Zai
      'z-ai/glm-4.7',

      // Anthropic
      'anthropic/claude-opus-4',
      'anthropic/claude-sonnet-4',
    ],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-gray-100 text-gray-800 border-gray-200',
    icon: OpenRouter,
  },
  xai: {
    channelType: 'xai',
    baseURL: 'https://api.x.ai/v1',
    defaultModels: ['grok-4', 'grok-3', 'grok-3-mini', 'grok-code-fast', 'grok-4-fast-reasoning', 'grok-4-fast-non-reasoning'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-black-100 text-black-800 border-black-200',
    icon: XAI,
  },
  longcat: {
    channelType: 'longcat',
    baseURL: 'https://api.longcat.chat/openai/v1',
    defaultModels: ['LongCat-Flash-Chat', 'LongCat-Flash-Thinking'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-green-100 text-green-800 border-green-200',
    icon: LongCat,
  },
  longcat_anthropic: {
    channelType: 'longcat_anthropic',
    baseURL: 'https://api.longcat.chat/anthropic',
    defaultModels: ['LongCat-Flash-Chat', 'LongCat-Flash-Thinking'],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-green-100 text-green-800 border-green-200',
    icon: LongCat,
  },
  ppio: {
    channelType: 'ppio',
    baseURL: 'https://api.ppinfra.com/openai/v1',
    defaultModels: [
      // DeepSeek
      'deepseek/deepseek-v3.2-exp',
      'deepseek/deepseek-v3.1',
      'deepseek/deepseek-r1-0528',

      // Qwen
      'qwen/qwen3-vl-235b-a22b-thinking',
      'qwen/qwen3-coder-480b-a35b-instruct',

      // Zai
      'zai-org/glm-4.6',
      'zai-org/glm-4.5',
      'zai-org/glm-4.5-air',

      // Moonshot
      'moonshotai/kimi-k2-0905',
    ],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: PPIO,
  },
  siliconflow: {
    channelType: 'siliconflow',
    baseURL: 'https://api.siliconflow.cn/v1',
    defaultModels: [
      // Zai
      'zai-org/GLM-4.6',
      'zai-org/GLM-4.5',
      'zai-org/GLM-4.5-air',

      // Qwen
      'Qwen/Qwen3-Coder-480B-A35B-Instruct',
      'Qwen/Qwen3-Coder-30B-A3B-Instruct',
      'Qwen/Qwen3-30B-A3B-Thinking-2507',
      'Qwen/Qwen3-235B-A22B-Instruct-2507',
    ],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-purple-100 text-purple-800 border-purple-200',
    icon: SiliconCloud,
  },
  volcengine: {
    channelType: 'volcengine',
    baseURL: 'https://ark.cn-beijing.volces.com/api/v3',
    defaultModels: [
      // DeepSeek
      'deepseek-r1-250528',
      'deepseek-v3-1-terminus',
      'deepseek-v3-250324',

      // Doubao
      'doubao-seed-1.6',
      'doubao-seed-1.6-flash',
      'doubao-seed-1.6-thinking',

      // Moonshot
      'kimi-k2-250905',
    ],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: Volcengine,
  },
  // Fake types for testing (not available for creation)
  anthropic_fake: {
    channelType: 'anthropic_fake',
    baseURL: 'https://api.anthropic.com/v1',
    defaultModels: [
      'claude-opus-4-1',
      'claude-opus-4-0',
      'claude-sonnet-4-0',
      'claude-sonnet-4-5',
      'claude-3-7-sonnet-latest',
      'claude-3-5-haiku-latest',
    ],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    icon: Anthropic,
  },
  openai_fake: {
    channelType: 'openai_fake',
    baseURL: 'https://api.openai.com/v1',
    defaultModels: ['gpt-3.5-turbo', 'gpt-4.5', 'gpt-4.1', 'gpt-4-turbo', 'gpt-4o', 'gpt-4o-mini', 'gpt-5'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-green-100 text-green-800 border-green-200',
    icon: OpenAI,
  },
  aihubmix: {
    channelType: 'aihubmix',
    baseURL: 'https://aihubmix.com/v1',
    defaultModels: [
      'DeepSeek-V3.2-Exp',
      'DeepSeek-V3.2-Exp-Think',
      // Google
      'gemini-3-flash',
      'gemini-3-pro',
      // Anthropic
      'claude-sonnet-4-5',
      // OpenAI
      'gpt-4o',
      // Moonshot
      'Kimi-K2-0905',
      // Zai/GLM
      'glm-4.7',
    ],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: OpenAI,
  },
  burncloud: {
    channelType: 'burncloud',
    baseURL: 'https://ai.burncloud.com/v1',
    defaultModels: BURNCLOUD_DEFAULT_MODELS,
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    icon: BurnCloud,
  },
  modelscope: {
    channelType: 'modelscope',
    baseURL: 'https://api-inference.modelscope.cn/v1',
    defaultModels: ['qwen-plus', 'qwen-turbo', 'qwen-max', 'qwen2.5-72b-instruct'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-purple-100 text-purple-800 border-purple-200',
    icon: ModelScope,
  },
  bailian: {
    channelType: 'bailian',
    baseURL: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    defaultModels: ['qwen-max-plus', 'qwen-turbo', 'qwen-max'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-blue-100 text-blue-800 border-blue-200',
    icon: Bailian,
  },
  anthropic_aws: {
    channelType: 'anthropic_aws',
    baseURL: 'https://bedrock-runtime.us-east-1.amazonaws.com',
    defaultModels: [
      'anthropic.claude-opus-4-1-20250805-v1:0',
      'anthropic.claude-opus-4-20250514-v1:0',
      'anthropic.claude-sonnet-4-20250514-v1:0',
      'anthropic.claude-3-7-sonnet-20250219-v1:0',
      'anthropic.claude-3-5-haiku-20241022-v1:0',
    ],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    icon: Anthropic,
  },
  anthropic_gcp: {
    channelType: 'anthropic_gcp',
    baseURL: 'https://us-east5-aiplatform.googleapis.com',
    defaultModels: [
      'claude-opus-4-1@20250805',
      'claude-opus-4@20250514',
      'claude-sonnet-4@20250514',
      'claude-3-7-sonnet@20250219',
      'claude-3-5-haiku@20241022',
    ],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-orange-100 text-orange-800 border-orange-200',
    icon: Anthropic,
  },
  jina: {
    channelType: 'jina',
    baseURL: 'https://api.jina.ai/v1',
    defaultModels: ['jina-embeddings-v3', 'jina-reranker-v3'],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-purple-100 text-purple-800 border-purple-200',
    icon: Jina,
  },
  github: {
    channelType: 'github',
    baseURL: 'https://models.github.ai/inference',
    defaultModels: [
      'openai/gpt-4.1',
      'openai/gpt-4o',
      'openai/gpt-4o-mini',
      'openai/o3',
      'openai/o4-mini',
      'anthropic/claude-sonnet-4',
      'anthropic/claude-3.5-sonnet',
      'meta/llama-4-scout-17b-16e-instruct',
      'meta/llama-4-maverick-17b-128e-instruct',
      'deepseek/DeepSeek-V3-0324',
      'mistral-ai/mistral-large-2411',
    ],
    apiFormat: OPENAI_CHAT_COMPLETIONS,
    color: 'bg-gray-100 text-gray-800 border-gray-200',
    icon: Github,
  },
  claudecode: {
    channelType: 'claudecode',
    baseURL: 'https://api.anthropic.com/v1',
    defaultModels: ['claude-haiku-4-5', 'claude-sonnet-4-5', 'claude-opus-4-5'],
    apiFormat: ANTHROPIC_MESSAGES,
    color: 'bg-purple-100 text-purple-800 border-purple-200',
    icon: Anthropic,
  },
};

/**
 * Get default base URL for a channel type
 */
export const getDefaultBaseURL = (channelType: ChannelType): string => {
  return CHANNEL_CONFIGS[channelType]?.baseURL || '';
};

/**
 * Get default models for a channel type
 */
export const getDefaultModels = (channelType: ChannelType): string[] => {
  return CHANNEL_CONFIGS[channelType]?.defaultModels || [];
};

/**
 * Provider type for grouping channel types
 */
export type Provider =
  | 'openai'
  | 'anthropic'
  | 'deepseek'
  | 'deepinfra'
  | 'gemini'
  | 'moonshot'
  | 'zhipu'
  | 'zai'
  | 'doubao'
  | 'minimax'
  | 'longcat'
  | 'xai'
  | 'openrouter'
  | 'vercel'
  | 'ppio'
  | 'siliconflow'
  | 'volcengine'
  | 'aihubmix'
  | 'burncloud'
  | 'modelscope'
  | 'bailian'
  | 'jina'
  | 'github'
  | 'codex';

/**
 * Map channel type to provider
 */
export const CHANNEL_TYPE_TO_PROVIDER: Record<ChannelType, Provider> = {
  openai: 'openai',
  openai_responses: 'openai',
  openai_fake: 'openai',
  anthropic: 'anthropic',
  anthropic_aws: 'anthropic',
  anthropic_gcp: 'anthropic',
  anthropic_fake: 'anthropic',
  deepseek: 'deepseek',
  deepseek_anthropic: 'deepseek',
  deepinfra: 'deepinfra',
  gemini: 'gemini',
  gemini_openai: 'gemini',
  gemini_vertex: 'gemini',
  moonshot: 'moonshot',
  moonshot_anthropic: 'moonshot',
  zhipu: 'zhipu',
  zhipu_anthropic: 'zhipu',
  zai: 'zai',
  zai_anthropic: 'zai',
  doubao: 'doubao',
  doubao_anthropic: 'doubao',
  minimax: 'minimax',
  minimax_anthropic: 'minimax',
  longcat: 'longcat',
  longcat_anthropic: 'longcat',
  xai: 'xai',
  openrouter: 'openrouter',
  vercel: 'vercel',
  ppio: 'ppio',
  siliconflow: 'siliconflow',
  volcengine: 'volcengine',
  aihubmix: 'aihubmix',
  burncloud: 'burncloud',
  modelscope: 'modelscope',
  bailian: 'bailian',
  jina: 'jina',
  github: 'github',
  codex: 'codex',
  claudecode: 'anthropic',
};

/**
 * Get provider for a channel type
 */
export const getProvider = (channelType: ChannelType): Provider => {
  return CHANNEL_TYPE_TO_PROVIDER[channelType];
};
