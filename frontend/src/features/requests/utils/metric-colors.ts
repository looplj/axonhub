/**
 * Metric colour thresholds for the request list.
 *
 * The colours are a per-browser display preference: they never leave the
 * machine, so the settings live in localStorage instead of the system
 * settings table (a shared table would force one palette onto every user and
 * would add upstream-conflicting GraphQL surface for a purely visual feature).
 */

export const METRIC_COLOR_STORAGE_KEY = 'axonhub:metric-colors';
export const METRIC_COLOR_STORAGE_VERSION = 1;

/** Extra headroom kept on the band so the last tier is always visible. */
export const METRIC_COLOR_RANGE_FACTOR = 1.1;

export type MetricColorKey = 'cacheHitRate' | 'firstTokenLatency' | 'throughput' | 'throughputIncludingFirstToken';

export type MetricColorTier = 'red' | 'yellow' | 'neutral' | 'blue' | 'green';

export type MetricColorCuts = [number, number, number, number];

export interface MetricColorRule {
  enabled: boolean;
  /** Four ascending boundaries splitting the range into five tiers. */
  cuts: MetricColorCuts;
}

export type MetricColorSettings = Record<MetricColorKey, MetricColorRule>;

export interface MetricColorMeta {
  /** Suffix rendered next to the inputs. */
  unit: string;
  /** Baseline right edge of the band. */
  baseMax: number;
  /** Hard semantic cap (percentages); null means "no upper bound". */
  cap: number | null;
  /** true when a bigger value is better (green sits on the right). */
  higherIsBetter: boolean;
  defaultCuts: MetricColorCuts;
  defaultEnabled: boolean;
}

export const METRIC_COLOR_KEYS: MetricColorKey[] = ['cacheHitRate', 'firstTokenLatency', 'throughput', 'throughputIncludingFirstToken'];

export const METRIC_COLOR_META: Record<MetricColorKey, MetricColorMeta> = {
  cacheHitRate: {
    unit: '%',
    baseMax: 100,
    cap: 100,
    higherIsBetter: true,
    defaultCuts: [50, 80, 90, 95],
    defaultEnabled: true,
  },
  firstTokenLatency: {
    unit: 's',
    baseMax: 30,
    cap: null,
    higherIsBetter: false,
    defaultCuts: [3, 5, 10, 20],
    defaultEnabled: true,
  },
  throughput: {
    unit: 'tok/s',
    baseMax: 150,
    cap: null,
    higherIsBetter: true,
    defaultCuts: [10, 20, 60, 100],
    defaultEnabled: true,
  },
  throughputIncludingFirstToken: {
    unit: 'tok/s',
    baseMax: 100,
    cap: null,
    higherIsBetter: true,
    defaultCuts: [10, 20, 40, 60],
    defaultEnabled: true,
  },
};

/**
 * One tier, one colour. `text` is what the request list renders and `swatch` is
 * what the dialog paints; they are declared together so the band can never
 * drift from the list — the band is a preview of the list colour, not a
 * separate palette.
 */
/**
 * Tier palette. `text` is the table-cell foreground and `swatch` the wide band
 * fill drawn in the dialog; they read the same semantic tokens but resolve to
 * different values, because small text needs weight while a full-width bar
 * needs restraint. Tokens live in index.css under `:root` and `.dark`.
 * `neutral` keeps an empty text class so callers inherit their own styling.
 */
export const METRIC_COLOR_TIER_STYLE: Record<MetricColorTier, { text: string; swatch: string }> = {
  red: { text: 'text-[color:var(--metric-tier-red-fg)]', swatch: 'bg-[var(--metric-tier-red-fill)]' },
  yellow: { text: 'text-[color:var(--metric-tier-yellow-fg)]', swatch: 'bg-[var(--metric-tier-yellow-fill)]' },
  neutral: { text: '', swatch: 'bg-[var(--metric-tier-neutral-fill)]' },
  blue: { text: 'text-[color:var(--metric-tier-blue-fg)]', swatch: 'bg-[var(--metric-tier-blue-fill)]' },
  green: { text: 'text-[color:var(--metric-tier-green-fg)]', swatch: 'bg-[var(--metric-tier-green-fill)]' },
};

/**
 * Tier to text class. `neutral` is intentionally empty so callers keep their
 * own default styling (muted or inherited) instead of forcing a colour.
 */
