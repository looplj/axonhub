// Utility functions for merging channel override configurations
// Mirrors backend merge logic in internal/server/biz/channel_merge.go
import type { ChannelSettings, OverrideOperation } from '../data/schema';

/**
 * Normalizes empty or whitespace-only parameter strings to "[]".
 * This ensures consistent representation across the system.
 */
export function normalizeOverrideParameters(params: string): string {
  if (!params || params.trim() === '') {
    return '[]';
  }
  return params;
}

/**
 * Merges override header operations with template header operations.
 * - For `set` ops: match by `path` (case-insensitive), template overrides existing
 * - Other ops (delete, rename, copy): always appended from template
 * - Existing ops not matched by template are preserved
 */
export function mergeOverrideHeaders(existing: OverrideOperation[], template: OverrideOperation[]): OverrideOperation[] {
  const result = [...existing];

  for (const templateOp of template) {
    if (templateOp.op === 'set' && templateOp.path) {
      const index = result.findIndex(
        (op) => op.op === 'set' && op.path?.toLowerCase() === templateOp.path?.toLowerCase()
      );
      if (index >= 0) {
        result[index] = templateOp;
      } else {
        result.push(templateOp);
      }
    } else {
      result.push(templateOp);
    }
  }

  return result;
}

export function mergeChannelSettingsForUpdate(
  existing: ChannelSettings | null | undefined,
  patch: Partial<ChannelSettings>
): ChannelSettings {
  const hasOwn = (key: keyof ChannelSettings) => Object.prototype.hasOwnProperty.call(patch, key);
  const pick = <K extends keyof ChannelSettings>(key: K, fallback: ChannelSettings[K]): ChannelSettings[K] => {
    if (!hasOwn(key)) {
      return fallback;
    }
    const value = patch[key];
    return value === undefined ? fallback : (value as ChannelSettings[K]);
  };

  return {
    extraModelPrefix: pick('extraModelPrefix', existing?.extraModelPrefix ?? ''),
    modelMappings: pick('modelMappings', existing?.modelMappings ?? []),
    autoTrimedModelPrefixes: pick('autoTrimedModelPrefixes', existing?.autoTrimedModelPrefixes ?? []),
    hideOriginalModels: pick('hideOriginalModels', existing?.hideOriginalModels ?? false),
    hideMappedModels: pick('hideMappedModels', existing?.hideMappedModels ?? false),
    bodyOverrideOperations: pick('bodyOverrideOperations', existing?.bodyOverrideOperations ?? []),
    headerOverrideOperations: pick('headerOverrideOperations', existing?.headerOverrideOperations ?? []),
    proxy: pick('proxy', existing?.proxy ?? null),
    transformOptions: pick('transformOptions', existing?.transformOptions ?? undefined),
  };
}

/**
 * Deep merges two JSON object strings.
 * - Both inputs must be JSON objects
 * - Nested objects are merged recursively
 * - Scalars and arrays are overwritten by template
 */
export function mergeOverrideParameters(existing: string, template: string): string {
  try {
    const existingObj = parseJSONObject(existing);
    const templateObj = parseJSONObject(template);

    const merged = deepMergeObjects(existingObj, templateObj);

    // Use compact format to match backend
    return JSON.stringify(merged);
  } catch (error) {
    // If parsing fails, return template
    return template;
  }
}

function parseJSONObject(input: string): Record<string, any> {
  const trimmed = input.trim();
  if (!trimmed) {
    return {};
  }

  const parsed = JSON.parse(trimmed);

  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    throw new Error('Input must be a JSON object');
  }

  return parsed;
}

function deepMergeObjects(base: Record<string, any>, override: Record<string, any>): Record<string, any> {
  const result: Record<string, any> = { ...base };

  for (const [key, overrideVal] of Object.entries(override)) {
    const baseVal = result[key];

    // If both values are objects (and not arrays), merge recursively
    if (
      baseVal &&
      typeof baseVal === 'object' &&
      !Array.isArray(baseVal) &&
      overrideVal &&
      typeof overrideVal === 'object' &&
      !Array.isArray(overrideVal)
    ) {
      result[key] = deepMergeObjects(baseVal, overrideVal);
    } else {
      // Otherwise, override with template value
      result[key] = overrideVal;
    }
  }

  return result;
}
