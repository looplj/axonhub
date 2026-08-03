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
  useQuotaEnforcementSettings,
  useUpdateProviderQuotaCollectionSettings,
  useUpdateQuotaEnforcementSettings,
  type ProviderQuotaCollectionProvider,
  type QuotaEnforcementMode,
} from '../data/system';

interface QuotaEnforcementFormData {
  enabled: boolean;
  mode: QuotaEnforcementMode;
}

interface ProviderQuotaCollectionFormData {
  enabled: boolean;
  providers: ProviderQuotaCollectionProvider[];
}

export function QuotaSettings() {
  const { t } = useTranslation();
  const { data: quotaSettings, isLoading: isQuotaSettingsLoading } = useQuotaEnforcementSettings();
  const { data: collectionSettings, isLoading: isCollectionSettingsLoading } = useProviderQuotaCollectionSettings();
  const updateQuotaEnforcementSettings = useUpdateQuotaEnforcementSettings();
  const updateProviderQuotaCollectionSettings = useUpdateProviderQuotaCollectionSettings();

  const [quotaFormData, setQuotaFormData] = useState<QuotaEnforcementFormData>({
    enabled: false,
    mode: 'EXHAUSTED_ONLY',
  });
  const [collectionFormData, setCollectionFormData] = useState<ProviderQuotaCollectionFormData>({
    enabled: true,
    providers: [],
  });

  useEffect(() => {
    if (quotaSettings) {
      setQuotaFormData({
        enabled: quotaSettings.enabled,
        mode: quotaSettings.mode,
      });
    }
  }, [quotaSettings]);

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
      providers: prev.providers.map((provider) =>
        provider.provider === providerType ? { ...provider, enabled: checked } : provider
      ),
    }));
  }, []);

  const handleCollectionSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      await updateProviderQuotaCollectionSettings.mutateAsync(collectionFormData);
    },
    [collectionFormData, updateProviderQuotaCollectionSettings]
  );

  const handleQuotaSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      await updateQuotaEnforcementSettings.mutateAsync(quotaFormData);
    },
    [quotaFormData, updateQuotaEnforcementSettings]
  );

  if (isQuotaSettingsLoading || isCollectionSettingsLoading) {
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
                {updateProviderQuotaCollectionSettings.isPending ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : (
                  t('common.buttons.save')
                )}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('system.quota.title')}</CardTitle>
          <CardDescription>{t('system.quota.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleQuotaSubmit} className='space-y-6'>
            <div className='flex items-center justify-between' id='quota-enabled-switch'>
              <div className='space-y-0.5'>
                <Label htmlFor='quota-enabled' className='text-base'>
                  {t('system.quota.enabled.label')}
                </Label>
                <div className='text-muted-foreground text-sm'>{t('system.quota.enabled.description')}</div>
              </div>
              <Switch
                id='quota-enabled'
                checked={quotaFormData.enabled}
                onCheckedChange={(checked) => setQuotaFormData((prev) => ({ ...prev, enabled: checked }))}
              />
            </div>

            <Separator />

            {quotaFormData.enabled && (
              <div className='space-y-4'>
                <div className='space-y-2'>
                  <Label htmlFor='quota-mode'>{t('system.quota.mode.label')}</Label>
                  <div className='text-muted-foreground mb-2 text-sm'>{t('system.quota.mode.description')}</div>
                  <Select
                    value={quotaFormData.mode}
                    onValueChange={(value) =>
                      setQuotaFormData((prev) => ({ ...prev, mode: value as QuotaEnforcementMode }))
                    }
                  >
                    <SelectTrigger id='quota-mode' className='w-56'>
                      <SelectValue placeholder={t('system.quota.mode.placeholder')} />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value='EXHAUSTED_ONLY'>{t('system.quota.mode.options.exhaustedOnly')}</SelectItem>
                      <SelectItem value='DE_PRIORITIZE'>{t('system.quota.mode.options.dePrioritize')}</SelectItem>
                    </SelectContent>
                  </Select>

                  {quotaFormData.mode && (
                    <div className='bg-muted/50 mt-3 rounded-md border p-3'>
                      <div className='text-muted-foreground text-xs leading-relaxed'>
                        {t(`system.quota.mode.documentation.${quotaFormData.mode}`)}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}

            <Separator />

            <div className='flex justify-end'>
              <Button type='submit' disabled={updateQuotaEnforcementSettings.isPending} className='min-w-24'>
                {updateQuotaEnforcementSettings.isPending ? (
                  <Loader2 className='h-4 w-4 animate-spin' />
                ) : (
                  t('common.buttons.save')
                )}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}
