import { useState } from 'react';
import { Zap } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';
import { useFastestChannels, useFastestModels } from '../data/fastest-performers';

type TimeWindow = '1h' | '24h' | '7d';

export function FastestPerformersCard() {
  const { t } = useTranslation();
  const [timeWindow, setTimeWindow] = useState<TimeWindow>('24h');

  const { data: channels, isLoading: channelsLoading, error: channelsError } = useFastestChannels(timeWindow);
  const { data: models, isLoading: modelsLoading, error: modelsError } = useFastestModels(timeWindow);

  const isLoading = channelsLoading || modelsLoading;
  const error = channelsError || modelsError;

  if (isLoading) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <Skeleton className='h-4 w-[120px]' />
          <Skeleton className='h-4 w-4' />
        </CardHeader>
        <CardContent>
          <div className='space-y-4'>
            <Skeleton className='h-6 w-[100px]' />
            <div className='space-y-2'>
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className='flex items-center justify-between'>
                  <Skeleton className='h-4 w-[150px]' />
                  <Skeleton className='h-4 w-[100px]' />
                </div>
              ))}
            </div>
          </div>
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card>
        <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
          <div className='flex items-center gap-2'>
            <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
              <Zap className='h-4 w-4' />
            </div>
            <CardTitle className='text-sm font-medium'>{t('cards.fastestPerformers.title')}</CardTitle>
          </div>
        </CardHeader>
        <CardContent>
          <div className='text-sm text-red-500'>{t('common.loadError')}</div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div className='flex items-center gap-2'>
          <div className='bg-primary/10 text-primary dark:bg-primary/20 rounded-lg p-1.5'>
            <Zap className='h-4 w-4' />
          </div>
          <CardTitle className='text-sm font-medium'>{t('cards.fastestPerformers.title')}</CardTitle>
        </div>
        <Tabs value={timeWindow} onValueChange={(v) => setTimeWindow(v as TimeWindow)}>
          <TabsList className='h-6 p-0.5'>
            <TabsTrigger value='1h' className='h-5 px-2 text-[10px]'>
              1h
            </TabsTrigger>
            <TabsTrigger value='24h' className='h-5 px-2 text-[10px]'>
              24h
            </TabsTrigger>
            <TabsTrigger value='7d' className='h-5 px-2 text-[10px]'>
              7d
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </CardHeader>
      <CardContent>
        <Tabs value={timeWindow} onValueChange={(v) => setTimeWindow(v as TimeWindow)}>
          <TabsContent value='1h'>
            <div className='space-y-4'>
              <div>
                <h4 className='mb-2 text-sm font-medium'>{t('cards.fastestPerformers.channels')}</h4>
                {channels && channels.length > 0 ? (
                  <div className='space-y-2'>
                    {channels.slice(0, 5).map((channel, index) => (
                      <div key={channel.channelId} className='flex items-center justify-between text-sm'>
                        <span className='text-muted-foreground'>
                          {index + 1}. {channel.channelName}
                        </span>
                        <span className='font-medium'>
                          {channel.throughput.toFixed(1)} tokens/s ({channel.requestCount})
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='text-muted-foreground text-sm'>{t('cards.fastestPerformers.noData')}</div>
                )}
              </div>
              <div>
                <h4 className='mb-2 text-sm font-medium'>{t('cards.fastestPerformers.models')}</h4>
                {models && models.length > 0 ? (
                  <div className='space-y-2'>
                    {models.slice(0, 5).map((model, index) => (
                      <div key={model.modelId} className='flex items-center justify-between text-sm'>
                        <span className='text-muted-foreground'>
                          {index + 1}. {model.modelName}
                        </span>
                        <span className='font-medium'>
                          {model.throughput.toFixed(1)} tokens/s ({model.requestCount})
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='text-muted-foreground text-sm'>{t('cards.fastestPerformers.noData')}</div>
                )}
              </div>
            </div>
          </TabsContent>
          <TabsContent value='24h'>
            <div className='space-y-4'>
              <div>
                <h4 className='mb-2 text-sm font-medium'>{t('cards.fastestPerformers.channels')}</h4>
                {channels && channels.length > 0 ? (
                  <div className='space-y-2'>
                    {channels.slice(0, 5).map((channel, index) => (
                      <div key={channel.channelId} className='flex items-center justify-between text-sm'>
                        <span className='text-muted-foreground'>
                          {index + 1}. {channel.channelName}
                        </span>
                        <span className='font-medium'>
                          {channel.throughput.toFixed(1)} tokens/s ({channel.requestCount})
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='text-muted-foreground text-sm'>{t('cards.fastestPerformers.noData')}</div>
                )}
              </div>
              <div>
                <h4 className='mb-2 text-sm font-medium'>{t('cards.fastestPerformers.models')}</h4>
                {models && models.length > 0 ? (
                  <div className='space-y-2'>
                    {models.slice(0, 5).map((model, index) => (
                      <div key={model.modelId} className='flex items-center justify-between text-sm'>
                        <span className='text-muted-foreground'>
                          {index + 1}. {model.modelName}
                        </span>
                        <span className='font-medium'>
                          {model.throughput.toFixed(1)} tokens/s ({model.requestCount})
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='text-muted-foreground text-sm'>{t('cards.fastestPerformers.noData')}</div>
                )}
              </div>
            </div>
          </TabsContent>
          <TabsContent value='7d'>
            <div className='space-y-4'>
              <div>
                <h4 className='mb-2 text-sm font-medium'>{t('cards.fastestPerformers.channels')}</h4>
                {channels && channels.length > 0 ? (
                  <div className='space-y-2'>
                    {channels.slice(0, 5).map((channel, index) => (
                      <div key={channel.channelId} className='flex items-center justify-between text-sm'>
                        <span className='text-muted-foreground'>
                          {index + 1}. {channel.channelName}
                        </span>
                        <span className='font-medium'>
                          {channel.throughput.toFixed(1)} tokens/s ({channel.requestCount})
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='text-muted-foreground text-sm'>{t('cards.fastestPerformers.noData')}</div>
                )}
              </div>
              <div>
                <h4 className='mb-2 text-sm font-medium'>{t('cards.fastestPerformers.models')}</h4>
                {models && models.length > 0 ? (
                  <div className='space-y-2'>
                    {models.slice(0, 5).map((model, index) => (
                      <div key={model.modelId} className='flex items-center justify-between text-sm'>
                        <span className='text-muted-foreground'>
                          {index + 1}. {model.modelName}
                        </span>
                        <span className='font-medium'>
                          {model.throughput.toFixed(1)} tokens/s ({model.requestCount})
                        </span>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className='text-muted-foreground text-sm'>{t('cards.fastestPerformers.noData')}</div>
                )}
              </div>
            </div>
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}
