'use client';

import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { FastestPerformersCard } from './fastest-performers-card';
import { useFastestChannels } from '../data/fastest-performers';
import type { FastestChannel } from '../data/fastest-performers';

interface FastestChannelsCardProps {
  timeWindow: string;
}

export function FastestChannelsCard({ timeWindow }: FastestChannelsCardProps) {
  const { t } = useTranslation();

  return (
    <FastestPerformersCard<FastestChannel>
      title={t('dashboard.cards.fastestPerformers.channels')}
      description={(totalRequests) => t('dashboard.cards.fastestPerformers.description', { type: t('dashboard.cards.fastestPerformers.channelType'), count: formatNumber(totalRequests) })}
      noDataLabel={t('dashboard.cards.fastestPerformers.noData')}
      timeWindow={timeWindow}
      useData={useFastestChannels}
      getName={(item) => item.channelName}
    />
  );
}
