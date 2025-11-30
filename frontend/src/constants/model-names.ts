/**
 * Model ID to display name mapping
 * Priority: Use alias if available, otherwise use model ID
 */
export const MODEL_NAMES: Record<string, string> = {
  // OpenAI models
  'gpt-4o': 'GPT-4o',
  'gpt-4o-mini': 'GPT-4o Mini',
  'gpt-4-turbo': 'GPT-4 Turbo',
  'gpt-4': 'GPT-4',
  'gpt-3.5-turbo': 'GPT-3.5 Turbo',
  'o1-preview': 'o1 Preview',
  'o1-mini': 'o1 Mini',

  // Anthropic models
  'claude-3-5-sonnet-20241022': 'Claude 3.5 Sonnet',
  'claude-3-5-sonnet-latest': 'Claude 3.5 Sonnet',
  'claude-3-opus-20240229': 'Claude 3 Opus',
  'claude-3-sonnet-20240229': 'Claude 3 Sonnet',
  'claude-3-haiku-20240307': 'Claude 3 Haiku',
  'claude-2.1': 'Claude 2.1',
  'claude-2': 'Claude 2',
  'claude-instant-1.2': 'Claude Instant',

  // DeepSeek models
  'deepseek-chat': 'DeepSeek Chat',
  'deepseek-reasoner': 'DeepSeek Reasoner',
  'deepseek-coder': 'DeepSeek Coder',

  // Gemini models
  'gemini-1.5-pro': 'Gemini 1.5 Pro',
  'gemini-1.5-flash': 'Gemini 1.5 Flash',
  'gemini-pro': 'Gemini Pro',

  // Moonshot models
  'moonshot-v1-8k': 'Moonshot v1 8K',
  'moonshot-v1-32k': 'Moonshot v1 32K',
  'moonshot-v1-128k': 'Moonshot v1 128K',

  // Doubao models
  'doubao-lite-4k': 'Doubao Lite 4K',
  'doubao-pro-4k': 'Doubao Pro 4K',
  'doubao-pro-32k': 'Doubao Pro 32K',

  // Zhipu models
  'glm-4': 'GLM-4',
  'glm-4-plus': 'GLM-4 Plus',
  'glm-4-air': 'GLM-4 Air',
  'glm-4-air-plus': 'GLM-4 Air Plus',
  'glm-4-flash': 'GLM-4 Flash',
  'glm-4v': 'GLM-4V',

  // Minimax models
  'abab6.5s-chat': 'ABAB 6.5s Chat',
  'abab6.5g-chat': 'ABAB 6.5g Chat',
  'abab6.5-chat': 'ABAB 6.5 Chat',
  'abab5.5-chat': 'ABAB 5.5 Chat',

  // xAI models
  'grok-beta': 'Grok Beta',
  'grok-vision-beta': 'Grok Vision Beta',

  // Longcat models
  'longcat-v1': 'Longcat v1',

  // SiliconFlow models
  'Qwen/Qwen2.5-72B-Instruct': 'Qwen2.5 72B Instruct',
  'Qwen/Qwen2.5-32B-Instruct': 'Qwen2.5 32B Instruct',
  'Qwen/Qwen2.5-7B-Instruct': 'Qwen2.5 7B Instruct',
  'meta-llama/Meta-Llama-3.1-70B-Instruct': 'Llama 3.1 70B Instruct',
  'meta-llama/Meta-Llama-3.1-8B-Instruct': 'Llama 3.1 8B Instruct',
  'deepseek-chat-v2': 'DeepSeek Chat v2',
  'deepseek-reasoner-v2': 'DeepSeek Reasoner v2',
}

/**
 * Get display name for a model ID
 * Priority: Use alias if available in MODEL_NAMES, otherwise return the original model ID
 */
export function getModelDisplayName(modelId: string): string {
  return MODEL_NAMES[modelId] || modelId
}
