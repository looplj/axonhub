import { useState, useCallback, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { IconCalendar, IconX, IconFilter } from '@tabler/icons-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Calendar } from '@/components/ui/calendar';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { useAnalyticsFilterStore } from '@/stores/analyticsStore';
import { useAllChannelSummarys } from '@/features/channels/data/channels';
import { useApiKeys } from '@/features/apikeys/data/apikeys';
import { useUsers } from '@/features/users/data/users';
import { useProjects } from '@/features/projects/data/projects';
import { AnalyticsFacetedFilter } from './analytics-faceted-filter';

// Calendar Date → 'YYYY-MM-DD' 字符串（直接取本地年月日，不做时区转换）
function formatDate(date: Date): string {
  const y = date.getFullYear();
  const m = String(date.getMonth() + 1).padStart(2, '0');
  const d = String(date.getDate()).padStart(2, '0');
  return `${y}-${m}-${d}`;
}

// 'YYYY-MM-DD' → Date（用于 Calendar selected）
function parseDate(dateStr: string): Date {
  const [y, m, d] = dateStr.split('-').map(Number);
  return new Date(y, m - 1, d);
}

interface DateRangePickerProps {
  startDate: string | null;
  endDate: string | null;
  onStartChange: (date: Date | null) => void;
  onEndChange: (date: Date | null) => void;
}

function DateRangePicker({ startDate, endDate, onStartChange, onEndChange }: DateRangePickerProps) {
  const { t } = useTranslation();
  const [startOpen, setStartOpen] = useState(false);
  const [endOpen, setEndOpen] = useState(false);

  const handleStartDateSelect = useCallback(
    (date: Date | undefined) => {
      if (date && endDate) {
        const end = parseDate(endDate);
        if (date > end) {
          toast.error(t('analytics.filter.startDateAfterEndError'));
          return;
        }
      }
      onStartChange(date || null);
      setStartOpen(false);
    },
    [endDate, onStartChange, t]
  );

  const handleEndDateSelect = useCallback(
    (date: Date | undefined) => {
      if (date && startDate) {
        const start = parseDate(startDate);
        if (date < start) {
          toast.error(t('analytics.filter.endDateBeforeStartError'));
          return;
        }
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
            className={cn(
              'h-8 w-[130px] justify-start text-left text-xs font-normal',
              !startDate && 'text-muted-foreground'
            )}
          >
            <IconCalendar className='mr-1 h-3 w-3' />
            {startDate || t('analytics.filter.startDate')}
          </Button>
        </PopoverTrigger>
        <PopoverContent className='w-auto p-0' align='start'>
          <Calendar
            mode='single'
            selected={startDate ? parseDate(startDate) : undefined}
            onSelect={handleStartDateSelect}
            initialFocus
          />
        </PopoverContent>
      </Popover>

      <span className='text-muted-foreground'>~</span>

      <Popover open={endOpen} onOpenChange={setEndOpen}>
        <PopoverTrigger asChild>
          <Button
            variant='outline'
            className={cn(
              'h-8 w-[130px] justify-start text-left text-xs font-normal',
              !endDate && 'text-muted-foreground'
            )}
          >
            <IconCalendar className='mr-1 h-3 w-3' />
            {endDate || t('analytics.filter.endDate')}
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

interface AnalyticsFilterBarProps {
  earliestDate?: string | null;
}

export function AnalyticsFilterBar({ earliestDate }: AnalyticsFilterBarProps) {
  const { t } = useTranslation();
  const filter = useAnalyticsFilterStore((state) => state.filter);
  const {
    setStartTime,
    setEndTime,
    setProjectIDs,
    setChannelIDs,
    setModelIDs,
    setAPIKeyIDs,
    setUserIDs,
    resetFilter,
  } = useAnalyticsFilterStore();

  // Fetch real data for dropdowns
  const { data: channels, isLoading: isLoadingChannels } = useAllChannelSummarys();
  const { data: apiKeysData, isLoading: isLoadingApiKeys } = useApiKeys({ first: 100 });
  const { data: usersData, isLoading: isLoadingUsers } = useUsers({ first: 100 });
  const { data: projectsData, isLoading: isLoadingProjects } = useProjects({ first: 100 });

  const channelOptions = useMemo(
    () =>
      (channels?.edges || []).map((edge) => ({
        label: edge.node.name,
        value: String(edge.node.id),
      })),
    [channels]
  );

  const modelOptions = useMemo(() => {
    const modelSet = new Set<string>();
    (channels?.edges || []).forEach((edge) => {
      (edge.node.allModelEntries || []).forEach((entry) => {
        if (entry.actualModel) modelSet.add(entry.actualModel);
      });
    });
    return Array.from(modelSet).sort().map((m) => ({ label: m, value: m }));
  }, [channels]);

  const apiKeyOptions = useMemo(
    () =>
      (apiKeysData?.edges || []).map((edge) => ({
        label: edge.node.name,
        value: String(edge.node.id),
      })),
    [apiKeysData]
  );

  const userOptions = useMemo(
    () =>
      (usersData?.edges || []).map((edge) => ({
        label: `${edge.node.firstName || ''} ${edge.node.lastName || ''}`.trim() || edge.node.email,
        value: String(edge.node.id),
      })),
    [usersData]
  );

  const projectOptions = useMemo(
    () =>
      (projectsData?.edges || []).map((edge) => ({
        label: edge.node.name,
        value: String(edge.node.id),
      })),
    [projectsData]
  );

  const handleStartDate = useCallback(
    (date: Date | null) => {
      setStartTime(date ? formatDate(date) : null);
    },
    [setStartTime]
  );

  const handleEndDate = useCallback(
    (date: Date | null) => {
      setEndTime(date ? formatDate(date) : null);
    },
    [setEndTime]
  );

  const setQuickRange = useCallback(
    (days: number) => {
      const now = new Date();
      const start = new Date();
      start.setDate(now.getDate() - days + 1);
      start.setHours(0, 0, 0, 0);
      setStartTime(formatDate(start));
      setEndTime(formatDate(now));
    },
    [setStartTime, setEndTime]
  );

  const setQuickMonth = useCallback(() => {
    const now = new Date();
    const start = new Date(now.getFullYear(), now.getMonth(), 1);
    setStartTime(formatDate(start));
    setEndTime(formatDate(now));
  }, [setStartTime, setEndTime]);

  const setQuickYear = useCallback(() => {
    const now = new Date();
    const start = new Date(now.getFullYear(), 0, 1);
    setStartTime(formatDate(start));
    setEndTime(formatDate(now));
  }, [setStartTime, setEndTime]);

  const setAllTime = useCallback(() => {
    if (earliestDate) {
      setStartTime(earliestDate);
      setEndTime(formatDate(new Date()));
    }
  }, [earliestDate, setStartTime, setEndTime]);

  const hasFilters =
    filter.startTime ||
    filter.endTime ||
    filter.projectIDs ||
    filter.channelIDs ||
    filter.modelIDs ||
    filter.apiKeyIDs ||
    filter.userIDs;

  return (
    <div className='space-y-3 rounded-lg border bg-card p-4'>
      {/* Date Filters */}
      <div className='flex flex-wrap items-center gap-2'>
        <div className='flex items-center gap-1.5 text-sm font-medium'>
          <IconFilter className='h-4 w-4 text-muted-foreground' />
          {t('analytics.filter.dateRange')}
        </div>

        {/* Date Range */}
        <DateRangePicker
          startDate={filter.startTime ?? null}
          endDate={filter.endTime ?? null}
          onStartChange={handleStartDate}
          onEndChange={handleEndDate}
        />

        {/* Quick Range Buttons */}
        <div className='flex flex-wrap items-center gap-1'>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={() => setQuickRange(1)}>
            {t('analytics.filter.today')}
          </Button>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={() => setQuickRange(7)}>
            {t('analytics.filter.last7Days')}
          </Button>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={() => setQuickRange(30)}>
            {t('analytics.filter.last30Days')}
          </Button>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={setQuickMonth}>
            {t('analytics.filter.thisMonth')}
          </Button>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={setQuickYear}>
            {t('analytics.filter.thisYear')}
          </Button>
          <Button variant='outline' size='sm' className='h-8 text-xs' onClick={setAllTime}>
            {t('analytics.filter.allTime')}
          </Button>
        </div>
      </div>

      {/* Dimension Filters */}
      <div className='flex flex-wrap items-center gap-2'>
        <AnalyticsFacetedFilter
          title={t('analytics.filter.project')}
          options={projectOptions}
          selectedValues={filter.projectIDs || []}
          onSelectedValuesChange={setProjectIDs}
          isLoading={isLoadingProjects}
        />

        <AnalyticsFacetedFilter
          title={t('analytics.filter.channel')}
          options={channelOptions}
          selectedValues={filter.channelIDs || []}
          onSelectedValuesChange={setChannelIDs}
          isLoading={isLoadingChannels}
        />

        <AnalyticsFacetedFilter
          title={t('analytics.filter.model')}
          options={modelOptions}
          selectedValues={filter.modelIDs || []}
          onSelectedValuesChange={setModelIDs}
          isLoading={isLoadingChannels}
        />

        <AnalyticsFacetedFilter
          title={t('analytics.filter.apiKey')}
          options={apiKeyOptions}
          selectedValues={filter.apiKeyIDs || []}
          onSelectedValuesChange={setAPIKeyIDs}
          isLoading={isLoadingApiKeys}
        />

        <AnalyticsFacetedFilter
          title={t('analytics.filter.user')}
          options={userOptions}
          selectedValues={filter.userIDs || []}
          onSelectedValuesChange={setUserIDs}
          isLoading={isLoadingUsers}
        />

        {/* Reset Button */}
        {hasFilters && (
          <Button variant='ghost' size='sm' className='h-8 text-xs text-muted-foreground' onClick={resetFilter}>
            <IconX className='mr-1 h-3 w-3' />
            {t('analytics.filter.reset')}
          </Button>
        )}
      </div>
    </div>
  );
}
