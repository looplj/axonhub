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
  deepseek: {
    baseURL: 'https://api.deepseek.com/v1',
    defaultModels: ['deepseek-chat', 'deepseek-reasoner'],
  },
  deepseek_anthropic: {
    baseURL: 'https://api.deepseek.com/anthropic',
    defaultModels: ['deepseek-chat', 'deepseek-reasoner'],
  },
  minimax: {
    baseURL: 'https://api.minimaxi.com/v1',
    defaultModels: ['MiniMax-M2'],
  },
  minimax_anthropic: {
    baseURL: 'https://api.minimaxi.com/anthropic',
    defaultModels: ['MiniMax-M2'],
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
  gemini_openai: {
    baseURL: 'https://generativelanguage.googleapis.com/v1beta/openai',
    defaultModels: ['gemini-2.5-pro', 'gemini-2.5-flash'],
  },
  doubao: {
    baseURL: 'https://ark.cn-beijing.volces.com/api/v3',
    defaultModels: ['doubao-seed-1.6', 'doubao-seed-1.6-flash'],
  },
  moonshot: {
    baseURL: 'https://api.moonshot.cn/v1',
    defaultModels: ['kimi-k2-0711-preview', 'kimi-k2-0905-preview', 'kimi-k2-turbo-preview'],
  },
  moonshot_anthropic: {
    baseURL: 'https://api.moonshot.cn/anthropic',
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
  zhipu_anthropic: {
    baseURL: 'https://open.bigmodel.cn/api/anthropic',
    defaultModels: ['glm-4.6', 'glm-4.5', 'glm-4.5-air', 'glm-4.5-x', 'glm-4.5v'],
  },
  zai_anthropic: {
    baseURL: 'https://api.z.ai/api/anthropic',
    defaultModels: ['glm-4.6', 'glm-4.5', 'glm-4.5-air', 'glm-4.5-x', 'glm-4.5v'],
  },
  vercel: {
    baseURL: 'https://ai-gateway.vercel.sh/v1',
    defaultModels: [
      'deepseek/deepseek-v3.2-exp-thinking',
      'deepseek/deepseek-v3.2-exp',
      'moonshotai/kimi-k2-thinking',
      'moonshotai/kimi-k2',
      'zai/glm-4.6',
      'anthropic/claude-sonnet-4.5',
      'google/gemini-2.5-flash',
      'google/gemini-2.5-pro',
      'openai/gpt-4o',
      'openai/gpt-4o-mini',
      'openai/gpt-5',
    ],
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
  longcat: {
    baseURL: 'https://api.longcat.chat/openai/v1',
    defaultModels: ['LongCat-Flash-Chat', 'LongCat-Flash-Thinking'],
  },
  longcat_anthropic: {
    baseURL: 'https://api.longcat.chat/anthropic',
    defaultModels: ['LongCat-Flash-Chat', 'LongCat-Flash-Thinking'],
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
  aihubmix: {
    baseURL: 'https://aihubmix.com/v1',
    defaultModels: [
      'DeepSeek-V3.2-Exp',
      'DeepSeek-V3.2-Exp-Think',
      'DeepSeek-V3.1-Terminus',
      // Google
      'gemini-2.5-flash',
      'gemini-2.5-pro',
      // Anthropic
      'claude-sonnet-4-5',
      // OpenAI
      'gpt-4o',
      'gpt-5',
      // Moonshot
      'Kimi-K2-0905',
      // Zai/GLM
      'glm-4.6',
      'glm-4.5',
    ],
  },
  burncloud: {
    baseURL: 'https://ai.burncloud.com/v1',
    defaultModels: [
      // Claude Models
      'claude-3-5-haiku-20241022',
      'claude-3-5-sonnet-20240620',
      'claude-3-5-sonnet-20241022',
      'claude-3-7-sonnet-20250219',
      'claude-3-7-sonnet-20250219-thinking',
      'claude-3-opus-20240229',
      'claude-3-sonnet-20240229',
      'claude-haiku-4-5-20251001',
      'claude-haiku-4-5-20251001-thinking',
      'claude-opus-4-1-20250805',
      'claude-opus-4-1-20250805-thinking',
      'claude-opus-4-20250514',
      'claude-opus-4-20250514-thinking',
      'claude-sonnet-4-20250514',
      'claude-sonnet-4-20250514-thinking',
      'claude-sonnet-4-5',
      'claude-sonnet-4-5-20250929',
      'claude-sonnet-4-5-20250929-thinking',
      // DeepSeek Models
      'deepseek-chat',
      'deepseek-r1',
      'deepseek-r1-250120',
      'deepseek-r1-250528',
      'deepseek-r1-search',
      'deepseek-reasoner',
      'deepseek-v3',
      'deepseek-v3-0324',
      'deepseek-v3-20250324',
      'deepseek-v3-250324',
      'deepseek-v3-search',
      'deepseek-v3.1-250821',
      'deepseek-v3.1-think-250821',
      // Gemini Models
      'gemini-2.0-flash',
      'gemini-2.0-flash-001',
      'gemini-2.0-flash-exp',
      'gemini-2.0-flash-lite',
      'gemini-2.0-flash-lite-001',
      'gemini-2.5-flash',
      'gemini-2.5-flash-image-preview',
      'gemini-2.5-flash-nothinking',
      'gemini-2.5-flash-thinking',
      'gemini-2.5-pro',
      'gemini-2.5-pro-nothinking',
      'gemini-2.5-pro-preview-03-25',
      'gemini-2.5-pro-preview-05-06',
      'gemini-2.5-pro-preview-06-05',
      'gemini-2.5-pro-thinking',
      // GPT-3.5 Models
      'gpt-3.5-turbo',
      'gpt-3.5-turbo-0125',
      'gpt-3.5-turbo-0613',
      'gpt-3.5-turbo-1106',
      'gpt-3.5-turbo-instruct',
      // GPT-4 Models
      'gpt-4',
      'gpt-4-0125-preview',
      'gpt-4-1106-preview',
      'gpt-4-turbo',
      'gpt-4-turbo-2024-04-09',
      'gpt-4.1',
      'gpt-4.1-2025-04-14',
      'gpt-4.1-mini',
      'gpt-4.1-mini-2025-04-14',
      'gpt-4.1-nano',
      'gpt-4.1-nano-2025-04-14',
      'gpt-4o',
      'gpt-4o-2024-05-13',
      'gpt-4o-2024-08-06',
      'gpt-4o-2024-11-20',
      'gpt-4o-mini',
      'gpt-4o-mini-2024-07-18',
      'gpt-4o-mini-transcribe',
      'gpt-4o-transcribe',
      // GPT-5 Models
      'gpt-5',
      'gpt-5-2025-08-07',
      'gpt-5-2025-08-07-high',
      'gpt-5-2025-08-07-low',
      'gpt-5-2025-08-07-medium',
      'gpt-5-2025-08-07-minimal',
      'gpt-5-chat',
      'gpt-5-chat-latest',
      'gpt-5-codex',
      'gpt-5-high',
      'gpt-5-low',
      'gpt-5-medium',
      'gpt-5-mini',
      'gpt-5-mini-2025-08-07',
      'gpt-5-mini-2025-08-07-high',
      'gpt-5-mini-2025-08-07-low',
      'gpt-5-mini-2025-08-07-medium',
      'gpt-5-mini-2025-08-07-minimal',
      'gpt-5-mini-high',
      'gpt-5-mini-low',
      'gpt-5-mini-medium',
      'gpt-5-mini-minimal',
      'gpt-5-minimal',
      'gpt-5-nano',
      'gpt-5-nano-2025-08-07',
      'gpt-5-nano-2025-08-07-high',
      'gpt-5-nano-2025-08-07-low',
      'gpt-5-nano-2025-08-07-medium',
      'gpt-5-nano-2025-08-07-minimal',
      'gpt-5-nano-high',
      'gpt-5-nano-low',
      'gpt-5-nano-medium',
      'gpt-5-nano-minimal',
      'gpt-5-pro',
      // GPT Audio/Image Models
      'gpt-audio',
      'gpt-audio-2025-08-28',
      'gpt-image-1',
      'gpt-image-1-mini',
      'gpt-realtime-2025-08-28',
      'gpt-realtime-mini-2025-10-06',
      // Grok Models
      'grok-2',
      'grok-2-vision',
      'grok-3',
      'grok-3-beta',
      'grok-3-deepsearch',
      'grok-3-fast-beta',
      'grok-3-mini',
      'grok-3-mini-beta',
      'grok-3-mini-beta-high',
      'grok-3-mini-beta-low',
      'grok-3-mini-beta-medium',
      'grok-3-mini-fast-beta',
      'grok-3-mini-fast-beta-high',
      'grok-3-mini-fast-beta-low',
      'grok-3-mini-fast-beta-medium',
      'grok-3-nx',
      'grok-3-reasoning',
      'grok-3-search',
      'grok-4',
      'grok-4-0709',
      'grok-4-0709-search',
      'grok-4-fast-non-reasoning',
      'grok-4-fast-reasoning',
      'grok-4-fast-reasoning-search',
      'grok-code-fast-1',
      // Jina Models
      'jina-colbert-v2',
      'jina-embeddings-v4',
      'jina-reranker-m0',
      'jina-reranker-v2-base-multilingual',
      // O1/O3/O4 Models
      'o1',
      'o1-2024-12-17',
      'o1-2024-12-17-high',
      'o1-2024-12-17-low',
      'o1-2024-12-17-medium',
      'o1-high',
      'o1-low',
      'o1-medium',
      'o1-mini',
      'o1-mini-2024-09-12',
      'o1-preview',
      'o1-preview-2024-09-12',
      'o3',
      'o3-2025-04-16-medium',
      'o3-deep-research',
      'o3-deep-research-2025-06-26',
      'o3-medium',
      'o3-mini',
      'o3-mini-2025-01-31',
      'o3-mini-2025-01-31-high',
      'o3-mini-2025-01-31-low',
      'o3-mini-2025-01-31-medium',
      'o3-mini-high',
      'o3-mini-low',
      'o3-mini-medium',
      'o4-mini',
      'o4-mini-2025-04-16',
      'o4-mini-2025-04-16-high',
      'o4-mini-2025-04-16-low',
      'o4-mini-2025-04-16-medium',
      'o4-mini-high',
      'o4-mini-low',
      'o4-mini-medium',
      // Qwen Models
      'qwen3-235b-a22b',
      'qwen3-235b-a22b-instruct-2507',
      'qwen3-coder-480b-a35b-instruct',
      'qwen3-coder-plus',
      // Sora Models
      'sora-2',
      'sora-2-pro',
      // Embedding Models
      'text-embedding-3-large',
      'text-embedding-3-small',
      'text-embedding-ada-002',
      // TTS Models
      'tts-1',
      'tts-1-hd',
      // Whisper Models
      'whisper-1',
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
