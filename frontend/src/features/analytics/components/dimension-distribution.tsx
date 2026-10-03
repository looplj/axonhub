import { useTranslation } from 'react-i18next';
import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import { formatCurrencySimple } from '../utils/format-currency';
import type { AnalyticsDimensionStat } from '../data/analytics';

export type DistributionMetric = 'requestCount' | 'totalTokens' | 'cost';

/** Top-N plus one "other" bar, so the tail is aggregated rather than hidden. */
const TOP_N = 5;
const BAR_COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)'];
const OTHER_COLOR = 'var(--chart-6)';

export interface DimensionDistributionGroup {
  key: string;
  label: string;
  data: AnalyticsDimensionStat[];
}

interface DimensionDistributionProps {
  metric: DistributionMetric;
  groups: DimensionDistributionGroup[];
  isLoading: boolean;
  currencyCode: string;
}

interface Slice {
  name: string;
  value: number;
  share: number;
  isOther: boolean;
}

function buildSlices(data: AnalyticsDimensionStat[], metric: DistributionMetric): Slice[] {
  const sorted = [...data].sort((a, b) => b[metric] - a[metric]);
  const total = sorted.reduce((sum, item) => sum + item[metric], 0);
  if (total <= 0) return [];

  const slices: Slice[] = sorted.slice(0, TOP_N).map((item) => ({
    name: item.name,
    value: item[metric],
    share: (item[metric] / total) * 100,
    isOther: false,
  }));

  const tail = sorted.slice(TOP_N);
  const tailValue = tail.reduce((sum, item) => sum + item[metric], 0);
  if (tail.length > 0 && tailValue > 0) {
    slices.push({ name: '', value: tailValue, share: (tailValue / total) * 100, isOther: true });
  }

  return slices;
}

function formatValue(value: number, metric: DistributionMetric, currencyCode: string): string {
  return metric === 'cost' ? formatCurrencySimple(value, currencyCode) : formatNumber(value);
}

interface DistributionListProps {
  label: string;
  slices: Slice[];
  metric: DistributionMetric;
  currencyCode: string;
}

function DistributionList({ label, slices, metric, currencyCode }: DistributionListProps) {
  const { t } = useTranslation();

  const headShare = slices
    .filter((slice) => !slice.isOther)
    .slice(0, 3)
    .reduce((sum, slice) => sum + slice.share, 0);

  return (
    <Card className='hover-card min-w-0'>
      <CardContent className='space-y-3 pt-4'>
        <div className='flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1'>
          <h3 className='truncate text-sm font-semibold'>{label}</h3>
          <span className='text-muted-foreground shrink-0 text-xs tabular-nums'>
            {t('analytics.distribution.headShare', { count: 3 })}{' '}
            <span className='text-foreground font-mono font-medium'>{headShare.toFixed(0)}%</span>
          </span>
        </div>

        <div className='text-muted-foreground text-[11px] font-medium'>{t(`analytics.distribution.metric.${metric}`)}</div>

        <div className='space-y-2'>
          {slices.map((slice, index) => (
            <div key={`${slice.isOther ? 'other' : slice.name}-${index}`} className='space-y-1'>
              <div className='flex items-baseline justify-between gap-2 text-xs'>
                <span className='min-w-0 truncate' title={slice.name}>
                  {slice.isOther ? t('dashboard.charts.other') : slice.name}
                </span>
                <span className='text-muted-foreground shrink-0 font-mono tabular-nums'>
                  {formatValue(slice.value, metric, currencyCode)} · {slice.share.toFixed(1)}%
                </span>
              </div>
              <div className='h-2 overflow-hidden rounded-full bg-muted'>
                <div
                  className='h-full rounded-full'
                  style={{
                    width: `${slice.share}%`,
                    backgroundColor: slice.isOther ? OTHER_COLOR : BAR_COLORS[index % BAR_COLORS.length],
                  }}
                />
              </div>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

/** Sorted horizontal bars per dimension: each dimension reads as its own ranking, with
 * the long tail kept as an explicit "other" bar and the top-3 concentration stated as
 * a number next to the title. Bars are absolute shares of their own dimension total,
 * which is what makes the four rankings comparable at a glance. */
export function DimensionDistribution({ metric, groups, isLoading, currencyCode }: DimensionDistributionProps) {
  const { t } = useTranslation();

  const prepared = groups.map((group) => ({ ...group, slices: buildSlices(group.data, metric) }));
  const hasAnyData = prepared.some((group) => group.slices.length > 0);

  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap items-baseline justify-between gap-2'>
        <h2 className='text-sm font-semibold'>{t('analytics.distribution.title')}</h2>
        <p className='text-muted-foreground text-xs'>{t('analytics.distribution.description')}</p>
      </div>

      {isLoading ? (
        <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
          {groups.map((group) => (
            <Skeleton key={group.key} className='h-[220px]' />
          ))}
        </div>
      ) : !hasAnyData ? (
        <Card className='hover-card'>
          <CardContent className='flex h-[120px] items-center justify-center'>
            <p className='text-muted-foreground text-sm'>{t('analytics.table.noData')}</p>
          </CardContent>
        </Card>
      ) : (
        <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
          {prepared.map((group) => (
            <DistributionList
              key={group.key}
              label={group.label}
              slices={group.slices}
              metric={metric}
              currencyCode={currencyCode}
            />
          ))}
        </div>
      )}
    </div>
  );
}
