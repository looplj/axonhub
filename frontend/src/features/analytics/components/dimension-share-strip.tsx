import { useTranslation } from 'react-i18next';
import { Card, CardContent } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import { formatCurrencySimple } from '../utils/format-currency';
import type { AnalyticsDimensionStat } from '../data/analytics';
import type { DistributionMetric, DimensionDistributionGroup } from './dimension-distribution';

const TOP_N = 5;
const RANK_COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)'];
const OTHER_COLOR = 'var(--chart-6)';

interface DimensionShareStripProps {
  metric: DistributionMetric;
  groups: DimensionDistributionGroup[];
  isLoading: boolean;
  currencyCode: string;
}

interface Segment {
  name: string;
  value: number;
  share: number;
}

interface DimensionShare {
  key: string;
  label: string;
  segments: Segment[];
}

function buildSegments(data: AnalyticsDimensionStat[], metric: DistributionMetric): Segment[] {
  const sorted = [...data].sort((a, b) => b[metric] - a[metric]);
  const total = sorted.reduce((sum, item) => sum + item[metric], 0);
  if (total <= 0) return [];

  const segments: Segment[] = sorted.slice(0, TOP_N).map((item) => ({
    name: item.name,
    value: item[metric],
    share: (item[metric] / total) * 100,
  }));

  const tail = sorted.slice(TOP_N);
  const tailValue = tail.reduce((sum, item) => sum + item[metric], 0);
  if (tail.length > 0 && tailValue > 0) {
    segments.push({ name: '', value: tailValue, share: (tailValue / total) * 100 });
  }

  return segments;
}

/** Colour by rank, not by name: the four bars each have their own top-N, so a shared
 * legend can only key on position. The bar's leading name is called out beside it so
 * the biggest segment is readable without hovering. */
function segmentColor(index: number, isOther: boolean): string {
  return isOther ? OTHER_COLOR : RANK_COLORS[index % RANK_COLORS.length];
}

/** Four equal-length 100% stacked bars, one per dimension. Normalising every bar to the
 * same 100% and left-aligning them is what makes the segments directly comparable,
 * which is what the previous donut matrix could not do. */
export function DimensionShareStrip({ metric, groups, isLoading, currencyCode }: DimensionShareStripProps) {
  const { t } = useTranslation();

  const shares: DimensionShare[] = groups.map((group) => ({
    key: group.key,
    label: group.label,
    segments: buildSegments(group.data, metric),
  }));

  const visible = shares.filter((share) => share.segments.length > 0);
  const formatValue = (value: number) =>
    metric === 'cost' ? formatCurrencySimple(value, currencyCode) : formatNumber(value);

  return (
    <Card className='hover-card'>
      <CardContent className='space-y-4 pt-4'>
        <div className='flex flex-wrap items-baseline justify-between gap-2'>
          <h2 className='text-sm font-semibold'>{t('analytics.shareStrip.title')}</h2>
          <p className='text-muted-foreground text-xs'>{t('analytics.shareStrip.description')}</p>
        </div>

        {isLoading ? (
          <Skeleton className='h-[140px] w-full' />
        ) : visible.length === 0 ? (
          <div className='flex h-[120px] items-center justify-center'>
            <p className='text-muted-foreground text-sm'>{t('analytics.table.noData')}</p>
          </div>
        ) : (
          <>
            <div className='space-y-3'>
              {visible.map((share) => {
                const [leader] = share.segments;
                return (
                  <div key={share.key} className='grid grid-cols-[minmax(4rem,7rem)_1fr] items-center gap-3'>
                    <div className='min-w-0 text-xs font-medium'>
                      <div className='truncate'>{share.label}</div>
                      <div className='text-muted-foreground truncate text-[11px]' title={leader.name}>
                        {leader.name || t('dashboard.charts.other')} {leader.share.toFixed(0)}%
                      </div>
                    </div>
                    <div className='flex h-6 overflow-hidden rounded-md bg-muted'>
                      {share.segments.map((segment, index) => {
                        const isOther = index === share.segments.length - 1 && !segment.name;
                        return (
                          <div
                            key={index}
                            className='h-full'
                            style={{ width: `${segment.share}%`, backgroundColor: segmentColor(index, isOther) }}
                            title={`${segment.name || t('dashboard.charts.other')} · ${formatValue(segment.value)} (${segment.share.toFixed(1)}%)`}
                          />
                        );
                      })}
                    </div>
                  </div>
                );
              })}
            </div>

            <div className='flex flex-wrap gap-x-4 gap-y-1.5'>
              {[...Array(TOP_N)].map((_, index) => (
                <span key={index} className='flex items-center gap-1.5 text-xs'>
                  <span className='h-2 w-2 shrink-0 rounded-full' style={{ backgroundColor: RANK_COLORS[index] }} />
                  <span className='text-muted-foreground'>{t('analytics.shareStrip.rank', { count: index + 1 })}</span>
                </span>
              ))}
              <span className='flex items-center gap-1.5 text-xs'>
                <span className='h-2 w-2 shrink-0 rounded-full' style={{ backgroundColor: OTHER_COLOR }} />
                <span className='text-muted-foreground'>{t('dashboard.charts.other')}</span>
              </span>
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
