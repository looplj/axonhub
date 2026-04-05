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
  // Detect leading ^ possibly preceded by inline modifier groups, e.g. (?i)^
  const hasStartAnchor = pattern.startsWith('^') || /^\(\?[a-z]+\)\^/.test(pattern);
  const hasEndAnchor = pattern.endsWith('$');

  if (!hasStartAnchor) pattern = '^' + pattern;
  if (!hasEndAnchor) pattern = pattern + '$';

  return pattern;
}

/**
 * Returns true if `model` matches `pattern` using the same rules as the backend.
 */
export function matchesModelPattern(model: string, pattern: string): boolean {
  if (!pattern) return true;

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
  return models.filter((model) => matchesModelPattern(model, pattern));
}
