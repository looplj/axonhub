import type { Model } from '@/features/models/data/schema';

/**
 * Pi's thinking levels. Pi talks to AxonHub over the OpenAI-compatible
 * `/v1` endpoint, and AxonHub carries these same labels as a
 * protocol-independent reasoning effort (see llm/reasoning.go), so every level
 * keeps its name and only `off` needs the AxonHub spelling `none`.
 */
export const PI_THINKING_LEVELS = ['off', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'] as const;

export type PiThinkingLevel = (typeof PI_THINKING_LEVELS)[number];

const PI_LEVEL_TO_EFFORT: Record<PiThinkingLevel, string> = {
  off: 'none',
  minimal: 'minimal',
  low: 'low',
  medium: 'medium',
  high: 'high',
  xhigh: 'xhigh',
  max: 'max',
};

/** Pi only understands text and image inputs, while AxonHub models may declare more. */
const PI_MODEL_INPUTS = ['text', 'image'] as const;

export const PI_PROVIDER_KEY = 'axonhub';

export interface PiThinkingLevelMap {
  off?: string | null;
  minimal?: string | null;
  low?: string | null;
  medium?: string | null;
  high?: string | null;
  xhigh?: string | null;
  max?: string | null;
}

export interface PiProviderModel {
  id: string;
  name?: string;
  reasoning?: boolean;
  thinkingLevelMap?: PiThinkingLevelMap;
  input?: ('text' | 'image')[];
  contextWindow?: number;
  maxTokens?: number;
  cost?: {
    input: number;
    output: number;
    cacheRead: number;
    cacheWrite: number;
  };
}

export interface PiProviderConfig {
  api: string;
  apiKey: string;
  baseUrl: string;
  models: PiProviderModel[];
}

export interface PiModelsConfig {
  providers: Record<string, PiProviderConfig>;
}

export interface BuildPiModelsConfigParams {
  origin: string;
  apiKey: string;
  models: Model[];
}

/**
 * Derives Pi's thinking level map from the reasoning effort levels a model accepts.
 *
 * Pi reads the map as: an explicit `null` hides that level, and `xhigh`/`max`
 * stay hidden unless they are declared. AxonHub effort names double as the values
 * Pi sends, so a level is either passed through or hidden. Unknown levels stay
 * unknown: no map at all, which leaves Pi's own defaults in place.
 */
export function buildPiThinkingLevelMap(
  reasoningSupported: boolean,
  reasoningEfforts: string[] | undefined
): PiThinkingLevelMap | undefined {
  if (!reasoningSupported || !reasoningEfforts?.length) return undefined;

  const map: PiThinkingLevelMap = {};
  for (const level of PI_THINKING_LEVELS) {
    map[level] = reasoningEfforts.includes(PI_LEVEL_TO_EFFORT[level]) ? PI_LEVEL_TO_EFFORT[level] : null;
  }

  return map;
}

function buildPiProviderModel(model: Model): PiProviderModel | undefined {
  if (model.type !== 'chat') return undefined;

  const card = model.modelCard;
  const reasoning = !!card?.reasoning?.supported;
  const thinkingLevelMap = buildPiThinkingLevelMap(reasoning, card?.reasoningEfforts ?? undefined);

  const declaredInputs = card?.modalities?.input ?? [];
  const input = PI_MODEL_INPUTS.filter((modality) => modality === 'text' || declaredInputs.includes(modality));

  const cost = card?.cost;
  const hasCost = !!cost && (cost.input > 0 || cost.output > 0 || !!cost.cacheRead || !!cost.cacheWrite);

  const entry: PiProviderModel = {
    id: model.modelID,
    reasoning,
    input: [...input],
  };

  if (model.name) entry.name = model.name;
  if (thinkingLevelMap) entry.thinkingLevelMap = thinkingLevelMap;
  if (card?.limit && card.limit.context > 0) entry.contextWindow = card.limit.context;
  if (card?.limit && card.limit.output > 0) entry.maxTokens = card.limit.output;
  if (hasCost && cost) {
    entry.cost = {
      input: cost.input,
      output: cost.output,
      cacheRead: cost.cacheRead ?? 0,
      cacheWrite: cost.cacheWrite ?? 0,
    };
  }

  return entry;
}

export function buildPiModelsConfig({ origin, apiKey, models }: BuildPiModelsConfigParams): PiModelsConfig {
  const entries = models
    .map((model) => buildPiProviderModel(model))
    .filter((entry): entry is PiProviderModel => !!entry)
    .sort((a, b) => a.id.localeCompare(b.id));

  return {
    providers: {
      [PI_PROVIDER_KEY]: {
        api: 'openai-completions',
        apiKey,
        baseUrl: `${origin}/v1`,
        models: entries,
      },
    },
  };
}

function sortKeysRecursively(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(sortKeysRecursively);
  if (value && typeof value === 'object') {
    return Object.fromEntries(
      Object.entries(value as Record<string, unknown>)
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([key, item]) => [key, sortKeysRecursively(item)])
    );
  }
  return value;
}

/** Serializes the config with sorted keys, matching the layout Pi writes to models.json. */
export function serializePiModelsConfig(config: PiModelsConfig): string {
  return `${JSON.stringify(sortKeysRecursively(config), null, 2)}\n`;
}
