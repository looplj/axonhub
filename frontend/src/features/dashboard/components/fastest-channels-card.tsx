'use client';

import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { FastestPerformersCard } from './fastest-performers-card';
import { useFastestChannels } from '../data/fastest-performers';
import type { FastestChannel } from '../data/fastest-performers';
import type { CoarseTimeWindow } from '../utils/time-window';

interface FastestChannelsCardProps {
  timeWindow: CoarseTimeWindow;
}

export function FastestChannelsCard({ timeWindow }: FastestChannelsCardProps) {
  const { t } = useTranslation();

  return (
    <FastestPerformersCard<FastestChannel>
      title={t('dashboard.cards.fastestPerformers.channels')}
      description={(totalRequests) => t('dashboard.cards.fastestPerformers.description', { type: t('dashboard.cards.fastestPerformers.channelType'), count: formatNumber(totalRequests) })}
      noDataLabel={t('dashboard.cards.fastestPerformers.noData')}
      useData={useFastestChannels}
      getName={(item) => item.channelName}
      timeWindow={timeWindow}
    />
  );
}
