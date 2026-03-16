import { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { BarChart3 } from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatNumber } from '@/utils/format-number';
import { useApiKeyTokenUsageStats } from '../data/apikeys';
import type { ApiKey } from '../data/schema';

type TimeRange = 'today' | 'last7days' | 'all';

interface ApiKeyTokenChartDialogProps {
  apiKey: ApiKey | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function ApiKeyTokenChartDialog({ apiKey, open, onOpenChange }: ApiKeyTokenChartDialogProps) {
  const { t } = useTranslation();
  const [timeRange, setTimeRange] = useState<TimeRange>('today');

  const usageDateRangeWhere = useMemo(() => {
    const getDateRange = (range: TimeRange) => {
      const now = new Date();

      switch (range) {
        case 'today': {
          // Get start of today in UTC
          const todayUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
          return {
            createdAtGTE: todayUTC.toISOString(),
            createdAtLTE: now.toISOString(),
          };
        }
        case 'last7days': {
          // Get 7 days ago from start of today in UTC
          const todayUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate()));
          const last7daysUTC = new Date(todayUTC);
          last7daysUTC.setUTCDate(last7daysUTC.getUTCDate() - 7);
          return {
            createdAtGTE: last7daysUTC.toISOString(),
            createdAtLTE: now.toISOString(),
          };
        }
        case 'all':
          return {};
        default:
          return {};
      }
    };

    return getDateRange(timeRange);
  }, [timeRange]);

  const { data: usageStats, isLoading, isFetching } = useApiKeyTokenUsageStats(
    {
      apiKeyIds: apiKey ? [apiKey.id] : [],
      ...usageDateRangeWhere,
    },
    {
      enabled: open && !!apiKey,
    }
  );

  const stat = usageStats?.[0];
  const totalTokens = stat ? stat.inputTokens + stat.outputTokens + stat.cachedTokens : 0;
  const hasTopModels = stat && stat.topModels && stat.topModels.length > 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-3xl">
        <DialogHeader className="flex flex-row items-center justify-between space-y-0">
          <DialogTitle>
            {t('apikeys.tokenUsageChart.title')} - {apiKey?.name}
          </DialogTitle>
          <Tabs value={timeRange} onValueChange={(value) => setTimeRange(value as TimeRange)}>
            <TabsList>
              <TabsTrigger value="today">{t('apikeys.tokenUsageChart.today')}</TabsTrigger>
              <TabsTrigger value="last7days">{t('apikeys.tokenUsageChart.last7days')}</TabsTrigger>
              <TabsTrigger value="all">{t('apikeys.tokenUsageChart.all')}</TabsTrigger>
            </TabsList>
          </Tabs>
        </DialogHeader>
        <div className="space-y-2">

          {isLoading ? (
            <Skeleton className="h-[200px] w-full" />
          ) : totalTokens === 0 ? (
            <div className="flex h-[200px] items-center justify-center text-muted-foreground">
              {t('apikeys.tokenUsageChart.noData')}
            </div>
          ) : (
            <div className="space-y-4" style={{ opacity: isFetching ? 0.6 : 1, transition: 'opacity 0.2s' }}>
              <div>
                <h3 className="mb-2 text-sm font-medium">{t('apikeys.tokenUsageChart.overallUsage')}</h3>
                <div className="rounded-lg border">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('apikeys.tokenUsageChart.tokenType')}</TableHead>
                        <TableHead className="text-right">{t('apikeys.tokenUsageChart.count')}</TableHead>
                        <TableHead className="text-right">{t('apikeys.tokenUsageChart.percentage')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow>
                        <TableCell className="font-medium">{t('apikeys.columns.inputTokens')}</TableCell>
                        <TableCell className="text-right tabular-nums">{formatNumber(stat.inputTokens)}</TableCell>
                        <TableCell className="text-right tabular-nums">
                          {((stat.inputTokens / totalTokens) * 100).toFixed(1)}%
                        </TableCell>
                      </TableRow>
                      <TableRow>
                        <TableCell className="font-medium">{t('apikeys.columns.outputTokens')}</TableCell>
                        <TableCell className="text-right tabular-nums">{formatNumber(stat.outputTokens)}</TableCell>
                        <TableCell className="text-right tabular-nums">
                          {((stat.outputTokens / totalTokens) * 100).toFixed(1)}%
                        </TableCell>
                      </TableRow>
                      <TableRow>
                        <TableCell className="font-medium">{t('apikeys.columns.cachedTokens')}</TableCell>
                        <TableCell className="text-right tabular-nums">{formatNumber(stat.cachedTokens)}</TableCell>
                        <TableCell className="text-right tabular-nums">
                          {((stat.cachedTokens / totalTokens) * 100).toFixed(1)}%
                        </TableCell>
                      </TableRow>
                      <TableRow className="bg-muted/50 font-semibold">
                        <TableCell>{t('apikeys.tokenUsageChart.total')}</TableCell>
                        <TableCell className="text-right tabular-nums">{formatNumber(totalTokens)}</TableCell>
                        <TableCell className="text-right tabular-nums">100%</TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
                </div>
              </div>

              {hasTopModels && (
                <div>
                  <Separator className="mb-4" />
                  <h3 className="mb-3 text-sm font-medium">{t('apikeys.tokenUsageChart.topModels')}</h3>
                  <div className="space-y-4">
                    {stat.topModels.map((model, index) => {
                      const modelTotal = model.inputTokens + model.outputTokens + model.cachedTokens;
                      return (
                        <div key={model.modelId} className="rounded-lg border">
                          <div className="bg-muted/30 px-4 py-2">
                            <div className="flex items-center justify-between">
                              <span className="font-medium">
                                #{index + 1} {model.modelId}
                              </span>
                              <span className="text-sm text-muted-foreground">
                                {t('apikeys.tokenUsageChart.totalTokens')}: {formatNumber(modelTotal)}
                              </span>
                            </div>
                          </div>
                          <Table>
                            <TableBody>
                              <TableRow>
                                <TableCell className="font-medium">{t('apikeys.columns.inputTokens')}</TableCell>
                                <TableCell className="text-right tabular-nums">{formatNumber(model.inputTokens)}</TableCell>
                                <TableCell className="text-right tabular-nums">
                                  {((model.inputTokens / modelTotal) * 100).toFixed(1)}%
                                </TableCell>
                              </TableRow>
                              <TableRow>
                                <TableCell className="font-medium">{t('apikeys.columns.outputTokens')}</TableCell>
                                <TableCell className="text-right tabular-nums">{formatNumber(model.outputTokens)}</TableCell>
                                <TableCell className="text-right tabular-nums">
                                  {((model.outputTokens / modelTotal) * 100).toFixed(1)}%
                                </TableCell>
                              </TableRow>
                              <TableRow>
                                <TableCell className="font-medium">{t('apikeys.columns.cachedTokens')}</TableCell>
                                <TableCell className="text-right tabular-nums">{formatNumber(model.cachedTokens)}</TableCell>
                                <TableCell className="text-right tabular-nums">
                                  {((model.cachedTokens / modelTotal) * 100).toFixed(1)}%
                                </TableCell>
                              </TableRow>
                            </TableBody>
                          </Table>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
