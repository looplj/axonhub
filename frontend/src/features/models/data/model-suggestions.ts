import type { ProviderModel } from './providers.schema';

// Suggestions are separate from saved model IDs and the raw metadata catalog.
// Match text families only: media models and custom aliases remain available.
export function isSuggestedModel(modelID: string): boolean {
  const id = modelID.toLowerCase();
  const name = id.slice(id.lastIndexOf('/') + 1);
  const isMiniMaxText = /^(minimax[-_](m\d|text|vl-\d)|abab\d)/.test(name);
  if (!isMiniMaxText) return true;
  return ['minimax-m3', 'minimax-m3.1-flash', 'minimax-m3.1-flash:free', 'minimax-m3.1-flash-free'].includes(name);
}

// Direct API metadata must never be applied to a gateway or subscription plan.
// https://platform.minimax.io/docs/guides/models-intro
const flashModel: ProviderModel = {
  id: 'MiniMax-M3.1-Flash',
  name: 'MiniMax-M3.1-Flash',
  type: 'chat',
  limit: { context: 1000000, output: 524288 },
  cost: { input: 0.15, output: 0.6, cache_read: 0.015 },
  reasoning: { supported: true, default: true },
  reasoning_options: [{ type: 'effort', values: ['low', 'medium', 'high', 'xhigh', 'max'] }],
  modalities: { input: ['text', 'image', 'video'], output: ['text'] },
  tool_call: true,
};

export function withMiniMaxFlash(models: ProviderModel[], developer: string): ProviderModel[] {
  if (developer !== 'minimax') return models;
  const existingFlash = models.find((model) => model.id === flashModel.id);
  return [existingFlash ?? flashModel, ...models.filter((model) => model.id !== flashModel.id)];
}
