'use client';

import { useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipProps } from 'recharts';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import { useFastestChannels, useFastestChannelsExpanded } from '../data/fastest-performers';

const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)'];

type TimeWindow = '1h' | '24h' | '7d';

interface ChartData {
  name: string;
  throughput: number;
  requestCount: number;
}

function HorizontalBarChart({ data, total, height = 280 }: { data: ChartData[]; total: number; height?: number }) {
  const tooltipContent = (props: TooltipProps<number, string>) => {
    const { active, payload } = props;
    if (!active || !payload?.length) return null;

    const item = payload[0].payload as ChartData;
    const percent = total ? (item.throughput / total) * 100 : 0;

    return (
      <div className='bg-background/90 rounded-md border px-3 py-2 text-xs shadow-sm backdrop-blur'>
        <div className='text-foreground text-sm font-medium'>{item.name}</div>
        <div className='text-muted-foreground'>
          {item.throughput.toFixed(1)} tokens/s ({percent.toFixed(0)}%)
        </div>
        <div className='text-muted-foreground text-xs'>
          {item.requestCount} requests
        </div>
      </div>
    );
  };

  return (
    <ResponsiveContainer width='100%' height={height}>
      <BarChart data={data} layout='vertical' barSize={32} margin={{ left: 20, right: 20, top: 10, bottom: 10 }}>
        <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' horizontal={false} />
        <XAxis type='number' hide />
        <YAxis
          type='category'
          dataKey='name'
          width={10}
          tick={false}
          tickLine={false}
          axisLine={false}
        />
        <Tooltip content={tooltipContent} cursor={{ fill: 'var(--muted)' }} />
        <Bar dataKey='throughput' radius={[0, 4, 4, 0]}>
          {data.map((_, index) => (
            <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
          ))}
        </Bar>
      </BarChart>
    </ResponsiveContainer>
  );
}

function ChartLegend({ items }: { items: Array<{ name: string; throughput: number; requestCount: number; color: string; index: number }> }) {
  return (
    <div className='grid gap-3'>
      {items.map((item) => (
        <div key={item.name} className='grid w-full grid-cols-[auto_auto_1fr_auto] items-center gap-3'>
          <span className='text-muted-foreground w-8 text-right text-sm font-semibold tabular-nums'>
            {item.index.toString().padStart(2, '0')}.
          </span>
          <span className='h-2.5 w-2.5 rounded-full' style={{ backgroundColor: item.color }} />
          <span className='text-foreground min-w-0 text-sm font-medium break-words'>{item.name}</span>
          <div className='text-right leading-tight'>
            <div className='text-foreground text-sm font-medium tabular-nums'>{item.throughput.toFixed(1)} tok/s</div>
            <div className='text-muted-foreground text-xs tabular-nums'>{formatNumber(item.requestCount)} req</div>
          </div>
        </div>
      ))}
    </div>
  );
}

