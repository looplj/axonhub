import { ChannelType } from './schema'

/**
 * Channel configuration interface
 */
export interface ChannelConfig {
  /** Default base URL for the channel type */
  baseURL: string
  /** Default models available for quick selection */
  defaultModels: string[]
}

/**
 * Unified channel configurations
 * Contains default base URLs and models for each channel type
 */
export const CHANNEL_CONFIGS: Record<ChannelType, ChannelConfig> = {
  openai: {
    baseURL: 'https://api.openai.com/v1',
    defaultModels: ['gpt-3.5-turbo', 'gpt-4.5', 'gpt-4.1', 'gpt-4-turbo', 'gpt-4o', 'gpt-4o-mini', 'gpt-5'],
  },
  anthropic: {
    baseURL: 'https://api.anthropic.com/v1',
    defaultModels: [
      'claude-opus-4-1',
      'claude-opus-4-0',
      'claude-sonnet-4-0',
      'claude-sonnet-4-1',
      'claude-sonnet-4-5',
      'claude-3-7-sonnet-latest',
      'claude-3-5-haiku-latest',
    ],
  },
  anthropic_aws: {
    baseURL: 'https://bedrock-runtime.us-east-1.amazonaws.com',
    defaultModels: [
      'anthropic.claude-opus-4-1-20250805-v1:0',
      'anthropic.claude-opus-4-20250514-v1:0',
      'anthropic.claude-sonnet-4-20250514-v1:0',
      'anthropic.claude-3-7-sonnet-20250219-v1:0',
      'anthropic.claude-3-5-haiku-20241022-v1:0',
    ],
  },
  anthropic_gcp: {
    baseURL: 'https://us-east5-aiplatform.googleapis.com',
    defaultModels: [
      'claude-opus-4-1@20250805',
      'claude-opus-4@20250514',
      'claude-sonnet-4@20250514',
      'claude-3-7-sonnet@20250219',
      'claude-3-5-haiku@20241022',
    ],
  },
  gemini_openai: {
    baseURL: 'https://generativelanguage.googleapis.com/v1beta/openai',
    defaultModels: ['gemini-2.5-pro', 'gemini-2.5-flash'],
  },
  deepseek: {
    baseURL: 'https://api.deepseek.com/v1',
    defaultModels: ['deepseek-chat', 'deepseek-reasoner'],
  },
  doubao: {
    baseURL: 'https://ark.cn-beijing.volces.com/api/v3',
    defaultModels: ['doubao-seed-1.6', 'doubao-seed-1.6-flash'],
  },
  moonshot: {
    baseURL: 'https://api.moonshot.cn/v1',
    defaultModels: ['kimi-k2-0711-preview', 'kimi-k2-0905-preview', 'kimi-k2-turbo-preview'],
  },
  zhipu: {
    baseURL: 'https://open.bigmodel.cn/api/paas/v4',
    defaultModels: ['glm-4.6', 'glm-4.5', 'glm-4.5-air', 'glm-4.5-x', 'glm-4.5v'],
  },
  zai: {
    baseURL: 'https://api.z.ai/api/paas/v4',
    defaultModels: ['glm-4.6', 'glm-4.5', 'glm-4.5-air', 'glm-4.5-x', 'glm-4.5v'],
  },
  deepseek_anthropic: {
    baseURL: 'https://api.deepseek.com/anthropic',
    defaultModels: ['deepseek-chat', 'deepseek-reasoner'],
  },
  moonshot_anthropic: {
    baseURL: 'https://api.moonshot.cn/anthropic',
    defaultModels: ['kimi-k2-0711-preview', 'kimi-k2-0905-preview', 'kimi-k2-turbo-preview'],
  },
  zhipu_anthropic: {
    baseURL: 'https://open.bigmodel.cn/api/anthropic',
    defaultModels: ['glm-4.6', 'glm-4.5', 'glm-4.5-air', 'glm-4.5-x', 'glm-4.5v'],
  },
  zai_anthropic: {
    baseURL: 'https://api.z.ai/api/anthropic',
    defaultModels: ['glm-4.6', 'glm-4.5', 'glm-4.5-air', 'glm-4.5-x', 'glm-4.5v'],
  },
  openrouter: {
    baseURL: 'https://openrouter.ai/api/v1',
    defaultModels: [
      // DeepSeek
      'deepseek/deepseek-chat-v3.1:free',
      'deepseek/deepseek-chat-v3.1',
      'deepseek/deepseek-r1-0528:free',
      'deepseek/deepseek-r1-0528',
      'deepseek/deepseek-r1:free',
      'deepseek/deepseek-r1',
      'deepseek/deepseek-chat-v3-0324:free',
      'deepseek/deepseek-chat-v3-0324',

      // Moonshot
      'moonshotai/kimi-k2:free',
      'moonshotai/kimi-k2-0905',

      // Zai
      'z-ai/glm-4.6',
      'z-ai/glm-4.5',
      'z-ai/glm-4.5-air',
      'z-ai/glm-4.5-air:free',

      // Google
      'google/gemini-2.5-flash-lite',
      'google/gemini-2.5-flash',
      'google/gemini-2.5-pro',

      // Anthropic
      'anthropic/claude-opus-4',
      'anthropic/claude-sonnet-4',
      'anthropic/claude-3.7-sonnet',

      // XAI
      'x-ai/grok-4-fast:free',
      'x-ai/grok-4-fast',
      'x-ai/grok-code-fast-1',
    ],
  },
  xai: {
    baseURL: 'https://api.x.ai/v1',
    defaultModels: [
      'grok-4',
      'grok-3',
      'grok-3-mini',
      'grok-code-fast',
      'grok-4-fast-reasoning',
      'grok-4-fast-non-reasoning',
    ],
  },
  ppio: {
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
  },
  siliconflow: {
    baseURL: 'https://api.siliconflow.cn/v1',
    defaultModels: [
      // DeepSeek
      'deepseek-ai/DeepSeek-V3.1',
      // Zai
      'zai-org/GLM-4.6',
      'zai-org/GLM-4.5',
      'zai-org/GLM-4.5-air',

      // Qwen
      'Qwen/Qwen3-Coder-480B-A35B-Instruct',
      'Qwen/Qwen3-Coder-30B-A3B-Instruct',
      'Qwen/Qwen3-30B-A3B-Thinking-2507',
      'Qwen/Qwen3-235B-A22B-Instruct-2507',
      'Qwen/Qwen3-235B-A22B',
    ],
  },
  volcengine: {
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
  },
  // Fake types for testing (not available for creation)
  anthropic_fake: {
    baseURL: 'https://api.anthropic.com/v1',
    defaultModels: [
      'claude-opus-4-1',
      'claude-opus-4-0',
      'claude-sonnet-4-0',
      'claude-sonnet-4-5',
      'claude-3-7-sonnet-latest',
      'claude-3-5-haiku-latest',
    ],
  },
  openai_fake: {
    baseURL: 'https://api.openai.com/v1',
    defaultModels: ['gpt-3.5-turbo', 'gpt-4.5', 'gpt-4.1', 'gpt-4-turbo', 'gpt-4o', 'gpt-4o-mini', 'gpt-5'],
  },
}

/**
 * Get default base URL for a channel type
 */
export const getDefaultBaseURL = (channelType: ChannelType): string => {
  return CHANNEL_CONFIGS[channelType]?.baseURL || ''
}

/**
 * Get default models for a channel type
 */
export const getDefaultModels = (channelType: ChannelType): string[] => {
  return CHANNEL_CONFIGS[channelType]?.defaultModels || []
}
