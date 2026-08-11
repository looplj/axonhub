import { useRef, useState } from 'react';
import { ChevronDown, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { AUTO_REFRESH_INTERVALS, type AutoRefreshInterval, type EnabledAutoRefreshInterval } from '@/hooks/use-auto-refresh-interval';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';

const CLOSED_VALUE = 'closed';
const MIN_REFRESH_SPIN_DURATION_MS = 1000;

interface AutoRefreshControlProps {
  interval: AutoRefreshInterval;
  onIntervalChange: (interval: AutoRefreshInterval) => void;
  onRefresh: () => void | Promise<unknown>;
  disabled?: boolean;
  className?: string;
}

function formatInterval(interval: EnabledAutoRefreshInterval) {
  return `${interval / 1000}s`;
}

export function AutoRefreshControl({ interval, onIntervalChange, onRefresh, disabled = false, className }: AutoRefreshControlProps) {
  const { t } = useTranslation();
  const [isRefreshing, setIsRefreshing] = useState(false);
  const refreshInFlightRef = useRef(false);
  const selectedValue = interval === null ? CLOSED_VALUE : interval.toString();

  const handleRefresh = async () => {
    if (refreshInFlightRef.current) return;

    refreshInFlightRef.current = true;
    setIsRefreshing(true);
    const startedAt = Date.now();

    try {
      await onRefresh();
    } catch {
      // Query errors are surfaced by the page's existing error handling.
    } finally {
      const remainingDuration = MIN_REFRESH_SPIN_DURATION_MS - (Date.now() - startedAt);
      if (remainingDuration > 0) {
        await new Promise((resolve) => setTimeout(resolve, remainingDuration));
      }

      refreshInFlightRef.current = false;
      setIsRefreshing(false);
    }
  };

  const handleValueChange = (value: string) => {
    const nextInterval = AUTO_REFRESH_INTERVALS.find((candidate) => candidate.toString() === value) ?? null;
    onIntervalChange(nextInterval);
  };

  return (
    <div className={cn('inline-flex shrink-0', className)}>
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={handleRefresh}
        disabled={disabled || isRefreshing}
        aria-busy={isRefreshing}
        aria-label={t('common.refresh')}
        data-testid='manual-refresh-button'
        className='active:bg-accent active:text-accent-foreground dark:active:bg-accent h-8 rounded-r-none border-r-0'
      >
        <RefreshCw className={cn('mr-2 h-4 w-4', (interval !== null || isRefreshing) && 'animate-spin')} />
        <span className='pointer-events-none select-none'>{interval === null ? t('common.refresh') : formatInterval(interval)}</span>
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            type='button'
            variant='outline'
            size='icon-sm'
            aria-label={t('common.autoRefresh')}
            data-testid='auto-refresh-trigger'
            className='data-[state=open]:bg-accent data-[state=open]:text-accent-foreground dark:data-[state=open]:bg-accent rounded-l-none px-0'
          >
            <ChevronDown className='h-4 w-4' />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' aria-label={t('common.autoRefresh')} data-testid='auto-refresh-menu' className='min-w-36'>
          <DropdownMenuLabel>{t('common.autoRefresh')}</DropdownMenuLabel>
          <DropdownMenuSeparator />
          <DropdownMenuRadioGroup value={selectedValue} onValueChange={handleValueChange}>
            <DropdownMenuRadioItem value={CLOSED_VALUE} data-testid='auto-refresh-option-closed'>
              {t('common.close')}
            </DropdownMenuRadioItem>
            {AUTO_REFRESH_INTERVALS.map((candidate) => (
              <DropdownMenuRadioItem key={candidate} value={candidate.toString()} data-testid={`auto-refresh-option-${candidate}`}>
                {formatInterval(candidate)}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
