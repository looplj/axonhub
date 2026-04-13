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

/**
 * Adds ^ prefix and $ suffix if not already present (accounting for common inline
 * modifier groups like (?i), (?m), (?s) that may precede the anchor).
 */
function ensureAnchored(pattern: string): string {
  const { modifier, body } = splitInlineModifier(pattern);
  const normalizedBody = body.replace(/^\^/, '').replace(/\$$/, '');
  return `${modifier}^(?:${normalizedBody})$`;
}

function splitInlineModifier(pattern: string): { modifier: string; body: string } {
  const match = pattern.match(/^\(\?([a-z]+)\)/);
  if (!match) {
    return { modifier: '', body: pattern };
  }

  if (/[ :=!<]/.test(match[1])) {
    return { modifier: '', body: pattern };
  }

  return {
    modifier: match[0],
    body: pattern.slice(match[0].length),
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
    return new RegExp(ensureAnchored(pattern)).test(model);
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
