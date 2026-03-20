'use client';

import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { FastestPerformersCard } from './fastest-performers-card';
import { useFastestModels } from '../data/fastest-performers';
import type { FastestModel } from '../data/fastest-performers';

interface FastestModelsCardProps {
  timeWindow: string;
}

export function FastestModelsCard({ timeWindow }: FastestModelsCardProps) {
  const { t } = useTranslation();

  return (
    <FastestPerformersCard<FastestModel>
      title={t('dashboard.cards.fastestPerformers.models')}
      description={(totalRequests) => t('dashboard.cards.fastestPerformers.description', { type: t('dashboard.cards.fastestPerformers.modelType'), count: formatNumber(totalRequests) })}
      noDataLabel={t('dashboard.cards.fastestPerformers.noData')}
      timeWindow={timeWindow}
      useData={useFastestModels}
      getName={(item) => item.modelName}
    />
  );
}
