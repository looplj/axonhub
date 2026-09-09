'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import {
  useProviderQuotaCollectionSettings,
  useQuotaRoutingSettings,
  useUpdateProviderQuotaCollectionSettings,
  useUpdateQuotaRoutingSettings,
  type ProviderQuotaCollectionProvider,
  type QuotaRoutingMode,
} from '../data/system';

interface ProviderQuotaCollectionFormData {
  enabled: boolean;
  providers: ProviderQuotaCollectionProvider[];
}

export function QuotaSettings() {
  const { t } = useTranslation();
  const { data: routingSettings, isError: isRoutingSettingsError, isLoading: isRoutingSettingsLoading } = useQuotaRoutingSettings();
  const { data: collectionSettings, isLoading: isCollectionSettingsLoading } = useProviderQuotaCollectionSettings();
  const updateQuotaRoutingSettings = useUpdateQuotaRoutingSettings();
  const updateProviderQuotaCollectionSettings = useUpdateProviderQuotaCollectionSettings();

  const [routingMode, setRoutingMode] = useState<QuotaRoutingMode>('IGNORE_QUOTA');
  const [collectionFormData, setCollectionFormData] = useState<ProviderQuotaCollectionFormData>({
    enabled: true,
    providers: [],
  });

  useEffect(() => {
    if (routingSettings) {
      setRoutingMode(routingSettings.defaultMode);
    }
  }, [routingSettings]);

  useEffect(() => {
    if (collectionSettings) {
      setCollectionFormData({
        enabled: collectionSettings.enabled,
        providers: collectionSettings.providers,
      });
    }
  }, [collectionSettings]);

  const handleCollectionProviderChange = useCallback((providerType: string, checked: boolean) => {
    setCollectionFormData((prev) => ({
      ...prev,
      providers: prev.providers.map((provider) => (provider.provider === providerType ? { ...provider, enabled: checked } : provider)),
    }));
  }, []);

  const handleCollectionSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      await updateProviderQuotaCollectionSettings.mutateAsync(collectionFormData);
    },
    [collectionFormData, updateProviderQuotaCollectionSettings]
  );

  const handleRoutingSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      await updateQuotaRoutingSettings.mutateAsync({ defaultMode: routingMode });
    },
    [routingMode, updateQuotaRoutingSettings]
  );

  if (isRoutingSettingsLoading || isCollectionSettingsLoading) {
    return (
      <div className='flex items-center justify-center p-8'>
        <Loader2 className='h-8 w-8 animate-spin' />
      </div>
    );
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.quota.collection.title')}</CardTitle>
          <CardDescription>{t('system.quota.collection.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleCollectionSubmit} className='space-y-6'>
            <div className='flex items-center justify-between' id='provider-quota-collection-enabled-switch'>
              <div className='space-y-0.5'>
                <Label htmlFor='provider-quota-collection-enabled' className='text-base'>
                  {t('system.quota.collection.enabled.label')}
                </Label>
                <div className='text-muted-foreground text-sm'>{t('system.quota.collection.enabled.description')}</div>
              </div>
              <Switch
                id='provider-quota-collection-enabled'
                checked={collectionFormData.enabled}
                onCheckedChange={(checked) => setCollectionFormData((prev) => ({ ...prev, enabled: checked }))}
              />
            </div>

            <Separator />

            <div className='grid gap-x-8 gap-y-4 sm:grid-cols-2'>
              {collectionFormData.providers.map((provider) => {
                const switchID = `provider-quota-collection-${provider.provider}`;
                return (
                  <div key={provider.provider} className='flex items-center justify-between gap-4'>
                    <Label htmlFor={switchID}>{t(`system.quota.collection.providers.${provider.provider}`)}</Label>
                    <Switch
                      id={switchID}
                      checked={provider.enabled}
                      disabled={!collectionFormData.enabled}
                      onCheckedChange={(checked) => handleCollectionProviderChange(provider.provider, checked)}
                    />
                  </div>
                );
              })}
            </div>

            <Separator />

            <div className='flex justify-end'>
              <Button type='submit' disabled={updateProviderQuotaCollectionSettings.isPending} className='min-w-24'>
                {updateProviderQuotaCollectionSettings.isPending ? <Loader2 className='h-4 w-4 animate-spin' /> : t('common.buttons.save')}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('system.quota.routing.title')}</CardTitle>
          <CardDescription>{t('system.quota.routing.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleRoutingSubmit} className='space-y-6'>
            <div className='space-y-2'>
              <Label htmlFor='quota-routing-default-mode'>{t('system.quota.routing.mode.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.quota.routing.mode.description')}</div>
              <Select value={routingMode} onValueChange={(value) => setRoutingMode(value as QuotaRoutingMode)}>
                <SelectTrigger id='quota-routing-default-mode' className='w-56'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='IGNORE_QUOTA'>{t('system.quota.routing.modes.IGNORE_QUOTA')}</SelectItem>
                  <SelectItem value='REMOVE_ON_EXHAUSTED'>{t('system.quota.routing.modes.REMOVE_ON_EXHAUSTED')}</SelectItem>
                  <SelectItem value='BACKPRESSURE'>{t('system.quota.routing.modes.BACKPRESSURE')}</SelectItem>
                </SelectContent>
              </Select>

              <div className='bg-muted/50 mt-3 rounded-md border p-3'>
                <div className='text-muted-foreground text-xs leading-relaxed'>
                  {t(`system.quota.routing.modes.${routingMode}.description`)}
                </div>
              </div>
            </div>

            <Separator />

            <div className='flex justify-end'>
              <Button
                type='submit'
                disabled={updateQuotaRoutingSettings.isPending || isRoutingSettingsLoading || isRoutingSettingsError || !routingSettings}
                className='min-w-24'
              >
                {updateQuotaRoutingSettings.isPending ? <Loader2 className='h-4 w-4 animate-spin' /> : t('common.buttons.save')}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
