'use client';

import { useEffect, useState } from 'react';
import { Loader2, RefreshCw, Save } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useRefreshProvidersCatalog } from '@/features/models/data/providers';
import { useCatalogSettings, useUpdateCatalogSettings } from '../data/system';

const MIN_CATALOG_REFRESH_SECONDS = 60;
const MAX_CATALOG_REFRESH_SECONDS = 604800;

function parseRefreshSeconds(value: string): number {
  return Number.parseInt(value, 10);
}

function isValidRefreshSeconds(value: number): boolean {
  return Number.isInteger(value) && value >= MIN_CATALOG_REFRESH_SECONDS && value <= MAX_CATALOG_REFRESH_SECONDS;
}

function clampRefreshSeconds(value: number): number {
  return Math.min(MAX_CATALOG_REFRESH_SECONDS, Math.max(MIN_CATALOG_REFRESH_SECONDS, value));
}

export function CatalogSettings() {
  const { t } = useTranslation();
  const { data, isLoading } = useCatalogSettings();
  const updateSettings = useUpdateCatalogSettings();
  const refreshCatalog = useRefreshProvidersCatalog();
  const [upstreamURL, setUpstreamURL] = useState('');
  const [refreshSeconds, setRefreshSeconds] = useState(3600);
  const canSave = Boolean(data) && isValidRefreshSeconds(refreshSeconds) && !updateSettings.isPending;

  useEffect(() => {
    if (!data) {
      return;
    }

    setUpstreamURL(data.upstreamURL);
    setRefreshSeconds(data.refreshSeconds);
  }, [data]);

  if (isLoading) {
    return (
      <div className='flex h-24 items-center justify-center'>
        <Loader2 className='h-5 w-5 animate-spin' />
      </div>
    );
  }

  const handleSave = async () => {
    if (!data || !isValidRefreshSeconds(refreshSeconds)) {
      return;
    }

    await updateSettings.mutateAsync({
      upstreamURL: upstreamURL.trim(),
      refreshSeconds: clampRefreshSeconds(refreshSeconds),
    });
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.catalog.title')}</CardTitle>
        <CardDescription>{t('system.catalog.description')}</CardDescription>
      </CardHeader>
      <CardContent className='space-y-6'>
        <div className='space-y-2'>
          <Label htmlFor='catalog-upstream-url'>{t('system.catalog.upstreamURL.label')}</Label>
          <Input
            id='catalog-upstream-url'
            value={upstreamURL}
            onChange={(event) => setUpstreamURL(event.target.value)}
            placeholder='https://raw.githubusercontent.com/ThinkInAIXYZ/PublicProviderConf/refs/heads/dev/dist/all.json'
          />
          <p className='text-muted-foreground text-sm'>{t('system.catalog.upstreamURL.description')}</p>
        </div>
        <div className='space-y-2'>
          <Label htmlFor='catalog-refresh-seconds'>{t('system.catalog.refreshSeconds.label')}</Label>
          <Input
            id='catalog-refresh-seconds'
            type='number'
            min={MIN_CATALOG_REFRESH_SECONDS}
            max={MAX_CATALOG_REFRESH_SECONDS}
            step={1}
            value={Number.isInteger(refreshSeconds) ? refreshSeconds : ''}
            onChange={(event) => {
              const parsed = parseRefreshSeconds(event.target.value);
              setRefreshSeconds(Number.isNaN(parsed) ? Number.NaN : parsed);
            }}
            onBlur={() => {
              if (Number.isInteger(refreshSeconds)) {
                setRefreshSeconds(clampRefreshSeconds(refreshSeconds));
              }
            }}
          />
          <p className='text-muted-foreground text-sm'>{t('system.catalog.refreshSeconds.description')}</p>
        </div>
        <div className='flex flex-wrap gap-2'>
          <Button onClick={handleSave} disabled={!canSave}>
            {updateSettings.isPending ? <Loader2 className='mr-2 h-4 w-4 animate-spin' /> : <Save className='mr-2 h-4 w-4' />}
            {t('system.buttons.save')}
          </Button>
          <Button variant='outline' onClick={() => refreshCatalog.mutate()} disabled={refreshCatalog.isPending}>
            {refreshCatalog.isPending ? (
              <Loader2 className='mr-2 h-4 w-4 animate-spin' />
            ) : (
              <RefreshCw className='mr-2 h-4 w-4' />
            )}
            {t('system.catalog.refreshNow')}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
