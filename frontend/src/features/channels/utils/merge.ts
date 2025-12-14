// Utility functions for merging channel override configurations
// Mirrors backend merge logic in internal/server/biz/channel_merge.go

import type { HeaderEntry } from '../data/schema'

const CLEAR_HEADER_DIRECTIVE = '__AXONHUB_CLEAR__'

/**
 * Normalizes empty or whitespace-only parameter strings to "{}".
 * This ensures consistent representation across the system.
 */
export function normalizeOverrideParameters(params: string): string {
  if (!params || params.trim() === '') {
    return '{}'
  }
  return params
}

/**
 * Merges override headers with template headers.
 * - Template entries override existing ones with the same key (case-insensitive)
 * - Template entries with value "__AXONHUB_CLEAR__" remove the header
 * - Existing headers not mentioned in template are preserved
 */
export function mergeOverrideHeaders(existing: HeaderEntry[], template: HeaderEntry[]): HeaderEntry[] {
  const result = [...existing]

  for (const templateHeader of template) {
    // Find existing header with same key (case-insensitive)
    const index = result.findIndex((h) => h.key.toLowerCase() === templateHeader.key.toLowerCase())

    if (templateHeader.value === CLEAR_HEADER_DIRECTIVE) {
      // Remove header if it exists
      if (index >= 0) {
        result.splice(index, 1)
      }
      continue
    }

    if (index >= 0) {
      // Override existing header
      result[index] = templateHeader
    } else {
      // Add new header
      result.push(templateHeader)
    }
  }

  return result
}

/**
 * Deep merges two JSON object strings.
 * - Both inputs must be JSON objects
 * - Nested objects are merged recursively
 * - Scalars and arrays are overwritten by template
 */
export function mergeOverrideParameters(existing: string, template: string): string {
  try {
    const existingObj = parseJSONObject(existing)
    const templateObj = parseJSONObject(template)

    const merged = deepMergeObjects(existingObj, templateObj)

    // Use compact format to match backend
    return JSON.stringify(merged)
  } catch (error) {
    // If parsing fails, return template
    return template
  }
}

function parseJSONObject(input: string): Record<string, any> {
  const trimmed = input.trim()
  if (!trimmed) {
    return {}
  }

  const parsed = JSON.parse(trimmed)

  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    throw new Error('Input must be a JSON object')
  }

  return parsed
}

function deepMergeObjects(base: Record<string, any>, override: Record<string, any>): Record<string, any> {
  const result: Record<string, any> = { ...base }

  for (const [key, overrideVal] of Object.entries(override)) {
    const baseVal = result[key]

    // If both values are objects (and not arrays), merge recursively
    if (
      baseVal &&
      typeof baseVal === 'object' &&
      !Array.isArray(baseVal) &&
      overrideVal &&
      typeof overrideVal === 'object' &&
      !Array.isArray(overrideVal)
    ) {
      result[key] = deepMergeObjects(baseVal, overrideVal)
    } else {
      // Otherwise, override with template value
      result[key] = overrideVal
    }
  }

  return result
}
