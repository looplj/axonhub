'use client';

import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { FastestPerformersCard } from './fastest-performers-card';
import { useFastestModels } from '../data/fastest-performers';
import type { FastestModel } from '../data/fastest-performers';

export function FastestModelsCard() {
  const { t } = useTranslation();

  return (
    <FastestPerformersCard<FastestModel>
      title={t('dashboard.cards.fastestPerformers.models')}
      description={(totalRequests) => `Fastest models by throughput · ${formatNumber(totalRequests)} total requests`}
      noDataLabel={t('dashboard.cards.fastestPerformers.noData')}
      useData={useFastestModels}
      getName={(item) => item.modelName}
    />
  );
}
