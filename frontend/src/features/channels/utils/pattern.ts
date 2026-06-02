/**
 * Mirrors the backend xregexp matching logic in internal/pkg/xregexp/match.go.
 *
 * Rules:
 * 1. If the pattern contains no regex special chars, do an exact string comparison.
 * 2. Otherwise, wrap the pattern with ^ / $ anchors (unless already present),
 *    then apply it as a regex — matching the full model name.
 */

// Characters that indicate a regex pattern (must stay in sync with backend containsRegexChars).
const REGEX_SPECIAL_CHARS_RE = /[*?+[\]{}()^$.|\\]/;

function containsRegexChars(pattern: string): boolean {
  return REGEX_SPECIAL_CHARS_RE.test(pattern);
}

// The channel UI documents only (?i) for case-insensitive model filters.
// Leave other regexp2 inline modifiers to backend-only/API usage so preview
// validation does not imply broader UI support than intended for model IDs.
const SUPPORTED_INLINE_MODIFIER_FLAGS = new Set(['i']);

/**
 * Compiles a pattern after translating backend-style leading inline modifiers
 * like (?i) into JavaScript RegExp flags.
 */
function compilePattern(pattern: string): RegExp {
  const { flags, body } = splitInlineModifier(pattern);
  const normalizedBody = body.replace(/^\^/, '').replace(/\$$/, '');
  return new RegExp(`^(?:${normalizedBody})$`, flags);
}

function splitInlineModifier(pattern: string): { flags: string; body: string } {
  if (!pattern.startsWith('(?')) {
    return { flags: '', body: pattern };
  }

  const end = pattern.indexOf(')');
  if (end <= 2) {
    return { flags: '', body: pattern };
  }

  const modifier = pattern.slice(2, end);
  const hasUnsupportedModifier = [...modifier].some((flag) => !SUPPORTED_INLINE_MODIFIER_FLAGS.has(flag));
  if (!/^[a-z]+$/.test(modifier) || hasUnsupportedModifier) {
    return { flags: '', body: pattern };
  }

  const flags = [...new Set(modifier)].join('');
  return {
    flags,
    body: pattern.slice(end + 1),
  };
}

/**
 * Returns true if `model` matches `pattern` using the same rules as the backend.
 */
export function matchesModelPattern(model: string, pattern: string): boolean {
  if (!pattern) return true;
  if (pattern === '*') return true;

  if (!containsRegexChars(pattern)) {
    return model === pattern;
  }

  try {
    return compilePattern(pattern).test(model);
  } catch {
    return false;
  }
}

export function isValidModelPattern(pattern: string): boolean {
  if (!pattern) return true;
  if (pattern === '*') return true;
  if (!containsRegexChars(pattern)) return true;

  try {
    compilePattern(pattern);
    return true;
  } catch {
    return false;
  }
}

/**
 * Filters `models` by `pattern` using the same rules as the backend Filter() function.
 * Returns an empty array when pattern is empty (mirrors backend behaviour).
 */
export function filterModelsByPattern(models: string[], pattern: string): string[] {
  if (!pattern) return [];
  if (pattern === '*') return models;
  return models.filter((model) => matchesModelPattern(model, pattern));
}
