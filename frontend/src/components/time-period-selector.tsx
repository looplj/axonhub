import { useTranslation } from 'react-i18next';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';

export type TimePeriod = 'allTime' | 'month' | 'week' | 'day';
export type FastestTimeWindow = 'month' | 'week' | 'day';

const DEFAULT_PERIODS: readonly TimePeriod[] = ['allTime', 'month', 'week', 'day'];
const DEFAULT_FASTEST_PERIODS: readonly FastestTimeWindow[] = ['month', 'week', 'day'];

interface TimePeriodSelectorProps<T extends string = TimePeriod> {
  value: T;
  onChange: (value: T) => void;
  periods?: readonly T[];
}

function getDefaultPeriods<T extends string>(value: T): readonly T[] {
  const isFastestTimeWindow = ['month', 'week', 'day'].includes(value) && value !== 'allTime';
  if (isFastestTimeWindow) {
    return DEFAULT_FASTEST_PERIODS as readonly T[];
  }
  return DEFAULT_PERIODS as readonly T[];
}

export function TimePeriodSelector<T extends string>({ value, onChange, periods }: TimePeriodSelectorProps<T>) {
  const effectivePeriods = periods ?? getDefaultPeriods(value);
  const { t } = useTranslation();

  return (
    <Tabs value={value} onValueChange={(v) => onChange(v as T)}>
      <TabsList className='h-7 p-0.5'>
        {effectivePeriods.map((period) => (
          <TabsTrigger key={period} value={period} className='h-6 px-2 text-[10px]'>
            {t(`dashboard.stats.${period === 'allTime' ? 'all' : period}`)}
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  );
}
