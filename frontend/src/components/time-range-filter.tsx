import { useState, useCallback, useEffect, useMemo } from 'react';
import { IconCalendar, IconX } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Calendar } from '@/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import type { TimeRangeValue } from '@/stores/dashboardStore';

/** Date to 'YYYY-MM-DD', read from local fields rather than toISOString so a
 * timezone offset cannot shift the date by a day. */
function formatDate(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

/** 'YYYY-MM-DD' to local midnight; Calendar's selected needs a Date. */
function parseDate(dateStr: string): Date {
  const [y, m, d] = dateStr.split('-').map(Number);
  return new Date(y, m - 1, d);
}

type PresetKey = 'today' | 'last7Days' | 'last30Days' | 'thisMonth' | 'thisYear' | 'allTime';

interface Preset {
  key: PresetKey;
  range: TimeRangeValue;
}

/** Quick presets derived from today; earliestDate is the first recorded date on the
 * backend and feeds the allTime preset. */
function buildPresets(earliestDate?: string | null, todayString = formatDate(new Date())): Preset[] {
  const now = parseDate(todayString);
  const today = formatDate(now);

  const shift = (days: number) => {
    const start = new Date(now);
    start.setDate(now.getDate() - days + 1);
    start.setHours(0, 0, 0, 0);
    return { startTime: formatDate(start), endTime: today };
  };

  const monthStart = new Date(now.getFullYear(), now.getMonth(), 1);
  const yearStart = new Date(now.getFullYear(), 0, 1);

  return [
    { key: 'today', range: shift(1) },
    { key: 'last7Days', range: shift(7) },
    { key: 'last30Days', range: shift(30) },
    { key: 'thisMonth', range: { startTime: formatDate(monthStart), endTime: today } },
    { key: 'thisYear', range: { startTime: formatDate(yearStart), endTime: today } },
    // "All time" resolves to the earliest recorded date so the backend never
    // falls back to its own 30-day default for trend queries.
    { key: 'allTime', range: { startTime: earliestDate || null, endTime: today } },
  ];
}

interface DateRangePickerProps {
  startDate: string | null;
  endDate: string | null;
  onStartChange: (date: Date | null) => void;
  onEndChange: (date: Date | null) => void;
}

/** Start/end date picker: two popover calendars that validate ordering against each other. */
function DateRangePicker({ startDate, endDate, onStartChange, onEndChange }: DateRangePickerProps) {
  const { t } = useTranslation();
  const [startOpen, setStartOpen] = useState(false);
  const [endOpen, setEndOpen] = useState(false);

  const handleStartDateSelect = useCallback(
    (date: Date | undefined) => {
      if (date && endDate && date > parseDate(endDate)) {
        toast.error(t('timeRange.startDateAfterEndError'));
        return;
      }
      onStartChange(date || null);
      setStartOpen(false);
    },
    [endDate, onStartChange, t]
  );

  const handleEndDateSelect = useCallback(
    (date: Date | undefined) => {
      if (date && startDate && date < parseDate(startDate)) {
        toast.error(t('timeRange.endDateBeforeStartError'));
        return;
      }
      onEndChange(date || null);
      setEndOpen(false);
    },
    [startDate, onEndChange, t]
  );

  return (
    <div className='flex items-center gap-2'>
      <Popover open={startOpen} onOpenChange={setStartOpen}>
        <PopoverTrigger asChild>
          <Button
            variant='outline'
            className={cn('h-8 w-[130px] justify-start text-left text-xs font-normal', !startDate && 'text-muted-foreground')}
          >
            <IconCalendar className='mr-1 h-3 w-3' />
            {startDate || t('timeRange.startDate')}
          </Button>
        </PopoverTrigger>
        <PopoverContent className='w-auto p-0' align='start'>
          <Calendar
            mode='single'
            selected={startDate ? parseDate(startDate) : undefined}
            onSelect={handleStartDateSelect}
            disabled={{ after: new Date() }}
            initialFocus
          />
        </PopoverContent>
      </Popover>

      <span className='text-muted-foreground'>~</span>

      <Popover open={endOpen} onOpenChange={setEndOpen}>
        <PopoverTrigger asChild>
          <Button
            variant='outline'
            className={cn('h-8 w-[130px] justify-start text-left text-xs font-normal', !endDate && 'text-muted-foreground')}
          >
            <IconCalendar className='mr-1 h-3 w-3' />
            {endDate || t('timeRange.endDate')}
          </Button>
        </PopoverTrigger>
        <PopoverContent className='w-auto p-0' align='start'>
          <Calendar
            mode='single'
            selected={endDate ? parseDate(endDate) : undefined}
            onSelect={handleEndDateSelect}
            disabled={{ after: new Date() }}
            initialFocus
          />
        </PopoverContent>
      </Popover>
    </div>
  );
}

/** Single-button date range used by the compact variant, so the slim filter row stays
 * on one line while the pickers themselves stay reachable. */
function CompactDateRangePicker({
  startDate,
  endDate,
  onStartChange,
  onEndChange,
}: DateRangePickerProps) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  return (
    <Popover open={open} onOpenChange={setOpen} modal={false}>
      <PopoverTrigger asChild>
        <Button variant='ghost' size='sm' className='text-muted-foreground hover:text-foreground h-7 gap-1 px-2 text-xs'>
          <IconCalendar className='h-3 w-3' />
          <span className='tabular-nums'>
            {startDate || t('timeRange.startDate')} – {endDate || t('timeRange.endDate')}
          </span>
        </Button>
      </PopoverTrigger>
      <PopoverContent className='w-auto p-2' align='end'>
        <DateRangePicker
          startDate={startDate}
          endDate={endDate}
          onStartChange={onStartChange}
          onEndChange={onEndChange}
        />
      </PopoverContent>
    </Popover>
  );
}

