'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { Check, ChevronsUpDown, Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem } from '@/components/ui/command';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { useProviderQuotaStatuses } from '../data/quotas';
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
  allowedChannelIDs: string[];
}

interface ProviderQuotaCollectionFormData {
  enabled: boolean;
  providers: ProviderQuotaCollectionProvider[];
}

export function QuotaSettings() {
  const { t } = useTranslation();
  const { data: quotaSettings, isLoading: isQuotaSettingsLoading } = useQuotaEnforcementSettings();
  const { data: collectionSettings, isLoading: isCollectionSettingsLoading } = useProviderQuotaCollectionSettings();
  const { channels: providerQuotaChannels } = useProviderQuotaStatuses();
  const updateQuotaEnforcementSettings = useUpdateQuotaEnforcementSettings();
  const updateProviderQuotaCollectionSettings = useUpdateProviderQuotaCollectionSettings();

  const [quotaFormData, setQuotaFormData] = useState<QuotaEnforcementFormData>({
    enabled: false,
    mode: 'EXHAUSTED_ONLY',
    allowedChannelIDs: [],
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
        allowedChannelIDs: quotaSettings.allowedChannelIDs || [],
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
                {updateProviderQuotaCollectionSettings.isPending ? <Loader2 className='h-4 w-4 animate-spin' /> : t('common.buttons.save')}
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
                    onValueChange={(value) => setQuotaFormData((prev) => ({ ...prev, mode: value as QuotaEnforcementMode }))}
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

                <div className='space-y-2'>
                  <Label>{t('system.quota.enforcement.allowedChannels.label')}</Label>
                  <div className='text-muted-foreground text-sm'>{t('system.quota.enforcement.allowedChannels.description')}</div>
                  <ChannelMultiSelect
                    value={quotaFormData.allowedChannelIDs}
                    onChange={(ids) => setQuotaFormData((prev) => ({ ...prev, allowedChannelIDs: ids }))}
                    channels={providerQuotaChannels || []}
                  />
                </div>
              </div>
            )}

            <Separator />

            <div className='flex justify-end'>
              <Button type='submit' disabled={updateQuotaEnforcementSettings.isPending} className='min-w-24'>
                {updateQuotaEnforcementSettings.isPending ? <Loader2 className='h-4 w-4 animate-spin' /> : t('common.buttons.save')}
              </Button>
            </div>
          </form>
        </CardContent>
      </Card>
    </div>
  );
}

function ChannelMultiSelect({
  value,
  onChange,
  channels,
}: {
  value: string[];
  onChange: (v: string[]) => void;
  channels: { id: string; name: string }[];
}) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);

  const handleSelect = (channelId: string) => {
    const newValue = value.includes(channelId) ? value.filter((v) => v !== channelId) : [...value, channelId];
    onChange(newValue);
  };

  const handleRemove = (channelId: string) => {
    onChange(value.filter((v) => v !== channelId));
  };

  return (
    <div className='space-y-2'>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <Button variant='outline' role='combobox' aria-expanded={open} className='w-full justify-between'>
            {value.length > 0 ? t('system.quota.enforcement.allowedChannels.selectedCount', { count: value.length }) : t('common.select.placeholder')}
            <ChevronsUpDown className='ml-2 h-4 w-4 shrink-0 opacity-50' />
          </Button>
        </PopoverTrigger>
        <PopoverContent className='w-full p-0' align='start'>
          <Command>
            <CommandInput placeholder={t('search.placeholder')} />
            <CommandEmpty>{t('common.noResults')}</CommandEmpty>
            <CommandGroup className='max-h-64 overflow-auto'>
              {channels.map((channel) => (
                <CommandItem key={channel.id} value={channel.name} onSelect={() => handleSelect(channel.id)}>
                  <Check className={cn('mr-2 h-4 w-4', value.includes(channel.id) ? 'opacity-100' : 'opacity-0')} />
                  <span>{channel.name}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </Command>
        </PopoverContent>
      </Popover>

      {value.length > 0 && (
        <div className='flex flex-wrap gap-2'>
          {value.map((channelId) => {
            const channel = channels.find((c) => c.id === channelId);
            return (
              <Badge key={channelId} variant='secondary' className='group flex items-center gap-0.5'>
                {channel?.name || channelId}
                <Button
                  variant='ghost'
                  size='sm'
                  className='h-4 w-4 p-0 opacity-70 hover:opacity-100'
                  aria-label={t('system.quota.enforcement.allowedChannels.removeChannel', { name: channel?.name || channelId })}
                  onClick={() => handleRemove(channelId)}
                >
                  ×
                </Button>
              </Badge>
            );
          })}
        </div>
      )}
    </div>
  );
}
