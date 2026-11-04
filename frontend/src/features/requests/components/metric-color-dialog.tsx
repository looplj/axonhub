'use client';

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Switch } from '@/components/ui/switch';
import {
  METRIC_COLOR_KEYS,
  METRIC_COLOR_META,
  METRIC_COLOR_TIER_STYLE,
  type MetricColorKey,
  type MetricColorSettings,
  type MetricColorTier,
  getMetricColorSettingsSnapshot,
  metricColorPercent,
  metricColorRange,
  replaceMetricColorCut,
  resetMetricColorSettings,
  setMetricColorSettings,
} from '../utils/metric-colors';
import { useMetricColorSettings } from '../utils/use-metric-colors';

/** Tier order per metric: green sits on the left when a smaller value is better. */
function tiersFor(key: MetricColorKey): MetricColorTier[] {
  return METRIC_COLOR_META[key].higherIsBetter
    ? ['red', 'yellow', 'neutral', 'blue', 'green']
    : ['green', 'blue', 'neutral', 'yellow', 'red'];
}

/** Draft key so an in-progress edit survives re-renders without touching the store. */
function draftKey(key: MetricColorKey, index: number): string {
  return `${key}:${index}`;
}

function roundCut(key: MetricColorKey, value: number): number {
  const { max } = metricColorRange(key, getMetricColorSettingsSnapshot()[key]);
  // Latency is the only metric where half seconds are meaningful at the low end.
  return key === 'firstTokenLatency' && max <= 40 ? Math.round(value * 2) / 2 : Math.round(value);
}

interface MetricColorDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * Per-browser palette for the request list metrics.
 *
 * Dialog content mirrors the "reorder columns" dialog: changes apply
 * immediately, there is no save/cancel pair, and the footer only offers a
 * reset.
 */