interface TimeRangeFilterProps {
  value: TimeRangeValue;
  onChange: (range: TimeRangeValue) => void;
  earliestDate?: string | null;
  variant?: 'bar' | 'compact';
  /** The page's own default selection. When given, reset is offered whenever the value
   * differs from it — not only for custom dates — because a preset otherwise leaves no
   * way back to a default that no preset emits. */
  defaultValue?: TimeRangeValue;
}

/** 'YYYY-MM-DD' to local midnight; used to count the days a custom range spans. */
function localMidnight(dateStr: string): Date {
  const [y, m, d] = dateStr.split('-').map(Number);
  return new Date(y, m - 1, d);
}

/** Active range as text, so the page always states which window is in effect:
 * a preset reads as "name · start – end", a custom range as "start – end · N days",
 * and a relative window as just its name, since it has no boundaries to print. */
function useRangeSummary(value: TimeRangeValue, presets: Preset[]): string {
  const { t } = useTranslation();

  return useMemo(() => {
    if (!value.startTime) return value.timeWindow ? t(`timeRange.${value.timeWindow}`) : '';

    const matched = presets.find((p) => p.range.startTime === value.startTime && p.range.endTime === value.endTime);
    const span = `${value.startTime} – ${value.endTime || t('timeRange.today')}`;

    if (matched) return `${t(`timeRange.${matched.key}`)} · ${span}`;

    // Floored to local midnight, not left as the current instant: after local noon the
    // elapsed hours round the span of a single day up to two.
    const end = value.endTime ? localMidnight(value.endTime) : localMidnight(new Date());
    const days = Math.round((end.getTime() - localMidnight(value.startTime).getTime()) / 86_400_000) + 1;
    return `${span} · ${t('timeRange.days', { count: days })}`;
  }, [presets, t, value.endTime, value.startTime, value.timeWindow]);
}

/** Unified time range filter: preset buttons plus a custom start/end pair, replacing
 * the per-page TimePeriodSelector. */