export const METRIC_COLOR_TEXT_CLASS: Record<MetricColorTier, string> = {
  red: METRIC_COLOR_TIER_STYLE.red.text,
  yellow: METRIC_COLOR_TIER_STYLE.yellow.text,
  neutral: METRIC_COLOR_TIER_STYLE.neutral.text,
  blue: METRIC_COLOR_TIER_STYLE.blue.text,
  green: METRIC_COLOR_TIER_STYLE.green.text,
};

const HIGHER_IS_BETTER_TIERS: MetricColorTier[] = ['red', 'yellow', 'neutral', 'blue', 'green'];
const LOWER_IS_BETTER_TIERS: MetricColorTier[] = ['green', 'blue', 'neutral', 'yellow', 'red'];

export function defaultMetricColorSettings(): MetricColorSettings {
  return {
    cacheHitRate: { enabled: METRIC_COLOR_META.cacheHitRate.defaultEnabled, cuts: [...METRIC_COLOR_META.cacheHitRate.defaultCuts] },
    firstTokenLatency: {
      enabled: METRIC_COLOR_META.firstTokenLatency.defaultEnabled,
      cuts: [...METRIC_COLOR_META.firstTokenLatency.defaultCuts],
    },
    throughput: { enabled: METRIC_COLOR_META.throughput.defaultEnabled, cuts: [...METRIC_COLOR_META.throughput.defaultCuts] },
    throughputIncludingFirstToken: {
      enabled: METRIC_COLOR_META.throughputIncludingFirstToken.defaultEnabled,
      cuts: [...METRIC_COLOR_META.throughputIncludingFirstToken.defaultCuts],
    },
  } as MetricColorSettings;
}

/**
 * Band range. Percentages are capped at 100; the other metrics stretch so the
 * largest boundary always keeps a sliver of the "beyond" tier visible, which is
 * why the last input has no upper limit of its own.
 */
export function metricColorRange(key: MetricColorKey, rule: MetricColorRule): { min: number; max: number } {
  const meta = METRIC_COLOR_META[key];
  if (meta.cap !== null) {
    return { min: 0, max: meta.cap };
  }

  // Round the derived edge so `200 * 1.1` does not surface as 220.00000000000003
  // in the band edge and any label that reads it.
  const stretched = Math.round(rule.cuts[3] * METRIC_COLOR_RANGE_FACTOR * 100) / 100;

  return { min: 0, max: Math.max(meta.baseMax, stretched) };
}

/** Where a value sits on the band, as a 0-100 percentage. */
export function metricColorPercent(key: MetricColorKey, rule: MetricColorRule, value: number): number {
  const { min, max } = metricColorRange(key, rule);
  if (max <= min) return 0;

  return ((value - min) / (max - min)) * 100;
}

/**
 * Keeps a boundary between its neighbours. Boundaries may coincide (a tier then
 * renders with zero width) but never cross, so the tier order stays stable
 * while dragging.
 */
export function clampMetricColorCut(key: MetricColorKey, rule: MetricColorRule, index: number, value: number): number {
  const meta = METRIC_COLOR_META[key];
  const lower = index === 0 ? 0 : rule.cuts[index - 1];
  const upper = index === rule.cuts.length - 1 ? (meta.cap ?? Number.POSITIVE_INFINITY) : rule.cuts[index + 1];
  if (!Number.isFinite(value)) return lower;

  return Math.min(upper, Math.max(lower, value));
}

export function replaceMetricColorCut(key: MetricColorKey, rule: MetricColorRule, index: number, value: number): MetricColorCuts {
  const next = [...rule.cuts] as MetricColorCuts;
  next[index] = clampMetricColorCut(key, rule, index, value);

  return next;
}

/** Tier index for a value, honouring whether bigger or smaller is better. */
export function metricColorTierIndex(key: MetricColorKey, rule: MetricColorRule, value: number): number {
  const { higherIsBetter } = METRIC_COLOR_META[key];
  let index = 0;

  for (let i = 0; i < rule.cuts.length; i++) {
    // "higher is better" counts the boundary itself as the better side
    // (>= 95 is green); "lower is better" counts it as the better side too
    // (<= 3s is green), hence the strict comparison.
    const reached = higherIsBetter ? value >= rule.cuts[i] : value > rule.cuts[i];
    if (!reached) break;
    index = i + 1;
  }

  return index;
}

