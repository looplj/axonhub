import { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Separator } from '@/components/ui/separator';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { formatNumber } from '@/utils/format-number';
import { useApiKeyTokenUsageStats } from '../data/apikeys';
import type { ApiKey } from '../data/schema';

type TimeRange = 'today' | 'last7days' | 'all';

const pct = (value: number, total: number) =>
  total > 0 ? ((value / total) * 100).toFixed(1) : '0.0';

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
          // Get start of today in local timezone
          const todayLocal = new Date(now.getFullYear(), now.getMonth(), now.getDate());
          return {
            createdAtGTE: todayLocal.toISOString(),
            createdAtLTE: now.toISOString(),
          };
        }
        case 'last7days': {
          // Get 7 days ago from start of today in local timezone
          const todayLocal = new Date(now.getFullYear(), now.getMonth(), now.getDate());
          const last7daysLocal = new Date(todayLocal);
          last7daysLocal.setDate(last7daysLocal.getDate() - 7);
          return {
            createdAtGTE: last7daysLocal.toISOString(),
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
    apiKey
      ? {
          apiKeyIds: [apiKey.id],
          ...usageDateRangeWhere,
        }
      : undefined,
    {
      enabled: open && !!apiKey,
    }
  );

  const stat = usageStats?.[0];
  const totalTokens = stat ? stat.inputTokens + stat.outputTokens + stat.cachedTokens + stat.reasoningTokens : 0;
  const hasTopModels = stat && stat.topModels && stat.topModels.length > 0;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-2xl max-h-[90vh] flex flex-col">
        <DialogHeader className="flex flex-col space-y-3 sm:flex-row sm:items-center sm:justify-between sm:space-y-0">
          <DialogTitle className="text-base sm:text-lg">
            {t('apikeys.tokenUsageChart.title')} - {apiKey?.name}
          </DialogTitle>
          <Tabs value={timeRange} onValueChange={(value) => setTimeRange(value as TimeRange)}>
            <TabsList className="grid w-full grid-cols-3 sm:w-auto sm:mr-6">
              <TabsTrigger value="today">{t('apikeys.tokenUsageChart.today')}</TabsTrigger>
              <TabsTrigger value="last7days">{t('apikeys.tokenUsageChart.last7days')}</TabsTrigger>
              <TabsTrigger value="all">{t('apikeys.tokenUsageChart.all')}</TabsTrigger>
            </TabsList>
          </Tabs>
        </DialogHeader>
        <div className="space-y-2 overflow-y-auto flex-1 min-h-0 scrollbar-thin -ml-6 pl-6">
          {isLoading ? (
            <Skeleton className="h-[200px] w-full" />
          ) : !stat || totalTokens === 0 ? (
            <div className="flex h-[200px] items-center justify-center text-muted-foreground">
              {t('apikeys.tokenUsageChart.noData')}
            </div>
          ) : (
            <div className="space-y-4" style={{ opacity: isFetching ? 0.6 : 1, transition: 'opacity 0.2s' }}>
              <div>
                <h3 className="mb-2 text-sm font-medium">{t('apikeys.tokenUsageChart.overallUsage')}</h3>
                <div className="rounded-lg border overflow-x-auto">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-2/5 whitespace-nowrap">{t('apikeys.tokenUsageChart.tokenType')}</TableHead>
                        <TableHead className="w-[30%] text-center whitespace-nowrap">{t('apikeys.tokenUsageChart.count')}</TableHead>
                        <TableHead className="w-[30%] text-center whitespace-nowrap">{t('apikeys.tokenUsageChart.percentage')}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow>
                        <TableCell className="font-medium">{t('apikeys.columns.inputTokens')}</TableCell>
                        <TableCell className="text-center tabular-nums">{formatNumber(stat.inputTokens)}</TableCell>
                        <TableCell className="text-center tabular-nums">
                          {pct(stat.inputTokens, totalTokens)}%
                        </TableCell>
                      </TableRow>
                      <TableRow>
                        <TableCell className="font-medium">{t('apikeys.columns.outputTokens')}</TableCell>
                        <TableCell className="text-center tabular-nums">{formatNumber(stat.outputTokens)}</TableCell>
                        <TableCell className="text-center tabular-nums">
                          {pct(stat.outputTokens, totalTokens)}%
                        </TableCell>
                      </TableRow>
                      <TableRow>
                        <TableCell className="font-medium">{t('apikeys.columns.cachedTokens')}</TableCell>
                        <TableCell className="text-center tabular-nums">{formatNumber(stat.cachedTokens)}</TableCell>
                        <TableCell className="text-center tabular-nums">
                          {pct(stat.cachedTokens, totalTokens)}%
                        </TableCell>
                      </TableRow>
                      <TableRow>
                        <TableCell className="font-medium">{t('apikeys.columns.reasoningTokens')}</TableCell>
                        <TableCell className="text-center tabular-nums">{formatNumber(stat.reasoningTokens)}</TableCell>
                        <TableCell className="text-center tabular-nums">
                          {pct(stat.reasoningTokens, totalTokens)}%
                        </TableCell>
                      </TableRow>
                      <TableRow className="bg-muted/50 font-semibold">
                        <TableCell>{t('apikeys.tokenUsageChart.total')}</TableCell>
                        <TableCell className="text-center tabular-nums">{formatNumber(totalTokens)}</TableCell>
                        <TableCell className="text-center tabular-nums">100%</TableCell>
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
                      const modelTotal = model.inputTokens + model.outputTokens + model.cachedTokens + model.reasoningTokens;
                      return (
                        <div key={model.modelId} className="rounded-lg border">
                          <div className="bg-muted/30 px-4 py-2">
                            <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                              <span className="font-medium text-sm break-all">
                                #{index + 1} {model.modelId}
                              </span>
                              <span className="text-sm text-muted-foreground whitespace-nowrap">
                                {t('apikeys.tokenUsageChart.totalTokens')}: {formatNumber(modelTotal)}
                              </span>
                            </div>
                          </div>
                          <div className="overflow-x-auto">
                            <Table>
                              <TableBody>
                                <TableRow>
                                  <TableCell className="w-2/5 font-medium whitespace-nowrap">{t('apikeys.columns.inputTokens')}</TableCell>
                                  <TableCell className="w-[30%] text-center tabular-nums">{formatNumber(model.inputTokens)}</TableCell>
                                  <TableCell className="w-[30%] text-center tabular-nums whitespace-nowrap">
                                    {pct(model.inputTokens, modelTotal)}%
                                  </TableCell>
                                </TableRow>
                                <TableRow>
                                  <TableCell className="font-medium whitespace-nowrap">{t('apikeys.columns.outputTokens')}</TableCell>
                                  <TableCell className="text-center tabular-nums">{formatNumber(model.outputTokens)}</TableCell>
                                  <TableCell className="text-center tabular-nums whitespace-nowrap">
                                    {pct(model.outputTokens, modelTotal)}%
                                  </TableCell>
                                </TableRow>
                                <TableRow>
                                  <TableCell className="font-medium whitespace-nowrap">{t('apikeys.columns.cachedTokens')}</TableCell>
                                  <TableCell className="text-center tabular-nums">{formatNumber(model.cachedTokens)}</TableCell>
                                  <TableCell className="text-center tabular-nums whitespace-nowrap">
                                    {pct(model.cachedTokens, modelTotal)}%
                                  </TableCell>
                                </TableRow>
                                <TableRow>
                                  <TableCell className="font-medium whitespace-nowrap">{t('apikeys.columns.reasoningTokens')}</TableCell>
                                  <TableCell className="text-center tabular-nums">{formatNumber(model.reasoningTokens)}</TableCell>
                                  <TableCell className="text-center tabular-nums whitespace-nowrap">
                                    {pct(model.reasoningTokens, modelTotal)}%
                                  </TableCell>
                                </TableRow>
                              </TableBody>
                            </Table>
                          </div>
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
