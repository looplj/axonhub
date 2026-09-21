import { useState, useCallback, useMemo } from 'react';
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
function buildPresets(earliestDate?: string | null): Preset[] {
  const now = new Date();
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

interface TimeRangeFilterProps {
  value: TimeRangeValue;
  onChange: (range: TimeRangeValue) => void;
  earliestDate?: string | null;
}

/** Unified time range filter: preset buttons plus a custom start/end pair, replacing
 * the per-page TimePeriodSelector. */
export function TimeRangeFilter({ value, onChange, earliestDate }: TimeRangeFilterProps) {
  const { t } = useTranslation();
  const presets = useMemo(() => buildPresets(earliestDate), [earliestDate]);

  const activeKey = useMemo(() => {
    const match = presets.find((p) => p.range.startTime === value.startTime && p.range.endTime === value.endTime);
    return match?.key ?? null;
  }, [presets, value.startTime, value.endTime]);

  const isCustom = !activeKey && (value.startTime !== null || value.endTime !== null);

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
        onStartChange={(date) => onChange({ startTime: date ? formatDate(date) : null, endTime: value.endTime })}
        onEndChange={(date) => onChange({ startTime: value.startTime, endTime: date ? formatDate(date) : null })}
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