export function MetricColorDialog({ open, onOpenChange }: MetricColorDialogProps) {
  const { t } = useTranslation();
  const settings = useMetricColorSettings();
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [guideStarts, setGuideStarts] = useState<Partial<Record<MetricColorKey, number[]>>>({});
  const [dragging, setDragging] = useState<{ key: MetricColorKey; index: number } | null>(null);
  const [hoveredCut, setHoveredCut] = useState<{ key: MetricColorKey; index: number } | null>(null);
  const [hoveredSegment, setHoveredSegment] = useState<{ key: MetricColorKey; index: number } | null>(null);

  const groupRefs = useRef<Partial<Record<MetricColorKey, HTMLDivElement | null>>>({});
  const boxRefs = useRef<Partial<Record<MetricColorKey, (HTMLDivElement | null)[]>>>({});
  const bandRefs = useRef<Partial<Record<MetricColorKey, HTMLDivElement | null>>>({});

  const applyRule = useCallback((key: MetricColorKey, cuts: MetricColorSettings[MetricColorKey]['cuts']) => {
    const latest = getMetricColorSettingsSnapshot();
    setMetricColorSettings({ ...latest, [key]: { ...latest[key], cuts } });
  }, []);

  // Dashed guides start at each input's centre and end on the boundary it owns.
  const measureGuides = useCallback(() => {
    const next: Partial<Record<MetricColorKey, number[]>> = {};
    for (const key of METRIC_COLOR_KEYS) {
      const group = groupRefs.current[key];
      const boxes = boxRefs.current[key] ?? [];
      if (!group) continue;

      const host = group.getBoundingClientRect();
      if (!host.width) continue;

      next[key] = boxes.map((box) => {
        if (!box) return 0;
        const rect = box.getBoundingClientRect();
        return ((rect.left + rect.width / 2 - host.left) / host.width) * 100;
      });
    }
    setGuideStarts(next);
  }, []);

  useLayoutEffect(() => {
    if (!open) return;

    // Radix mounts the dialog content before its layout is measurable, so the
    // first pass can read zero widths and leave the guides empty. Measure now
    // and again over the next two frames to cover the opening animation.
    let firstFrame = 0;
    let secondFrame = 0;
    measureGuides();
    firstFrame = requestAnimationFrame(() => {
      measureGuides();
      secondFrame = requestAnimationFrame(measureGuides);
    });

    const observer = new ResizeObserver(measureGuides);
    for (const key of METRIC_COLOR_KEYS) {
      const group = groupRefs.current[key];
      if (group) observer.observe(group);
    }
    window.addEventListener('resize', measureGuides);

    return () => {
      cancelAnimationFrame(firstFrame);
      cancelAnimationFrame(secondFrame);
      observer.disconnect();
      window.removeEventListener('resize', measureGuides);
    };
  }, [open, measureGuides, settings]);

  // Holds the teardown of the current drag so a closed dialog cannot keep writing.
  const cleanupRef = useRef<(() => void) | null>(null);

  useEffect(() => {
    if (!open) cleanupRef.current?.();
  }, [open]);

  useEffect(() => () => cleanupRef.current?.(), []);

  const startDrag = (key: MetricColorKey, index: number, event: React.PointerEvent<HTMLDivElement>) => {
    const band = bandRefs.current[key];
    if (!band) return;
    event.preventDefault();

    const rect = band.getBoundingClientRect();
    // Capture the range once: it grows with the last cut, so re-reading it on
    // every move would let a boundary near the right edge chase its own tail
    // (150 → 165 → 182 for throughput while the pointer sits still).
    const { min, max } = metricColorRange(key, getMetricColorSettingsSnapshot()[key]);
    setDragging({ key, index });

    const move = (pointer: PointerEvent) => {
      const ratio = Math.min(1, Math.max(0, (pointer.clientX - rect.left) / rect.width));
      const latest = getMetricColorSettingsSnapshot();
      const raw = min + ratio * (max - min);
      const cuts = replaceMetricColorCut(key, latest[key], index, roundCut(key, raw));
      setMetricColorSettings({ ...latest, [key]: { ...latest[key], cuts } });
    };

    const finish = () => {
      setDragging(null);
      cleanupRef.current = null;
      window.removeEventListener('pointermove', move);
      window.removeEventListener('pointerup', finish);
      window.removeEventListener('pointercancel', finish);
    };

    window.addEventListener('pointermove', move);
    window.addEventListener('pointerup', finish);
    // Touch gestures can end as pointercancel; without this the window listeners
    // survive the gesture and keep writing thresholds.
    window.addEventListener('pointercancel', finish);
    cleanupRef.current = finish;
  };

  const commitDraft = (key: MetricColorKey, index: number) => {
    const id = draftKey(key, index);
    const raw = drafts[id];
    if (raw === undefined) return;

    const parsed = Number(raw);
    const latest = getMetricColorSettingsSnapshot();
    if (raw.trim() !== '' && Number.isFinite(parsed)) {
      applyRule(key, replaceMetricColorCut(key, latest[key], index, parsed));
    }
    setDrafts((current) => {
      const next = { ...current };
      delete next[id];

      return next;
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-h-[90vh] max-w-[560px]'>
        <DialogHeader>
          <DialogTitle>{t('requests.metricColors.title')}</DialogTitle>
          <DialogDescription>{t('requests.metricColors.description')}</DialogDescription>
        </DialogHeader>

        <div className='max-h-[64vh] overflow-y-auto pr-1'>
          {METRIC_COLOR_KEYS.map((key) => {
            const rule = settings[key];
            const meta = METRIC_COLOR_META[key];
            const tiers = tiersFor(key);
            const { max } = metricColorRange(key, rule);
            const edges = [0, ...rule.cuts, max];

            return (
              <div
                key={key}
                ref={(node) => {
                  groupRefs.current[key] = node;
                }}
                className={cn('border-border/60 border-t py-3.5 first:border-t-0', !rule.enabled && 'opacity-55')}
              >
                <div className='mb-2.5 flex items-center gap-2.5'>
                  <Switch
                    checked={rule.enabled}
                    onCheckedChange={(checked) => {
                      const latest = getMetricColorSettingsSnapshot();
                      setMetricColorSettings({ ...latest, [key]: { ...latest[key], enabled: checked } });
                    }}
                    aria-label={t(`requests.metricColors.metrics.${key}`)}
                  />
                  <span className='text-sm font-semibold'>{t(`requests.metricColors.metrics.${key}`)}</span>
                </div>

                <div className='flex justify-between gap-2'>
                  {rule.cuts.map((cut, index) => {
                    const isHighlighted =
                      (hoveredCut?.key === key && hoveredCut.index === index) ||
                      (hoveredSegment?.key === key && (hoveredSegment.index === index || hoveredSegment.index === index + 1));

                    return (
                      <div
                        key={index}
                        ref={(node) => {
                          const boxes = boxRefs.current[key] ?? [];
                          boxes[index] = node;
                          boxRefs.current[key] = boxes;
                        }}
                        className={cn(
                          'border-input flex h-8 w-24 items-center gap-1 rounded-lg border bg-transparent px-2',
                          isHighlighted && 'border-foreground ring-foreground/10 ring-2'
                        )}
                        onMouseEnter={() => setHoveredCut({ key, index })}
                        onMouseLeave={() => setHoveredCut(null)}
                      >
                        <input
                          value={drafts[draftKey(key, index)] ?? String(cut)}
                          disabled={!rule.enabled}
                          aria-label={`${t(`requests.metricColors.metrics.${key}`)} ${t('requests.metricColors.boundaryLabel', { index: index + 1 })}`}
                          onChange={(event) => setDrafts((current) => ({ ...current, [draftKey(key, index)]: event.target.value }))}
                          onBlur={() => commitDraft(key, index)}
                          onKeyDown={(event) => {
                            if (event.key === 'Enter') {
                              event.preventDefault();
                              commitDraft(key, index);
                            }
                          }}
                          inputMode='decimal'
                          className='h-full w-full bg-transparent text-right font-mono text-sm leading-none outline-none disabled:cursor-not-allowed'
                        />
                        <span className='text-muted-foreground shrink-0 text-[10px]'>{meta.unit}</span>
                      </div>
                    );
                  })}
                </div>

                <svg aria-hidden className='block h-5 w-full' viewBox='0 0 100 20' preserveAspectRatio='none'>
                  {(guideStarts[key] ?? []).map((start, index) => (
                    <line
                      key={index}
                      x1={start}
                      y1={0}
                      x2={metricColorPercent(key, rule, rule.cuts[index])}
                      y2={20}
                      className={cn('text-muted-foreground', dragging?.key === key && dragging.index === index && 'text-foreground')}
                      stroke='currentColor'
                      strokeWidth={1}
                      strokeDasharray='3 3'
                      vectorEffect='non-scaling-stroke'
                    />
                  ))}
                </svg>

                <div className='relative'>
                  <div
                    ref={(node) => {
                      bandRefs.current[key] = node;
                    }}
                    className='flex h-6 overflow-hidden rounded-md'
                  >
                    {tiers.map((tier, index) => {
                      const width = ((edges[index + 1] - edges[index]) / (max || 1)) * 100;

                      return (
                        <div
                          key={tier}
                          className={cn('shrink-0 grow-0', METRIC_COLOR_TIER_STYLE[tier].swatch)}
                          style={{ flexBasis: `${Math.max(width, 0)}%` }}
                          onMouseEnter={() => setHoveredSegment({ key, index })}
                          onMouseLeave={() => setHoveredSegment(null)}
                          title={t(`requests.metricColors.tiers.${tier}`)}
                        />
                      );
                    })}
                  </div>
                  {rule.cuts.map((cut, index) => {
                    const left = metricColorPercent(key, rule, cut);
                    const isActive = dragging?.key === key && dragging.index === index;
                    const isHighlighted = hoveredCut?.key === key && hoveredCut.index === index;

                    return (
                      <div
                        key={index}
                        role='slider'
                        aria-label={t('requests.metricColors.boundaryLabel', { index: index + 1 })}
                        aria-valuenow={cut}
                        aria-valuemin={index === 0 ? 0 : rule.cuts[index - 1]}
                        aria-valuemax={index === rule.cuts.length - 1 ? max : rule.cuts[index + 1]}
                        tabIndex={rule.enabled ? 0 : -1}
                        className={cn(
                          'absolute -top-0.5 -bottom-0.5 -ml-1.5 w-3 cursor-col-resize touch-none',
                          !rule.enabled && 'pointer-events-none'
                        )}
                        style={{ left: `${left}%` }}
                        onPointerDown={(event) => rule.enabled && startDrag(key, index, event)}
                        onKeyDown={(event) => {
                          if (!rule.enabled) return;
                          const step = event.shiftKey ? 5 : 1;
                          const delta =
                            event.key === 'ArrowRight' || event.key === 'ArrowUp'
                              ? step
                              : event.key === 'ArrowLeft' || event.key === 'ArrowDown'
                                ? -step
                                : 0;
                          if (delta === 0) return;
                          event.preventDefault();
                          applyRule(key, replaceMetricColorCut(key, rule, index, cut + delta));
                        }}
                        onMouseEnter={() => setHoveredCut({ key, index })}
                        onMouseLeave={() => setHoveredCut(null)}
                      >
                        <span
                          className={cn(
                            'absolute inset-y-0 left-1/2 w-px -translate-x-1/2 bg-zinc-500 dark:bg-zinc-400',
                            (isActive || isHighlighted) && 'bg-foreground w-0.5'
                          )}
                        />
                      </div>
                    );
                  })}
                  {dragging?.key === key && (
                    <div
                      className='bg-foreground text-background pointer-events-none absolute -top-6 -translate-x-1/2 rounded-full px-2 py-0.5 font-mono text-[11px]'
                      style={{ left: `${metricColorPercent(key, rule, rule.cuts[dragging.index])}%` }}
                    >
                      {rule.cuts[dragging.index]} {meta.unit}
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>

        <DialogFooter className='flex-row items-center gap-3 sm:justify-start'>
          <Button variant='outline' onClick={() => resetMetricColorSettings()}>
            {t('requests.metricColors.reset')}
          </Button>
          <span className='text-muted-foreground text-xs'>{t('requests.metricColors.localOnlyHint')}</span>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
