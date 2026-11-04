import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import ts from 'typescript';

// metric-colors.ts is deliberately React-free, so it can be transpiled and
// imported as-is instead of stubbing the app around it.
async function loadModule() {
  const source = readFileSync(new URL('./metric-colors.ts', import.meta.url), 'utf8');
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  });

  return import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`);
}

const m = await loadModule();

const rule = (cuts, enabled = true) => ({ enabled, cuts });

test('defaults follow the agreed boundaries', () => {
  const defaults = m.defaultMetricColorSettings();

  assert.deepEqual(defaults.cacheHitRate, { enabled: true, cuts: [50, 80, 90, 95] });
  assert.deepEqual(defaults.firstTokenLatency, { enabled: true, cuts: [3, 5, 10, 20] });
  assert.deepEqual(defaults.throughput, { enabled: true, cuts: [10, 20, 60, 100] });
  assert.deepEqual(defaults.throughputIncludingFirstToken, { enabled: true, cuts: [10, 20, 40, 60] });
});

test('cache band is fixed at 0-100 and other bands keep 10% headroom', () => {
  assert.deepEqual(m.metricColorRange('cacheHitRate', rule([50, 80, 90, 95])), { min: 0, max: 100 });
  // 100 * 1.1 = 110 is below the 150 baseline, so the baseline wins
  assert.deepEqual(m.metricColorRange('throughput', rule([10, 20, 60, 100])), { min: 0, max: 150 });
  // a boundary beyond the baseline stretches the band instead of being clipped
  assert.deepEqual(m.metricColorRange('throughput', rule([10, 20, 60, 200])), { min: 0, max: 220 });
  assert.equal(m.metricColorPercent('throughputIncludingFirstToken', rule([10, 20, 40, 60]), 50), 50);
});

test('cache hit rate treats the boundary itself as the better tier', () => {
  const r = rule([50, 80, 90, 95]);

  assert.equal(m.metricColorTier('cacheHitRate', r, 100), 'green');
  assert.equal(m.metricColorTier('cacheHitRate', r, 95), 'green');
  assert.equal(m.metricColorTier('cacheHitRate', r, 94.9), 'blue');
  assert.equal(m.metricColorTier('cacheHitRate', r, 90), 'blue');
  assert.equal(m.metricColorTier('cacheHitRate', r, 89.9), 'neutral');
  assert.equal(m.metricColorTier('cacheHitRate', r, 80), 'neutral');
  assert.equal(m.metricColorTier('cacheHitRate', r, 79.9), 'yellow');
  assert.equal(m.metricColorTier('cacheHitRate', r, 50), 'yellow');
  assert.equal(m.metricColorTier('cacheHitRate', r, 49.9), 'red');
});

test('first token latency runs the other way: shorter is greener', () => {
  const r = rule([3, 5, 10, 20]);

  assert.equal(m.metricColorTier('firstTokenLatency', r, 0.4), 'green');
  assert.equal(m.metricColorTier('firstTokenLatency', r, 3), 'green');
  assert.equal(m.metricColorTier('firstTokenLatency', r, 3.1), 'blue');
  assert.equal(m.metricColorTier('firstTokenLatency', r, 5), 'blue');
  assert.equal(m.metricColorTier('firstTokenLatency', r, 5.1), 'neutral');
  assert.equal(m.metricColorTier('firstTokenLatency', r, 10), 'neutral');
  assert.equal(m.metricColorTier('firstTokenLatency', r, 10.1), 'yellow');
  assert.equal(m.metricColorTier('firstTokenLatency', r, 20), 'yellow');
  assert.equal(m.metricColorTier('firstTokenLatency', r, 20.1), 'red');
});

test('throughput boundaries land on the expected tiers', () => {
  const r = rule([10, 20, 60, 100]);

  assert.equal(m.metricColorTier('throughput', r, 9.9), 'red');
  assert.equal(m.metricColorTier('throughput', r, 10), 'yellow');
  assert.equal(m.metricColorTier('throughput', r, 20), 'neutral');
  assert.equal(m.metricColorTier('throughput', r, 60), 'blue');
  assert.equal(m.metricColorTier('throughput', r, 100), 'green');
  assert.equal(m.metricColorTier('throughput', r, 400), 'green');
});

test('a boundary may meet its neighbour but never cross it', () => {
  const r = rule([50, 80, 90, 95]);

  assert.equal(m.clampMetricColorCut('cacheHitRate', r, 1, 40), 50, 'cannot fall below the previous boundary');
  assert.equal(m.clampMetricColorCut('cacheHitRate', r, 1, 95), 90, 'cannot pass the next boundary');
  assert.equal(m.clampMetricColorCut('cacheHitRate', r, 1, 80), 80, 'equal to the previous boundary is allowed');
  assert.equal(m.clampMetricColorCut('cacheHitRate', r, 3, 140), 100, 'percentages stop at 100');
  assert.equal(
    m.clampMetricColorCut('throughput', rule([10, 20, 60, 100]), 3, 900),
    900,
    'throughput has no hard cap, the band just stretches'
  );
  assert.equal(m.clampMetricColorCut('cacheHitRate', r, 0, 0), 0);
});

test('replacing a boundary keeps the others untouched', () => {
  const next = m.replaceMetricColorCut('throughput', rule([10, 20, 60, 100]), 2, 45);

  assert.deepEqual(next, [10, 20, 45, 100]);
});

test('colour classes are empty when a metric is off or neutral', () => {
  const settings = m.defaultMetricColorSettings();

  assert.equal(m.metricColorTextClass(settings, 'cacheHitRate', 96), 'text-green-600 dark:text-green-400');
  assert.equal(m.metricColorTextClass(settings, 'cacheHitRate', 85), '');
  assert.equal(
    m.metricColorTextClass(settings, 'throughputIncludingFirstToken', 90),
    'text-green-600 dark:text-green-400',
    'the total rate is on by default'
  );
  assert.equal(m.metricColorTextClass(settings, 'firstTokenLatency', null), '');
  assert.equal(m.metricColorTextClass(settings, 'firstTokenLatency', Number.NaN), '');
  assert.equal(
    m.metricColorTextClass({ ...settings, cacheHitRate: rule([50, 80, 90, 95], false) }, 'cacheHitRate', 99),
    '',
    'a disabled metric never colours'
  );
});

test('a stored payload round-trips', () => {
  const settings = m.defaultMetricColorSettings();
  settings.throughput = rule([5, 15, 30, 80], false);
  const parsed = m.parseMetricColorSettings(m.serializeMetricColorSettings(settings));

  assert.deepEqual(parsed, settings);
});

test('unusable payloads fall back to defaults', () => {
  const defaults = m.defaultMetricColorSettings();

  assert.deepEqual(m.parseMetricColorSettings(null), defaults);
  assert.deepEqual(m.parseMetricColorSettings('not json'), defaults);
  assert.deepEqual(m.parseMetricColorSettings(JSON.stringify({ v: 99, metrics: {} })), defaults);

  const unordered = JSON.stringify({
    v: m.METRIC_COLOR_STORAGE_VERSION,
    metrics: { cacheHitRate: { enabled: true, cuts: [80, 50, 90, 95] } },
  });
  assert.deepEqual(m.parseMetricColorSettings(unordered), defaults, 'crossed boundaries are rejected');

  const shortCuts = JSON.stringify({
    v: m.METRIC_COLOR_STORAGE_VERSION,
    metrics: { cacheHitRate: { enabled: true, cuts: [50, 80, 90] } },
  });
  assert.deepEqual(m.parseMetricColorSettings(shortCuts), defaults);

  // A partially valid payload keeps the good entries and repairs the rest.
  const partial = JSON.stringify({
    v: m.METRIC_COLOR_STORAGE_VERSION,
    metrics: { cacheHitRate: { enabled: false, cuts: [40, 70, 85, 92] } },
  });
  const repaired = m.parseMetricColorSettings(partial);
  assert.deepEqual(repaired.cacheHitRate, { enabled: false, cuts: [40, 70, 85, 92] });
  assert.deepEqual(repaired.throughput, defaults.throughput);
});