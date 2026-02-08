'use client';

import { useTranslation } from 'react-i18next';
import { Zap } from 'lucide-react';
import { formatNumber } from '@/utils/format-number';
import { FastestPerformersCard } from './fastest-performers-card';
import { useFastestChannels } from '../data/fastest-performers';
import type { FastestChannel } from '../data/fastest-performers';

export function FastestChannelsCard() {
  const { t } = useTranslation();

  return (
    <FastestPerformersCard<FastestChannel>
      title={t('dashboard.cards.fastestPerformers.channels')}
      titleIcon={<Zap className="h-4 w-4" />}
      description={(totalRequests) => t('dashboard.cards.fastestPerformers.description', { type: t('dashboard.cards.fastestPerformers.channelType'), count: formatNumber(totalRequests) })}
      noDataLabel={t('dashboard.cards.fastestPerformers.noData')}
      useData={useFastestChannels}
      getName={(item) => item.channelName}
    />
  );
}
