export interface ClaudeCodeSafeguardsConfig {
  classifierModel: string;
  policy: string;
  fallbackModels: string[];
  escalateOnDenial: boolean;
  attemptTimeoutSeconds: number;
  timeoutSeconds: number;
  maxTokens: number;
  transcriptCharBudget: number;
}

export interface ClaudeCodeSafeguardsSettings {
  config: ClaudeCodeSafeguardsConfig;
  usingServerDefaults: boolean;
  defaultPolicy: string;
}

export function parseFallbackModels(value: string): string[] {
  return value
    .split(/[\n,]/)
    .map((model) => model.trim())
    .filter(Boolean);
}

export function safeguardsValidation(config: ClaudeCodeSafeguardsConfig): 'fallbacks' | 'limits' | null {
  const models = [config.classifierModel.trim(), ...config.fallbackModels.map((model) => model.trim())];
  if (config.fallbackModels.some((model) => !model.trim()) || new Set(models).size !== models.length) return 'fallbacks';
  if (
    !Number.isFinite(config.timeoutSeconds) ||
    config.timeoutSeconds <= 0 ||
    config.timeoutSeconds > 120 ||
    !Number.isFinite(config.attemptTimeoutSeconds) ||
    config.attemptTimeoutSeconds < 0 ||
    config.attemptTimeoutSeconds > 120 ||
    !Number.isInteger(config.maxTokens) ||
    config.maxTokens < 1 ||
    config.maxTokens > 16384 ||
    !Number.isInteger(config.transcriptCharBudget) ||
    config.transcriptCharBudget < 1 ||
    config.transcriptCharBudget > 1000000
  )
    return 'limits';
  return null;
}
