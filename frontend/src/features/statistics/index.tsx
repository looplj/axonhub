import { useState } from 'react';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useTranslation } from 'react-i18next';
import { TimeFilter, type StatisticsTimeWindow } from './components/time-filter';
import { ChannelStatisticsTable } from './components/channel-statistics-table';
import { ModelStatisticsTable } from './components/model-statistics-table';

export function StatisticsPage() {
  const { t } = useTranslation();
  const [timeWindow, setTimeWindow] = useState<StatisticsTimeWindow>('day');
  const [activeTab, setActiveTab] = useState<'channel' | 'model'>('channel');

  return (
    <div className="space-y-6 p-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">{t('statistics.title')}</h1>
        <TimeFilter value={timeWindow} onChange={setTimeWindow} />
      </div>

      <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as 'channel' | 'model')}>
        <TabsList>
          <TabsTrigger value="channel">{t('statistics.byChannel')}</TabsTrigger>
          <TabsTrigger value="model">{t('statistics.byModel')}</TabsTrigger>
        </TabsList>

        <TabsContent value="channel">
          <ChannelStatisticsTable timeWindow={timeWindow} />
        </TabsContent>

        <TabsContent value="model">
          <ModelStatisticsTable timeWindow={timeWindow} />
        </TabsContent>
      </Tabs>
    </div>
  );
}