function ExpandedChannelItem({ 
  channel, 
  index,
  defaultExpanded = false
}: { 
  channel: { 
    channelName: string; 
    throughput: number; 
    requestCount: number;
    models: Array<{
      modelName: string;
      throughput: number;
      requestCount: number;
    }>;
  };
  index: number;
  defaultExpanded?: boolean;
}) {
  const [isExpanded, setIsExpanded] = useState(defaultExpanded);
  const color = COLORS[index % COLORS.length];

  const modelData = channel.models
    .map((m) => ({ name: m.modelName, throughput: m.throughput, requestCount: m.requestCount }))
    .sort((a, b) => b.throughput - a.throughput);

  const modelTotal = modelData.reduce((sum, item) => sum + item.throughput, 0);

  return (
    <div className='border-b border-border/50 last:border-0 pb-4 mb-4 last:pb-0 last:mb-0'>
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className='w-full flex items-center justify-between py-2 hover:bg-muted/50 rounded-lg px-2 transition-colors'
      >
        <div className='flex items-center gap-3'>
          <span className='text-muted-foreground w-8 text-right text-sm font-semibold tabular-nums'>
            {index + 1}.
          </span>
          <span className='h-3 w-3 rounded-full' style={{ backgroundColor: color }} />
          <span className='text-foreground text-sm font-medium'>{channel.channelName}</span>
        </div>
        <div className='flex items-center gap-4'>
          <div className='text-right leading-tight'>
            <div className='text-foreground text-sm font-medium tabular-nums'>{channel.throughput.toFixed(1)} tok/s</div>
            <div className='text-muted-foreground text-xs tabular-nums'>{formatNumber(channel.requestCount)} req</div>
          </div>
          {isExpanded ? (
            <ChevronDown className='h-4 w-4 text-muted-foreground' />
          ) : (
            <ChevronRight className='h-4 w-4 text-muted-foreground' />
          )}
        </div>
      </button>
      
      {isExpanded && (
        <div className='ml-11 mt-3 space-y-3 pl-4 border-l-2 border-border'>
          {modelData.length > 0 ? (
            <>
              <div className='h-[150px]'>
                <HorizontalBarChart data={modelData.slice(0, 5)} total={modelTotal} height={150} />
              </div>
              <div className='grid gap-2'>
                {modelData.slice(0, 5).map((model, modelIndex) => (
                  <div key={model.name} className='grid w-full grid-cols-[auto_1fr_auto] items-center gap-2 text-xs'>
                    <span className='text-muted-foreground w-6 text-right font-semibold tabular-nums'>
                      {modelIndex + 1}.
                    </span>
                    <span className='text-foreground min-w-0 font-medium break-words'>{model.name}</span>
                    <div className='text-right leading-tight'>
                      <div className='text-foreground font-medium tabular-nums'>{model.throughput.toFixed(1)} tok/s</div>
                      <div className='text-muted-foreground tabular-nums'>{formatNumber(model.requestCount)} req</div>
                    </div>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className='text-muted-foreground text-xs'>No model data available</div>
          )}
        </div>
      )}
    </div>
  );
}

export function FastestChannelsCard() {
  const { t } = useTranslation();
  const [timeWindow, setTimeWindow] = useState<TimeWindow>('24h');
  const [isExpanded, setIsExpanded] = useState(false);

  const { data: channels, isLoading, error } = useFastestChannels(timeWindow);
  const { data: expandedChannels, isLoading: expandedLoading } = useFastestChannelsExpanded(timeWindow, isExpanded);

  if (isLoading) {
    return (
      <Card className='hover-card'>
        <CardHeader>
          <Skeleton className='h-5 w-[180px]' />
          <Skeleton className='h-4 w-[120px]' />
        </CardHeader>
        <CardContent>
          <div className='flex h-[250px] items-center justify-center'>
            <Skeleton className='h-[200px] w-full' />
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card className='hover-card'>
        <CardHeader>
          <CardTitle>{t('dashboard.cards.fastestPerformers.channels')}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    );
  }

  const channelData: ChartData[] = (channels || [])
    .slice(0, 5)
    .map((c) => ({ name: c.channelName, throughput: c.throughput, requestCount: c.requestCount }))
    .sort((a, b) => b.throughput - a.throughput);

  const channelTotal = channelData.reduce((sum, item) => sum + item.throughput, 0);

  const channelLegendItems = channelData.map((item, index) => ({
    ...item,
    index: index + 1,
    color: COLORS[index % COLORS.length],
  }));

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div>
          <CardTitle className='text-base font-medium'>{t('dashboard.cards.fastestPerformers.channels')}</CardTitle>
          <CardDescription>Fastest channels by throughput</CardDescription>
        </div>
        <div className='flex items-center gap-2'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => setIsExpanded(!isExpanded)}
            className='h-7 text-xs px-2'
          >
            {isExpanded ? 'Collapse' : 'Expand'}
          </Button>
          <Tabs value={timeWindow} onValueChange={(v) => setTimeWindow(v as TimeWindow)}>
            <TabsList className='h-7 p-0.5'>
              <TabsTrigger value='1h' className='h-6 px-2 text-[10px]'>
                1h
              </TabsTrigger>
              <TabsTrigger value='24h' className='h-6 px-2 text-[10px]'>
                24h
              </TabsTrigger>
              <TabsTrigger value='7d' className='h-6 px-2 text-[10px]'>
                7d
              </TabsTrigger>
            </TabsList>
          </Tabs>
        </div>
      </CardHeader>
      <CardContent>
        {!isExpanded ? (
          <div className='space-y-4'>
            <HorizontalBarChart data={channelData} total={channelTotal} />
            <ChartLegend items={channelLegendItems} />
          </div>
        ) : (
          expandedLoading ? (
            <div className='flex h-[250px] items-center justify-center'>
              <Skeleton className='h-[200px] w-full' />
            </div>
          ) : (expandedChannels || []).length > 0 ? (
            <div>
              {(expandedChannels || []).slice(0, 5).map((channel, index) => (
                <ExpandedChannelItem
                  key={channel.channelId}
                  channel={channel}
                  index={index}
                  defaultExpanded={true}
                />
              ))}
            </div>
          ) : (
            <div className='text-muted-foreground text-sm'>{t('dashboard.cards.fastestPerformers.noData')}</div>
          )
        )}
      </CardContent>
    </Card>
  );
}
