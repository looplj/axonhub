'use client';

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Skeleton } from '@/components/ui/skeleton';
import { useFastestChannels, useFastestModels } from '../data/fastest-performers';
import type { FastestChannel, FastestModel } from '../data/fastest-performers';

interface PerformersListProps {
  channels: FastestChannel[] | undefined;
  models: FastestModel[] | undefined;
}

function PerformersList({ channels, models }: PerformersListProps) {
  const { t } = useTranslation();

  return (
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
                  {channel.throughput.toFixed(1)} {t('dashboard.cards.fastestPerformers.tokensPerSecond')} ({channel.requestCount} {t('dashboard.cards.fastestPerformers.requests')})
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
                  {model.throughput.toFixed(1)} {t('dashboard.cards.fastestPerformers.tokensPerSecond')} ({model.requestCount} {t('dashboard.cards.fastestPerformers.requests')})
                </span>
              </div>
            ))}
          </div>
        ) : (
          <div className='text-muted-foreground text-sm'>{t('cards.fastestPerformers.noData')}</div>
        )}
      </div>
    </div>
  );
}

export function FastestPerformersCard() {
  const { t } = useTranslation();
  const [timeWindow, setTimeWindow] = useState<'1h' | '24h' | '7d'>('24h');

  const { data: channels, isLoading: isLoadingChannels } = useFastestChannels(timeWindow);
  const { data: models, isLoading: isLoadingModels } = useFastestModels(timeWindow);

  const isLoading = isLoadingChannels || isLoadingModels;

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

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between space-y-0 pb-2'>
        <div>
          <CardTitle className='text-base font-medium'>{t('cards.fastestPerformers.title')}</CardTitle>
          <CardDescription>{t('cards.fastestPerformers.description')}</CardDescription>
        </div>
        <Tabs value={timeWindow} onValueChange={(v) => setTimeWindow(v as '1h' | '24h' | '7d')}>
          <TabsList className='h-6 p-0.5'>
            <TabsTrigger value='1h' className='h-5 px-2 text-[10px]'>
              {t('dashboard.timeWindow.1h')}
            </TabsTrigger>
            <TabsTrigger value='24h' className='h-5 px-2 text-[10px]'>
              {t('dashboard.timeWindow.24h')}
            </TabsTrigger>
            <TabsTrigger value='7d' className='h-5 px-2 text-[10px]'>
              {t('dashboard.timeWindow.7d')}
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </CardHeader>
      <CardContent>
        <Tabs value={timeWindow} onValueChange={(v) => setTimeWindow(v as '1h' | '24h' | '7d')}>
          <TabsContent value='1h'>
            <PerformersList channels={channels} models={models} />
          </TabsContent>
          <TabsContent value='24h'>
            <PerformersList channels={channels} models={models} />
          </TabsContent>
          <TabsContent value='7d'>
            <PerformersList channels={channels} models={models} />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}