export function metricColorTier(key: MetricColorKey, rule: MetricColorRule, value: number): MetricColorTier {
  const tiers = METRIC_COLOR_META[key].higherIsBetter ? HIGHER_IS_BETTER_TIERS : LOWER_IS_BETTER_TIERS;

  return tiers[metricColorTierIndex(key, rule, value)];
}

/**
 * Text class for a value, or an empty string when the metric is switched off so
 * the caller keeps its own default styling.
 */
export function metricColorTextClass(settings: MetricColorSettings, key: MetricColorKey, value: number | null | undefined): string {
  const rule = settings[key];
  if (!rule?.enabled || value === null || value === undefined || !Number.isFinite(value)) return '';

  return METRIC_COLOR_TEXT_CLASS[metricColorTier(key, rule, value)];
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function readRule(raw: unknown, meta: MetricColorMeta): MetricColorRule {
  const fallback: MetricColorRule = { enabled: meta.defaultEnabled, cuts: [...meta.defaultCuts] };
  if (!raw || typeof raw !== 'object') return fallback;

  const candidate = raw as { enabled?: unknown; cuts?: unknown };
  const cuts = Array.isArray(candidate.cuts) ? candidate.cuts : [];
  if (cuts.length !== 4 || !cuts.every(isFiniteNumber)) return fallback;

  // Stored cuts must stay non-decreasing; a corrupted order would make every
  // tier lookup meaningless, so fall back instead of trying to repair it.
  for (let i = 1; i < cuts.length; i++) {
    if (cuts[i] < cuts[i - 1]) return fallback;
  }

  return {
    enabled: typeof candidate.enabled === 'boolean' ? candidate.enabled : meta.defaultEnabled,
    cuts: cuts as MetricColorCuts,
  };
}

/** Parses a stored payload, falling back to defaults for anything unusable. */
export function parseMetricColorSettings(raw: string | null): MetricColorSettings {
  const defaults = defaultMetricColorSettings();
  if (!raw) return defaults;

  try {
    const parsed: unknown = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object') return defaults;

    const payload = parsed as { v?: unknown; metrics?: unknown };
    if (payload.v !== METRIC_COLOR_STORAGE_VERSION || !payload.metrics || typeof payload.metrics !== 'object') {
      return defaults;
    }

    const metrics = payload.metrics as Record<string, unknown>;
    const next = {} as MetricColorSettings;
    for (const key of METRIC_COLOR_KEYS) {
      next[key] = readRule(metrics[key], METRIC_COLOR_META[key]);
    }

    return next;
  } catch {
    return defaults;
  }
}

export function serializeMetricColorSettings(settings: MetricColorSettings): string {
  return JSON.stringify({ v: METRIC_COLOR_STORAGE_VERSION, metrics: settings });
}

export function readMetricColorSettings(): MetricColorSettings {
  if (typeof window === 'undefined') return defaultMetricColorSettings();

  try {
    return parseMetricColorSettings(window.localStorage.getItem(METRIC_COLOR_STORAGE_KEY));
  } catch {
    return defaultMetricColorSettings();
  }
}

export function writeMetricColorSettings(settings: MetricColorSettings): void {
  if (typeof window === 'undefined') return;

  try {
    window.localStorage.setItem(METRIC_COLOR_STORAGE_KEY, serializeMetricColorSettings(settings));
  } catch {
    /* quota or unavailable storage: keep the in-memory value only */
  }
}

let snapshot: MetricColorSettings | null = null;
const listeners = new Set<() => void>();
let storageBound = false;

function emit(): void {
  for (const listener of listeners) listener();
}

export function getMetricColorSettingsSnapshot(): MetricColorSettings {
  if (!snapshot) snapshot = readMetricColorSettings();

  return snapshot;
}

function bindStorageOnce(): void {
  if (storageBound || typeof window === 'undefined') return;
  storageBound = true;

  // Another tab changed the palette: refresh so the list re-renders with it.
  window.addEventListener('storage', (event) => {
    if (event.key !== METRIC_COLOR_STORAGE_KEY) return;
    snapshot = readMetricColorSettings();
    emit();
  });
}

export function subscribeMetricColors(listener: () => void): () => void {
  bindStorageOnce();
  listeners.add(listener);

  return () => {
    listeners.delete(listener);
  };
}

export function setMetricColorSettings(next: MetricColorSettings): void {
  snapshot = next;
  writeMetricColorSettings(next);
  emit();
}

export function resetMetricColorSettings(): MetricColorSettings {
  const defaults = defaultMetricColorSettings();
  setMetricColorSettings(defaults);

  return defaults;
}