export function TimeRangeFilter({ value, onChange, earliestDate, variant = 'bar', defaultValue }: TimeRangeFilterProps) {
  const { t } = useTranslation();
  const [today, setToday] = useState(() => formatDate(new Date()));
  useEffect(() => {
    const timer = window.setInterval(() => {
      const nextToday = formatDate(new Date());
      setToday((currentToday) => (currentToday === nextToday ? currentToday : nextToday));
    }, 60_000);
    return () => window.clearInterval(timer);
  }, []);
  const presets = useMemo(() => buildPresets(earliestDate, today), [earliestDate, today]);

  const activeKey = useMemo(() => {
    const match = presets.find((p) => p.range.startTime === value.startTime && p.range.endTime === value.endTime);
    return match?.key ?? null;
  }, [presets, value.startTime, value.endTime]);

  const isDefaultValue =
    value.startTime === (defaultValue?.startTime ?? null) &&
    value.endTime === (defaultValue?.endTime ?? null) &&
    value.timeWindow === defaultValue?.timeWindow;
  const isCustom = defaultValue
    ? !isDefaultValue
    : value.startTime !== null || value.endTime !== null;
  const summary = useRangeSummary(value, presets);

  if (variant === 'compact') {
    return (
      <div className='flex flex-wrap items-center justify-end gap-2'>
        <div className='flex flex-wrap items-center gap-1'>
          {presets.map((preset) => (
            <Button
              key={preset.key}
              variant={activeKey === preset.key ? 'default' : 'ghost'}
              size='sm'
              className='text-muted-foreground h-7 px-2 text-xs'
              // Until earliestDate is ready, allTime can only emit startTime: null, and
              // the backend then falls back to its own 30-day default, which does not
              // match what the preset promises.
              disabled={preset.key === 'allTime' && !earliestDate}
              onClick={() => onChange(preset.range)}
            >
              {t(`timeRange.${preset.key}`)}
            </Button>
          ))}
        </div>

        <CompactDateRangePicker
          startDate={value.startTime}
          endDate={value.endTime}
          onStartChange={(date) =>
            // An end date with no start date is not a range the queries can express: every
            // consumer keys off startTime, so it would fall back to allTime and silently
            // discard the end date. Clearing the start clears the end with it.
            onChange({ startTime: date ? formatDate(date) : null, endTime: date ? value.endTime : null })
          }
          onEndChange={(date) => {
            if (!value.startTime) return;
            onChange({ startTime: value.startTime, endTime: date ? formatDate(date) : null });
          }}
        />

        {isCustom && (
          <Button variant='ghost' size='sm' className='text-muted-foreground h-8 text-xs' onClick={() => onChange({ startTime: null, endTime: null })}>
            <IconX className='mr-1 h-3 w-3' />
            {t('timeRange.reset')}
          </Button>
        )}

        {summary && <span className='text-muted-foreground text-xs tabular-nums'>{summary}</span>}
      </div>
    );
  }

  return (
    <div className='bg-card flex flex-wrap items-center gap-2 rounded-lg border p-3'>
      <div className='flex items-center gap-1.5 text-sm font-medium'>
        <IconCalendar className='text-muted-foreground h-4 w-4' />
        {t('timeRange.label')}
      </div>

      <div className='flex flex-wrap items-center gap-1'>
        {presets.map((preset) => (
          <Button
            key={preset.key}
            variant={activeKey === preset.key ? 'default' : 'outline'}
            size='sm'
            className='h-8 text-xs'
            // Until earliestDate is ready, allTime can only emit startTime: null, and
            // the backend then falls back to its own 30-day default, which does not
            // match what the preset promises.
            disabled={preset.key === 'allTime' && !earliestDate}
            onClick={() => onChange(preset.range)}
          >
            {t(`timeRange.${preset.key}`)}
          </Button>
        ))}
      </div>

      <DateRangePicker
        startDate={value.startTime}
        endDate={value.endTime}
        onStartChange={(date) =>
          // An end date with no start date is not a range the queries can express: every
          // consumer keys off startTime, so it would fall back to allTime and silently
          // discard the end date. Clearing the start clears the end with it.
          onChange({ startTime: date ? formatDate(date) : null, endTime: date ? value.endTime : null })
        }
        onEndChange={(date) => {
          if (!value.startTime) return;
          onChange({ startTime: value.startTime, endTime: date ? formatDate(date) : null });
        }}
      />

      {isCustom && (
        <Button variant='ghost' size='sm' className='text-muted-foreground h-8 text-xs' onClick={() => onChange({ startTime: null, endTime: null })}>
          <IconX className='mr-1 h-3 w-3' />
          {t('timeRange.reset')}
        </Button>
      )}
    </div>
  );
}
